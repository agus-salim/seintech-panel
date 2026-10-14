package database

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
)

type DatabaseManager interface {
	Type() model.DBType
	CreateDatabase(ctx context.Context, name, charset, collation string) error
	DropDatabase(ctx context.Context, name string) error
	CreateUser(ctx context.Context, username, password, host string) error
	DropUser(ctx context.Context, username, host string) error
	GrantPrivileges(ctx context.Context, username, host, dbName string) error
	ChangePassword(ctx context.Context, username, host, newPassword string) error
	GetDatabaseSize(ctx context.Context, dbName string) (int64, error)
	Status(ctx context.Context) (system.ServiceState, error)
}

// MySQLManager handles MariaDB and MySQL provisioning
type MySQLManager struct {
	sm *system.ServiceManager
}

func NewMySQLManager(sm *system.ServiceManager) *MySQLManager {
	return &MySQLManager{sm: sm}
}

func (m *MySQLManager) Type() model.DBType {
	return model.DBTypeMySQL
}

func (m *MySQLManager) runSQL(ctx context.Context, query string) (string, error) {
	cmd := exec.CommandContext(ctx, "mysql", "-u", "root", "-e", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("mysql query failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m *MySQLManager) CreateDatabase(ctx context.Context, name, charset, collation string) error {
	if charset == "" {
		charset = "utf8mb4"
	}
	if collation == "" {
		collation = "utf8mb4_unicode_ci"
	}
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s;", name, charset, collation)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) DropDatabase(ctx context.Context, name string) error {
	q := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", name)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) CreateUser(ctx context.Context, username, password, host string) error {
	if host == "" {
		host = "localhost"
	}
	q := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';", username, host, password)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) DropUser(ctx context.Context, username, host string) error {
	if host == "" {
		host = "localhost"
	}
	q := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s';", username, host)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) GrantPrivileges(ctx context.Context, username, host, dbName string) error {
	if host == "" {
		host = "localhost"
	}
	q := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s'; FLUSH PRIVILEGES;", dbName, username, host)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) ChangePassword(ctx context.Context, username, host, newPassword string) error {
	if host == "" {
		host = "localhost"
	}
	q := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;", username, host, newPassword)
	_, err := m.runSQL(ctx, q)
	return err
}

func (m *MySQLManager) GetDatabaseSize(ctx context.Context, dbName string) (int64, error) {
	q := fmt.Sprintf("SELECT SUM(data_length + index_length) FROM information_schema.TABLES WHERE table_schema = '%s';", dbName)
	out, err := m.runSQL(ctx, q)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) >= 2 {
		val, _ := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
		return val, nil
	}
	return 0, nil
}

func (m *MySQLManager) Status(ctx context.Context) (system.ServiceState, error) {
	// Check mariadb first, then mysql
	state, err := m.sm.GetStatus(ctx, "mariadb")
	if err == nil && state.Active {
		return state, nil
	}
	return m.sm.GetStatus(ctx, "mysql")
}

// PostgresManager handles PostgreSQL provisioning
type PostgresManager struct {
	sm *system.ServiceManager
}

func NewPostgresManager(sm *system.ServiceManager) *PostgresManager {
	return &PostgresManager{sm: sm}
}

func (p *PostgresManager) Type() model.DBType {
	return model.DBTypePostgres
}

func (p *PostgresManager) runSQL(ctx context.Context, query string) (string, error) {
	cmd := exec.CommandContext(ctx, "su", "-", "postgres", "-c", fmt.Sprintf("psql -t -c %q", query))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("postgres query failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (p *PostgresManager) CreateDatabase(ctx context.Context, name, charset, collation string) error {
	if charset == "" {
		charset = "UTF8"
	}
	q := fmt.Sprintf("CREATE DATABASE \"%s\" ENCODING '%s';", name, charset)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) DropDatabase(ctx context.Context, name string) error {
	q := fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\";", name)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) CreateUser(ctx context.Context, username, password, host string) error {
	q := fmt.Sprintf("CREATE USER \"%s\" WITH ENCRYPTED PASSWORD '%s';", username, password)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) DropUser(ctx context.Context, username, host string) error {
	q := fmt.Sprintf("DROP USER IF EXISTS \"%s\";", username)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) GrantPrivileges(ctx context.Context, username, host, dbName string) error {
	q := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\";", dbName, username)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) ChangePassword(ctx context.Context, username, host, newPassword string) error {
	q := fmt.Sprintf("ALTER USER \"%s\" WITH ENCRYPTED PASSWORD '%s';", username, newPassword)
	_, err := p.runSQL(ctx, q)
	return err
}

func (p *PostgresManager) GetDatabaseSize(ctx context.Context, dbName string) (int64, error) {
	q := fmt.Sprintf("SELECT pg_database_size('%s');", dbName)
	out, err := p.runSQL(ctx, q)
	if err != nil {
		return 0, err
	}
	val, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return val, nil
}

func (p *PostgresManager) Status(ctx context.Context) (system.ServiceState, error) {
	return p.sm.GetStatus(ctx, "postgresql")
}
