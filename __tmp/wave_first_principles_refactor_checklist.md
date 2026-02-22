# Wave First-Principles Refactor Checklist

Scope: `/wave/*` and `/wave/tooling/*` without compat layers or shims.

## Immediate

- [x] Rename `/wave/tooling/internal/toolingshared` to
      `/wave/tooling/internal/shared`.
- [x] Rename package identifiers/imports from `toolingshared` to `shared`.
- [x] Re-run compile checks for `/wave/tooling/...` and `/wave/...`.

## Architecture Audit

- [x] Delete vestigial `/wave/internal/pathnorm` package.
- [x] Move pathnorm tests into the owning package and remove wrapper
      indirection.
- [x] Replace `/wave/internal/waveshared` with a first-principles boundary/name
      (`/wave/internal/wavecore`).
- [x] Collapse wavecore functionality into one source file package (200-2000
      LOC).
- [x] Reconfirm public/internal API surface after the boundary rename.

## Wave-Wide End-State

- [x] Enforce one source file per package (200-2000 LOC) across `/wave/*`.
      Current state: every package under `/wave/*` is now one source file
      between 200 and 2000 LOC.
- [x] Remove unused exported API surface not required by in-repo production
      consumers. Completed: de-exported all prior zero non-test export
      candidates, including `ViteConfig`, `Timing`, `StaticAssetDirs`, and
      `CSSEntryFiles`.
- [x] Keep zero compat adapters and zero back-compat shims.

## Execution Mode

- [x] Execute directly in `/wave` (recommended; no `wave_2` branch/package
      tree).
