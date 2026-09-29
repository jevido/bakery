# Glossary

The ubiquitous language. These are the words used in conversation, in issues,
in the docs **and in the code**: class, type, function, table and event names
follow this list. If a term is missing or wrong, fix it here first, then in the
code.

A term that means different things in different contexts gets one row per
context. Terms used only inside one context can also live in that context's
document; list them here when people outside the context use them too.

| Term | Context | Meaning | Not to be confused with |
| ---- | ------- | ------- | ----------------------- |
| Owner | identity | The one person who administers this Bakery installation. Created by setup on first run. | User, admin, account |
| Session | identity | Proof that the Owner signed in: a JWT carried in the `bakery_session` cookie. | Token (API tokens come later) |
| Setup | identity | The one-time step that creates the Owner. Refused once an Owner exists. | Install |
| Project | projects | A named group of Environments, usually one product. | Repository |
| Environment | projects | A stage inside a Project (`production` is created with every Project). Holds Applications. | Env var, the dev/next/prod environments of this repo |
| Application | projects | Something Bakery builds from a Source and runs as one Container behind one Domain. | Service (templates, later), Container |
| Slug | projects | The URL-safe, unique short name of an Application, used in its default Domain and image name. | Name |
| Source | projects | Where an Application's code comes from: a git URL and a branch. Either a public `https://` URL, or an SSH URL (`ssh://git@host/path` or `git@host:path`) that always has a Deploy key. | Repository on disk |
| Deploy key | projects | The SSH key pair Bakery generates for one Application. The Owner adds the public half to the repository as a read-only deploy key; the private half is encrypted at rest. | The Server's SSH key, API token |
| Build pack | projects | How a Source becomes an Image. Only `dockerfile` exists: build the Dockerfile at a path in the Source. | Buildpacks (Heroku/CNB) |
| Env var | projects | A name and value handed to an Application's Container. The value is encrypted at rest. | Environment |
| Domain | projects, routing | The hostname an Application is reached on. Unique across Bakery, and never the dashboard domain. Defaults to `<slug>.<domain suffix>` (`localhost` in development). | URL |
| Deployment | deployments | One attempt to turn an Application's Source into its running Container. | Release, build |
| Deploy trigger | deployments | What started a Deployment: `manual` (the Owner pressed Deploy) or `webhook` (a push). | Build pack |
| Deployment status | deployments | Where a Deployment is: `queued`, `cloning`, `building`, `starting`, then `finished` or `failed`. The first four are *active*; `cloning`, `building` and `starting` are *running*. | Container state |
| Deployment log | deployments | The ordered lines a Deployment wrote: its own `info` lines and the `out`/`err` output of git and the build. | Container logs |
| Container logs | deployments | What a running Application's Container writes to stdout and stderr. | Deployment log |
| Image | deployments | The built result of a Deployment, tagged `localhost/bakery/<slug>:<deployment-id>`. | Container |
| Container | deployments | A running instance of an Image, named `bakery-app-<application-id>-<deployment-id>`. | Application |
| Webhook | deployments | The URL and secret a git host calls on every push, so a push to an Application's branch deploys it. | Notification webhook (later) |
| Auto-deploy | deployments | Whether a verified push to the Application's branch queues a Deployment. On by default. | Redeploy |
| Known host | deployments | A git host's SSH host key, remembered on the first clone from that host and required to match on every later one. | Server |
| Server | deployments | A machine Bakery runs Containers on. Only the local one (the rootless Podman socket) exists. | Proxy |
| Proxy | routing | The Caddy container `bakery-proxy`, configured only through its admin API. | Server |
| Route | routing | A Domain pointed at one Container and port. One per Application. | Endpoint |
| Dashboard Route | routing | The Route to Bakery's own dashboard and API on the configured dashboard domain: `/api/*` to the API, the rest to the dashboard. Derived from configuration, never stored. | Route |
