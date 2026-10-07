// The work context's API: Goals here; Issues and Comments join them in later
// pages. Every call answers in the Current guild.
import { api } from './api'

/** How wide a Goal reaches, widest first. */
export const goalLevels = ['guild', 'agent', 'task'] as const
export type GoalLevel = (typeof goalLevels)[number]

/** Where a Goal stands, in the order a Goal moves through them. */
export const goalStatuses = ['planned', 'active', 'achieved', 'cancelled'] as const
export type GoalStatus = (typeof goalStatuses)[number]

export const goalLevelLabels: Record<GoalLevel, string> = { guild: 'Guild', agent: 'Agent', task: 'Task' }

/** A Member as a Goal or an Issue names them. */
export type WorkMember = { id: number; name: string }

export type Goal = {
  id: number
  title: string
  description: string
  level: GoalLevel
  status: GoalStatus
  parent_id: number | null
  owner: WorkMember | null
  created_at: string
  updated_at: string
}

/** An Issue as a Goal's page lists it. */
export type GoalIssue = { id: number; identifier: string; title: string; status: string }

/** One Goal as its page shows it: with the Issues that serve it. */
export type GoalDetail = Goal & { issues: GoalIssue[]; issue_counts: Record<string, number> }

/** What creating or changing a Goal sends; null clears a parent or owner. */
export type GoalInput = Partial<{
  title: string
  description: string
  level: GoalLevel
  status: GoalStatus
  parent_id: number | null
  owner_id: number | null
}>

export const listGoals = () => api<{ goals: Goal[] }>('GET', '/goals').then((r) => r.goals)
export const getGoal = (id: number) => api<{ goal: GoalDetail }>('GET', `/goals/${id}`).then((r) => r.goal)
export const createGoal = (input: GoalInput) => api<{ goal: Goal }>('POST', '/goals', input).then((r) => r.goal)
export const updateGoal = (id: number, patch: GoalInput) => api<{ goal: Goal }>('PATCH', `/goals/${id}`, patch).then((r) => r.goal)
export const deleteGoal = (id: number) => api<void>('DELETE', `/goals/${id}`)
