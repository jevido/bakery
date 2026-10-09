// The ordering and parsing behind the sidebar's Chats, as Paperclip's
// ui/src/lib/recent-agent-chats.ts (MIT, see NOTICE), kept free of runes so
// bun test can run it; recentChats.svelte.ts holds the stored lists.

/** How many Agent ids a list keeps. */
export const CHAT_LIST_LIMIT = 50

/** A stored list of Agent ids: anything else is dropped, duplicates once, at most 50. */
export function parseChatAgentIds(raw: string | null): number[] {
  try {
    const ids: unknown = JSON.parse(raw ?? '[]')
    if (!Array.isArray(ids)) return []
    return [...new Set(ids.filter((id): id is number => Number.isInteger(id) && id > 0))].slice(0, CHAT_LIST_LIMIT)
  } catch {
    return []
  }
}

/** The list with id first, as a visit leaves it. */
export function withVisit(ids: number[], id: number): number[] {
  return [id, ...ids.filter((x) => x !== id)].slice(0, CHAT_LIST_LIMIT)
}

/** The list with id added, or removed when it was there. */
export function withToggle(ids: number[], id: number): number[] {
  return ids.includes(id) ? ids.filter((x) => x !== id) : [id, ...ids].slice(0, CHAT_LIST_LIMIT)
}

/**
 * The Agents under Chats: the starred ones by name, then the Guild's first
 * Agent (so the section is never empty while the Guild has one), then up to
 * four of the most recently visited.
 */
export function orderChatAgents<T extends { id: number; name: string; created_at: string }>(agents: T[], starred: number[], recent: number[]): T[] {
  const first = [...agents].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime() || a.id - b.id)[0]
  return [
    ...agents.filter((a) => starred.includes(a.id)).sort((a, b) => a.name.localeCompare(b.name) || a.id - b.id),
    ...(first && !starred.includes(first.id) ? [first] : []),
    ...[...new Set(recent)]
      .filter((id) => !starred.includes(id) && id !== first?.id)
      .flatMap((id) => agents.filter((a) => a.id === id))
      .slice(0, 4),
  ]
}
