# AGENT_HANDOFF

## Scope

- Work only in `vormaruntime/`.
- Tests and benchmarks should encode first-principles external behavior, not
  incidental implementation quirks.
- Keep frontend/backend public contract shape stable while tightening
  correctness.

## Current Snapshot (2026-02-11)

- The package now has a broad regression harness covering:
    - loader/action HTTP contracts (status/header/content-type/JSON shape)
    - loader redirect short-circuit contracts (HTML/JSON and client-redirect
      header semantics)
    - loader `HEAD` contracts via default router (`HEAD` no-body behavior for
      current-build JSON, stale-build reload signal, and HTML requests)
    - action redirect short-circuit contracts (server redirect vs
      `X-Client-Redirect`)
    - action parse/error contracts in matrix form (invalid JSON, typed-input
      form rejection, unsupported method, unknown-path behavior, and
      `GET`/`HEAD`/`POST`/`PUT`/`PATCH`/`DELETE` happy-path coverage)
    - loader JSON header/cache contracts in matrix form (loader-set headers,
      including `Cache-Control`, are preserved)
    - custom action mount-root contract via `ActionsRouterOptions.MountRoot`
      (mount-pattern normalization and route isolation)
    - `SupportedMethods` casing normalization contract (configured methods are
      trimmed/canonicalized to uppercase, with blank entries ignored)
    - constructor/startup panic contracts (missing `Wave`, missing `Vorma`
      section, unavailable private static FS)
    - loader HTML failure contracts for app-provided head/template failures
      (invalid head-element render path and template execute failure path)
    - parsed route-file structural validation contracts (null path entries,
      missing `originalPattern`, and key/pattern mismatch fail fast)
    - dev route-reload artifact coherence contracts (client-entry deps and CSS
      maps refresh with new stage-one route artifacts)
    - `HEAD` action contracts (fallback-to-GET query parsing with no response
      body)
    - `HEAD` mount semantics for actions (`HEAD` works when `GET` is mounted and
      stays `404` when only non-GET methods are mounted)
    - startup and route-sync behavior (stage files, nested-route decoration)
    - SSR output and serializability validation
    - action input parsing/content-type behavior
    - static middleware contract behavior (`ServeStatic` asset serving and
      passthrough, including root-prefix behavior)
    - dev-only reload guards and endpoint behavior
    - reload/read concurrency coherence
    - helper contracts (`paths`, deps/css ordering, error marker behavior, etc.)
- New correctness fix in this pass:
    - Loader HTML path now handles `GetRootTemplateData` returning `(nil, nil)`
      without panicking.
    - Regression test: `TestLoadersHandler_NilRootTemplateDataMapDoesNotPanic`.
    - Actions input parsing now treats `HEAD` like `GET` for query-param input,
      fixing `HEAD` fallback failures for typed GET action routes.
    - Regression tests:
      `TestHTTPContractMatrix_LoadersAndActions/Actions_HEAD_QueryInputAndHeader_NoBody`
      and `TestInitWithDefaultRouter_ActionsHeadFallsBackToGetWithoutBody`.
    - Loader `HEAD` route behavior is locked with
      `TestInitWithDefaultRouter_LoadersHeadContracts`.
    - Custom action mount root behavior is locked with
      `TestInitWithDefaultRouter_RespectsCustomActionsMountRoot`.
    - `SupportedMethods` casing normalization is locked with
      `TestActionsSupportedMethods_NormalizesConfiguredMethodCasing` and
      `TestInitWithDefaultRouter_SupportedMethodsCasingIsNormalized`.
    - Constructor/startup panic behavior is locked with
      `TestNewVormaApp_RequiresWaveInstance`,
      `TestNewVormaApp_MissingVormaSectionStillTriggersRequiredValidation`, and
      `TestInit_PanicsWhenPrivateFSUnavailable`.
    - Loader HTML failure behavior is locked with
      `TestLoadersHandler_HeadRenderingAndTemplateExecutionFailuresReturn500`.
    - Parsed route-file validation and reload-failure state safety are locked
      with:
      `TestGetBasePathsStageOneOrTwo_ErrorPaths/{NullPathEntry,PathEntryMissingOriginalPattern,PathEntryPatternMismatch}`
      and
      `TestDevReloadRoutesFromDisk_InvalidPathsFileDoesNotMutateRuntimeState`.
    - Dev reload client-entry artifact refresh is locked with
      `TestDevReloadRoutesFromDisk_UpdatesClientEntryDepsAndCSSArtifacts`.
    - `HEAD`/`GET` action mount semantics are locked with
      `TestInitWithDefaultRouter_RespectsSupportedMethods` and
      `TestInitWithDefaultRouter_HeadRequiresMountedGetActionHandler`.

## Verification (Latest Run)

- `go test ./vormaruntime -count=1`
- `go test -race ./vormaruntime -count=1`
- `go test ./vormaruntime -cover -count=1`
- `go test ./kit/mux ./vormaruntime -cover -count=1`
- Latest coverage: `96.9%` statements.

## Benchmark Policy

- Keep all benchmark artifacts under `vormaruntime/benchmark_results/`.
- Store raw `go test -bench` stdout only (no process listings or host diagnostic
  output).
- If host load is clearly unstable/high, skip benchmarking and note deferment
  here.

Clean old-vs-current reference pair:

- `2026-02-11_154353_OLD_pre_refactor_71515bd_clean_raw.txt`
- `2026-02-11_153905_CURRENT_clean_raw.txt`

## Next High-Leverage Work

1. Continue matrix-style HTTP contract tests for externally observable behavior
   with minimal overlap.
2. Expand matrix coverage for remaining rare response-shape branches without
   coupling to internals.
3. Keep this handoff concise and refreshed each batch (remove stale history).
