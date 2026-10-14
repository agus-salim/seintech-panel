package storage

import (
	"errors"
	"path/filepath"
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

func (r *Repository) GetDomainByName(name string) (*model.Domain, error) {
	all, err := r.GetDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range all {
		if d.DomainName == name {
			return &d, nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repository) UpsertDomain(domain model.Domain) error {
	domains, _ := r.GetDomains()
	found := false
	for i, d := range domains {
		if d.ID == domain.ID {
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
	for _, d := range domains {
		if d.ID != id {
			updated = append(updated, d)
		}
	}
	return r.SaveDomains(updated)
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

// Settings methods
func (r *Repository) GetSettings() (*model.PanelSettings, error) {
	var settings model.PanelSettings
	err := r.settingsStore.Read(&settings)
	if err != nil {
		// default settings
		return &model.PanelSettings{
			Hostname:               "panel.local",
			Port:                   8443,
			ListenAddr:             "0.0.0.0",
			DefaultWebServer:       "nginx",
			DefaultPHPVersion:      "8.3",
			SessionTimeoutMinutes:  120,
			SecurityHeadersEnabled: true,
			AutoRenewSSL:           true,
		}, nil
	}
	return &settings, nil
}

func (r *Repository) SaveSettings(settings model.PanelSettings) error {
	return r.settingsStore.Write(settings)
}
