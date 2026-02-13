# Idea Backlog

Purpose: hold candidate ideas and zany experiments without polluting active
refactor tracks.

## Backlog Rules

- This file is not the active tracks document.
- Active tracks live in `ACTIVE_EXECUTION_PLAN.md`.
- Keep entries as candidates until explicitly promoted or rejected.

## Active Candidates

```json
[
	{
		"id": "P03",
		"proposal": "Strict frontend route DSL + strict manifest generation",
		"direction": "keep",
		"acceptanceGate": "Unresolved/dynamic frontend route definitions fail fast with clear diagnostics"
	}
]
```

## Parked For Revisit

```json
[
	{
		"id": "P06",
		"proposal": "Typed immutable context extension pipeline",
		"revisitCondition": "Revisit only if simpler than current typed context decorators"
	},
	{
		"id": "P08",
		"proposal": "Explicit app client instance (createAppClient)",
		"revisitCondition": "Revisit only if default usage is at least as simple as global helper path"
	},
	{
		"id": "P10",
		"proposal": "Remove all implicit singleton behavior",
		"revisitCondition": "Revisit only with zero regression to default-path ergonomics"
	},
	{
		"id": "P12",
		"proposal": "Explicit transport/serialization codec contracts",
		"revisitCondition": "Revisit with concrete user demand and low-ceremony shape"
	},
	{
		"id": "P13",
		"proposal": "Stable plugin API as kernel boundary",
		"revisitCondition": "Revisit after core boundaries settle"
	}
]
```

## Rejected Unless Reopened

- Route IDs as primary default authoring identity.
- Unified default primitive for loader/query/mutation.
- Any non-JSON app-authored config path.
- Default-required module/feature composition manifest.
- Helper-name-only backend route discovery.
- Any default model requiring extra backend registration file edits per route.
- Framework-enforced application package-organization model.

## Zany Ideas Inbox

- Keep experimental ideas here until they have concrete constraints and
  acceptance gates.
- Promote ideas only if they preserve the UX contract in
  `RULES_AND_GUARDRAILS.md`.
