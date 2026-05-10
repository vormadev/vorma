# Core3 Implementation Roadmap

Core3 should move in small, verifiable slices. The existing full gate remains
the acceptance suite for the replacement.

## How To Use This Roadmap

This file defines the intended implementation sequence, not just inventory.
Core3 work should stay accountable to it, but the roadmap is not a substitute
for judgment.

When local context shows that a different next slice is more coherent, revise
this roadmap first so the plan and the work stay intentionally aligned.
Already-built later pieces do not make earlier unchecked pieces complete. They
remain useful scaffolding, and any sequencing mismatch should be made explicit
rather than accidental.

## Ground Rules

These are constraints, not milestones:

- Preserve the current product contract as the behavioral target.
- Do not copy implementation code from baseline-core or core2.
- Do not introduce a parallel acceptance suite.
- Add targeted model tests only when they isolate new internal primitives.
- Keep docs forward-looking and current; revise them instead of appending
  historical notes.
- Keep every implementation slice small enough to compare against the existing
  gate.
- Treat bundle size as part of correctness. Prefer type-only vocabulary and
  runtime code that deletes duplication.
- Raise any public API concern explicitly before changing contract shape.

## Current Stage

Core3 has completed the pure model proof stage. The center is:

```text
operation start -> classified outcome -> operation transition -> publication -> settlement
```

The route outcome/publication proof, operation entry boundaries, and effect
runner boundary are complete enough to show the intended shape. Before adding
more host or adapter surface, the active work is a baseline-gate checkpoint:
read the legacy `core/create_client_core.ts` behavior and its existing tests,
verify each core3 production slice earns its runtime weight, and keep dev-only
HMR separate from production bundle accounting. Production runtime accounting
means `core3/*.ts` excluding tests and excluding `core3/dev/*`.

The completed pure-model slice:

- [x] Start visible navigation route work.
- [x] Start boot route work and consume boot payload facts.
- [x] Start popstate route work.
- [x] Normalize every refresh demand source into pending/running revalidation.
- [x] Decide the smallest pure-model lifecycle for route prefetch.
- [x] Own full navigation request policy, including same-document navigation,
      hard redirects, active-work replacement, and prefetch promotion.
- [x] Decide the smallest pure-model lifecycle for HMR route updates.
- [x] Verify public call settlement coverage across operation classes.
- [x] Verify no DOM, fetch, timer, module import, or callback code exists in the
      model.

The next implementation slice should resume Phase 6 only after this checkpoint
has a concrete coordinator shape and no obvious production-runtime contraction
is left on the table.

## Phase 1: Domain Types

- [x] Internal identity types.
- [x] Operation records and rights.
- [x] Model state records.
- [x] Classified route, API, preparation, and publication outcome types.
- [x] Effect descriptions.
- [x] Publication transaction types.
- [x] No browser imports.
- [x] No public adapter.
- [x] No effect execution.
- [x] No runtime vocabulary objects unless they clearly reduce emitted code.
- [x] No copied baseline-core or core2 implementation shape.

## Phase 2: Classifiers

- [x] Route response classifier.
- [x] API response classifier.
- [x] Route preparation completion classifier.
- [x] Route hook completion classifier.
- [x] Build-skew policy represented as data.
- [x] Redirect decisions explicit and testable.
- [x] Browser navigation request classifier.

## Phase 3: Operation Model

- [x] Publication rights are explicit.
- [x] Stale visible route completions are rejected by identity and rights
      checks.
- [x] Work projection is derived from operation facts.
- [x] Boot provisional snapshot is modeled.
- [x] API submit completion and refresh demand creation are modeled.
- [x] Refresh demand, retry, pending, and running states are modeled.
- [x] Route response data and soft navigation redirects are modeled.
- [x] Boot operation start and completion transitions.
- [x] Navigation operation start transition.
- [x] Popstate operation start transition.
- [x] Route revalidation start transition from every demand source.
- [x] Route prefetch operation lifecycle.
- [x] HMR update operation lifecycle.
- [x] Route preparation outcome transition for visible route work.
- [x] Route preparation outcome transition for background revalidation
      publication ownership.
- [x] Route hook outcome transition.
- [x] Visible route failure, hard redirect, and build-skew reload transitions.
- [x] Background revalidation build-skew, retry, and exhausted retry
      transitions.
- [x] Revalidation redirect transfer, ignored background refresh, and stale
      settlement transitions.
- [x] Build-skew reporting effects.
- [x] Public call settlement for every operation class.
- [x] No DOM, fetch, timer, module import, or callback code exists in the model.

## Phase 4: Publication Planner

- [x] Publication planner consumes prepared data and operation rights.
- [x] History, DOM, commit, render, scroll, listener, and public settlement
      effects are ordered in the transaction.
- [x] View transition phases are explicit.
- [x] Commit boundary installs the next route snapshot before route callbacks
      observe publication facts.
- [x] Publication settlement handles boot readiness.
- [x] Publication settlement satisfies running refresh cleanup.
- [x] Publication settlement can start pending refresh continuation.
- [x] Same-document navigation and popstate publication are represented without
      fetch shortcuts hidden in host code.
- [x] Route hooks are modeled as a gate before publication.
- [x] Hook-completed prepared data re-enters publication through the operation
      model.
- [x] Background revalidation obtains publication ownership only when it is
      actually ready to publish.

## Phase 5: Effect Runner

- [x] One abortable operation registry.
- [x] One timer registry.
- [x] One public waiter registry.
- [x] Ordered immediate effects inside publication transactions.
- [x] Async effects always return operation-correlated outcomes.
- [x] Effect runner owns resources but not route policy.

## Phase 6: Browser Host

- [x] Boot payload reading.
- [x] Fetch route and API.
- [ ] Module import.
- [ ] Client loader execution.
- [ ] CSS preload and wait.
- [ ] DOM/head/CSS publication.
- [x] History and scroll storage.
- [ ] Browser listeners.
- [x] View transition execution.
- [ ] User callback invocation.
- [ ] HMR dev bridge: production must only reach dev HMR through a direct
      `if (import.meta.env.DEV) { import(...) }` boundary.
- [ ] Host returns facts or executes commanded effects.
- [ ] Host does not decide stale, redirect, build-skew, retry, publication, or
      public settlement policy.
- [ ] Host can be tested through existing integration paths.

## Phase 7: Public Adapter

- [ ] Adapter allocates IDs and public waiters.
- [ ] Adapter normalizes public option names.
- [ ] Adapter exposes the existing public surface.
- [ ] Adapter does not inspect model state for policy decisions.
- [ ] Adapter reports the same public errors and results as the current product
      contract.

## Phase 8: Integration Under Existing Entry

- [ ] Wire core3 under the existing package entry in the smallest reversible
      slice.
- [ ] Existing focused tests pass without rewriting them for core3.
- [ ] Existing full gate passes.
- [ ] Core2 is not used as an oracle.
- [ ] Targeted model tests remain supplemental and do not replace the gate.

## Phase 9: Cleanup

- [ ] Remove unused experimental wiring.
- [ ] Remove temporary shims.
- [ ] Keep only forward-looking architecture docs.
- [ ] Keep the smallest useful set of targeted model tests.
- [ ] Verify gzip size and readability against the model's goals.
