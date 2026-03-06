# Wave2 Architecture (Phased, Task-Native)

Wave2 is a clean-slate rewrite focused on the smallest architecture that still
covers all required observable behavior.

## Public Surfaces

- `wave2/wave.go`: app-facing runtime API
- `wave2/wavebuild/wavebuild.go`: build/dev phase contracts and phase pipeline
- `wave2/wavefw/wavefw.go`: framework-facing signal translation

## Runtime/Buildtime Boundary

This split is strict:

- `wave2` is runtime-facing and must stay free of build/dev-heavy dependencies.
- `wave2/wavebuild` is build/dev orchestration.
- `wave2` must never import `wave2/wavebuild`.

This keeps production binaries lean and prevents file-watching/build toolchain
weight from leaking into runtime application servers.

## Canonical Pipeline

Wave2 orchestration is phase-based:

1. `events`
2. `build`
3. `backend_settling`
4. `frontend_settling`

Mode policy:

- `dev` executes phases 1-4.
- `prod` bypasses phases 1/3/4 and executes phase 2 only.

Each phase:

- plans terminal goals for that phase
- runs terminal roots in `kit/tasks`
- lets `kit/tasks` own prerequisite ordering, dedupe, and parallelism
- frontend settling uses one terminal browser action with explicit precedence

## Task Context Contract

One batch-scoped `tasks.Ctx` is shared across all executed phases in a batch.

That gives:

- one identity domain for all task keys in the batch
- one cache domain for task outputs reused by downstream callers
- no second DAG scheduler outside `kit/tasks`

## Execution Port Contract

Terminal side effects are emitted through one explicit execution port:

- `PhaseEffectExecutor`
- keyed by explicit `PhaseEffectID`
- invoked only by terminal phase tasks

This keeps phase planning pure while allowing side-effect implementation to be
swapped without changing the task graph.

For design runs:

- Provide a no-op execution scope directly on batch input:
  `Execution: wavebuild.PhaseExecutionScope{EffectExecutor: wavebuild.NoopPhaseEffectExecutor{}}`.

## Framework Boundary

Wave2 emits framework-agnostic signals.

`wavefw` translates those signals into framework-owned actions. Wave2 does not
encode framework endpoint names, conventions, or transport details.
