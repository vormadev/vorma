# Vorma Infallible State Machine Execution Checklist

Date: 2026-02-25 Scope: `typescript/vorma/*` (exclude e2e)

## Non-Negotiable Invariants

1. Route-scoped reads must be ownership-safe by construction.
2. Transition windows must be explicit state-machine states, never implicit
   fallback.
3. Route props produced by Vorma must resolve deterministically across
   transitions.
4. Forged/stale/invalid route props must fail loudly.
5. React, Preact, and Solid must share one ownership model.
6. Pattern-only APIs remain global selectors, route-props APIs remain
   route-instance selectors.

- [x] Create and pin this execution checklist in `__tmp` for live progress
      tracking.
- [x] Re-state non-negotiable invariants from prior plan and bind implementation
      to them.
- [x] Replace read-time token mutation with explicit route-instance store
      transitions.
- [x] Implement explicit store operations: create, sync-from-navigation,
      mark-exiting, dispose, read-or-throw.
- [x] Move transition updates to shared route-outlet runtime sync path (not
      hook-read paths).
- [x] Keep overloaded public APIs unchanged while enforcing internal route-props
      token contract.
- [x] Remove legacy implicit read-time synchronization path.
- [x] Add strict unit tests for explicit token-store lifecycle and contract
      violations.
- [x] Add per-tick transition-trace dist tests that record every render/effect
      tick for React.
- [x] Add per-tick transition-trace dist tests that record every render/effect
      tick for Preact.
- [x] Add per-tick transition-trace dist tests that record every render/effect
      tick for Solid.
- [x] Run Prettier on all touched TypeScript test/source files.
- [x] Run source test split: `make tstest-source`.
- [x] Rebuild dist artifacts:
      `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [x] Run dist test split: `make tstest-dist`.
- [x] Final self-audit against this checklist and report any remaining gaps
      explicitly.
