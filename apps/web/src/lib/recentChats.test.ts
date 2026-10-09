import { expect, test } from 'bun:test'
import { orderChatAgents, parseChatAgentIds, withToggle, withVisit } from './recentChats'

const agent = (id: number, name: string, created_at = `2026-01-0${id}T00:00:00Z`) => ({ id, name, created_at })

test('parseChatAgentIds keeps positive integer ids once, at most 50', () => {
  expect(parseChatAgentIds('[3, 1, 3, "2", null, -4, 1.5, 7]')).toEqual([3, 1, 7])
  expect(parseChatAgentIds(JSON.stringify(Array.from({ length: 60 }, (_, i) => i + 1)))).toHaveLength(50)
  expect(parseChatAgentIds(null)).toEqual([])
  expect(parseChatAgentIds('{"a":1}')).toEqual([])
  expect(parseChatAgentIds('not json')).toEqual([])
})

test('withVisit moves the Agent to the front', () => {
  expect(withVisit([1, 2, 3], 3)).toEqual([3, 1, 2])
  expect(withVisit([], 5)).toEqual([5])
})

test('withToggle adds or removes the Agent', () => {
  expect(withToggle([1, 2], 3)).toEqual([3, 1, 2])
  expect(withToggle([1, 2], 1)).toEqual([2])
})

test('orderChatAgents puts starred by name, then the first Agent, then four recent', () => {
  const agents = [agent(1, 'Ada'), agent(2, 'Zed'), agent(3, 'Bo'), agent(4, 'Cy'), agent(5, 'Di'), agent(6, 'Ed'), agent(7, 'Fi')]
  const ids = (as: { id: number }[]) => as.map((a) => a.id)
  expect(ids(orderChatAgents(agents, [], []))).toEqual([1])
  expect(ids(orderChatAgents(agents, [2, 3], [5]))).toEqual([3, 2, 1, 5])
  expect(ids(orderChatAgents(agents, [1], [7, 6, 5, 4, 3, 1]))).toEqual([1, 7, 6, 5, 4])
  // A recent id whose Agent is gone is skipped.
  expect(ids(orderChatAgents(agents, [], [99, 4]))).toEqual([1, 4])
  expect(orderChatAgents([], [1], [1])).toEqual([])
})
