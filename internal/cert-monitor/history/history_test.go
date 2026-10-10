package history

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func cert(fp string, notAfter time.Time) *scanner.CertInfo {
	return &scanner.CertInfo{Host: "a.example", Port: "443", Serial: "s-" + fp, Fingerprint: fp, NotAfter: notAfter, Issuer: "CA"}
}

func open(t *testing.T) (*Store, string) {
	p := filepath.Join(t.TempDir(), "sub", "h.json")
	s, err := Open(p)
	require.NoError(t, err)
	return s, p
}

func TestRecord_Lifecycle(t *testing.T) {
	s, _ := open(t)

	ev, err := s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0)
	require.NoError(t, err)
	require.Len(t, ev, 1)
	assert.Equal(t, EventFirstSeen, ev[0].Type)

	ev, err = s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, ev, "same cert → no event")

	ev, _ = s.Record([]*scanner.CertInfo{cert("B", t0.AddDate(1, 0, 0))}, t0.Add(2*time.Hour))
	require.Len(t, ev, 1)
	assert.Equal(t, EventRenewed, ev[0].Type)
	assert.Equal(t, "s-A", ev[0].OldSerial)
	assert.Equal(t, "s-B", ev[0].NewSerial)

	ev, _ = s.Record([]*scanner.CertInfo{cert("C", t0.AddDate(0, 1, 0))}, t0.Add(3*time.Hour))
	require.Len(t, ev, 1)
	assert.Equal(t, EventChanged, ev[0].Type, "shorter NotAfter is not a renewal")

	sum := s.Summary("")
	require.Len(t, sum, 1)
	assert.Equal(t, 1, sum[0].Renewals)
	assert.Equal(t, t0.Add(2*time.Hour), sum[0].LastRenewed)
}

func TestRecord_SkipsFailedScans(t *testing.T) {
	s, _ := open(t)
	_, _ = s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0)

	bad := &scanner.CertInfo{Host: "a.example", Port: "443", Error: "timeout"}
	ev, err := s.Record([]*scanner.CertInfo{bad, nil, {Host: "x", Port: "1"}}, t0.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, ev)

	ev, _ = s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0.Add(2*time.Hour))
	assert.Empty(t, ev, "last good cert preserved across failed scan")
}

func TestPersistenceAndPerms(t *testing.T) {
	s, p := open(t)
	_, err := s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0)
	require.NoError(t, err)

	st, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	s2, err := Open(p)
	require.NoError(t, err)
	ev, _ := s2.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0.Add(time.Hour))
	assert.Empty(t, ev, "state survives reopen")
	assert.Len(t, s2.Events(Filter{}), 1)
}

func TestOpen_MissingAndCorrupt(t *testing.T) {
	s, _ := open(t)
	assert.Empty(t, s.Events(Filter{}))

	p := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(p, []byte("{nope"), 0o600))
	_, err := Open(p)
	assert.ErrorContains(t, err, "corrupt")
	b, _ := os.ReadFile(p)
	assert.Equal(t, "{nope", string(b), "corrupt file untouched")
}

func TestEventCap(t *testing.T) {
	s, _ := open(t)
	for i := 0; i < MaxEventsPerTarget+10; i++ {
		fp := string(rune('A'+i%26)) + string(rune('a'+i/26))
		_, err := s.Record([]*scanner.CertInfo{cert(fp, t0.AddDate(0, 0, i+1))}, t0.Add(time.Duration(i)*time.Hour))
		require.NoError(t, err)
	}
	ev := s.Events(Filter{})
	assert.Len(t, ev, MaxEventsPerTarget)
	assert.True(t, ev[0].Time.After(ev[len(ev)-1].Time), "newest first")
}

func TestEventsFilter(t *testing.T) {
	s, _ := open(t)
	other := cert("Z", t0.AddDate(0, 3, 0))
	other.Host = "b.example"
	_, _ = s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0)), other}, t0)
	_, _ = s.Record([]*scanner.CertInfo{cert("B", t0.AddDate(1, 0, 0))}, t0.Add(48*time.Hour))

	assert.Len(t, s.Events(Filter{}), 3)
	assert.Len(t, s.Events(Filter{Target: "a.example:443"}), 2)
	assert.Len(t, s.Events(Filter{Type: EventRenewed}), 1)
	assert.Len(t, s.Events(Filter{Since: t0.Add(24 * time.Hour)}), 1)
	assert.Len(t, s.Summary("b.example:443"), 1)
}

func TestRecord_Concurrent(t *testing.T) {
	s, _ := open(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Record([]*scanner.CertInfo{cert("A", t0.AddDate(0, 3, 0))}, t0)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Len(t, s.Events(Filter{}), 1)
}

func TestTargetKey_K8s(t *testing.T) {
	assert.Equal(t, "ns/secret", TargetKey(&scanner.CertInfo{Host: "ns/secret"}))
}
