# vormaclient/client High-Level Plan

Purpose: keep a forward-only execution plan for client runtime architecture and
test strategy.

## Handoff Freshness Rules

- This file is the canonical execution plan for `vormaclient/client`.
- Keep this file future-looking only.
- Do not add implemented/completed/changelog logs.
- Update milestones when scope or sequence changes.

## End State

- Maintainable client runtime with clear module boundaries.
- Deterministic navigation behavior under races and cancellation churn.
- Strict test contracts as the behavioral source of truth.
- No legacy compatibility cruft in the public or internal surface.

## Constraints

- No builder-pattern API additions.
- No duplicate API paths for the same common task.
- Prefer explicit decisions and explicit execution phases in runtime internals.
- Refactors must preserve or improve type safety and test clarity.

## Workstreams

### 1) Navigation Runtime Architecture

1. Keep navigation lifecycle split into explicit decision and execution phases.
2. Remove residual duplicated branch trees across navigation entry points.
3. Keep target ownership checks explicit at every async boundary.

### 2) Fetch/Loader Execution

1. Keep fetch-route-data flow split by responsibility (match, skip, server,
   normalize).
2. Consolidate repeated invariants into shared validation primitives.
3. Ensure malformed matcher or manifest inputs fail fast with explicit errors.

### 3) State and Slot Management

1. Keep slot lifecycle transitions explicit and centralized.
2. Prevent stale entry mutation by enforcing identity checks before side
   effects.
3. Keep mutation primitives DRY and side-effect ordering consistent.

### 4) Public Boundary Hygiene

1. Remove unnecessary passthrough/export facades.
2. Keep runtime-only and buildtime-only boundaries explicit and enforced.
3. Prevent accidental internal API exposure through convenience re-exports.

### 5) Test and Conformance Strategy

1. Continue race-focused tests for navigation replacement, aborts, and
   prefetch/revalidation interaction.
2. Keep deterministic contracts around head/scroll/history behavior.
3. Add stress-style randomized execution sequences where they reveal stale-state
   risks.

## Next Milestones

1. Final boundary pass over navigation modules to remove remaining overlap.
2. Expand randomized race contracts around same-target replacement behavior.
3. Tighten runtime/buildtime boundary assertions for route and manifest data.
4. Document and enforce a minimal internal module graph for navigation runtime.

## Exit Criteria

- Runtime logic is organized around explicit decision/execution stages.
- Stale ownership/race protections are enforced by tests across all nav paths.
- Internal module boundaries are cohesive and minimal.
- Test suite guards the intended behavior without mirroring implementation
  hacks.
