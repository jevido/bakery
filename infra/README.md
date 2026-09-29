# infra

Container, environment and deploy configuration. No application code.

| Directory            | What goes there                                                     |
| -------------------- | ------------------------------------------------------------------- |
| [`dev/`](dev)        | Local development: compose stack for backing services, helper scripts. |
| [`next/`](next)      | The pre-production environment: config that differs from prod.      |
| [`prod/`](prod)      | Production: config, domains, runbook notes.                          |
| [`images/`](images)  | Containerfiles, one directory per deployed resource, shared by next and prod. |

**One image, many environments.** An image is built once from
`infra/images/<resource>/Containerfile` and runs unchanged in `next` and then
`prod`. Only configuration (environment variables, domains, replicas) differs
between environments, and that lives in `infra/next/` and `infra/prod/`. If
something only works in one environment, it belongs in config, not in the
image.
