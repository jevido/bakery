package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// step is one [mcp <tool> <json arguments>] or [git …] in the prompt.
type step struct {
	tool string
	args map[string]any
	git  *gitStep
}

// gitStep is a [git commit <file> <text>] (file set) or a [git push].
type gitStep struct {
	file, text string
}

// steps finds every [mcp …] and [git …] in prompt, in prompt order.
func steps(prompt string) ([]step, error) {
	var out []step
	rest := prompt
	for {
		m, g := strings.Index(rest, "[mcp "), strings.Index(rest, "[git ")
		switch {
		case m < 0 && g < 0:
			return out, nil
		case g >= 0 && (m < 0 || g < m):
			end := strings.Index(rest[g:], "]")
			if end < 0 {
				return nil, fmt.Errorf("the stand-in found no ] after [git ...]")
			}
			words := strings.TrimSpace(rest[g+len("[git ") : g+end])
			rest = rest[g+end+1:]
			verb, after, _ := strings.Cut(words, " ")
			switch verb {
			case "push":
				out = append(out, step{git: &gitStep{}})
			case "commit":
				file, text, _ := strings.Cut(strings.TrimSpace(after), " ")
				if file == "" {
					return nil, fmt.Errorf("the stand-in's [git commit] needs a file")
				}
				out = append(out, step{git: &gitStep{file: file, text: text}})
			default:
				return nil, fmt.Errorf("the stand-in knows [git commit <file> <text>] and [git push], not [git %s]", words)
			}
		default:
			r := strings.TrimLeft(rest[m+len("[mcp "):], " ")
			tool, after, _ := strings.Cut(r, " ")
			dec := json.NewDecoder(strings.NewReader(after))
			var args map[string]any
			if err := dec.Decode(&args); err != nil {
				return nil, fmt.Errorf("the stand-in could not read the arguments of [mcp %s ...]: %v", tool, err)
			}
			after = strings.TrimLeft(after[dec.InputOffset():], " ")
			if !strings.HasPrefix(after, "]") {
				return nil, fmt.Errorf("the stand-in found no ] after [mcp %s ...]", tool)
			}
			out = append(out, step{tool: tool, args: args})
			rest = after[1:]
		}
	}
}

// firstTool is the first [mcp …] step's tool, or "" when there is none.
func firstTool(steps []step) string {
	for _, s := range steps {
		if s.git == nil {
			return s.tool
		}
	}
	return ""
}

// command is the shell command a Bash tool call would show for g.
func (g *gitStep) command() string {
	if g.file == "" {
		return "git push -u origin HEAD"
	}
	return fmt.Sprintf("printf '%%s\\n' %q > %s && git add %s && git commit -m %q", g.text, g.file, g.file, "stand-in: "+g.file)
}

func (g *gitStep) description() string {
	if g.file == "" {
		return "Push the Agent branch"
	}
	return "Commit " + g.file
}

// run does g in the working directory and answers git's output and whether
// it failed.
func (g *gitStep) run(ctx context.Context) (string, bool) {
	git := func(args ...string) (string, bool) {
		out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
		text := strings.TrimSpace(string(out))
		if err != nil {
			return strings.TrimSpace(text + "\n" + err.Error()), true
		}
		return text, false
	}
	if g.file == "" {
		return git("push", "-u", "origin", "HEAD")
	}
	if filepath.IsAbs(g.file) || strings.Contains(g.file, "..") {
		return "the stand-in only writes files inside its working directory", true
	}
	if dir := filepath.Dir(g.file); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err.Error(), true
		}
	}
	if err := os.WriteFile(g.file, []byte(g.text+"\n"), 0o644); err != nil {
		return err.Error(), true
	}
	if out, failed := git("add", "--", g.file); failed {
		return out, true
	}
	return git("commit", "-m", "stand-in: "+g.file)
}
