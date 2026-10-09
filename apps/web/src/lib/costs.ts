// Costs and Budgets of the Guild's Agents' Runs: what the API answers at
// /api/costs/* and /api/budgets/overview, read from the Runs' Run usage.
import { api } from './api'
import { runTime, tokens } from './format'

/** What a set of Runs used; tokens is input plus output, as the tokens Budget metric counts them. */
export type Figures = {
  input_tokens: number
  cached_input_tokens: number
  output_tokens: number
  tokens: number
  runs: number
  run_time_ms: number
  cost_equivalent_usd: number
}

export type AgentCosts = Figures & { agent: { id: number; name: string; icon: string; status: string } }
/** A null project is the Runs without one. */
export type ProjectCosts = Figures & { project: { id: number; name: string } | null }

export type BudgetMetric = 'tokens' | 'runs' | 'run_time'
export type BudgetWindow = 'calendar_month_utc' | 'lifetime'
export type BudgetScope = { type: 'guild' | 'agent' | 'project'; id: number; name: string }

export type Budget = {
  id: number
  scope: BudgetScope
  metric: BudgetMetric
  window: BudgetWindow
  amount: number
  warn_percent: number
  hard_stop: boolean
  notify: boolean
  observed: number
  status: 'ok' | 'warning' | 'hard_stop'
  window_start: string | null
  window_end: string | null
  updated_at: string
}

export type BudgetIncident = {
  id: number
  budget_id: number
  scope: BudgetScope
  metric: BudgetMetric
  window: BudgetWindow
  threshold: 'soft' | 'hard'
  amount: number
  observed: number
  status: string
  approval_id: number | null
  window_start: string | null
  window_end: string | null
  created_at: string
  resolved_at: string | null
}

export type BudgetOverview = { budgets: Budget[]; incidents: BudgetIncident[]; paused_agents: number; stopped_projects: number }

/** A Costs range: RFC 3339 times, to exclusive; an empty one is unbounded. */
export type CostRange = { from: string; to: string }

function query(r: CostRange): string {
  const q = new URLSearchParams()
  if (r.from) q.set('from', r.from)
  if (r.to) q.set('to', r.to)
  return q.size ? `?${q}` : ''
}

export function costSummary(r: CostRange): Promise<Figures> {
  return api<Figures>('GET', `/costs/summary${query(r)}`)
}

export async function costsByAgent(r: CostRange): Promise<AgentCosts[]> {
  return (await api<{ agents: AgentCosts[] }>('GET', `/costs/by-agent${query(r)}`)).agents
}

export async function costsByProject(r: CostRange): Promise<ProjectCosts[]> {
  return (await api<{ projects: ProjectCosts[] }>('GET', `/costs/by-project${query(r)}`)).projects
}

export function budgetOverview(): Promise<BudgetOverview> {
  return api<BudgetOverview>('GET', '/budgets/overview')
}

/** An amount of a Budget metric for people: tokens compact, run time (seconds on the wire) as a duration. */
export function budgetAmount(metric: string, n: number): string {
  if (metric === 'tokens') return `${tokens(n)} tokens`
  if (metric === 'runs') return `${n.toLocaleString('en-US')} ${n === 1 ? 'run' : 'runs'}`
  return `${runTime(n * 1000)} run time`
}

export const budgetMetricLabels: Record<BudgetMetric, string> = { tokens: 'Tokens', runs: 'Runs', run_time: 'Run time' }
export const budgetWindowLabels: Record<BudgetWindow, string> = {
  calendar_month_utc: 'Monthly UTC budget',
  lifetime: 'Lifetime budget',
}

/** The Observed amount as a whole percent of the amount, 0 without a cap. */
export const budgetUtilisation = (b: Pick<Budget, 'amount' | 'observed'>) =>
  b.amount > 0 ? Math.round((b.observed / b.amount) * 100) : 0

export type BudgetInput = {
  scope_type: BudgetScope['type']
  scope_id?: number
  metric: BudgetMetric
  window: BudgetWindow
  amount: number
  warn_percent?: number
}

export async function setBudget(input: BudgetInput): Promise<Budget> {
  return (await api<{ budget: Budget }>('PUT', '/budgets', input)).budget
}

export async function resolveBudgetIncident(
  id: number,
  action: 'raise_budget_and_resume' | 'keep_paused',
  amount?: number,
): Promise<BudgetIncident> {
  return (await api<{ incident: BudgetIncident }>('POST', `/budget-incidents/${id}/resolve`, { action, amount })).incident
}

/** The unit a Budget metric's amount is typed in: run time in minutes on screen, seconds on the wire. */
export const budgetInputUnit = (metric: BudgetMetric) => (metric === 'run_time' ? 'minutes' : metric)
export const toBudgetInput = (metric: BudgetMetric, n: number) => (metric === 'run_time' ? Math.ceil(n / 60) : n)
/** A typed amount on the wire, or null when it is not a whole number ≥ 0 (minutes may be fractional). */
export function fromBudgetInput(metric: BudgetMetric, raw: string): number | null {
  const v = Number(raw.trim() || '0')
  if (!Number.isFinite(v) || v < 0) return null
  if (metric === 'run_time') return Math.round(v * 60)
  return Number.isInteger(v) ? v : null
}
