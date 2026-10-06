package infra

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jevido/bakery/services/api/contexts/notifications/domain"
)

type captured struct {
	method, path string
	header       http.Header
	body         []byte
}

func capture(t *testing.T, status int, answer string) (*httptest.Server, *captured) {
	t.Helper()
	got := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.header = r.Method, r.URL.Path, r.Header.Clone()
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

var failed = domain.Notification{
	Kind: domain.DeploymentFailure, Title: "Deployment of shop failed", Body: "build failed",
	Link: "http://localhost:4930/applications/3", At: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	return m
}

func TestDiscordAndSlack(t *testing.T) {
	srv, got := capture(t, http.StatusNoContent, "")
	c := domain.Channel{Kind: domain.Discord, Settings: domain.Settings{URL: srv.URL + "/api/webhooks/1/tok"}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || got.path != "/api/webhooks/1/tok" || got.header.Get("Content-Type") != "application/json" {
		t.Errorf("got %s %s %v", got.method, got.path, got.header)
	}
	if m := decode(t, got.body); m["content"] != "**Deployment of shop failed**\nbuild failed\nhttp://localhost:4930/applications/3" {
		t.Errorf("content %q", m["content"])
	}

	c = domain.Channel{Kind: domain.Slack, Settings: domain.Settings{URL: srv.URL + "/services/x"}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	if m := decode(t, got.body); !strings.HasPrefix(m["text"].(string), "*Deployment of shop failed*\n") {
		t.Errorf("text %q", m["text"])
	}
}

func TestErrorsHideSecrets(t *testing.T) {
	srv, _ := capture(t, http.StatusNotFound, `{"message": "Unknown Webhook"}`)
	c := domain.Channel{Kind: domain.Discord, Settings: domain.Settings{URL: srv.URL + "/api/webhooks/1/secret-token"}}
	err := (Senders{}).Send(context.Background(), c, failed)
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "Unknown Webhook") {
		t.Fatalf("got %v", err)
	}
	// Unreachable: Go's error quotes the URL.
	c.Settings.URL = "http://127.0.0.1:1/api/webhooks/1/secret-token"
	err = (Senders{}).Send(context.Background(), c, failed)
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("got %v", err)
	}
	tg := domain.Channel{Kind: domain.Telegram, Settings: domain.Settings{BotToken: "123:very-secret", ChatID: "1"}}
	err = (Senders{TelegramAPI: "http://127.0.0.1:1"}).Send(context.Background(), tg, failed)
	if err == nil || strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("got %v", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	srv := httptest.NewServer(http.RedirectHandler("http://example.com/", http.StatusFound))
	defer srv.Close()
	c := domain.Channel{Kind: domain.Webhook, Settings: domain.Settings{URL: srv.URL}}
	if err := (Senders{}).Send(context.Background(), c, failed); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("got %v", err)
	}
}

func TestTelegram(t *testing.T) {
	srv, got := capture(t, http.StatusOK, `{"ok":true}`)
	c := domain.Channel{Kind: domain.Telegram, Settings: domain.Settings{BotToken: "123:abc", ChatID: "-100"}}
	if err := (Senders{TelegramAPI: srv.URL + "/"}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	m := decode(t, got.body)
	if got.path != "/bot123:abc/sendMessage" || m["chat_id"] != "-100" || !strings.HasPrefix(m["text"].(string), "Deployment of shop failed\n") {
		t.Errorf("got %s %v", got.path, m)
	}

	bad, _ := capture(t, http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`)
	err := (Senders{TelegramAPI: bad.URL}).Send(context.Background(), c, failed)
	if err == nil || err.Error() != "telegram: Bad Request: chat not found" {
		t.Fatalf("got %v", err)
	}
}

func TestNtfy(t *testing.T) {
	srv, got := capture(t, http.StatusOK, `{}`)
	c := domain.Channel{Kind: domain.Ntfy, Settings: domain.Settings{URL: srv.URL, Topic: "bakery", Token: "tk_1"}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	h := got.header
	if got.path != "/bakery" || h.Get("Title") != "Deployment of shop failed" || h.Get("Tags") != "warning" ||
		h.Get("Click") != failed.Link || h.Get("Authorization") != "Bearer tk_1" {
		t.Errorf("got %s %v", got.path, h)
	}
	if string(got.body) != "build failed\n"+failed.Link {
		t.Errorf("body %q", got.body)
	}
	ok := failed
	ok.Kind = domain.ServerReachable
	if err := (Senders{}).Send(context.Background(), c, ok); err != nil || got.header.Get("Tags") != "white_check_mark" {
		t.Errorf("tags %q, %v", got.header.Get("Tags"), err)
	}
}

func TestWebhookSigned(t *testing.T) {
	srv, got := capture(t, http.StatusAccepted, "")
	c := domain.Channel{Kind: domain.Webhook, Settings: domain.Settings{URL: srv.URL + "/hook", Secret: "s3cret"}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	m := decode(t, got.body)
	if m["event"] != "deployment_failure" || m["title"] != failed.Title || m["at"] != "2026-09-30T12:00:00Z" || m["link"] != failed.Link {
		t.Errorf("payload %v", m)
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(got.body)
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); got.header.Get(SignatureHeader) != want {
		t.Errorf("signature %q, want %q", got.header.Get(SignatureHeader), want)
	}
	c.Settings.Secret = ""
	_ = (Senders{}).Send(context.Background(), c, failed)
	if got.header.Get(SignatureHeader) != "" {
		t.Error("signed without a secret")
	}
}

// smtpServer is just enough SMTP for one session: it records AUTH, MAIL,
// RCPT and the DATA.
type smtpServer struct {
	addr string
	mu   sync.Mutex
	auth string
	ehlo string
	from string
	to   []string
	data string
}

func startSMTP(t *testing.T) *smtpServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	s := &smtpServer{addr: l.Addr().String()}
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		say := func(line string) { _, _ = io.WriteString(conn, line+"\r\n") }
		say("220 test ESMTP")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
			s.mu.Lock()
			switch {
			case cmd == "EHLO":
				s.ehlo = line
				say("250-test")
				say("250 AUTH PLAIN")
			case cmd == "AUTH":
				raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
				s.auth = string(raw)
				say("235 ok")
			case cmd == "MAIL":
				s.from = line
				say("250 ok")
			case cmd == "RCPT":
				s.to = append(s.to, line)
				say("250 ok")
			case cmd == "DATA":
				say("354 go")
				var b strings.Builder
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					b.WriteString(l)
				}
				s.data = b.String()
				say("250 queued")
			case cmd == "QUIT":
				say("221 bye")
				s.mu.Unlock()
				return
			default:
				say("250 ok")
			}
			s.mu.Unlock()
		}
	}()
	return s
}

func TestEmail(t *testing.T) {
	srv := startSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	var p int
	p, _ = strconv.Atoi(port)
	c := domain.Channel{Kind: domain.Email, Settings: domain.Settings{
		Host: host, Port: p, Security: domain.SecurityNone, Username: "u", Password: "pw",
		From: "bakery@example.com", To: []string{"a@example.com", "b@example.com"},
	}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.auth != "\x00u\x00pw" || srv.from != "MAIL FROM:<bakery@example.com>" || len(srv.to) != 2 {
		t.Errorf("auth %q from %q to %v", srv.auth, srv.from, srv.to)
	}
	if srv.ehlo != "EHLO localhost" {
		t.Errorf("ehlo %q, want Go's default", srv.ehlo)
	}
	for _, want := range []string{"From: \"The Bakery\" <bakery@example.com>\r\n", "Subject: [The Bakery] Deployment of shop failed\r\n", "To: a@example.com, b@example.com\r\n", "\r\n\r\nbuild failed\r\nhttp://localhost:4930/applications/3\r\n"} {
		if !strings.Contains(srv.data, want) {
			t.Errorf("message lacks %q:\n%s", want, srv.data)
		}
	}
}

func TestStartTLSRequired(t *testing.T) {
	srv := startSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	var p int
	p, _ = strconv.Atoi(port)
	c := domain.Channel{Kind: domain.Email, Settings: domain.Settings{Host: host, Port: p, Security: domain.SecurityStartTLS, From: "b@example.com", To: []string{"a@example.com"}}}
	if err := (Senders{}).Send(context.Background(), c, failed); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("got %v", err)
	}
}

func TestEmailFromNameAndEHLO(t *testing.T) {
	srv := startSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	p, _ := strconv.Atoi(port)
	c := domain.Channel{Kind: domain.Email, Settings: domain.Settings{
		Host: host, Port: p, Security: domain.SecurityNone, From: "bakery@example.com", To: []string{"a@example.com"},
		FromName: "Ops", Timeout: 5, EHLODomain: "mail.example.com",
	}}
	if err := (Senders{}).Send(context.Background(), c, failed); err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.ehlo != "EHLO mail.example.com" {
		t.Errorf("ehlo %q", srv.ehlo)
	}
	if !strings.Contains(srv.data, "From: \"Ops\" <bakery@example.com>\r\n") {
		t.Errorf("message lacks the from name:\n%s", srv.data)
	}
}
