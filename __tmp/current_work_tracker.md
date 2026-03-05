# Current Work Tracker

## Contracts / Assumptions

- Parsed config types are constructor-only (parse, don't validate) and not
  directly constructible by consumers.
- Internal config values must stay repository-relative/logical and must not
  store machine-absolute paths.
- `.wavedist` is Wave-owned at runtime/build time, but first-run bootstrap must
  ensure `.wavedist/static/.keep` exists for Go embed compatibility.
- E2E contracts should only assert Wave/Vorma-owned behavior, not Vite-internal
  transport recovery semantics.

## Completed

- Removed the out-of-scope E2E test
  `recovers HMR stream after forced websocket transport drop` from
  `internal/e2e/framework.integration.shared.ts`.
- Removed the websocket-drop tracking helpers that existed only for that
  out-of-scope test.
- Fixed E2E runtime harness port drift by pinning
  `__WAVE_PORT_HAS_BEEN_SET=true` in runtime env
  (`internal/e2e/runtime_harness.ts`), so fixture base URL and actual app port
  cannot silently diverge.
- Fixed cross-lane port-family collisions by replacing random fallback port
  selection with lane-family probing (`+2000` stride) for both app and Vite
  ports in `internal/e2e/runtime_harness.ts`.
- Added a fast non-E2E contract test to guard the harness port-pin invariant
  (`internal/e2e/runtime_harness_contract_test.go`).
- Verified `make gotestloud` passes in user environment after harness fixes.
- Verified full parallel `make e2e-test` passes in user environment
  (`93 passed`).
- Fixed current `make staticcheck` findings:
    - SA5011 nil-deref warnings in `internal/vormaruntime/test_helpers_test.go`.
    - Unused symbols in routeparse/testkit/vormagogen/builder test files.
- Verified `make full-gate` passes end-to-end in user environment
  (`RELEASE GATE: PASS`).
- Re-formatted TypeScript files with `make tsfmt`.

## Open Items

- Continue log-shape alignment against `main` spirit for user-facing dev logs
  where behavior still diverges.
