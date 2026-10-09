// The Agents a Member recently chatted with and the ones they starred, per
// Guild and Member in localStorage, as Paperclip keeps its recent chats
// (ui/src/lib/recent-agent-chats.ts; MIT, see NOTICE). Paperclip stars an
// Agent on the server; here the star stays in the browser beside the recent
// list, as nothing else reads it. Another tab's change follows through the
// storage event.
import { parseChatAgentIds, withToggle, withVisit } from './recentChats'
import { session } from './session.svelte'

type Kind = 'recent' | 'starred'

const prefix: Record<Kind, string> = { recent: 'bakery.recentAgentChats', starred: 'bakery.starredAgentChats' }

// Bumped on every write, so the getters below are read again.
let version = $state(0)

if (typeof window !== 'undefined') window.addEventListener('storage', () => version++)

function key(kind: Kind): string | null {
  const guild = session.guild?.id
  const member = session.member?.id
  return guild && member ? `${prefix[kind]}:${guild}:${member}` : null
}

function read(kind: Kind): number[] {
  void version
  const k = key(kind)
  if (!k) return []
  try {
    return parseChatAgentIds(localStorage.getItem(k))
  } catch {
    return []
  }
}

function write(kind: Kind, ids: number[]) {
  const k = key(kind)
  if (!k) return
  try {
    localStorage.setItem(k, JSON.stringify(ids))
  } catch {
    // Without storage the lists stay as they were; the chat still works.
  }
  version++
}

export const chatLists = {
  /** The Agent ids most recently chatted with, newest first. */
  get recent(): number[] {
    return read('recent')
  },
  /** The starred Agent ids. */
  get starred(): number[] {
    return read('starred')
  },
}

/** Puts the Agent first among the recent chats; the chat page calls it once the Agent is known. */
export function recordChatVisit(agentId: number) {
  write('recent', withVisit(read('recent'), agentId))
}

/** Stars the Agent, or unstars it. */
export function toggleChatStar(agentId: number) {
  write('starred', withToggle(read('starred'), agentId))
}
