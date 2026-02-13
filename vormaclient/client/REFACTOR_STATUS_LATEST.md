# Refactor Status Latest

Last updated: 2026-02-13

## Open Items

- Confirm IDE diagnostics are clean after serializing `tsc` declaration emits in
  `internal/scripts/buildts`; if not, capture exact error set and map each to
  owning config/project.
- Decide whether additional compiled-output root-outlet behavioral tests should
  be added beyond current
  `client-runtime + typed-link + addClientLoader + root-outlet route-change/scroll`.
- Evaluate extracting shared runtime-state/outlet remount logic into a common
  internal helper to keep `react/preact/solid` behavior locked together.
- Measure and tune `internal/scripts/buildts` non-`tsc` stage parallelism on CI
  hardware to confirm best runtime/CPU tradeoff now that declaration emits are
  serialized for determinism.
- Confirm whether `onRegistrationError` should remain as an escape hatch in
  `__registerClientLoaderForAdapter` now that default behavior throws.
- Clarify expected Solid root-outlet behavior when route changes between two
  non-error component functions at the same index, then lock it with an explicit
  test assertion.
- Keep `dist_tests/ambient_modules.d.ts` declarations in sync with
  `vitest.dist.config.ts` alias modules to avoid IDE/typecheck drift.
