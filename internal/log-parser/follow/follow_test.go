package follow

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type collector struct {
	mu    sync.Mutex
	lines []string
}

func (c *collector) add(l string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, l)
}

func (c *collector) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

func start(t *testing.T, path string, offset int64) (*collector, func()) {
	t.Helper()
	c := &collector{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Lines(ctx, path, offset, 10*time.Millisecond, c.add) }()
	return c, func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Lines did not stop after cancel")
		}
	}
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(s)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestLines_AppendedLinesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	appendTo(t, path, "old\n")
	c, stop := start(t, path, 4)
	defer stop()

	appendTo(t, path, "one\r\ntwo\nthr")
	require.Eventually(t, func() bool { return len(c.get()) == 2 }, 2*time.Second, 10*time.Millisecond)
	appendTo(t, path, "ee\n")
	require.Eventually(t, func() bool { return len(c.get()) == 3 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"one", "two", "three"}, c.get())
}

func TestLines_Truncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	appendTo(t, path, "aaaaaaaaaa\n")
	c, stop := start(t, path, 11)
	defer stop()

	require.NoError(t, os.WriteFile(path, []byte("new\n"), 0o600))
	require.Eventually(t, func() bool { return len(c.get()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"new"}, c.get())
}

func TestLines_Rotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	appendTo(t, path, "a\n")
	c, stop := start(t, path, 2)
	defer stop()

	appendTo(t, path, "b\n")
	require.Eventually(t, func() bool { return len(c.get()) == 1 }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, os.Rename(path, path+".1"))
	time.Sleep(50 * time.Millisecond) // file missing mid-rotation
	appendTo(t, path, "c\nd\n")
	require.Eventually(t, func() bool { return len(c.get()) == 3 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"b", "c", "d"}, c.get())
}

func TestLines_MissingFile(t *testing.T) {
	err := Lines(context.Background(), filepath.Join(t.TempDir(), "nope.log"), 0, 0, func(string) {})
	assert.Error(t, err)
}
