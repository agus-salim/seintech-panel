package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/acme"

	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
)

type SSLStatusInfo struct {
	IsActive      bool       `json:"is_active"`
	Provider      string     `json:"provider"`
	Issuer        string     `json:"issuer"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	StatusText    string     `json:"status_text"`
	IsLetsEncrypt bool       `json:"is_lets_encrypt"`
}

type SSLManager struct {
	repo     *storage.Repository
	registry *webserver.Registry
	certBase string
	acmeRoot string
}

func NewSSLManager(repo *storage.Repository, registry *webserver.Registry, certBase, acmeRoot string) *SSLManager {
	_ = os.MkdirAll(certBase, 0755)
	_ = os.Chmod(certBase, 0755)
	_ = os.MkdirAll(acmeRoot, 0755)
	_ = os.Chmod(acmeRoot, 0755)
	if parent := filepath.Dir(acmeRoot); parent != "" && parent != "/" {
		_ = os.Chmod(parent, 0755)
	}

	return &SSLManager{
		repo:     repo,
		registry: registry,
		certBase: certBase,
		acmeRoot: acmeRoot,
	}
}

// GenerateSelfSigned creates an RSA 2048-bit self-signed certificate for local development or default panel HTTPS.
func (s *SSLManager) GenerateSelfSigned(domain string, certPath, keyPath string) error {
	_ = os.MkdirAll(filepath.Dir(certPath), 0755)
	_ = os.MkdirAll(filepath.Dir(keyPath), 0700)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Seintech Hosting Panel"},
			CommonName:   domain,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(domain); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, domain)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}

	return nil
}

// CheckCertExpiry inspects a PEM certificate file and returns its expiration time.
func (s *SSLManager) CheckCertExpiry(certPath string) (*time.Time, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &cert.NotAfter, nil
}

// GetDomainSSLStatus inspects the certificate on disk and returns authentic verification data.
func (s *SSLManager) GetDomainSSLStatus(domain *model.Domain) SSLStatusInfo {
	info := SSLStatusInfo{
		IsActive:      false,
		Provider:      string(domain.SSLProvider),
		StatusText:    "Belum Aktif",
		IsLetsEncrypt: false,
	}

	if !domain.SSLEnabled || domain.SSLCertPath == "" {
		return info
	}

	data, err := os.ReadFile(domain.SSLCertPath)
	if err != nil {
		info.StatusText = "Sertifikat Tidak Ditemukan"
		return info
	}

	block, _ := pem.Decode(data)
	if block == nil {
		info.StatusText = "Format Sertifikat Rusak"
		return info
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		info.StatusText = "Sertifikat Tidak Valid"
		return info
	}

	info.ExpiresAt = &cert.NotAfter

	issuerOrg := ""
	if len(cert.Issuer.Organization) > 0 {
		issuerOrg = cert.Issuer.Organization[0]
	}
	info.Issuer = issuerOrg
	if info.Issuer == "" {
		info.Issuer = cert.Issuer.CommonName
	}

	// Check if certificate has expired
	if time.Now().After(cert.NotAfter) {
		info.StatusText = fmt.Sprintf("Kadaluarsa (%s)", cert.NotAfter.Format("02 Jan 2006"))
		return info
	}

	// Determine if issued by genuine Let's Encrypt CA
	isLE := strings.Contains(info.Issuer, "Let's Encrypt") ||
		strings.Contains(cert.Issuer.CommonName, "Let's Encrypt") ||
		info.Issuer == "R3" || info.Issuer == "R10" || info.Issuer == "R11" ||
		info.Issuer == "E1" || info.Issuer == "E5" || info.Issuer == "E6" ||
		strings.Contains(info.Issuer, "ISRG")

	// Determine if this is the fallback/dummy self-signed certificate
	isDummy := strings.Contains(info.Issuer, "Seintech Hosting Panel") ||
		(cert.Issuer.CommonName == domain.DomainName && len(cert.Issuer.Organization) == 0)

	if domain.SSLProvider == model.SSLProviderLetsEncrypt {
		if isDummy || !isLE {
			// Was marked as Let's Encrypt in DB, but certificate is not genuine Let's Encrypt
			info.IsActive = false
			info.IsLetsEncrypt = false
			info.StatusText = "Belum Aktif (Belum Terverifikasi)"
			return info
		}
		info.IsActive = true
		info.IsLetsEncrypt = true
		info.StatusText = "Active & Valid"
		return info
	}

	// Other SSL providers
	if domain.SSLProvider == model.SSLProviderCustom || domain.SSLProvider == model.SSLProviderSelfSigned {
		info.IsActive = true
		info.StatusText = "Active"
		return info
	}

	return info
}

// SyncDomainSSLStatus checks disk state and aligns domain.SSLEnabled and repo if status differs.
func (s *SSLManager) SyncDomainSSLStatus(domain *model.Domain) SSLStatusInfo {
	status := s.GetDomainSSLStatus(domain)

	changed := false
	if domain.SSLEnabled != status.IsActive {
		domain.SSLEnabled = status.IsActive
		changed = true
	}

	if !status.IsActive && domain.SSLProvider == model.SSLProviderLetsEncrypt && !status.IsLetsEncrypt {
		domain.SSLProvider = model.SSLProviderNone
		changed = true
	}

	if status.ExpiresAt != nil && (domain.SSLExpiry == nil || !domain.SSLExpiry.Equal(*status.ExpiresAt)) {
		domain.SSLExpiry = status.ExpiresAt
		changed = true
	}

	if changed {
		_ = s.repo.UpsertDomain(*domain)
	}

	return status
}

// getOrCreateAccountKey loads or creates an ECDSA account key for ACME.
func (s *SSLManager) getOrCreateAccountKey() (crypto.Signer, error) {
	keyPath := filepath.Join(s.certBase, "acme_account.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if signer, ok := key.(crypto.Signer); ok {
					return signer, nil
				}
			}
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err == nil {
		_ = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0600)
	}
	return key, nil
}

// tryCertbot attempts certificate issuance using local certbot if available on the system.
func (s *SSLManager) tryCertbot(ctx context.Context, domainName string, certPath, keyPath string) error {
	certbotPath, err := exec.LookPath("certbot")
	if err != nil {
		return errors.New("certbot not installed on system")
	}

	cmd := exec.CommandContext(ctx, certbotPath, "certonly",
		"--webroot",
		"-w", s.acmeRoot,
		"-d", domainName,
		"--agree-tos",
		"--register-unsafely-without-email",
		"--non-interactive",
		"--keep-until-expiring",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot error: %s (%w)", string(output), err)
	}

	liveDir := filepath.Join("/etc/letsencrypt/live", domainName)
	srcCert := filepath.Join(liveDir, "fullchain.pem")
	srcKey := filepath.Join(liveDir, "privkey.pem")

	certData, err := os.ReadFile(srcCert)
	if err != nil {
		return fmt.Errorf("gagal membaca sertifikat certbot: %w", err)
	}
	keyData, err := os.ReadFile(srcKey)
	if err != nil {
		return fmt.Errorf("gagal membaca private key certbot: %w", err)
	}

	if err := os.WriteFile(certPath, certData, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return err
	}

	return nil
}

// provisionNativeACME uses golang.org/x/crypto/acme for direct Let's Encrypt HTTP-01 issuance.
func (s *SSLManager) provisionNativeACME(ctx context.Context, domainName string, certPath, keyPath string) error {
	accountKey, err := s.getOrCreateAccountKey()
	if err != nil {
		return fmt.Errorf("gagal menyiapkan account key ACME: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: acme.LetsEncryptURL,
	}

	// Register account (accept TOS)
	_, err = client.Register(ctx, &acme.Account{}, acme.AcceptTOS)
	if err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		var acmeErr *acme.Error
		if errors.As(err, &acmeErr) && acmeErr.StatusCode != 409 {
			return fmt.Errorf("gagal registrasi akun Let's Encrypt: %w", err)
		}
	}

	// Authorize order for domain
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domainName))
	if err != nil {
		return fmt.Errorf("gagal membuat pesanan otorisasi Let's Encrypt: %w", err)
	}

	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("gagal mengambil data otorisasi ACME: %w", err)
		}
		if authz.Status == acme.StatusValid {
			continue
		}

		var httpChal *acme.Challenge
		for _, chal := range authz.Challenges {
			if chal.Type == "http-01" {
				httpChal = chal
				break
			}
		}
		if httpChal == nil {
			return errors.New("tidak ada challenge HTTP-01 yang didukung dari Let's Encrypt")
		}

		keyAuth, err := client.HTTP01ChallengeResponse(httpChal.Token)
		if err != nil {
			return fmt.Errorf("gagal membuat respon HTTP-01: %w", err)
		}

		// Write token to ACME challenge roots with explicit 0644 / 0755 permissions
		wellKnownDir := filepath.Join(s.acmeRoot, ".well-known", "acme-challenge")
		_ = os.MkdirAll(wellKnownDir, 0755)
		_ = os.Chmod(s.acmeRoot, 0755)
		_ = os.Chmod(filepath.Join(s.acmeRoot, ".well-known"), 0755)
		_ = os.Chmod(wellKnownDir, 0755)

		_ = os.WriteFile(filepath.Join(s.acmeRoot, httpChal.Token), []byte(keyAuth), 0644)
		_ = os.Chmod(filepath.Join(s.acmeRoot, httpChal.Token), 0644)
		_ = os.WriteFile(filepath.Join(wellKnownDir, httpChal.Token), []byte(keyAuth), 0644)
		_ = os.Chmod(filepath.Join(wellKnownDir, httpChal.Token), 0644)

		if s.repo != nil {
			domains, _ := s.repo.GetDomains()
			for _, d := range domains {
				if strings.EqualFold(d.DomainName, domainName) && d.DocumentRoot != "" {
					docWellKnown := filepath.Join(d.DocumentRoot, ".well-known", "acme-challenge")
					_ = os.MkdirAll(docWellKnown, 0755)
					_ = os.Chmod(filepath.Join(d.DocumentRoot, ".well-known"), 0755)
					_ = os.Chmod(docWellKnown, 0755)
					_ = os.WriteFile(filepath.Join(docWellKnown, httpChal.Token), []byte(keyAuth), 0644)
					_ = os.Chmod(filepath.Join(docWellKnown, httpChal.Token), 0644)
					break
				}
			}
		}

		// Accept challenge
		if _, err := client.Accept(ctx, httpChal); err != nil {
			return fmt.Errorf("gagal menerima challenge Let's Encrypt: %w", err)
		}

		// Wait for authorization to succeed
		if _, err := client.WaitAuthorization(ctx, authzURL); err != nil {
			return fmt.Errorf("verifikasi challenge Let's Encrypt gagal (pastikan domain mengarah ke IP publik server dan port 80 terbuka): %w", err)
		}
	}

	// Wait for order status to become Ready
	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return fmt.Errorf("status order Let's Encrypt gagal: %w", err)
	}

	// Generate RSA 2048 private key for the certificate
	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("gagal membuat private key sertifikat: %w", err)
	}

	csrReq := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domainName},
		DNSNames: []string{domainName},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrReq, certKey)
	if err != nil {
		return fmt.Errorf("gagal membuat CSR: %w", err)
	}

	derBundle, _, err := client.CreateCertFromOrder(ctx, order, csrDER, true)
	if err != nil {
		return fmt.Errorf("gagal menerbitkan sertifikat dari Let's Encrypt: %w", err)
	}

	// Encode full chain into PEM
	var chainPEM strings.Builder
	for _, der := range derBundle {
		if err := pem.Encode(&chainPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
			return fmt.Errorf("gagal encode sertifikat PEM: %w", err)
		}
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(certKey)
	if err != nil {
		return fmt.Errorf("gagal encode private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	if err := os.WriteFile(certPath, []byte(chainPEM.String()), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return err
	}

	return nil
}

// ProvisionLetsEncrypt prepares challenges, requests Let's Encrypt cert, and configures webserver.
func (s *SSLManager) ProvisionLetsEncrypt(ctx context.Context, domain *model.Domain) error {
	// Pre-flight check: verify domain DNS resolution
	ips, err := net.LookupHost(domain.DomainName)
	if err != nil || len(ips) == 0 {
		// Fallback to public DNS (8.8.8.8) in case local systemd-resolved has lookup latency
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(dialCtx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(dialCtx, "udp", "8.8.8.8:53")
			},
		}
		ips, err = r.LookupHost(ctx, domain.DomainName)
	}
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("domain '%s' belum dapat di-resolve oleh DNS publik. Pastikan DNS A record sudah terpasang dan mengarah ke IP publik server ini", domain.DomainName)
	}

	domainDir := filepath.Join(s.certBase, domain.DomainName)
	_ = os.MkdirAll(domainDir, 0755)
	_ = os.Chmod(domainDir, 0755)

	certPath := filepath.Join(domainDir, "fullchain.pem")
	keyPath := filepath.Join(domainDir, "privkey.pem")

	// Ensure challenge directories exist with proper 0755 permissions
	_ = os.MkdirAll(s.acmeRoot, 0755)
	_ = os.Chmod(s.acmeRoot, 0755)
	wellKnownDir := filepath.Join(s.acmeRoot, ".well-known", "acme-challenge")
	_ = os.MkdirAll(wellKnownDir, 0755)
	_ = os.Chmod(filepath.Join(s.acmeRoot, ".well-known"), 0755)
	_ = os.Chmod(wellKnownDir, 0755)
	if parent := filepath.Dir(s.acmeRoot); parent != "" && parent != "/" {
		_ = os.Chmod(parent, 0755)
	}

	// Step 1: Try Certbot if available
	err = s.tryCertbot(ctx, domain.DomainName, certPath, keyPath)
	if err != nil {
		// Step 2: Fallback to native Go ACME client
		err = s.provisionNativeACME(ctx, domain.DomainName, certPath, keyPath)
	}

	if err != nil {
		return fmt.Errorf("gagal menerbitkan sertifikat Let's Encrypt: %w", err)
	}

	// Inspect the newly generated certificate
	expiry, err := s.CheckCertExpiry(certPath)
	if err != nil {
		return fmt.Errorf("sertifikat terbit tetapi gagal divalidasi: %w", err)
	}

	domain.SSLEnabled = true
	domain.SSLProvider = model.SSLProviderLetsEncrypt
	domain.SSLCertPath = certPath
	domain.SSLKeyPath = keyPath
	domain.SSLExpiry = expiry
	domain.UpdatedAt = time.Now()

	// Update domain in storage
	if err := s.repo.UpsertDomain(*domain); err != nil {
		return fmt.Errorf("gagal menyimpan domain ke storage: %w", err)
	}

	// Re-generate webserver vhost and reload webserver
	ws, err := s.registry.Get(domain.WebServerType)
	if err != nil {
		return fmt.Errorf("webserver '%s' tidak ditemukan: %w", domain.WebServerType, err)
	}

	if err := ws.GenerateVHost(ctx, domain); err != nil {
		return fmt.Errorf("gagal meregenerasi virtual host: %w", err)
	}

	if err := ws.Reload(ctx); err != nil {
		// Fallback to restart if reload fails
		_ = ws.Restart(ctx)
	}

	return nil
}

// StartAutoRenewalWorker starts a background loop that checks all domains every 12 hours.
func (s *SSLManager) StartAutoRenewalWorker(ctx context.Context) {
	ticker := time.NewTicker(12 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.renewExpiringDomains(ctx)
			}
		}
	}()
}

func (s *SSLManager) renewExpiringDomains(ctx context.Context) {
	domains, err := s.repo.GetDomains()
	if err == nil {
		now := time.Now()
		for _, d := range domains {
			status := s.SyncDomainSSLStatus(&d)
			if !status.IsActive || !status.IsLetsEncrypt || d.SSLExpiry == nil {
				continue
			}

			// If expires in less than 30 days, trigger renewal
			if d.SSLExpiry.Sub(now) < 30*24*time.Hour {
				_ = s.ProvisionLetsEncrypt(ctx, &d)
			}
		}
	}

	// Also renew expiring panel host certificates
	if settings, err := s.repo.GetSettings(); err == nil {
		now := time.Now()
		for _, h := range settings.Hosts {
			status := s.GetPanelHostSSLStatus(h.Hostname)
			if status.IsActive && status.IsLetsEncrypt && status.ExpiresAt != nil {
				if status.ExpiresAt.Sub(now) < 30*24*time.Hour {
					_, _ = s.ProvisionPanelHostLetsEncrypt(ctx, h.Hostname)
				}
			}
		}
	}
}

// ProvisionPanelHostLetsEncrypt requests and installs a Let's Encrypt certificate for a panel hostname.
func (s *SSLManager) ProvisionPanelHostLetsEncrypt(ctx context.Context, hostname string) (*time.Time, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return nil, errors.New("nama domain/hostname panel tidak boleh kosong")
	}

	// Pre-flight check: verify DNS resolution
	ips, err := net.LookupHost(hostname)
	if err != nil || len(ips) == 0 {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(dialCtx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(dialCtx, "udp", "8.8.8.8:53")
			},
		}
		ips, err = r.LookupHost(ctx, hostname)
	}
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("domain panel '%s' belum dapat di-resolve oleh DNS publik. Pastikan DNS A record sudah terpasang dan mengarah ke IP publik server ini", hostname)
	}

	// Ensure challenge directory and proper permissions
	_ = os.MkdirAll(s.acmeRoot, 0755)
	_ = os.Chmod(s.acmeRoot, 0755)
	wellKnownDir := filepath.Join(s.acmeRoot, ".well-known", "acme-challenge")
	_ = os.MkdirAll(wellKnownDir, 0755)
	_ = os.Chmod(filepath.Join(s.acmeRoot, ".well-known"), 0755)
	_ = os.Chmod(wellKnownDir, 0755)
	if parent := filepath.Dir(s.acmeRoot); parent != "" && parent != "/" {
		_ = os.Chmod(parent, 0755)
	}

	// Ensure Nginx reverse proxy & ACME challenge configuration is synced
	_ = s.SyncPanelNginxReverseProxy(ctx)

	// Prepare directories for panel certs
	panelHostDir := filepath.Join(s.certBase, "panel", hostname)
	_ = os.MkdirAll(panelHostDir, 0755)
	_ = os.Chmod(panelHostDir, 0755)

	certPath := filepath.Join(panelHostDir, "fullchain.pem")
	keyPath := filepath.Join(panelHostDir, "privkey.pem")

	// Step 1: Try Certbot if available
	err = s.tryCertbot(ctx, hostname, certPath, keyPath)
	if err != nil {
		// Step 2: Fallback to native Go ACME client
		err = s.provisionNativeACME(ctx, hostname, certPath, keyPath)
	}

	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan sertifikat Let's Encrypt: %w", err)
	}

	// Inspect the newly generated certificate
	expiry, err := s.CheckCertExpiry(certPath)
	if err != nil {
		return nil, fmt.Errorf("sertifikat terbit tetapi gagal divalidasi: %w", err)
	}

	// Also mirror to main panel cert.pem & key.pem
	mainCert := filepath.Join(s.certBase, "panel", "cert.pem")
	mainKey := filepath.Join(s.certBase, "panel", "key.pem")
	certBytes, _ := os.ReadFile(certPath)
	keyBytes, _ := os.ReadFile(keyPath)
	if len(certBytes) > 0 && len(keyBytes) > 0 {
		_ = os.WriteFile(mainCert, certBytes, 0644)
		_ = os.WriteFile(mainKey, keyBytes, 0600)
	}

	// Update Nginx reverse proxy with the newly issued certificate
	_ = s.SyncPanelNginxReverseProxy(ctx)

	return expiry, nil
}

// GetPanelHostSSLStatus checks the certificate status on disk for a panel host.
func (s *SSLManager) GetPanelHostSSLStatus(hostname string) SSLStatusInfo {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	info := SSLStatusInfo{
		IsActive:   false,
		Provider:   "none",
		StatusText: "Belum Aktif (Self-Signed / Insecure)",
	}
	if hostname == "" {
		return info
	}

	candidates := []string{
		filepath.Join(s.certBase, "panel", hostname, "fullchain.pem"),
		filepath.Join(s.certBase, hostname, "fullchain.pem"),
		filepath.Join(s.certBase, "panel", "cert.pem"),
	}

	var foundCert string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			foundCert = p
			break
		}
	}

	if foundCert == "" {
		return info
	}

	data, err := os.ReadFile(foundCert)
	if err != nil {
		return info
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return info
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return info
	}

	info.ExpiresAt = &cert.NotAfter
	if len(cert.Issuer.Organization) > 0 {
		info.Issuer = cert.Issuer.Organization[0]
	} else {
		info.Issuer = cert.Issuer.CommonName
	}

	if time.Now().After(cert.NotAfter) {
		info.StatusText = fmt.Sprintf("Kadaluarsa (%s)", cert.NotAfter.Format("02 Jan 2006"))
		return info
	}

	isLE := strings.Contains(info.Issuer, "Let's Encrypt") ||
		strings.Contains(cert.Issuer.CommonName, "Let's Encrypt") ||
		info.Issuer == "R3" || info.Issuer == "R10" || info.Issuer == "R11" ||
		info.Issuer == "E1" || info.Issuer == "E5" || info.Issuer == "E6" ||
		strings.Contains(info.Issuer, "ISRG")

	isDummy := strings.Contains(info.Issuer, "Seintech Hosting Panel") ||
		(cert.Issuer.CommonName == "localhost" && len(cert.Issuer.Organization) == 0)

	if isDummy {
		info.IsActive = false
		info.IsLetsEncrypt = false
		info.Provider = "selfsigned"
		info.StatusText = "Belum Aktif (Self-Signed)"
		return info
	}

	if isLE {
		info.IsActive = true
		info.IsLetsEncrypt = true
		info.Provider = "letsencrypt"
		info.StatusText = "Active & Valid"
		return info
	}

	info.IsActive = true
	info.Provider = "custom"
	info.StatusText = "Active"
	return info
}

// GetHostCertPaths searches available certificate files for a given panel hostname.
func (s *SSLManager) GetHostCertPaths(hostname string) (certPath string, keyPath string, found bool) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	candidates := [][]string{
		{filepath.Join(s.certBase, "panel", hostname, "fullchain.pem"), filepath.Join(s.certBase, "panel", hostname, "privkey.pem")},
		{filepath.Join("/etc/letsencrypt/live", hostname, "fullchain.pem"), filepath.Join("/etc/letsencrypt/live", hostname, "privkey.pem")},
		{filepath.Join(s.certBase, hostname, "fullchain.pem"), filepath.Join(s.certBase, hostname, "privkey.pem")},
		{filepath.Join(s.certBase, "panel", "cert.pem"), filepath.Join(s.certBase, "panel", "key.pem")},
	}
	for _, pair := range candidates {
		if _, err1 := os.Stat(pair[0]); err1 == nil {
			if _, err2 := os.Stat(pair[1]); err2 == nil {
				return pair[0], pair[1], true
			}
		}
	}
	return "", "", false
}

// SyncPanelNginxReverseProxy configures Nginx port 80 & 443 reverse proxy for all panel domain hostnames.
func (s *SSLManager) SyncPanelNginxReverseProxy(ctx context.Context) error {
	if _, err := os.Stat("/etc/nginx/sites-available"); err != nil {
		return nil // Nginx not installed or not Debian/Ubuntu layout
	}

	settings, err := s.repo.GetSettings()
	if err != nil || len(settings.Hosts) == 0 {
		return nil
	}

	var allHostnames []string
	var httpsBlocks []string

	for _, host := range settings.Hosts {
		h := strings.ToLower(strings.TrimSpace(host.Hostname))
		if h == "" {
			continue
		}
		allHostnames = append(allHostnames, h)

		certPath, keyPath, found := s.GetHostCertPaths(h)
		if !found {
			// Fallback to default panel self-signed cert
			certPath = filepath.Join(s.certBase, "panel", "cert.pem")
			keyPath = filepath.Join(s.certBase, "panel", "key.pem")
		}

		backendPort := host.Port
		if backendPort == 0 {
			backendPort = 8443
		}

		block := fmt.Sprintf(`
# HTTPS Reverse Proxy (Port 443) -> Seintech Panel (:%d)
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    ssl_certificate %s;
    ssl_certificate_key %s;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # ACME Challenge Rule for automated Let's Encrypt validation
    location /.well-known/acme-challenge/ {
        root %s;
        try_files $uri =404;
    }

    # Reverse proxy to Seintech Panel
    location / {
        proxy_pass https://127.0.0.1:%d;
        proxy_ssl_verify off;
        proxy_ssl_server_name on;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
        proxy_connect_timeout 60s;
    }
}
`, backendPort, h, certPath, keyPath, s.acmeRoot, backendPort)
		httpsBlocks = append(httpsBlocks, block)
	}

	if len(allHostnames) == 0 {
		return nil
	}

	httpBlock := fmt.Sprintf(`# Auto-generated by Seintech Panel - Reverse Proxy Configuration
# Direct access at https://<domain>/ without specifying port 8443

# HTTP (Port 80) Redirect to HTTPS (Port 443) & ACME Challenge
server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location /.well-known/acme-challenge/ {
        root %s;
        try_files $uri =404;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}
`, strings.Join(allHostnames, " "), s.acmeRoot)

	fullConfig := httpBlock + strings.Join(httpsBlocks, "\n")

	nginxPanelConf := "/etc/nginx/sites-available/seintech-panel.conf"
	nginxPanelEnabled := "/etc/nginx/sites-enabled/seintech-panel.conf"

	_ = os.WriteFile(nginxPanelConf, []byte(fullConfig), 0644)
	if _, err := os.Lstat(nginxPanelEnabled); os.IsNotExist(err) {
		_ = os.Symlink(nginxPanelConf, nginxPanelEnabled)
	}

	// Test configuration before reloading
	if err := exec.CommandContext(ctx, "nginx", "-t").Run(); err != nil {
		return fmt.Errorf("konfigurasi nginx tidak valid: %w", err)
	}

	_ = exec.CommandContext(ctx, "nginx", "-s", "reload").Run()
	return nil
}


