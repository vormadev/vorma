WAVE TODO

Context:

- This list is rebuilt from a full `/wave/*` audit.
- Items are tracked as candidates until explicitly accepted/completed.

## 1) Runtime Browser Bootstrap Contract Hardening (active)

- Replace hardcoded refresh WebSocket host/protocol assumptions with
  origin-aware behavior (`ws` vs `wss`, non-`localhost` compatibility).
- Introduce explicit, overridable internal browser namespace/ID settings for
  refresh/filemap globals and DOM IDs to avoid collisions with application
  choices.
- Target files: `wave/refresh.go`, `wave/filemap.go`, `wave/types.go`,
  `wave/runtime_framework.go`.

Acceptance:

- Dev refresh works correctly for proxied, remote, and HTTPS development setups.
- Internal browser globals/IDs are configurable with safe defaults and no hidden
  assumptions.

## 2) Static Route Ownership Correctness

- Make static middleware ownership checks path-existence-driven (with cache),
  not prefix-only, even when `PublicPathPrefix` is not `/`.
- Ensure application routes that merely share the static prefix are not
  swallowed by static middleware.
- Target files: `wave/runtime_assets.go`, `wave/runtime_static.go`,
  `wave/wave_runtime_test.go`.

Acceptance:

- Static middleware serves real assets and forwards non-assets to downstream
  handlers across both root and prefixed public-path modes.

## 3) Dev Lock Acquisition Atomicity

- Replace lock-file read-then-write acquisition with atomic create semantics and
  stale-lock recovery that cannot allow dual acquires under races.
- Keep PID-based stale lock detection behavior.
- Target files: `wave/tooling/lock.go`, `wave/tooling/lock_unix.go`,
  `wave/tooling/lock_windows.go`, `wave/tooling/lock_test.go`.

Acceptance:

- Two concurrent `wave dev` processes cannot both acquire the project lock.
- Stale lock recovery remains deterministic.

## 4) Event Pipeline Package Decomposition

- Split `wave/tooling/events.go` into cohesive modules by concern (dedup,
  classification, planning, hook execution, build phase, browser phase,
  orchestration) without behavior change.
- Keep pure decision helpers and their contracts colocated.
- Target files: `wave/tooling/events*.go`, `wave/tooling/events_*_test.go`.

Acceptance:

- Behavior/tests remain unchanged while event pipeline structure is materially
  easier to navigate and refactor.

## 5) Port Resolution State Isolation

- Remove process-global singleton coupling in `MustGetPort` internals by
  introducing explicit resolver state that can be owned by server/runtime
  instances.
- Keep ergonomic top-level helpers while avoiding hidden global state lock-in
  for complex hosts/tests.
- Target files: `wave/env.go`, `wave/runtime_framework.go`,
  `wave/tooling/devserver.go`, `wave/env_test.go`.

Acceptance:

- Port resolution behavior remains stable while state ownership is explicit and
  reset-free global coupling is removed from core logic.

## 6) Panic Boundary Tightening For Tooling Entry Paths

- Keep intentional `Must*` APIs, but remove panic pathways from non-`Must`
  build-tooling flows (for example, TypeScript helper generation utilities and
  CLI wrappers) so callers can handle errors explicitly.
- Target files: `wave/tooling/url.go`, `wave/tooling/cli.go`,
  `wave/tooling/builder.go`, `wave/tooling/*_test.go`.

Acceptance:

- Non-`Must` tooling APIs are consistently error-returning and panic-free.

## 7) Diagnostics (deferred)

- Revisit only after functionality/structure items above are stable.
- If reintroduced, scope must remain minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events*.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
