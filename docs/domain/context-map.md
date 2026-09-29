# Context map

The bounded contexts in this project, which unit hosts each one, and how they
depend on each other.

## Contexts

| Context | Subdomain | Hosted in | Owns |
| ------- | --------- | --------- | ---- |
|         | core / supporting / generic | `services/<unit>` | |

- **Core:** where the project competes. Gets the most care and the richest model.
- **Supporting:** needed and specific to this project, but not the differentiator.
- **Generic:** solved problems (auth, billing, email). Prefer buying or reusing
  over building.

## Relationships

One row per dependency. Upstream is the side whose model the other has to
adapt to.

| Upstream | Downstream | Pattern | Through |
| -------- | ---------- | ------- | ------- |
|          |            |         |         |

Patterns: *customer/supplier*, *conformist*, *anticorruption layer*,
*open host service* / *published language*, *shared kernel*, *separate ways*.
A shared kernel is a deliberate exception and needs a line saying why.

"Through" names the contract: an API, a set of domain events, a module
interface. Never another context's tables or internal types.

## Diagram

Optional. Keep it in sync with the tables above, or leave it out.

```mermaid
flowchart LR
  %% A -->|events| B
```
