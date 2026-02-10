# Traceability Matrix

Use this file as the cross-package map from user goals to package capabilities,
requirements, and evidence.

## Matrix Template

| User Goal ID | User Goal Summary | Capability IDs | Requirement IDs | Owner Package Paths | Upstream Requirement Refs | Evidence IDs |
| ------------ | ----------------- | -------------- | --------------- | ------------------- | ------------------------- | ------------ |
| GOAL-0001    | TODO              | TODO           | TODO            | TODO                | TODO                      | TODO         |

## Rules

- Every critical user goal must map to at least one requirement.
- Every listed requirement must resolve to package-local evidence in
  `spec/packages/**/evidence.yaml`.
- Every requirement should have exactly one owner package path.
- Inherited behavior must reference upstream requirement IDs instead of
  duplicating normative text.
- If a goal cannot be mapped, add an entry to `spec/DECISIONS.md`.
