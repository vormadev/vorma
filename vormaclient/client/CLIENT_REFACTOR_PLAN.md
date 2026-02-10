# vormaclient/client Major Refactor Plan

## Goal

Refactor the client runtime into a cleaner, more maintainable architecture while
keeping behavior locked to the strict contract suite.

## Non-Negotiables

- Contract behavior is the acceptance gate.
- No weak assertions; first-principles correctness only.
- Keep legacy tests in-tree with `READY_TO_DELETE_AFTER_SIGNOFF` markers until
  explicit signoff.
- Avoid adding new runtime cycles.

## Target Architecture

- Functional/core style runtime (no class-based state holders in runtime code).
- Explicitly scoped state containers (navigation, scroll, history) with small
  API surfaces.
- Thin public API facade in `src/client.ts`.
- Narrow integration seams between modules (`history`, `redirects`,
  `navigation`).

## Work Streams

1. Runtime shape cleanup

- Replace class-based runtime managers with closure/object modules.
- Preserve all externally consumed method names and behavior.

2. Navigation/runtime boundary cleanup

- Reduce hidden coupling to `client.ts` from deep runtime modules.
- Use explicit access bridges where runtime internals need shared state access.

3. Monolith decomposition

- Carve `src/client.ts` into focused files (state machine, fetch/lifecycle,
  submission handling, status/event emission) once behavior is stable.

4. API + contract hardening

- Keep public API stable (`index.ts`).
- Expand contracts only when new behavior is made explicit.

## Acceptance Criteria

- `pnpm vitest --run vormaclient/client/src/contracts` passes.
- `pnpm vitest --run vormaclient/client/src` passes.
- Runtime modules are class-free.
- Refactor progress and verification are logged in
  `CLIENT_REFACTOR_PROGRESS.md`.
