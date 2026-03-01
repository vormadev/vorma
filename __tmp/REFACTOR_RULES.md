# Refactor Rules (Binding)

This file captures the rules I will follow while running tests and fixing the
rewrite.

## Core Intent

- Build the most correct first-principles runtime behavior, in as small of a
  footprint as possible to keep bundle size small.
- Do not add compatibility layers, transitional shims, or legacy wrapper seams.
- Keep contract tests as the end-user behavior source of truth.

## Execution Workflow

1. Run `make tstest-source` first.
2. Triage failures one by one.
3. For each failure:
    - If it reflects intended behavior, fix runtime/source.
    - If it reflects obsolete internal-shape assumptions, update the test.
4. After source tests are stable, run `make tstest-dist`.
5. Repeat the same triage discipline for dist/adapters.

## Contract-Test Rules

1. Treat contract tests as normative for end-user behavior.
2. If any contract test looks suspicious, bug-memorializing, or
   first-principles-wrong:
    - Stop.
    - Discuss with user before changing the contract expectation.
3. Do not edit contract expectations without explicit user sign-off.
4. Non-contract/internal tests may be updated for internal shape changes unless
   they reveal a real behavior bug.
5. If any test requires the frontend to be resilient against backend contract
   violations, the test is wrong and the test needs to be changed. The frontend
   should merely assume the backend is following the required contracts and
   neither fallback nor validate/panic on backend violations (zero frontend code
   should be written to validate or double check that the backend is following
   its contracts).

## No-Shim / No-Compat Rules

1. Do not restore removed internal helpers just to make tests pass.
2. Do not add wrapper APIs that emulate old internals.
3. Do not add no-op placeholder internals to satisfy legacy tests.
4. Keep one canonical implementation path for each behavior.
5. If tests require old seams, rewrite tests toward canonical internals instead
   of reintroducing seams.

## Backend-Contract Trust Rules

1. Frontend runtime must trust backend contracts for backend-owned data shapes.
2. For backend-owned contract fields, do not add fallback/recovery logic in
   frontend runtime.
3. For backend-owned contract fields, also do not add frontend panic/assert
   validation branches; assume the contract is satisfied.
4. Backend contract enforcement belongs in backend tests and backend build-time
   invariants, not frontend runtime checks.
5. Keep frontend runtime checks only for non-backend-owned surfaces
   (user-controlled input, browser APIs, environment conditions).

## Client-Side Persistence Rules

1. Browser-owned persisted state (`sessionStorage`, `localStorage`, IndexedDB,
   Cache API, cookies) is not backend-owned; treat it as mutable/tamperable.
2. For client-owned persisted state, perform minimal safe parse + shape checks
   before use.
3. Keep validation minimal and size-conscious:
    - parse safely
    - validate only fields actually required for current behavior
    - ignore/drop malformed entries rather than adding broad fallback systems
4. Do not apply backend-contract trust rules to client-owned persisted state.
5. Add black-box regression tests for client-owned persistence parsing paths.

## Browser-Only Runtime Rules

1. `typescript/vorma/client/src/runtime.ts` is browser-only runtime code.
2. Do not add environment guards like:
    - `typeof window === "undefined"`
    - `typeof document === "undefined"`
    - optionalized browser globals purely for non-browser fallback behavior
3. Assume browser APIs are present for supported runtime paths.
4. If a path truly must be portable across environments, move that logic to a
   separate non-browser runtime module instead of bloating browser runtime with
   server guards.

## Runtime Audit Criteria (runtime.ts)

1. Remove fallback defaults for required backend payload fields in route-data
   decode and router-data projection paths.
2. Remove frontend payload-shape assertions/panics for backend-owned contract
   fields; do not replace fallbacks with panic branches.
3. Remove frontend acceptance branches for impossible backend import/dependency
   URL schemes (example: `blob:`/`data:` for backend-produced module paths).
4. Keep graceful behavior only for truly non-contract, user-controlled browser
   surfaces (example: storage API availability), not backend-owned payload
   contracts.

## “Do Not Write To Tests” Guardrails

1. Never force runtime behavior to match accidental old internals.
2. Never patch tests to mirror a bug unless user explicitly confirms that
   behavior is intended.
3. Every changed expectation must be recorded with a short reason so intent is
   explicit.

## Audit Requirement

Before and during triage, audit unstaged changes for:

- Reintroduced wrappers/seams.
- Duplicate state paths.
- Legacy helper aliases.
- No-op test-only stubs.

If found, remove them and keep the canonical first-principles path.

## Communication Protocol

1. If a suspicious contract appears, pause and ask user.
2. If a fix requires changing a public behavior contract, pause and ask user.
3. Otherwise continue execution without adding compatibility scaffolding.

## Tracker And Reporting Rules

1. Category model is fixed to exactly:
    - `zero_value_due_to_duplication_or_non_observability`
    - `must_be_ported_to_black_box`
2. Reporting must distinguish category from work status.
3. `handled` semantics:
    - `zero_value_due_to_duplication_or_non_observability`: implicitly handled
      by final categorization rule
    - `must_be_ported_to_black_box`: handled only when row status is `handled`
      in `__tmp/vorma_must_be_ported_to_black_box_tracker_2026-02-27.md`
4. Progress reports must include:
    - per-category handled numerator/denominator
    - overall handled numerator/denominator across full 818 rows
5. Do not claim `must_be_ported_to_black_box` rows are handled by category label
   alone.
