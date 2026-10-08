<script lang="ts" module>
  // Paperclip's agent icons (ui/src/lib/agent-icons.ts; MIT, see NOTICE):
  // each Agent icon name drawn with its Lucide icon, a bot when none.
  import {
    Atom, Bot, Brain, Bug, CircuitBoard, Code, Cog, Cpu, Crown, Database, Eye, FileCode, Fingerprint, Flame, Gem, GitBranch, Globe,
    Hammer, Heart, Hexagon, Lightbulb, Lock, Mail, MessageSquare, Microscope, Package, Pentagon, Puzzle, Radar, Rocket, Search,
    Shield, Sparkles, Star, Swords, Target, Telescope, Terminal, WandSparkles, Wrench, Zap,
  } from '@lucide/svelte'
  import type { Component } from 'svelte'
  import type { AgentIconName } from './agentIcons'

  export const agentIcons: Record<AgentIconName, Component<{ class?: string }>> = {
    bot: Bot, cpu: Cpu, brain: Brain, zap: Zap, rocket: Rocket, code: Code, terminal: Terminal, shield: Shield, eye: Eye,
    search: Search, wrench: Wrench, hammer: Hammer, lightbulb: Lightbulb, sparkles: Sparkles, star: Star, heart: Heart,
    flame: Flame, bug: Bug, cog: Cog, database: Database, globe: Globe, lock: Lock, mail: Mail,
    'message-square': MessageSquare, 'file-code': FileCode, 'git-branch': GitBranch, package: Package, puzzle: Puzzle,
    target: Target, wand: WandSparkles, atom: Atom, 'circuit-board': CircuitBoard, radar: Radar, swords: Swords,
    telescope: Telescope, microscope: Microscope, crown: Crown, gem: Gem, hexagon: Hexagon, pentagon: Pentagon,
    fingerprint: Fingerprint,
  }
</script>

<script lang="ts">
  let { icon, class: className = 'size-4' }: { icon: string | null | undefined; class?: string } = $props()

  const Glyph = $derived(agentIcons[icon as AgentIconName] ?? Bot)
</script>

<Glyph class={className} />
