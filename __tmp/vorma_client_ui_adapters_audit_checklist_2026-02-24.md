# Vorma Client/UI Adapters Audit Follow-Up Checklist (2026-02-24)

## Fixes

- [x] Block client-only skip path for `redirect` navigation type in
      `typescript/vorma/client/src/core/navigation/fetch_route_data_skip.ts`.
- [x] Respect consumer cancellation (`event.preventDefault()`) in composed link
      handlers in `typescript/vorma/client/src/ui/helpers.ts`.
- [x] Remove `preact/compat` `memo` usage from preact link adapter in
      `typescript/vorma/ui-adapters/preact/src/link.tsx`.

## New Regression Tests

- [x] Add submit redirect freshness contract test:
      redirect-to-current/skip-eligible target must still fetch server route
      data.
- [x] Add link helper contract test: user `onClick` calling `preventDefault()`
      must cancel internal navigation.

## Validation

- [x] Run Prettier on all edited `.ts`, `.tsx`, and `.md` files.
- [x] Run targeted contract tests covering submit/redirect and link click
      behavior.
- [x] Run targeted dist adapter tests to validate preact adapter behavior after
      memo/compat removal.
- [x] Validate with Makefile-intended test split: source tests exclude dist
      cases; dist tests run with
      `typescript/vorma/client/vitest.dist.config.ts`.

## Hash-Only + Skip Removal Scope

- [x] Guarantee hash-only URL changes never fetch from the server through any
      navigation path, not only link wrappers.
- [x] Remove all client skip logic and skip-dependent behavior from runtime and
      route-data fetch flows.
- [x] Remove skip-related downstream cruft (helpers/types/tests/wiring) so no
      vestigial skip branches remain.
- [x] Add/update tests to cover hash-only no-fetch behavior and no-skip fetch
      behavior.
- [x] Update client-navigation comments/docs for the resolved contract and run
      formatting + Makefile-aligned tests (including dist-focused UI adapter
      tests).
