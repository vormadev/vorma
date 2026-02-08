# Vorma Release and Distribution Specification

Status: Draft  
Last Updated: 2026-02-08  
Applies To: Vorma release process and distributed artifacts (Go module, npm package, create CLI package)

## 1. Why This Spec Exists

This document defines what “a valid Vorma release” means at artifact and
process level.

It exists to:

- prevent accidental breaking changes in published surfaces,
- keep Go and npm distribution channels consistent,
- make release readiness testable and repeatable.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Distribution Units

Vorma release units are:

- Go module: `github.com/vormadev/vorma`
- npm framework package: `vorma`
- npm scaffolding package: `create-vorma`

### 2.3 Out of Scope

Out of scope:

- downstream app deployment release policy,
- non-official mirrors/forks,
- package-manager-specific CDN behavior.

## 3. Requirement Catalog

## 3.1 Versioning and Compatibility

### REL-VERS-001: Semver-Style Versioning

Vorma release identifiers MUST follow semantic versioning conventions across
Go tags and npm package versions.

### REL-VERS-002: Pre-Release Channel Semantics

Given a pre-release version (for example containing `-pre`)  
When published to npm  
Then publish channel MUST use npm `pre` tag rather than default latest tag.

### REL-VERS-003: Final Release Channel Semantics

Given a non-pre-release version  
When published to npm  
Then publish SHOULD target default latest channel.

### REL-VERS-004: Root/Create Version Alignment

Given root npm package version changes  
When release bump workflow runs  
Then `package.json` and `internal/framework/_typescript/create/package.json`
MUST be updated to matching version values.

### REL-VERS-005: Go Tag Format

Go release tags MUST use `v`-prefixed semver format (for example `v0.85.0`).

### REL-VERS-006: Runtime Compatibility Floors

Compatibility floors for official tooling/releases MUST preserve at least:

- Go >= 1.24 (module/tooling baseline),
- Node >= 22.11 for `create-vorma` package engine contract.

## 3.2 Go Module Distribution Contracts

### REL-GO-001: Module Path Stability

Published Go module path MUST remain `github.com/vormadev/vorma` unless
accompanied by explicit migration policy.

### REL-GO-002: Public Entry Surface

Top-level Go package `vorma` MUST continue exposing documented public API
entrypoints (aliases/re-exports and constructors) used by downstream apps.

### REL-GO-003: Tag Publication Requirement

A Go release MUST publish git tag to origin so module consumers can resolve
versioned modules.

### REL-GO-004: Go Proxy Propagation Step

Release workflow SHOULD trigger module proxy refresh step after tag publication.

### REL-GO-005: Embedded npm Version Signal

Given npm package version changes  
When Go module is built from matching source revision  
Then embedded package.json version accessor (`Internal__GetCurrentNPMVersion`)
SHOULD reflect the same root package version.

## 3.3 npm `vorma` Package Distribution Contracts

### REL-NPM-001: Package Identity

Primary npm package identity MUST remain `vorma`.

### REL-NPM-002: Exported Subpath Contract

The following subpath exports are release-contract surfaces and MUST remain
intentionally managed:

- `vorma/client`
- `vorma/react`
- `vorma/solid`
- `vorma/preact`
- `vorma/vite`
- `vorma/kit/converters`
- `vorma/kit/cookies`
- `vorma/kit/csrf`
- `vorma/kit/debounce`
- `vorma/kit/fmt`
- `vorma/kit/json`
- `vorma/kit/listeners`
- `vorma/kit/matcher/register`
- `vorma/kit/matcher/find-best`
- `vorma/kit/matcher/find-nested`
- `vorma/kit/theme`
- `vorma/kit/url`

Changes to these exports MUST be treated as compatibility-sensitive.

### REL-NPM-003: Import+Types Pairing

Each exported npm subpath MUST provide both import target (`.js`) and type
definition target (`.d.ts`) entries.

### REL-NPM-004: Publish File Set Contract

Published `vorma` package MUST include intended release file set from
`files` allowlist, including `npm_dist` and required docs/license metadata.

### REL-NPM-005: Test/Bench Artifact Exclusion

Published npm artifacts MUST NOT include test/bench implementation files in
`npm_dist` output.

### REL-NPM-006: Side-Effects Metadata Stability

`sideEffects` metadata SHOULD remain intentionally maintained (currently `false`)
to preserve tree-shaking expectations.

### REL-NPM-007: Build Artifact Root

Framework npm JS/type artifacts MUST be generated under `npm_dist/` according
to the documented build pipeline.

### REL-NPM-008: Module-System Metadata Contract

Root npm manifest MUST keep package `type` metadata explicitly set to
`"module"` so published subpath exports continue to resolve under ESM package
semantics.

## 3.4 npm `create-vorma` Package Contracts

### REL-CREATE-001: Package Identity and Bin Entry

Scaffolding package identity MUST remain `create-vorma` with executable bin
entry `create-vorma` mapped to `./dist/main.js`.

### REL-CREATE-002: Publish Contents

`create-vorma` package publish contents MUST be limited to intended runtime
artifact directory (`dist`).

### REL-CREATE-003: Version Alignment With Root Package

`create-vorma` package version MUST track the same release version as root
`vorma` npm package.

### REL-CREATE-004: Engine Contract

`create-vorma` package MUST preserve explicit Node engine floor contract
(currently `>=22.11.0`) unless migration policy is documented.

### REL-CREATE-005: Scaffold Compatibility Guards

Scaffolding runtime MUST validate local Go and Node version prerequisites before
generating project files.

Guard behavior:

- Go runtime MUST be discoverable via `go version` and MUST satisfy minimum
  supported floor (currently Go `1.24+`),
- Node runtime MUST satisfy create package engine floor (currently
  `>=22.11.0`),
- guard failures MUST terminate scaffolding with non-zero exit before bootstrap
  generation/execution.

### REL-CREATE-006: Interactive Cancellation Contract

If user cancels at any prompt stage, CLI MUST terminate as a cancelled
invocation (successful cancellation exit code) and MUST NOT proceed with
remaining scaffold/bootstrap steps.

### REL-CREATE-007: Target Directory Prompt/Validation Contract

Scaffolding prompt flow MUST support optional creation of a new directory before
module/project setup.

When new-directory mode is selected:

- directory name MUST be required and validated to `[a-zA-Z0-9-_]+`,
- existing target path collision MUST be rejected,
- directory MUST be created and process working directory switched to it before
  module discovery and option prompts continue.

### REL-CREATE-008: Parent Go Module Discovery and Choice Contract

CLI MUST search upward from current working directory for nearest `go.mod`.

If parent module exists, user choice contract MUST include:

- create a new module in current directory, or
- use parent module.

If no module is found, scaffolding MUST require new module initialization.

### REL-CREATE-009: New Module Initialization Contract

When new module flow is selected/required:

- module name prompt MUST require non-empty value,
- CLI MUST run `go mod init <module-name>` in target module root,
- in local-test mode, module replace mapping for
  `github.com/vormadev/vorma` MUST be applied to local repository path.

Initialization failures MUST terminate scaffolding with non-zero exit.

### REL-CREATE-010: Underscore Path Guard Contract

Scaffolding MUST reject target paths where any directory segment between module
root and current working directory starts with underscore (`_`), because such
directories are ignored by Go package loading.

### REL-CREATE-011: Go Import Base Derivation Contract

Generated `GoImportBase` MUST be derived as:

- module name when current directory equals module root,
- module name joined with module-root-relative directory segments (POSIX path
  separators) when scaffolding in nested path.

### REL-CREATE-012: Scaffold Option Prompt Surface Contract

CLI option prompts MUST expose and persist selected values for:

- `UIVariant`: `react` | `preact` | `solid`,
- JS package manager: `npm` | `pnpm` | `yarn` | `bun`,
- deployment target: `docker` | `vercel` | `none`,
- Tailwind inclusion boolean (default `false`).

### REL-CREATE-013: Bootstrap Program Generation Contract

Scaffolding MUST generate a temporary Go bootstrap program invoking
`bootstrap.Init(bootstrap.Options{...})` with resolved options including:

- `GoImportBase`, `UIVariant`, `JSPackageManager`, `DeploymentTarget`,
  `IncludeTailwind`,
- runtime metadata (`NodeMajorVersion`, `GoVersion`),
- directory/module context (`ModuleRoot`, `CurrentDir`, `HasParentModule`),
- `CreatedInDir` only when new-directory mode was used.

### REL-CREATE-014: Version-Pinned Vorma Dependency Install Contract

Before bootstrap run, CLI MUST resolve `create-vorma` package version and
install matching Vorma Go dependency via:
`go get github.com/vormadev/vorma@v<create-package-version>`.

Explicit skip case:

- local-test mode with existing parent-module flow MAY skip `go get`.

### REL-CREATE-015: Bootstrap Execution and Temp Cleanup Contract

CLI MUST run generated bootstrap program via `go run <temp-main.go>` in target
working directory and MUST stream bootstrap output to terminal.

Temporary bootstrap directory/file cleanup MUST run in `finally` semantics,
using recursive/force removal, and cleanup failure MUST NOT mask primary
scaffold outcome.

### REL-CREATE-016: Bootstrap Option Derivation Defaults and Mapping Contract

Bootstrap option-derivation behavior MUST preserve:

- bootstrap entry precondition:
  `GoImportBase` MUST be non-empty at `Init` entry (missing value fails fast),
- default `UIVariant=react` when omitted,
- default `JSPackageManager=npm` when omitted,
- JS package-manager base command mapping:
  `npm->npx`, `pnpm->pnpm`, `yarn->yarn`, `bun->bunx`,
- UI-variant TypeScript/adapter mapping:
  `react` (`jsx=react-jsx`, import source `react`),
  `preact` (`jsx=react-jsx`, import source `preact`),
  `solid` (`jsx=preserve`, import source `solid-js`),
- template-style helper mapping:
  default `backgroundColor` key for non-Solid,
  Solid override key `"background-color"`,
  Solid call-suffix helper value `()`,
- Docker Go-version derivation:
  use provided `GoVersion` when present, else `runtime.Version()`,
  strip leading `go` prefix and trim to major/minor (`go1.24.0 -> 1.24`),
- UI-plugin resolution mapping:
  `react -> @vitejs/plugin-react-swc`,
  `solid -> vite-plugin-solid`,
  `preact -> @preact/preset-vite`,
  and empty/unknown UI variant MUST fail fast,
- deployment target validation gate allowing only `docker`, `vercel`, or
  `none` (unknown targets MUST fail fast).

### REL-CREATE-017: Deployment-Target Scaffold Wiring Contract

When deployment target is `vercel`, scaffold output MUST include:

- deployment files (`vercel.json`, `api/proxy.ts`),
- package-script extras for Vercel install/build flow,
- Vercel runtime dependency (`@vercel/node`).

When deployment target is `docker`, scaffold output MUST include:

- `Dockerfile`,
- package-script extras for docker build/run,
- package-manager-specific lockfile/install-command mapping (`npm`, `pnpm`,
  `yarn`, `bun`),
- monorepo-aware Docker context/workdir/binary-path derivation when scaffold is
  nested under parent module root.

Docker and monorepo wiring MUST additionally preserve:

- monorepo mode gates:
  enabled only when `HasParentModule=true` and both `ModuleRoot` and
  `CurrentDir` are non-empty,
- normalized slash-form path derivation for:
  module-root-relative app path and reverse Docker build-context path,
- docker-build script shape:
  default `docker build -t vorma-app .`,
  monorepo override `docker build -f Dockerfile -t vorma-app <context-path>`,
- package-manager install prerequisites in Docker template:
  pnpm/yarn/bun global install lines injected; npm injects none,
- Docker workdir/binary path mapping:
  monorepo includes app-subpath workdir and binary path;
  non-monorepo uses `/app/backend/dist/main`.

### REL-CREATE-018: Generated File and Dependency Matrix Contract

Bootstrap directory creation MUST include:

- `backend/assets`,
- `frontend/assets`,
- `backend/src/router`,
- `backend/cmd/serve`,
- `backend/cmd/build`,
- `backend/dist/static/internal`,
- `frontend/src/components`,
- `frontend/src/styles`,
- `api` when deployment target is `vercel`.

Bootstrap file emission MUST include all core scaffold targets:

- `backend/cmd/serve/main.go`,
- `backend/cmd/build/main.go`,
- `backend/dist/static/.keep`,
- `backend/assets/entry.go.html`,
- `backend/src/router/router.go`,
- `backend/wave.dev.go`,
- `backend/wave.prod.go`,
- `backend/wave.config.json`,
- `vite.config.ts`,
- `package.json`,
- `.gitignore`,
- `frontend/src/styles/main.css`,
- `frontend/src/styles/main.critical.css`,
- `frontend/src/vorma.routes.ts`,
- `frontend/src/components/root.tsx`,
- `frontend/src/components/home.tsx`,
- `frontend/src/components/links.tsx`,
- `frontend/src/vorma.utils.tsx`,
- `frontend/src/vorma.api.ts`,
- `frontend/vite.d.ts`,
- `tsconfig.json`,
- `frontend/assets/favicon.svg`,
- `vercel.json` + `api/proxy.ts` when deployment target is `vercel`,
- `Dockerfile` when deployment target is `docker`.

`tsconfig.json` emission MUST occur after earlier template writes in the
bootstrap file-write phase.

Bootstrap MUST select exactly one UI-variant entry template:

- `frontend/src/vorma.entry.tsx` from React template when UI variant is `react`,
- `frontend/src/vorma.entry.tsx` from Preact template when UI variant is
  `preact`,
- `frontend/src/vorma.entry.tsx` from Solid template when UI variant is
  `solid`.

Common dependency install baseline MUST include `typescript`, `vite`, current
`vorma` npm package version, and selected UI Vite plugin.

UI variant dependency matrix MUST preserve:

- React: `react`, `react-dom`, `@types/react`, `@types/react-dom`,
- Preact: `preact`, `@preact/signals`,
- Solid: `solid-js`.

Deployment target dependency matrix MUST preserve:

- Vercel target adds `@vercel/node`.

Tailwind opt-in contract:

- given Tailwind option enabled, bootstrap MUST emit Tailwind dependency/file
  wiring (`@tailwindcss/vite`, `tailwindcss`,
  `frontend/src/styles/tailwind.css`),
- given Tailwind option disabled, bootstrap MUST NOT emit those additions.

### REL-CREATE-019: Post-Bootstrap Command Sequence Contract

After file/dependency scaffold steps complete, bootstrap MUST execute:

1. `go mod tidy`,
2. `go run ./backend/cmd/build --no-binary`.

Either failure MUST terminate scaffold outcome as failed.

On success, bootstrap terminal guidance MUST preserve run-command format:

- created-in-dir mode: `cd <CreatedInDir> && <run-prefix> dev`,
- in-place mode: `<run-prefix> dev`.

### REL-CREATE-020: Package-Manager Command Helper Contract

Bootstrap helper command mapping MUST preserve:

- run-script prefix mapping:
  `npm -> "npm run"`, non-npm -> package-manager binary name,
- install command mapping:
  `npm i`, `pnpm i`, `yarn`, `bun i`,
- dev-dependency install command shape in package install helper:
  `npm i -D`, `pnpm add -D`, `yarn add -D`, `bun add -d`.

Unknown/unsupported package manager values in bootstrap helper resolution MUST
fail fast rather than silently fallback.

### REL-CREATE-021: Template and Asset Write Fail-Fast Contract

Bootstrap template and embedded-asset write helpers MUST preserve:

- template-source lookup from embedded template FS (`tmpls`),
- asset-source lookup from embedded asset FS (`assets`),
- template parse + execute against derived bootstrap options before write,
- write permission mode `0644` for scaffold outputs,
- fail-fast error propagation for read/parse/execute/write failures.

## 3.5 Build and Publish Pipeline Contracts

### REL-PIPE-001: Release Prep Gate

npm release prep gate MUST include TypeScript/test/lint/typecheck steps via
release prep workflow (`make tsprepforpub` or equivalent).

### REL-PIPE-002: npm Dist Build Pipeline

npm release pipeline MUST rebuild distribution artifacts from source
(`make npmbuild` or equivalent) before publish.

### REL-PIPE-003: Dist Rebuild Cleanliness

Distribution build pipeline MUST clear previous dist output before generating
fresh artifacts.

### REL-PIPE-004: Warning Intolerance in Distribution Build

Distribution build pipeline SHOULD treat bundler warnings as release-blocking
until reviewed/resolved.

### REL-PIPE-005: Publish Sequence

Official publish sequence SHOULD preserve order:

1. version bump and prep,
2. npm publish (`vorma` and `create-vorma`),
3. source push,
4. Go tag/release propagation.

### REL-PIPE-006: Release Instructions Alignment

Human release instructions and automated scripts SHOULD remain aligned; if one
changes, the other SHOULD be updated in same change set.

## 3.6 Artifact Integrity and Validation

### REL-ART-001: Export Target Existence Check

For each declared npm export, release validation SHOULD verify referenced
import/type files exist in publish artifact tree.

### REL-ART-002: Type Declaration Presence

Published TS-facing APIs MUST include corresponding declaration files.

### REL-ART-003: No Stale Export Targets

Release validation SHOULD fail if package exports reference missing or stale
artifact paths.

### REL-ART-004: Binary/CLI Entrypoint Check

Release validation SHOULD verify `create-vorma` bin target exists and is
executable in package context.

### REL-ART-005: Version Consistency Check

Release validation SHOULD verify version consistency across:

- root `package.json`,
- create package `package.json`,
- any embedded/version-reporting helpers.

## 3.7 Compatibility and Change Policy

### REL-COMPAT-001: Export Surface Changes Are Breaking-Sensitive

Removing or renaming exported npm subpaths or top-level Go API symbols MUST be
treated as breaking-sensitive and documented explicitly.

### REL-COMPAT-002: Pre-1.0 Breaking Change Disclosure

Given project is pre-1.0 and breaking changes occur  
When releasing  
Then release notes SHOULD explicitly disclose impacted surfaces and migration
expectations.

### REL-COMPAT-003: Deprecation-First Preference

Where practical, release policy SHOULD prefer deprecation path over immediate
surface removal.

### REL-COMPAT-004: UI Variant Parity in Release Artifacts

Release artifacts for supported UI variants (React, Preact, Solid) MUST remain
simultaneously published and resolvable through package exports.

## 4. Release Validation Guidance

Release validation checklist SHOULD include:

1. run prep gate and verify clean pass,
2. rebuild npm dist and verify no stale/missing export targets,
3. verify root/create version alignment,
4. dry-check npm publish contents for both packages,
5. verify Go tag format and module resolution path,
6. verify post-publish install/import smoke tests for key subpaths.

## 5. Executable Conformance Scenario Catalog

### RDC-VERS-001 (covers REL-VERS-001, REL-VERS-004, REL-VERS-006)

Given root and create package manifests plus `go.mod`  
When release version and runtime floor contracts are validated  
Then semver shape, root/create version alignment, Go floor, and create package
Node engine floor MUST hold.

### RDC-VERS-002 (covers REL-VERS-002, REL-VERS-003)

Given npm publish Makefile targets  
When release channel mapping contract is validated  
Then pre-release publish path MUST use npm `pre` tag and final-release publish
path MUST target default/latest channel.

### RDC-VERS-003 (covers REL-VERS-005)

Given Go tag bump/publish script path  
When Go release tag format contract is validated  
Then tag creation flow MUST construct tags using `v`-prefixed version form.

### RDC-GO-001 (covers REL-GO-001)

Given repository module declaration  
When module path contract is validated  
Then module path MUST remain `github.com/vormadev/vorma`.

### RDC-GO-002 (covers REL-GO-005, REL-ART-005)

Given embedded package.json version accessor and package manifests  
When version consistency contract is validated  
Then root package version, create package version, and
`Internal__GetCurrentNPMVersion` value MUST match.

### RDC-GO-003 (covers REL-GO-002)

Given top-level `vorma` package source  
When public Go surface contract is validated  
Then documented constructors, re-exported variables, and public type aliases
MUST be present.

### RDC-GO-004 (covers REL-GO-003, REL-GO-004)

Given Go release bumper flow  
When tag publication and proxy propagation contract is validated  
Then flow MUST include git tag creation, tag push to origin, and go proxy
refresh command path.

### RDC-NPM-001 (covers REL-NPM-001, REL-COMPAT-004)

Given root npm manifest identity and exported UI variant subpaths  
When package identity/parity contract is validated  
Then package name MUST be `vorma` and React/Preact/Solid exports MUST all be
present.

### RDC-NPM-002 (covers REL-NPM-002, REL-NPM-003, REL-ART-002)

Given root npm export map  
When export surface contract is validated  
Then required subpaths MUST be present and each export MUST provide both
`import` and `types` entries.

### RDC-NPM-003 (covers REL-NPM-004, REL-NPM-005, REL-NPM-007, REL-ART-001, REL-ART-003)

Given root npm files allowlist and export targets  
When publish artifact contract is validated  
Then `npm_dist` MUST be included, test/bench artifacts MUST be excluded by
pattern, and exported import/type targets MUST resolve under `./npm_dist/`.

### RDC-NPM-004 (covers REL-NPM-006, REL-NPM-008)

Given root npm side-effects and module-system metadata  
When tree-shaking contract is validated  
Then `sideEffects` MUST remain explicit and `false`, and package `type` MUST be
`"module"`.

### RDC-CREATE-001 (covers REL-CREATE-001, REL-CREATE-002, REL-CREATE-003, REL-CREATE-004, REL-ART-004)

Given `create-vorma` package manifest  
When create package distribution contract is validated  
Then package name, bin entry, files allowlist, version alignment, and Node
engine floor MUST satisfy release contract.

### RDC-CREATE-002 (covers REL-CREATE-005)

Given create CLI bootstrap script  
When scaffolding prerequisite guard contract is validated  
Then script MUST enforce Go/Node prerequisite checks and fail before bootstrap
execution when prerequisites are not met.

### RDC-CREATE-003 (covers REL-CREATE-006, REL-CREATE-007, REL-CREATE-008, REL-CREATE-010, REL-CREATE-011, REL-CREATE-012)

Given create CLI source prompt/discovery flow  
When interactive workflow contracts are validated  
Then cancellation behavior, target-directory validation/creation, parent-module
discovery and selection, underscore-path rejection, import-base derivation, and
option prompt surfaces MUST match specified contract.

### RDC-CREATE-004 (covers REL-CREATE-009, REL-CREATE-013, REL-CREATE-014, REL-CREATE-015)

Given create CLI module/bootstrap implementation  
When module-init/bootstrap execution contracts are validated  
Then new-module init + local-test replace path, bootstrap program option shape,
version-pinned `go get` behavior (including documented skip case), and temp
artifact cleanup semantics MUST match contract.

### RDC-CREATE-005 (covers REL-CREATE-016, REL-CREATE-017, REL-CREATE-020)

Given bootstrap options and package-manager/deployment-target matrices  
When bootstrap option-derivation and helper-command contracts are validated  
Then defaults, UI/package/deployment mapping, Docker monorepo path derivation,
and package-manager helper command shapes/fail-fast guards MUST match contract.

### RDC-CREATE-006 (covers REL-CREATE-018, REL-CREATE-021)

Given bootstrap scaffold implementation and embedded template/assets  
When scaffold artifact and file-write contracts are validated  
Then directory/file/dependency matrices, deployment/variant/tailwind conditionals,
and template/asset write fail-fast semantics (including `0644` mode) MUST match
contract.

### RDC-CREATE-007 (covers REL-CREATE-019)

Given bootstrap post-write execution flow  
When command-sequencing contract is validated  
Then `go mod tidy` followed by `go run ./backend/cmd/build --no-binary` MUST
run in order, failures MUST terminate scaffold success, and success terminal
run-command guidance MUST match contract.

### RDC-PIPE-001 (covers REL-PIPE-001, REL-PIPE-002)

Given Makefile release targets  
When release prep/build pipeline contract is validated  
Then prep gate target MUST chain reset/test/lint/typecheck and npm dist build
target MUST run `internal/scripts/buildts`.

### RDC-PIPE-002 (covers REL-PIPE-003, REL-PIPE-004)

Given buildts implementation and release pipeline  
When dist cleanliness and warning intolerance are validated  
Then npm dist output MUST be reset before rebuild and bundler warnings MUST be
treated as failures.

### RDC-PIPE-003 (covers REL-PIPE-005, REL-PIPE-006)

Given release helper scripts and Makefile targets  
When release sequence/alignment contract is validated  
Then scripts SHOULD expose version-bump/prep/build/publish/tag building blocks
in intended phase order and keep command references aligned with canonical
targets.

### RDC-COMPAT-001 (covers REL-COMPAT-001, REL-COMPAT-002, REL-COMPAT-003)

Given compatibility policy artifacts (`README`, compatibility spec, release
spec)  
When compatibility-policy contract is validated  
Then breaking-sensitive export policy, pre-1.0 disclosure, and
deprecation-first guidance MUST remain explicitly documented.

## 6. Relation to Other Specs

- Build/dev contracts: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Security model: `/Users/sjc/__code/river/specs/VORMA_SECURITY_MODEL_SPEC.md`
- Testing strategy: `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Performance model: `/Users/sjc/__code/river/specs/VORMA_PERFORMANCE_MODEL_SPEC.md`
- Roadmap checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
