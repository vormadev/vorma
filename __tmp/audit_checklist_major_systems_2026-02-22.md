# Audit Checklist (Created 2026-02-22)

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Code Issues

- [x] Fix stale HTTP middleware chain cache in `kit/mux/mux.go` (compiled chain
      was not invalidated after late middleware registration).
- [x] Fix transient nested-router inconsistency in `kit/mux/nested_mux.go`
      (route registration and compiled-route index update were split across lock
      windows).
- [x] Fix nested-router matcher read/write race in `kit/mux/nested_mux.go`
      (`FindNestedMatches` released lock before matcher access).
- [x] Fix `ViteContext` synchronization in `wave/tooling/devserver/devserver.go`
      (mixed locked writes and unlocked reads).
- [x] Fix watcher-start channel synchronization in
      `wave/tooling/devserver/devserver.go` (removed shared mutable
      `WatcherStartCh`; now uses per-cycle local channels).
- [x] Fix check-then-register race in `internal/vormaruntime/vorma_init.go`
      (`validateAndDecorateNestedRouter` now uses atomic idempotent
      registration).
- [x] Fix nil-context watcher panic path in
      `wave/tooling/devserver/internal/runloop/runloop.go`
      (`RunWatcherWithContext` now normalizes nil to `context.Background()`).
- [x] Fix semantic duplicate default watch-pattern injection in
      `vormabuild/build_watch.go` (route definition watch patterns now dedupe by
      normalized absolute path, not just raw string equality).
- [x] Fix mixed snapshot risk in `kit/mux/nested_mux.go` (compiled routes and
      pattern index map now load/store atomically as one snapshot).
- [x] Fix discovered registration call-site collapse in
      `vormabuild/backend_route_registration_generation.go` (discovered call ID
      now includes source position, preventing distinct same-expression call
      sites from deduping together).

## Coverage / Verification

- [ ] Raise test coverage for requested scope to 100%.
- [x] Re-run `go test` and `go test -race` on modified packages after fixes.
- [x] Re-run full repository unit test suite (`go test ./...`) after changes.
- [x] Recompute current scope coverage baseline.
- [ ] Eliminate remaining coverage gaps.

Current coverage snapshot
(`go test ./wave/... ./internal/vormaruntime ./kit/mux ./vormabuild ./vormagogen -cover`):

- `wave`: 94.9%
- `wave/internal/wavecore`: 40.0%
- `wave/internal/waveruntime`: 3.1%
- `wave/tooling/builder`: 76.0%
- `wave/tooling/builder/internal/css`: 68.2%
- `wave/tooling/builder/internal/fileops`: 63.6%
- `wave/tooling/builder/internal/schema`: 85.7%
- `wave/tooling/builder/internal/static`: 77.8%
- `wave/tooling/devserver`: 80.2%
- `wave/tooling/devserver/internal/eventpipeline`: 90.3%
- `wave/tooling/devserver/internal/hooks`: 91.1%
- `wave/tooling/devserver/internal/restartengine`: 64.7%
- `wave/tooling/devserver/internal/runloop`: 85.8%
- `wave/tooling/devserver/internal/runtimeprocess`: 77.1%
- `wave/tooling/internal/broadcast`: 66.5%
- `wave/tooling/internal/shared`: 67.0%
- `wave/tooling/internal/watch`: 70.0%
- `wave/tooling/internal/watch/classification`: 75.4%
- `wave/tooling/internal/watch/dedup`: 76.9%
- `internal/vormaruntime`: 91.9%
- `kit/mux`: 89.2%
- `vormabuild`: 90.9%
- `vormagogen`: 100.0%

## AGENTS.md Compliance Items

- [x] Add explicit `vormagogen` exception text to `AGENTS.md`.
- [x] Clarify alias rule scope in `AGENTS.md`: it applies to internal repository
      package names; external import aliasing is allowed when it improves
      clarity or avoids collisions.
- [x] Revert external import naming regression in
      `vormabuild/route_parsing_pipeline.go` (`esbuild` alias restored; removed
      generic `api` naming).
- [x] Validate scoped package compliance matrix for one-file/200-2000 rule.
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `internal/vormaruntime` (18 non-test files, 3044 lines total).
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `kit/mux` (2 non-test files, 1741 lines total).
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `vormabuild` (35 non-test files, 10424 lines total).

## Decision

- [x] `vormagogen` is approved as an exception to the 200-line minimum.
