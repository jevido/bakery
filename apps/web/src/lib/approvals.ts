// The work context's Approvals: a Member asks the Board to decide, and those
// with the approve Permission approve, reject or ask for a revision. Every
// call answers in the Current guild. The labels and icons are Paperclip's
// (ui/src/components/ApprovalPayload.tsx; MIT, see NOTICE).
import { ShieldAlert, ShieldCheck, UserPlus } from "@lucide/svelte";
import { api } from "./api";
import type { Issue, WorkAgent, WorkMember } from "./work";

/**
 * Where an Approval stands; pending and revision_requested are Actionable.
 * cancelled is a hire whose Agent was terminated before a Decision.
 */
export type ApprovalStatus =
  "pending" | "revision_requested" | "approved" | "rejected" | "cancelled";

/** What a request_board_approval asks: Paperclip's BoardApprovalPayload. */
export type ApprovalPayload = {
  title: string;
  summary: string;
  recommended_action: string;
  next_action_on_approval: string;
  risks: string[];
};

/** What a hire_agent asks: the Agent waiting to be hired, Paperclip's HireAgentPayload. */
export type HirePayload = {
  agent_id: number;
  name: string;
  job: string;
  title: string;
  icon: string;
  capabilities: string;
  reports_to: { id: number; name: string } | null;
  roles: string[];
};

/** What a budget_override_required asks: the Budget at its Hard stop, Paperclip's BudgetOverridePayload. */
export type BudgetOverridePayload = {
  budget_id: number;
  scope_type: string;
  scope_id: number;
  scope_name: string;
  metric: string;
  window: string;
  threshold: string;
  amount: number;
  observed: number;
  warn_percent: number;
  guidance: string;
};

export type Approval = {
  id: number;
  type: string;
  status: ApprovalStatus;
  /** A HirePayload when type is hire_agent. */
  payload: ApprovalPayload;
  requester: WorkMember | null;
  requester_agent?: WorkAgent | null;
  decided_by: WorkMember | null;
  decision_note: string | null;
  decided_at: string | null;
  created_at: string;
  updated_at: string;
};

export type ApprovalComment = {
  id: number;
  body: string;
  author: WorkMember | null;
  author_agent?: WorkAgent | null;
  created_at: string;
};

export const approvalTypeLabels: Record<string, string> = {
  hire_agent: "Hire Agent",
  request_board_approval: "Board Approval",
  budget_override_required: "Budget Override",
};
export const approvalTypeLabel = (type: string) =>
  approvalTypeLabels[type] ?? type;
export const approvalTypeIcon = (type: string) =>
  type === "hire_agent"
    ? UserPlus
    : type === "budget_override_required"
      ? ShieldAlert
      : ShieldCheck;

/**
 * Whether the Approval is a Budget's Hard stop, decided only by resolving
 * its Budget incident on Costs, never by Approve or Reject.
 */
export const isBudgetOverride = (a: Pick<Approval, "type">) =>
  a.type === "budget_override_required";

/** The payload of a hire_agent Approval, else null. */
export const hirePayload = (a: Pick<Approval, "type" | "payload">) =>
  a.type === "hire_agent" ? (a.payload as unknown as HirePayload) : null;

/** The glossary's Jobs by key, as Paperclip's AGENT_ROLE_LABELS. */
export const jobLabels: Record<string, string> = {
  ceo: "CEO",
  cto: "CTO",
  cmo: "CMO",
  cfo: "CFO",
  security: "Security",
  engineer: "Engineer",
  designer: "Designer",
  pm: "PM",
  qa: "QA",
  devops: "DevOps",
  researcher: "Researcher",
  general: "General",
};
export const jobLabel = (job: string) => jobLabels[job] ?? job;

const firstText = (...values: unknown[]) => {
  for (const v of values)
    if (typeof v === "string" && v.trim()) return v.trim();
  return null;
};

/**
 * What the Approval is about: a hire's Agent name, else its title, summary or
 * recommended action. A hire's title is the Agent's Title, so the name goes
 * first here, where Paperclip's payload has no title for a hire.
 */
export const approvalSubject = (
  type: string,
  payload: Partial<ApprovalPayload & HirePayload & BudgetOverridePayload> | null,
) =>
  type === "hire_agent"
    ? firstText(payload?.name)
    : type === "budget_override_required"
      ? firstText(payload?.scope_name)
      : firstText(payload?.title, payload?.summary, payload?.recommended_action);

/** "Board Approval: <title>", "Hire Agent: <name>", as Paperclip's approvalLabel. */
export function approvalLabel(
  type: string,
  payload: Partial<ApprovalPayload & HirePayload & BudgetOverridePayload> | null,
): string {
  const subject = approvalSubject(type, payload);
  return subject
    ? `${approvalTypeLabel(type)}: ${subject}`
    : approvalTypeLabel(type);
}

export const isActionable = (a: Approval) =>
  a.status === "pending" || a.status === "revision_requested";

export const listApprovals = (status = "") =>
  api<{ approvals: Approval[] }>(
    "GET",
    `/approvals${status ? `?status=${status}` : ""}`,
  ).then((r) => r.approvals);
export const getApproval = (id: number) =>
  api<{ approval: Approval }>("GET", `/approvals/${id}`).then(
    (r) => r.approval,
  );
export const requestApproval = (payload: ApprovalPayload, issueIds: number[]) =>
  api<{ approval: Approval }>("POST", "/approvals", {
    type: "request_board_approval",
    payload,
    issue_ids: issueIds,
  }).then((r) => r.approval);
export const listApprovalIssues = (id: number) =>
  api<{ issues: Issue[] }>("GET", `/approvals/${id}/issues`).then(
    (r) => r.issues,
  );
export const listIssueApprovals = (issue: number | string) =>
  api<{ approvals: Approval[] }>("GET", `/issues/${issue}/approvals`).then(
    (r) => r.approvals,
  );

/** A Decision, with an optional Decision note. */
export type Decision = "approve" | "reject" | "request-revision";
export const decide = (id: number, decision: Decision, note = "") =>
  api<{ approval: Approval }>("POST", `/approvals/${id}/${decision}`, {
    decision_note: note,
  }).then((r) => r.approval);
/** Makes the Requester's own Approval pending again; without a payload it keeps the one it has. */
export const resubmitApproval = (id: number, payload?: ApprovalPayload) =>
  api<{ approval: Approval }>(
    "POST",
    `/approvals/${id}/resubmit`,
    payload ? { payload } : {},
  ).then((r) => r.approval);

export const listApprovalComments = (id: number) =>
  api<{ comments: ApprovalComment[] }>("GET", `/approvals/${id}/comments`).then(
    (r) => r.comments,
  );
export const writeApprovalComment = (id: number, body: string) =>
  api<{ comment: ApprovalComment }>("POST", `/approvals/${id}/comments`, {
    body,
  }).then((r) => r.comment);
