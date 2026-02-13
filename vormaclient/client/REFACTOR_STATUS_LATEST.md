# Refactor Status Latest

Last updated: 2026-02-13

## Open Items

- Add adapter tests for `react/preact/solid` link wrappers
  (`vormaclient/*/src/link.tsx`) covering typed href resolution and forwarded
  event-handler behavior.
- Add adapter tests for `react/preact/solid` root outlet runtime synchronization
  (`vormaclient/*/src/{react,preact,solid}.tsx`) covering route-change updates
  and scroll-state application.
- Decide whether `makeTypedAddClientLoader` should keep swallowing pattern
  registration failures via logging or expose a stronger failure path for app
  code.
- Evaluate extracting shared runtime-state/outlet remount logic into a common
  internal helper to keep `react/preact/solid` behavior locked together.
