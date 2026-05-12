# Core5 Work Plan

This plan is the source of truth for the core5 hardening work until it is
replaced intentionally. The two candidate implementations are legacy and core5.
No core4 comparison should steer this work.

## Current Goal

Move core5 toward an implementation that is simpler, more correct, easier to
reason about, and small enough to plausibly ship over legacy.

The immediate goal is not to prove legacy is bad. The immediate goal is to make
core5 earn its own keep.

## Working Rules

1. Do not start broad edits without naming the phase they belong to.
2. Do not use the docs app as a correctness signal. It is only useful for bundle
   size checks.
3. Prefer black-box tests when the bug is observable through the public client
   core contract.
4. Use core5 reducer/model tests for mathematical invariants that are not
   meaningfully observable through the public browser surface.
5. Do not add a parallel acceptance suite. Existing legacy tests and framework
   tests remain the acceptance suite.
6. Bundle size is always relevant, but size work should happen after each
   correctness pass has a stable boundary.
7. Do not compare against core4 except when removing accidental references or
   stale wiring.
8. Before finalizing any phase, run the smallest useful tests first, then the
   relevant Makefile target.

## Current In-Flight State

The current workspace already has partial core5 work in progress. Before moving
to new architectural work, the first task is to stabilize or deliberately revert
only the edits from the latest interrupted slice.

Known latest-slice edits:

1. Added popstate regression tests for destination scroll restoration and reload
   on route data, route preparation, and publication failure.
2. Added core5 behavior for full popstate scroll restoration.
3. Added core5 reload effects for full popstate failure paths.
4. Added a submit/revalidation race regression test.
5. Began runtime lifetime cleanup for abort handles, speculative prefetches,
   API response side tables, and settled revalidation side tables.
6. Began removing unused protocol surface such as `queue_microtask`,
   `has_response`, `notify_route_update`, and unused platform methods.

The protocol-surface removal was interrupted and must be completed or backed
out before any broader work continues.

## Phase 0: Stabilize The Current Slice

Purpose: get back to a coherent, tested checkpoint.

Tasks:

1. Finish or back out the interrupted protocol-surface removal.
2. Re-run the focused core5/router tests.
3. Re-run TypeScript typecheck.
4. Write a short status note listing exactly what was kept and what remains
   unfinished.

Exit criteria:

1. Focused tests pass.
2. TypeScript typecheck passes.
3. The current diff is internally coherent.

## Phase 1: Core5 Invariant Audit

Purpose: turn the audit into explicit invariants before more refactoring.

Core invariants to write down and test:

1. Model state contains logical facts, not live browser/runtime resources.
2. Every async route/API operation has exactly one active owner token.
3. Late async completions from stale owners cannot mutate state, emit public
   effects, redirect, report build skew, or publish.
4. Every public call settles exactly once.
5. Revalidation demand cannot be lost when navigation/API work supersedes
   another owner.
6. Popstate full-route failure reloads; hash-only popstate does not fetch.
7. Publication is cancellable until the final commit point.
8. Runtime side tables have explicit lifetimes.

Deliverables:

1. Core5 invariant tests in `core5/core.test.ts` for reducer/model laws.
2. Public regression tests in existing `core/*.test.ts` files only when behavior
   is observable through the public core contract.
3. A short notes section in this file or a successor audit file identifying
   which invariants are still untested and why.

## Phase 2: Runtime Ownership And Lifetime

Purpose: make runtime side tables obey ownership laws instead of accumulating
dead state.

Tasks:

1. Define clear lifetime rules for abort handles.
2. Define clear lifetime rules for API response storage.
3. Define clear lifetime rules for revalidation waiters and late revalidation
   settlement.
4. Define clear lifetime rules for speculative prefetch loader results.
5. Decide whether route hook registrations are route-lifetime state,
   module-lifetime state, or view-registry state; then encode that decision.
6. Add tests for observable races; add reducer/runtime-adjacent invariant tests
   only where black-box tests cannot see the issue.

Exit criteria:

1. No side table exists without a documented owner and release point.
2. Stale async completions are either ignored or explicitly release their
   runtime resources.

## Phase 3: Browser Contract Parity Against Legacy

Purpose: make core5 match the important public browser behavior of legacy.

Areas:

1. History state wrapping and browser keys.
2. Scroll persistence, reload scroll, popstate scroll, and same-document scroll.
3. Redirect chains and loop behavior.
4. Build skew behavior for navigation, popstate, prefetch, revalidation, and API
   routes.
5. API request normalization and response semantics.
6. Mutation-triggered revalidation, including boot-time and superseded work.
7. Work indicator behavior.
8. Route update and work update callback timing.
9. HMR behavior, especially `runClientLoaderOnHMR`.

Exit criteria:

1. Relevant existing core tests pass with core5 enabled.
2. Any newly discovered bug gets a focused regression test.
3. Any intentional legacy behavior difference is explicitly documented before it
   is implemented.

## Phase 4: Architectural Cleanup

Purpose: make the architecture actually feel smaller and cleaner, not merely
more distributed.

Tasks:

1. Remove unused protocol fields and dead effects.
2. Remove unused platform methods or route them through one owner.
3. Consolidate single-use helpers unless they objectively clarify a real law.
4. Keep internal names in repo style.
5. Keep comments sparse and in repo style.
6. Reduce duplication between route navigation, popstate, revalidation, and
   prefetch transitions where doing so clarifies ownership.

Exit criteria:

1. Less dead surface than before the phase.
2. No new abstraction without a concrete repeated law or meaningful complexity
   reduction.
3. Focused tests and typecheck pass.

## Phase 5: Bundle Size Pass

Purpose: decide whether core5 can plausibly ship.

Tasks:

1. Build the same bundle-size target for legacy and core5.
2. Record raw and gzip deltas.
3. Identify the largest core5-only contributors.
4. Remove accidental size first: dead types, duplicate helpers, unused platform
   methods, redundant effect variants, overly-general structures.
5. Only then consider structural compression that might trade clarity for size.

Exit criteria:

1. A measured size delta exists.
2. The remaining delta is explained in terms of real architecture or real
   behavior, not accidental weight.
3. A recommendation exists: ship core5, keep hardening, or abandon core5.

## Phase 6: Full Gate

Purpose: prove the work is not locally correct but globally broken.

Tasks:

1. Run focused tests for changed areas.
2. Run `make test-ts` or the narrower Makefile target if the changed surface is
   still limited.
3. Run `make gate` when core5 is a serious replacement candidate.

Exit criteria:

1. Gate result is recorded.
2. Any failure is categorized as core5 regression, unrelated existing failure,
   or test expectation mismatch.

## Next Action

Complete Phase 0 only.

Do not begin new architectural work until Phase 0 has a coherent diff and a
clear status update.
