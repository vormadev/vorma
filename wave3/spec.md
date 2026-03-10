# `wave3` System Spec (From Zero)

## What `wave` Is

`wave` is a generic build system for Go applications. Framework authors and
normal application developers can both use it, and it supports both
`server-only` and `full-stack` application shapes.

In this document, `wave` refers to the build system that uses heavy dependencies
like `esbuild` (for CSS processing), `fsnotify` (for file system watching), and
`doublestar` (for traditional "globstar"-style glob support). Insofar as this
document in concerned, `wave` does NOT refer to the lightweight
application-facing system used for accessing canonical build outputs at runtime.

In `dev_mode`, `wave` runs as a long-lived process that watches files, decides
what work is required, and converges backend and frontend runtime state after
each change batch. In `prod_mode`, `wave` runs a single build cycle and exits.

This document defines the `wave3` orchestration model in enough detail that a
new engineer could implement it from scratch.

## Why `wave3` Exists

`wave1` proved the required behavior surface but accumulated orchestration
complexity across watcher handling, hook timing, build steps, lifecycle control,
and framework integration. `wave3` keeps the same required outcomes while making
the control model explicit: one process-level controller, one per-cycle task
graph, one config-aware event reducer, and one shared checkpoint model for all
external lifecycle work.

## Definitions

`external_invoker`: The external caller that starts `wave` from a terminal or
CI.

`ultimate_wave_process`: The OS process started by the `external_invoker`.

`dev_mode`: Long-running mode where `wave` continuously watches for file changes
and runs incremental builds. A full build is always performed on startup.

`prod_mode`: One-shot mode where `wave` runs a single build cycle and exits,
skipping any `dev_mode`-only work.

`wave_supervisor`: The top-level controller in the `ultimate_wave_process` that
owns `fs_watcher` lifecycle, config/plugin loading, `raw_fs_event` event
batching and classification, `workset` calculation, cycle launch/cancel policy,
`build_failure_idle_state` policy, and process-lifetime supervisor state.

`fs_watcher`: The filesystem event listener owned by the `wave_supervisor`,
built using `fsnotify`.

`raw_fs_event`: One low-level filesystem event from the `fs_watcher` for one
path.

`raw_fs_event_batch`: A short-window grouped (e.g., 20ms debounce time) slice of
`raw_fs_event` values.

`config_file`: The user's authored JSON config source file on disk.

`parsed_config`: Normalized, resolved, validated representation of the "Core"
and "Vite" portions of a `config_file`. The "Watch" portion of the user's config
is converted into `lifecycle_task_registry` entries.

`lifecycle_task_registry`: The runtime state containing all then-current
`lifecycle_task` definitions, including both user-defined cmd hooks and
framework-defined callbacks (all of which the `wave_supervisor` will convert
into `task` nodes and insert into the `lifecycle_task_registry`). Does not
include standard `work_orchestrator`-owned tasks.

`workset`: The reduced union of required work for one `raw_fs_event_batch`,
derived by `evtparse` from event data plus current config-derived lifecycle
policy.

`short_circuit_decision`: A `wave_supervisor` decision to skip launching a
`work_cycle` for a batch and instead apply an immediate control-flow branch (for
example config restart or no-op).

`pre_cycle_work`: Pre-requisite work the `wave_supervisor` must complete before
launching a `work_cycle`. Considered complete when the `wave_supervisor` has:
(1) parsed the `config_file` into a `parsed_config`, (2) run the config-relevant
portions of the `framework_plugin`, (3) ensured the `fs_watcher` is up-to-date,
(4) ensured the `lifecycle_task_registry` is up-to-date, and (5) classified
events into a `workset`.

`framework_plugin`: Framework-owned extension contract consumed by `wave`.

`lifecycle_task`: An externally defined piece of work (supplied either by the
user or `framework_plugin`) to be scheduled and run by the `work_orchestrator`.

`work_cycle_input`: Input assembled by the `wave_supervisor` to feed into a
single specific `work_cycle`. This input is read-only by contract for the
duration of that cycle.

`work_cycle`: One complete orchestration run from one `work_cycle_input` to
completion. For `dev_mode`, completion means the point at which frontend effect
signals have been dispatched. For `prod_mode`, completion means all production
build artifacts have been created and ready to be consumed by the application
runtime.

`work_orchestrator`: `kit/tasks` DAG executor launched by the `wave_supervisor`
to perform build work once `pre_cycle_work` is true.

`task`: One `kit/tasks` node.

`tasks_ctx`: One `tasks.Ctx` instance scoped to one `work_cycle`.

`checkpoint`: Named `work_cycle`-time boundary used to schedule external
lifecycle tasks. `checkpoints` include (exclusive): `cycle_start`,
`wave_build_start`, `wave_build_go_compile_start`, `wave_build_end`, and
`cycle_end`.

`wave_build`: Standard `wave` artifact/static/css handling and `go_compile`
path. In `dev_mode`, this work is done incrementally/marginally based on what is
needed to keep observable state fresh. In `prod_mode`, all work is performed,
except that `go_compile` can be skipped if the user desires to build their own
binary.

`wave_build_go_compile`: The Go binary compilation step inside `wave_build`;
when requested during `dev_mode`, it implies both `app_restart` and
`frontend_hard_reload` semantics for that cycle path.

`app_restart`: The act of stopping the running application and starting it again
using the then-current Go binary.

`build_failure_idle_state`: A `wave_supervisor`-owned state entered when any
`task` in a `work_cycle` fails. In this state, the `ultimate_wave_process` does
not panic and remains alive while waiting for the next `raw_fs_event_batch`.
When the next batch arrives, `wave_supervisor` exits `build_failure_idle_state`
and runs a full from-scratch `wave_build` cycle. If that cycle fails, the
`wave_supervisor` re-enters `build_failure_idle_state`.

## The Event Reduction Boundary (`evtparse`)

`evtparse` is the config-aware reducer that converts a raw event batch into one
canonical workset. Its input is raw fsnotify events plus the current
`parsed_config` and caller context flags. Its output is the union work required
for that batch, split into watcher-facing work, builder/orchestrator-facing
work, and an explicit short-circuit decision with reason.

After calling `evtparse`, downstream orchestration should not re-analyze raw
events. If a downstream consumer needs additional facts, `evtparse` should be
extended to provide those facts directly.

## Checkpoint Model for External Lifecycle Work

`wave3` exposes one shared checkpoint model for framework tasks and user tasks.
A checkpoint is a named moment in cycle time, and tasks declare `StartAt` and
`FinishBy` boundaries against that checkpoint set.

The public checkpoint set is:

1. `cycle_start`
2. `wave_build_start`
3. `wave_build_go_compile_start`
4. `wave_build_end`
5. `cycle_end`

`StartAt` means the task may start at that checkpoint. `FinishBy` means the task
must be complete by that checkpoint. Checkpoints are scheduling boundaries, not
triggers. Triggers still come from the `wave_supervisor` event loop.

For current Vorma requirements, this maps cleanly: config-derived setup at
`cycle_start`; pre-wave dependencies by `wave_build_start`; route/template
settling by `wave_build_go_compile_start`; post-wave dependent work by
`wave_build_end`; and final runtime/frontend signaling by `cycle_end`.

## End-to-End Flow

1. The `external_invoker` starts `wave` in `dev_mode` or `prod_mode`.
2. The `wave_supervisor` initializes long-lived resources and control state.
3. The `wave_supervisor` loads and parses the user `config_file`.
4. The `wave_supervisor` applies plugin augmentation and updates
   `parsed_config`-derived policy plus `lifecycle_task_registry`.
5. In `dev_mode`, the `fs_watcher` emits raw events and the `wave_supervisor`
   batches them. In `prod_mode`, the `wave_supervisor` creates one one-shot
   trigger.
6. The `wave_supervisor` calls `evtparse` to reduce trigger facts into one
   canonical workset.
7. The `wave_supervisor` applies short-circuit policy from that workset (for
   example config-restart or no-op branches).
8. The `wave_supervisor` assembles one read-only-by-contract `work_cycle_input`.
9. The `wave_supervisor` launches one `work_cycle` in the `work_orchestrator`.
10. The `work_orchestrator` creates one `tasks_ctx` and executes the cycle DAG.
11. `wave`-owned tasks execute with dependency correctness and maximum safe
    parallelism.
12. External lifecycle tasks execute at checkpoint boundaries according to
    `StartAt` and `FinishBy`.
13. The `work_orchestrator` returns cycle completion facts.
14. The `wave_supervisor` updates its process-lifetime supervisor state for the
    next run.
15. If any `task` fails in a `work_cycle`, the `wave_supervisor` enters
    `build_failure_idle_state` instead of panicking the `ultimate_wave_process`.
16. While in `build_failure_idle_state`, the `wave_supervisor` waits for the
    next `raw_fs_event_batch`.
17. On that next batch, the `wave_supervisor` exits `build_failure_idle_state`
    and runs a full from-scratch `wave_build` cycle.
18. In `dev_mode`, newer intent preempts stale in-flight cycles.
19. In `prod_mode`, the process exits after cycle completion.

## Build-From-Scratch Implementation Order

1. Implement config loading and parsing to `parsed_config`, then implement the
   plugin contract and `lifecycle_task_registry` update path.
2. Implement watcher lifecycle, recursive watch maintenance, event ingestion,
   and debounced batching.
3. Implement `evtparse` so raw events plus config become one deterministic
   canonical workset with explicit short-circuit reasons.
4. Implement the `wave_supervisor` cycle engine: short-circuit branching,
   read-only-by-contract cycle input assembly, cycle launch/cancel,
   `build_failure_idle_state` admission behavior, and process-lifetime
   supervisor state handoff.
5. Implement the `work_orchestrator` DAG on top of `kit/tasks` with one
   `tasks_ctx` per cycle and explicit checkpoint scheduling for external work.
6. Implement side-effect adapters for compile/artifact writes, app/vite
   lifecycle mutations, readiness waits, and frontend settle actions.
7. Implement black-box tests that prove sequencing, timing, cancellation, and
   short-circuit behavior from observable outputs.

## Non-Negotiable Invariants

Each `work_cycle` has one read-only-by-contract input and one `tasks_ctx`.
`kit/tasks` dedupe and cache behavior is cycle-local only. Process-level policy
stays in the `wave_supervisor`, not in the `work_orchestrator`. Raw-event
semantics are reduced once in `evtparse`, not re-derived downstream. Framework
behavior is plugin-owned, not hard-coded into Wave core. Any `work_cycle` task
failure must transition to `build_failure_idle_state`, not panic the
`ultimate_wave_process`. Only the `wave_supervisor` may mutate process-lifetime
supervisor state.

## Observable Correctness Criteria

Noise-only batches produce no user-visible actions. Actionable changes are never
dropped. Config-change behavior is explicit and deterministic. Dist output does
not self-trigger watcher loops. Batch reduction uses conservative union-of-work
semantics across all matching events and patterns. Stale cycles cannot override
newer intent. Frontend settle actions only happen after required backend and
`wave_build` prerequisites are complete.

## Watch Config Migration (`old_watch_shape` -> `new_watch_shape`)

The old model centered on `Watch.Include[]` entries with many boolean flags and
`OnChangeHooks[]` timing values. The new model centers on `BuildHook` entries
with explicit `StartAt`/`FinishBy` checkpoint windows plus one `RequiredWork`
outcome.

### `old_watch_shape`

The old shape looked like this at a high level:

```go
type RawIncludeEntry struct {
	Pattern                            string // glob
	OnChangeHooks                      []RawOnChangeHook
	RecompileGoBinary                  bool
	RestartApp                         bool
	OnlyRunClientDefinedRevalidateFunc bool
	RunOnChangeOnly                    bool
	SkipRebuildingNotification         bool
	TreatAsNonGo                       bool
}

type RawOnChangeHook struct {
	Cmd     string
	Timing  string // pre, concurrent, concurrent-no-wait, post
	Exclude []string
}
```

### `new_watch_shape`

The new shape is:

```go
type BuildHook struct {
	Pattern  strict.CWDRelPath
	Cmd      strict.Cmd
	CmdDir   strict.CWDRelPath
	Callback func(*BuildCtx) error
	StartAt  Checkpoint
	FinishBy Checkpoint

	RequiredWork                     RequiredWork
	IncludeFrontendRebuildingOverlay bool
}
```

### `field_translation_rules`

- `OnChangeHooks[].Timing=pre` translates to `StartAt=cycle_start`,
  `FinishBy=wave_build_start`.
- `OnChangeHooks[].Timing=concurrent` translates to `StartAt=wave_build_start`,
  `FinishBy=wave_build_end`.
- `OnChangeHooks[].Timing=post` translates to `StartAt=wave_build_end`,
  `FinishBy=cycle_end`.
- `OnChangeHooks[].Timing=concurrent-no-wait` has no direct equivalent in the
  new contract. The closest safe mapping is `StartAt=wave_build_start`,
  `FinishBy=cycle_end`, or an internally detached callback if truly required.
- `RecompileGoBinary=true` translates to `RequiredWorkGoCompile`.
- `RestartApp=true` translates to `RequiredWorkAppRestart` when
  `RecompileGoBinary` is false.
- `OnlyRunClientDefinedRevalidateFunc=true` translates to
  `RequiredWorkFrontendSoftRevalidate`.
- `SkipRebuildingNotification=true` translates to
  `IncludeFrontendRebuildingOverlay=false`.
- `SkipRebuildingNotification=false` translates to
  `IncludeFrontendRebuildingOverlay=true`.
- `TreatAsNonGo` does not live on `BuildHook` in the new model. It moves to a
  global Go-source pattern policy (`AppGoSrcPattern`) used during event
  classification.
- `RunOnChangeOnly` is no longer a direct per-entry boolean. In the new model,
  behavior is represented by selected `RequiredWork` plus whether any other
  matched work forces `wave_build`.

### `translation_examples`

Example 1: pre-build command hook, no compile, no restart.

```go
// old
Watch.Include = []RawIncludeEntry{
	{
		Pattern: "content/**/*.md",
		OnChangeHooks: []RawOnChangeHook{
			{Cmd: "pnpm run lint:md", Timing: "pre"},
		},
		RunOnChangeOnly: true,
	},
}

// new
BuildHooks = []BuildHook{
	{
		Pattern:  "content/**/*.md",
		Cmd:      "pnpm run lint:md",
		StartAt:  CheckpointCycleStart,
		FinishBy: CheckpointWaveBuildStart,
	},
}
```

Example 2: `.go` changes requiring compile and restart.

```go
// old
Watch.Include = []RawIncludeEntry{
	{
		Pattern:           "backend/**/*.go",
		RecompileGoBinary: true,
	},
}

// new
BuildHooks = []BuildHook{
	{
		Pattern:      "backend/**/*.go",
		RequiredWork: RequiredWorkGoCompile,
	},
}
```

Example 3: app restart without compile, with post-cycle callback.

```go
// old
Watch.Include = []RawIncludeEntry{
	{
		Pattern:    "config/runtime/**/*.json",
		RestartApp: true,
		OnChangeHooks: []RawOnChangeHook{
			{Cmd: "echo runtime config changed", Timing: "post"},
		},
	},
}

// new
BuildHooks = []BuildHook{
	{
		Pattern:      "config/runtime/**/*.json",
		RequiredWork: RequiredWorkAppRestart,
	},
	{
		Pattern:  "config/runtime/**/*.json",
		Cmd:      "echo runtime config changed",
		StartAt:  CheckpointWaveBuildEnd,
		FinishBy: CheckpointCycleEnd,
	},
}
```

Example 4: soft revalidate with no rebuilding overlay.

```go
// old
Watch.Include = []RawIncludeEntry{
	{
		Pattern:                            "content/**/*.md",
		OnlyRunClientDefinedRevalidateFunc: true,
		SkipRebuildingNotification:         true,
	},
}

// new
BuildHooks = []BuildHook{
	{
		Pattern:                           "content/**/*.md",
		RequiredWork:                      RequiredWorkFrontendSoftRevalidate,
		IncludeFrontendRebuildingOverlay:  false,
	},
}
```
