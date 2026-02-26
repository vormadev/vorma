# Vorma Cleanup Pass Checklist (2026-02-25)

Goal: remove now-vestigial abstractions/exports after the shared atomic
transition model refactor, while preserving behavior and test coverage.

- [x] Remove successful-navigation global commit wrapper abstraction and keep
      one direct `syncBuildIDFromResponse` path.
- [x] Remove post-asset side-effect planner wrapper from state machine and keep
      side-effect decision local to successful-runtime checkpoint.
- [x] Remove `deriveAndSetErrorState` export/function and corresponding
      re-export seam.
- [x] Minimize `setClientLoadersState` export surface if now test-only.
- [x] Update affected unit tests to target the deeper shared runtime seams.
- [x] Run `pnpm prettier --write` on changed TypeScript/Markdown files.
- [x] Run source TypeScript tests.
- [x] Rebuild TS dist artifacts.
- [x] Run dist TypeScript tests.
- [x] Add direct successful-runtime coverage for post-asset build-ID sync timing
      (`after wait iff not stopped`) and re-run source tests.
