package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/seintech/panel/internal/model"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource already exists")
)

type Repository struct {
	userStore     *JSONStore
	domainStore   *JSONStore
	quotaStore    *JSONStore
	databaseStore *JSONStore
	settingsStore *JSONStore
	mu            sync.RWMutex
}

func NewRepository(dataDir string) (*Repository, error) {
	uStore, err := NewJSONStore(filepath.Join(dataDir, "users.json"))
	if err != nil {
		return nil, err
	}
	dStore, err := NewJSONStore(filepath.Join(dataDir, "domains.json"))
	if err != nil {
		return nil, err
	}
	qStore, err := NewJSONStore(filepath.Join(dataDir, "quotas.json"))
	if err != nil {
		return nil, err
	}
	dbStore, err := NewJSONStore(filepath.Join(dataDir, "databases.json"))
	if err != nil {
		return nil, err
	}
	sStore, err := NewJSONStore(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		return nil, err
	}

	return &Repository{
		userStore:     uStore,
		domainStore:   dStore,
		quotaStore:    qStore,
		databaseStore: dbStore,
		settingsStore: sStore,
	}, nil
}

// User methods
func (r *Repository) GetUsers() ([]model.User, error) {
	var users []model.User
	err := r.userStore.Read(&users)
	if errors.Is(err, ErrNotFound) || err != nil && err.Error() == "file does not exist" {
		return []model.User{}, nil
	}
	return users, err
}

func (r *Repository) SaveUsers(users []model.User) error {
	return r.userStore.Write(users)
}

func (r *Repository) GetUserByUsername(username string) (*model.User, error) {
	users, err := r.GetUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repository) GetUserByID(id string) (*model.User, error) {
	users, err := r.GetUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repository) UpsertUser(user model.User) error {
	users, err := r.GetUsers()
	if err != nil {
		users = []model.User{}
	}

	found := false
	for i, u := range users {
		if u.ID == user.ID {
			users[i] = user
			found = true
			break
		}
	}
	if !found {
		users = append(users, user)
	}
	return r.SaveUsers(users)
}

func (r *Repository) DeleteUser(id string) error {
	users, _ := r.GetUsers()
	var updated []model.User
	cleanID := strings.TrimSpace(id)
	for _, u := range users {
		if u.ID != cleanID && u.Username != cleanID {
			updated = append(updated, u)
		}
	}
	_ = r.DeleteQuota(cleanID)
	return r.SaveUsers(updated)
}

// Domain methods
func (r *Repository) GetDomains() ([]model.Domain, error) {
	var domains []model.Domain
	err := r.domainStore.Read(&domains)
	if err != nil {
		return []model.Domain{}, nil
	}
	return domains, nil
}

func (r *Repository) SaveDomains(domains []model.Domain) error {
	return r.domainStore.Write(domains)
}

func (r *Repository) GetDomainsByUser(userID string) ([]model.Domain, error) {
	all, err := r.GetDomains()
	if err != nil {
		return nil, err
	}
	var res []model.Domain
	for _, d := range all {
		if d.UserID == userID {
			res = append(res, d)
		}
	}
	return res, nil
}

func (r *Repository) GetDomainByID(id string) (*model.Domain, error) {
	all, err := r.GetDomains()
	if err != nil {
		return nil, err
	}
	cleanID := strings.TrimSpace(id)
	for _, d := range all {
		if d.ID == cleanID {
			return &d, nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repository) GetDomainByName(name string) (*model.Domain, error) {
	all, err := r.GetDomains()
	if err != nil {
		return nil, err
	}
	cleanName := strings.TrimSpace(name)
	for _, d := range all {
		if strings.EqualFold(strings.TrimSpace(d.DomainName), cleanName) {
			return &d, nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repository) UpsertDomain(domain model.Domain) error {
	domains, _ := r.GetDomains()
	found := false
	domain.DomainName = strings.TrimSpace(domain.DomainName)
	domain.ID = strings.TrimSpace(domain.ID)

	for i, d := range domains {
		if (domain.ID != "" && d.ID == domain.ID) ||
			(domain.DomainName != "" && strings.EqualFold(strings.TrimSpace(d.DomainName), domain.DomainName)) {
			if domain.ID == "" && d.ID != "" {
				domain.ID = d.ID
			}
			domains[i] = domain
			found = true
			break
		}
	}
	if !found {
		domains = append(domains, domain)
	}
	return r.SaveDomains(domains)
}

func (r *Repository) DeleteDomain(id string) error {
	domains, _ := r.GetDomains()
	var updated []model.Domain
	cleanID := strings.TrimSpace(id)
	for _, d := range domains {
		if d.ID != cleanID && !strings.EqualFold(strings.TrimSpace(d.DomainName), cleanID) {
			updated = append(updated, d)
		}
	}
	return r.SaveDomains(updated)
}

// Database methods
func (r *Repository) GetDatabases() ([]model.Database, error) {
	var dbs []model.Database
	err := r.databaseStore.Read(&dbs)
	if errors.Is(err, ErrNotFound) || (err != nil && err.Error() == "file does not exist") {
		return []model.Database{}, nil
	}
	return dbs, err
}

func (r *Repository) SaveDatabases(dbs []model.Database) error {
	return r.databaseStore.Write(dbs)
}

func (r *Repository) UpsertDatabase(db model.Database) error {
	dbs, _ := r.GetDatabases()
	found := false
	db.Name = strings.TrimSpace(db.Name)
	for i, d := range dbs {
		if strings.EqualFold(d.Name, db.Name) {
			dbs[i] = db
			found = true
			break
		}
	}
	if !found {
		dbs = append(dbs, db)
	}
	return r.SaveDatabases(dbs)
}

func (r *Repository) DeleteDatabase(name string) error {
	dbs, _ := r.GetDatabases()
	var updated []model.Database
	cleanName := strings.TrimSpace(name)
	for _, d := range dbs {
		if !strings.EqualFold(d.Name, cleanName) {
			updated = append(updated, d)
		}
	}
	return r.SaveDatabases(updated)
}

// Quota methods
func (r *Repository) GetQuota(userID string) (*model.Quota, error) {
	var quotas map[string]model.Quota
	err := r.quotaStore.Read(&quotas)
	if err != nil || quotas == nil {
		return &model.Quota{UserID: userID}, nil
	}
	q, ok := quotas[userID]
	if !ok {
		return &model.Quota{UserID: userID}, nil
	}
	return &q, nil
}

func (r *Repository) SaveQuota(quota model.Quota) error {
	var quotas map[string]model.Quota
	_ = r.quotaStore.Read(&quotas)
	if quotas == nil {
		quotas = make(map[string]model.Quota)
	}
	quotas[quota.UserID] = quota
	return r.quotaStore.Write(quotas)
}

func (r *Repository) GetAllQuotas() (map[string]model.Quota, error) {
	var quotas map[string]model.Quota
	err := r.quotaStore.Read(&quotas)
	if err != nil || quotas == nil {
		return make(map[string]model.Quota), nil
	}
	return quotas, nil
}

func (r *Repository) DeleteQuota(userID string) error {
	var quotas map[string]model.Quota
	_ = r.quotaStore.Read(&quotas)
	if quotas != nil {
		delete(quotas, userID)
		return r.quotaStore.Write(quotas)
	}
	return nil
}

// Settings methods
func (r *Repository) GetSettings() (*model.PanelSettings, error) {
	var settings model.PanelSettings
	err := r.settingsStore.Read(&settings)
	sysHost, _ := os.Hostname()
	if sysHost == "" {
		sysHost = "localhost"
	}
	if err != nil {
		// default settings
		settings = model.PanelSettings{
			Hostname:               sysHost,
			Port:                   8443,
			ListenAddr:             "0.0.0.0",
			DefaultWebServer:       "nginx",
			DefaultPHPVersion:      "8.3",
			SessionTimeoutMinutes:  120,
			SecurityHeadersEnabled: true,
			AutoRenewSSL:           true,
			Hosts:                  []model.PanelHost{},
			Language:               "en",
		}
	}
	if settings.Hostname == "" {
		settings.Hostname = sysHost
	}
	if settings.Port == 0 {
		settings.Port = 8443
	}
	if settings.Language == "" {
		settings.Language = "en"
	}

	var cleanHosts []model.PanelHost
	for _, h := range settings.Hosts {
		if strings.TrimSpace(h.Hostname) != "" {
			cleanHosts = append(cleanHosts, h)
		}
	}
	settings.Hosts = cleanHosts

	return &settings, nil
}

func (r *Repository) SaveSettings(settings model.PanelSettings) error {
	return r.settingsStore.Write(settings)
}
