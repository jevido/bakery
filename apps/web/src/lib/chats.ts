// The work context's Conversations: the asking Member's chats with the
// Guild's Agents, each an Issue (Paperclip's agent chat, ui/src/api/agentChats.ts;
// MIT, see NOTICE). Every call answers in the Current guild; its messages are
// the Issue's Comments, read and written through ./work.
import { api } from './api'
import type { Issue, IssueDetail } from './work'

/** The asking Member's Conversations, most recently updated first. */
export const listChats = () => api<{ issues: Issue[] }>('GET', '/chats').then((r) => r.issues)

/** The asking Member's Conversation with the Agent, or null while they have none. */
export const getChat = (agentId: number) => api<{ issue: IssueDetail | null }>('GET', `/chats/${agentId}`).then((r) => r.issue)

/** Opens the asking Member's Conversation with the Agent, or answers the one they have. */
export const openChat = (agentId: number) => api<{ issue: IssueDetail }>('POST', `/chats/${agentId}`).then((r) => r.issue)
