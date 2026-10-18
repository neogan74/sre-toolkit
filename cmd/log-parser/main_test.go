package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
