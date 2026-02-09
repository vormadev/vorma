# vormaclient/create Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `vormaclient/create`

## Scope

Package-owned CLI scaffolding contracts in `vormaclient/create/**`.

This package owns interactive app-scaffolding behavior in `main.ts` and package
metadata/distribution contracts in package-local config files.

Current evidence note:

- Requirements are currently source-backed in `vormaclient/create/**`.
- No active legacy tests outside `conformance/**` were found for this package in
  the repo.

## Requirements

- `VORMACLIENT-CREATE-001` Runtime prerequisite gate contract.
  CLI MUST fail fast when Go is missing or detected below 1.24, and when Node is
  below major 22; recognized failure paths MUST exit non-zero with explanatory
  cancellation/error messaging.
- `VORMACLIENT-CREATE-002` Prompt cancellation contract.
  Every interactive prompt decision point MUST treat user cancellation as a
  graceful cancel path (`cancel(...)`) and terminate without partial scaffold
  continuation.
- `VORMACLIENT-CREATE-003` Target-directory creation/validation contract.
  CLI MUST support optional new-directory creation; directory names MUST be
  non-empty, restricted to alnum/hyphen/underscore, and rejected when target
  already exists.
- `VORMACLIENT-CREATE-004` Go-module discovery/mode-selection contract.
  CLI MUST search upward for parent `go.mod`; when found, user MUST choose
  between creating a new module or using parent module; when absent, new-module
  creation path MUST be selected.
- `VORMACLIENT-CREATE-005` New-module initialization contract.
  New-module path MUST prompt for module name, run `go mod init`, and when
  `--local-test` is used in this path MUST add local replace directive for
  `github.com/vormadev/vorma`.
- `VORMACLIENT-CREATE-006` Underscore-directory guard contract.
  CLI MUST reject scaffolding when any path segment from module root to working
  directory starts with `_`.
- `VORMACLIENT-CREATE-007` Go import-base derivation contract.
  CLI MUST derive bootstrap import base from module name and relative subpath
  when current directory is nested under module root.
- `VORMACLIENT-CREATE-008` Scaffold option collection contract.
  CLI MUST collect UI variant (`react`/`preact`/`solid`), JS package manager
  (`npm`/`pnpm`/`yarn`/`bun`), deployment target (`docker`/`vercel`/`none`), and
  Tailwind include flag.
- `VORMACLIENT-CREATE-009` Bootstrap program generation contract.
  CLI MUST generate temporary Go bootstrap program invoking `bootstrap.Init` with
  collected scaffold options plus environment-derived fields
  (`NodeMajorVersion`, `GoVersion`, module/root/current-dir context,
  parent-module flag, and optional created-directory name).
- `VORMACLIENT-CREATE-010` Vorma dependency acquisition contract.
  CLI MUST read local package version from package metadata and run
  `go get github.com/vormadev/vorma@v<version>` before bootstrap, except when
  `--local-test` with existing parent module path explicitly skips this step.
- `VORMACLIENT-CREATE-011` Bootstrap execution and failure/cleanup contract.
  CLI MUST run generated bootstrap program via `go run`, propagate failures as
  scaffold failure, and always attempt temporary-directory cleanup.
- `VORMACLIENT-CREATE-012` Distribution/config contract.
  Package metadata/config MUST define `create-vorma` bin target at
  `dist/main.js`, Node engine floor `>=22.11.0`, TypeScript base config
  extension, and distribution ignore rule for generated `dist/` artifact source
  directory.
