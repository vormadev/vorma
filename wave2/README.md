# Wave2 Architecture (Goal-First, Task-Native)

This directory is a clean-slate architecture spike for a simpler and more
deterministic Wave model.

The core idea is:

- plan in terms of **what must be true** (effects), not imperative steps
- execute with `kit/tasks` so dependency ordering and maximum safe parallelism
  come from the task graph

## Public Surfaces

- `wave2/wave.go`: app-facing API
- `wave2/wavefw/wavefw.go`: framework adapter registration API
- `wave2/wavebuild/wavebuild.go`: planner + execution engine

Wave2 must remain framework-agnostic. Framework-specific semantics belong in
adapters (`wavefw` registrations), not in `wavebuild` defaults.

## Runtime/Buildtime Boundary (Non-Negotiable)

This architecture must preserve a strict runtime/buildtime split so production
apps do not pull dev/build dependencies.

- `wave2` is runtime-facing and must stay free of build/dev-heavy dependencies.
- `wave2/wavebuild` is build/dev orchestration and may depend on build/dev-only
  toolchains.
- Runtime-facing packages must never import build/dev packages.
- Required direction: runtime-safe core -> consumed by runtime and buildtime.
- Forbidden direction: `wave2` importing `wave2/wavebuild`.

## Model

1. **Events**: normalized change classes (`go_source_changed`,
   `public_static_asset_changed`, etc.).
2. **Effects**: observable outcomes to ensure (`recompile`, `restart`,
   `css_hot_reload`, `hard_reload`, etc.).
3. **Mode applicability**: each effect is dev/prod/both by planning policy.
4. **Planner reduction**:
    - union event->effect sets across the whole batch
    - dedupe
    - filter by mode
    - apply precedence/exclusivity rules (for example browser action winner)
    - output terminal goals

In prod, this naturally becomes: union all relevant prod-capable effects for the
batch, dedupe, run once each.

## Goal-First Execution with `kit/tasks`

Execution is intentionally last-to-first:

- planner chooses **terminal goals** (final effects for this batch)
- executor runs terminal goal tasks
- each goal task calls its own prerequisite tasks
- `kit/tasks` guarantees:
    - a prereq runs before dependents that call it
    - shared prereqs run once per execution context
    - shared prerequisite outputs are reused by later callers in the same
      execution context
    - independent branches run in parallel

This keeps orchestration small and avoids hand-written topo schedulers for most
cases.

## Effect Contract

Every effect task root should follow this contract:

- **Ensure semantics**: "ensure X is true" instead of "do X step now"
- **idempotent** under one execution context
- **explicit dependencies** by calling prerequisite tasks
- **no hidden ordering assumptions** outside the task graph
- **observable outcomes only** (internal mechanism can change)

## What Stays in Planner vs Task Graph

Planner responsibilities:

- map event classes to candidate effects
- mode filtering (dev/prod)
- precedence/mutual exclusivity resolution
- batch-level policy decisions

Task graph responsibilities:

- dependency ordering
- dedupe across shared prereqs
- typed output reuse across shared prereqs
- parallel execution of independent work

## Current Direction

1. planner outputs terminal goal sets only
2. execution invokes terminal goal task roots only
3. dependency sequencing/parallelism lives in `kit/tasks`

## Pseudocode

```go
plannedGoals := planner.Reduce(batchEvents, mode, conditionFacts)

execCtx := tasks.NewCtx(nativeCtx)

_ = execCtx.RunParallel(
    EnsureGoalA.Bind(goalInputA, nil),
    EnsureGoalB.Bind(goalInputB, nil),
)

// where EnsureGoalB may call EnsureSharedPrereq internally,
// and EnsureGoalA may call the same EnsureSharedPrereq.
// It still runs once in this execution context.
```

## Design Guardrails

- Keep API/data model simple and explicit.
- Prefer deterministic behavior over "smart" implicit fallbacks.
- Keep Wave2 independent from Vorma naming and endpoint conventions.
- Avoid growing convenience wrappers that do not add real ergonomic value.
