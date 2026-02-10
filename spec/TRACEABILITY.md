# Traceability Matrix

Use this file as the cross-package map from user goals to package capabilities,
requirements, and evidence.

## Matrix Template

| User Goal ID | User Goal Summary | Capability IDs | Requirement IDs | Evidence IDs |
| ------------ | ----------------- | -------------- | --------------- | ------------ |
| GOAL-0001    | TODO              | TODO           | TODO            | TODO         |

## Rules

- Every critical user goal must map to at least one requirement.
- Every listed requirement must resolve to package-local evidence in
  `spec/packages/**/evidence.yaml`.
- If a goal cannot be mapped, add an entry to `spec/DECISIONS.md`.
