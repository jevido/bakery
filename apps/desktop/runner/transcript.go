package runner

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jevido/bakery/apps/desktop/bakery"
)

// The most of a tool result, a text and a tool call's input a Run event
// carries; the Bakery takes at most 256 KiB per payload.
const (
	maxToolResult = 16 << 10
	maxText       = 64 << 10
	maxToolInput  = 64 << 10
)

// keepStderr is how many of the last stderr lines a Run that ended
// without a result line reports as its error.
const keepStderr = 10

// keepTexts is how many of the last assistant texts Finish searches for a
// Subscription limit's wording.
const keepTexts = 3

// unreadableReset is how long a Subscription limit whose text names no
// readable reset time is waited out, as Paperclip's bounded retry does at
// its first step.
const unreadableReset = time.Hour

// The Subscription limit's wording, ported from Paperclip's
// CLAUDE_PROVIDER_QUOTA_RE and CLAUDE_EXTRA_USAGE_RESET_RE
// (packages/adapters/claude-local/src/server/parse.ts, MIT, see NOTICE).
const limitWords = `(?:you(?:'|’)ve\s+hit\s+your\s+(?:\w+\s+)?limit|session\s+limit\s+(?:reached|exceeded)|out\s+of\s+extra\s+usage|extra\s+usage\b|claude\s+usage\s+limit\s+reached|5[-\s]?hour\s+limit\s+reached|weekly\s+limit\s+reached|usage\s+limit\s+reached|usage\s+cap\s+reached|servicequotaexceededexception)`

var (
	limitRE      = regexp.MustCompile(`(?i)` + limitWords)
	limitResetRE = regexp.MustCompile(`(?i)` + limitWords + `[\s\S]{0,120}?\bresets?\s+(?:at\s+)?([^\n()]+?)(?:\s*\(([^)]+)\))?(?:[.!]|\n|$)`)
	clockRE      = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*([ap])\.?\s*m\.?`)
)

// Event is one Run event before it is numbered.
type Event struct {
	Kind    string
	Payload json.RawMessage
}

// The payloads of the Run event kinds (docs/domain/contexts/agents).
type (
	initPayload struct {
		SessionID string `json:"session_id"`
		Model     string `json:"model"`
	}
	textPayload struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated,omitempty"`
	}
	toolCallPayload struct {
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	toolResultPayload struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
		IsError   bool   `json:"is_error"`
		Truncated bool   `json:"truncated,omitempty"`
	}
)

// Result is claude's final `result` line, also the `result` Run event's
// payload.
type Result struct {
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	NumTurns     int64   `json:"num_turns"`
	DurationMS   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	SessionID    string  `json:"session_id"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// Transcript turns what `claude --output-format stream-json` prints into
// Run events, and keeps what finishing the Run needs.
type Transcript struct {
	SessionID string
	Model     string
	// Result is the result line, nil until claude printed one.
	Result *Result
	// LimitResetsAt is the Limit reset of a `rejected` rate_limit_event,
	// nil until claude printed one.
	LimitResetsAt *time.Time
	stderr        []string
	texts         []string
	// now is the clock Finish reads a reset time against, nil meaning
	// time.Now.
	now func() time.Time
}

// line is one stream-json line, with only what the Run events use.
type line struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	RateLimitInfo *struct {
		Status        string `json:"status"`
		ResetsAt      int64  `json:"resetsAt"`
		RateLimitType string `json:"rateLimitType"`
	} `json:"rate_limit_info"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Stdout turns one stdout line into its Run events: none for an empty
// line or one the Transcript does not show, `system` for one that is not
// JSON.
func (t *Transcript) Stdout(raw string) []Event {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var l line
	if err := json.Unmarshal([]byte(raw), &l); err != nil || l.Type == "" {
		return []Event{event("system", textOf(raw))}
	}
	switch l.Type {
	case "system":
		if l.Subtype != "init" {
			return nil
		}
		t.SessionID, t.Model = l.SessionID, l.Model
		return []Event{event("init", initPayload{SessionID: l.SessionID, Model: l.Model})}
	case "assistant":
		var out []Event
		for _, b := range blocks(l.Message.Content) {
			switch b.Type {
			case "text":
				if b.Text != "" {
					t.keepText(b.Text)
					out = append(out, event("assistant", textOf(b.Text)))
				}
			case "thinking":
				if b.Thinking != "" {
					out = append(out, event("thinking", textOf(b.Thinking)))
				}
			case "tool_use":
				input := b.Input
				if len(input) == 0 {
					input = json.RawMessage(`{}`)
				}
				if len(input) > maxToolInput {
					input, _ = json.Marshal(map[string]any{"truncated": true, "preview": cut(string(input), maxToolInput)})
				}
				out = append(out, event("tool_call", toolCallPayload{ID: b.ID, Name: b.Name, Input: input}))
			}
		}
		return out
	case "user":
		var out []Event
		for _, b := range blocks(l.Message.Content) {
			if b.Type != "tool_result" {
				continue
			}
			content := contentText(b.Content)
			p := toolResultPayload{ToolUseID: b.ToolUseID, Content: content, IsError: b.IsError}
			if len(content) > maxToolResult {
				p.Content, p.Truncated = cut(content, maxToolResult), true
			}
			out = append(out, event("tool_result", p))
		}
		return out
	case "rate_limit_event":
		// Shown nowhere: it only tells whether the login hit its
		// Subscription limit, and when that resets.
		if i := l.RateLimitInfo; i != nil && i.Status == "rejected" && i.ResetsAt > 0 {
			at := time.Unix(i.ResetsAt, 0).UTC()
			t.LimitResetsAt = &at
		}
		return nil
	case "result":
		var r Result
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			return []Event{event("system", textOf(raw))}
		}
		t.Result = &r
		if r.SessionID != "" && t.SessionID == "" {
			t.SessionID = r.SessionID
		}
		short := r
		short.Result = cut(r.Result, maxText)
		return []Event{event("result", short)}
	}
	return nil
}

// Stderr turns one stderr line into a `stderr` Run event and keeps it for
// a Run that ends without a result.
func (t *Transcript) Stderr(raw string) []Event {
	raw = strings.TrimRight(raw, "\r\n")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	t.stderr = append(t.stderr, raw)
	if len(t.stderr) > keepStderr {
		t.stderr = t.stderr[len(t.stderr)-keepStderr:]
	}
	return []Event{event("stderr", textOf(raw))}
}

// Finish is how the Run ended, from the result line, or from the exit
// code and the last stderr lines when claude printed none. A Run whose
// login hit its Subscription limit ends `limited` with the Limit reset,
// whatever the exit code.
func (t *Transcript) Finish(exitCode int) bakery.Finish {
	code := exitCode
	f := bakery.Finish{ExitCode: &code}
	r := t.Result
	if r != nil {
		f.Usage = bakery.Usage{
			InputTokens: r.Usage.InputTokens, CachedInputTokens: r.Usage.CacheReadInputTokens,
			OutputTokens: r.Usage.OutputTokens, Turns: r.NumTurns,
			CostEquivalentUSD: r.TotalCostUSD, DurationMS: r.DurationMS,
		}
	}
	if at := t.limit(); at != nil {
		f.Status, f.LimitResetsAt = "limited", at
		if r != nil {
			f.Error = cut(strings.TrimSpace(r.Result), 4000)
		}
		if f.Error == "" {
			f.Error = "claude hit the subscription limit"
		}
		return f
	}
	if r != nil {
		if !r.IsError {
			f.Status = "succeeded"
			return f
		}
		f.Status, f.Error = "failed", cut(strings.TrimSpace(r.Result), 4000)
		if f.Error == "" {
			f.Error = "claude ended with " + r.Subtype
		}
		return f
	}
	f.Status = "failed"
	f.Error = fmt.Sprintf("claude exited with code %d and no result", exitCode)
	if len(t.stderr) > 0 {
		f.Error += ":\n" + strings.Join(t.stderr, "\n")
	}
	f.Error = cut(f.Error, 4000)
	return f
}

func (t *Transcript) keepText(s string) {
	t.texts = append(t.texts, s)
	if len(t.texts) > keepTexts {
		t.texts = t.texts[len(t.texts)-keepTexts:]
	}
}

// limit is the Limit reset when claude hit the Subscription limit, nil
// when it did not: the rejected rate_limit_event's, else one read from
// the limit's wording in the result, the last texts or stderr, else an
// hour ahead when the wording names no readable time.
func (t *Transcript) limit() *time.Time {
	if t.LimitResetsAt != nil {
		return t.LimitResetsAt
	}
	var parts []string
	if t.Result != nil {
		parts = append(parts, t.Result.Result)
	}
	parts = append(parts, t.texts...)
	parts = append(parts, t.stderr...)
	haystack := strings.Join(parts, "\n")
	if !limitRE.MatchString(haystack) {
		return nil
	}
	now := time.Now()
	if t.now != nil {
		now = t.now()
	}
	if m := limitResetRE.FindStringSubmatch(haystack); m != nil {
		if at, ok := resetClock(m[1], m[2], now); ok {
			return &at
		}
	}
	at := now.Add(unreadableReset)
	return &at
}

// resetClock is the next time after now that the clock text ("2:30am",
// "11pm") names, in the zone named in the parentheses, else in local time,
// as Paperclip's parseClaudeResetClockTime reads it.
func resetClock(clock, zone string, now time.Time) (time.Time, bool) {
	m := clockRE.FindStringSubmatch(strings.Join(strings.Fields(clock), " "))
	if m == nil {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(m[1])
	minute := 0
	if m[2] != "" {
		minute, _ = strconv.Atoi(m[2])
	}
	if hour < 1 || hour > 12 || minute > 59 {
		return time.Time{}, false
	}
	hour %= 12
	if strings.EqualFold(m[3], "p") {
		hour += 12
	}
	loc := time.Local
	if z := strings.TrimSpace(zone); z != "" {
		if l, err := time.LoadLocation(z); err == nil {
			loc = l
		}
	}
	n := now.In(loc)
	at := time.Date(n.Year(), n.Month(), n.Day(), hour, minute, 0, 0, loc)
	if !at.After(n) {
		at = at.AddDate(0, 0, 1)
	}
	return at, true
}

func event(kind string, payload any) Event {
	b, _ := json.Marshal(payload)
	return Event{Kind: kind, Payload: b}
}

func textOf(s string) textPayload {
	if len(s) > maxText {
		return textPayload{Text: cut(s, maxText), Truncated: true}
	}
	return textPayload{Text: s}
}

func blocks(raw json.RawMessage) []block {
	var bs []block
	if json.Unmarshal(raw, &bs) == nil {
		return bs
	}
	// A message whose content is a plain string is one text block.
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return []block{{Type: "text", Text: s}}
	}
	return nil
}

// contentText is a tool result's content as text: a string as it is, a
// list of blocks as their texts, anything else as its JSON.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var bs []block
	if json.Unmarshal(raw, &bs) == nil {
		parts := make([]string, 0, len(bs))
		for _, b := range bs {
			if b.Type == "text" {
				parts = append(parts, b.Text)
			} else {
				parts = append(parts, "["+b.Type+"]")
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

// cut shortens s to at most n bytes without splitting a rune.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
