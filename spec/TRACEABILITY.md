# Traceability Matrix

Use this file as the cross-package map from user goals to capabilities,
requirements, ownership, and evidence.

## Entry Shape

Add append-only JSON objects inside the fenced block below.

```json
[]
```

Object shape:

- `user_goal_id`: `GOAL-0001`
- `user_goal_summary`: concise user-goal statement
- `capability_ids`: list of `CAP-*` IDs
- `requirement_ids`: list of `REQ-*` IDs
- `owner_package_paths`: list of `spec/packages/...` paths
- `upstream_requirement_refs`: list of upstream `REQ-*` IDs (or empty)
- `evidence_ids`: list of `EVID-*` IDs

## Rules

- Every critical user goal maps to at least one requirement.
- Every listed requirement resolves to package-local evidence in
  `spec/packages/**/spec.json`.
- Every requirement has exactly one owner package path.
- Inherited behavior references upstream requirement IDs instead of duplicating
  normative text.
- No placeholder values (`TODO`/`TBD`) are allowed in traceability entries.
- If a goal cannot be mapped, add an entry to `spec/DECISIONS.md`.
