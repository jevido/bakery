# web

The Bakery's dashboard: Svelte 5 + Vite + TypeScript (no SvelteKit). It only
talks to `services/api`, through `/api` on its own origin; in dev Vite
proxies that to `127.0.0.1:4910`, so the HttpOnly session cookie needs no
CORS.

```sh
task web:dev     # from the repo root: http://127.0.0.1:4930
task web:check   # svelte-check, tsc, build
task web:walk    # every route in headless Chromium, both sizes and themes (needs task dev)
task web:rail    # the guild rail in headless Chromium (needs task dev)
task web:settings # the settings pages in headless Chromium (needs task dev)
```

- `src/lib/router.svelte.ts` is a small hash router (`#/projects`,
  `#/project/:id`, `#/project/:id/environment/:envId`,
  `#/project/:id/environment/:envId/application/:appId[/:page]`, `#/members`, `#/security/api-tokens`,
  `#/invite/:token`, …). An Invitation link opens `#/invite/:token` for
  anyone, signed in or not.
- `src/lib/session.svelte.ts` knows the signed-in Member, the Current guild
  and its Permissions; pages hide what the Member may not do with
  `session.can('deploy')` and the like (`src/lib/projectAccess.svelte.ts`
  for a Project's, after its Permission overrides). Never a Role's name.
- `src/lib/api.ts` is the only place that calls `fetch`.

Styling is Tailwind CSS v4 (through `@tailwindcss/vite`) in Paperclip's look.
`@bakery/ui/theme.css` (`packages/ui`, imported by `src/theme.css`) is the only token source: Paperclip's semantic tokens
(`background`, `foreground`, `card`, `primary`, `muted`, `accent`,
`destructive`, `border`, `sidebar-*`, …) for dark and light, its radius and
type ladders, plus The Bakery's status colors (`success`, `warning`,
`error`). The font is Inter and the icons are lucide (`src/lib/Icon.svelte`
maps the names pages use). The components are shadcn-svelte on bits-ui in
`packages/ui` (imported as `@bakery/ui/components/ui/...`, shared with the
Desktop app) with
Paperclip's classes; the `src/lib/ui` kit is built on them with its props
unchanged. `#/dev/components` (dev builds only) shows all of them. List
pages are built from Paperclip's list pieces in `src/lib`: `PageHeader` (tile,
name, description, actions), `CollectionToolbar` (search, controls, actions)
with `SearchField`, `SortPopover` and `ViewToggle`, `EntityRow`,
`ProjectTile` and `PageSkeleton`; settings pages from
`src/lib/settings/SettingsPage` and `SettingsGroup`. The Application,
Database and Service pages share one frame: `PageHeader` with the status
and an Actions menu, and `ResourceNav`, the grouped sub-page nav drawn
with `SidebarSection`/`SidebarNavItem` from `xl` and as a select below it.
Every status pill takes its tone from `src/lib/statusColors.ts`, the one
status palette (success, warning, error, muted). Deployment and Runtime
Logs draw on the `log` and `log-foreground` tokens (a dark panel in dark
mode, a light one in light mode). Modals, menus and toasts take their
transition lengths from `src/lib/ui/motion.ts`, which makes them instant
when the person (or an e2e script's browser) asks for reduced motion. The legacy names from the Coolify
port that the Server, Notifications, Keys & Tokens, Profile and Settings
pages still use (`button`, `button-highlighted`, `input`, `select`,
`menu-item*`, `nav-section`, `listbox-*`, `data-table*`,
`application-settings-section*`, `settings-mobile-*`, `text-fg`,
`text-fg-dim`, `text-fg-faint`, `bg-raised`, `bg-selected`, the `coollabs`
and `coolgray-*` colors, …) are kept, defined from the new tokens, and
removed as those pages are restyled; the Projects, Environment and
Resource pages and `src/lib/ui` use none of them. The theme is `localStorage.theme` (`dark`, `light` or
`system`, dark by default): `index.html` applies it before first paint and
`src/lib/theme.svelte.ts` keeps it in step. `src/app.css` is the legacy layer
for pages not yet ported: plain CSS in Tailwind's base layer whose variables
point at the new tokens; it shrinks as pages are ported.

The shell is Paperclip's Layout (`src/lib/Layout.svelte`): `GuildRail.svelte`,
the column of guild icons at the far left (Discord's server list, with a "+"
for a new Guild); `Sidebar.svelte`
(resizable from 240px, collapsing to a 64px icon rail with tooltips, a drawer
below 768px) headed by `GuildMenu.svelte` (switch, create or invite to a
Guild) and ending in `AccountMenu.svelte` (Profile, Settings, theme, Sign
out); `BreadcrumbBar.svelte`, the 60px bar with the page's title; and
`SettingsSidebar.svelte`, which settings routes (Guild, Notifications, Keys &
Tokens, Profile, Settings) show in place of the sidebar. Login, Setup and
Invite open outside it.

`e2e/walk.ts` signs in as the Owner of the dev installation
(`infra/dev/state/owner.env`) and opens every route in `router.svelte.ts`
at 1440×900 and 390×844, dark and light. It fails on a console or page
error, a sideways scrollbar at 390px, a page outside the shell, or the words
"Coolify" or "Paperclip" anywhere a person reads them.

`e2e/settings.ts` (`task web:settings`, needs `task dev`) checks the settings
pages, one section per page (`bun e2e/settings.ts profile` runs one):
`notifications` (every Channel kind, a Discord channel saved, enabled,
disabled and tested, an event toggled, the Deliveries and the Send Test Email
dialog), `tokens` (create, copy once, search, use and revoke an API token),
`profile` (rename, a wrong password, two-factor setup), `instance` (Known
hosts and the Forget confirmation) and `viewer` (what a Viewer cannot see),
each once in the dark theme at 1440×900.

`e2e/work.ts` (`task web:work`, needs `task dev`) checks the work pages,
one section per flow (`bun e2e/work.ts issue` runs one): `goals` (the tree,
New Goal, a Sub-goal, in-place editing, Markdown), `issues` (New Issue, search,
filters and grouping), `issue` (the Issue page: properties, sub-issues and
the comment thread), `viewer` (a Viewer, invited for the run, reads and
changes nothing), `hidden` (an Issue in a Project denied to the Member
Role is gone for a Member), and more for Blockers, documents, Activity,
the Inbox, Approvals, Runs and Agents (the file's header lists them all);
`pull-request` starts the Forgejo stand-in and follows an Agent's Run from
the Issue's Application to its Pull request and Preview on the Issue page.
Each runs once in the dark theme at 1440×900.

`e2e/servers.ts` (`task web:servers`, needs `task dev`) checks the Servers
pages and S3 Storage, one section per flow (`bun e2e/servers.ts remote`
runs one): the list, the Server frame, Resources, Metrics, Cleanup, the
Remote server stand-in (`task remote:up`, else skipped), General and S3
Storage, each once in the dark theme at 1440×900, plus a Viewer pass. The
Server switcher sits beside the Server's name in its page header
(`src/pages/server/Heading.svelte`); the breadcrumb bar has no portal slots
any more, so nothing renders into it from a page.
