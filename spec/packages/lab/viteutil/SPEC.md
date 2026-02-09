# lab/viteutil Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `lab/viteutil`

## Scope

Package-owned Vite build/dev orchestration and manifest/dev-script helper
contracts in `lab/viteutil/**`.

Current evidence note:

- Requirements are mined from `lab/viteutil/cmd.go` and
  `lab/viteutil/viteutil.go`.
- No active legacy tests outside `conformance/**` were found for this package.

## Requirements

- `LAB-VITEUTIL-001` Build-context shape contract. `BuildCtx` MUST hold `mu`,
  `cmd`, `opts`, and `port`, and expose `GetPort()`.
- `LAB-VITEUTIL-002` Build-context initialization contract. `NewBuildCtx(opts)`
  MUST default `port` to `5173` when `opts.DefaultPort==0`.
- `LAB-VITEUTIL-003` Command preparation contract. `prep_cmd` MUST split
  `JSPackageManagerBaseCmd` via `strings.Fields`, create
  `exec.Command(first, rest...)`, wire stdout/stderr, and apply optional
  `JSPackageManagerCmdDir` as command working directory.
- `LAB-VITEUTIL-004` Dev lifecycle contract. `DevBuild` MUST lock, terminate an
  existing tracked process if present, prepare a fresh command, and update
  `BuildCtx.port` from `InitPort`.
- `LAB-VITEUTIL-005` Dev-command argument contract. `DevBuild` MUST append
  `vite --port <port> --clearScreen false --strictPort true` and include
  `--config <ViteConfigFile>` when configured.
- `LAB-VITEUTIL-006` Dev-start error handling contract. When `cmd.Start()`
  fails, `DevBuild` currently logs the failure and still returns `nil`.
- `LAB-VITEUTIL-007` Wait/Cleanup guard contract. `Wait` and `Cleanup` MUST only
  act when a command/process exists and `ProcessState==nil`; `Cleanup` uses
  `grace.TerminateProcess`.
- `LAB-VITEUTIL-008` Prod build command contract. `ProdBuild` MUST run
  `vite build` with out-dir/assets-dir/manifest temp args and optional
  `--config`, under write lock.
- `LAB-VITEUTIL-009` Prod environment contract. `ProdBuild` MUST set
  `ROLLDOWN_OPTIONS_VALIDATION=loose` before execution.
- `LAB-VITEUTIL-010` Manifest relocation contract. After successful prod build,
  `ProdBuild` MUST rename `./<OutDir>/__temp_viteutil_manifest__.json` to
  `ManifestOut` and return rename errors.
- `LAB-VITEUTIL-011` Manifest read contract. `ReadManifest(path)` MUST read file
  contents and JSON-unmarshal into `Manifest`.
- `LAB-VITEUTIL-012` Manifest dependency walk contract. `FindAllDependencies`
  MUST recursively traverse `Imports`, dedupe by seen key, return basename chunk
  files, and ensure entry chunk basename inclusion.
- `LAB-VITEUTIL-013` Entry lookup contract. `FindRelativeEntrypointPath` MUST
  return the manifest key whose chunk is `IsEntry` and basename-matches the
  requested entry file; otherwise return `entrypoint not found`.
- `LAB-VITEUTIL-014` Dev script rendering contract. `ToDevScripts` MUST
  optionally inject the React refresh preamble for `Variant==react`, then render
  module scripts for `@vite/client` and the client entry path
  (`stripPrecedingSlash` applied).
- `LAB-VITEUTIL-015` Port env contract. `PortEnvName` is `__VITE_PORT`;
  `InitPort` MUST choose a free port via `netutil.GetFreePort(5199)`, set
  `__VITE_PORT`, and return the chosen port; `GetVitePortStr` MUST return
  current env value.
