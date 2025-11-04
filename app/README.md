# Bakery App

This directory contains the SvelteKit interface that powers Bakery’s web console. It is JavaScript-only (no TypeScript) and targets the Bun runtime through `svelte-adapter-bun`.

## Useful Commands

- `bun run dev` – Launches the Vite dev server on <http://localhost:5173>. Use `PUBLIC_API_URL=http://localhost:4100` to point at the local backend.
- `bun run build` – Produces the production bundle under `build/`, consumed by `backend/server.js`.
- `bun run preview` – Serves the built app locally to mimic the Bun adapter output.
- `bun run lint` – Runs Prettier against the codebase.

## Project Notes

- Tailwind CSS v4 is configured through `src/app.css`. Utility classes are available globally.
- UI elements lean on pre-built `bits-ui` primitives (buttons, separators, navigation) instead of bespoke components.
- Global layout and theming are defined in `src/routes/+layout.svelte`; feature pages live under `src/routes/`.

When running both the backend and this app for local development, start the backend first (`bun backend/server.js`) so the UI can access the API without CORS issues.
