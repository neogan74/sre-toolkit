// Package analyzer provides database performance analysis.
package analyzer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/connector"
)

// TableStat holds size and row count for a table.
type TableStat struct {
	Schema    string
	Table     string
	Rows      int64
	TotalSize string
	IndexSize string
	TableSize string
}

// SlowQuery represents a slow or expensive query.
type SlowQuery struct {
	Query      string
	Calls      int64
	TotalTime  time.Duration
	MeanTime   time.Duration
	MinTime    time.Duration
	MaxTime    time.Duration
	StddevTime time.Duration
	Rows       int64
}

// IndexStat holds index usage information.
type IndexStat struct {
	Schema     string
	Table      string
	Index      string
	Scans      int64
	TuplesRead int64
	Size       string
	Unused     bool
}

// Report aggregates performance analysis results.
type Report struct {
	DBType        connector.DBType
	Database      string
	AnalyzedAt    time.Time
	TopTables     []TableStat
	SlowQueries   []SlowQuery
	UnusedIndexes []IndexStat
	TopIndexes    []IndexStat
}

// Config controls what to analyze.
type Config struct {
	TopN           int // number of top items to return
	MinQueryMs     int // minimum mean query time in ms for slow query list
	IncludeIndexes bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		TopN:           10,
		MinQueryMs:     100,
		IncludeIndexes: true,
	}
}

// Analyze connects and runs performance analysis.
func Analyze(ctx context.Context, cfg *connector.Config, opts *Config) (*Report, error) {
	if opts == nil {
		opts = DefaultConfig()
	}

	db, err := connector.Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	report := &Report{
		DBType:     cfg.Type,
		Database:   cfg.Database,
		AnalyzedAt: time.Now(),
	}

	switch cfg.Type {
	case connector.PostgreSQL:
		if err := analyzePostgres(ctx, db, opts, report); err != nil {
			return nil, err
		}
	case connector.MySQL:
		if err := analyzeMySQL(ctx, db, opts, report); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported db type: %s", cfg.Type)
	}

	return report, nil
}

func analyzePostgres(ctx context.Context, db *sql.DB, opts *Config, report *Report) error {
	// Top tables by size
	rows, err := db.QueryContext(ctx, `
		SELECT
			schemaname,
			tablename,
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS total_size,
			pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) AS index_size,
			pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) AS table_size,
			n_live_tup AS row_estimate
		FROM pg_stat_user_tables
		ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
		LIMIT $1`, opts.TopN)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t TableStat
			if err := rows.Scan(&t.Schema, &t.Table, &t.TotalSize, &t.IndexSize, &t.TableSize, &t.Rows); err == nil {
				report.TopTables = append(report.TopTables, t)
			}
		}
	}

	// Slow queries via pg_stat_statements (extension must be enabled)
	slowRows, err := db.QueryContext(ctx, `
		SELECT
			query,
			calls,
			total_exec_time::bigint,
			mean_exec_time::bigint,
			min_exec_time::bigint,
			max_exec_time::bigint,
			stddev_exec_time::bigint,
			rows
		FROM pg_stat_statements
		WHERE mean_exec_time > $1
		ORDER BY mean_exec_time DESC
		LIMIT $2`, opts.MinQueryMs, opts.TopN)
	if err == nil {
		defer slowRows.Close()
		for slowRows.Next() {
			var q SlowQuery
			var totalMs, meanMs, minMs, maxMs, stddevMs int64
			if err := slowRows.Scan(&q.Query, &q.Calls, &totalMs, &meanMs, &minMs, &maxMs, &stddevMs, &q.Rows); err == nil {
				q.TotalTime = time.Duration(totalMs) * time.Millisecond
				q.MeanTime = time.Duration(meanMs) * time.Millisecond
				q.MinTime = time.Duration(minMs) * time.Millisecond
				q.MaxTime = time.Duration(maxMs) * time.Millisecond
				q.StddevTime = time.Duration(stddevMs) * time.Millisecond
				report.SlowQueries = append(report.SlowQueries, q)
			}
		}
	}

	if !opts.IncludeIndexes {
		return nil
	}

	// Unused indexes
	idxRows, err := db.QueryContext(ctx, `
		SELECT
			schemaname,
			relname AS table,
			indexrelname AS index,
			idx_scan,
			idx_tup_read,
			pg_size_pretty(pg_relation_size(indexrelid)) AS size
		FROM pg_stat_user_indexes
		WHERE idx_scan = 0
		  AND schemaname NOT IN ('pg_catalog','pg_toast')
		ORDER BY pg_relation_size(indexrelid) DESC
		LIMIT $1`, opts.TopN)
	if err == nil {
		defer idxRows.Close()
		for idxRows.Next() {
			var idx IndexStat
			if err := idxRows.Scan(&idx.Schema, &idx.Table, &idx.Index, &idx.Scans, &idx.TuplesRead, &idx.Size); err == nil {
				idx.Unused = true
				report.UnusedIndexes = append(report.UnusedIndexes, idx)
			}
		}
	}

	// Top used indexes
	topIdxRows, err := db.QueryContext(ctx, `
		SELECT
			schemaname,
			relname AS table,
			indexrelname AS index,
			idx_scan,
			idx_tup_read,
			pg_size_pretty(pg_relation_size(indexrelid)) AS size
		FROM pg_stat_user_indexes
		WHERE idx_scan > 0
		ORDER BY idx_scan DESC
		LIMIT $1`, opts.TopN)
	if err == nil {
		defer topIdxRows.Close()
		for topIdxRows.Next() {
			var idx IndexStat
			if err := topIdxRows.Scan(&idx.Schema, &idx.Table, &idx.Index, &idx.Scans, &idx.TuplesRead, &idx.Size); err == nil {
				report.TopIndexes = append(report.TopIndexes, idx)
			}
		}
	}

	return nil
}

func analyzeMySQL(ctx context.Context, db *sql.DB, opts *Config, report *Report) error {
	// Top tables by size
	rows, err := db.QueryContext(ctx, `
		SELECT
			table_schema,
			table_name,
			IFNULL(table_rows, 0),
			IFNULL(CONCAT(ROUND((data_length+index_length)/1024/1024, 2),' MB'), '0 MB') AS total_size,
			IFNULL(CONCAT(ROUND(index_length/1024/1024, 2),' MB'), '0 MB') AS index_size,
			IFNULL(CONCAT(ROUND(data_length/1024/1024, 2),' MB'), '0 MB') AS data_size
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		ORDER BY data_length+index_length DESC
		LIMIT ?`, opts.TopN)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t TableStat
			if err := rows.Scan(&t.Schema, &t.Table, &t.Rows, &t.TotalSize, &t.IndexSize, &t.TableSize); err == nil {
				report.TopTables = append(report.TopTables, t)
			}
		}
	}

	// Slow queries from performance_schema
	slowRows, err := db.QueryContext(ctx, `
		SELECT
			DIGEST_TEXT,
			COUNT_STAR,
			SUM_TIMER_WAIT/1000000000,
			AVG_TIMER_WAIT/1000000000,
			MIN_TIMER_WAIT/1000000000,
			MAX_TIMER_WAIT/1000000000,
			SUM_ROWS_EXAMINED
		FROM performance_schema.events_statements_summary_by_digest
		WHERE AVG_TIMER_WAIT/1000000 > ?
		ORDER BY AVG_TIMER_WAIT DESC
		LIMIT ?`, opts.MinQueryMs, opts.TopN)
	if err == nil {
		defer slowRows.Close()
		for slowRows.Next() {
			var q SlowQuery
			var totalMs, meanMs, minMs, maxMs int64
			var queryText sql.NullString
			if err := slowRows.Scan(&queryText, &q.Calls, &totalMs, &meanMs, &minMs, &maxMs, &q.Rows); err == nil {
				q.Query = queryText.String
				q.TotalTime = time.Duration(totalMs) * time.Millisecond
				q.MeanTime = time.Duration(meanMs) * time.Millisecond
				q.MinTime = time.Duration(minMs) * time.Millisecond
				q.MaxTime = time.Duration(maxMs) * time.Millisecond
				report.SlowQueries = append(report.SlowQueries, q)
			}
		}
	}

	return nil
}
