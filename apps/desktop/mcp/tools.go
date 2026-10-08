package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The tools are Paperclip's MCP surface in The Bakery's words, limited to
// the routes open to Agents: who the Agent is, its Inbox, Agents,
// Issues with their Comments and documents, Projects, Goals and Approvals,
// Checkout and Release, and bakeryApiRequest for any other /api route.

type none struct{}

type issueRef struct {
	IssueID string `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
}

type documentRef struct {
	IssueID string `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
	Key     string `json:"key" jsonschema:"the document's key, such as plan"`
}

type agentRef struct {
	AgentID uint64 `json:"agentId" jsonschema:"the Agent's id"`
}

type projectRef struct {
	ProjectID uint64 `json:"projectId" jsonschema:"the Project's id"`
}

type goalRef struct {
	GoalID uint64 `json:"goalId" jsonschema:"the Goal's id"`
}

type approvalRef struct {
	ApprovalID uint64 `json:"approvalId" jsonschema:"the Approval's id"`
}

type listAgents struct {
	Status string `json:"status,omitempty" jsonschema:"only Agents with this status, such as active or paused"`
}

type listIssues struct {
	Status   string `json:"status,omitempty" jsonschema:"comma-separated statuses: backlog, todo, in_progress, in_review, blocked, done, cancelled"`
	Priority string `json:"priority,omitempty" jsonschema:"comma-separated priorities: critical, high, medium, low"`
	Assignee string `json:"assignee,omitempty" jsonschema:"agent:<id> for an Agent's Issues, or a Member's id"`
	Q        string `json:"q,omitempty" jsonschema:"text to search for in the title, identifier and description"`
}

type listApprovals struct {
	Status string `json:"status,omitempty" jsonschema:"only Approvals with this status: pending, revision_requested, approved, rejected or cancelled"`
}

// issueFields are an Issue's fields as creating and changing one sends
// them; an empty one is left out, so it is left as it is.
type issueFields struct {
	Title           string   `json:"title,omitempty" jsonschema:"the Issue's title"`
	Description     *string  `json:"description,omitempty" jsonschema:"the Issue's description, in Markdown"`
	Status          string   `json:"status,omitempty" jsonschema:"backlog, todo, in_progress, in_review, blocked, done or cancelled"`
	Priority        string   `json:"priority,omitempty" jsonschema:"critical, high, medium or low"`
	AssigneeID      *uint64  `json:"assignee_id,omitempty" jsonschema:"the Member to assign it to"`
	AssigneeAgentID *uint64  `json:"assignee_agent_id,omitempty" jsonschema:"the Agent to assign it to"`
	ProjectID       *uint64  `json:"project_id,omitempty" jsonschema:"the Project it belongs to"`
	GoalID          *uint64  `json:"goal_id,omitempty" jsonschema:"the Goal it serves"`
	ParentID        *uint64  `json:"parent_id,omitempty" jsonschema:"the parent Issue's id, for a sub-issue"`
	BlockedByIDs    []uint64 `json:"blocked_by_ids,omitempty" jsonschema:"the ids of the Issues that block it"`
}

type createIssue struct {
	issueFields
}

type updateIssue struct {
	IssueID string `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
	issueFields
}

type checkoutIssue struct {
	IssueID          string   `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
	ExpectedStatuses []string `json:"expectedStatuses,omitempty" jsonschema:"the statuses the Issue may be in to be checked out; todo, backlog and blocked when left out"`
}

type addComment struct {
	IssueID string `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
	Body    string `json:"body" jsonschema:"the Comment, in Markdown"`
}

type upsertDocument struct {
	IssueID        string `json:"issueId" jsonschema:"the Issue's id or its Issue identifier, such as DEF-12"`
	Key            string `json:"key" jsonschema:"the document's key, such as plan"`
	Title          string `json:"title,omitempty" jsonschema:"the document's title"`
	Body           string `json:"body" jsonschema:"the document, in Markdown"`
	ChangeSummary  string `json:"changeSummary,omitempty" jsonschema:"what this revision changes"`
	BaseRevisionID uint64 `json:"baseRevisionId,omitempty" jsonschema:"the revision this one is based on; a newer one in between is a conflict"`
}

type createApproval struct {
	Type     string          `json:"type,omitempty" jsonschema:"the Approval's type; request_board_approval when left out"`
	Payload  json.RawMessage `json:"payload" jsonschema:"what the Board decides on: title, summary, recommended_action, next_action_on_approval and risks"`
	IssueIDs []uint64        `json:"issueIds,omitempty" jsonschema:"the ids of the Issues it is about"`
}

type addApprovalComment struct {
	ApprovalID uint64 `json:"approvalId" jsonschema:"the Approval's id"`
	Body       string `json:"body" jsonschema:"the Comment, in Markdown"`
}

type apiRequest struct {
	Method   string `json:"method" jsonschema:"GET, POST, PUT, PATCH or DELETE"`
	Path     string `json:"path" jsonschema:"the path, starting with /api/, with its query if any"`
	JSONBody string `json:"jsonBody,omitempty" jsonschema:"the request body as a JSON string"`
}

// tool adds one tool whose call is the request route builds from its input.
func tool[In any](s *mcp.Server, a *api, name, description string, route func(In) (method, path string, body any, err error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		method, path, body, err := route(in)
		if err != nil {
			return nil, nil, err
		}
		return a.call(ctx, method, path, body)
	})
}

// get is a route with no body.
func get[In any](path func(In) string) func(In) (string, string, any, error) {
	return func(in In) (string, string, any, error) { return "GET", path(in), nil, nil }
}

func id(n uint64) string { return strconv.FormatUint(n, 10) }

// query is "?k=v&…" for the values that are not empty, or "".
func query(kv ...string) string {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func issuePath(issue string) string { return "/api/issues/" + url.PathEscape(issue) }

func documentPath(d documentRef) string {
	return issuePath(d.IssueID) + "/documents/" + url.PathEscape(d.Key)
}

func addTools(s *mcp.Server, a *api) {
	tool(s, a, "bakeryMe", "Get the Agent this Run acts as: its Guild, Roles, Permissions and Run", get(func(none) string { return "/api/agents/me" }))
	tool(s, a, "bakeryInbox", "Get the Agent's Inbox: the open Issues assigned to it", get(func(none) string { return "/api/agents/me/inbox" }))
	tool(s, a, "bakeryListAgents", "List the Agents in the Guild", get(func(in listAgents) string { return "/api/agents" + query("status", in.Status) }))
	tool(s, a, "bakeryGetAgent", "Get a single Agent by id", get(func(in agentRef) string { return "/api/agents/" + id(in.AgentID) }))
	tool(s, a, "bakeryListIssues", "List the Guild's Issues with optional filters", get(func(in listIssues) string {
		return "/api/issues" + query("status", in.Status, "priority", in.Priority, "assignee", in.Assignee, "q", in.Q)
	}))
	tool(s, a, "bakeryGetIssue", "Get a single Issue by id or Issue identifier", get(func(in issueRef) string { return issuePath(in.IssueID) }))
	tool(s, a, "bakeryListComments", "List an Issue's Comments", get(func(in issueRef) string { return issuePath(in.IssueID) + "/comments" }))
	tool(s, a, "bakeryListDocuments", "List an Issue's documents", get(func(in issueRef) string { return issuePath(in.IssueID) + "/documents" }))
	tool(s, a, "bakeryGetDocument", "Get one of an Issue's documents by key", get(documentPath))
	tool(s, a, "bakeryListDocumentRevisions", "List the revisions of an Issue's document", get(func(in documentRef) string { return documentPath(in) + "/revisions" }))
	tool(s, a, "bakeryListProjects", "List the Guild's Projects", get(func(none) string { return "/api/projects" }))
	tool(s, a, "bakeryGetProject", "Get a Project by id", get(func(in projectRef) string { return "/api/projects/" + id(in.ProjectID) }))
	tool(s, a, "bakeryListGoals", "List the Guild's Goals", get(func(none) string { return "/api/goals" }))
	tool(s, a, "bakeryGetGoal", "Get a Goal by id", get(func(in goalRef) string { return "/api/goals/" + id(in.GoalID) }))
	tool(s, a, "bakeryListApprovals", "List the Guild's Approvals", get(func(in listApprovals) string { return "/api/approvals" + query("status", in.Status) }))
	tool(s, a, "bakeryGetApproval", "Get an Approval by id", get(func(in approvalRef) string { return "/api/approvals/" + id(in.ApprovalID) }))

	tool(s, a, "bakeryCreateIssue", "Create a new Issue, or with parent_id a sub-issue", func(in createIssue) (string, string, any, error) {
		return "POST", "/api/issues", in.issueFields, nil
	})
	tool(s, a, "bakeryUpdateIssue", "Change an Issue: only the fields given change", func(in updateIssue) (string, string, any, error) {
		return "PATCH", issuePath(in.IssueID), in.issueFields, nil
	})
	tool(s, a, "bakeryCheckoutIssue", "Check an Issue out for this Run, which moves it to in_progress; another live Run holding it is a conflict", func(in checkoutIssue) (string, string, any, error) {
		expected := in.ExpectedStatuses
		if len(expected) == 0 {
			expected = []string{"todo", "backlog", "blocked"}
		}
		return "POST", issuePath(in.IssueID) + "/checkout", map[string]any{"expected_statuses": expected}, nil
	})
	tool(s, a, "bakeryReleaseIssue", "Release this Run's Checkout of an Issue", func(in issueRef) (string, string, any, error) {
		return "POST", issuePath(in.IssueID) + "/release", struct{}{}, nil
	})
	tool(s, a, "bakeryAddComment", "Add a Comment to an Issue", func(in addComment) (string, string, any, error) {
		return "POST", issuePath(in.IssueID) + "/comments", map[string]string{"body": in.Body}, nil
	})
	tool(s, a, "bakeryUpsertIssueDocument", "Create or update an Issue's document; each save is a new revision", func(in upsertDocument) (string, string, any, error) {
		return "PUT", documentPath(documentRef{IssueID: in.IssueID, Key: in.Key}), map[string]any{
			"title": in.Title, "body": in.Body, "change_summary": in.ChangeSummary, "base_revision_id": in.BaseRevisionID,
		}, nil
	})
	tool(s, a, "bakeryCreateApproval", "Ask the Board for an Approval, optionally about one or more Issues", func(in createApproval) (string, string, any, error) {
		typ := in.Type
		if typ == "" {
			typ = "request_board_approval"
		}
		return "POST", "/api/approvals", map[string]any{"type": typ, "payload": in.Payload, "issue_ids": in.IssueIDs}, nil
	})
	tool(s, a, "bakeryAddApprovalComment", "Add a Comment to an Approval", func(in addApprovalComment) (string, string, any, error) {
		return "POST", "/api/approvals/" + id(in.ApprovalID) + "/comments", map[string]string{"body": in.Body}, nil
	})

	tool(s, a, "bakeryApiRequest", "Make a JSON request to any other route of The Bakery's /api, for what no other tool does", func(in apiRequest) (string, string, any, error) {
		method := strings.ToUpper(in.Method)
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return "", "", nil, errors.New("method must be GET, POST, PUT, PATCH or DELETE")
		}
		if !strings.HasPrefix(in.Path, "/api/") || strings.Contains(in.Path, "..") {
			return "", "", nil, errors.New("path must start with /api/ and must not contain '..'")
		}
		var body any
		if in.JSONBody != "" {
			if !json.Valid([]byte(in.JSONBody)) {
				return "", "", nil, errors.New("jsonBody is not valid JSON")
			}
			body = json.RawMessage(in.JSONBody)
		}
		return method, in.Path, body, nil
	})
}
