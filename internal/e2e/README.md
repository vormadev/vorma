# Framework E2E Harness

This directory contains standalone end-to-end integration coverage for wave +
vorma across both frontend and backend runtime paths.

It does not depend on `internal/site`.

## Scope

- Runs the same browser tests against:
    - `dev` mode (`go run ./backend/cmd/build --dev`)
    - `prod` mode (build + compiled binary under `backend/dist/main`)
- Runs across UI adapters:
    - `solid` (full stress suite)
    - `react` (full stress suite)
    - `preact` (full stress suite)
- Validates cross-stack behavior:
    - loader rendering and nested route data
    - dynamic params + query decoding
    - mutation round-trips (simple and complex payloads)
    - deterministic stress-matrix action coverage
    - multi-seed randomized sweeps + soak sequences
    - broader concurrent mutation/navigation chaos
    - dev-time file-change/HMR race convergence
    - dev-time websocket/HMR transport-drop recovery
    - backend restart recovery during in-flight navigation
    - forced fetch/xhr abort + timeout chaos
    - multi-tab shared-session state races
    - form action redirects
    - loader redirect-chain resolution
    - odd-shape payload rendering invariants
    - loader error boundaries
    - client navigation with server state continuity

## Layout

- `framework.integration.shared.ts`: shared browser test definitions.
- `framework.integration.*.spec.ts`: lane wrappers for each mode+adapter pair.
- `runtime_harness.ts`: boots fixture app in each mode and manages lifecycle.
- `overlay_templates/`: additive E2E fixture layer (`.tmpl`/`.txt`) applied on
  top of bootstrap-generated temporary apps.
- No committed runnable fixture app: each fixture is generated into a temporary
  directory and dependencies are installed there from pnpm's store.

## Run

1. `make e2e-setup`
2. `make e2e-test`

By default, `make e2e-test` now runs the full `dev/prod × solid/react/preact`
lane matrix in parallel (one isolated temp fixture per lane).

Granular setup targets:

- `make e2e-install`
- `make e2e-install-browsers`

Targeted mode runs:

- `make e2e-test-dev`
- `make e2e-test-prod`

Targeted adapter runs:

- `VORMA_E2E_UI_ADAPTERS=solid make e2e-test`
- `VORMA_E2E_UI_ADAPTERS=react make e2e-test-dev`
- `VORMA_E2E_UI_ADAPTERS=preact make e2e-test-prod`

Worker tuning:

- `VORMA_E2E_WORKERS=6 make e2e-test`
