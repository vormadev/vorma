# Framework Audit Tracker

## Objective

Run a from-scratch audit of the Vorma + Wave frameworks for correctness,
resilience, API quality, performance opportunities, and test quality.

## Scope

- In scope packages for this audit:
    - `github.com/vormadev/vorma` (root package, `vorma.go`)
    - `github.com/vormadev/vorma/vormabuild`
    - `github.com/vormadev/vorma/vormaruntime`
    - `github.com/vormadev/vorma/vormaclient`
    - `github.com/vormadev/vorma/wave`
    - `github.com/vormadev/vorma/wave/tooling`
    - `github.com/vormadev/vorma/bootstrap` (template/default setup review)
- In scope non-Go client surface (explicit):
    - `vormaclient/client/*`
    - `vormaclient/react/*`
    - `vormaclient/preact/*`
    - `vormaclient/solid/*`
    - `vormaclient/vite/*`
    - `vormaclient/create/*`
- Out of scope unless needed by a concrete finding:
    - unrelated kit/lab utilities not on a call path from the packages above
    - app-specific behavior in `internal/site` unless it indicates framework
      bugs

## Constraints

- Findings are reported as an exhaustive plain list.
- No severity labels or triage categories.
- Tests must not be weakened or watered down.
- Prefer correctness/resilience over test-suite speed when tradeoffs conflict.

## Audit Questions

1. Are there bad/questionable designs in framework internals?
2. Are there user APIs that are confusing or needlessly complex?
3. Does `bootstrap` impose needless boilerplate that framework defaults could
   absorb while preserving flexibility?
4. Are there latent bugs or correctness violations?
5. Are there obvious performance wins with low complexity/risk?
6. Is test coverage missing for behavior that should obviously be protected?
7. Do any tests claim guarantees they do not actually verify?
8. Are there hidden coupling points that increase fragility (cross-package
   assumptions, ordering assumptions, global state)?
9. Are there footguns where API behavior is surprising relative to naming?
10. Are there config defaults that are too presumptive vs. app-level override
    needs?

## Method

1. Surface/API pass:
    - exported APIs, naming, ergonomics, consistency
2. Correctness/resilience pass:
    - state transitions, lifecycle, race potential, stale/cached state, error
      propagation
3. Perf pass:
    - avoidable work in hot paths, unnecessary process spawn/IO, serialization
      points
4. Test pass:
    - coverage gaps, false-confidence tests, regression lock quality
5. Bootstrap pass:
    - default app scaffolding, unnecessary boilerplate, flexibility tradeoffs
6. TypeScript client pass (`vormaclient/*`):
    - runtime/navigation state machine correctness, adapter API clarity,
      SSR/hydration boundaries, contract/unit test quality

## Progress Board

- Total files in scope: `386`
- Test files in scope: `145`
- Test files in `vormaclient/*`: `35`

### Package Status

- `vorma` (root package): in progress
- `vormabuild`: not started
- `vormaruntime`: not started
- `vormaclient`: in progress
- `wave`: in progress
- `wave/tooling`: in progress
- `bootstrap`: in progress

### Current Pass

- Pass: surface/correctness seed pass
- Active target: `wave/tooling`, `wave`, and `vormaclient/client` (recent
  watcher/reload and client-runtime paths first)

## Findings Log

1. Bootstrap server template drops `http.ListenAndServe` errors.
    - File: `bootstrap/tmpls/cmd_app_main_go_tmpl.txt:12`
    - Why this is a problem: generated apps can fail to bind/start and exit
      without surfacing the server error through process exit status or logs.
      This makes startup failures harder to diagnose and can produce false
      success in script automation.
2. Bootstrap example route template introduces a data race via shared mutable
   global state.
    - File: `bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt:11`
      and `bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt:26`
    - Why this is a problem: the default generated `count` state is mutated by
      request handlers without synchronization, so concurrent requests can race.
      This creates a correctness footgun in scaffolded starter apps.
3. Bootstrap package installation path spawns one package-manager process per
   dependency.
    - File: `bootstrap/bootstrap.go:376`
    - Why this is a problem: project scaffolding does avoidable repeated lock
      resolution/network work and slower install time; this is an obvious
      performance win opportunity by batching package installs.
4. Docker bootstrap path has no guard for empty `NodeMajorVersion`.
    - File: `bootstrap/bootstrap.go:79` and
      `bootstrap/tmpls/dockerfile_tmpl.txt:3`
    - Why this is a problem: `NodeMajorVersion` can flow empty into generated
      Dockerfile (`setup_.x`), producing invalid setup URL and broken default
      docker build path.
5. Wave dev config validation does not validate `Watch.HealthcheckEndpoint`
   shape.
    - File: `wave/tooling/builder_validation.go:12`
    - Related runtime use: `wave/tooling/devserver_readiness.go:38`
    - Why this is a problem: readiness URL construction assumes an endpoint path
      shape; invalid values (for example missing leading slash) can produce
      malformed probe URLs and silent wait failures.
6. Wave exposes mutable internal parsed config pointer directly.
    - File: `wave/runtime_framework.go:108`
    - Why this is a problem: `GetParsedConfig()` returns framework internal
      state by pointer with no immutability boundary, allowing external mutation
      of runtime/tooling behavior and increasing hidden-coupling risk.

## Test Gap Notes

1. No bootstrap regression test currently asserts generated `cmd/serve/main.go`
   handles `http.ListenAndServe` errors.
2. No bootstrap regression test currently protects against the unsynchronized
   mutable `count` example in generated routes.
3. No bootstrap regression test currently enforces non-empty `NodeMajorVersion`
   for docker-target scaffolds.
4. No validation regression test currently enforces healthcheck endpoint path
   shape requirements in `wave/tooling` config validation.
5. No contract test currently enforces immutability expectations for
   `wave.GetParsedConfig()` consumers.

## Open Assumptions

- Assumption A1: `internal/site` watcher temp-file event noise is app/tooling
  interaction unless proven to affect default framework behavior.
- Assumption A2: audit will prioritize framework-level fixes over app-level
  workarounds.

## Session Notes

- 2026-02-14: tracker initialized; audit kickoff.
- 2026-02-14: baseline verification runs:
    - `go test ./wave/...`
    - `go test -race ./wave/...`
    - `go test -race ./vormabuild`
    - `go test -race ./vormaruntime`
    - `go test ./wave -coverprofile=/tmp/wave_pkg.cover` (95.4%)
    - `go test ./wave/tooling -coverprofile=/tmp/wave_tooling.cover` (89.5%)
    - `go test ./vormabuild -coverprofile=/tmp/vormabuild.cover` (~91%)
    - `go test ./vormaruntime -coverprofile=/tmp/vormaruntime.cover` (95.5%)
- 2026-02-14: scope expanded explicitly to include full `vormaclient/*`
  TypeScript audit (not Go-only).
- 2026-02-14: `vormaclient/client` TypeScript tests verified:
    - `pnpm exec vitest --config vormaclient/client/vitest.dist.config.ts run`
      (6 files / 71 tests passing)
    - `pnpm exec vitest run vormaclient/client/src/tests` (29 files / 468 tests
      passing)
