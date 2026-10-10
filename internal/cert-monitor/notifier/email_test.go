package notifier

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailConfig_Validate(t *testing.T) {
	assert.Error(t, (&EmailConfig{}).Validate())
	assert.Error(t, (&EmailConfig{Host: "h", From: "a@x"}).Validate())
	assert.Error(t, (&EmailConfig{Host: "h", From: "a@x", To: []string{"b@x"}, TLS: "bogus"}).Validate())
	assert.Error(t, (&EmailConfig{Host: "h", From: "a@x\r\nBcc: evil@x", To: []string{"b@x"}}).Validate())

	c := EmailConfig{Host: "h", From: "a@x", To: []string{"b@x"}}
	require.NoError(t, c.Validate())
	assert.Equal(t, 587, c.Port)
	assert.Equal(t, TLSStartTLS, c.TLS)
}

func TestEmailNotifier_Message(t *testing.T) {
	var sent []byte
	n, err := NewEmailNotifier(EmailConfig{Host: "h", From: "cm@x", To: []string{"a@x", "b@x"}})
	require.NoError(t, err)
	n.send = func(_ context.Context, _ EmailConfig, msg []byte) error { sent = msg; return nil }

	require.NoError(t, n.Notify(context.Background(), Report{}))
	assert.Nil(t, sent, "no alerts => no mail")

	require.NoError(t, n.Notify(context.Background(), BuildReport(sampleResults(), Options{})))
	s := string(sent)
	assert.Contains(t, s, "To: a@x, b@x\r\n")
	assert.Contains(t, s, "Subject: [cert-monitor] 5 certificate alert(s) (worst: EXPIRED/ERROR)")
	assert.Contains(t, s, "crit:443")
}

func TestEmailNotifier_RefusesCredentialsWithoutTLS(t *testing.T) {
	addr := fakeSMTP(t, nil)
	host, port, _ := net.SplitHostPort(addr)
	p := 0
	for _, c := range port {
		p = p*10 + int(c-'0')
	}
	cfg := EmailConfig{Host: host, Port: p, From: "a@x", To: []string{"b@x"}, TLS: TLSNone, Username: "u", Password: "p"}
	n, err := NewEmailNotifier(cfg)
	require.NoError(t, err)
	err = n.Notify(context.Background(), Report{Alerts: []CertAlert{{Host: "h", Status: "WARNING"}}})
	assert.ErrorContains(t, err, "without TLS")
}

func TestEmailNotifier_SendsOverSMTP(t *testing.T) {
	var mu sync.Mutex
	var data []string
	addr := fakeSMTP(t, func(lines []string) { mu.Lock(); data = lines; mu.Unlock() })
	host, port, _ := net.SplitHostPort(addr)
	p := 0
	for _, c := range port {
		p = p*10 + int(c-'0')
	}

	n, err := NewEmailNotifier(EmailConfig{Host: host, Port: p, From: "cm@x", To: []string{"a@x"}, TLS: TLSNone, Timeout: 2 * time.Second})
	require.NoError(t, err)
	require.NoError(t, n.Notify(context.Background(), BuildReport(sampleResults(), Options{})))

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(data, "\n")
	assert.Contains(t, joined, "Subject: [cert-monitor]")
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
