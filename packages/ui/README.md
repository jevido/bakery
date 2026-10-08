# @bakery/ui

The Bakery's look in Paperclip's style, shared by the dashboard (`apps/web`)
and the Desktop app (`apps/desktop`). Source only, no build step: consumers
compile the `.svelte` and `.ts` files with their own Vite.

- `@bakery/ui/theme.css`: the design tokens (dark and light), the `dark`
  variant and the base rules. Import it after `tailwindcss`, and add
  `@source "<path to>/packages/ui/src";` so Tailwind builds the components'
  classes (it does not scan `node_modules`).
- `@bakery/ui/components/ui/<name>`: the shadcn-svelte components on bits-ui.
- `@bakery/ui/utils`: `cn` and shadcn-svelte's helper types.
- `@bakery/ui/theme`: the dark/light/system theme state (`theme.set`), and
  `@bakery/ui/ThemeToggle.svelte`, its icon toggle.
- `@bakery/ui/BakeryLockup.svelte`, `GuildIcon.svelte`, `AgentIcon.svelte`
  and `@bakery/ui/agentIcons` (the Agent icon names).
- `@bakery/ui/StatusBadge.svelte` with `@bakery/ui/statusColors` (the one
  status palette), `EntityRow.svelte` (a row of a bordered list), and
  `AgentRow.svelte` with `@bakery/ui/agentStatus` (an Agent in the Agents
  list, its status words and hues).

Only presentational code lives here: nothing that calls the API, routes or
knows the session. Inside the package, imports are relative (no `$lib`), so it
builds in any consumer.

New shadcn-svelte components are added from `apps/web`, whose
`components.json` points at this package: `bunx shadcn-svelte add <name>`.
Rewrite the `@bakery/ui/utils.js` imports it writes to relative ones.

Installed with the root Bun workspace: `bun install` (or `task install`) at the
repository root. Type-checked through `task web:check` and `task desktop:check`.
