package notifier

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleMsg() Message {
	return Message{
		Source:  "tool",
		Time:    time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC),
		Header:  "head",
		Subject: "subj",
		Summary: "sum",
		Danger:  true,
		Lines:   []Line{{Label: "HIGH", Target: "rule_a", Detail: "noisy"}},
	}
}

func TestSlackSender_Payload(t *testing.T) {
	var got SlackMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()

	s := NewSlackSender(srv.URL, "#ops", "bot", time.Second)
	require.NoError(t, s.Send(context.Background(), sampleMsg()))
	assert.Equal(t, "#ops", got.Channel)
	assert.Equal(t, "bot", got.Username)
	assert.Equal(t, "head", got.Text)
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "danger", got.Attachments[0].Color)
	assert.Contains(t, got.Attachments[0].Text, "*HIGH* `rule_a` — noisy")
}

func TestSlackSender_WarningColorNoLinesNoCall(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	s := NewSlackSender(srv.URL, "", "", time.Second)
	require.NoError(t, s.Send(context.Background(), Message{}))
	assert.Zero(t, calls)
	m := sampleMsg()
	m.Danger = false
	assert.Equal(t, "warning", s.Format(m).Attachments[0].Color)
}

func TestSlackSender_TruncatesAndEscapes(t *testing.T) {
	s := NewSlackSender("http://x", "", "", time.Second)
	m := Message{}
	for i := 0; i < MaxSlackLines+5; i++ {
		m.Lines = append(m.Lines, Line{Label: "L", Target: "<!channel>&"})
	}
	text := s.Format(m).Attachments[0].Text
	assert.Contains(t, text, "and 5 more")
	assert.Equal(t, MaxSlackLines+1, strings.Count(text, "\n"))
	assert.NotContains(t, text, "<!channel>")
	assert.Contains(t, text, "&lt;!channel&gt;&amp;")
}

func TestPostJSON_ErrorsDoNotLeakURL(t *testing.T) {
	s := NewSlackSender("http://127.0.0.1:1/secret-token", "", "", time.Second)
	err := s.Send(context.Background(), sampleMsg())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	err = NewSlackSender(srv.URL, "", "", time.Second).Send(context.Background(), sampleMsg())
	assert.ErrorContains(t, err, "403")
}

func TestWebhookSender(t *testing.T) {
	var got Message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	w := NewWebhookSender(srv.URL, time.Second)
	require.NoError(t, w.Send(context.Background(), sampleMsg()))
	assert.Equal(t, "tool", got.Source)
	require.Len(t, got.Lines, 1)
	assert.Equal(t, "rule_a", got.Lines[0].Target)
	assert.Equal(t, "webhook", w.Name())
}

type fakeSender struct {
	name string
	err  error
	n    int
}

func (f *fakeSender) Name() string                        { return f.name }
func (f *fakeSender) Send(context.Context, Message) error { f.n++; return f.err }

func TestMulti_ContinuesAfterFailureAndJoinsErrors(t *testing.T) {
	bad := &fakeSender{name: "a", err: errors.New("boom")}
	good := &fakeSender{name: "b"}
	m := Multi{bad, good}
	err := m.Send(context.Background(), sampleMsg())
	assert.ErrorContains(t, err, "a: boom")
	assert.Equal(t, 1, good.n)
	assert.Equal(t, "a,b", m.Name())
}

func TestEmailConfig_Validate(t *testing.T) {
	c := EmailConfig{Host: "h", From: "a@x", To: []string{"b@x"}}
	require.NoError(t, c.Validate())
	assert.Equal(t, 587, c.Port)
	assert.Equal(t, TLSStartTLS, c.TLS)

	bad := []EmailConfig{
		{From: "a@x", To: []string{"b@x"}},
		{Host: "h", To: []string{"b@x"}},
		{Host: "h", From: "a@x"},
		{Host: "h", From: "a@x", To: []string{"b@x"}, TLS: "weird"},
		{Host: "h", From: "a@x\r\nBcc: z@x", To: []string{"b@x"}},
	}
	for _, b := range bad {
		assert.Error(t, b.Validate(), "%+v", b)
	}
}

func TestBuildEmail_HeadersAndInjection(t *testing.T) {
	m := sampleMsg()
	m.Subject = "ok\r\nBcc: evil@x"
	m.Lines[0].Target = "r\nX-Evil: 1"
	out := string(BuildEmail(EmailConfig{From: "f@x", To: []string{"a@x", "b@x"}}, m))
	assert.Contains(t, out, "To: a@x, b@x\r\n")
	assert.Contains(t, out, "Subject: ok  Bcc: evil@x\r\n")
	assert.NotContains(t, out, "\r\nBcc:")
	assert.NotContains(t, out, "\nX-Evil")
	assert.Contains(t, out, "sum\r\n")
	assert.Contains(t, out, "HIGH          r X-Evil: 1 — noisy")
}

func TestEmailSender_SendNoLinesAndCapture(t *testing.T) {
	var sent []byte
	s, err := NewEmailSender(EmailConfig{Host: "h", From: "f@x", To: []string{"a@x"}})
	require.NoError(t, err)
	s.send = func(_ context.Context, _ EmailConfig, msg []byte) error { sent = msg; return nil }
	require.NoError(t, s.Send(context.Background(), Message{}))
	assert.Nil(t, sent)
	require.NoError(t, s.Send(context.Background(), sampleMsg()))
	assert.Contains(t, string(sent), "Subject: subj")
	assert.Equal(t, "email", s.Name())
}

func TestEmailSender_RefusesCredentialsWithoutTLS(t *testing.T) {
	host, port := splitAddr(t, fakeSMTP(t, nil))
	s, err := NewEmailSender(EmailConfig{Host: host, Port: port, From: "a@x", To: []string{"b@x"}, TLS: TLSNone, Username: "u", Password: "p"})
	require.NoError(t, err)
	assert.ErrorContains(t, s.Send(context.Background(), sampleMsg()), "without TLS")
}

func TestEmailSender_SendsOverSMTP(t *testing.T) {
	var mu sync.Mutex
	var data []string
	host, port := splitAddr(t, fakeSMTP(t, func(l []string) { mu.Lock(); data = l; mu.Unlock() }))
	s, err := NewEmailSender(EmailConfig{Host: host, Port: port, From: "f@x", To: []string{"a@x"}, TLS: TLSNone, Timeout: 2 * time.Second})
	require.NoError(t, err)
	require.NoError(t, s.Send(context.Background(), sampleMsg()))
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, strings.Join(data, "\n"), "Subject: subj")
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	h, p, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	n, err := strconv.Atoi(p)
	require.NoError(t, err)
	return h, n
}

// fakeSMTP is a minimal plaintext SMTP server; onData receives the DATA lines.
func fakeSMTP(t *testing.T, onData func([]string)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		write("220 fake ESMTP")
		var body []string
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					if onData != nil {
						onData(body)
					}
					write("250 queued")
					continue
				}
				body = append(body, line)
				continue
			}
			switch cmd := strings.ToUpper(line); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				write("250 fake")
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				write("354 go")
			case strings.HasPrefix(cmd, "QUIT"):
				write("221 bye")
				return
			default:
				write("250 ok")
			}
		}
	}()
	return l.Addr().String()
}
