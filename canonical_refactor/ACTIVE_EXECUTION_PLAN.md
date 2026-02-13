# Active Refactor Tracks

## Program Focus

Wave simplification and hardening is the primary refactor focus.

## Core Wave Objectives (In Scope, Not Backlog)

```json
[
	{
		"id": "P05",
		"objective": "Explicit internal lifecycle phases",
		"context": "Wave devserver/build internals should operate as explicit lifecycle stages with clear boundaries instead of implicit intertwined control flow",
		"primaryTargets": [
			"wave/tooling/devserver.go",
			"wave/tooling/events.go"
		],
		"acceptanceGate": "No user-facing ceremony increase"
	},
	{
		"id": "P14",
		"objective": "Wave devserver pure pipeline (classify -> plan -> execute)",
		"context": "Watcher events should always flow through a deterministic pipeline where classification and execution planning are explicit data steps before side effects",
		"primaryTargets": ["wave/tooling/events.go"],
		"acceptanceGate": "Planner output is deterministic and easier to reason about"
	},
	{
		"id": "P15",
		"objective": "Artifact DAG with fingerprinted incremental rebuild nodes",
		"context": "Build/rebuild work should be represented as explicit artifact dependencies keyed by stable fingerprints rather than scattered ad hoc flags",
		"primaryTargets": ["wave/tooling/builder.go", "wave/tooling/events.go"],
		"acceptanceGate": "Rebuild cost drops at high route counts"
	},
	{
		"id": "P19",
		"objective": "Diagnostics expansion (wave explain, wave doctor)",
		"context": "Diagnostics should explain trigger origin, plan decisions, and rebuild/restart behavior in source-oriented terms that are directly actionable",
		"primaryTargets": ["wave/tooling/diagnostics.go"],
		"acceptanceGate": "Trigger/conflict/rebuild reasoning is explicit and source-oriented"
	}
]
```

## Active Tracks (No Prescribed Sequence)

- Simplify Wave event processing internals while preserving current app-author
  ergonomics.
- Collapse needless configuration indirection in Wave internals while keeping
  JSON as the sole authored app config path.
- Tighten Wave diagnostics so they explain rebuild/restart behavior directly in
  terms users can act on.
- Keep runtime/build tooling boundaries explicit and robust.
- Keep conformance and stress validation broad enough to catch regressions while
  internals are being aggressively simplified.

## Wave Audit Findings (Needless Indirection Targets)

```json
[
	{
		"id": "WAVE-AUD-001",
		"area": "config reload indirection",
		"status": "implemented",
		"observation": "config reload no longer depends on source interfaces; runtime state now stores an explicit resolved JSON config file path and reload reparses bytes from disk",
		"evidence": [
			"wave/config_runtime_parse.go",
			"wave/config_runtime_state.go",
			"wave/tooling/devserver.go",
			"wave/wave.go"
		],
		"refactorDirection": "keep JSON reload metadata explicit and avoid reintroducing source abstraction layers"
	},
	{
		"id": "WAVE-AUD-002",
		"area": "stale provider naming and structure",
		"status": "implemented",
		"observation": "provider-oriented naming and protocol files were removed, and the follow-on ConfigDependencies abstraction was removed entirely",
		"evidence": [
			"wave/wave.go",
			"wave/config_runtime_state.go",
			"vormabuild/vorma_gen_ts_vite_and_write.go"
		],
		"refactorDirection": "keep reload metadata focused on the resolved config file path only"
	},
	{
		"id": "WAVE-AUD-003",
		"area": "config runtime-state normalization churn",
		"status": "implemented",
		"observation": "runtime state no longer stores dependency slices or matcher state; only resolved config file path and fingerprint are retained",
		"evidence": ["wave/config_runtime_state.go"],
		"refactorDirection": "keep runtime config state scalar and direct, without collection normalization layers"
	},
	{
		"id": "WAVE-AUD-004",
		"area": "event pipeline duplication",
		"status": "active",
		"observation": "single-event and batched-event flows duplicate pre/concurrent/post hook orchestration, restart handling, and workset application logic",
		"evidence": ["wave/tooling/events.go"],
		"refactorDirection": "unify execution flow into one deterministic pipeline with explicit batch policy points"
	},
	{
		"id": "WAVE-AUD-005",
		"area": "refresh-action determinism",
		"status": "active",
		"observation": "applyRefreshActions exits on first restart action while concurrent hook actions are appended from goroutines in completion order, making restart-action interpretation order-sensitive",
		"evidence": ["wave/tooling/events.go"],
		"refactorDirection": "reduce refresh actions deterministically before applying restart decisions"
	},
	{
		"id": "WAVE-AUD-006",
		"area": "diagnostics config signal quality",
		"status": "implemented",
		"observation": "diagnostics now report concrete config_file_path and resolved-file accessibility signals instead of source-type/dependency-vector output",
		"evidence": ["wave/tooling/diagnostics.go"],
		"refactorDirection": "keep diagnostics source-oriented around concrete config-file reload behavior"
	},
	{
		"id": "WAVE-AUD-007",
		"area": "config wrapper surface cruft",
		"status": "implemented",
		"observation": "wavecfg package was vestigial config-as-code surface from a later refactor leg and had no non-test usages; package removed",
		"evidence": [
			"wavecfg/document.go",
			"wavecfg/runtime.go",
			"canonical_refactor/API_SIMPLIFICATION_AUDIT.md"
		],
		"refactorDirection": "keep Wave config entrypoints directly on wave JSON config paths"
	}
]
```

## Ongoing Guardrails

- Keep `BREAKING_CHANGES_LEDGER.md` measured only against `main`.
- Keep docs forward-looking and decision-oriented.
- Keep Wave work aligned with `RULES_AND_GUARDRAILS.md`.
