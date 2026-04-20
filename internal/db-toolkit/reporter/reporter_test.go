package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/analyzer"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/backup"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/connector"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/health"
	"github.com/stretchr/testify/assert"
)

func TestPrintHealthReport_Table(t *testing.T) {
	report := &health.Report{
		DBType:    connector.PostgreSQL,
		Host:      "localhost",
		Port:      5432,
		Database:  "testdb",
		Connected: true,
		Latency:   2 * time.Millisecond,
		Overall:   health.StatusOK,
		CheckedAt: time.Now(),
		Checks: []health.Check{
			{Name: "connection", Status: health.StatusOK, Message: "connected", Value: "2ms"},
			{Name: "connections", Status: health.StatusWarning, Message: "80/100 (80%)", Value: "80"},
		},
	}

	var buf bytes.Buffer
	PrintHealthReport(&buf, report, FormatTable)
	out := buf.String()

	assert.Contains(t, out, "Database Health Report")
	assert.Contains(t, out, "localhost")
	assert.Contains(t, out, "testdb")
	assert.Contains(t, out, "connection")
	assert.Contains(t, out, "[WARN]")
}

func TestPrintHealthReport_JSON(t *testing.T) {
	report := &health.Report{
		DBType:    connector.MySQL,
		Host:      "db.prod",
		Port:      3306,
		Database:  "prod",
		Connected: true,
		Overall:   health.StatusCritical,
		CheckedAt: time.Now(),
	}

	var buf bytes.Buffer
	PrintHealthReport(&buf, report, FormatJSON)
	out := buf.String()

	assert.True(t, strings.HasPrefix(strings.TrimSpace(out), "{"))
	assert.Contains(t, out, "CRITICAL")
}

func TestPrintAnalysisReport_Table(t *testing.T) {
	report := &analyzer.Report{
		DBType:     connector.PostgreSQL,
		Database:   "mydb",
		AnalyzedAt: time.Now(),
		TopTables: []analyzer.TableStat{
			{Schema: "public", Table: "users", Rows: 50000, TotalSize: "500 MB", IndexSize: "100 MB", TableSize: "400 MB"},
		},
		SlowQueries: []analyzer.SlowQuery{
			{Query: "SELECT * FROM big_table", Calls: 100, MeanTime: 500 * time.Millisecond},
		},
		UnusedIndexes: []analyzer.IndexStat{
			{Schema: "public", Table: "users", Index: "users_email_idx", Size: "10 MB", Unused: true},
		},
	}

	var buf bytes.Buffer
	PrintAnalysisReport(&buf, report, FormatTable)
	out := buf.String()

	assert.Contains(t, out, "Database Performance Report")
	assert.Contains(t, out, "Top Tables")
	assert.Contains(t, out, "users")
	assert.Contains(t, out, "Slow Queries")
	assert.Contains(t, out, "Unused Indexes")
}

func TestPrintBackupResult_Table(t *testing.T) {
	result := &backup.Result{
		FilePath:   "/tmp/postgres_mydb_20260420_120000.sql.gz",
		Size:       1024 * 1024 * 50, // 50 MB
		Duration:   15 * time.Second,
		Compressed: true,
	}

	var buf bytes.Buffer
	PrintBackupResult(&buf, result, FormatTable)
	out := buf.String()

	assert.Contains(t, out, "Backup Complete")
	assert.Contains(t, out, ".sql.gz")
	assert.Contains(t, out, "50.0 MB")
	assert.Contains(t, out, "15s")
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tt := range tests {
		got := humanSize(tt.bytes)
		assert.Equal(t, tt.want, got)
	}
}
