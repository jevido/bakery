# web

The Bakery's dashboard: Svelte 5 + Vite + TypeScript (no SvelteKit). It only
talks to `services/api`, through `/api` on its own origin; in dev Vite
proxies that to `127.0.0.1:4910`, so the HttpOnly session cookie needs no
CORS.

```sh
task web:dev     # from the repo root: http://127.0.0.1:4930
task web:check   # svelte-check, tsc, build
```

- `src/lib/router.svelte.ts` is a small hash router (`#/projects`,
  `#/project/:id`, `#/project/:id/environment/:envId`, `#/applications/:id`, `#/members`, `#/api-tokens`,
  `#/invite/:token`, …). An Invitation link opens `#/invite/:token` for
  anyone, signed in or not.
- `src/lib/session.svelte.ts` knows the signed-in Member and their Role;
  pages hide what the Role may not do with `session.canWrite`,
  `session.canSeeSecrets` and `session.isAdmin`.
- `src/lib/api.ts` is the only place that calls `fetch`.

Styling is Tailwind CSS v4 (through `@tailwindcss/vite`). `src/theme.css`
holds Coolify's design tokens and shared utilities under their original names
(`bg-app`, `text-fg-dim`, `button`, `input`, `menu-item`, …), so ported markup
keeps its class lists. The theme is `localStorage.theme` (`dark`, `light` or
`system`, dark by default): `index.html` applies it before first paint and
`src/lib/theme.svelte.ts` (`setTheme()`) keeps it in step. `src/app.css` is
the legacy layer for pages not yet ported: plain CSS in Tailwind's base layer
whose variables point at the new tokens; it shrinks as pages are ported.
