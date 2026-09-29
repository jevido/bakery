# prod

Production.

Document here, per resource: domain, port, health check path, what triggers a
deploy, and which environment variables it needs (names only, never values).
Also note backups, how to roll back, and how to redeploy by hand.

Commit an `.env.example` or equivalent for the variables; real values live in
the deploy platform, never in the repo.
