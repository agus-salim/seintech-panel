package model

import "time"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleReseller Role = "reseller"
	RoleUser     Role = "user"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	ResellerID   string    `json:"reseller_id,omitempty"` // ID of reseller who owns this user
	HomeDir      string    `json:"home_dir"`
	Status       string    `json:"status"` // active, suspended
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
