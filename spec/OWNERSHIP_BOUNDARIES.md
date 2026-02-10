# Ownership Boundaries

This file defines how package specs avoid duplicate normative definitions.

## Core Rule

Each normative behavior is owned exactly once by one package spec.

- Owner package: defines the full normative requirement and supporting evidence.
- Consumer package: references the owner requirement and defines only local
  deltas.

## Required Pattern

When a package reuses behavior from another package:

1. Do not restate the full upstream requirement text.
2. Add an inherited/consumed requirement row with an upstream reference.
3. Specify only local adaptation details in the local statement.

Use `Upstream Requirement Refs` in `20-requirements.md` for cross-package links.

## Allowed Cases

- `OWNED`: behavior is defined first in this package.
- `INHERITED`: behavior comes from an upstream package without semantic change.
- `DELTA`: behavior comes from upstream plus local additions/constraints.

## Disallowed Case

- Duplicating the same normative behavior in multiple package specs without
  explicit owner and reference linkage.

## Conflict Handling

If two packages appear to own the same normative behavior:

1. Record the conflict in `spec/DECISIONS.md`.
2. Select one owner package.
3. Update the non-owner package to reference the owner.

## Example

- Owner requirement: `REQ-VORMACLIENT-CLIENT-0012`
- Consumer package: `spec/packages/vormaclient/react`
- Consumer row:
    - Local statement: React adapter must preserve the owner navigation
      contract.
    - Upstream Requirement Refs: `REQ-VORMACLIENT-CLIENT-0012`
