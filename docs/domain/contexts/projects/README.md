# projects

- Subdomain: supporting
- Hosted in: `services/api` (module `contexts/projects`)

## Purpose

Holds what the Owner wants to run: Projects, their Environments, the
Applications in them, and each Application's Env vars. It is **not**
responsible for building or running anything (deployments) or for how traffic
reaches an Application (routing).

## Language

| Term | Meaning |
| ---- | ------- |
| Project | A named group of Environments. |
| Environment | A stage in a Project; `production` exists from the start. Names are unique per Project. |
| Application | Source + Dockerfile path + port + Domain, plus Env vars. |
| Application snapshot | The read-only view of an Application handed to deployments, with Env var values decrypted. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Project | Name is not empty. Is created together with its `production` Environment, in one transaction. Environment names are unique within the Project. |
| Application | Slug is unique across Bakery, lowercase `[a-z0-9-]`. Source URL starts with `https://` (no `file://`, `ssh://`, `git@` or local paths). Branch is not empty. Dockerfile path is relative and does not contain `..`. Port is 1–65535. Domain is a lowercase hostname, unique across Bakery, default `<slug>.localhost`. Env var names match `^[A-Za-z_][A-Za-z0-9_]*$` and are unique per Application; values are encrypted at rest and never logged. |

### Commands

- `CreateProject`, `RenameProject`, `DeleteProject` (refused while it has Applications).
- `CreateApplication(environment, ...)`, `UpdateApplication`, `DeleteApplication`.
- `ReplaceEnvVars(application, [name, value])`: the whole set is replaced at once.

### Domain events

- `ApplicationDeleted { applicationID }`: routing drops its Route.

## Integration

- **Publishes:** `ApplicationForDeploy(id) (ApplicationSnapshot, error)` for
  deployments. Changing its fields is a breaking change for deployments.
- **Consumes:** the Owner's Session (identity middleware).

## Why it's shaped this way

- **Environments exist from day one**, even though only `production` is
  used. Coolify's Project > Environment > Resource shape is what later phases
  (previews, staging) need, and adding a level under existing Applications
  later would be a data migration.
- **Source URLs must be `https://`.** The deployment worker clones whatever
  URL it is given on the host; a `file://` URL or a local path would let an
  Application read the host's files. Private repositories (deploy keys,
  GitHub App) come later as their own Source kinds rather than by loosening
  this rule.
- **Env var values are encrypted with the application key**
  (`APP_KEY`, AES via Goravel's Crypt). A database dump alone does not leak
  them. Cost: losing `APP_KEY` loses every value.
