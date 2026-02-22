# Wave First-Principles Remediation Checklist

Scope: `wave/*` and `wave/tooling/*`. Goal: remove vestigial complexity and land
the true first-principles end state for in-repo consumers only.

## 1) Remove test-only runtime helpers from production source

- [x] Remove test-only helpers from `wave/wave.go` and port tests to assert
      behavior through production APIs only: `parseEnvPort`,
      `copyFrameworkRuntimeFieldsFrom`, `mustPublicFS`,
      `criticalCSSStyleElementSha256Hash`, `publicFileMapElements`,
      `publicFileMapScriptSha256Hash`, `refreshScriptSha256Hash`,
      `refreshScriptInner`, `mustStaticHandler`, `faviconRedirect`.
- [x] Keep equivalent coverage by rewriting tests to use stable external/runtime
      behavior.
- [x] Run targeted `go test ./wave`.

## 2) Remove test-only exported hooks from wavecore

- [x] Remove `ResetDefaultResolverForTest` and `SetGetFreePortForTest` from
      `wave/internal/wavecore/shared.go`.
- [x] Replace with test setup that does not require production-only test hooks.
- [x] Run targeted `go test ./wave ./wave/tooling/devserver/...`.

## 3) Remove dead private methods from builder orchestration

- [x] Delete dead builder methods in `wave/tooling/builder/builder.go`:
      `compileGoOnly`, `buildAllCSS`, `readCriticalCSS`, `readNormalCSSURL`,
      `setTrackedCriticalCSSImportPaths`, `setTrackedNormalCSSImportPaths`,
      `countTrackedCriticalCSSImportPaths`, `listTrackedCriticalCSSImportPaths`,
      `getCriticalCSS`, `getNormalCSSURL`, `runHooks`.
- [x] Ensure no behavior regressions in build and hot-reload flow.
- [x] Run targeted
      `go test ./wave/tooling/builder ./wave/tooling/devserver/...`.

## 4) Remove test-only command-construction helper

- [x] Remove `buildGoBuildCommand` from `wave/tooling/builder/builder.go`.
- [x] Update tests to validate command behavior via production entrypoints, not
      a dedicated test-only helper.
- [x] Run targeted `go test ./wave/tooling/builder`.

## 5) Simplify static/fileops boundary to remove wrapper cruft

- [x] Remove alias-wrapper surface in
      `wave/tooling/builder/internal/static/static.go` that duplicates `fileops`
      without runtime need.
- [x] Keep one clear owner for hashing/atomic-write low-level primitives.
- [x] Ensure remaining API shape is minimal and coherent for in-repo consumers.
- [x] Run targeted
      `go test ./wave/tooling/builder/internal/static ./wave/tooling/builder/internal/fileops`.

## 6) Remove unused worker-count helper

- [x] Delete `determineStaticProcessingWorkerCountFromRuntime` from
      `wave/tooling/builder/internal/static/static.go` or wire it into real
      runtime logic if required.
- [x] Run targeted `go test ./wave/tooling/builder/internal/static`.

## 7) Fix schema timing string mismatch

- [x] Align schema wording/value semantics so hook timing docs match runtime
      accepted values (`concurrent-no-wait`) in
      `wave/tooling/builder/internal/schema/schema.go`.
- [x] Add regression test asserting schema/runtime timing alignment.
- [x] Run targeted
      `go test ./wave/tooling/builder/internal/schema ./wave/tooling/builder`.

## 8) Add missing watch timeout schema fields

- [x] Add `Watch.HookCommandTimeouts` and `Watch.HookCallbackTimeouts` to
      generated schema in `wave/tooling/builder/internal/schema/schema.go`.
- [x] Add regression tests proving parity between config validation and schema.
- [x] Run targeted
      `go test ./wave/tooling/builder/internal/schema ./wave/tooling/builder`.

## 9) Remove or use dead schema helper

- [x] Remove `EnsureSchemaDirectoryExists` from
      `wave/tooling/builder/internal/schema/schema.go` if not needed, or
      integrate it into real generation path.
- [x] Run targeted `go test ./wave/tooling/builder/internal/schema`.

## 10) Remove duplicate config-directory watch registration

- [x] Eliminate duplicate `addConfigFileDirectory` call path in
      `wave/tooling/devserver/devserver.go` so watcher setup has one canonical
      flow.
- [x] Add regression test for config-dir watch registration idempotence.
- [x] Run targeted `go test ./wave/tooling/devserver`.

## 11) Make port resolution fail loud, not silently to zero

- [x] Refactor `runtimeServer.MustGetPort` in
      `wave/tooling/devserver/devserver.go` to avoid panic-swallow fallback to
      `0`.
- [x] Preserve deterministic failure semantics from first principles.
- [x] Add regression test for invalid resolver behavior.
- [x] Run targeted `go test ./wave/tooling/devserver`.

## 12) Restrict refresh server listen surface

- [x] Rework refresh-server binding in `wave/tooling/devserver/devserver.go` to
      prefer loopback by default instead of all interfaces.
- [x] Add regression coverage for resolved listen address behavior.
- [x] Run targeted `go test ./wave/tooling/devserver`.

## 13) Remove dead refresh cleanup method

- [x] Remove unused `CleanupRefreshServer` in
      `wave/tooling/devserver/devserver.go` or integrate it into the canonical
      shutdown path.
- [x] Run targeted `go test ./wave/tooling/devserver`.

## 14) Deduplicate framework runtime-field copy logic

- [x] Replace manual copy block in `loadParsedConfigForReload` with one
      canonical copier to avoid drift (`wave/tooling/devserver/devserver.go` and
      `wave/wave.go`).
- [x] Keep runtime copy semantics explicit and single-sourced.
- [x] Run targeted `go test ./wave ./wave/tooling/devserver`.

## 15) Remove unconsumed restart metadata from eventpipeline

- [x] Delete or fully consume `RestartActionEncountered` and
      `RestartActionIndex` in
      `wave/tooling/devserver/internal/eventpipeline/eventpipeline.go`.
- [x] Keep reduction result shape minimal and behaviorally meaningful.
- [x] Run targeted
      `go test ./wave/tooling/devserver/internal/eventpipeline ./wave/tooling/devserver`.

## 16) Remove unused hook stage parameter

- [x] Simplify `DeriveHookStageFailurePolicy` in
      `wave/tooling/devserver/internal/hooks/hooks.go` to remove discarded
      `stageType` parameter.
- [x] Update callsites and tests accordingly.
- [x] Run targeted
      `go test ./wave/tooling/devserver/internal/hooks ./wave/tooling/devserver/internal/runloop`.

## 17) Collapse redundant restartengine decision fields

- [x] Minimize `RunBuildExecutionOrderingDecision` in
      `wave/tooling/devserver/internal/restartengine/restartengine.go` to fields
      actually used by non-test flow.
- [x] Remove duplicate/parallel representations of same state.
- [x] Add regression tests around ordering decisions through real consumers.
- [x] Run targeted
      `go test ./wave/tooling/devserver/internal/restartengine ./wave/tooling/devserver`.

## 18) Remove fail-open watcher registration behavior

- [x] Stop swallowing walk errors in `wave/tooling/internal/watch/watcher.go`.
- [x] Revisit `isKnownNonWatchablePathError` policy so permission-denied and
      similar states do not silently pass unless explicitly justified.
- [x] Add regression tests for expected failure behavior.
- [x] Run targeted
      `go test ./wave/tooling/internal/watch ./wave/tooling/devserver`.

## 19) Harden shared atomic write temp-path strategy

- [x] Replace deterministic `path + \".tmp\"` in
      `wave/tooling/internal/shared/shared.go` with collision-safe temp file
      strategy.
- [x] Preserve atomicity and cleanup guarantees under concurrent writers.
- [x] Add regression tests for concurrent-write safety.
- [x] Run targeted `go test ./wave/tooling/internal/shared`.

## 20) Fix dead branch in CSS URL skip logic

- [x] Correct `shouldSkipPublicURLResolution` ordering in
      `wave/tooling/builder/internal/css/css.go` so protocol-relative URL
      handling is explicit and non-dead.
- [x] Add regression test for protocol-relative inputs.
- [x] Run targeted `go test ./wave/tooling/builder/internal/css`.

## Final verification gate

- [x] Run `make gotest`.
- [x] Run `make full-gate`.
- [x] Manual source pass on `wave/*` to confirm no vestigial/test-only
      production symbols remain.
