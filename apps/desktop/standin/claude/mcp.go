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

// mcpServer is one server of an --mcp-config file.
type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// callTools runs steps in order. When any is an [mcp …], it first starts
// the bakery server configFile names, as the CLI would (its own environment
// plus the server's env); a Run asked to call tools without a config fails,
// as the CLI's would for a tool it does not have. Every step prints its
// tool_use and tool_result; a [git …] that fails ends the Run as [fail] does.
func (s *session) callTools(configFile string, steps []step, slow bool) int {
	fail := func(msg string) int {
		s.result("error_during_execution", true, msg)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var session *mcp.ClientSession
	if tool := firstTool(steps); tool == "" {
		s.init()
	} else {
		if configFile == "" {
			s.init()
			return fail("No MCP server was configured (no --mcp-config), so mcp__bakery__" + tool + " is not a tool here.")
		}
		server, err := readServer(configFile, "bakery")
		if err != nil {
			s.init()
			return fail(err.Error())
		}
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
		session, err = client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			s.init(map[string]any{"name": "bakery", "status": "failed"})
			return fail("The bakery MCP server did not start: " + err.Error())
		}
		defer session.Close()
		s.init(map[string]any{"name": "bakery", "status": "connected"})
	}

	tools := 0
	for i, st := range steps {
		id := fmt.Sprintf("toolu_standin_%d", i+1)
		var content string
		var isError bool
		if st.git != nil {
			command := st.git.command()
			s.assistant(map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": command, "description": st.git.description()}})
			content, isError = st.git.run(ctx)
		} else {
			tools++
			s.assistant(map[string]any{"type": "tool_use", "id": id, "name": "mcp__bakery__" + st.tool, "input": st.args})
			res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: st.tool, Arguments: st.args})
			if err != nil {
				content, isError = err.Error(), true
			} else {
				content, isError = toolText(res), res.IsError
			}
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
		if st.git != nil && isError {
			return fail("The stand-in's " + st.git.command() + " failed: " + content)
		}
	}
	final := fmt.Sprintf("Called %d of The Bakery's tools.", tools)
	if tools == 0 {
		final = "Committed and pushed."
	}
	if slow {
		for i := 1; i <= 18; i++ {
			s.emitSlow(i)
		}
		s.delay = time.Second
		s.result("success", false, final)
		return 0
	}
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
