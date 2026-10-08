// Package mcp is The Bakery's MCP server: what `bakery-desktop mcp` serves
// over stdio to the `claude` a Run starts, so the Agent reaches The
// Bakery's API as itself with its Run key. Its tools are listed in tools.go.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jevido/bakery/apps/desktop/bakery"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config is the Run the server acts for: the Bakery's address, the Run
// key, and the Guild, Agent and Run it belongs to.
type Config struct {
	APIURL  string
	APIKey  string
	GuildID uint64
	AgentID uint64
	RunID   uint64
}

// ConfigFromEnv reads the Config from BAKERY_API_URL, BAKERY_API_KEY,
// BAKERY_GUILD_ID, BAKERY_AGENT_ID and BAKERY_RUN_ID, naming every one that
// is missing or not a number.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	c := Config{APIURL: strings.TrimRight(getenv("BAKERY_API_URL"), "/"), APIKey: getenv("BAKERY_API_KEY")}
	var missing []string
	if c.APIURL == "" {
		missing = append(missing, "BAKERY_API_URL")
	}
	if c.APIKey == "" {
		missing = append(missing, "BAKERY_API_KEY")
	}
	for _, v := range []struct {
		name string
		to   *uint64
	}{{"BAKERY_GUILD_ID", &c.GuildID}, {"BAKERY_AGENT_ID", &c.AgentID}, {"BAKERY_RUN_ID", &c.RunID}} {
		n, err := strconv.ParseUint(getenv(v.name), 10, 64)
		if err != nil || n == 0 {
			missing = append(missing, v.name)
			continue
		}
		*v.to = n
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("the Bakery's MCP server needs %s in its environment", strings.Join(missing, ", "))
	}
	return c, nil
}

// NewServer is the server named bakery, with every tool, calling the API
// through client in cfg's Guild.
func NewServer(cfg Config, client *bakery.Client, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "bakery", Title: "The Bakery", Version: version}, nil)
	addTools(s, &api{client: client, guild: cfg.GuildID})
	return s
}

// Run serves the server on stdin and stdout until the client hangs up or
// ctx ends.
func Run(ctx context.Context, cfg Config, version string) error {
	return NewServer(cfg, bakery.New(cfg.APIURL, cfg.APIKey), version).Run(ctx, &mcp.StdioTransport{})
}

// api is the Bakery the tools call, in the Run's Guild.
type api struct {
	client *bakery.Client
	guild  uint64
}

// call sends one request and turns the answer into the tool's result: the
// API's JSON as text, or for a refusal an error carrying its status and
// message, which the SDK answers as a tool error (isError), never a crash.
func (a *api) call(ctx context.Context, method, path string, body any) (*mcp.CallToolResult, any, error) {
	data, err := a.client.Call(ctx, method, path, a.guild, body)
	var refused *bakery.Error
	switch {
	case errors.As(err, &refused):
		msg := refused.Message
		if msg == "" {
			msg = "refused"
		}
		return nil, nil, fmt.Errorf("the Bakery answered %d: %s", refused.Status, msg)
	case err != nil:
		return nil, nil, fmt.Errorf("the Bakery could not be reached: %w", err)
	}
	text := string(data)
	if text == "" {
		text = "{}"
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
