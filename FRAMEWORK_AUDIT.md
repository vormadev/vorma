# Framework Audit Tracker

## Objective

Run a from-scratch audit of the Vorma + Wave frameworks for correctness,
resilience, API quality, performance opportunities, and test quality.

## Scope

- `vorma.go` (root package surface)
- `vormabuild/*`
- `vormaruntime/*`
- `wave/*`
- `wave/tooling/*`
- `bootstrap/*` (including generated defaults/templates)
- `vormaclient/client/*`
- `vormaclient/react/*`
- `vormaclient/preact/*`
- `vormaclient/solid/*`
- `vormaclient/vite/*`
- `vormaclient/create/*`

## Constraints

- Findings are reported as an exhaustive plain list.
- No severity labels or triage categories.
- Tests must not be weakened.
- For this audit, correctness and resilience take precedence over test-suite
  runtime.

## Change Authorization Policy

- Automatic fixes are allowed only when the answer is obvious and
  first-principles-correct with no reasonable semantics/design alternative.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

| Date       | Change                                                                                                  | Bucket                         | Approval Status  |
| ---------- | ------------------------------------------------------------------------------------------------------- | ------------------------------ | ---------------- |
| 2026-02-14 | `vormaclient/vite/vite.ts` object-input merge preserves user entries with collision-free internal keys. | design/semantics (non-obvious) | approved by user |

## Handoff Ledger

### Full-Pass Matrix (Handoff-Safe)

Status values are strict: `done` means complete package-wide pass, `not done`
means the pass is still pending.

| Package                | Surface/API pass | Correctness pass | Performance pass | Test-quality pass |
| ---------------------- | ---------------- | ---------------- | ---------------- | ----------------- |
| `vorma.go`             | not done         | not done         | not done         | not done          |
| `vormabuild/*`         | not done         | not done         | not done         | not done          |
| `vormaruntime/*`       | not done         | not done         | not done         | not done          |
| `wave/*`               | not done         | not done         | not done         | not done          |
| `wave/tooling/*`       | not done         | not done         | not done         | not done          |
| `bootstrap/*`          | not done         | not done         | not done         | not done          |
| `vormaclient/client/*` | not done         | not done         | not done         | not done          |
| `vormaclient/react/*`  | not done         | not done         | not done         | not done          |
| `vormaclient/preact/*` | done             | done             | done             | done              |
| `vormaclient/solid/*`  | not done         | not done         | not done         | not done          |
| `vormaclient/vite/*`   | done             | done             | done             | done              |
| `vormaclient/create/*` | done             | done             | done             | done              |

## Findings Log

1. Bootstrap server template drops `http.ListenAndServe` errors.
    - File: `bootstrap/tmpls/cmd_app_main_go_tmpl.txt:12`
    - Status: open
    - Problem: generated apps can fail to bind/start without surfacing the
      startup error through process exit semantics.
2. Bootstrap example route template introduces unsynchronized shared mutable
   state.
    - File: `bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt:11`
    - File: `bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt:26`
    - Status: open
    - Problem: generated apps get a default request-handler race footgun.
3. Bootstrap package install path spawns one package-manager process per
   dependency.
    - File: `bootstrap/bootstrap.go:376`
    - Status: open
    - Problem: avoidable repeated lock-resolution/install overhead during
      scaffolding.
4. Docker bootstrap path does not guard empty `NodeMajorVersion`.
    - File: `bootstrap/bootstrap.go:79`
    - File: `bootstrap/tmpls/dockerfile_tmpl.txt:3`
    - Status: open
    - Problem: generated Dockerfile can be invalid (`setup_.x`).
5. Wave dev config validation does not validate `Watch.HealthcheckEndpoint` path
   shape.
    - File: `wave/tooling/builder_validation.go:12`
    - Related runtime use: `wave/tooling/devserver_readiness.go:38`
    - Status: open
    - Problem: malformed endpoints can produce invalid readiness probe URLs.
6. `wave.GetParsedConfig()` exposes mutable internal framework config by
   pointer.
    - File: `wave/runtime_framework.go:108`
    - Status: open
    - Problem: external callers can mutate runtime internals after parse.

## Test Gap Notes

1. No bootstrap regression test currently asserts generated `cmd/serve/main.go`
   handles `http.ListenAndServe` errors.
2. No bootstrap regression test currently protects against unsynchronized
   mutable `count` state in generated example routes.
3. No bootstrap regression test currently enforces non-empty `NodeMajorVersion`
   for docker-target scaffolds.
4. No validation regression test currently enforces `Watch.HealthcheckEndpoint`
   path-shape requirements in `wave/tooling`.
5. No contract test currently enforces immutability expectations for
   `wave.GetParsedConfig()` consumers.

## Next Queue

1. Complete full passes for `vormaclient/client/*`, `vormaclient/react/*`, and
   `vormaclient/solid/*`.
2. Complete full passes for `vorma.go`, `vormabuild/*`, `vormaruntime/*`,
   `wave/*`, `wave/tooling/*`, and `bootstrap/*`.
3. Convert open findings (`1` through `6`) into fixes + regression tests.
4. Move package-by-package through remaining full-pass matrix and keep this
   tracker synchronized with explicit `done`/`not done` states.
