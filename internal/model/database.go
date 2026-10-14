package model

import "time"

type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
)

type Database struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Type      DBType    `json:"type"`
	Username  string    `json:"username"`
	Host      string    `json:"host"`
	Charset   string    `json:"charset"`
	Collation string    `json:"collation,omitempty"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type DatabaseUser struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Type      DBType    `json:"type"`
	Host      string    `json:"host"` // localhost or %
	CreatedAt time.Time `json:"created_at"`
}
