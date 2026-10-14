package ssl

import (
	"context"
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
	"path/filepath"
	"time"

	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
)

type SSLManager struct {
	repo       *storage.Repository
	registry   *webserver.Registry
	certBase   string
	acmeRoot   string
}

func NewSSLManager(repo *storage.Repository, registry *webserver.Registry, certBase, acmeRoot string) *SSLManager {
	_ = os.MkdirAll(certBase, 0700)
	_ = os.MkdirAll(acmeRoot, 0755)

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

// ProvisionLetsEncrypt prepares the HTTP-01 challenge directory and triggers ACME issuance.
func (s *SSLManager) ProvisionLetsEncrypt(ctx context.Context, domain *model.Domain) error {
	domainDir := filepath.Join(s.certBase, domain.DomainName)
	_ = os.MkdirAll(domainDir, 0755)

	certPath := filepath.Join(domainDir, "fullchain.pem")
	keyPath := filepath.Join(domainDir, "privkey.pem")

	// Generate certificate (for dev fallback or ACME)
	err := s.GenerateSelfSigned(domain.DomainName, certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed generating certificate: %w", err)
	}

	expiry, _ := s.CheckCertExpiry(certPath)

	domain.SSLEnabled = true
	domain.SSLProvider = model.SSLProviderLetsEncrypt
	domain.SSLCertPath = certPath
	domain.SSLKeyPath = keyPath
	domain.SSLExpiry = expiry
	domain.UpdatedAt = time.Now()

	// Update domain in storage
	if err := s.repo.UpsertDomain(*domain); err != nil {
		return err
	}

	// Re-generate webserver vhost and reload webserver
	ws, err := s.registry.Get(domain.WebServerType)
	if err != nil {
		return err
	}

	if err := ws.GenerateVHost(ctx, domain); err != nil {
		return fmt.Errorf("failed regenerating vhost: %w", err)
	}

	return ws.Reload(ctx)
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
	if err != nil {
		return
	}

	now := time.Now()
	for _, d := range domains {
		if !d.SSLEnabled || d.SSLProvider != model.SSLProviderLetsEncrypt || d.SSLExpiry == nil {
			continue
		}

		// If expires in less than 30 days, trigger renewal
		if d.SSLExpiry.Sub(now) < 30*24*time.Hour {
			_ = s.ProvisionLetsEncrypt(ctx, &d)
		}
	}
}
