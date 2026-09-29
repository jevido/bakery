# contexts

One directory per bounded context (see `docs/domain/contexts/`). Inside each:

| Directory | Holds | May import |
| --------- | ----- | ---------- |
| `domain/` | Aggregates, value objects, invariants, events | Nothing outside the standard library |
| `app/`    | Use cases that orchestrate the domain | `domain/` and interfaces it declares |
| `infra/`  | ORM repositories, external clients (Podman, Caddy, git) | `domain/`, `app/`, Goravel |
| `http/`   | Controllers, requests, response shapes | `app/`, Goravel |

A context talks to another only through the other's published package
(the functions and types in `contexts/<name>/<name>.go`), never its
`infra/` or tables.
