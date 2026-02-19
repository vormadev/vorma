# Wave Refactor Plan (Type-Ownership First, No Half-Migration)

## Goals

- Split `wave` and `wave/tooling` by type ownership and runtime responsibility.
- Keep all existing test files at current paths.
- Refactor in a way that avoids a half-old/half-new architecture.
- Complete compile + tests before any optional file-count consolidation.

## Core Design (First Principles)

## `wave` side

- `wave/config`
    - Owns `ParsedConfig` and all parse/clone/config-state behavior.
    - Any method with receiver `*ParsedConfig` belongs here.

- `wave/internal/shared`
    - Owns shared Wave internals: port env/resolver, URL/path normalization,
      cache primitives, low-level fs/path helpers.
    - Includes logic currently in `internal/waveport`, `internal/waveurl`, and
      `wave/internal/pathnorm`.

- `wave/runtime`
    - Owns runtime engine internals: fs/static/assets/filemap/css/refresh
      behavior.
    - Holds mutable runtime/cache behavior currently spread across `wave` files.

- `wave`
    - Thin facade only.
    - Owns public constructor and ergonomic API surface; delegates to `config`
      and `runtime`.

## `wave/tooling` side

- `wave/tooling/builder`
    - Owns `Builder` and all `func (b *Builder)` methods.
    - Includes schema validation, timeout policy, static/css/filemap
      integration.

- `wave/tooling/watch`
    - Owns `watcher` and all watch matching/plan/preclass/dedup behavior.

- `wave/tooling/events`
    - Owns event pipeline logic.
    - Introduce package-owned pipeline type(s) so event logic is not methods on
      devserver `server`.

- `wave/tooling/broadcast`
    - Owns websocket client manager and browser payload fanout logic.

- `wave/tooling/devserver`
    - Owns orchestration only: process lifecycle, restart state machine,
      coordination across builder/watch/events/broadcast.
    - Depends on package interfaces, not deep internals.

- `wave/tooling/cli`
    - Owns CLI parsing and top-level dispatch only.

- `wave/tooling/internal/shared`
    - Owns tooling-only shared helpers/types (lock and similar internals).

## Boundaries and Dependency Direction

- `wave`: `wave` facade -> `wave/runtime`, `wave/config`,
  `wave/internal/shared`.
- `tooling`: `cli` -> `devserver` -> (`builder`, `watch`, `events`, `broadcast`)
  -> tooling shared + wave packages.
- `events` and `broadcast` must not depend on `devserver` concrete types.
- `builder` owns builder receivers; `watch` owns watcher receivers.

## Files to Move by Ownership

### Direct mechanical moves

- `internal/waveport/port.go` -> `wave/internal/shared`
- `internal/waveurl/public_url.go` -> `wave/internal/shared`
- `wave/internal/pathnorm/pathnorm.go` -> `wave/internal/shared`
- `wave/tooling/internal/watchereventclassification/*` -> `wave/tooling/watch`
- `wave/tooling/internal/watchereventdedup/*` -> `wave/tooling/watch`
- `wave/tooling/lock*.go` -> `wave/tooling/internal/shared`

### Ownership refactors before full move

- `func (w *Wave)` logic: move implementation into `wave/runtime` and keep thin
  facade in `wave`.
- `func (b *Builder)` logic: consolidate under `wave/tooling/builder`.
- `func (s *server)` event/broadcast logic: extract into `events`/`broadcast`
  package-owned types and functions; devserver orchestrates.
- `func (w *watcher)` logic: consolidate under `wave/tooling/watch`.

## Test Policy (Required)

- Do not move any existing `*_test.go` files.
- Update test imports/calls to new package ownership.
- Preserve all current harness logic and assertions.
- No deleting, bypassing, or weakening tests to make migration easier.

## Anti-Purgatory Execution Protocol (Hard Cutover)

1. Freeze target ownership map (this document) and do not mix alternative split
   strategies mid-flight.
2. Perform one hard cutover move:
    1. move files/packages to target ownership boundaries
    2. rewrite package names/imports/callsites in one sweep
    3. allow the tree to be broken during this phase
3. Do not stop for per-domain green states. Keep pushing until the ownership
   cutover is fully landed.
4. Start stabilization only after the full cutover is in place:
    1. fix compiler errors to zero
    2. run full test suite and fix to green
5. After full green, run a dedicated consolidation pass to reach the 3-file
   package rule, then run full tests again.

## Definition of Done

- No `internal/waveport` or `internal/waveurl` remaining.
- New ownership structure in place for `wave` and `wave/tooling`.
- All existing tests still at original paths and passing.
- No mixed architecture where old and new ownership models both remain active.
- Optional final pass: package file-count consolidation with tests green again.
