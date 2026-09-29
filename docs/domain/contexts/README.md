# contexts

One directory per bounded context, named after the context in the ubiquitous
language (`docs/domain/contexts/<context>/README.md`). Copy
[`_template`](_template) to start one, and add the context to
[`../context-map.md`](../context-map.md).

A context document is the reasoning behind the model: why its boundary is
where it is, which invariants it protects, and what it gave up. When a choice
about a context is costly to reverse, write it down in that context's
"Why it's shaped this way" section.
