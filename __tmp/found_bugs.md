# Found Bugs Tracker

Last updated: 2026-03-05

## Current State

- Backend discovered-route typing/registration inconsistency between hook-only
  execution paths is fixed with a canonical outer/inner hook model: `--hook`
  delegates to framework hook orchestration and `--hook-inner` executes
  build-hook work directly.
- The temporary env-marker recursion guard path has been removed.
- Direct hook-only builds and framework-run hook builds now converge on the same
  discovered-route overlay behavior and backend typing output.
- Verified via direct `go run ... --dev --hook` in both local framework fixture
  app and a separate non-repo app fixture.
- Canonical hook-path regression tests and package tests are green across the
  touched vormabuild/backend-route packages.
- New investigation found dev-lifecycle process-leak behavior consistent with
  duplicate/stray Vite logs: orphaned Vite/app processes can remain alive
  (`PPID=1`) and continue emitting logs.
- Comparing with `main`: single-process termination behavior (terminate only the
  tracked parent process, not an entire wrapper-child tree) is longstanding from
  earlier `kit/viteutil` lifecycle code.
- Current branch adds extra fragility in Wave integration by clearing
  `ViteContext` before cleanup/start success is known, which can drop the only
  handle needed for diagnostics/retry when start/stop paths fail.
- Dev lifecycle now terminates Vite wrapper-child process trees, surfaces
  cleanup errors to Wave orchestration, and retains `ViteContext` across
  start/stop failures for retry/diagnostics safety.
- Added non-E2E lifecycle coverage for wrapper-child tree termination and
  Vite-context-retention semantics.
- Validation status for this change set: `make gotest` passed (2026-03-05);
  `make e2e-test` repeatedly fails in three dev-variant cases with `ENOTEMPTY`
  during temp fixture directory removal.
- Re-running `make e2e-test` after explicit orphan-process cleanup still fails
  the same dev-variant test group (`framework.integration.shared.ts:572`) with
  fixture cleanup `ENOTEMPTY`; one run also included a dev-react hydration
  timeout while the same suite was failing.
- Config-edit handling around `wave.config.json` still has unresolved issues
  where Vorma-related changes are not handled consistently across repeated
  edits/reloads.
- Route-definition edit flakiness was reproduced as expected-build-id mismatch
  causing restart; that specific issue is fixed in framework code and has
  regression coverage.

## Open Items (Checklist)

- [x] Reproduce and isolate the exact condition where hook-only build path loses
      discovered backend output typing while route patterns are still present.
- [x] Unify hook-only build behavior with full framework build-hook behavior so
      discovered registration/typing is identical in both paths.
- [x] Add/adjust regression coverage for the canonical hook flow (`--hook` outer
      orchestration, `--hook-inner` execution path) so discovered typing
      behavior remains stable.
- [ ] Fix config comparator/reload behavior so any semantic edit inside `Vorma`
      config is detected and applied.
- [ ] Fix schema regeneration/reload behavior so `Vorma` schema fields are never
      dropped after repeated config edits.
- [ ] Verify route-definition file edits are always deterministic: no silent
      no-op, no unnecessary full rebuild, and no mismatch-based restart.
- [ ] Add/expand non-E2E regression tests for the config-edit and route-edit
      paths above.
- [x] Fix dev-lifecycle process termination so stopping/restarting dev cannot
      leave orphaned Vite/app processes (including wrapper-child process trees).
- [x] Fix Vite shutdown error propagation so failed cleanup is surfaced to
      devserver orchestration rather than silently dropped.
- [x] Stop clearing `ViteContext` before cleanup/start success is known; keep a
      recoverable handle until lifecycle transition is confirmed.
- [x] Add regression tests that model wrapper/child process trees and verify no
      orphaned Vite child remains after `StopVite`/`CycleVite`/restart.
- [ ] Investigate deterministic `make e2e-test` failures in dev variants where
      fixture temp directory removal returns `ENOTEMPTY`.

## Closed Items

- [x] Fixed route-edit reload mismatch where runtime endpoint rejected
      fast-rebuild reload requests due expected-build-id mismatch.
- [x] Added regression coverage for expected-build-id behavior in deferred
      reload request generation.
- [x] Fixed hook-only build path inconsistency where direct `go run ... --hook`
      could miss discovered backend loader typing.
- [x] Replaced temporary hook-only env-marker/overlay-recursion guard with
      canonical outer/inner hook execution (`--hook`, `--hook-inner`).
- [x] Updated buildentry/buildenv regression coverage for new hook execution
      semantics and hook flag parsing.
- [x] Fixed Vite dev lifecycle termination to target wrapper-child process
      trees, not only wrapper parent PIDs.
- [x] Fixed Wave Vite lifecycle state handling to avoid dropping context handles
      before stop/start success is known.
- [x] Added regression tests covering wrapper-child cleanup behavior and Vite
      context retention on start failure.

## Notes

- This file is the active bug tracker for current framework investigation status
  and next actions.
- `--hook-inner` is internal-only and should not be used as an app-facing hook
  command.
