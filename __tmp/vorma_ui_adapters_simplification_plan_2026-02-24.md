# Vorma UI Adapters Simplification Plan (2026-02-24)

## Scope

Audit and simplify `typescript/vorma/ui-adapters/*` for unnecessary complexity,
duplication, latent bugs, and avoidable runtime overhead while preserving
adapter contracts.

## Checklist

### 1) Dedupe route/location listener bootstrap across adapters

- [x] Extract one shared listener initializer used by React/Preact/Solid root
      outlets.
- [x] Preserve once-only listener registration semantics across remounts.
- [x] Keep route-change scroll-application behavior unchanged.
- [x] Validate with dist adapter root-outlet suites.

### 2) Remove render-time side effects from Solid root outlet

- [x] Move root listener/store bootstrap out of render body and into mount-time
      initialization.
- [x] Remove one-shot branch-input signal forwarding indirection.
- [x] Keep Solid route/outlet identity behavior unchanged.
- [x] Validate with dist root-outlet runtime-state + branches suites.

### 3) Remaining non-low-hanging opportunities

- [x] Consolidate duplicated typed data-hook factories in
      `react/src/helpers.ts`, `preact/src/helpers.ts`, and
      `solid/src/helpers.ts` via a shared adapter-agnostic core with thin
      framework wrappers.
- [x] Consolidate duplicated typed-link composition in `react/src/link.tsx`,
      `preact/src/link.tsx`, and `solid/src/link.tsx` while preserving
      framework-native event/types surfaces.
- [ ] Evaluate whether adapter store bootstrapping can share a common internal
      engine contract (framework-specific subscription glue only) without
      hurting debuggability. Status: pending (not yet decided/scoped for this
      pass).

## Validation

- [x] `make tscheck-fw-react`
- [x] `make tscheck-fw-preact`
- [x] `make tscheck-fw-solid`
- [x] `make npmbuild`
- [x] `make tstest-dist`
- [x] `make tstest-source`

## Progress Log

- [x] Plan created.
- [x] Step 1 complete.
- [x] Step 2 complete.
- [x] Step 3.1 complete.
- [x] Step 3.2 complete.
