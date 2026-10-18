package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTP TLS modes.
const (
	TLSNone     = "none"
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"
)

// EmailConfig configures the SMTP notifier.
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	TLS      string // none | starttls (default) | tls
	Timeout  time.Duration
}

// Validate checks the config and applies defaults.
func (c *EmailConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("smtp host is required")
	}
	if c.From == "" || len(c.To) == 0 {
		return fmt.Errorf("smtp from and at least one recipient are required")
	}
	if c.Port == 0 {
		c.Port = 587
	}
	if c.TLS == "" {
		c.TLS = TLSStartTLS
	}
	switch c.TLS {
	case TLSNone, TLSStartTLS, TLSImplicit:
	default:
		return fmt.Errorf("invalid smtp tls mode %q (want none, starttls or tls)", c.TLS)
	}
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	for _, a := range append([]string{c.From}, c.To...) {
		if strings.ContainsAny(a, "\r\n") {
			return fmt.Errorf("invalid address %q", a)
		}
	}
	return nil
}

// EmailSender sends messages over SMTP.
type EmailSender struct {
	cfg  EmailConfig
	send func(ctx context.Context, cfg EmailConfig, msg []byte) error
}

// NewEmailSender validates cfg and returns an EmailSender.
func NewEmailSender(cfg EmailConfig) (*EmailSender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &EmailSender{cfg: cfg, send: SendSMTP}, nil
}

// Name implements Sender.
func (n *EmailSender) Name() string { return "email" }

// Send implements Sender.
func (n *EmailSender) Send(ctx context.Context, m Message) error {
	if len(m.Lines) == 0 {
		return nil
	}
	return n.send(ctx, n.cfg, BuildEmail(n.cfg, m))
}

// BuildEmail renders m as an RFC 5322 plain-text mail. CR/LF are stripped from
// header values to prevent header injection from untrusted names.
func BuildEmail(cfg EmailConfig, m Message) []byte {
	ts := m.Time
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(cfg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", stripCRLF(m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", ts.Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	if m.Summary != "" {
		fmt.Fprintf(&b, "%s\r\n\r\n", m.Summary)
	}
	for _, l := range m.Lines {
		fmt.Fprintf(&b, "%-13s %s", stripCRLF(l.Label), stripCRLF(l.Target))
		if l.Detail != "" {
			fmt.Fprintf(&b, " — %s", stripCRLF(l.Detail))
		}
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func stripCRLF(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

// SendSMTP delivers a prebuilt message using cfg.
func SendSMTP(ctx context.Context, cfg EmailConfig, msg []byte) error {
	c, closeConn, err := dialSMTP(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeConn()
	defer c.Close()

	if cfg.Username != "" {
		if cfg.TLS == TLSNone {
			return fmt.Errorf("refusing to send smtp credentials without TLS")
		}
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	for _, to := range cfg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp RCPT %s: %w", to, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finishing message: %w", err)
	}
	return c.Quit()
}

// dialSMTP connects according to cfg.TLS and returns a ready client (STARTTLS
// already negotiated when requested) plus a func that closes the connection.
func dialSMTP(ctx context.Context, cfg EmailConfig) (*smtp.Client, func(), error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := net.Dialer{Timeout: cfg.Timeout}
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	var (
		conn net.Conn
		err  error
	)
	if cfg.TLS == TLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: &d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to smtp server: %w", err)
	}
	closeConn := func() {
		if err := conn.Close(); err != nil {
			_ = err // already closed by the SMTP client on the happy path
		}
	}
	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout * 3)); err != nil {
		closeConn()
		return nil, nil, fmt.Errorf("setting smtp deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		closeConn()
		return nil, nil, fmt.Errorf("smtp handshake: %w", err)
	}
	if cfg.TLS == TLSStartTLS {
		if err := c.StartTLS(tlsCfg); err != nil {
			closeConn()
			return nil, nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return c, closeConn, nil
}
