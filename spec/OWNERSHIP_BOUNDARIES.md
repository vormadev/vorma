# Ownership Boundaries

This file defines how package specs avoid duplicate normative definitions.

## Core Rule

Each normative behavior is owned exactly once by one package spec.

- Owner package: defines full normative requirements and evidence.
- Consumer package: references upstream requirements and defines only local
  deltas.

## Required Pattern

When a package reuses behavior from another package:

1. Do not restate full upstream normative statements.
2. Add a requirement with `ownership = INHERITED` or `ownership = DELTA`.
3. Populate `upstream_requirement_refs` with upstream requirement IDs.
4. Define only local adaptation behavior in local normative statements.

## Allowed Cases

- `OWNED`: behavior is defined first in this package.
- `INHERITED`: behavior comes from upstream without semantic change.
- `DELTA`: behavior comes from upstream plus local additions/constraints.

## Disallowed Case

- Duplicating the same normative behavior in multiple package specs without
  explicit owner and reference linkage.

## Conflict Handling

If two packages appear to own the same normative behavior:

1. Record the conflict in `spec/DECISIONS.json`.
2. Select one owner package.
3. Update the non-owner package requirement to reference the owner.

## Example

- Owner requirement: `REQ-VORMACLIENT-CLIENT-0012`
- Consumer package path: `spec/packages/vormaclient/react`
- Consumer requirement fields:
    - `ownership = DELTA`
    - `upstream_requirement_refs = ["REQ-VORMACLIENT-CLIENT-0012"]`
    - local `normative_statements` describe only adapter-specific behavior.
