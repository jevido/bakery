// Costs and Budgets of the Guild's Agents' Runs: what the API answers at
// /api/costs/* and /api/budgets/overview, read from the Runs' Run usage.
import { api } from './api'

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
