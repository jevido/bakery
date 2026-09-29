# next

The pre-production environment. Changes land here before `prod`, running the
same images with their own configuration.

Document here, per resource: domain, port, health check path, what triggers a
deploy, and which environment variables it needs (names only, never values).
Keep it structured like `infra/prod/README.md` so the two are easy to diff.

Commit an `.env.example` or equivalent for the variables; real values live in
the deploy platform, never in the repo.
