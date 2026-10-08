package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// withoutAPIKey runs the test as the Runner would start the stand-in: with
// ANTHROPIC_API_KEY removed, whatever the developer's shell has set.
func withoutAPIKey(t *testing.T) {
	t.Helper()
	if v, ok := os.LookupEnv("ANTHROPIC_API_KEY"); ok {
		os.Unsetenv("ANTHROPIC_API_KEY")
		t.Cleanup(func() { os.Setenv("ANTHROPIC_API_KEY", v) })
	}
	t.Setenv("BAKERY_STANDIN_DELAY", "0")
}

func lines(t *testing.T, out string) []map[string]any {
	t.Helper()
	var parsed []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("line is not JSON: %q: %v", l, err)
		}
		parsed = append(parsed, m)
	}
	return parsed
}

var baseArgs = []string{"--print", "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions"}

func TestAnswers(t *testing.T) {
	cases := []struct {
		prompt      string
		code        int
		lastType    string
		lastSubtype string
	}{
		{"ENG-1: hello\n\nSay hi.", 0, "result", "success"},
		{"ENG-2: [fail] break", 1, "result", "error_during_execution"},
		{"ENG-3: [limit] out of usage", 1, "result", "success"},
		{"ENG-4: [crash] go away", 1, "assistant", ""},
	}
	for _, c := range cases {
		t.Run(c.prompt, func(t *testing.T) {
			withoutAPIKey(t)
			var out, errOut bytes.Buffer
			code := run(append(append([]string{}, baseArgs...), c.prompt), strings.NewReader(""), &out, &errOut)
			if code != c.code {
				t.Fatalf("exit %d, want %d (stderr %q)", code, c.code, errOut.String())
			}
			ls := lines(t, out.String())
			if ls[0]["type"] != "system" || ls[0]["subtype"] != "init" {
				t.Fatalf("first line is not system/init: %v", ls[0])
			}
			last := ls[len(ls)-1]
			if last["type"] != c.lastType {
				t.Fatalf("last line type %v, want %s", last["type"], c.lastType)
			}
			if c.lastType == "result" {
				if last["subtype"] != c.lastSubtype {
					t.Fatalf("result subtype %v, want %s", last["subtype"], c.lastSubtype)
				}
				if _, ok := last["usage"].(map[string]any)["output_tokens"]; !ok {
					t.Fatalf("result has no usage: %v", last)
				}
			}
			if c.code == 0 && !strings.Contains(last["result"].(string), "ENG-1: hello") {
				t.Fatalf("result does not quote the prompt's first line: %v", last["result"])
			}
			if strings.Contains(c.prompt, "[limit]") && last["result"] != limitMessage {
				t.Fatalf("limit result %v", last["result"])
			}
			if strings.Contains(c.prompt, "[crash]") && errOut.Len() == 0 {
				t.Fatal("crash printed nothing on stderr")
			}
		})
	}
}

func TestToolCallAndResult(t *testing.T) {
	withoutAPIKey(t)
	var out, errOut bytes.Buffer
	if code := run(append(append([]string{}, baseArgs...), "ENG-1: hello"), nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var sawTool, sawResult, sawThinking bool
	for _, l := range lines(t, out.String()) {
		msg, _ := l["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		for _, c := range content {
			switch c.(map[string]any)["type"] {
			case "tool_use":
				sawTool = true
			case "tool_result":
				sawResult = true
			case "thinking":
				sawThinking = true
			}
		}
	}
	if !sawTool || !sawResult || !sawThinking {
		t.Fatalf("tool_use %v, tool_result %v, thinking %v", sawTool, sawResult, sawThinking)
	}
}

func TestPromptOnStdin(t *testing.T) {
	withoutAPIKey(t)
	var out, errOut bytes.Buffer
	if code := run(append(append([]string{}, baseArgs...), "-"), strings.NewReader("ENG-9: from stdin\n"), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	ls := lines(t, out.String())
	if !strings.Contains(ls[len(ls)-1]["result"].(string), "ENG-9: from stdin") {
		t.Fatalf("result %v", ls[len(ls)-1]["result"])
	}
}

func TestRefusesAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x")
	var out, errOut bytes.Buffer
	code := run(append(append([]string{}, baseArgs...), "hi"), nil, &out, &errOut)
	if code != 3 || !strings.Contains(errOut.String(), "ANTHROPIC_API_KEY must not be set") || out.Len() != 0 {
		t.Fatalf("exit %d, stderr %q, stdout %q", code, errOut.String(), out.String())
	}
}

func TestRefusesWithoutStreamJSONFlags(t *testing.T) {
	withoutAPIKey(t)
	for _, args := range [][]string{
		{"--output-format", "stream-json", "--verbose", "hi"},
		{"--print", "--verbose", "hi"},
		{"--print", "--output-format", "stream-json", "hi"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, nil, &out, &errOut); code != 2 || errOut.Len() == 0 {
			t.Fatalf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}

func TestMCPCallsAreReadInOrder(t *testing.T) {
	calls, err := mcpCalls(`Do it [mcp bakeryCheckoutIssue {"issueId":"DEF-1"}] then [mcp bakeryAddComment {"issueId":"DEF-1","body":"a ] in it"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0].tool != "bakeryCheckoutIssue" || calls[1].tool != "bakeryAddComment" || calls[1].args["body"] != "a ] in it" {
		t.Fatalf("calls %+v", calls)
	}
	if _, err := mcpCalls(`[mcp bakeryMe {"x":1}`); err == nil {
		t.Fatal("no error for a missing ]")
	}
}

func TestMCPNeedsAConfig(t *testing.T) {
	withoutAPIKey(t)
	var out, errOut bytes.Buffer
	if code := run(append(baseArgs, "--strict-mcp-config", "hi"), strings.NewReader(""), &out, &errOut); code != 2 ||
		!strings.Contains(errOut.String(), "--strict-mcp-config requires --mcp-config") {
		t.Fatalf("strict without a config: exit %d, %q", code, errOut.String())
	}
	out.Reset()
	code := run(append(baseArgs, `ENG-5 [mcp bakeryMe {}]`), strings.NewReader(""), &out, &errOut)
	ls := lines(t, out.String())
	last := ls[len(ls)-1]
	if code != 1 || last["type"] != "result" || last["is_error"] != true || !strings.Contains(last["result"].(string), "--mcp-config") {
		t.Fatalf("exit %d, last %v", code, last)
	}
}
