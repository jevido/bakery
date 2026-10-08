// Command claude-standin answers like `claude --print --output-format
// stream-json --verbose <prompt>`, so the Runner's tests and the e2e checks
// never spend a person's subscription. It is not part of the Desktop app's
// binary; `task desktop:standin` builds it to apps/desktop/bin/claude-standin.
//
// Words in the prompt choose what it does:
//
//	[slow]   20 lines, 1 s apart (for cancel checks)
//	[fail]   an error_during_execution result, exit 1
//	[crash]  two lines, something on stderr, exit 1 with no result
//	[limit]  the CLI's usage-limit message, exit 1
//	[env]    says its working directory and its git and Worktree variables
//	         (GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL, BAKERY_WORKTREE,
//	         BAKERY_BRANCH, BAKERY_BASE_BRANCH) in one text line, then a
//	         short successful Run
//	[mcp <tool> <json arguments>]
//	         calls tool on the bakery MCP server --mcp-config names, printing
//	         the tool_use and tool_result lines the CLI would (several are
//	         called in order), then a short successful Run, or [slow]'s
//	         steps first when the prompt also says [slow] (so a check sees
//	         what the tools changed while the Run is still live)
//
// Anything else prints a short successful Run. BAKERY_STANDIN_DELAY sets the
// pause between lines (milliseconds or a Go duration, default 300 ms).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// limitMessage is the wording the real CLI prints when a subscription's
// window runs out (Paperclip's claude-local parse tests carry the same text).
const limitMessage = "You've hit your limit · resets 2:30am (UTC)"

type options struct {
	print          bool
	outputFormat   string
	verbose        bool
	permissionMode string
	model          string
	mcpConfig      string
	strictMCP      bool
	prompt         string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if _, ok := os.LookupEnv("ANTHROPIC_API_KEY"); ok {
		fmt.Fprintln(stderr, "ANTHROPIC_API_KEY must not be set")
		return 3
	}
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	switch {
	case !opts.print:
		fmt.Fprintln(stderr, "Error: the stand-in only answers in --print mode")
		return 2
	case opts.outputFormat != "stream-json":
		fmt.Fprintln(stderr, "Error: the stand-in only speaks --output-format stream-json")
		return 2
	case !opts.verbose:
		fmt.Fprintln(stderr, "Error: When using --print, --output-format=stream-json requires --verbose")
		return 2
	}
	if opts.strictMCP && opts.mcpConfig == "" {
		fmt.Fprintln(stderr, "Error: --strict-mcp-config requires --mcp-config")
		return 2
	}
	if opts.prompt == "" || opts.prompt == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "Error: reading the prompt:", err)
			return 2
		}
		opts.prompt = string(b)
	}
	if strings.TrimSpace(opts.prompt) == "" {
		fmt.Fprintln(stderr, "Error: Input must be provided either through stdin or as a prompt argument when using --print")
		return 2
	}

	s := &session{
		out:   stdout,
		delay: delayFromEnv(),
		id:    fmt.Sprintf("standin-%d", time.Now().UnixNano()),
		model: opts.model,
		start: time.Now(),
	}
	if s.model == "" {
		s.model = "claude-standin"
	}
	if calls, err := mcpCalls(opts.prompt); err != nil || len(calls) > 0 {
		if err != nil {
			s.init()
			s.result("error_during_execution", true, err.Error())
			return 1
		}
		return s.callTools(opts.mcpConfig, calls, strings.Contains(opts.prompt, "[slow]"))
	}
	return s.answer(opts.prompt, stderr)
}

func parseArgs(args []string) (options, error) {
	var o options
	value := func(i *int, name string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("option %s needs a value", name)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, inline, hasInline := strings.Cut(a, "=")
		var err error
		get := func() string {
			if hasInline {
				return inline
			}
			var v string
			v, err = value(&i, name)
			return v
		}
		switch name {
		case "--print", "-p":
			o.print = true
		case "--verbose":
			o.verbose = true
		case "--output-format":
			o.outputFormat = get()
		case "--permission-mode":
			o.permissionMode = get()
		case "--model":
			o.model = get()
		case "--mcp-config":
			o.mcpConfig = get()
		case "--append-system-prompt", "--max-turns", "--allowedTools", "--disallowedTools", "--resume", "--session-id", "--add-dir":
			get()
		case "--strict-mcp-config":
			o.strictMCP = true
		case "--dangerously-skip-permissions", "--include-partial-messages":
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return o, fmt.Errorf("unknown option '%s'", a)
			}
			o.prompt = a
		}
		if err != nil {
			return o, err
		}
	}
	return o, nil
}

func delayFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("BAKERY_STANDIN_DELAY"))
	if v == "" {
		return 300 * time.Millisecond
	}
	if ms, err := strconv.Atoi(v); err == nil {
		return time.Duration(ms) * time.Millisecond
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return 300 * time.Millisecond
}

type session struct {
	out   io.Writer
	delay time.Duration
	id    string
	model string
	start time.Time
	lines int
	turns int
}

// emit prints one stream-json line, pausing before every line but the first
// so a live Transcript has something to show.
func (s *session) emit(line map[string]any, pause time.Duration) {
	if s.lines > 0 {
		time.Sleep(pause)
	}
	line["session_id"] = s.id
	b, _ := json.Marshal(line)
	fmt.Fprintf(s.out, "%s\n", b)
	s.lines++
}

func (s *session) init(mcpServers ...map[string]any) {
	cwd, _ := os.Getwd()
	s.emit(map[string]any{
		"type":           "system",
		"subtype":        "init",
		"model":          s.model,
		"cwd":            cwd,
		"tools":          []string{"Bash", "Edit", "Glob", "Grep", "Read", "Write"},
		"permissionMode": "default",
		"apiKeySource":   "none",
		"mcp_servers":    append([]map[string]any{}, mcpServers...),
	}, s.delay)
}

func (s *session) assistant(content ...map[string]any) {
	s.turns++
	s.emit(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"id":      fmt.Sprintf("msg_standin_%d", s.lines),
			"type":    "message",
			"role":    "assistant",
			"model":   s.model,
			"content": content,
		},
	}, s.delay)
}

func text(t string) map[string]any { return map[string]any{"type": "text", "text": t} }

func (s *session) result(subtype string, isError bool, result string) {
	// The real CLI reports the subscription's window before its result.
	s.emit(map[string]any{
		"type": "rate_limit_event",
		"rate_limit_info": map[string]any{
			"status":        "allowed",
			"resetsAt":      s.start.Add(5 * time.Hour).Unix(),
			"rateLimitType": "five_hour",
			"utilization":   0.1,
		},
	}, s.delay)
	line := map[string]any{
		"type":            "result",
		"subtype":         subtype,
		"is_error":        isError,
		"duration_ms":     time.Since(s.start).Milliseconds() + s.delay.Milliseconds(),
		"duration_api_ms": time.Since(s.start).Milliseconds(),
		"num_turns":       s.turns,
		"result":          result,
		"total_cost_usd":  0.0123 * float64(max(s.turns, 1)),
		"usage": map[string]any{
			"input_tokens":                1200 * max(s.turns, 1),
			"output_tokens":               180 * max(s.turns, 1),
			"cache_creation_input_tokens": 2400,
			"cache_read_input_tokens":     9600 * max(s.turns, 1),
		},
	}
	s.emit(line, s.delay)
}

func (s *session) answer(prompt string, stderr io.Writer) int {
	first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(prompt), "\n", 2)[0])
	switch {
	case strings.Contains(prompt, "[slow]"):
		s.init()
		for i := 1; i <= 18; i++ {
			s.emitSlow(i)
		}
		s.delay = time.Second
		s.result("success", false, "Took my time over "+first+".")
		return 0
	case strings.Contains(prompt, "[crash]"):
		s.init()
		s.assistant(text("Starting on " + first + "."))
		fmt.Fprintln(stderr, "standin: crashed on purpose ([crash] in the prompt)")
		return 1
	case strings.Contains(prompt, "[limit]"):
		s.init()
		s.assistant(text(limitMessage))
		s.result("success", true, limitMessage)
		return 1
	case strings.Contains(prompt, "[env]"):
		s.init()
		cwd, _ := os.Getwd()
		said := []string{"cwd=" + cwd}
		for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "BAKERY_WORKTREE", "BAKERY_BRANCH", "BAKERY_BASE_BRANCH"} {
			said = append(said, name+"="+os.Getenv(name))
		}
		s.assistant(text(strings.Join(said, "\n")))
		s.result("success", false, "Said where I am.")
		return 0
	case strings.Contains(prompt, "[fail]"):
		s.init()
		s.assistant(text("Starting on " + first + "."))
		s.result("error_during_execution", true, "The stand-in failed on purpose ([fail] in the prompt).")
		return 1
	}

	s.init()
	s.assistant(text("I'll start by reading the README to see what this is about."))
	s.assistant(map[string]any{
		"type":  "tool_use",
		"id":    "toolu_standin_1",
		"name":  "Read",
		"input": map[string]any{"file_path": "README.md"},
	})
	s.emit(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": "toolu_standin_1",
				"content":     "# Stand-in\n\nNothing to see here: this is the claude stand-in.",
				"is_error":    false,
			}},
		},
	}, s.delay)
	s.assistant(map[string]any{"type": "thinking", "thinking": "The README is a placeholder, so the Issue is all there is to go on."})
	final := "Done with " + first + "."
	s.assistant(text(final))
	s.result("success", false, final)
	return 0
}

// emitSlow prints one of [slow]'s steps a second after the previous line.
func (s *session) emitSlow(i int) {
	s.turns++
	s.emit(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"id":      fmt.Sprintf("msg_standin_%d", s.lines),
			"type":    "message",
			"role":    "assistant",
			"model":   s.model,
			"content": []map[string]any{text(fmt.Sprintf("Step %d of 18.", i))},
		},
	}, time.Second)
}
