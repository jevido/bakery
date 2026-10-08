# apps

Things a person opens and interacts with: websites, desktop apps, CLIs.

One directory per app, directly under `apps/`, each with its own `README.md`
saying what it is, which bounded contexts it hosts, and how to run it.
Each context is its own top-level module in the unit; see `CLAUDE.md`.

- `web`: the dashboard, in the browser.
- `desktop`: the Desktop app (Wails 3), on a person's own machine; it
  connects to one or more Bakeries and will run their Agents with `claude`.
  It hosts no context of its own: like `web`, it is a client of the API.
