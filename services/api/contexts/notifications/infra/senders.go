package infra

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

// sendTimeout bounds one send when the caller's context has no deadline.
const sendTimeout = 10 * time.Second

// Senders sends to every Channel kind.
type Senders struct {
	// TelegramAPI is the Bot API's base URL, https://api.telegram.org.
	TelegramAPI string
	// Client is used for every HTTP kind; nil means one that follows no
	// redirects (an answer is taken as it comes).
	Client *http.Client
}

func (s Senders) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Send sends n to c.
func (s Senders) Send(ctx context.Context, c domain.Channel, n domain.Notification) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sendTimeout)
		defer cancel()
	}
	var err error
	switch c.Kind {
	case domain.Email:
		err = sendEmail(ctx, c.Settings, n)
	case domain.Discord:
		err = s.postJSON(ctx, c.Settings.URL, nil, map[string]any{"content": lines("**"+n.Title+"**", n.Body, n.Link)})
	case domain.Slack:
		err = s.postJSON(ctx, c.Settings.URL, nil, map[string]any{"text": lines("*"+n.Title+"*", n.Body, n.Link)})
	case domain.Telegram:
		err = s.telegram(ctx, c.Settings, n)
	case domain.Ntfy:
		err = s.ntfy(ctx, c.Settings, n)
	case domain.Webhook:
		err = s.webhook(ctx, c.Settings, n)
	default:
		err = fmt.Errorf("unknown channel kind %q", c.Kind)
	}
	if err != nil {
		return errors.New(scrub(err.Error(), c))
	}
	return nil
}

// lines joins the non-empty parts with newlines.
func lines(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}

// scrub replaces the channel's secrets in an error message: Go's HTTP
// errors quote the URL, which for Discord, Slack and Telegram is the
// credential.
func scrub(msg string, c domain.Channel) string {
	s := c.Settings
	if c.Kind.URLIsSecret() {
		if u, err := url.Parse(s.URL); err == nil && u.Host != "" {
			msg = strings.ReplaceAll(msg, s.URL, u.Scheme+"://"+u.Host+"/…")
		}
	}
	for _, secret := range []string{s.BotToken, s.Token, s.Password, s.Secret} {
		if len(secret) >= 4 {
			msg = strings.ReplaceAll(msg, secret, "…")
		}
	}
	return msg
}

func (s Senders) post(ctx context.Context, target, contentType string, headers map[string]string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Bakery")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		excerpt := strings.TrimSpace(string(answer))
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		if excerpt == "" {
			return answer, fmt.Errorf("answered %s", resp.Status)
		}
		return answer, fmt.Errorf("answered %s: %s", resp.Status, excerpt)
	}
	return answer, nil
}

func (s Senders) postJSON(ctx context.Context, target string, headers map[string]string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.post(ctx, target, "application/json", headers, body)
	return err
}

func (s Senders) telegram(ctx context.Context, st domain.Settings, n domain.Notification) error {
	base := strings.TrimRight(s.TelegramAPI, "/")
	if base == "" {
		base = "https://api.telegram.org"
	}
	body, err := json.Marshal(map[string]any{
		"chat_id": st.ChatID, "text": lines(n.Title, n.Body, n.Link), "disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	answer, err := s.post(ctx, base+"/bot"+st.BotToken+"/sendMessage", "application/json", nil, body)
	// Telegram explains a refusal in its JSON answer.
	var tg struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if json.Unmarshal(answer, &tg) == nil && !tg.OK && tg.Description != "" {
		return errors.New("telegram: " + tg.Description)
	}
	return err
}

func (s Senders) ntfy(ctx context.Context, st domain.Settings, n domain.Notification) error {
	headers := map[string]string{"Title": mime.QEncoding.Encode("utf-8", n.Title), "Tags": "white_check_mark"}
	if n.Kind.Alarming() {
		headers["Tags"] = "warning"
	}
	if n.Link != "" {
		headers["Click"] = n.Link
	}
	if st.Token != "" {
		headers["Authorization"] = "Bearer " + st.Token
	}
	_, err := s.post(ctx, strings.TrimRight(st.URL, "/")+"/"+url.PathEscape(st.Topic), "text/plain; charset=utf-8", headers, []byte(lines(n.Body, n.Link)))
	return err
}

// SignatureHeader carries the HMAC-SHA256 of a webhook's body, hex, after
// "sha256=", as GitHub signs its webhooks.
const SignatureHeader = "X-Bakery-Signature"

func (s Senders) webhook(ctx context.Context, st domain.Settings, n domain.Notification) error {
	body, err := json.Marshal(map[string]any{
		"event": n.Kind, "title": n.Title, "body": n.Body, "link": n.Link, "at": n.At.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	var headers map[string]string
	if st.Secret != "" {
		mac := hmac.New(sha256.New, []byte(st.Secret))
		mac.Write(body)
		headers = map[string]string{SignatureHeader: "sha256=" + hex.EncodeToString(mac.Sum(nil))}
	}
	_, err = s.post(ctx, st.URL, "application/json", headers, body)
	return err
}

// plainAuth is SMTP AUTH PLAIN without net/smtp's refusal to send it
// unencrypted to anything but localhost: security "none" is the admin's
// choice, made on the channel.
type plainAuth struct{ username, password string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected server challenge")
	}
	return nil, nil
}

// emailMessage is n as a plain-text email from st.From to st.To.
func emailMessage(st domain.Settings, to []string, subject, body string, now time.Time) []byte {
	var b strings.Builder
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domainPart := st.From[strings.LastIndex(st.From, "@")+1:]
	fmt.Fprintf(&b, "From: Bakery <%s>\r\n", st.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domainPart)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	b.WriteString("\r\n")
	return []byte(b.String())
}

func sendEmail(ctx context.Context, st domain.Settings, n domain.Notification) error {
	msg := emailMessage(st, st.To, "[Bakery] "+n.Title, lines(n.Body, n.Link), time.Now())
	return sendMail(ctx, st, st.To, msg)
}

// Mail sends one email through the email channel c to the recipients
// given, not the channel's own.
func (s Senders) Mail(ctx context.Context, c domain.Channel, to []string, subject, body string) error {
	if c.Kind != domain.Email {
		return fmt.Errorf("channel %q is not an email channel", c.Name)
	}
	if err := sendMail(ctx, c.Settings, to, emailMessage(c.Settings, to, subject, body, time.Now())); err != nil {
		return errors.New(scrub(err.Error(), c))
	}
	return nil
}

// sendMail sends msg to the recipients through the email channel's SMTP
// server.
func sendMail(ctx context.Context, st domain.Settings, to []string, msg []byte) error {
	addr := net.JoinHostPort(st.Host, strconv.Itoa(st.Port))
	d := net.Dialer{}
	var conn net.Conn
	var err error
	tlsConfig := &tls.Config{ServerName: st.Host, MinVersion: tls.VersionTLS12}
	if st.Security == domain.SecurityTLS {
		conn, err = (&tls.Dialer{NetDialer: &d, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, st.Host)
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	defer c.Close()
	if st.Security == domain.SecurityStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: the server does not offer STARTTLS; choose security none or tls")
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if st.Username != "" {
		if err := c.Auth(plainAuth{st.Username, st.Password}); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := c.Mail(st.From); err != nil {
		return fmt.Errorf("smtp: from: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp: to %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	return c.Quit()
}
