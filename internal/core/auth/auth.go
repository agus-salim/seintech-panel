package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserSuspended      = errors.New("user account is suspended")
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrSessionExpired     = errors.New("session expired")
)

type Session struct {
	Token     string
	UserID    string
	Username  string
	Role      model.Role
	ExpiresAt time.Time
}

type AuthManager struct {
	repo     *storage.Repository
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewAuthManager(repo *storage.Repository) *AuthManager {
	return &AuthManager{
		repo:     repo,
		sessions: make(map[string]*Session),
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (a *AuthManager) Authenticate(username, password string, timeoutMinutes int) (*Session, error) {
	user, err := a.repo.GetUserByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.Status == "suspended" {
		return nil, ErrUserSuspended
	}

	if !CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	sess := &Session{
		Token:     token,
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		ExpiresAt: time.Now().Add(time.Duration(timeoutMinutes) * time.Minute),
	}

	a.mu.Lock()
	a.sessions[token] = sess
	a.mu.Unlock()

	return sess, nil
}

func (a *AuthManager) ValidateSession(token string) (*Session, error) {
	a.mu.RLock()
	sess, ok := a.sessions[token]
	a.mu.RUnlock()

	if !ok {
		return nil, ErrUnauthorized
	}

	if time.Now().After(sess.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return nil, ErrSessionExpired
	}

	return sess, nil
}

func (a *AuthManager) InvalidateSession(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}
