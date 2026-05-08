// Package connector provides database connection management for PostgreSQL and MySQL.
package connector

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/neogan/sre-toolkit/pkg/logging"
)

// DBType represents the type of database.
type DBType string

const (
	PostgreSQL DBType = "postgres"
	MySQL      DBType = "mysql"
)

// Config holds connection parameters.
type Config struct {
	Type     DBType
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string // postgres only: disable, require, verify-full

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:            5432,
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// Connect opens and verifies a database connection.
func Connect(cfg *Config) (*sql.DB, error) {
	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}

	driver := string(cfg.Type)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			// Log but don't override the original error
			logger := logging.GetLogger()
			logger.Warn().Err(closeErr).Msg("Failed to close database connection")
		}
		return nil, fmt.Errorf("ping %s at %s:%d: %w", driver, cfg.Host, cfg.Port, err)
	}

	return db, nil
}

func buildDSN(cfg *Config) (string, error) {
	switch cfg.Type {
	case PostgreSQL:
		sslmode := cfg.SSLMode
		if sslmode == "" {
			sslmode = "disable"
		}
		return fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslmode,
		), nil
	case MySQL:
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=10s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
		), nil
	default:
		return "", fmt.Errorf("unsupported db type: %q (use postgres or mysql)", cfg.Type)
	}
}
