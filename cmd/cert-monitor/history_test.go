package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/history"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordHistoryAndPrint(t *testing.T) {
	t.Setenv(envHistoryFile, "")
	p := filepath.Join(t.TempDir(), "h.json")
	mk := func(fp string, na time.Time) []*scanner.CertInfo {
		return []*scanner.CertInfo{{Host: "a", Port: "443", Serial: "s" + fp, Fingerprint: fp, NotAfter: na}}
	}
	now := time.Now()
	recordHistory(zerolog.Nop(), p, mk("A", now.Add(24*time.Hour)))
	recordHistory(zerolog.Nop(), p, mk("B", now.Add(48*time.Hour)))
	recordHistory(zerolog.Nop(), "", mk("C", now.Add(72*time.Hour))) // disabled: no-op

	store, err := history.Open(p)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, printSummary(&buf, store.Summary(""), false))
	assert.Contains(t, buf.String(), "a:443")

	buf.Reset()
	require.NoError(t, printEvents(&buf, store.Events(history.Filter{Type: history.EventRenewed}), true))
	var evs []history.Event
	require.NoError(t, json.Unmarshal(buf.Bytes(), &evs))
	require.Len(t, evs, 1)
	assert.Equal(t, "sB", evs[0].NewSerial)

	buf.Reset()
	require.NoError(t, printEvents(&buf, nil, true))
	assert.JSONEq(t, "[]", buf.String())
}

func TestHistoryCmd_RequiresFile(t *testing.T) {
	t.Setenv(envHistoryFile, "")
	cmd := newHistoryCmd()
	cmd.SetArgs(nil)
	assert.ErrorContains(t, cmd.Execute(), "history-file")

	p := filepath.Join(t.TempDir(), "none.json")
	cmd = newHistoryCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--history-file", p})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "TARGET")
}
