/** The Agent icons, Paperclip's AGENT_ICON_NAMES; an Agent without one is a bot. */
export const agentIconNames = [
  "bot", "cpu", "brain", "zap", "rocket", "code", "terminal", "shield", "eye",
  "search", "wrench", "hammer", "lightbulb", "sparkles", "star", "heart",
  "flame", "bug", "cog", "database", "globe", "lock", "mail",
  "message-square", "file-code", "git-branch", "package", "puzzle", "target",
  "wand", "atom", "circuit-board", "radar", "swords", "telescope",
  "microscope", "crown", "gem", "hexagon", "pentagon", "fingerprint",
] as const;
export type AgentIconName = (typeof agentIconNames)[number];
