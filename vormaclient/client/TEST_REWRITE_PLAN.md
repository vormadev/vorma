# vormaclient/client Test Rewrite Plan

## Why

The current suite has broad behavior coverage, but it is spread across many
legacy files with granular assertions tied to implementation details. This makes
safe refactors difficult and slows down intentional architectural changes.

## Primary Goal

Create a contract-first, maintainable test system that preserves user-visible
behavior while allowing large internal refactors (including a full rewrite) with
confidence.

## Non-Goals

- No incidental behavior changes unrelated to strict first-principles contract
  failures.
- No physical deletion of migrated legacy tests before explicit signoff.
- No coupling to current private implementation details in new suites.

## Guiding Principles

- Public API first: new tests should drive `vormaclient/client` via exported API
  where practical.
- Deterministic async: use controlled timers and deferred promises.
- One behavior, one assertion story: avoid near-duplicate tests in multiple
  files.
- Regression retention: observed bug reproductions remain as regression
  scenarios, rewritten at higher level when possible.
- First-principles correctness over implementation mirroring: suspicious current
  behavior should be fixed in source or escalated to the user immediately.
- Strict-first and fix-forward: when strict contracts expose an objectively
  incorrect behavior, fix production code immediately.

## Contract Guardrails (Anti-Weak-Tests)

- Assert externally meaningful behavior first (URL/state transitions, returned
  values, event payloads), not incidental internals.
- Assert what is correct from first principles, not what current implementation
  quirks happen to do.
- Do not assert private state (e.g. private maps/fields) in contract suites.
- Do not lock contracts to logging message text or other debug-only signals.
- Avoid asserting internal call ordering unless it is user-visible behavior.
- If behavior is ambiguous, ask the user immediately; do not defer decisions to
  a future review queue.
- Do not weaken strict assertions after a failing run in order to mirror current
  implementation behavior.
- Never weaken a strict assertion solely because legacy tests or current
  implementation disagree.

## Strictness Escalation Rule

When a strict, first-principles assertion fails:

1. Classify the failure:
    - `objective defect`: expectation is correct from first principles.
    - `ambiguous`: multiple reasonable behaviors exist and product intent is
      unclear.
2. For `objective defect`, keep the strict assertion and fix production code in
   the same slice.
3. For `ambiguous`, stop and ask the user immediately for a product-level
   decision.
4. Do not merge relaxed interim assertions while waiting for ambiguity
   resolution.

## Legacy Suite Role During Migration

- Legacy tests are a temporary safety net and discovery aid, not the contract
  source of truth.
- New strict contract tests should be authored and executed while legacy tests
  remain in-tree.
- If strict contracts and legacy tests disagree, prefer first-principles
  correctness and treat the mismatch as:
    - an implementation defect to fix now, or
    - an ambiguous behavior that requires an immediate user decision.
- Legacy tests that encode superseded/brittle behavior are marked
  `READY_TO_DELETE_AFTER_SIGNOFF` once replacement coverage is validated.

## Target Test Architecture

```text
vormaclient/client/src/contracts/      # stable public behavior guarantees
vormaclient/client/src/integration/    # cross-module flows and lifecycle wiring
vormaclient/client/src/regressions/    # named bug reproductions with issue links
vormaclient/client/src/legacy/         # temporary holding area during migration
```

Notes:

- Existing `src/*.test.ts` files are currently treated as legacy.
- Migration is incremental; migrated legacy files remain in-tree with a first
  line marker `READY_TO_DELETE_AFTER_SIGNOFF` until explicit signoff.

## Migration Workflow

1. Pick one behavior slice (e.g. loading lifecycle, redirects, scroll restore).
2. Add/expand strict first-principles contract coverage for that slice in
   `src/contracts/`.
3. Fill uncovered matrix cells in that same slice.
4. Run the strict contract slice and full legacy-inclusive suite.
5. If strict tests expose objective defects, fix source code immediately and
   re-run both suites.
6. For truly ambiguous failures only, pause and ask the user immediately before
   adding or changing assertions.
7. Map overlapping legacy cases to one of:
    - `retained` (still needed),
    - `covered-by-contract`,
    - `duplicate/remove`.
8. Once parity is verified, mark overlapping legacy files with
   `READY_TO_DELETE_AFTER_SIGNOFF` at line 1 and defer physical deletion until
   explicit signoff.
9. Update `TEST_REWRITE_PROGRESS.md` with exact changes and next-safe-step.
10. Update `TEST_REWRITE_LEGACY_INVENTORY.md` statuses for every touched legacy
    file.

## Coordination Files

- `TEST_REWRITE_PLAN.md`: strategy and migration rules.
- `TEST_REWRITE_PROGRESS.md`: current phase, verification log, takeover steps.
- `TEST_REWRITE_LEGACY_INVENTORY.md`: per-legacy-file migration ledger.
- `TEST_REWRITE_CASE_PARITY.md`: case-level parity checklist for every legacy
  `it/test` case.
- `TEST_REWRITE_SHOULD_MAP.json`: generated machine-readable map of all legacy
  and contract test case titles (with `isShould` and normalized intent keys).
- `TEST_REWRITE_BUG_CANDIDATES.md`: resolved first-principles decisions and
  source fixes from strict-contract failures (no deferred open queue).

## Coverage Matrix Backlog

- `Loading state/event semantics`: contract + integration (started).
- `Focus/visibility revalidation`: contract (started).
- `Navigation outcomes (success/abort/redirect)`: contract.
- `Link/prefetch behavior`: contract + regression.
- `Form submissions/dedupe/revalidate`: contract + regression.
- `History + scroll restoration`: integration.
- `Initialization/HMR/module loading`: integration.
- `Head element updates`: integration + regression.
- `Error boundary and error prioritization`: contract + regression.

## Definition Of Done

- Contract suite covers all public client APIs and core state transitions.
- Regression suite contains all known bug classes as named scenarios.
- Legacy tests are either `READY_TO_DELETE_AFTER_SIGNOFF` (awaiting signoff
  deletion) or intentionally retained with rationale.
- Full client test run is green with reduced test sprawl and clear ownership.
- A major internal refactor can be attempted with contract suite as the
  acceptance gate.

## Operating Commands

- Run contract slice only:
    - `pnpm vitest --run vormaclient/client/src/contracts`
- Run full client suite:
    - `pnpm vitest --run vormaclient/client/src`
