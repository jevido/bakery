// Package bakery is the Desktop app's thin client of a Bakery's API: JSON
// over net/http, the Desktop key as a Bearer, and the Guild a request is
// meant for in the Bakery-Guild header. It knows only the routes the
// Desktop app calls.
package bakery

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The prefixes the Bakery expects on what the Desktop app mints
// (services/api contexts/identity/domain/desktop.go).
const (
	signInSecretPrefix = "bky_signin_"
	desktopKeyPrefix   = "bky_desk_"
)

// ErrSignedOut is a 401 to a Desktop key: the Desktop was signed out on the
// Bakery (from its Desktops page, by a password change, or idle too long).
var ErrSignedOut = errors.New("this desktop is signed out of the Bakery")

// ErrNotFound is a 404: for a sign-in, an unknown id or a wrong secret.
var ErrNotFound = errors.New("not found")

// Error is any other answer the Bakery refused, with its message.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("the Bakery answered %d", e.Status)
	}
	return e.Message
}

// NormalizeAddress turns what a person typed into a Bakery's address:
// https:// when no scheme is given, no trailing slash, no query or fragment.
func NormalizeAddress(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("enter the Bakery's address")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not an address", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("the address must start with https:// or http://, not %s://", u.Scheme)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath, u.RawQuery, u.Fragment, u.User = "", "", "", nil
	return u.String(), nil
}

// Client talks to one Bakery. Key is empty before the Desktop is signed in.
type Client struct {
	Address string
	Key     string
	HTTP    *http.Client
}

// New returns a Client for address with key (empty before sign-in).
func New(address, key string) *Client {
	return &Client{Address: address, Key: key, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// do sends body as JSON (when not nil) and decodes the answer into out
// (when not nil). guildID 0 sends no Bakery-Guild.
func (c *Client) do(ctx context.Context, method, path string, guildID uint64, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Address+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if guildID != 0 {
		req.Header.Set("Bakery-Guild", strconv.FormatUint(guildID, 10))
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized && c.Key != "":
		return ErrSignedOut
	case res.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case res.StatusCode >= 300:
		var e struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Message == "" {
			e.Message = e.Error
		}
		return &Error{Status: res.StatusCode, Message: e.Message}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("the Bakery's answer to %s %s: %w", method, path, err)
	}
	return nil
}

// SignIn is a Desktop sign-in this app started: its secret and the Desktop
// key it will hold once approved never leave this computer except the
// secret in the approve link.
type SignIn struct {
	ID           uint64        `json:"id"`
	ApprovalURL  string        `json:"approval_url"`
	ExpiresAt    time.Time     `json:"expires_at"`
	PollInterval time.Duration `json:"-"`
	Token        string        `json:"-"`
	Key          string        `json:"-"`
}

func minted(prefix string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

// StartSignIn starts a Desktop sign-in named clientName. It mints the
// secret and the Desktop key here and sends only the key's SHA-256, as
// Paperclip's CLI auth does with its board API key.
func (c *Client) StartSignIn(ctx context.Context, clientName string) (SignIn, error) {
	token, err := minted(signInSecretPrefix)
	if err != nil {
		return SignIn{}, err
	}
	key, err := minted(desktopKeyPrefix)
	if err != nil {
		return SignIn{}, err
	}
	sum := sha256.Sum256([]byte(key))
	var out struct {
		ID             uint64    `json:"id"`
		ApprovalURL    string    `json:"approval_url"`
		ExpiresAt      time.Time `json:"expires_at"`
		PollIntervalMs int       `json:"poll_interval_ms"`
	}
	in := map[string]string{"client_name": clientName, "token": token, "desktop_key_hash": hex.EncodeToString(sum[:])}
	if err := c.do(ctx, http.MethodPost, "/api/desktop-sign-ins", 0, in, &out); err != nil {
		return SignIn{}, err
	}
	poll := time.Duration(out.PollIntervalMs) * time.Millisecond
	if poll < 500*time.Millisecond {
		poll = time.Second
	}
	return SignIn{ID: out.ID, ApprovalURL: out.ApprovalURL, ExpiresAt: out.ExpiresAt, PollInterval: poll, Token: token, Key: key}, nil
}

// SignInStatus is the sign-in's status: pending, approved, expired or
// cancelled.
func (c *Client) SignInStatus(ctx context.Context, s SignIn) (string, error) {
	var out struct {
		Status string `json:"status"`
	}
	path := fmt.Sprintf("/api/desktop-sign-ins/%d?token=%s", s.ID, url.QueryEscape(s.Token))
	if err := c.do(ctx, http.MethodGet, path, 0, nil, &out); err != nil {
		return "", err
	}
	return out.Status, nil
}

// CancelSignIn cancels a pending sign-in.
func (c *Client) CancelSignIn(ctx context.Context, s SignIn) error {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/desktop-sign-ins/%d/cancel", s.ID), 0, map[string]string{"token": s.Token}, nil)
}

// Member is the person a Desktop key acts as.
type Member struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Me is the person the key acts as.
func (c *Client) Me(ctx context.Context) (Member, error) {
	var out struct {
		Member Member `json:"member"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/me", 0, nil, &out); err != nil {
		return Member{}, err
	}
	return out.Member, nil
}

// DesktopID is the id the Bakery knows this Desktop by: the one its
// Desktops list marks current.
func (c *Client) DesktopID(ctx context.Context) (uint64, error) {
	var out []struct {
		ID      uint64 `json:"id"`
		Current bool   `json:"current"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/desktops", 0, nil, &out); err != nil {
		return 0, err
	}
	for _, d := range out {
		if d.Current {
			return d.ID, nil
		}
	}
	return 0, errors.New("the Bakery does not list this desktop")
}

// SignOut signs this Desktop out on the Bakery; its key stops working.
func (c *Client) SignOut(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/desktops/current/sign-out", 0, nil, nil)
}

// Guild is one Guild the person is in.
type Guild struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	IssuePrefix string `json:"issue_prefix"`
}

// Guilds lists the Guilds the person is in.
func (c *Client) Guilds(ctx context.Context) ([]Guild, error) {
	var out struct {
		Guilds []Guild `json:"guilds"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/guilds", 0, nil, &out); err != nil {
		return nil, err
	}
	return out.Guilds, nil
}

// Named is a Member or Agent the Bakery names by id.
type Named struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// Role is a Role an Agent holds.
type Role struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

// Agent is an Agent as the Bakery's agents context shows it
// (services/api contexts/agents/http/agents.go), less what only the
// dashboard uses to manage it.
type Agent struct {
	ID           uint64     `json:"id"`
	Name         string     `json:"name"`
	Job          string     `json:"job"`
	JobLabel     string     `json:"job_label"`
	Title        string     `json:"title"`
	Icon         string     `json:"icon"`
	Capabilities string     `json:"capabilities"`
	Status       string     `json:"status"`
	ReportsTo    *Named     `json:"reports_to"`
	Hirer        *Named     `json:"hirer"`
	Roles        []Role     `json:"roles"`
	CreatedAt    time.Time  `json:"created_at"`
	TerminatedAt *time.Time `json:"terminated_at"`
}

// Agents lists the Agents of the Guild guildID, filtered by status as the
// Agents page's tabs do: all, active, paused, pending or terminated.
func (c *Client) Agents(ctx context.Context, guildID uint64, status string) ([]Agent, error) {
	var out struct {
		Agents []Agent `json:"agents"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/agents?status="+url.QueryEscape(status), guildID, nil, &out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

// Agent reads one Agent of the Guild guildID.
func (c *Client) Agent(ctx context.Context, guildID, id uint64) (Agent, error) {
	var out struct {
		Agent Agent `json:"agent"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/agents/%d", id), guildID, nil, &out); err != nil {
		return Agent{}, err
	}
	return out.Agent, nil
}
