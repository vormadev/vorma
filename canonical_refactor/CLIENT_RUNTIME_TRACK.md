# Client Runtime Track

Scope: `vormaclient/client`, plus adapter packages for React, Preact, and Solid.

## End State

- Clear runtime boundaries with explicit decision/execution stages.
- Deterministic behavior under navigation, submit, and race churn.
- Strict contract tests as behavior source of truth.
- No public/internal boundary leakage.

## Constraints

- No builder-pattern API additions.
- No duplicated API paths for the same common task.
- Keep one obvious default app path.
- Remove legacy compatibility cruft instead of layering adapters.

## Active Workstreams

- Navigation runtime decomposition and race ownership hardening.
- Fetch/loader flow simplification with shared invariants.
- Slot/state mutation centralization and stale-entry protections.
- Public boundary hygiene between runtime and buildtime exports.
- Framework adapter parity and typed behavior conformance.

## Near-Term Milestones

- Finish remaining boundary cleanup in navigation runtime modules.
- Expand randomized race contracts for same-target replacement behavior.
- Reduce internal-only exports leaking through `vorma/client`.
- Keep runtime/buildtime separation strict in package exports.

## Mandatory Validation Gate

Run all commands below after any substantial client refactor leg:

```sh
pnpm oxlint vormaclient/client/src
pnpm tsc --noEmit --project vormaclient/client
pnpm tsgo --noEmit --project vormaclient/client
pnpm vitest --run vormaclient/client/src
pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary
make npmbuild
```
