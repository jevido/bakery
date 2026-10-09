package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			t.Setenv("BAKERY_STANDIN_STATE", t.TempDir())
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
			if strings.Contains(c.prompt, "[limit]") {
				if r, _ := last["result"].(string); !strings.HasPrefix(r, "You've hit your limit · resets ") || !strings.HasSuffix(r, " (UTC)") {
					t.Fatalf("limit result %v", last["result"])
				}
				info := ls[len(ls)-2]["rate_limit_info"].(map[string]any)
				if info["status"] != "rejected" || info["resetsAt"].(float64) <= float64(time.Now().Unix()) {
					t.Fatalf("limit rate_limit_event %v", info)
				}
			}
			if strings.Contains(c.prompt, "[crash]") && errOut.Len() == 0 {
				t.Fatal("crash printed nothing on stderr")
			}
		})
	}
}

func TestLimitOncePerFirstLine(t *testing.T) {
	withoutAPIKey(t)
	t.Setenv("BAKERY_STANDIN_STATE", t.TempDir())
	t.Setenv("BAKERY_STANDIN_LIMIT_RESET", "2m")
	prompt := "ENG-9: [limit] once\n\nGo."
	var out, errOut bytes.Buffer
	if code := run(append(append([]string{}, baseArgs...), prompt), strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("first run exit %d, want 1", code)
	}
	ls := lines(t, out.String())
	at := int64(ls[len(ls)-2]["rate_limit_info"].(map[string]any)["resetsAt"].(float64))
	if d := time.Until(time.Unix(at, 0)); d < time.Minute || d > 2*time.Minute {
		t.Fatalf("reset %v ahead, want about 2m", d)
	}
	out.Reset()
	if code := run(append(append([]string{}, baseArgs...), prompt), strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("second run exit %d, want 0", code)
	}
	ls = lines(t, out.String())
	if last := ls[len(ls)-1]; last["is_error"] != false {
		t.Fatalf("second run %v", last)
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

func TestStepsAreReadInOrder(t *testing.T) {
	calls, err := steps(`Do it [mcp bakeryCheckoutIssue {"issueId":"DEF-1"}] [git commit web/index.html hello there] [git push] then [mcp bakeryAddComment {"issueId":"DEF-1","body":"a ] in it"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 || calls[0].tool != "bakeryCheckoutIssue" ||
		calls[1].git == nil || calls[1].git.file != "web/index.html" || calls[1].git.text != "hello there" ||
		calls[2].git == nil || calls[2].git.file != "" ||
		calls[3].tool != "bakeryAddComment" || calls[3].args["body"] != "a ] in it" {
		t.Fatalf("calls %+v", calls)
	}
	for _, bad := range []string{`[mcp bakeryMe {"x":1}`, `[git push`, `[git rebase main]`, `[git commit ]`} {
		if _, err := steps(bad); err == nil {
			t.Fatalf("no error for %q", bad)
		}
	}
}

// git runs git in dir and fails the test when it fails.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// worktree makes a bare repository with one commit on main and a clone of
// it on branch bakery/def-1, and runs the test inside the clone.
func worktree(t *testing.T) (bare string) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Coder", "GIT_AUTHOR_EMAIL": "agent-3@bakery.test",
		"GIT_COMMITTER_NAME": "Person", "GIT_COMMITTER_EMAIL": "person@example.test",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
	} {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	bare = filepath.Join(root, "app.git")
	git(t, root, "init", "--bare", "-b", "main", bare)
	clone := filepath.Join(root, "clone")
	git(t, root, "clone", bare, clone)
	os.WriteFile(filepath.Join(clone, "README.md"), []byte("app\n"), 0o644)
	git(t, clone, "add", "README.md")
	git(t, clone, "commit", "-m", "first")
	git(t, clone, "push", "origin", "HEAD:main")
	git(t, clone, "checkout", "-b", "bakery/def-1")
	t.Chdir(clone)
	return bare
}

func TestGitCommitAndPush(t *testing.T) {
	withoutAPIKey(t)
	bare := worktree(t)
	var out, errOut bytes.Buffer
	code := run(append(baseArgs, "DEF-1: say hello [git commit web/index.html hello] [git push]"), nil, &out, &errOut)
	ls := lines(t, out.String())
	if last := ls[len(ls)-1]; code != 0 || last["subtype"] != "success" {
		t.Fatalf("exit %d, last %v, stderr %q", code, last, errOut.String())
	}
	if got := git(t, bare, "show", "bakery/def-1:web/index.html"); got != "hello" {
		t.Fatalf("pushed file %q", got)
	}
	if got := git(t, bare, "log", "-1", "--format=%an <%ae> %s", "bakery/def-1"); got != "Coder <agent-3@bakery.test> stand-in: web/index.html" {
		t.Fatalf("pushed commit %q", got)
	}
	var bash int
	for _, l := range ls {
		msg, _ := l["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		for _, c := range content {
			if c := c.(map[string]any); c["type"] == "tool_use" && c["name"] == "Bash" {
				bash++
			}
		}
	}
	if bash != 2 {
		t.Fatalf("%d Bash tool calls, want 2", bash)
	}
}

func TestFailingGitFailsTheRun(t *testing.T) {
	withoutAPIKey(t)
	bare := worktree(t)
	os.RemoveAll(bare)
	var out, errOut bytes.Buffer
	code := run(append(baseArgs, "DEF-1 [git commit a.txt x] [git push]"), nil, &out, &errOut)
	ls := lines(t, out.String())
	last := ls[len(ls)-1]
	if code != 1 || last["subtype"] != "error_during_execution" || !strings.Contains(last["result"].(string), "git push") {
		t.Fatalf("exit %d, last %v", code, last)
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

func TestSkillsSaysWhatItFound(t *testing.T) {
	withoutAPIKey(t)
	dir := t.TempDir()
	for _, name := range []string{"release-notes", "bakery"} {
		if err := os.MkdirAll(filepath.Join(dir, ".claude", "skills", name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	if code := run(append(append([]string{}, baseArgs...), "--add-dir", dir, "--add-dir", t.TempDir(), "ENG-3 [skills]"), nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"skills: bakery, release-notes"`) {
		t.Fatalf("output %s", out.String())
	}
}
