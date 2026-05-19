package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"
)

// SMTPConfig holds SMTP transport parameters.
type SMTPConfig struct {
	Addr     string // host:port, e.g. "mailout.tum.de:587"
	From     string
	Username string // optional; if empty, no AUTH is attempted
	Password string
}

// SMTPMailer sends email via SMTP with STARTTLS.
type SMTPMailer struct {
	cfg SMTPConfig
}

// NewSMTPMailer constructs an SMTPMailer. Call Send to deliver each message.
func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Send delivers msg over SMTP with STARTTLS. A new connection is opened per
// message, which is fine at the expected send volume (chair-scale).
func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	host, _, err := net.SplitHostPort(m.cfg.Addr)
	if err != nil {
		return fmt.Errorf("smtp: bad addr %q: %w", m.cfg.Addr, err)
	}

	conn, err := net.DialTimeout("tcp", m.cfg.Addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer c.Close() //nolint:errcheck

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp: STARTTLS: %w", err)
		}
	}

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp: RCPT TO: %w", err)
	}

	raw, err := buildMIME(m.cfg.From, msg)
	if err != nil {
		return fmt.Errorf("smtp: build MIME: %w", err)
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := wc.Write(raw); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp: write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp: close data: %w", err)
	}

	return c.Quit()
}

// buildMIME produces a multipart/alternative message with text and HTML parts.
func buildMIME(from string, msg Message) ([]byte, error) {
	var bodyBuf bytes.Buffer
	mw := multipart.NewWriter(&bodyBuf)

	tw, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprint(tw, msg.BodyText); err != nil {
		return nil, err
	}

	hw, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/html; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprint(hw, msg.BodyHTML); err != nil {
		return nil, err
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&out, "From: %s\r\n", from)
	fmt.Fprintf(&out, "To: %s\r\n", msg.To)
	fmt.Fprintf(&out, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&out, "Content-Type: multipart/alternative; boundary=%q\r\n", mw.Boundary())
	fmt.Fprintf(&out, "\r\n")
	out.Write(bodyBuf.Bytes())

	return out.Bytes(), nil
}
