# Refactor Status Latest

Last updated: 2026-02-13

## Open Items

- Add a compiled-output (`npm_dist`) adapter test pass for
  `client/react/preact/solid` and treat it as a release gate.
- After `npm_dist` tests exist, decide whether additional source-level Solid
  behavioral tests are still needed.
- Decide whether `makeTypedAddClientLoader` should keep swallowing pattern
  registration failures via logging or expose a stronger failure path for app
  code.
- Evaluate extracting shared runtime-state/outlet remount logic into a common
  internal helper to keep `react/preact/solid` behavior locked together.
