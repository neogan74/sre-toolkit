package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailCmd_FollowNeedsOneFile(t *testing.T) {
	cmd := newTailCmd()
	cmd.SetArgs([]string{"-f"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	assert.ErrorContains(t, cmd.Execute(), "exactly one file")
}

func TestCountingReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.log")
	require.NoError(t, os.WriteFile(path, []byte("hello\nworld\n"), 0o600))
	r, closer, err := openInput([]string{path})
	require.NoError(t, err)
	defer closer()
	cr := &countingReader{r: r}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(cr)
	require.NoError(t, err)
	assert.Equal(t, int64(12), cr.n)
}

func TestTraceCmd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.log")
	require.NoError(t, os.WriteFile(path, []byte(
		`{"level":"info","msg":"one","trace_id":"abc"}
{"level":"info","msg":"other","trace_id":"zzz"}
{"level":"error","msg":"two","request_id":"abc"}
`), 0o600))

	// printEntry writes to os.Stdout, so capture it.
	old := os.Stdout
	rd, wr, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = wr
	cmd := newTraceCmd()
	cmd.SetArgs([]string{"abc", path})
	cmd.SetErr(&bytes.Buffer{})
	execErr := cmd.Execute()
	require.NoError(t, wr.Close())
	os.Stdout = old
	require.NoError(t, execErr)

	var out bytes.Buffer
	_, err = out.ReadFrom(rd)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "one")
	assert.Contains(t, out.String(), "two")
	assert.NotContains(t, out.String(), "other")
}

func TestBelongsToTrace(t *testing.T) {
	with := &formats.Entry{Raw: "x abc", Fields: map[string]string{"trace_id": "zzz"}}
	assert.False(t, belongsToTrace(with, "abc", nil), "ID field wins over raw text")
	plain := &formats.Entry{Raw: "request abc failed"}
	assert.True(t, belongsToTrace(plain, "abc", nil), "no field: raw match")
	assert.False(t, belongsToTrace(plain, "nope", nil))
}
