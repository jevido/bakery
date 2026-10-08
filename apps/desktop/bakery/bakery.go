// Package bakery is the Desktop app's thin client of a Bakery's API: JSON
// over net/http, the Desktop key as a Bearer, and the Guild a request is
// meant for in the Bakery-Guild header. It knows only the routes the
// Desktop app calls.
package bakery

import (
	"bufio"
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
	status, data, err := c.send(ctx, method, path, guildID, body)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusUnauthorized && c.Key != "":
		return ErrSignedOut
	case status == http.StatusNotFound:
		return ErrNotFound
	case status >= 300:
		return refusal(status, data)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("the Bakery's answer to %s %s: %w", method, path, err)
	}
	return nil
}

// Call sends body as JSON (when not nil) and returns the answer as it
// came, for a caller that passes it on (The Bakery's MCP server). Every
// refusal, a 401 and a 404 included, is an *Error with its status and
// message. guildID 0 sends no Bakery-Guild.
func (c *Client) Call(ctx context.Context, method, path string, guildID uint64, body any) (json.RawMessage, error) {
	status, data, err := c.send(ctx, method, path, guildID, body)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, refusal(status, data)
	}
	return data, nil
}

// send is one request: body as JSON when not nil, the Key as Bearer and
// guildID (when not 0) as Bakery-Guild; it answers the status and body.
func (c *Client) send(ctx context.Context, method, path string, guildID uint64, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Address+path, reader)
	if err != nil {
		return 0, nil, err
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
		return 0, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, data, nil
}

// refusal is the *Error for a refused answer, with the message the
// Bakery gave (its message or error field).
func refusal(status int, data []byte) *Error {
	var e struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(data, &e)
	if e.Message == "" {
		e.Message = e.Error
	}
	return &Error{Status: status, Message: e.Message}
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

// RunAgent is the Agent a Run is of.
type RunAgent struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// RunIssue is the Issue a Run works on.
type RunIssue struct {
	ID         uint64 `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// DesktopRun is a Run as the Bakery hands it to a Desktop
// (services/api contexts/agents/http/desktop.go).
type DesktopRun struct {
	ID               uint64     `json:"id"`
	Status           string     `json:"status"`
	Guild            Named      `json:"guild"`
	Agent            RunAgent   `json:"agent"`
	Issue            *RunIssue  `json:"issue"`
	InvocationSource string     `json:"invocation_source"`
	WakeReason       string     `json:"wake_reason"`
	WakeCount        int        `json:"wake_count"`
	Prompt           string     `json:"prompt"`
	RetryOfRunID     *uint64    `json:"retry_of_run_id"`
	SessionID        string     `json:"session_id"`
	NextSeq          int64      `json:"next_seq"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at"`
	LeaseExpiresAt   *time.Time `json:"lease_expires_at"`
	// RunKey is only in the claim's answer: the Run key the Runner hands to
	// this Run's claude. It is kept in memory only, never stored or logged.
	RunKey string `json:"run_key,omitempty"`
	// Workspace is only in the claim's answer: the git Worktree the Run
	// works in. Nil when its Issue names no Application with a repository.
	Workspace *RunWorkspace `json:"workspace"`
}

// RunWorkspace is where a claimed Run works: the Issue's Application, its
// git repository and base branch, and the Agent branch to work on.
type RunWorkspace struct {
	Application Named  `json:"application"`
	Repository  string `json:"repository"`
	BaseBranch  string `json:"base_branch"`
	Branch      string `json:"branch"`
}

// RunEvent is one thing a Run's claude printed, numbered by Seq from 1.
type RunEvent struct {
	Seq     int64           `json:"seq"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// Run is a Run as the Bakery's agents context shows it
// (services/api contexts/agents/http/runs.go), for an Agent's Runs on its
// page.
type Run struct {
	ID               uint64     `json:"id"`
	Agent            RunAgent   `json:"agent"`
	Issue            *RunIssue  `json:"issue"`
	InvocationSource string     `json:"invocation_source"`
	WakeReason       string     `json:"wake_reason"`
	WakeCount        int        `json:"wake_count"`
	Status           string     `json:"status"`
	RequestedBy      *Named     `json:"requested_by"`
	Desktop          *Named     `json:"desktop"`
	RetryOfRunID     *uint64    `json:"retry_of_run_id"`
	Usage            Usage      `json:"usage"`
	ExitCode         *int       `json:"exit_code"`
	Error            string     `json:"error"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	CanCancel        bool       `json:"can_cancel"`
}

// Runs lists the Agent id's Runs in the Guild guildID, newest first, at
// most limit.
func (c *Client) Runs(ctx context.Context, guildID, id uint64, limit int) ([]Run, error) {
	var out struct {
		Runs []Run `json:"runs"`
	}
	path := fmt.Sprintf("/api/runs?agent=%d&limit=%d", id, limit)
	if err := c.do(ctx, http.MethodGet, path, guildID, nil, &out); err != nil {
		return nil, err
	}
	return out.Runs, nil
}

// RunEvents answers the Run id's stored events after seq after, in the
// Guild guildID.
func (c *Client) RunEvents(ctx context.Context, guildID, id uint64, after int64) ([]RunEvent, error) {
	var out struct {
		Events []RunEvent `json:"events"`
	}
	path := fmt.Sprintf("/api/runs/%d/events?after=%d", id, after)
	if err := c.do(ctx, http.MethodGet, path, guildID, nil, &out); err != nil {
		return nil, err
	}
	return out.Events, nil
}

// RunState is where a Run stands after a report.
type RunState struct {
	ID             uint64     `json:"id"`
	Status         string     `json:"status"`
	NextSeq        int64      `json:"next_seq"`
	SessionID      string     `json:"session_id"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at"`
}

// Usage is a Run's usage as the claude CLI reported it; the cost is an
// equivalent, never billed.
type Usage struct {
	InputTokens       int64   `json:"input_tokens"`
	CachedInputTokens int64   `json:"cached_input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	Turns             int64   `json:"turns"`
	CostEquivalentUSD float64 `json:"cost_equivalent_usd"`
	DurationMS        int64   `json:"duration_ms"`
}

// Finish is how a Run ended on this Desktop: succeeded or failed.
type Finish struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
	Error    string `json:"error"`
	Usage    Usage  `json:"usage"`
}

// IsStatus says whether err is the Bakery refusing with status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// DesktopRuns lists the queued Runs of the person's Agents and the running
// Runs this Desktop holds, across every Guild, oldest first.
func (c *Client) DesktopRuns(ctx context.Context) ([]DesktopRun, error) {
	var out struct {
		Runs []DesktopRun `json:"runs"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/desktop/runs", 0, nil, &out); err != nil {
		return nil, err
	}
	return out.Runs, nil
}

// ClaimRun takes the queued Run id for this Desktop. A 409 means another
// Desktop of the person took it first, or its Agent cannot take it now.
func (c *Client) ClaimRun(ctx context.Context, id uint64) (DesktopRun, error) {
	var out struct {
		Run DesktopRun `json:"run"`
	}
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/runs/%d/claim", id), 0, nil, &out); err != nil {
		return DesktopRun{}, err
	}
	return out.Run, nil
}

func (c *Client) runState(ctx context.Context, path string, body any) (RunState, error) {
	var out struct {
		Run RunState `json:"run"`
	}
	if err := c.do(ctx, http.MethodPost, path, 0, body, &out); err != nil {
		return RunState{}, err
	}
	return out.Run, nil
}

// AppendRunEvents reports Run events in order by seq. The Bakery ignores a
// seq it already has, so a report that failed can be sent again; a 409
// means the Run is no longer running (cancelled, or lost).
func (c *Client) AppendRunEvents(ctx context.Context, id uint64, events []RunEvent) (RunState, error) {
	return c.runState(ctx, fmt.Sprintf("/api/runs/%d/events", id), map[string]any{"events": events})
}

// KeepLease tells the Bakery the Run is still running when there is
// nothing else to report.
func (c *Client) KeepLease(ctx context.Context, id uint64) (RunState, error) {
	return c.runState(ctx, fmt.Sprintf("/api/runs/%d/lease", id), nil)
}

// FinishRun ends the Run this Desktop ran. One cancelled meanwhile is
// answered as it is.
func (c *Client) FinishRun(ctx context.Context, id uint64, f Finish) (RunState, error) {
	return c.runState(ctx, fmt.Sprintf("/api/runs/%d/finish", id), f)
}

// RunsWatcher is told what the Desktop's stream of Runs says: the whole
// list whenever it changes, and each Run of this Desktop cancelled.
type RunsWatcher struct {
	Runs   func([]DesktopRun)
	Cancel func(runID uint64)
}

// The stream's backoff between reconnects, and how long it may stay
// silent (the Bakery pings far more often) before it counts as dropped.
var (
	streamBackoffMin = time.Second
	streamBackoffMax = 30 * time.Second
	streamSilence    = 90 * time.Second
)

// WatchRuns follows GET /api/desktop/runs/stream until ctx ends,
// reconnecting with a backoff of 1 s doubling to 30 s. It returns
// ErrSignedOut when the key stops working and ctx's error otherwise.
func (c *Client) WatchRuns(ctx context.Context, w RunsWatcher) error {
	backoff := streamBackoffMin
	for {
		connected, err := c.streamRuns(ctx, w)
		if errors.Is(err, ErrSignedOut) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if connected {
			backoff = streamBackoffMin
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, streamBackoffMax)
	}
}

// streamRuns reads one connection of the stream until it ends; connected
// says whether the Bakery accepted it.
func (c *Client) streamRuns(ctx context.Context, w RunsWatcher) (connected bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Address+"/api/desktop/runs/stream", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	// The stream outlives the client's request timeout: same transport,
	// no timeout; a silent connection is dropped by the watchdog below.
	res, err := (&http.Client{Transport: c.HTTP.Transport}).Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return false, ErrSignedOut
	case res.StatusCode != http.StatusOK:
		return false, &Error{Status: res.StatusCode}
	}
	watchdog := time.AfterFunc(streamSilence, cancel)
	defer watchdog.Stop()
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	var event string
	var data []string
	for scanner.Scan() {
		watchdog.Reset(streamSilence)
		line := scanner.Text()
		switch {
		case line == "":
			if event != "" || len(data) > 0 {
				dispatch(w, event, strings.Join(data, "\n"))
			}
			event, data = "", nil
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	return true, scanner.Err()
}

// RunStreamUpdate is one message off a Run's own stream
// (GET /api/runs/{id}/stream): "event" carries one of its Run events,
// "end" its final status.
type RunStreamUpdate struct {
	Kind   string
	Event  RunEvent
	Status string
}

// FollowRunStream reads the Run id's stream in the Guild guildID until ctx
// ends or the Run ends, calling onUpdate for each Run event and once more
// for "end". Used for a Run this Desktop did not claim: one of the
// person's other connected Desktops runs it.
func (c *Client) FollowRunStream(ctx context.Context, guildID, id uint64, onUpdate func(RunStreamUpdate)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/runs/%d/stream", c.Address, id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Bakery-Guild", strconv.FormatUint(guildID, 10))
	res, err := (&http.Client{Transport: c.HTTP.Transport}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrSignedOut
	case res.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case res.StatusCode != http.StatusOK:
		return &Error{Status: res.StatusCode}
	}
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	var event string
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if event != "" || len(data) > 0 {
				dispatchRunStream(onUpdate, event, strings.Join(data, "\n"))
			}
			event, data = "", nil
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	return scanner.Err()
}

func dispatchRunStream(onUpdate func(RunStreamUpdate), event, data string) {
	switch event {
	case "event":
		var e RunEvent
		if json.Unmarshal([]byte(data), &e) == nil {
			onUpdate(RunStreamUpdate{Kind: "event", Event: e})
		}
	case "end":
		var m struct {
			Status string `json:"status"`
		}
		if json.Unmarshal([]byte(data), &m) == nil {
			onUpdate(RunStreamUpdate{Kind: "end", Status: m.Status})
		}
	}
}

func dispatch(w RunsWatcher, event, data string) {
	switch event {
	case "runs":
		var m struct {
			Runs []DesktopRun `json:"runs"`
		}
		if json.Unmarshal([]byte(data), &m) == nil && w.Runs != nil {
			w.Runs(m.Runs)
		}
	case "cancel":
		var m struct {
			RunID uint64 `json:"run_id"`
		}
		if json.Unmarshal([]byte(data), &m) == nil && m.RunID != 0 && w.Cancel != nil {
			w.Cancel(m.RunID)
		}
	}
}
