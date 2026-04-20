// Package health provides database health checks.
package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/connector"
)

// Status represents overall health status.
type Status string

const (
	StatusOK       Status = "OK"
	StatusWarning  Status = "WARNING"
	StatusCritical Status = "CRITICAL"
)

// Check holds the result of a single health check.
type Check struct {
	Name    string
	Status  Status
	Message string
	Value   string
}

// Report aggregates all health checks for a database.
type Report struct {
	DBType    connector.DBType
	Host      string
	Port      int
	Database  string
	Connected bool
	Latency   time.Duration
	Checks    []Check
	Overall   Status
	CheckedAt time.Time
}

// Run connects to the database and performs health checks.
func Run(ctx context.Context, cfg *connector.Config) (*Report, error) {
	report := &Report{
		DBType:    cfg.Type,
		Host:      cfg.Host,
		Port:      cfg.Port,
		Database:  cfg.Database,
		CheckedAt: time.Now(),
	}

	start := time.Now()
	db, err := connector.Connect(cfg)
	if err != nil {
		report.Connected = false
		report.Overall = StatusCritical
		report.Checks = append(report.Checks, Check{
			Name:    "connection",
			Status:  StatusCritical,
			Message: err.Error(),
		})
		return report, nil
	}
	defer db.Close()

	report.Latency = time.Since(start)
	report.Connected = true
	report.Checks = append(report.Checks, Check{
		Name:    "connection",
		Status:  StatusOK,
		Message: "connected",
		Value:   report.Latency.String(),
	})

	switch cfg.Type {
	case connector.PostgreSQL:
		checks := postgresChecks(ctx, db)
		report.Checks = append(report.Checks, checks...)
	case connector.MySQL:
		checks := mysqlChecks(ctx, db)
		report.Checks = append(report.Checks, checks...)
	}

	report.Overall = worstStatus(report.Checks)
	return report, nil
}

// postgresChecks runs PostgreSQL-specific health checks.
func postgresChecks(ctx context.Context, db *sql.DB) []Check {
	var checks []Check

	// Version
	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err == nil {
		checks = append(checks, Check{Name: "version", Status: StatusOK, Value: version})
	}

	// Active connections
	var active, max int
	err := db.QueryRowContext(ctx,
		"SELECT count(*), (SELECT setting::int FROM pg_settings WHERE name='max_connections') FROM pg_stat_activity WHERE state='active'",
	).Scan(&active, &max)
	if err == nil {
		pct := float64(active) / float64(max) * 100
		s := StatusOK
		msg := fmt.Sprintf("%d/%d (%.0f%%)", active, max, pct)
		if pct > 90 {
			s = StatusCritical
		} else if pct > 75 {
			s = StatusWarning
		}
		checks = append(checks, Check{Name: "connections", Status: s, Message: msg, Value: fmt.Sprintf("%d", active)})
	}

	// Database size
	var dbSize string
	err = db.QueryRowContext(ctx,
		"SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize)
	if err == nil {
		checks = append(checks, Check{Name: "db_size", Status: StatusOK, Value: dbSize})
	}

	// Replication lag (if replica)
	var isReplica bool
	_ = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isReplica)
	if isReplica {
		var lagSeconds sql.NullFloat64
		_ = db.QueryRowContext(ctx,
			"SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))").Scan(&lagSeconds)
		if lagSeconds.Valid {
			s := StatusOK
			msg := fmt.Sprintf("%.1fs", lagSeconds.Float64)
			if lagSeconds.Float64 > 60 {
				s = StatusCritical
			} else if lagSeconds.Float64 > 10 {
				s = StatusWarning
			}
			checks = append(checks, Check{Name: "replication_lag", Status: s, Message: msg, Value: msg})
		}
	}

	// Long-running queries (> 5 min)
	var longQueries int
	_ = db.QueryRowContext(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE state='active' AND now()-query_start > interval '5 minutes'",
	).Scan(&longQueries)
	s := StatusOK
	msg := fmt.Sprintf("%d queries running > 5min", longQueries)
	if longQueries > 5 {
		s = StatusCritical
	} else if longQueries > 0 {
		s = StatusWarning
	}
	checks = append(checks, Check{Name: "long_queries", Status: s, Message: msg, Value: fmt.Sprintf("%d", longQueries)})

	return checks
}

// mysqlChecks runs MySQL-specific health checks.
func mysqlChecks(ctx context.Context, db *sql.DB) []Check {
	var checks []Check

	// Version
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err == nil {
		checks = append(checks, Check{Name: "version", Status: StatusOK, Value: version})
	}

	// Active connections
	rows, err := db.QueryContext(ctx, "SHOW STATUS WHERE Variable_name IN ('Threads_connected','Max_used_connections')")
	if err == nil {
		defer rows.Close()
		vars := map[string]string{}
		for rows.Next() {
			var k, v string
			if rows.Scan(&k, &v) == nil {
				vars[k] = v
			}
		}
		if conn, ok := vars["Threads_connected"]; ok {
			s := StatusOK
			checks = append(checks, Check{Name: "connections", Status: s, Value: conn, Message: "threads connected"})
		}
	}

	// Uptime
	var uptime int64
	if err := db.QueryRowContext(ctx, "SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME='Uptime'").Scan(&uptime); err == nil {
		checks = append(checks, Check{Name: "uptime", Status: StatusOK, Value: fmt.Sprintf("%ds", uptime)})
	}

	// Slow queries
	var slowQueries int64
	if err := db.QueryRowContext(ctx, "SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME='Slow_queries'").Scan(&slowQueries); err == nil {
		s := StatusOK
		if slowQueries > 100 {
			s = StatusWarning
		}
		checks = append(checks, Check{Name: "slow_queries", Status: s, Value: fmt.Sprintf("%d", slowQueries)})
	}

	return checks
}

func worstStatus(checks []Check) Status {
	worst := StatusOK
	for _, c := range checks {
		if c.Status == StatusCritical {
			return StatusCritical
		}
		if c.Status == StatusWarning {
			worst = StatusWarning
		}
	}
	return worst
}
