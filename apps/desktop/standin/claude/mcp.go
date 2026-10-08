package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpCall is one [mcp <tool> <json arguments>] in the prompt.
type mcpCall struct {
	tool string
	args map[string]any
}

// mcpCalls finds every [mcp …] in prompt, in order.
func mcpCalls(prompt string) ([]mcpCall, error) {
	var calls []mcpCall
	rest := prompt
	for {
		i := strings.Index(rest, "[mcp ")
		if i < 0 {
			return calls, nil
		}
		rest = strings.TrimLeft(rest[i+len("[mcp "):], " ")
		tool, after, _ := strings.Cut(rest, " ")
		dec := json.NewDecoder(strings.NewReader(after))
		var args map[string]any
		if err := dec.Decode(&args); err != nil {
			return nil, fmt.Errorf("the stand-in could not read the arguments of [mcp %s ...]: %v", tool, err)
		}
		after = strings.TrimLeft(after[dec.InputOffset():], " ")
		if !strings.HasPrefix(after, "]") {
			return nil, fmt.Errorf("the stand-in found no ] after [mcp %s ...]", tool)
		}
		calls = append(calls, mcpCall{tool: tool, args: args})
		rest = after[1:]
	}
}

// mcpServer is one server of an --mcp-config file.
type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// callTools starts the bakery server configFile names, as the CLI would
// (its own environment plus the server's env), and calls every tool in
// order, printing each tool_use and tool_result. A Run asked to call tools
// without a config fails, as the CLI's would for a tool it does not have.
func (s *session) callTools(configFile string, calls []mcpCall) int {
	fail := func(msg string) int {
		s.result("error_during_execution", true, msg)
		return 1
	}
	if configFile == "" {
		s.init()
		return fail("No MCP server was configured (no --mcp-config), so mcp__bakery__" + calls[0].tool + " is not a tool here.")
	}
	server, err := readServer(configFile, "bakery")
	if err != nil {
		s.init()
		return fail(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.Command(server.Command, server.Args...)
	cmd.Env = os.Environ()
	names := make([]string, 0, len(server.Env))
	for name := range server.Env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cmd.Env = append(cmd.Env, name+"="+server.Env[name])
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "claude-standin", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		s.init(map[string]any{"name": "bakery", "status": "failed"})
		return fail("The bakery MCP server did not start: " + err.Error())
	}
	defer session.Close()
	s.init(map[string]any{"name": "bakery", "status": "connected"})

	for i, call := range calls {
		id := fmt.Sprintf("toolu_standin_mcp_%d", i+1)
		s.assistant(map[string]any{"type": "tool_use", "id": id, "name": "mcp__bakery__" + call.tool, "input": call.args})
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: call.tool, Arguments: call.args})
		content, isError := "", false
		switch {
		case err != nil:
			content, isError = err.Error(), true
		default:
			content, isError = toolText(res), res.IsError
		}
		s.emit(map[string]any{
			"type": "user",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": id,
					"content":     content,
					"is_error":    isError,
				}},
			},
		}, s.delay)
	}
	final := fmt.Sprintf("Called %d of The Bakery's tools.", len(calls))
	s.assistant(text(final))
	s.result("success", false, final)
	return 0
}

func readServer(configFile, name string) (mcpServer, error) {
	b, err := os.ReadFile(configFile)
	if err != nil {
		return mcpServer{}, fmt.Errorf("reading --mcp-config: %w", err)
	}
	var config struct {
		MCPServers map[string]mcpServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &config); err != nil {
		return mcpServer{}, fmt.Errorf("reading --mcp-config: %w", err)
	}
	server, ok := config.MCPServers[name]
	if !ok || server.Command == "" {
		return mcpServer{}, errors.New("--mcp-config names no " + name + " server")
	}
	return server, nil
}

// toolText is a tool result's text, as the CLI shows it.
func toolText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}
