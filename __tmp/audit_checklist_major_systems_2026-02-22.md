# Open Items

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Active TODOs

- [ ] Raise test coverage for requested scope to 100%.
- [ ] Eliminate remaining coverage gaps.
- [ ] Bring non-exempt noncompliant packages into one-file/<2000 compliance:
      `internal/vormaruntime` (1 non-test file, 2981 lines total).
- [ ] `internal/vormaruntime` split points for subpackages: `routepipeline`
      (route planning/cache/core payload logic), `rendering` (loaders HTML + SSR
      render helpers), `runtimecore` (Vorma state/config/init orchestration).
- [ ] Bring non-exempt noncompliant packages into one-file/<2000 compliance:
      `vormabuild` (1 non-test file, 9862 lines total).
- [ ] `vormabuild` split points for subpackages: `routeparse` (route
      discovery/parsing/registration generation), `buildruntime` (build
      lifecycle + watch + runtime state commit/snapshot), `output`
      (atomic/artifact/fs/hash/vite manifest staging), `tsgenruntime` (TS
      generation + metadata + vite config synthesis).
- [ ] Bring non-exempt noncompliant packages into one-file/<2000 compliance:
      `wave/tooling/devserver` (1 non-test file, 2408 lines total).
- [ ] `wave/tooling/devserver` split points for subpackages: `refreshruntime`
      (refresh server lifecycle), `reloadwait` (readiness
      generation/cancel/broadcast sequencing), `viteinvalidate` (invalidate
      generation + fallback orchestration).
