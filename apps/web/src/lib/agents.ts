// The agents context's Agents: the AI workers a Guild hires, their Org
// chart and their hire. Every call answers in the Current guild. Jobs and
// their labels are Paperclip's (packages/shared; MIT, see NOTICE).
import { api } from "./api";

export { jobLabel, jobLabels } from "./approvals";

/** An Agent status; running and error come with Runs. */
export type AgentStatus = "pending_approval" | "idle" | "paused" | "terminated";

/** The Agents list's tabs: all leaves out the terminated ones. */
export type AgentFilter =
  "all" | "active" | "paused" | "pending" | "terminated";

export type AgentRole = {
  id: number;
  name: string;
  color: string;
  position: number;
};

export type Agent = {
  id: number;
  name: string;
  job: string;
  job_label: string;
  title: string;
  icon: string;
  capabilities: string;
  status: AgentStatus;
  reports_to: { id: number; name: string } | null;
  hirer: { id: number; name: string } | null;
  roles: AgentRole[];
  approval_id: number | null;
  /** Whether the asker may edit, pause, resume, terminate and re-role it. */
  can_manage: boolean;
  created_at: string;
  updated_at: string;
  paused_at: string | null;
  terminated_at: string | null;
};

/** An Agent in the Org chart, with its direct reports. */
export type OrgNode = {
  id: number;
  name: string;
  job: string;
  job_label: string;
  title: string;
  icon: string;
  status: AgentStatus;
  reports: OrgNode[];
};

/** A hire as typed; reports_to null reports to no one. */
export type Hire = {
  name: string;
  job: string;
  title: string;
  icon: string;
  capabilities: string;
  reports_to: number | null;
  role_ids: number[];
};

export const listAgents = (status: AgentFilter = "all") =>
  api<{ agents: Agent[] }>("GET", `/agents?status=${status}`).then(
    (r) => r.agents,
  );
export const getAgent = (id: number) =>
  api<{ agent: Agent }>("GET", `/agents/${id}`).then((r) => r.agent);
export const hireAgent = (hire: Hire) =>
  api<{ agent: Agent; approval_id: number }>("POST", "/agents", hire);
export const getOrg = () =>
  api<{ org: OrgNode[] }>("GET", "/org").then((r) => r.org);

/** Any of an Agent's editable fields; reports_to null reports to no one. */
export type AgentPatch = Partial<Omit<Hire, "role_ids">>;

const agentOf = (r: { agent: Agent }) => r.agent;
export const editAgent = (id: number, patch: AgentPatch) =>
  api<{ agent: Agent }>("PATCH", `/agents/${id}`, patch).then(agentOf);
export const pauseAgent = (id: number) =>
  api<{ agent: Agent }>("POST", `/agents/${id}/pause`).then(agentOf);
export const resumeAgent = (id: number) =>
  api<{ agent: Agent }>("POST", `/agents/${id}/resume`).then(agentOf);
export const terminateAgent = (id: number) =>
  api<{ agent: Agent }>("POST", `/agents/${id}/terminate`).then(agentOf);
export const addAgentRole = (id: number, roleID: number) =>
  api<{ agent: Agent }>("PUT", `/agents/${id}/roles/${roleID}`).then(agentOf);
export const removeAgentRole = (id: number, roleID: number) =>
  api<{ agent: Agent }>("DELETE", `/agents/${id}/roles/${roleID}`).then(
    agentOf,
  );
