# VormaClient Refactor Cleanup Checklist

This checklist replaces the earlier decision matrix and is the handoff source of
truth for cleanup.

## Final Decisions

- Query input contract: enforce object-root query inputs
  (`object | null | undefined`) with both type-level and runtime guards.
- `beginNavigation` immediate-abort path: keep return shape stable, but return a
  truthfully aborted controller.
- npm package boundary: strict allowlist for intentional public artifacts.
- Build output hygiene: keep incremental caching, but force clean rebuilds when
  cache/schema changes and remove stale internal artifacts from outputs.
- Legacy navigation alias cleanup: remove `slots` alias surface and keep `lanes`
  naming only.
- Micro-seam reconsolidation: not required for this pass; current split files
  stay.

## Checklist

### 1) Query Input Contract (`api.query`)

- [x] Restrict query input typing so non-object root inputs are rejected at
      compile time.
- [x] Add runtime guard in URL builder to throw on non-object query input roots.
- [x] Ensure falsey primitives (`0`, `false`, `""`) are not silently dropped.
- [x] Add/adjust unit tests to cover invalid query inputs and expected throw
      behavior.

Validation:

- [x] `pnpm vitest vormaclient/client/src/tests/unit/app_helpers.test.ts`

### 2) Immediate-Abort Navigation Control Semantics

- [x] Update immediate-abort control factory to return an already-aborted
      signal.
- [x] Keep API signature unchanged (`NavigationControl` return type).
- [x] Add regression coverage that asserts
      `control.abortController.signal.aborted === true` for the immediate-abort
      path.

Validation:

- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`

### 3) npm Publish Boundary

- [x] Replace broad `files` includes with strict allowlist in root
      `package.json`.
- [x] Confirm internal source/test/docs files are no longer included in tarball
      output.

Validation:

- [x] `npm_config_cache=/tmp/.npm npm pack --dry-run --json`
- [x] Verify tarball excludes:
- [x] `vormaclient/REFACTORING_CHECKLIST.tmp.md`
- [x] `vormaclient/client/src/tests/contracts/contract_test_harness.ts`
- [x] `vormaclient/client/src/tests/dist/dist_test_harness.ts`
- [x] `vormaclient/create/pnpm-lock.yaml`

### 4) Build Output Hygiene (`internal/cmd/buildts`)

- [x] Add explicit clean step for output roots before rebuild when rebuild is
      required.
- [x] Bump build cache version so existing stale outputs are migrated out on
      next build.
- [x] Expand output cleanup to remove internal declaration artifacts (for
      example `src/tests` declarations) from `npm_dist`.
- [x] Keep incremental skip path when input/output hash checks remain valid.

Validation:

- [x] `go run ./internal/cmd/buildts`
- [x] Verify stale artifacts are absent from `npm_dist`, including:
- [x] `npm_dist/vormaclient/client/src/core/navigation/begin_navigation_flow.d.ts`
- [x] `npm_dist/vormaclient/client/chunk-UZYFIXMX.js`
- [x] `npm_dist/vormaclient/client/src/tests/contracts/contract_test_harness.d.ts`

### 5) Legacy Alias Removal (`slots` -> `lanes`)

- [x] Remove deprecated `NavigationSlots` alias type and `*Slots` wrapper
      exports from runtime modules.
- [x] Remove deprecated operation-id alias wrapper in runtime state machine.
- [x] Update internal tests/imports to use `NavigationLanes` names directly.

Validation:

- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts`
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts`
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`

### 6) Final Package Verification

- [x] Re-run build and pack dry-run after all changes.
- [x] Confirm no stale/orphan artifacts or internal test declarations ship in
      tarball.

Validation:

- [x] `go run ./internal/cmd/buildts`
- [x] `npm_config_cache=/tmp/.npm npm pack --dry-run --json`
