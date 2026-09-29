# Context name

- Subdomain: core | supporting | generic
- Hosted in: `services/<unit>` (module `<path>`)

## Purpose

What this context is responsible for, in two or three sentences. What it is
explicitly **not** responsible for.

## Language

Terms that have a specific meaning inside this context. Shared terms go in
[`glossary.md`](../../glossary.md).

| Term | Meaning |
| ---- | ------- |
|      |         |

## Model

### Aggregates

For each aggregate: its root, what it contains, and the invariants it
guarantees. One transaction changes one aggregate.

| Aggregate | Invariants |
| --------- | ---------- |
|           |            |

### Commands

What callers can ask this context to do.

### Domain events

What this context announces happened, in past tense (e.g. `OrderPlaced`), and
what each event carries.

## Integration

- **Publishes:** events and APIs other contexts may rely on. Changing these is
  a breaking change for downstream contexts.
- **Consumes:** what it takes from other contexts, and where it translates that
  into its own language (anticorruption layer).

## Why it's shaped this way

Choices that are costly to reverse: where the boundary sits, why an aggregate
is split or merged, why an integration is synchronous or event-driven. For
each: the context that forced it, what was chosen, what it costs, and what
else was considered.
