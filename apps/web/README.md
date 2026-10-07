# web

The Bakery's dashboard: Svelte 5 + Vite + TypeScript (no SvelteKit). It only
talks to `services/api`, through `/api` on its own origin; in dev Vite
proxies that to `127.0.0.1:4910`, so the HttpOnly session cookie needs no
CORS.

```sh
task web:dev     # from the repo root: http://127.0.0.1:4930
task web:check   # svelte-check, tsc, build
task web:walk    # every route in headless Chromium, both sizes and themes (needs task dev)
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
`src/theme.css` is the only token source: Paperclip's semantic tokens
(`background`, `foreground`, `card`, `primary`, `muted`, `accent`,
`destructive`, `border`, `sidebar-*`, …) for dark and light, its radius and
type ladders, plus The Bakery's status colors (`success`, `warning`,
`error`). The font is Inter and the icons are lucide (`src/lib/Icon.svelte`
maps the names pages use). The components are shadcn-svelte on bits-ui in
`src/lib/components/ui` (imported as `$lib/components/ui/...`) with
Paperclip's classes; the `src/lib/ui` kit is built on them with its props
unchanged. `#/dev/components` (dev builds only) shows all of them. The
legacy names from the Coolify port (`bg-app`, `text-fg-dim`, `button`,
`input`, `menu-item`, …) are kept, defined from the new tokens, and removed
as pages are restyled. The theme is `localStorage.theme` (`dark`, `light` or
`system`, dark by default): `index.html` applies it before first paint and
`src/lib/theme.svelte.ts` keeps it in step. `src/app.css` is the legacy layer
for pages not yet ported: plain CSS in Tailwind's base layer whose variables
point at the new tokens; it shrinks as pages are ported.

The shell is Paperclip's Layout (`src/lib/Layout.svelte`): `Sidebar.svelte`
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
