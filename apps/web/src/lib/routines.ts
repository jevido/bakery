// The work context's Routines: recurring work that, on a Schedule or when
// someone runs it, creates an Execution Issue for its Agent. Every call
// answers in the Current guild. The run status labels are Paperclip's
// (ui/src/components/RoutineList.tsx; MIT, see NOTICE).
import { api } from "./api";
import type { Priority } from "./work";

export const routineStatuses = ["active", "paused", "archived"] as const;
export type RoutineStatus = (typeof routineStatuses)[number];

export const concurrencyPolicies = [
  "coalesce_if_active",
  "always_enqueue",
  "skip_if_active",
] as const;
export type ConcurrencyPolicy = (typeof concurrencyPolicies)[number];

export const catchUpPolicies = [
  "skip_missed",
  "enqueue_missed_with_cap",
] as const;
export type CatchUpPolicy = (typeof catchUpPolicies)[number];

/** Paperclip's one-line help for each policy. */
export const policyHelp: Record<ConcurrencyPolicy | CatchUpPolicy, string> = {
  coalesce_if_active:
    "If a run is already active, keep just one follow-up run queued.",
  always_enqueue:
    "Queue every trigger occurrence, even if the routine is already running.",
  skip_if_active: "Drop new trigger occurrences while a run is still active.",
  skip_missed:
    "Ignore windows that were missed while the scheduler or routine was paused.",
  enqueue_missed_with_cap:
    "Catch up missed schedule windows after recovery; sub-hourly schedules are combined into one catch-up run, slower schedules replay each missed window up to a cap.",
};

export const variableTypes = [
  "text",
  "textarea",
  "number",
  "boolean",
  "select",
  "date",
] as const;
export type VariableType = (typeof variableTypes)[number];

/** A placeholder `{{name}}` in a Routine's title or description, with its definition. */
export type RoutineVariable = {
  name: string;
  label: string | null;
  type: VariableType;
  default_value: string | number | boolean | null;
  required: boolean;
  options: string[];
};

/** The placeholders every Routine run fills in by itself, with an example of each. */
export const builtinVariables = [
  {
    name: "date",
    example: "2026-04-28",
    help: "The date the Routine runs, as YYYY-MM-DD (UTC).",
  },
  {
    name: "timestamp",
    example: "April 28, 2026 at 12:17 PM UTC",
    help: "The date and time the Routine runs, in words (UTC).",
  },
] as const;

// The API's matcher (domain.variableMatcher): a letter, then letters,
// digits or _, which Markdown may have written as \_.
const placeholder = /\{\{\s*([A-Za-z](?:\\_|[A-Za-z0-9_])*)\s*\}\}/g;

/** The placeholder names in the templates, in the order first seen. */
export function variableNames(...templates: string[]): string[] {
  const names: string[] = [];
  for (const t of templates) {
    for (const m of t.matchAll(placeholder)) {
      const name = m[1].replaceAll("\\_", "_");
      if (!names.includes(name)) names.push(name);
    }
  }
  return names;
}

/**
 * Paperclip's syncRoutineVariablesWithTemplate, as the API keeps them: one
 * definition per placeholder that is not built in, the existing one of that
 * name, else a required text one (date when the name ends in Date).
 */
export function syncVariables(
  title: string,
  description: string,
  existing: RoutineVariable[],
): RoutineVariable[] {
  const builtin: readonly string[] = builtinVariables.map((b) => b.name);
  return variableNames(title, description)
    .filter((name) => !builtin.includes(name))
    .map(
      (name) =>
        existing.find((v) => v.name === name) ?? {
          name,
          label: null,
          type: name.length > 4 && name.endsWith("Date") ? "date" : "text",
          default_value: null,
          required: true,
          options: [],
        },
    );
}

export type RoutineTriggerKind = "schedule" | "api" | "webhook";

export const signingModes = [
  "bearer",
  "hmac_sha256",
  "github_hmac",
  "none",
] as const;
export type SigningMode = (typeof signingModes)[number];

/** How a Webhook trigger's sender proves a delivery, in a word and a line. */
export const signingModeHelp: Record<
  SigningMode,
  { label: string; help: string }
> = {
  bearer: {
    label: "Bearer",
    help: "The sender puts the secret in an Authorization: Bearer header.",
  },
  hmac_sha256: {
    label: "HMAC-SHA256",
    help: "The sender signs the timestamp, a period and the exact body, and sends X-Bakery-Timestamp and X-Bakery-Signature.",
  },
  github_hmac: {
    label: "GitHub",
    help: "GitHub signs the body with the secret in X-Hub-Signature-256; paste the secret into its Secret field.",
  },
  none: {
    label: "None",
    help: "No check: anyone who knows the URL can fire it, so the URL is the secret.",
  },
};

/** The Replay window of a new hmac_sha256 Webhook trigger, and its bounds. */
export const replayWindow = { default: 300, min: 30, max: 86400 } as const;

export type RoutineTrigger = {
  id: number;
  kind: RoutineTriggerKind;
  label: string;
  enabled: boolean;
  cron_expression: string | null;
  timezone: string | null;
  next_run_at: string | null;
  last_fired_at: string | null;
  last_result: string | null;
  signing_mode: SigningMode | null;
  replay_window_sec: number | null;
  webhook_path: string | null;
  last_rotated_at: string | null;
  last_delivery: {
    status: "accepted" | "rejected";
    received_at: string;
  } | null;
};

/** A Webhook trigger's path and secret, answered only when it is made or rotated. */
export type SecretMaterial = { webhook_path: string; webhook_secret: string };

export type RoutineRunSource = "schedule" | "manual" | "api" | "webhook";
export type RoutineRunStatus =
  | "received"
  | "issue_created"
  | "coalesced"
  | "skipped"
  | "completed"
  | "failed";

export type RoutineRun = {
  id: number;
  routine: { id: number; title: string };
  source: RoutineRunSource;
  status: RoutineRunStatus;
  triggered_at: string;
  completed_at: string | null;
  failure_reason: string | null;
  trigger: { id: number; kind: RoutineTriggerKind; label: string } | null;
  issue: {
    id: number;
    identifier: string;
    title: string;
    status: string;
  } | null;
  variables: Record<string, unknown> | null;
  /** The Routine revision it ran; null for a run from before revisions. */
  routine_revision_id: number | null;
};

export type Routine = {
  id: number;
  title: string;
  description: string;
  project: { id: number; name: string } | null;
  goal: { id: number; title: string } | null;
  parent_issue: { id: number; identifier: string; title: string } | null;
  assignee_agent: { id: number; name: string; icon: string } | null;
  priority: Priority;
  status: RoutineStatus;
  concurrency_policy: ConcurrencyPolicy;
  catch_up_policy: CatchUpPolicy;
  variables: RoutineVariable[];
  last_triggered_at: string | null;
  latest_revision_id: number | null;
  latest_revision_number: number;
  created_at: string;
  updated_at: string;
  triggers: RoutineTrigger[];
  last_run: RoutineRun | null;
};

/** What a Routine revision keeps of a Routine trigger: never its secret, Next run or last delivery. */
export type SnapshotTrigger = {
  id: number;
  kind: RoutineTriggerKind;
  label: string;
  enabled: boolean;
  cron_expression: string;
  timezone: string;
  public_id: string;
  signing_mode: SigningMode | "";
  replay_window_sec: number;
};

/** What a Routine revision keeps; ids are 0 for none. */
export type RoutineSnapshot = {
  version: 1;
  routine: {
    id: number;
    guild_id: number;
    project_id: number;
    goal_id: number;
    parent_issue_id: number;
    assignee_agent_id: number;
    title: string;
    description: string;
    priority: Priority;
    status: RoutineStatus;
    concurrency_policy: ConcurrencyPolicy;
    catch_up_policy: CatchUpPolicy;
    variables: RoutineVariable[];
  };
  triggers: SnapshotTrigger[];
};

/** One kept version of a Routine and its triggers, numbered from 1. */
export type RoutineRevision = {
  id: number;
  routine_id: number;
  revision_number: number;
  title: string;
  description: string;
  snapshot: RoutineSnapshot;
  change_summary: string | null;
  restored_from_revision_id: number | null;
  author: {
    kind: "member" | "agent";
    id: number;
    name: string;
    icon?: string;
  } | null;
  created_at: string;
};

/** One Routine as GET /api/routines/{id} answers it, with its latest runs. */
export type RoutineDetail = Routine & { recent_runs: RoutineRun[] };

/** A Routine's editable fields; null clears a link. */
export type RoutineInput = {
  title: string;
  description: string;
  priority: Priority;
  status: RoutineStatus;
  concurrency_policy: ConcurrencyPolicy;
  catch_up_policy: CatchUpPolicy;
  project_id: number | null;
  goal_id: number | null;
  parent_issue_id: number | null;
  assignee_agent_id: number | null;
  variables: RoutineVariable[];
};

export type TriggerInput = {
  kind: RoutineTriggerKind;
  label: string;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
  signing_mode: SigningMode;
  replay_window_sec: number;
};

/** Paperclip's formatRoutineRunStatus: "issue_created" reads "issue created". */
export const routineRunStatusLabel = (s: string) => s.replaceAll("_", " ");

/** Paperclip's nextRoutineStatus: the toggle turns a Routine on or off. */
export const nextRoutineStatus = (
  status: RoutineStatus,
  enabled: boolean,
): RoutineStatus =>
  status === "archived" && enabled ? "active" : enabled ? "active" : "paused";

const routineOf = (r: { routine: RoutineDetail }) => r.routine;
const triggerOf = (r: { trigger: RoutineTrigger }) => r.trigger;
type WithSecret = { trigger: RoutineTrigger; secret_material?: SecretMaterial };

export const listRoutines = () =>
  api<{ routines: Routine[] }>("GET", "/routines").then((r) => r.routines);
export const getRoutine = (id: number) =>
  api<{ routine: RoutineDetail }>("GET", `/routines/${id}`).then(routineOf);
export const createRoutine = (
  input: Partial<RoutineInput> & { title: string },
) =>
  api<{ routine: RoutineDetail }>("POST", "/routines", input).then(routineOf);
export const updateRoutine = (
  id: number,
  patch: Partial<RoutineInput> & { base_revision_id?: number | null },
) =>
  api<{ routine: RoutineDetail }>("PATCH", `/routines/${id}`, patch).then(
    routineOf,
  );
/**
 * Runs it now: manual, or api when it names one of its api triggers, with
 * the values of its Routine variables; a variable left out takes its default.
 */
export const runRoutine = (
  id: number,
  triggerId?: number,
  variables?: Record<string, string | number | boolean>,
) =>
  api<{ routine_run: RoutineRun }>(
    "POST",
    `/routines/${id}/run`,
    triggerId || variables
      ? {
          ...(triggerId ? { trigger_id: triggerId } : {}),
          ...(variables ? { variables } : {}),
        }
      : undefined,
  ).then((r) => r.routine_run);
/** Adds a trigger; a Webhook trigger comes with its secret, shown this once. */
export const addTrigger = (
  routineId: number,
  input: Partial<TriggerInput> & { kind: RoutineTriggerKind },
) => api<WithSecret>("POST", `/routines/${routineId}/triggers`, input);
export const updateTrigger = (
  id: number,
  patch: Partial<Omit<TriggerInput, "kind">>,
) =>
  api<{ trigger: RoutineTrigger }>(
    "PATCH",
    `/routine-triggers/${id}`,
    patch,
  ).then(triggerOf);
/** A new secret for a Webhook trigger; the old one stops working at once. */
export const rotateTriggerSecret = (id: number) =>
  api<Required<WithSecret>>("POST", `/routine-triggers/${id}/rotate-secret`);
export const deleteTrigger = (id: number) =>
  api<void>("DELETE", `/routine-triggers/${id}`);
/** The Routine's revisions, newest first, at most 100. */
export const routineRevisions = (id: number) =>
  api<{ revisions: RoutineRevision[] }>(
    "GET",
    `/routines/${id}/revisions`,
  ).then((r) => r.revisions);

export const routineRuns = (id: number, limit = 50) =>
  api<{ routine_runs: RoutineRun[] }>(
    "GET",
    `/routines/${id}/runs?limit=${limit}`,
  ).then((r) => r.routine_runs);
/** The Current guild's Routine runs across the Routines the asker may view. */
export const recentRoutineRuns = (limit = 50) =>
  api<{ routine_runs: RoutineRun[] }>(
    "GET",
    `/routine-runs?limit=${limit}`,
  ).then((r) => r.routine_runs);

/**
 * Paperclip's summarizeRoutineSchedule (ui/src/components/RoutineOverview.tsx):
 * the Routine's enabled Schedules in a line, the first one's cron and time
 * zone under it, and the soonest Next run.
 */
export function summarizeSchedule(triggers: RoutineTrigger[]): {
  label: string;
  detail: string;
  nextRunAt: string | null;
} {
  const schedules = triggers.filter((t) => t.kind === "schedule" && t.enabled);
  const nextRunAt =
    schedules
      .map((t) => t.next_run_at)
      .filter((at): at is string => at !== null)
      .sort((a, b) => Date.parse(a) - Date.parse(b))[0] ?? null;
  if (schedules.length === 0) {
    const apis = triggers.filter((t) => t.kind === "api" && t.enabled).length;
    const webhooks = triggers.filter(
      (t) => t.kind === "webhook" && t.enabled,
    ).length;
    if (apis > 0)
      return {
        label: `${apis} API trigger${apis === 1 ? "" : "s"}`,
        detail: "Runs when the API is called",
        nextRunAt: null,
      };
    if (webhooks > 0)
      return {
        label: `${webhooks} webhook${webhooks === 1 ? "" : "s"}`,
        detail: "Runs when a webhook arrives",
        nextRunAt: null,
      };
    return {
      label: "No active schedule",
      detail: "Manual runs only",
      nextRunAt: null,
    };
  }
  const first = schedules[0];
  return {
    label:
      schedules.length === 1
        ? "1 active schedule"
        : `${schedules.length} active schedules`,
    detail: `${first.cron_expression ?? ""}${first.timezone ? ` · ${first.timezone}` : ""}`,
    nextRunAt,
  };
}

/**
 * The Routine's state as its page shows it, Paperclip's automationLabel: a
 * Routine without an Agent is a Draft whatever its status.
 */
export function routineState(
  r: Pick<Routine, "status" | "assignee_agent">,
): "archived" | "draft" | "active" | "paused" {
  if (r.status === "archived") return "archived";
  return r.assignee_agent ? r.status : "draft";
}

/** Paperclip's RoutineActivityRow labels for the routine.* Actions. */
const routineActionLabels: Record<string, string> = {
  "routine.created": "Routine created",
  "routine.updated": "Routine updated",
  "routine.archived": "Routine archived",
  "routine.trigger_created": "Trigger added",
  "routine.trigger_updated": "Trigger updated",
  "routine.trigger_deleted": "Trigger removed",
  "routine.trigger_secret_rotated": "Webhook secret rotated",
  "routine.run_triggered": "Routine started",
  "routine.webhook_rejected": "Webhook delivery rejected",
};
export const routineActionLabel = (action: string) =>
  routineActionLabels[action] ??
  action.replace(/^routine\./, "").replaceAll("_", " ");

/** Paperclip's summarizeEvent: the one line of what a routine.* event says. */
export function routineEventSummary(
  action: string,
  details: Record<string, unknown>,
): string {
  const str = (v: unknown) => (typeof v === "string" ? v : "");
  if (action === "routine.run_triggered") {
    const source = str(details.source);
    return `${source ? source[0].toUpperCase() + source.slice(1) : "Run"} · ${routineRunStatusLabel(str(details.status))}`;
  }
  if (
    action.startsWith("routine.trigger_") ||
    action === "routine.webhook_rejected"
  ) {
    const name =
      str(details.label) ||
      ({ api: "API", webhook: "Webhook" }[str(details.kind)] ?? "Schedule");
    if (action === "routine.webhook_rejected")
      return `${name} · ${str(details.reason)}`;
    const changes = details.changes as Record<string, unknown> | undefined;
    if (changes)
      return `${name} · ${Object.keys(changes)
        .map((k) => k.replaceAll("_", " "))
        .join(", ")}`;
    return details.cron_expression
      ? `${name} · ${details.cron_expression} · ${str(details.timezone)}`
      : name;
  }
  if (action === "routine.updated" && details.changes) {
    return Object.keys(details.changes as Record<string, unknown>)
      .map((k) => k.replaceAll("_", " "))
      .join(", ");
  }
  return "";
}
