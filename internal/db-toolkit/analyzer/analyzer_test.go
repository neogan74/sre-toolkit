package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 10, cfg.TopN)
	assert.Equal(t, 100, cfg.MinQueryMs)
	assert.True(t, cfg.IncludeIndexes)
}

func TestSlowQueryFields(t *testing.T) {
	q := SlowQuery{
		Query: "SELECT * FROM users WHERE id = $1",
		Calls: 1000,
		Rows:  500,
	}
	assert.Equal(t, int64(1000), q.Calls)
	assert.Equal(t, int64(500), q.Rows)
	assert.Contains(t, q.Query, "SELECT")
}

func TestTableStatFields(t *testing.T) {
	ts := TableStat{
		Schema:    "public",
		Table:     "orders",
		Rows:      1_000_000,
		TotalSize: "1.2 GB",
		IndexSize: "200 MB",
		TableSize: "1.0 GB",
	}
	assert.Equal(t, "public", ts.Schema)
	assert.Equal(t, int64(1_000_000), ts.Rows)
	assert.Equal(t, "1.2 GB", ts.TotalSize)
}

func TestIndexStatFields(t *testing.T) {
	idx := IndexStat{
		Schema: "public",
		Table:  "orders",
		Index:  "orders_pkey",
		Scans:  0,
		Size:   "8 MB",
		Unused: true,
	}
	assert.True(t, idx.Unused)
	assert.Equal(t, int64(0), idx.Scans)
}
