package connector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDSN_Postgres(t *testing.T) {
	cfg := &Config{
		Type:     PostgreSQL,
		Host:     "localhost",
		Port:     5432,
		User:     "admin",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "disable",
	}
	dsn, err := buildDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=admin")
	assert.Contains(t, dsn, "dbname=mydb")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestBuildDSN_PostgresDefaultSSL(t *testing.T) {
	cfg := &Config{
		Type:     PostgreSQL,
		Host:     "db.example.com",
		Port:     5432,
		User:     "user",
		Password: "pass",
		Database: "testdb",
		SSLMode:  "",
	}
	dsn, err := buildDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestBuildDSN_MySQL(t *testing.T) {
	cfg := &Config{
		Type:     MySQL,
		Host:     "mysql.local",
		Port:     3306,
		User:     "root",
		Password: "rootpass",
		Database: "shop",
	}
	dsn, err := buildDSN(cfg)
	require.NoError(t, err)
	assert.Contains(t, dsn, "root:rootpass@tcp(mysql.local:3306)/shop")
	assert.Contains(t, dsn, "parseTime=true")
}

func TestBuildDSN_UnsupportedType(t *testing.T) {
	cfg := &Config{
		Type: "mssql",
	}
	_, err := buildDSN(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported db type")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "disable", cfg.SSLMode)
	assert.Equal(t, 10, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
}
