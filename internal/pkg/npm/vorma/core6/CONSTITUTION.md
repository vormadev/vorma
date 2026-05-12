The core6 bet is **not** “rewrite legacy until the public tests pass.” It is
also **not** core5’s reducer/event-machine direction.

The core6 bet is:

Build a **small scoped imperative core** where the hardest correctness problems
are handled by a few hard primitives, not by a giant closure full of temporal
conditionals.

The spine is:

1. **Ownership scopes** Every async unit that can affect visible client state
   has an owner. If that owner is no longer current, it cannot commit. Late
   fetches, late module imports, late client loaders, late CSS waits, late
   revalidations, late prefetches all become harmless by construction.

2. **Transactions** Route-affecting work runs inside explicit transactions.
   Starting a new transaction cancels/supersedes the prior one. A transaction
   has one signal, one currentness rule, one cleanup path, and one final
   completion boundary.

3. **Pipeline** Route work is shaped as `fetch -> prepare -> publish`. Fetch and
   prepare can be async and stale-guarded. Publish is the final boundary and
   should be small, synchronous, and ownership-checked. No hand-rolled duplicate
   lifecycle next to the pipeline.

4. **Small stateful owner modules** Revalidation, prefetch, API submit,
   popstate, HMR, and navigation should each have explicit ownership and
   settlement rules. They can be imperative, but they must be small and named.
   No loose constellation of mutable slots hidden in one mega `core.ts`.

5. **Adapter as orchestration** The outer client adapter should wire platform
   concerns: DOM, history, fetch, modules, CSS/head, public callbacks. It must
   not become the place where every temporal rule lives. If a rule is about
   ownership, cancellation, freshness, or stale commits, it belongs in a small
   primitive/owner module.

6. **No decorative abstractions** A primitive either gets used by the real core
   path or it dies. No parallel “nice module” plus separate real implementation.

7. **Tests follow the boundary** Public behavioral tests stay in the existing
   core suites and run against candidates. Core6-local tests only prove internal
   primitive laws and small owner-state laws.

8. **Succinctness matters** Core6 only wins if it is meaningfully clearer and
   not wildly bigger than legacy. Correctness alone is not enough if achieved by
   copying legacy plus adding machinery.

So the religious rule is: **every line of adapter code must justify itself
against the scoped transaction/pipeline architecture.** If it starts becoming
“legacy but moved,” stop and extract or delete.
