# web

Bakery's dashboard: Svelte 5 + Vite + TypeScript (no SvelteKit). It only
talks to `services/api`, through `/api` on its own origin; in dev Vite
proxies that to `127.0.0.1:4910`, so the HttpOnly session cookie needs no
CORS.

```sh
task web:dev     # from the repo root: http://127.0.0.1:4930
task web:check   # svelte-check, tsc, build
```

- `src/lib/router.svelte.ts` is a small hash router (`#/projects`,
  `#/projects/:id`, `#/applications/:id`).
- `src/lib/api.ts` is the only place that calls `fetch`.
