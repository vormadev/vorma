# Vorma Client Simplification Plan (2026-02-23)

## Scope

Improve whole-runtime simplicity, dedupe, and aggregate client bundle efficiency
for `typescript/vorma/client/src` without changing framework behavior.

## Checklist

### 1) Remove dead redirect plumbing

- [x] Remove unused `isPrefetch` parameter threading from redirect fetch path.
- [x] Remove unused `newURL` value from redirect target parsing.
- [x] Keep redirect behavior parity for navigation/prefetch promotion paths.
- [x] Add/adjust unit tests for redirect path signatures and behavior.

### 2) Consolidate request-body serialization logic

- [x] Introduce one shared request-body normalization/serialization helper.
- [x] Reuse it from `app/helpers.ts` (`resolveVormaRequestBody`).
- [x] Reuse it from `core/redirects.ts` (`buildRedirectRequestInit`).
- [x] Preserve content-type and method/body semantics.
- [x] Add/adjust tests to verify parity across both callsites.

### 3) Eliminate duplicated CSS application responsibilities

- [x] Choose one authoritative stage for stylesheet application.
- [x] Keep prefetch behavior intentional (decide whether prefetch should apply
      CSS or only preload).
- [x] Remove duplicate `AssetManager.applyCSS` invocation path.
- [x] Codify prefetch commit boundary: pure prefetch may warm internal framework
      state (for example build ID and internal cache state), but must not mutate
      committed page state (stylesheet/title/head/history/route-change commit).
- [x] Add contract tests for navigation + prefetch CSS behavior after refactor.
- [x] Add contract tests that assert internal warm-up is allowed while
      page-committed mutations are blocked during pure prefetch.

### 4) Make debug journal production-optional

- [x] Gate debug journal runtime machinery behind a compile-time/dev guard.
- [x] Keep public debug APIs stable but no-op in production builds.
- [x] Ensure navigation lifecycle still works identically when disabled.
- [x] Add tests for dev-enabled and prod-disabled behavior.
- [x] Add artifact-level production-bundle test that asserts debug transition
      machinery is excluded when `import.meta.env.DEV` is `false`.

### 5) Flatten navigation/render orchestration layers

- [ ] Collapse repetitive `execution plan -> command array -> executor` loops
      where they do not provide unique value.
- [ ] Keep pure decision functions where they protect tricky invariants.
- [ ] Reduce intermediate object creation in hot paths.
- [ ] Preserve existing state-machine contract tests while reducing internal
      indirection.

### 6) Reduce reason-string runtime payload

- [ ] Replace broad runtime string-reason unions with compact constants or
      dev-only mappings.
- [ ] Keep debugging detail available in tests/dev builds.
- [ ] Ensure runtime behavior does not depend on string literal text.

### 7) Simplify history integration surface

- [ ] Extract minimal internal history adapter contract used by runtime.
- [ ] Decouple runtime from direct third-party `history` API shape.
- [ ] Evaluate replacing dependency usage with a smaller internal adapter where
      feasible.
- [ ] Keep POP sequencing and hard-reload fallback behavior fully covered.

### 8) Shift matcher/route registration work toward build output

- [ ] Define precompiled route/matcher payload shape.
- [ ] Initialize runtime from precompiled data rather than dynamic progressive
      pattern registration where possible.
- [ ] Preserve lazy/progressive enhancement behavior when manifest is absent.
- [ ] Add tests for both precompiled and fallback runtime paths.

## Execution Order

1. Dead redirect plumbing + request-body dedupe.
2. CSS responsibility unification.
3. Debug journal gating.
4. Navigation/render layer flattening and reason-string compression.
5. History adapter simplification.
6. Matcher build-output shift.

## Validation Gate (each step)

- [x] `pnpm prettier --write <edited files>`
- [x] `pnpm tsc -p typescript/vorma/client/tsconfig.json --noEmit`
- [x] Targeted vitest suites for changed modules.
- [x] `make tstest-source`

## Progress Log

- [x] Plan created.
- [x] Step 1 complete.
- [x] Step 2 complete.
- [x] Step 3 complete.
