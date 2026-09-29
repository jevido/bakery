# services

Things that run unattended, with no person interacting with them directly:
APIs, workers, schedulers, MCP servers.

One directory per service, directly under `services/`, each with its own
`README.md` saying what it is, which bounded contexts it hosts, and how to run it.
Each context is its own top-level module in the unit; see `CLAUDE.md`.
