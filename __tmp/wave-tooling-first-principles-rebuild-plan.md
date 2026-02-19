# Wave Tooling First-Principles Rebuild Plan

## Objective

Replace the current incremental split with a single intentional architecture for
`wave/tooling` that satisfies all package constraints:

- Each package has exactly one non-test `.go` source file.
- Each such file is between 200 and 2000 lines.
- If a package exceeds 2000 lines, split by domain into subpackages.
- If a package falls below 200 lines, merge it into an owning package or shared
  package.
- No compatibility wrappers, no transitional adapter layers.

## Current Diagnosis

- `wave/tooling/devserver/devserver.go` is still 4857 lines and contains many
  domains.
- `wave/tooling/tooling.go` and `wave/tooling/cli/cli.go` are thin/underweight.
- `wave/tooling/toolingshared` still has multiple source files and mixed
  concerns.
- Empty directories (`wave/tooling/devserver/eventpipeline`,
  `wave/tooling/devserver/hooks`) suggest unfinished incremental extraction.

## First-Principles Boundary Rules

- Orchestration is separate from domain engines.
- Domain engines expose explicit inputs/outputs; orchestration wires them.
- Event pipeline logic is not owned by `Server` methods.
- Hook logic is not interleaved with watcher classification/build decisions.
- Runtime process control (app/vite lifecycle + readiness waits) is isolated.
- Shared helpers are only for truly cross-domain primitives.

## Recommended Strategy

Use `wave/tooling_2` as a greenfield target, then hard-swap:

1. Build the entire desired architecture in `wave/tooling_2`.
2. Reach compile + tests green for `tooling_2`.
3. In one cutover commit, replace `wave/tooling` with `wave/tooling_2`.
4. Delete old implementation immediately. No dual-path period.

Reason: this avoids the half-old/half-new state that caused the current
vestigial boundaries.

## Target Package Topology

The following is the desired end state after cutover.

1. `wave/tooling` (public entry + CLI intent + top-level wiring)
    - Owns `RunDev`, `RunBuild`, CLI parse/dispatch, and dependency wiring.
    - Target size: 250-600 lines.

2. `wave/tooling/builder`
    - Owns `Builder` orchestration and build-phase APIs.
    - Target size: 700-1200 lines.

3. `wave/tooling/builder/css`
    - Owns CSS build processing and output lookup rules.
    - Target size: 400-900 lines.

4. `wave/tooling/builder/static`
    - Owns static asset graph/hash/cache/filemap generation.
    - Target size: 1000-1900 lines.

5. `wave/tooling/builder/schema`
    - Owns config/schema generation and validation primitives.
    - Target size: 300-700 lines.

6. `wave/tooling/watch`
    - Owns fsnotify watcher setup, directory watch plan, ignore/match
      evaluation.
    - Target size: 600-1300 lines.

7. `wave/tooling/watch/classification`
    - Owns pre-classification decisions and filesystem probe interpretation.
    - Target size: 250-700 lines.

8. `wave/tooling/watch/dedup`
    - Owns canonical-path dedup and alias resolution.
    - Target size: 250-700 lines.

9. `wave/tooling/devserver`
    - Owns run-loop orchestration only.
    - Owns state transitions and engine coordination, but not the internals of
      event/hook processing.
    - Target size: 700-1500 lines.

10. `wave/tooling/devserver/eventpipeline`
    - Owns event-to-workset planning and decision reduction.
    - Owns build/restart/browser phase decision derivation.
    - Target size: 900-1800 lines.

11. `wave/tooling/devserver/hooks`
    - Owns hook planning, timeout policy resolution, execution, and aggregation.
    - Target size: 900-1800 lines.

12. `wave/tooling/devserver/restartengine`
    - Owns restart intent queueing/coalescing and run-cycle scope lifecycle.
    - Target size: 300-900 lines.

13. `wave/tooling/devserver/runtimeprocess`
    - Owns app/vite process lifecycle + readiness waiting behavior.
    - Target size: 250-800 lines.

14. `wave/tooling/broadcast`
    - Owns websocket refresh manager and payload fanout.
    - Target size: 250-800 lines.

15. `wave/tooling/toolingshared`
    - Owns lock lifecycle and cross-package primitives such as pattern
      validation.
    - Must be a single source file; redesign lock semantics to avoid OS-split
      files.
    - Target size: 250-700 lines.

## Ownership Moves From Current `devserver.go`

Move directly by domain ownership:

1. Run lifecycle + state transition loop -> `wave/tooling/devserver`
2. Event classification/workset/phase decisions ->
   `wave/tooling/devserver/eventpipeline`
3. Hook planning/execution/timeout policy -> `wave/tooling/devserver/hooks`
4. Restart request queue/coalescing/run-cycle context ->
   `wave/tooling/devserver/restartengine`
5. App/vite start-stop/wait readiness -> `wave/tooling/devserver/runtimeprocess`
6. Keep only orchestration wiring in `wave/tooling/devserver`

## Non-Negotiable Execution Rules For Next Pass

1. No incremental “just split a little more” pass.
2. No temporary wrappers/adapters for old APIs.
3. No duplicate ownership (same responsibility in two packages).
4. No stopping at intermediate architecture checkpoints.
5. No package under 200 lines at cutover.
6. No package above 2000 lines at cutover.

## Concrete Cutover Sequence

1. Create the full `wave/tooling_2` tree with all target packages.
2. Copy code into owning package files by domain, not by mechanical chunk size.
3. Rename types/functions as needed to reflect package ownership cleanly.
4. Rewire imports until `go test ./wave/tooling_2/...` is green.
5. Validate line counts and one-file-per-package constraints with a script.
6. Replace old tree in one commit:
    - Remove `wave/tooling`
    - Move `wave/tooling_2` to `wave/tooling`
    - Update import paths in one sweep
7. Run full verification:
    - `go test ./wave/tooling/...`
    - targeted integration tests already covering current behavior
8. If any package is out of bounds (<200 or >2000), fix immediately before
   merging.

## Validation Checklist

- [ ] Every `wave/tooling/**` package has exactly one non-test `.go` source
      file.
- [ ] Every such file is 200-2000 lines.
- [ ] No placeholder/empty directories remain.
- [ ] No compatibility wrappers remain.
- [ ] `wave/tooling/devserver` is orchestration-only.
- [ ] Event pipeline and hooks each live in dedicated packages.
- [ ] Cross-platform lock behavior is implemented in one package file.
- [ ] All existing tests pass after the one-shot cutover.

## Fallback

If `tooling_2` proves slower than expected after initial extraction, fallback is
still a hard-cutover in place:

1. Create all target packages directly under `wave/tooling`.
2. Move all ownership in one uninterrupted pass.
3. Reach compile/test green only after full ownership migration is complete.

No compatibility layer fallback is allowed.

## Progress Snapshot (2026-02-18)

- `wave/tooling_2` is now the active rebuild target and compiles cleanly.
- `wave/tooling_2/tooling.go` now includes CLI dispatch directly (removed tiny
  `cli` package).
- `wave/tooling_2/toolingshared` lock handling is consolidated to one source
  file.
- New domain packages exist and are wired:
  - `wave/tooling_2/devserver/eventpipeline`
  - `wave/tooling_2/devserver/executionengine`
  - `wave/tooling_2/devserver/hooks`
  - `wave/tooling_2/devserver/restartengine`
  - `wave/tooling_2/devserver/runtimeprocess`
- Large duplicate forwarding layers in `devserver.go` have been removed.
- Hook-stage runtime, watcher batch intake, and deterministic pipeline
  execution orchestration now live in `devserver/executionengine`.

Current package-constraint status in `wave/tooling_2`:

- All packages currently satisfy one-file-per-package and 200-2000 line
  constraints.

Next aggressive move to finish:

1. Keep pressure on boundary polish: remove any remaining orchestration leakage
   from `devserver` into domain engines as discovered.
2. Continue replacing residual vestigial wrappers with direct ownership calls.
3. When architecture stabilizes, perform one-shot cutover from `tooling_2` to
   `tooling`.
