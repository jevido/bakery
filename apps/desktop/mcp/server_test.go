package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jevido/bakery/apps/desktop/bakery"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seen is one request the stand-in Bakery got.
type seen struct {
	Method, Path, Auth, Guild, Body string
}

// connect starts the server against a stand-in Bakery answering with
// answer and returns a connected client and what the Bakery saw.
func connect(t *testing.T, answer http.HandlerFunc) (*mcp.ClientSession, *[]seen) {
	t.Helper()
	var got []seen
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, seen{r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"), r.Header.Get("Bakery-Guild"), string(b)})
		answer(w, r)
	}))
	t.Cleanup(api.Close)
	cfg := Config{APIURL: api.URL, APIKey: "bky_run_test", GuildID: 7, AgentID: 3, RunID: 11}
	server := NewServer(cfg, bakery.New(cfg.APIURL, cfg.APIKey), "test")
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, &got
}

func ok(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}
}

func text(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) != 1 {
		t.Fatalf("content = %v", r.Content)
	}
	return r.Content[0].(*mcp.TextContent).Text
}

func TestListsEveryTool(t *testing.T) {
	cs, _ := connect(t, ok(`{}`))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
		if strings.Contains(strings.ToLower(tl.Description), "paperclip") || strings.Contains(strings.ToLower(tl.Description), "company") {
			t.Errorf("%s's description %q is not in The Bakery's words", tl.Name, tl.Description)
		}
	}
	want := []string{
		"bakeryMe", "bakeryInbox", "bakeryListAgents", "bakeryGetAgent", "bakeryListIssues", "bakeryGetIssue",
		"bakeryListComments", "bakeryListDocuments", "bakeryGetDocument", "bakeryListDocumentRevisions",
		"bakeryListProjects", "bakeryGetProject", "bakeryListGoals", "bakeryGetGoal", "bakeryListApprovals",
		"bakeryGetApproval", "bakeryCreateIssue", "bakeryUpdateIssue", "bakeryCheckoutIssue", "bakeryReleaseIssue",
		"bakeryAddComment", "bakeryUpsertIssueDocument", "bakeryCreateApproval", "bakeryAddApprovalComment",
		"bakeryApiRequest",
	}
	slices.Sort(names)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

func TestAddCommentCallsTheAPIAsTheRun(t *testing.T) {
	cs, got := connect(t, ok(`{"comment":{"id":5,"body":"On it"}}`))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bakeryAddComment", Arguments: map[string]any{"issueId": "DEF-12", "body": "On it"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", text(t, res))
	}
	if s := text(t, res); s != `{"comment":{"id":5,"body":"On it"}}` {
		t.Errorf("text = %s", s)
	}
	want := seen{"POST", "/api/issues/DEF-12/comments", "Bearer bky_run_test", "7", `{"body":"On it"}`}
	if len(*got) != 1 || (*got)[0] != want {
		t.Errorf("requests = %+v, want %+v", *got, want)
	}
}

func TestUpdateIssueSendsOnlyTheFieldsGiven(t *testing.T) {
	cs, got := connect(t, ok(`{}`))
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bakeryUpdateIssue", Arguments: map[string]any{"issueId": "4", "status": "in_review"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r := (*got)[0]; r.Method != "PATCH" || r.Path != "/api/issues/4" || r.Body != `{"status":"in_review"}` {
		t.Errorf("request = %+v", r)
	}
}

func TestCheckoutExpectsOpenStatusesByDefault(t *testing.T) {
	cs, got := connect(t, ok(`{}`))
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bakeryCheckoutIssue", Arguments: map[string]any{"issueId": "DEF-1"},
	}); err != nil {
		t.Fatal(err)
	}
	var body struct {
		ExpectedStatuses []string `json:"expected_statuses"`
	}
	r := (*got)[0]
	if err := json.Unmarshal([]byte(r.Body), &body); err != nil || r.Path != "/api/issues/DEF-1/checkout" ||
		!slices.Equal(body.ExpectedStatuses, []string{"todo", "backlog", "blocked"}) {
		t.Errorf("request = %+v", r)
	}
}

func TestRefusalIsAToolError(t *testing.T) {
	cs, _ := connect(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"message":"You may not do that in this guild."}`)
	})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bakeryAddComment", Arguments: map[string]any{"issueId": "1", "body": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s := text(t, res); !res.IsError || s != "the Bakery answered 403: You may not do that in this guild." {
		t.Errorf("result = %v %q", res.IsError, s)
	}
}

func TestAPIRequestStaysUnderAPI(t *testing.T) {
	cs, got := connect(t, ok(`{}`))
	for _, path := range []string{"/health", "/api/../admin", "api/issues"} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "bakeryApiRequest", Arguments: map[string]any{"method": "GET", "path": path},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Errorf("%s: not refused", path)
		}
	}
	if len(*got) != 0 {
		t.Errorf("requests = %+v", *got)
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bakeryApiRequest", Arguments: map[string]any{"method": "post", "path": "/api/issues/1/read", "jsonBody": `{"a":1}`},
	})
	if err != nil || res.IsError {
		t.Fatalf("%v %v", err, res)
	}
	if r := (*got)[0]; r.Method != "POST" || r.Path != "/api/issues/1/read" || r.Body != `{"a":1}` {
		t.Errorf("request = %+v", r)
	}
}

func TestConfigFromEnvNamesWhatIsMissing(t *testing.T) {
	env := map[string]string{"BAKERY_API_URL": "http://b/", "BAKERY_API_KEY": "k", "BAKERY_GUILD_ID": "2", "BAKERY_RUN_ID": "x"}
	_, err := ConfigFromEnv(func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "BAKERY_AGENT_ID, BAKERY_RUN_ID") || strings.Contains(err.Error(), "GUILD") {
		t.Errorf("err = %v", err)
	}
	env["BAKERY_AGENT_ID"], env["BAKERY_RUN_ID"] = "3", "4"
	c, err := ConfigFromEnv(func(k string) string { return env[k] })
	if err != nil || c != (Config{APIURL: "http://b", APIKey: "k", GuildID: 2, AgentID: 3, RunID: 4}) {
		t.Errorf("config = %+v, %v", c, err)
	}
}
