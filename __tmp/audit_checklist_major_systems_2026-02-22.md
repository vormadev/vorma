# Audit Checklist (Created 2026-02-22)

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Code Issues

- [x] Fix stale HTTP middleware chain cache in `kit/mux/mux.go` (compiled chain
      was not invalidated after late middleware registration).
- [x] Fix transient nested-router inconsistency in `kit/mux/nested_mux.go`
      (route registration and compiled-route index update were split across lock
      windows).
- [x] Fix nested-router matcher read/write race in `kit/mux/nested_mux.go`
      (`FindNestedMatches` released lock before matcher access).
- [x] Fix `ViteContext` synchronization in `wave/tooling/devserver/devserver.go`
      (mixed locked writes and unlocked reads).
- [x] Fix watcher-start channel synchronization in
      `wave/tooling/devserver/devserver.go` (removed shared mutable
      `WatcherStartCh`; now uses per-cycle local channels).
- [x] Fix check-then-register race in `internal/vormaruntime/vorma_init.go`
      (`validateAndDecorateNestedRouter` now uses atomic idempotent
      registration).
- [x] Fix nil-context watcher panic path in
      `wave/tooling/devserver/internal/runloop/runloop.go`
      (`RunWatcherWithContext` now normalizes nil to `context.Background()`).
- [x] Fix semantic duplicate default watch-pattern injection in
      `vormabuild/build_watch.go` (route definition watch patterns now dedupe by
      normalized absolute path, not just raw string equality).
- [x] Fix mixed snapshot risk in `kit/mux/nested_mux.go` (compiled routes and
      pattern index map now load/store atomically as one snapshot).
- [x] Fix discovered registration call-site collapse in
      `vormabuild/backend_route_registration_generation.go` (discovered call ID
      now includes source position, preventing distinct same-expression call
      sites from deduping together).
- [x] Fix same-line discovered registration call-site identity in
      `vormabuild/backend_route_registration_generation.go` (discovered call ID
      now includes source column in addition to file and line, preventing
      distinct same-expression call sites on the same line from collapsing).
- [x] Fix build-retry restart intent downgrade in
      `wave/tooling/devserver/devserver.go` (`QueueRestartRequest` no longer
      drops stronger intents while waiting for build retry; intents now always
      merge to strongest semantics).
- [x] Fix refresh-action restart reduction downgrade in
      `wave/tooling/devserver/internal/eventpipeline/eventpipeline.go`
      (`ReduceRefreshActionsInStableOrder` now merges restart actions so later
      `RecompileGo=true` requests cannot be downgraded by earlier weaker restart
      actions).
- [x] Fix HEAD fallback error-path body handling in `kit/mux/mux.go` (HEAD
      fallback to GET now preserves no-body semantics when request
      parsing/validation fails before route handler execution).
- [x] Fix HEAD fallback inferred-header parity in `kit/mux/mux.go`
      (`treatGetAsHead` now captures GET execution with a recorder and preserves
      inferred headers like `Content-Type`, while explicitly setting
      `Content-Length` from the would-be GET body length).
- [x] Fix readiness-policy partial-defaulting in
      `wave/tooling/devserver/internal/runtimeprocess/runtimeprocess.go`
      (`WaitForAnyReady` now normalizes each policy field independently so
      caller-provided delay/timeout overrides are not discarded when only
      `HTTPClientTimeout` is unset).
- [x] Fix build-retry watcher freeze in `wave/tooling/devserver/devserver.go`
      (while lifecycle is `awaiting_build_retry`, browser-phase execution is now
      skipped so watcher/debouncer callbacks cannot block on reload readiness
      timeouts before processing follow-up fix events).
- [x] Add direct reload-broadcast guard for build-retry wait in
      `wave/tooling/devserver/devserver.go` (`BroadcastReload` now
      short-circuits when `WaitingForBuildRetry=true`, preventing non-runloop
      call paths from emitting payloads or entering readiness waits during retry
      wait state).
- [x] Add build-retry responsiveness regression tests for syntax and
      config/source error classes in
      `wave/tooling/devserver/site_regression_additional_test.go`
      (`TestSiteRegression_WaitingForBuildRetry_GoSyntaxErrorThenFixDoesNotBlock`
      and `TestSiteRegression_WaitingForBuildRetry_ErrorClassEventsDoNotBlock`)
      to ensure watcher batches return promptly while waiting for build retry
      and do not emit reload/revalidate payloads that would trigger readiness
      timeout waits.
- [x] Fix retry-wait lifecycle unblock semantics in
      `wave/tooling/devserver/internal/runloop/runloop.go` (when lifecycle is
      waiting for build retry, watcher batches now queue a restart intent and
      skip in-place hook/build/browser pipeline execution).
- [x] Add end-to-end and runloop-level retry-wait unblock regression tests:
      `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_GoSyntaxErrorThenQuickFixRecoversWithoutReadinessStall` and
      `wave/tooling/devserver/internal/runloop/events_batched_watcher_test.go`:
      `TestProcessEvents_WaitingForBuildRetryQueuesRestartAndSkipsPipeline`.
- [x] Add browser-mode `MainAppEntry` typo/fix retry-wait recovery regression
      tests in `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_MainAppEntryTypoThenQuickFixRecoversWithoutReadinessStall`
      and
      `TestServerRun_MainAppEntryTypo_NonConfigEventThenQuickFixRecoversWithoutReadinessStall`
      to cover both direct config-fix recovery and the interleaved non-config
      watcher-event path that previously led to apparent devserver
      unresponsiveness.
- [x] Expand runloop retry-wait watcher-op coverage in
      `wave/tooling/devserver/internal/runloop/events_batched_watcher_test.go`
      so `TestProcessEvents_WaitingForBuildRetryQueuesRestartAndSkipsPipeline`
      now validates `WRITE`, `CREATE`, `RENAME`, and `REMOVE` file-operation
      variants.
- [x] Add `kit/mux` HEAD fallback inferred-header regression coverage in
      `kit/mux/mux_test.go`:
      `TestHTTPHandlers/HEAD_Fallback_To_GET_PreservesInferredHeaders`.
- [x] Fix watcher-batch responsiveness for non-cycle browser readiness waits in
      `wave/tooling/devserver/devserver.go` (reload paths that require
      `WaitApp`/`WaitVite` now run readiness waits asynchronously with
      cancellation and generation guards, so new watcher batches are not blocked
      behind readiness timeout budgets).
- [x] Fix devserver port resolver mode snapshot in
      `wave/tooling/devserver/devserver.go` (`RunDev` and `MustGetPort` now use
      `wavecore.NewResolverForMode(true)` to enforce dev-mode port semantics for
      devserver lifecycle operations).
- [x] Add cancellation-aware readiness wait API in
      `wave/tooling/devserver/internal/runtimeprocess/runtimeprocess.go`
      (`WaitForAnyReadyWithContext`) so in-flight probe loops can terminate
      immediately when superseded by newer reload work.
- [x] Add browser reload responsiveness and cancellation regression coverage in
      `wave/tooling/devserver/broadcast_behavior_test.go`:
      `TestBroadcastReload_WaitAppDoesNotBlockSubsequentReloads` and
      `TestBroadcastReload_CleanupCancelsOutstandingReadinessWait`.
- [x] Add readiness cancellation regression coverage in
      `wave/tooling/devserver/internal/runtimeprocess/devserver_wait_ready_test.go`:
      `TestWaitForAnyReadyWithContext_CancellationStopsWaitEarly`.
- [x] Add watcher-batch long-duration warning guard in
      `wave/tooling/devserver/internal/runloop/runloop.go` (batches that exceed
      the configured/default threshold now emit a warning with cycle/batch trace
      fields and retry-wait state context).
- [x] Add watcher-batch duration warning regression coverage in
      `wave/tooling/devserver/internal/runloop/events_process_test.go`:
      `TestProcessEvents_LogsWarningWhenBatchDurationExceedsThreshold` and
      `TestProcessEvents_DoesNotLogBatchDurationWarningWhenBelowThreshold`.
- [x] Fix config-reload failure run termination in
      `wave/tooling/devserver/devserver.go` (`prepareRunCycle` now logs config
      reload failures and continues with the previous valid in-memory config
      instead of aborting the devserver lifecycle).
- [x] Add config syntax/validation reload-failure resilience regression coverage
      in `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_ConfigReloadFailureDoesNotTerminateRun`.
- [x] Add browser-mode Go type-error recovery regression coverage in
      `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_GoTypeErrorThenQuickFixRecoversWithoutReadinessStall`.
- [x] Add browser-mode config syntax/validation quick-fix responsiveness
      coverage in `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_ConfigReloadErrorThenQuickFixRecoversWithoutReadinessStall`.
- [x] Add layered build-retry + config-error recovery coverage in
      `wave/tooling/devserver/devserver_run_test.go`:
      `TestServerRun_MainAppEntryTypo_ConfigErrorEventThenQuickFixRecoversWithoutReadinessStall`
      (covers syntax and validation config errors injected during
      `awaiting_build_retry` before a quick fix).
- [x] Fix `vormabuild` watch-hook execution-context propagation in
      `vormabuild/build_watch.go` and `vormabuild/reload_endpoint.go` (reload
      endpoint requests now inherit hook callback execution context so callback
      timeout/cancellation can interrupt stuck endpoint calls instead of always
      waiting on the internal 10s request budget).
- [x] Add execution-context propagation coverage for `vormabuild` watch reload
      callbacks and reload endpoint execution in
      `vormabuild/watch_hooks_test.go` and `vormabuild/reload_endpoint_test.go`
      (covers non-nil/nil hook-context passthrough and canceled-context request
      failure behavior).
- [x] Fix runloop pre/post hook callback context source in
      `wave/tooling/devserver/internal/runloop/runloop.go` (`RunPreHooks` and
      `RunPostHooks` now derive callback execution context from
      `CurrentRunCycleContextOrBackground` instead of `context.Background()`, so
      run-cycle cancellation can interrupt callback work consistently across
      pre/concurrent/post stages).
- [x] Add run-cycle cancellation coverage for pre/post callback stages in
      `wave/tooling/devserver/internal/runloop/events_hook_execution_test.go`
      (`TestRunPreHooks_UsesRunCycleContextForCallbackExecution` and
      `TestRunPostHooks_UsesRunCycleContextForCallbackExecution`).

## Coverage / Verification

- [ ] Raise test coverage for requested scope to 100%.
- [x] Re-run `go test` and `go test -race` on modified packages after fixes.
- [x] Re-run full repository unit test suite (`go test ./...`) after changes.
- [x] Recompute current scope coverage baseline.
- [ ] Eliminate remaining coverage gaps.

## Proactive Robustness Follow-Ups

- [x] Add an end-to-end devserver regression that reproduces: build-failure ->
      non-config watch event -> immediate config fix, and asserts the
      retry-restart is processed without waiting for readiness timeout budgets.
- [x] Add watcher-batch phase duration guards/logging in runloop to flag when a
      single batch callback monopolizes the debouncer for unexpectedly long
      intervals.
- [x] Evaluate a bounded/asynchronous browser readiness wait strategy so browser
      payload gating cannot monopolize watcher callback execution in edge cases
      outside build-retry mode.

Current coverage snapshot
(`go test ./wave/... ./internal/vormaruntime ./kit/mux ./vormabuild ./vormagogen -cover`):

- `wave`: 94.9%
- `wave/internal/wavecore`: 40.0%
- `wave/internal/waveruntime`: 3.1%
- `wave/tooling/builder`: 76.0%
- `wave/tooling/builder/internal/css`: 68.2%
- `wave/tooling/builder/internal/fileops`: 63.6%
- `wave/tooling/builder/internal/schema`: 85.7%
- `wave/tooling/builder/internal/static`: 77.8%
- `wave/tooling/devserver`: 80.4%
- `wave/tooling/devserver/internal/eventpipeline`: 90.5%
- `wave/tooling/devserver/internal/hooks`: 91.1%
- `wave/tooling/devserver/internal/restartengine`: 61.9%
- `wave/tooling/devserver/internal/runloop`: 86.0%
- `wave/tooling/devserver/internal/runtimeprocess`: 79.4%
- `wave/tooling/internal/broadcast`: 66.5%
- `wave/tooling/internal/shared`: 67.0%
- `wave/tooling/internal/watch`: 70.0%
- `wave/tooling/internal/watch/classification`: 75.4%
- `wave/tooling/internal/watch/dedup`: 76.9%
- `internal/vormaruntime`: 91.9%
- `kit/mux`: 89.3%
- `vormabuild`: 90.9%
- `vormagogen`: 100.0%

## AGENTS.md Compliance Items

- [x] Add explicit `vormagogen` exception text to `AGENTS.md`.
- [x] Clarify alias rule scope in `AGENTS.md`: it applies to internal repository
      package names; external import aliasing is allowed when it improves
      clarity or avoids collisions.
- [x] Revert external import naming regression in
      `vormabuild/route_parsing_pipeline.go` (`esbuild` alias restored; removed
      generic `api` naming).
- [x] Validate scoped package compliance matrix for one-file/200-2000 rule.
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `internal/vormaruntime` (18 non-test files, 3044 lines total).
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `kit/mux` (2 non-test files, 1741 lines total). For this one, we should
      move the "nested" code to `kit/mux/nested` instead of concat'ing it into
      `kit/mux`, even though it could fit in under 2k lines.
- [ ] Bring non-exempt noncompliant packages into one-file/200-2000 compliance:
      `vormabuild` (35 non-test files, 10424 lines total).

## Decision

- [x] `vormagogen` is approved as an exception to the 200-line minimum.
