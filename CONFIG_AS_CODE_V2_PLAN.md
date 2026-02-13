# Config-As-Code V2 Plan (Wave + Vorma)

- Last updated: 2026-02-12
- Status: in progress (hard-cut implementation complete; cleanup remains)
- Scope: framework-wide, intentionally breaking

Parent roadmap: `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.

## Objective

Move Wave/Vorma to a single config-as-code model while keeping full user freedom
and deterministic dev reload behavior.

Target shape:

- config authored as Go via `wavecfg.Document`
- config loaded through `wave.ConfigSource`
- dependency-aware reload from explicit config dependencies
- no authored `wave.config.json` requirement

## UX Guardrails (Locked)

- Config-as-code must not increase per-route app-builder ceremony.
- Route authoring ergonomics are governed by the framework UX contract in
  `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.
- Deterministic codegen output is solved in codegen/tooling; it must not force
  extra route registration boilerplate.
- This track is bound by the same hard non-regression baseline anchored in
  commit `4dc9191a` for simple app-builder UX.

Config-specific non-regression constraints:

- Config changes must not force extra route authoring files/steps.
- Config changes must not create multiple competing "default" ways to configure
  the same app concern.
- Framework robustness improvements must be internalized behind clear defaults,
  not pushed into user boilerplate.

## Completed Work

- Added provider protocol + parser + normalization.
- Added provider subprocess runner with timeout/stdout bounds.
- Added `wave.ConfigSource` abstraction and integrated it into `wave.New`.
- Added resolved config dependency/fingerprint plumbing through runtime/tooling.
- Migrated devserver reload/watch flow to resolved config source + dependency
  graph.
- Added/updated integration tests for source loading, reload, fallback, and
  dependency-triggered restart behavior.

### Hard-Cut Removals Already Landed

- Removed JSON-first runtime API surface:
- `wave.Config.WaveConfigJSON`
- `wave.CoreConfig.ConfigLocation`
- `wave.ParseConfigFile`
- `wave.RuntimeFramework.GetConfigFile`
- `wave.JSONConfigSource`
- `wave.ConfigSource` is now the config loading path.

### Typed Config Authoring Simplified Further

- `wavecfg.Document` is now the single authored config object.
- Reload dependencies are declared directly on the document via:
- `Document.ConfigDependencies`
- Removed `wavecfg.AppConfig` wrapper to reduce boilerplate.
- `wavecfg` provides direct source constructors:
- `NewGoRunProviderConfigSource(...)` for development
- `NewStaticConfigSourceFromDocument(...)` for static/document-backed production
  config
- App-facing Wave initialization is now one line with no dev/prod split files:
- `wavecfg.NewWave(config.WaveConfig)`
- `wavecfg.NewWaveOptions` removed from app-facing usage.
- `wavecfg.Document` now exposes direct methods for config/payload emission:
- `MarshalConfigJSON` / `MustMarshalConfigJSON`
- `BuildProviderPayloadJSON` / `MustBuildProviderPayloadJSON`
- `EmitProviderPayloadToStdout`
- `wavecfg.Document` custom root extension map is now named `Custom`.
- `wavecfg` provider/config-path helpers now require explicit relative
  package-path inputs.

### Bootstrap + Reference App

- Bootstrap templates now scaffold config-as-code by default (`backend/config`).
- Reference app (`internal/site`) migrated to `wavecfg.Document`.
- Authored `wave.config.json` removed from reference app.

### Vercel Runtime Path Updated

- Vercel flow no longer consumes authored `backend/wave.config.json`.
- Build emits provider payload artifact (`backend/dist/config.provider.json`).
- Proxy templates/readers consume provider payload artifact.

## Verification Completed

- `go test ./...`
- `make tstest`
- `make tscheck`
- `go test ./backend/...` (inside `internal/site`)
- `make vercel-build` (inside `internal/site`)
- `pnpm -C internal/site exec tsc --noEmit`
- `pnpm -C internal/site exec vite build`

## Remaining Before Config Track Can Be Marked Closed

- Final docs sweep for outdated JSON-era wording outside historical/plan docs.
- Bootstrap smoke test run for all deployment modes (`none`, `vercel`, `docker`)
  to confirm generated apps use the new config model cleanly.
- Keep route-authoring UX checks wired as ongoing conformance tests.

## Key Design Decisions (Locked)

- Keep provider subprocess execution for dev reload path.
- Keep dependency declaration explicit (files/globs/env) and co-located on
  `wavecfg.Document`.
- Keep one obvious high-level path (document-first) while preserving low-level
  control through explicit dependency fields and `wave.ConfigSource`.
