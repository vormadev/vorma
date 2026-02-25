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
    - `react` (core + chaos + HMR)
    - `preact` (core + chaos + HMR)
- Validates cross-stack behavior:
    - loader rendering and nested route data
    - dynamic params + query decoding
    - mutation round-trips (simple and complex payloads)
    - deterministic stress-matrix action coverage
    - long-run randomized request/navigation sequences
    - broader concurrent mutation/navigation chaos
    - dev-time file-change/HMR race convergence
    - form action redirects
    - loader error boundaries
    - client navigation with server state continuity

## Layout

- `framework.integration.spec.ts`: browser test matrix for mode + adapter.
- `runtime_harness.ts`: boots fixture app in each mode and manages lifecycle.
- `fixture_app/`: isolated wave/vorma app intentionally built for E2E coverage.

## Run

1. `make e2e-setup`
2. `make e2e-test`

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
