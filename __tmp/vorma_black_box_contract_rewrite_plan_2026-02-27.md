# Vorma Black-Box Contract Rewrite Plan (2026-02-27)

This plan defines how to make tests authoritative for first-principles behavior
while avoiding internal-coupled or bundle-bloating assertions.

Migration denominator baseline:

- Full legacy rows: `818`
- `zero_value_due_to_duplication_or_non_observability`: `134`
- `must_be_ported_to_black_box`: `684`
- Two-category full audit artifact:
  `__tmp/vorma_two_category_full_audit_2026-02-27.md`
- Must-be-ported row-level execution tracker:
  `__tmp/vorma_must_be_ported_to_black_box_tracker_2026-02-27.md`

Handled accounting rules:

- `zero_value_due_to_duplication_or_non_observability` is implicitly handled by
  final categorization rule.
- `must_be_ported_to_black_box` is handled only when row status is `handled` in
  the must-be-ported tracker.

## Goal

- Replace internal-shape-coupled expectations with black-box observable contract
  tests.
- Keep normative framework behavior coverage strong.
- Remove poisonous expectations that force backend-contract validation/fallbacks
  or server-runtime resiliency for browser-only runtime code.

## Non-Negotiable Rules

- Trust backend contracts for backend-owned payload shape and semantics.
- Browser runtime is browser-only; do not add server guards for runtime paths.
- Contract expectations are changed only with explicit user sign-off when
  semantics are ambiguous.
- No compat layers, no transitional shims, no legacy seam restoration.
- No tests asserting internal state shape, helper existence, reducer internals,
  event-plan internals, or implementation sequence that is not publicly
  observable.
- UI adapter behavior tests stay dist-only and import through `npm_dist` adapter
  entrypoints (`vorma/react`, `vorma/preact`, `vorma/solid`), never source-path
  aliases.
- Freeze legacy suites during migration:
    - Do not continue refactoring old contract/unit test internals.
    - Port normative behaviors into `tests/black_box` first.
    - Delete legacy tests/harness in consolidation pass after black-box parity.

## Phase 1: Extract All Existing “Should” Statements

- [x] Enumerate all tests under `typescript/vorma/*` (source + dist).
    - Artifact: `__tmp/vorma_test_should_mapping_2026-02-27.md`
- [x] For each test, write a single plain-language “should” statement.
    - Artifact: `__tmp/vorma_test_should_mapping_2026-02-27.md`
- [x] Record test file + test name + “should” statement in a mapping table.
    - Artifact: `__tmp/vorma_test_should_mapping_2026-02-27.md`
- [x] Mark each statement as one of:
    - [x] public black-box observable behavior
    - [x] internal-coupled behavior
    - [x] backend-contract distrust behavior
    - [x] non-browser resiliency behavior
    - [x] ambiguous (requires user sign-off)
    - Note: this is an initial heuristic pass. Manual confirmation is pending.

## Phase 2: Build Canonical Contract Matrix

- [x] Produce one canonical behavior matrix grouped by public API surface:
    - [x] navigation
    - [x] revalidation
    - [x] submissions
    - [x] client loaders (parallelism + ownership + commit behavior)
    - [x] route-props scoped readers/hooks
    - [x] global readers/selectors
    - [x] link behavior (click semantics + prefetch intent)
    - [x] head/title/css/module preload side effects
    - Artifact: `__tmp/vorma_black_box_contract_matrix_2026-02-27.md`
- [x] For each behavior, define the observable inputs and outputs only.
    - Artifact: `__tmp/vorma_black_box_contract_matrix_2026-02-27.md`
- [x] Flag any behavior that currently depends on internal representation.
    - Artifacts:
        - `__tmp/vorma_contract_internal_dependency_map_2026-02-27.md`
        - `__tmp/vorma_test_poisonous_manual_triage_2026-02-27.md`
- [ ] Pause for user sign-off on every ambiguous or suspicious item before
      contract expectation edits.

## Phase 3: Write New Authoritative Black-Box Contract Suite

- [x] Create a new contract suite file (single-file is fine initially) that
      tests only black-box observables.
    - Artifact:
        - `typescript/vorma/client/src/tests/black_box/client.black_box_authoritative.test.ts`
- [x] Establish a passing authoritative seed suite before legacy-port pruning.
    - Current state: 131 black-box tests passing in
      `client.black_box_authoritative.test.ts`.
- [x] Establish dist-only adapter black-box seed coverage via `npm_dist`
      imports.
    - Artifact:
        - `typescript/vorma/client/src/tests/dist/npm_dist_black_box_authoritative.test.ts`
    - Current state: 21 dist black-box adapter tests passing.
- [x] Track migration directly against the full extracted "should" mapping.
    - Artifact:
        - `__tmp/vorma_should_audit_tracker_2026-02-27.md`
        - `__tmp/vorma_two_category_full_audit_2026-02-27.md`
        - `__tmp/vorma_must_be_ported_to_black_box_tracker_2026-02-27.md`
- [ ] Remove internal-runtime helper wiring from legacy contract harness.
    - Note: deferred while legacy suites are frozen; black-box suite does not
      depend on this harness.
    - Artifact:
        - `typescript/vorma/client/src/tests/contracts/contract_test_harness.ts`
- [ ] Encode backend-contract trust explicitly:
    - [ ] do not test frontend fallback logic for backend-owned violations
    - [ ] do not test frontend panic/assert branches for backend-owned
          violations
- [ ] Encode browser-only runtime assumptions explicitly:
    - [ ] no required server-guard behavior tests for runtime code
- [x] Keep strict per-tick transition assertions where behavior is observable.
    - Current coverage examples:
        - submit -> auto-revalidate has no loading-gap snapshots
        - redirect chains preserve loading continuity
        - same-document no-op does not emit duplicate idle snapshots
- [ ] Keep parallel client-loader behavior assertions where behavior is
      observable (concurrency and commit semantics).

## Phase 4: Migrate and Prune Legacy Tests

- [ ] Compare each old test against the new canonical matrix.
- [ ] Keep old tests only if they verify unique black-box behavior not already
      covered.
- [ ] Rewrite old tests that are valid but internal-coupled.
- [ ] Delete old tests that are poisonous/counterproductive:
    - [ ] backend-contract distrust assertions
    - [ ] server-runtime resiliency assertions for browser-only paths
    - [ ] internal seam/shape assertions with no observable value
    - Partial progress:
        - removed window-unavailable assertions from
          `typescript/vorma/client/src/tests/unit/events_platform.test.ts`
        - removed backend-contract-violation assertions from
          `typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts`
          and
          `typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts`
- [ ] Record each deletion/rewrite with one-line rationale.

## Phase 5: Acceptance Criteria

- [ ] New contract suite covers all canonical public behaviors.
- [ ] No contract test requires backend-contract fallback/panic logic in
      frontend runtime.
- [ ] No contract test requires non-browser runtime resiliency behavior.
- [ ] No contract test depends on internal state shape.
- [ ] Dist/source split remains compliant:
    - [ ] source checks via `make tstest-source`
    - [ ] dist checks via `make tstest-dist`
- [ ] UI adapter validation remains dist-only via
      `typescript/vorma/client/vitest.dist.config.ts` with `npm_dist` imports.
- [ ] Any contract expectation semantic change has explicit user sign-off noted.

## Execution Order

- [x] Complete Phase 1 mapping first.
- [x] Complete Phase 2 matrix second.
- [ ] Get user confirmation on ambiguous/suspicious items.
- [ ] Then implement Phases 3 and 4.
- [ ] Run acceptance checks only after rewrite is done.
