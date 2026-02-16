Wave Normative Specification

Purpose

- Define authoritative, user-observable behavior for wave.
- Preserve semantics across implementation refactors and integration changes.

Guiding Principle

- Build orchestration, watch/reload behavior, static/CSS processing, and
  framework integration perform only the minimum work required to keep outputs
  and runtime data non-stale.

Scope

- In scope: externally observable `/wave/*` runtime and tooling behavior.
- Out of scope: non-`/wave/*` behavior unless required to explain wave
  semantics.

Correctness Precedence

- Baseline implementation behavior informs intent discovery but does not
  override correctness.
- Clearly incorrect or footgun baseline behavior is excluded from normative
  requirements.

Normative Record Format

- Every requirement contains:
    - user story
    - trigger conditions
    - expected observable behavior
    - protected user interest
    - cancellation/ordering/race constraints

Section Coverage

- This specification defines requirements for:
    - `HL-01` Config parsing, defaults, and environment override rules
    - `HL-02` Runtime state model and cache access surfaces
    - `HL-03` Dist layout and filesystem abstraction semantics
    - `HL-04` URL/filemap/public-path generation contracts
    - `HL-05` CSS build graph and output semantics
    - `HL-06` Static asset pipeline semantics
    - `HL-07` Build orchestration ordering and hook behavior
    - `HL-08` Dev bootstrap and lifecycle initialization
    - `HL-09` Watcher setup and directory-plan behavior
    - `HL-10` Watch event intake and debounce behavior
    - `HL-11` Event classification and matching semantics
    - `HL-12` On-change hook planning and execution semantics
    - `HL-13` Rebuild/restart decision matrix
    - `HL-14` App process lifecycle behavior
    - `HL-15` Vite integration lifecycle behavior
    - `HL-16` Refresh server and websocket protocol semantics
    - `HL-17` Browser reload/revalidate/hot-css behavior
    - `HL-18` Readiness/wait/timeout policy behavior
    - `HL-19` Static serving and middleware behavior
    - `HL-20` Lock/single-runner and command serialization behavior
    - `HL-21` CLI and run-mode contracts
    - `HL-22` Error handling and non-mutation safety guarantees
    - `HL-23` Cross-platform path and OS behavior
    - `HL-24` Framework integration runtime contracts
    - `HL-25` Development vs production behavioral deltas
    - `HL-26` Public API behavior surface

Coverage Traceability

- Requirement-to-test traceability is defined in:
  `wave/spec/wave-normative-test-coverage.md`.

Requirements

## HL-01 Config Parsing, Defaults, and Environment Override Rules

### `WAVE-HL01-001`

- User Story: Invalid config JSON fails fast before runtime/build execution.
- Trigger Conditions: `ParseConfig` receives invalid JSON bytes.
- Expected Observable Behavior: Parsing returns an error and no parsed config.
- Protected User Interest: Misconfigured applications fail early and clearly.
- Cancellation/Ordering/Race Constraints: Failure occurs before any dist-layout
  or runtime cache initialization.

### `WAVE-HL01-002`

- User Story: Core configuration is mandatory.
- Trigger Conditions: Parsed JSON omits `Core`.
- Expected Observable Behavior: Parsing/validation reports `Core` as required.
- Protected User Interest: Required foundational paths and behavior flags always
  exist before build/runtime logic runs.
- Cancellation/Ordering/Race Constraints: Requirement is enforced before fields
  under `Core` are dereferenced.

### `WAVE-HL01-003`

- User Story: Dist-root paths are normalized once at parse time.
- Trigger Conditions: `ParseConfig` succeeds with `Core.DistDir` set.
- Expected Observable Behavior: `ParsedConfig.Dist.Root` stores
  `filepath.Clean(Core.DistDir)`.
- Protected User Interest: Dist path behavior is stable across equivalent input
  spellings.
- Cancellation/Ordering/Race Constraints: Dist-layout computation precedes
  downstream path derivation.

### `WAVE-HL01-004`

- User Story: Empty config-file input is rejected.
- Trigger Conditions: `ParseConfigFile` receives empty/whitespace path.
- Expected Observable Behavior: `ParseConfigFile` returns an explicit error.
- Protected User Interest: File-based configuration cannot silently proceed with
  ambiguous path input.
- Cancellation/Ordering/Race Constraints: Path validation occurs before file
  read.

### `WAVE-HL01-005`

- User Story: File-based config preserves canonical config location.
- Trigger Conditions: `ParseConfigFile` successfully reads and parses config.
- Expected Observable Behavior: `Core.ConfigLocation` is set to the absolute
  normalized input path.
- Protected User Interest: Config reload and diagnostics reference a stable
  on-disk location.
- Cancellation/Ordering/Race Constraints: Location assignment happens after
  parse success and before returning parsed config.

### `WAVE-HL01-006`

- User Story: Build validation requires core app entry and dist directory.
- Trigger Conditions: `tooling.ValidateConfig` is called.
- Expected Observable Behavior: Validation fails when `Core.MainAppEntry` or
  `Core.DistDir` is empty.
- Protected User Interest: Build does not proceed without required source/output
  roots.
- Cancellation/Ordering/Race Constraints: Core field checks run before build
  orchestration.

### `WAVE-HL01-007`

- User Story: Static source directories are mandatory in browser-serving mode.
- Trigger Conditions: Validation runs with `Core.ServerOnlyMode=false`.
- Expected Observable Behavior: Validation requires both
  `Core.StaticAssetDirs.Private` and `Core.StaticAssetDirs.Public`.
- Protected User Interest: Browser/static pipeline has complete source roots.
- Cancellation/Ordering/Race Constraints: Requirement is conditionally skipped
  only for server-only mode.

### `WAVE-HL01-008`

- User Story: Vite integration requires an executable base command.
- Trigger Conditions: Validation runs with a non-nil `Vite` config block.
- Expected Observable Behavior: Validation fails when
  `Vite.JSPackageManagerBaseCmd` is empty.
- Protected User Interest: Vite invocation cannot enter partially configured
  states.
- Cancellation/Ordering/Race Constraints: Vite-required-field check is scoped to
  Vite-enabled configs.

### `WAVE-HL01-009`

- User Story: Healthcheck endpoint input remains a path, not a URL.
- Trigger Conditions: Validation sees non-empty `Watch.HealthcheckEndpoint`.
- Expected Observable Behavior: Endpoint is rejected when it has whitespace,
  lacks a leading `/`, starts with `//`, includes `://`, or contains query/hash
  segments.
- Protected User Interest: App readiness probing uses deterministic local path
  semantics.
- Cancellation/Ordering/Race Constraints: Endpoint normalization/validation
  occurs before runtime wait policies consume the value.

### `WAVE-HL01-010`

- User Story: Hook timeout configuration stays internally coherent.
- Trigger Conditions: Validation inspects stage-level and per-hook timeout
  fields.
- Expected Observable Behavior: Negative timeout values are rejected; per-hook
  explicit timeout cannot be combined with corresponding disable-stage-timeout
  flag.
- Protected User Interest: Hook execution timing behavior remains explicit and
  predictable.
- Cancellation/Ordering/Race Constraints: Timeout coherence checks run before
  hook pipeline execution planning.

### `WAVE-HL01-011`

- User Story: `RunOnChangeOnly` command hooks avoid ambiguous stage timing.
- Trigger Conditions: Validation inspects watched files with
  `RunOnChangeOnly=true`.
- Expected Observable Behavior: Command-like hooks under such watched files must
  use default/pre timing; non-pre timing is rejected.
- Protected User Interest: Change-only hook flows remain deterministic and avoid
  stage-order surprises.
- Cancellation/Ordering/Race Constraints: Rule applies to command-like hooks and
  allows callback-only hooks.

### `WAVE-HL01-012`

- User Story: Hook command source remains unambiguous.
- Trigger Conditions: Validation inspects each on-change hook.
- Expected Observable Behavior: A hook cannot set both `Cmd` and
  `RunCombinedDevBuildHookCommands`.
- Protected User Interest: Hook dispatch has one clear command source per hook
  entry.
- Cancellation/Ordering/Race Constraints: Conflict is rejected before plan
  generation.

### `WAVE-HL01-013`

- User Story: Public path prefix uses canonical slash behavior.
- Trigger Conditions: `ParsedConfig.PublicPathPrefix()` is evaluated.
- Expected Observable Behavior: Empty or `/` resolves to `/`; other values are
  normalized to leading-and-trailing slash form.
- Protected User Interest: Public URL generation is stable across prefix input
  variants.
- Cancellation/Ordering/Race Constraints: Normalization occurs before file-map
  URL lookup/join behavior.

### `WAVE-HL01-014`

- User Story: Browser runtime integration names are safe by default and
  configurable when needed.
- Trigger Conditions: Browser runtime namespace/function/element IDs are
  requested from parsed config.
- Expected Observable Behavior: Default internal names are used when framework
  overrides are unset; provided overrides are used when set.
- Protected User Interest: Applications avoid naming collisions while keeping
  explicit override control.
- Cancellation/Ordering/Race Constraints: Defaults are pure accessor-time
  behavior and do not mutate parsed config.

### `WAVE-HL01-015`

- User Story: Port env parsing rejects invalid values.
- Trigger Conditions: `GetPort` or `GetRefreshServerPort` reads environment.
- Expected Observable Behavior: Returned port is `0` unless env value parses to
  an integer in `1..65535`.
- Protected User Interest: Runtime and refresh networking avoid invalid bind
  targets.
- Cancellation/Ordering/Race Constraints: Validation happens on every env read
  path.

### `WAVE-HL01-016`

- User Story: Dev mode chooses a usable port once per resolver when app port is
  not already locked.
- Trigger Conditions: `MustGetPort` executes in dev mode and
  `WAVE_PORT_HAS_BEEN_SET` is not `true`.
- Expected Observable Behavior: A free port near configured/default port is
  selected, `PORT` is updated, and `WAVE_PORT_HAS_BEEN_SET=true` is written.
- Protected User Interest: Dev startup avoids port conflicts while keeping
  follow-up reads stable.
- Cancellation/Ordering/Race Constraints: Port selection runs inside
  resolver-scoped `sync.Once`.

### `WAVE-HL01-017`

- User Story: Non-dev or pre-set port paths remain deterministic and safe.
- Trigger Conditions: `MustGetPort` executes outside dev auto-allocation path.
- Expected Observable Behavior: Configured valid `PORT` is used; otherwise
  fallback defaults to `8080`.
- Protected User Interest: App port behavior is stable even with missing/invalid
  env values.
- Cancellation/Ordering/Race Constraints: Result is cached per resolver after
  first resolution.

## HL-02 Runtime State Model and Cache Access Surfaces

### `WAVE-HL02-001`

- User Story: Wave runtime initialization fails fast on missing/invalid config.
- Trigger Conditions: `wave.New` is called.
- Expected Observable Behavior: `wave.New` panics when config bytes are empty or
  parse fails.
- Protected User Interest: Runtime instances are never created in partially
  configured states.
- Cancellation/Ordering/Race Constraints: Validation/parse gates all cache and
  filesystem initialization.

### `WAVE-HL02-002`

- User Story: Caller-owned config bytes cannot mutate Wave internals after
  construction.
- Trigger Conditions: `wave.New` accepts `Config.WaveConfigJSON`.
- Expected Observable Behavior: Config bytes are cloned before parsing and
  stored as cloned bytes.
- Protected User Interest: Runtime behavior is isolated from caller buffer
  mutation.
- Cancellation/Ordering/Race Constraints: Clone occurs before parse and before
  storing `rawCfg`.

### `WAVE-HL02-003`

- User Story: Runtime cache surfaces are initialized from one deterministic
  constructor path.
- Trigger Conditions: `wave.New` completes object construction.
- Expected Observable Behavior: Base/public/private FS, file map, CSS,
  stylesheet refs, filemap refs, URL resolver cache, and public-asset cache are
  all initialized via `initRuntimeCaches`.
- Protected User Interest: Runtime accessors observe consistent cache behavior
  across surfaces.
- Cancellation/Ordering/Race Constraints: Cache wiring happens once during
  constructor flow.

### `WAVE-HL02-004`

- User Story: Dev mode reflects fresh filesystem state without manual
  cache-busting.
- Trigger Conditions: Any `cache` or `cacheMap` accessor is called while
  `GetIsDev()` is true.
- Expected Observable Behavior: Value function executes on every read instead of
  returning a persistent cached value.
- Protected User Interest: Dev runtime reflects latest build/static outputs.
- Cancellation/Ordering/Race Constraints: Dev bypass path is evaluated at each
  read.

### `WAVE-HL02-005`

- User Story: Production mode avoids repeated expensive initialization for the
  same cache key.
- Trigger Conditions: Any `cache`/`cacheMap` accessor is called while
  `GetIsDev()` is false.
- Expected Observable Behavior: Value is resolved once via `sync.Once` and
  reused for subsequent reads.
- Protected User Interest: Stable low-overhead runtime behavior in production.
- Cancellation/Ordering/Race Constraints: `cacheMap` stores one entry per key
  and can evict entries when cache policy rejects caching.

### `WAVE-HL02-006`

- User Story: Public-asset existence cache avoids sticky false negatives.
- Trigger Conditions: `isAsset` cache stores results from `checkIsAsset`.
- Expected Observable Behavior: Cache stores only successful `true` results;
  false or errored results are not retained.
- Protected User Interest: Newly created assets become discoverable without
  stale-negative cache poisoning.
- Cancellation/Ordering/Race Constraints: Custom cache policy gates entry
  retention.

### `WAVE-HL02-007`

- User Story: Public file-map access is defensive against accidental caller
  mutation.
- Trigger Conditions: `GetPublicFileMap` is called.
- Expected Observable Behavior: Returned map is a clone, not the internal cache
  backing map.
- Protected User Interest: Callers cannot corrupt shared runtime filemap state.
- Cancellation/Ordering/Race Constraints: Clone occurs on every accessor call.

### `WAVE-HL02-008`

- User Story: Non-panicking and panicking accessor variants are explicit.
- Trigger Conditions: Filesystem accessor methods are used.
- Expected Observable Behavior: `GetBaseFS`/`GetPublicFS`/`GetPrivateFS` return
  errors; `MustGetPublicFS`/`MustGetPrivateFS` panic on initialization failure.
- Protected User Interest: Applications choose explicit error-handling policy.
- Cancellation/Ordering/Race Constraints: `Must*` variants delegate through
  cache-get and panic only on non-nil error.

## HL-03 Dist Layout and Filesystem Abstraction Semantics

### `WAVE-HL03-001`

- User Story: Dist output paths are derived from a stable layout contract.
- Trigger Conditions: Dist layout methods on `DistLayout` are used.
- Expected Observable Behavior: Layout resolves `static`, `assets/public`,
  `assets/private`, `internal`, and internal reference/gob files under one dist
  root.
- Protected User Interest: Build/runtime components agree on where artifacts are
  written and read.
- Cancellation/Ordering/Race Constraints: Path derivation is pure and
  deterministic.

### `WAVE-HL03-002`

- User Story: Binary output name is OS-correct.
- Trigger Conditions: `DistLayout.Binary()` is computed.
- Expected Observable Behavior: Binary name is `main` on non-Windows and
  `main.exe` on Windows.
- Protected User Interest: Build/run command wiring targets the correct binary
  artifact.
- Cancellation/Ordering/Race Constraints: OS switch is evaluated at call time.

### `WAVE-HL03-003`

- User Story: Runtime relative-path constants remain fs.Sub-compatible.
- Trigger Conditions: `RelPaths` methods are used by runtime/build code.
- Expected Observable Behavior: Returned paths are relative, forward-slash
  separated, and omit leading slash.
- Protected User Interest: Shared runtime/build FS lookups remain portable.
- Cancellation/Ordering/Race Constraints: RelPath methods are static and
  side-effect free.

### `WAVE-HL03-004`

- User Story: Dist directory bootstrap always creates required directories.
- Trigger Conditions: `tooling.SetupDistDir` runs.
- Expected Observable Behavior: Internal/public/private directories are created
  and `.keep` file is written in `dist/static`.
- Protected User Interest: Runtime embed/filesystem assumptions are satisfied
  before serving/building.
- Cancellation/Ordering/Race Constraints: Directory creation completes before
  `.keep` file write.

### `WAVE-HL03-005`

- User Story: Full file-processing resets static outputs while preserving dev
  lock coordination.
- Trigger Conditions: `Builder.processFiles(granular=false, ...)` runs.
- Expected Observable Behavior: Contents of `dist/static` are removed except
  lock files, then required dist structure is recreated.
- Protected User Interest: Full builds start from clean artifact state without
  breaking active lock semantics.
- Cancellation/Ordering/Race Constraints: Cleanup occurs before static/CSS
  rebuild tasks start.

### `WAVE-HL03-006`

- User Story: Runtime filesystem source is environment-appropriate.
- Trigger Conditions: `initBaseFS` executes.
- Expected Observable Behavior: Dev mode uses `os.DirFS(dist/static)`;
  production mode uses provided `DistStaticFS`.
- Protected User Interest: Dev reads live dist output while production reads
  shipped static FS.
- Cancellation/Ordering/Race Constraints: Production path fails if
  `DistStaticFS` is nil.

### `WAVE-HL03-007`

- User Story: Public and private FS views are scoped sub-filesystems.
- Trigger Conditions: `initPublicFS`/`initPrivateFS` executes.
- Expected Observable Behavior: `fs.Sub(baseFS, assets/public)` and
  `fs.Sub(baseFS, assets/private)` are used.
- Protected User Interest: Public/private asset reads cannot cross-contaminate
  directory roots.
- Cancellation/Ordering/Race Constraints: Base FS must resolve before sub-FS
  creation.

## HL-04 URL/Filemap/Public-Path Generation Contracts

### `WAVE-HL04-001`

- User Story: Runtime public file map is loaded from canonical internal gob
  location.
- Trigger Conditions: `initFileMap` executes.
- Expected Observable Behavior: Runtime opens `internal/public_filemap.gob` from
  base FS and decodes it.
- Protected User Interest: Runtime URL resolution uses build-produced filemap
  data.
- Cancellation/Ordering/Race Constraints: File-open and decode failures are
  returned as errors.

### `WAVE-HL04-002`

- User Story: File map lookup normalizes lookup keys and always returns a URL.
- Trigger Conditions: `FileMap.Lookup` runs with an original path and public
  prefix.
- Expected Observable Behavior: Original path is cleaned/normalized; hashed
  dist-name is used when present, otherwise normalized original is returned
  under the same prefix.
- Protected User Interest: URL generation remains stable even when filemap entry
  is missing.
- Cancellation/Ordering/Race Constraints: Returned URL is always leading-slash
  normalized.

### `WAVE-HL04-003`

- User Story: Already-absolute/passthrough URLs are not rewritten.
- Trigger Conditions: `resolvePublicURL` receives URL with supported passthrough
  scheme/prefix (`data:`, `http:`, `https:`, `ws:`, `wss:`, `blob:`, `file:`,
  `//`).
- Expected Observable Behavior: Input URL is returned unchanged.
- Protected User Interest: External or embedded URLs remain valid.
- Cancellation/Ordering/Race Constraints: Passthrough check runs before filemap
  lookup.

### `WAVE-HL04-004`

- User Story: Missing filemap entries degrade gracefully.
- Trigger Conditions: `resolvePublicURL` cannot find mapped dist name for a
  normalized original path.
- Expected Observable Behavior: Resolver logs warning and returns normalized
  prefix-joined original path.
- Protected User Interest: Runtime remains functional during partial/missing map
  conditions.
- Cancellation/Ordering/Race Constraints: Lookup uses cached filemap accessor;
  lookup failure does not abort URL generation.

### `WAVE-HL04-005`

- User Story: Public URL construction from internal ref files normalizes and
  validates empty paths.
- Trigger Conditions: `joinPublicURLFromRefPath` receives a ref path.
- Expected Observable Behavior: Empty/invalid normalized ref path yields empty
  string; valid ref path is joined with public prefix as leading-slash URL.
- Protected User Interest: Ref-file parsing does not emit malformed URLs.
- Cancellation/Ordering/Race Constraints: Ref content is trimmed before
  normalization.

### `WAVE-HL04-006`

- User Story: Runtime public filemap URL comes from internal ref file contract.
- Trigger Conditions: `initFileMapURL` executes.
- Expected Observable Behavior: URL is resolved from
  `internal/public_file_map_file_ref.txt`.
- Protected User Interest: Runtime script/link tags target current hashed
  filemap artifact.
- Cancellation/Ordering/Race Constraints: Ref file read failure propagates.

### `WAVE-HL04-007`

- User Story: Browser-side public URL resolver bootstrap is emitted with CSP
  hash support.
- Trigger Conditions: `initFileMapDetails` executes with non-empty filemap URL.
- Expected Observable Behavior: Generated HTML includes modulepreload link and a
  module script that installs public URL resolver function; script SHA-256 hash
  is computed and exposed.
- Protected User Interest: Browser can resolve hashed public assets safely under
  CSP.
- Cancellation/Ordering/Race Constraints: On rendering/hash errors, details
  initialization fails.

### `WAVE-HL04-008`

- User Story: Public-asset routing only handles paths under configured public
  prefix.
- Trigger Conditions: `publicAssetPath`/`checkIsAsset` processes request URL.
- Expected Observable Behavior: Path is cleaned, prefix-stripped when
  applicable, root/empty paths are rejected, and only existing non-directory
  files count as assets.
- Protected User Interest: Static middleware serves only valid public assets.
- Cancellation/Ordering/Race Constraints: Prefix validation occurs before FS
  stat.

### `WAVE-HL04-009`

- User Story: Static serving policy is explicit and prefix-aware.
- Trigger Conditions: Static handlers/middleware are created and request routing
  runs.
- Expected Observable Behavior: Static file server strips public prefix;
  optional immutable cache header mode sets
  `Cache-Control: public, max-age=31536000, immutable`; middleware serves static
  handler only for public-asset requests.
- Protected User Interest: Asset caching and route passthrough behavior remain
  deterministic.
- Cancellation/Ordering/Race Constraints: Non-asset requests must fall through
  to next handler.

### `WAVE-HL04-010`

- User Story: Favicon request routing uses public file-map lookup and does not
  guess paths.
- Trigger Conditions: `FaviconRedirect` handles `GET`/`HEAD /favicon.ico`.
- Expected Observable Behavior: Middleware resolves `favicon.ico` through public
  filemap and issues redirect when found; otherwise returns 404.
- Protected User Interest: Favicon serving remains compatible with hashed public
  outputs.
- Cancellation/Ordering/Race Constraints: Missing filemap/load errors are
  treated as not found.

## HL-05 CSS Build Graph and Output Semantics

### `WAVE-HL05-001`

- User Story: CSS build ordering is deterministic.
- Trigger Conditions: `cssProcessor.buildAll` runs.
- Expected Observable Behavior: Critical CSS build runs before non-critical CSS
  build.
- Protected User Interest: Critical-style output availability is predictable.
- Cancellation/Ordering/Race Constraints: Any critical-build failure stops the
  normal-build step.

### `WAVE-HL05-002`

- User Story: Missing CSS entries disable only their own CSS branch.
- Trigger Conditions: A CSS build nature has no configured entry file.
- Expected Observable Behavior: That build nature clears tracked state and exits
  without output writes.
- Protected User Interest: Optional CSS entry configuration behaves as opt-in.
- Cancellation/Ordering/Race Constraints: Entry check happens before esbuild
  context build.

### `WAVE-HL05-003`

- User Story: CSS build rejects empty esbuild outputs.
- Trigger Conditions: esbuild rebuild completes with zero output files.
- Expected Observable Behavior: Build returns an explicit error for missing CSS
  output.
- Protected User Interest: CSS pipeline does not silently succeed with no
  artifact.
- Cancellation/Ordering/Race Constraints: Output-count validation runs before
  output write paths.

### `WAVE-HL05-004`

- User Story: CSS `url(...)` resolution preserves external URLs and resolves
  local asset URLs through Wave public filemap.
- Trigger Conditions: esbuild URL-resolver plugin handles a CSS URL token.
- Expected Observable Behavior: Absolute/protocol-relative URLs are left as-is;
  relative URLs are resolved with buildtime public URL resolver and emitted as
  external URLs.
- Protected User Interest: CSS asset URLs remain valid across hashed/non-hashed
  outputs.
- Cancellation/Ordering/Race Constraints: Resolver is applied only for
  CSS-URL-token resolution kind.

### `WAVE-HL05-005`

- User Story: CSS import dependency tracking is path-normalized.
- Trigger Conditions: Build parses esbuild metafile inputs.
- Expected Observable Behavior: Critical and normal import sets store normalized
  absolute-or-cleaned paths for dependency checks.
- Protected User Interest: CSS-change classification is robust across symlink
  and path-variant forms.
- Cancellation/Ordering/Race Constraints: Import-set updates are write-locked
  per build nature.

### `WAVE-HL05-006`

- User Story: Critical CSS output is written to stable internal output path.
- Trigger Conditions: Critical CSS build succeeds.
- Expected Observable Behavior: Critical CSS is written atomically to
  `dist/static/internal/critical.css`, skipping rewrite when unchanged.
- Protected User Interest: Runtime critical CSS reads target a stable output
  contract.
- Cancellation/Ordering/Race Constraints: Atomic-write behavior prevents partial
  writes.

### `WAVE-HL05-007`

- User Story: Normal CSS output is hashed and referenced through internal ref
  file.
- Trigger Conditions: Normal CSS build succeeds.
- Expected Observable Behavior: Hashed normal CSS artifact is published under
  public static output; internal normal-CSS ref file points to active hashed
  filename.
- Protected User Interest: Browser cache busting remains automatic for
  non-critical CSS.
- Cancellation/Ordering/Race Constraints: Old hashed artifacts are cleaned per
  ref/glob policy before/while publishing new artifact.

### `WAVE-HL05-008`

- User Story: Hot-reload CSS output reads can require fresh rebuild output.
- Trigger Conditions: Hot-reload read methods are called with
  `requireFreshBuildOutput=true`.
- Expected Observable Behavior: If build invalidated cache and new output has
  not been produced, read returns explicit unavailability error instead of stale
  fallback.
- Protected User Interest: Reload payload generation can enforce freshness when
  required.
- Cancellation/Ordering/Race Constraints: Cached output is preferred when
  available; disk fallback remains available when freshness requirement allows.

## HL-06 Static Asset Pipeline Semantics

### `WAVE-HL06-001`

- User Story: Public and private static pipelines apply distinct hashing policy.
- Trigger Conditions: Static processing runs for public/private roots.
- Expected Observable Behavior: Public outputs are hashed by default; private
  outputs preserve relative path naming.
- Protected User Interest: Browser-served assets get cache-busting names while
  private server-only files keep stable names.
- Cancellation/Ordering/Race Constraints: Hashing policy is bound to pipeline
  (`processPublicFiles` vs `processPrivateFiles`).

### `WAVE-HL06-002`

- User Story: `prehashed` and `__nohash` source folders opt files out of hashing
  while preserving logical paths.
- Trigger Conditions: Static source file path is resolved to file info.
- Expected Observable Behavior: Prefix is stripped from logical relative path
  and output marks file as prehashed/nohash.
- Protected User Interest: Applications can provide pre-versioned assets or
  explicit unhashed paths.
- Cancellation/Ordering/Race Constraints: Prefix stripping occurs before logical
  path collision checks.

### `WAVE-HL06-003`

- User Story: Known ignorable files do not enter static maps.
- Trigger Conditions: Static file resolution handles ignored file basenames.
- Expected Observable Behavior: Ignored entries (for example `.DS_Store`) are
  skipped.
- Protected User Interest: Spurious filesystem noise does not mutate output
  artifacts.
- Cancellation/Ordering/Race Constraints: Ignore check occurs before processing
  and map mutation.

### `WAVE-HL06-004`

- User Story: Multiple source files cannot map to the same logical static path.
- Trigger Conditions: Static discovery/resolution finds collisions across base,
  `prehashed`, or `__nohash` variants.
- Expected Observable Behavior: Build fails with explicit collision error.
- Protected User Interest: Asset lookup remains one-to-one and deterministic.
- Cancellation/Ordering/Race Constraints: Collision detection runs before
  writing output/map updates.

### `WAVE-HL06-005`

- User Story: Full static builds process discovered files concurrently.
- Trigger Conditions: Full static processing walks source directory.
- Expected Observable Behavior: Worker pool processes resolved files
  concurrently and produces final map from successful results.
- Protected User Interest: Static build scales while preserving deterministic
  map output.
- Cancellation/Ordering/Race Constraints: First processing error cancels
  remaining work.

### `WAVE-HL06-006`

- User Story: Incremental static processing updates only changed logical paths
  when safe.
- Trigger Conditions: Changed-source-path incremental path processing runs.
- Expected Observable Behavior: Changed files are resolved to logical paths,
  updated/removed selectively, and full fallback runs only when resolution or
  map preconditions require it.
- Protected User Interest: Dev static changes apply quickly without unnecessary
  full rebuild.
- Cancellation/Ordering/Race Constraints: Source-root change or unresolved state
  triggers full rebuild fallback.

### `WAVE-HL06-007`

- User Story: Missing source directory clears stale static outputs for that
  pipeline.
- Trigger Conditions: Static processing starts and source directory does not
  exist.
- Expected Observable Behavior: Existing map/artifacts for that pipeline are
  cleaned and replaced with empty map state.
- Protected User Interest: Removed static roots do not leave stale served files.
- Cancellation/Ordering/Race Constraints: Cleanup occurs before saving empty
  map.

### `WAVE-HL06-008`

- User Story: Static artifact cleanup removes stale output names when mappings
  change.
- Trigger Conditions: Old and new static maps differ by removed keys or changed
  output dist names.
- Expected Observable Behavior: Old mapped dist artifacts are removed from
  output directory.
- Protected User Interest: Output directories do not accumulate stale hashed
  files that no longer belong to current map.
- Cancellation/Ordering/Race Constraints: Cleanup runs before final map is saved
  and before public filemap JS refresh.

## HL-07 Build Orchestration Ordering and Hook Behavior

### `WAVE-HL07-001`

- User Story: Build execution validates config before touching artifacts.
- Trigger Conditions: `Builder.Build` starts.
- Expected Observable Behavior: `ValidateConfig` runs before processing files,
  running hooks, writing schema, or compiling go.
- Protected User Interest: Build failures from config issues are immediate and
  do not mutate build outputs.
- Cancellation/Ordering/Race Constraints: Validation failure terminates build.

### `WAVE-HL07-002`

- User Story: File processing runs before and after hooks.
- Trigger Conditions: Full `Builder.Build` path executes.
- Expected Observable Behavior: File processing runs once before hooks and again
  after hooks to include hook-generated files.
- Protected User Interest: Generated assets/artifacts from hooks are reflected
  in final outputs.
- Cancellation/Ordering/Race Constraints: Post-hook processing runs only after
  successful hook stage.

### `WAVE-HL07-003`

- User Story: Build hooks execute in deterministic order.
- Trigger Conditions: `runHooks` executes.
- Expected Observable Behavior: User hook runs before framework hook(s);
  framework function hook override executes in place of framework command string
  hook when provided.
- Protected User Interest: Hook side effects and generated outputs follow stable
  precedence.
- Cancellation/Ordering/Race Constraints: Hook stage stops on first hook error.

### `WAVE-HL07-004`

- User Story: Hook command timeout policy is mode-aware and explicitly applied.
- Trigger Conditions: Hook command execution is prepared for dev or prod build.
- Expected Observable Behavior: Timeout duration is derived from mode-specific
  core timeout settings and enforced via command execution context.
- Protected User Interest: Hook commands cannot hang build indefinitely when
  timeout policy is configured.
- Cancellation/Ordering/Race Constraints: Timeout context is created per hook
  command invocation.

### `WAVE-HL07-005`

- User Story: Schema emission failure does not invalidate otherwise successful
  build outputs.
- Trigger Conditions: `writeConfigSchema` fails during build.
- Expected Observable Behavior: Build logs schema write warning and continues.
- Protected User Interest: Non-critical schema emission does not block artifact
  generation.
- Cancellation/Ordering/Race Constraints: Warning path is non-fatal by design.

### `WAVE-HL07-006`

- User Story: Go compilation is optional and mode-aware.
- Trigger Conditions: Build reaches compile stage with `CompileGo` true.
- Expected Observable Behavior: Go build command runs with production tag
  (`-tags=prod`) only in prod mode and outputs binary to configured dist binary
  path.
- Protected User Interest: Dev/prod binary compilation semantics remain
  explicit.
- Cancellation/Ordering/Race Constraints: Compile stage is skipped when
  `CompileGo` is false.

### `WAVE-HL07-007`

- User Story: Framework Go build overlay lifecycle is explicit and cleaned up.
- Trigger Conditions: Framework overlay provider is configured for go build.
- Expected Observable Behavior: Overlay config path is passed to `go build`;
  overlay cleanup function runs after build and cleanup errors are surfaced.
- Protected User Interest: Temporary overlay state does not leak between builds.
- Cancellation/Ordering/Race Constraints: Cleanup runs even when go build fails.

### `WAVE-HL07-008`

- User Story: Browser-related file processing is skipped in server-only mode.
- Trigger Conditions: `processFiles` executes while `cfg.UsingBrowser()` is
  false.
- Expected Observable Behavior: Public/private static processing and CSS build
  steps are skipped.
- Protected User Interest: Server-only builds avoid unnecessary browser-asset
  work.
- Cancellation/Ordering/Race Constraints: Skip decision occurs after dist setup
  and before per-pipeline processing.

### `WAVE-HL07-009`

- User Story: Browser builds prioritize public static processing before CSS.
- Trigger Conditions: `processFiles` executes with browser mode enabled.
- Expected Observable Behavior: Public static files process first; private
  static and CSS processing run concurrently afterward.
- Protected User Interest: CSS URL resolution sees up-to-date public file map.
- Cancellation/Ordering/Race Constraints: Public processing completion precedes
  starting private/CSS parallel group.

## HL-08 Dev Bootstrap and Lifecycle Initialization

### `WAVE-HL08-001`

- User Story: Watcher intake batches bursty fsnotify events before execution.
- Trigger Conditions: `runWatcherWithContext` receives watcher events.
- Expected Observable Behavior: Events are debounced (30ms window) and
  dispatched to `processEvents` as a batch.
- Protected User Interest: Event processing avoids rebuild storms from
  high-frequency filesystem bursts.
- Cancellation/Ordering/Race Constraints: Debounce callback checks watcher
  execution context cancellation before dispatch.

### `WAVE-HL08-002`

- User Story: Watcher loop exits deterministically during shutdown.
- Trigger Conditions: Watcher context is canceled, watcher event channel closes,
  or watcher error channel closes.
- Expected Observable Behavior: Watcher loop returns without additional event
  dispatch.
- Protected User Interest: Dev runtime teardown avoids orphan goroutines and
  duplicate processing.
- Cancellation/Ordering/Race Constraints: Exit checks happen in main watcher
  select loop.

### `WAVE-HL08-003`

- User Story: Event processing does not run without required live runtime
  dependencies.
- Trigger Conditions: `processEvents` is invoked when watcher or builder is nil.
- Expected Observable Behavior: Processing returns immediately.
- Protected User Interest: Partial lifecycle states do not execute inconsistent
  event plans.
- Cancellation/Ordering/Race Constraints: Dependency guard runs before planning.

### `WAVE-HL08-004`

- User Story: Config file mutation has top priority over normal event execution.
- Trigger Conditions: Execution planning detects config-changed event.
- Expected Observable Behavior: Config restart is triggered and normal
  events-with-hooks execution is skipped.
- Protected User Interest: Config changes always apply through restart semantics
  before continuing normal watch pipeline.
- Cancellation/Ordering/Race Constraints: Config-change short-circuit occurs
  before rebuilding-overlay broadcast or hook execution.

## HL-09 Watcher Setup and Directory-Plan Behavior

### `WAVE-HL09-001`

- User Story: Event pre-classification computes one deterministic plan per
  watcher batch.
- Trigger Conditions: `buildWatcherEventPreClassificationPlanFromEvents` runs.
- Expected Observable Behavior: Plan contains config-changed flag,
  add-directory-watch list, and events-to-classify list.
- Protected User Interest: Directory-watch mutations and event classification
  are coordinated and reproducible.
- Cancellation/Ordering/Race Constraints: Config-change detection short-circuits
  the remaining plan.

### `WAVE-HL09-002`

- User Story: Config-file detection is path-shape resilient.
- Trigger Conditions: Event path is compared against configured
  `Core.ConfigLocation`.
- Expected Observable Behavior: Config detection uses location-equivalence logic
  instead of raw string equality.
- Protected User Interest: Config restarts still trigger when event path
  spellings differ but refer to the same file.
- Cancellation/Ordering/Race Constraints: Config-path probe is memoized per path
  during planning.

### `WAVE-HL09-003`

- User Story: New/renamed directories become watched without requiring manual
  watcher restart.
- Trigger Conditions: Pre-classification sees create/rename paths that are
  directories or stat-probe-missing candidates.
- Expected Observable Behavior: Path is queued for watcher `AddDir` side effect.
- Protected User Interest: Newly created source directories are observed by the
  watcher pipeline.
- Cancellation/Ordering/Race Constraints: Add-dir side effects execute before
  event classification phase.

### `WAVE-HL09-004`

- User Story: Directory-watch add errors are logged only when actionable.
- Trigger Conditions: `AddDir` returns error during pre-classification side
  effects.
- Expected Observable Behavior: Expected transient errors (`not exist`,
  `ENOTDIR`) are suppressed; other errors are logged.
- Protected User Interest: Watcher diagnostics highlight real problems without
  noisy benign warnings.
- Cancellation/Ordering/Race Constraints: Error filtering runs per add-dir
  attempt.

## HL-10 Watch Event Intake and Debounce Behavior

### `WAVE-HL10-001`

- User Story: Duplicate watcher events for the same logical file collapse into
  one event.
- Trigger Conditions: Multiple fsnotify events map to equivalent path keys in a
  batch.
- Expected Observable Behavior: Event ops are OR-merged per deduplicated path.
- Protected User Interest: Build/hook pipeline avoids duplicate work for
  repeated path notifications.
- Cancellation/Ordering/Race Constraints: Deduplication runs before event
  classification.

### `WAVE-HL10-002`

- User Story: Deduplicated watcher output is deterministic.
- Trigger Conditions: `deduplicateWatcherEventsByPath` returns merged events.
- Expected Observable Behavior: Output is sorted by deduplicated event path.
- Protected User Interest: Batch processing behavior and logs are stable across
  equivalent inputs.
- Cancellation/Ordering/Race Constraints: Sorting occurs after merge map
  construction.

### `WAVE-HL10-003`

- User Story: Absolute-path aliases dedupe to one logical event key.
- Trigger Conditions: Different absolute path spellings
  (canonical/symlink/missing alias variants) refer to same location or same
  missing-file alias key.
- Expected Observable Behavior: Deduplication index resolves an existing path
  key and merges operations into first-seen key.
- Protected User Interest: Symlink/path-shape noise does not duplicate rebuild
  decisions.
- Cancellation/Ordering/Race Constraints: Canonical/missing-alias probes are
  memoized per absolute path.

### `WAVE-HL10-004`

- User Story: Ignored/chmod-only events do not reach execution planning.
- Trigger Conditions: Post-classification filter evaluates classified events.
- Expected Observable Behavior: Events marked `ignored` or `chmodOnly` are
  removed from processing set.
- Protected User Interest: No-op or irrelevant file events do not trigger hook
  or rebuild work.
- Cancellation/Ordering/Race Constraints: Post-classification filtering runs
  before event-to-hook plan construction.

## HL-11 Event Classification and Matching Semantics

### `WAVE-HL11-001`

- User Story: Empty watcher batches do not produce execution plans.
- Trigger Conditions: Classification/planning receives no processable events.
- Expected Observable Behavior: Planning result returns no `eventsWithHooks` and
  no config-change trigger.
- Protected User Interest: No accidental work when batch has nothing to do.
- Cancellation/Ordering/Race Constraints: Empty checks occur at both
  classification and plan-build boundaries.

### `WAVE-HL11-002`

- User Story: Mixed file classes remain distinguishable through planning.
- Trigger Conditions: Batch includes files across distinct classes (for example
  go/static/other/css).
- Expected Observable Behavior: Classified events preserve their per-file
  `fileType` and corresponding downstream work semantics.
- Protected User Interest: Per-class build/restart behavior remains accurate in
  mixed batches.
- Cancellation/Ordering/Race Constraints: File-type classification precedes hook
  planning and implicit-work derivation.

### `WAVE-HL11-003`

- User Story: Event planning preserves event order while still enabling
  per-pattern hook dedupe.
- Trigger Conditions: `buildEventExecutionPlanFromClassifiedEvents` transforms
  classified events.
- Expected Observable Behavior: Events remain one-to-one with classified inputs;
  duplicate-hook suppression is tracked separately through flags.
- Protected User Interest: File-level traceability remains intact while avoiding
  redundant hook execution.
- Cancellation/Ordering/Race Constraints: Skip-duplicate decision is derived
  once per watched pattern occurrence order.

## HL-12 On-Change Hook Planning and Execution Semantics

### `WAVE-HL12-001`

- User Story: Hook context path surfaces are normalized and stable.
- Trigger Conditions: Event-with-hooks plan is built.
- Expected Observable Behavior: Hook context `FilePath` is absolute-normalized;
  `ChangedFilePaths` is pattern-scoped list with deduped normalized paths.
- Protected User Interest: Hook callbacks receive deterministic file path
  inputs.
- Cancellation/Ordering/Race Constraints: Pattern-scoped changed-path lists are
  copied for multi-event pattern usage.

### `WAVE-HL12-002`

- User Story: Hook execution for repeated matched patterns runs once per stage.
- Trigger Conditions: Multiple events in a batch match the same watched pattern.
- Expected Observable Behavior: First event executes hooks; later matching
  events set `skipDuplicateHooks` and skip stage execution.
- Protected User Interest: Pattern-level hooks avoid duplicate side effects in
  one batch.
- Cancellation/Ordering/Race Constraints: Duplicate suppression is
  stage-agnostic and descriptor-based.

### `WAVE-HL12-003`

- User Story: Hook execution does not mutate shared hook contexts across
  callbacks.
- Trigger Conditions: Stage or no-wait callbacks run for one event.
- Expected Observable Behavior: Each callback receives a cloned hook context
  with independent `ChangedFilePaths` slice and execution context.
- Protected User Interest: Callback-local mutations do not leak across hooks.
- Cancellation/Ordering/Race Constraints: Context cloning happens per callback
  execution.

### `WAVE-HL12-004`

- User Story: Run-on-change-only mode keeps callbacks while suppressing command
  execution in stage-aware paths.
- Trigger Conditions: Hook is resolved for stage execution with
  `runOnChangeOnly=true` and stage applies run-on-change-only command rules.
- Expected Observable Behavior: Command-only hooks are skipped; callback hooks
  remain with command fields stripped.
- Protected User Interest: Run-on-change-only semantics avoid implicit shell
  command side effects while preserving callback behavior.
- Cancellation/Ordering/Race Constraints: Exclude matching is evaluated before
  run-on-change-only filtering.

### `WAVE-HL12-005`

- User Story: Hook callback panics surface as errors and do not run paired
  commands.
- Trigger Conditions: Callback execution panics inside hook execution plan.
- Expected Observable Behavior: Panic is recovered and returned as callback
  execution error; command phase for that hook plan is skipped.
- Protected User Interest: Hook failures are visible and avoid partially unsafe
  mixed callback-command execution.
- Cancellation/Ordering/Race Constraints: Panic recovery wraps callback
  execution boundary.

### `WAVE-HL12-006`

- User Story: Hook execution errors include stage and file-path attribution.
- Trigger Conditions: Stage hook command/callback returns execution error.
- Expected Observable Behavior: Error message includes stage label and changed
  file path where available.
- Protected User Interest: Hook failure diagnosis is actionable.
- Cancellation/Ordering/Race Constraints: Stage/path wrapping happens before
  stage-level error aggregation.

### `WAVE-HL12-007`

- User Story: Concurrent hook stage preserves descriptor order for accumulated
  actions while allowing parallel execution.
- Trigger Conditions: Concurrent hooks run across multiple eligible events.
- Expected Observable Behavior: Event descriptors execute concurrently, but
  merged action list preserves descriptor-order concatenation.
- Protected User Interest: Concurrent performance does not destroy deterministic
  downstream action reduction behavior.
- Cancellation/Ordering/Race Constraints: Per-descriptor results are placed back
  into fixed index slots before flattening.

### `WAVE-HL12-008`

- User Story: No-wait hooks run asynchronously with bounded concurrency and
  lifecycle cancellation.
- Trigger Conditions: Concurrent-no-wait hooks are fired.
- Expected Observable Behavior: Hook jobs enqueue through bounded limiter
  (capacity 16), inherit lifecycle execution context, and can be canceled on
  run-cycle cleanup.
- Protected User Interest: Fire-and-forget hooks avoid blocking main pipeline
  while still respecting lifecycle boundaries.
- Cancellation/Ordering/Race Constraints: Limiter acquisition and lifecycle
  context checks gate execution.

### `WAVE-HL12-009`

- User Story: Stage timeout policy applies by stage with per-hook override
  controls.
- Trigger Conditions: Stage command/callback execution resolves timeout values.
- Expected Observable Behavior: Stage timeout defaults are used unless per-hook
  timeout override is set; stage timeout can be disabled per hook.
- Protected User Interest: Hook latency bounds are configurable without losing
  per-hook control.
- Cancellation/Ordering/Race Constraints: Timeout contexts are derived at hook
  execution boundary.

## HL-13 Rebuild/Restart Decision Matrix

### `WAVE-HL13-001`

- User Story: Hard-reload sensitivity is derived from event type and watch
  flags.
- Trigger Conditions: Event is transformed into event-with-hooks plan.
- Expected Observable Behavior: Go-file events or watched files with
  `RecompileGoBinary`/`RestartApp` mark `needsHardReload=true`.
- Protected User Interest: App-stop/restart behavior aligns with file-change
  impact.
- Cancellation/Ordering/Race Constraints: Hard-reload derivation occurs before
  app-stop strategy resolution.

### `WAVE-HL13-002`

- User Story: App-stop strategy selection is deterministic for single vs batch
  plans.
- Trigger Conditions: Behavioral decision resolves app-stop strategy.
- Expected Observable Behavior: Single hard-reload event uses
  `single-event-hard-reload`; batch with any hard-reload event uses
  `batch-hard-reload`; otherwise `none`.
- Protected User Interest: App-stop behavior is predictable and reproducible.
- Cancellation/Ordering/Race Constraints: Strategy resolution happens before
  hook pipeline execution.

### `WAVE-HL13-003`

- User Story: Implicit build runs only when at least one event is not
  run-on-change-only.
- Trigger Conditions: Behavioral decision resolves implicit-build flag.
- Expected Observable Behavior: Implicit build is skipped only when all events
  in the plan are run-on-change-only.
- Protected User Interest: Batch behavior respects explicit change-only intent.
- Cancellation/Ordering/Race Constraints: Skip decision is computed from full
  events-with-hooks set.

### `WAVE-HL13-004`

- User Story: Restart requests from hook stages short-circuit subsequent stages.
- Trigger Conditions: Pre, concurrent, or post stage actions include
  `TriggerRestart`.
- Expected Observable Behavior: Pipeline stops and restart request is enqueued
  (`recompileGo` aware) instead of executing later stages/browser phase.
- Protected User Interest: Restart intent takes priority over stale follow-on
  processing.
- Cancellation/Ordering/Race Constraints: Continuation decision is evaluated
  after each stage result.

### `WAVE-HL13-005`

- User Story: Hook stage failure continuation policy is explicit.
- Trigger Conditions: Hook stage produces execution errors and no restart
  action.
- Expected Observable Behavior: `fail-open` continues pipeline; `fail-closed`
  stops pipeline at that stage.
- Protected User Interest: Applications choose strict vs permissive hook failure
  handling semantics.
- Cancellation/Ordering/Race Constraints: Restart-request stop reason takes
  precedence over stage-failure policy.

### `WAVE-HL13-006`

- User Story: Run-on-change-only implicit-build skip logging is deterministic by
  batch size.
- Trigger Conditions: Implicit build is skipped.
- Expected Observable Behavior: Single-event skip logs
  `RunOnChangeOnly: skipping implicit build phase`; multi-event skip logs
  `All events are RunOnChangeOnly, skipping implicit build phase`.
- Protected User Interest: Operator logs explain why implicit build was skipped.
- Cancellation/Ordering/Race Constraints: Skip log string is derived before
  pipeline build stage branch.

### `WAVE-HL13-007`

- User Story: Browser phase runs only when all hook stage continuation decisions
  permit it.
- Trigger Conditions: Pre/concurrent/post stage results are available.
- Expected Observable Behavior: Browser phase executes only when no stage result
  short-circuits for restart or configured fail-closed stage errors.
- Protected User Interest: Browser reload/revalidate is not emitted from aborted
  pipelines.
- Cancellation/Ordering/Race Constraints: Browser-phase gate evaluates all stage
  results.

### `WAVE-HL13-008`

- User Story: Build phase execution decomposes into deterministic sub-decisions.
- Trigger Conditions: Build-phase execution decision is derived from workset and
  static changed-path sets.
- Expected Observable Behavior: Decision independently selects compile-go, CSS
  builds, static processing mode (`none/full/changed-paths`), and framework
  public-filemap write.
- Protected User Interest: Build work does only necessary tasks for current
  event batch.
- Cancellation/Ordering/Race Constraints: Static changed-path slices are copied
  when embedded in execution decision.

### `WAVE-HL13-009`

- User Story: Browser action planning for reload/revalidate/invalidate fallback
  is deterministic.
- Trigger Conditions: Browser phase action is resolved.
- Expected Observable Behavior: Hard reload and revalidate actions map to reload
  plans; invalidate-vite action attempts invalidate only when vite is enabled
  and otherwise falls back to hard reload decision.
- Protected User Interest: Browser refresh behavior remains explicit when vite
  invalidation is unavailable.
- Cancellation/Ordering/Race Constraints: Browser execution category is derived
  from resolved post-fallback action.

## HL-14 App Process Lifecycle Behavior

### `WAVE-HL14-001`

- User Story: App process startup is explicit and observable.
- Trigger Conditions: Devserver starts app runtime for a run cycle.
- Expected Observable Behavior: The configured dist binary path is executed as a
  child process, with stdout/stderr attached to the devserver process streams.
- Protected User Interest: App logs and runtime behavior are visible during dev.
- Cancellation/Ordering/Race Constraints: Process handle assignment occurs after
  successful process start.

### `WAVE-HL14-002`

- User Story: App process management state is lazily initialized and reused.
- Trigger Conditions: App start/stop operations are requested.
- Expected Observable Behavior: A single app process manager instance is created
  on first use and reused for subsequent app lifecycle operations.
- Protected User Interest: App lifecycle behavior remains stable across
  restarts.
- Cancellation/Ordering/Race Constraints: Manager initialization is synchronized
  before process operations.

### `WAVE-HL14-003`

- User Story: App shutdown prefers graceful termination before force-kill.
- Trigger Conditions: Devserver stops a running app process.
- Expected Observable Behavior: App process receives interrupt first and is
  given a graceful-stop window before force termination is attempted.
- Protected User Interest: Clean shutdown hooks can run when possible.
- Cancellation/Ordering/Race Constraints: Graceful wait precedes kill fallback.

### `WAVE-HL14-004`

- User Story: Hung app shutdown cannot block run-cycle progress indefinitely.
- Trigger Conditions: Graceful app stop timeout elapses.
- Expected Observable Behavior: App process is force-killed and waited to
  completion after graceful timeout.
- Protected User Interest: Rebuild/restart loops remain responsive.
- Cancellation/Ordering/Race Constraints: Kill path executes only after timeout
  expiry.

### `WAVE-HL14-005`

- User Story: Benign process-termination races do not produce false failures.
- Trigger Conditions: Stop operations observe already-finished/already-killed
  process states.
- Expected Observable Behavior: Known benign termination/wait errors are ignored
  instead of being surfaced as hard failures.
- Protected User Interest: Dev lifecycle logs emphasize actionable failures.
- Cancellation/Ordering/Race Constraints: Error filtering is applied before
  joined error return.

### `WAVE-HL14-006`

- User Story: Rebuild cleanup cancels run-cycle asynchronous work first.
- Trigger Conditions: Devserver enters rebuild cleanup phase.
- Expected Observable Behavior: Run-cycle context and concurrent-no-wait hook
  lifecycle contexts are canceled before stopping app/watcher/builder.
- Protected User Interest: Prior-cycle async tasks do not leak into next cycle.
- Cancellation/Ordering/Race Constraints: Context cancellation precedes resource
  closure.

### `WAVE-HL14-007`

- User Story: Watcher and builder teardown is race-safe with event processing.
- Trigger Conditions: Rebuild cleanup closes watcher and builder.
- Expected Observable Behavior: Watcher and builder pointers are nilled under
  lock before closing underlying resources.
- Protected User Interest: Event processing cannot continue against stale
  pointers during teardown.
- Cancellation/Ordering/Race Constraints: Pointer swap-to-nil is synchronized
  before close calls.

### `WAVE-HL14-008`

- User Story: Full shutdown cleans refresh infrastructure after runtime cleanup.
- Trigger Conditions: Devserver exits run loop.
- Expected Observable Behavior: Refresh HTTP server is gracefully shut down and
  refresh-manager goroutine is canceled and joined.
- Protected User Interest: Shutdown leaves no lingering refresh listeners.
- Cancellation/Ordering/Race Constraints: Refresh manager cancel precedes wait.

## HL-15 Vite Integration Lifecycle Behavior

### `WAVE-HL15-001`

- User Story: Vite dev context is created only when configured.
- Trigger Conditions: Devserver requests Vite startup.
- Expected Observable Behavior: Vite context creation is a no-op when Vite is
  disabled and initializes context when enabled.
- Protected User Interest: Non-Vite projects do not pay Vite lifecycle costs.
- Cancellation/Ordering/Race Constraints: Vite context assignment is
  lock-guarded.

### `WAVE-HL15-002`

- User Story: Vite startup ordering follows build completion.
- Trigger Conditions: Run-cycle runtime starts after successful build.
- Expected Observable Behavior: Vite starts after build outputs exist and before
  app start when Vite is enabled and not already running.
- Protected User Interest: Vite serves current artifacts and reconnects to a
  valid app runtime.
- Cancellation/Ordering/Race Constraints: Vite-start decision is evaluated once
  per cycle runtime start.

### `WAVE-HL15-003`

- User Story: Vite stop always clears active context state.
- Trigger Conditions: Devserver stops or cycles Vite.
- Expected Observable Behavior: Existing Vite context is cleaned up and pointer
  is set to nil.
- Protected User Interest: Vite restarts do not retain stale process state.
- Cancellation/Ordering/Race Constraints: Cleanup and pointer clear are
  lock-guarded.

### `WAVE-HL15-004`

- User Story: Vite cycling only occurs when Vite is active and configured.
- Trigger Conditions: Reload orchestration requests `cycleVite=true`.
- Expected Observable Behavior: Cycle is skipped when Vite is disabled or no
  active context exists.
- Protected User Interest: Cycle requests do not produce false-positive side
  effects in non-Vite states.
- Cancellation/Ordering/Race Constraints: Active-context check gates cycle.

### `WAVE-HL15-005`

- User Story: Vite cycle readiness is verified before declaring success.
- Trigger Conditions: Vite cycle restarts process.
- Expected Observable Behavior: Cycle reports success only when stop/start
  succeeds and readiness probe passes.
- Protected User Interest: Browser reload sequencing is based on actual Vite
  readiness.
- Cancellation/Ordering/Race Constraints: Readiness check runs after restart.

### `WAVE-HL15-006`

- User Story: Vite readiness probing tolerates loopback host variation.
- Trigger Conditions: Devserver waits for Vite readiness.
- Expected Observable Behavior: Probe attempts both `127.0.0.1` and `localhost`
  for `/@vite/client`.
- Protected User Interest: Vite readiness is robust across host resolution
  differences.
- Cancellation/Ordering/Race Constraints: Probe list is deduped before polling.

### `WAVE-HL15-007`

- User Story: Vite filemap invalidation endpoint behavior is explicit.
- Trigger Conditions: Browser phase requests Vite invalidate action.
- Expected Observable Behavior: Devserver issues POST to
  `/__vorma_invalidate_filemap` on Vite port and treats non-200 as failure.
- Protected User Interest: Invalidate-vite behavior is deterministic and
  diagnosable.
- Cancellation/Ordering/Race Constraints: Request is bounded by timeout context.

### `WAVE-HL15-008`

- User Story: Cycle-Vite reload flow avoids double reload signaling.
- Trigger Conditions: Reload orchestration runs with `cycleVite=true`.
- Expected Observable Behavior: When Vite cycle is successfully applied, Wave
  does not additionally broadcast hard-reload payload.
- Protected User Interest: Browser avoids redundant reload events.
- Cancellation/Ordering/Race Constraints: Broadcast decision follows cycle
  outcome evaluation.

### `WAVE-HL15-009`

- User Story: Production Vite build is optional and config-driven.
- Trigger Conditions: Tooling runs Vite production build path.
- Expected Observable Behavior: Vite prod build runs only when Vite is enabled
  and uses configured package-manager command, cwd, manifest target, and outdir.
- Protected User Interest: Vite output generation is predictable and explicit.
- Cancellation/Ordering/Race Constraints: Disabled-Vite path returns without
  side effects.

## HL-16 Refresh Server and Websocket Protocol Semantics

### `WAVE-HL16-001`

- User Story: Refresh server lifecycle is browser-mode scoped.
- Trigger Conditions: Devserver starts refresh infrastructure.
- Expected Observable Behavior: Refresh server and manager are started only when
  browser mode is enabled.
- Protected User Interest: Server-only mode avoids unnecessary websocket
  infrastructure.
- Cancellation/Ordering/Race Constraints: Browser-mode gate runs before listener
  setup.

### `WAVE-HL16-002`

- User Story: Refresh listener binds to an available port even when default is
  unavailable.
- Trigger Conditions: Refresh server attempts to bind configured/default port.
- Expected Observable Behavior: If requested port bind fails, fallback bind to
  ephemeral port is attempted.
- Protected User Interest: Dev refresh channel still starts in busy-port
  environments.
- Cancellation/Ordering/Race Constraints: Fallback bind occurs only after
  initial bind failure.

### `WAVE-HL16-003`

- User Story: Effective refresh port is published for runtime script consumers.
- Trigger Conditions: Refresh listener starts successfully.
- Expected Observable Behavior: Actual bound refresh port is persisted via
  refresh-port environment setter.
- Protected User Interest: Browser refresh script targets correct websocket
  port.
- Cancellation/Ordering/Race Constraints: Port publication happens before
  serving begins.

### `WAVE-HL16-004`

- User Story: Refresh HTTP endpoints expose explicit websocket and script
  contracts.
- Trigger Conditions: Refresh mux handles requests.
- Expected Observable Behavior: `/events` upgrades websocket;
  `/get-refresh-script-inner` returns JS refresh script payload; both set
  permissive CORS header.
- Protected User Interest: Browser integration has a stable endpoint contract.
- Cancellation/Ordering/Race Constraints: Endpoint handlers are installed at
  refresh-server startup.

### `WAVE-HL16-005`

- User Story: Refresh script endpoint reflects framework runtime naming
  overrides.
- Trigger Conditions: Refresh script endpoint response is generated.
- Expected Observable Behavior: Script uses parsed-config resolved runtime
  function/element naming values.
- Protected User Interest: Framework-custom runtime names are honored in browser
  integration.
- Cancellation/Ordering/Race Constraints: Parsed-config lookup occurs per
  request.

### `WAVE-HL16-006`

- User Story: Refresh client manager remains non-blocking under
  connect/disconnect pressure.
- Trigger Conditions: Register/unregister/broadcast events flow through manager.
- Expected Observable Behavior: Register/unregister channels are buffered and
  slow-client notify sends are skipped rather than blocking broadcaster.
- Protected User Interest: Refresh pipeline remains responsive under uneven
  client consumption.
- Cancellation/Ordering/Race Constraints: Per-client notify send uses
  non-blocking select.

### `WAVE-HL16-007`

- User Story: Refresh manager shutdown cleans connections and drains channels.
- Trigger Conditions: Manager context is canceled.
- Expected Observable Behavior: All client notify channels/connections are
  closed and pending register/unregister/broadcast channels are drained.
- Protected User Interest: Shutdown avoids goroutine/channel leaks.
- Cancellation/Ordering/Race Constraints: Cleanup runs before manager done
  signal.

### `WAVE-HL16-008`

- User Story: Websocket acceptance rejects new clients during shutdown.
- Trigger Conditions: `/events` receives a websocket upgrade request while
  manager context is canceled.
- Expected Observable Behavior: Request is rejected with `503` and no websocket
  registration occurs.
- Protected User Interest: Shutdown behavior is predictable and race-safe.
- Cancellation/Ordering/Race Constraints: Shutdown check gates upgrade.

### `WAVE-HL16-009`

- User Story: Websocket client lifecycle unregisters safely on read/write exit.
- Trigger Conditions: Client read/write loops encounter disconnect, context
  cancellation, or write failure.
- Expected Observable Behavior: Client unregister is attempted and connection is
  closed without blocking on full/closing channels.
- Protected User Interest: Dead clients do not remain registered.
- Cancellation/Ordering/Race Constraints: Unregister paths use guarded
  non-blocking sends.

### `WAVE-HL16-010`

- User Story: Refresh server shutdown is graceful and bounded.
- Trigger Conditions: Devserver stops refresh server.
- Expected Observable Behavior: Refresh HTTP server shuts down with
  timeout-bound context and clears server pointer on success.
- Protected User Interest: Shutdown completes without indefinite wait.
- Cancellation/Ordering/Race Constraints: Timeout context bounds shutdown call.

## HL-17 Browser Reload/Revalidate/Hot-CSS Behavior

### `WAVE-HL17-001`

- User Story: Rebuild overlay signaling is explicit and gated.
- Trigger Conditions: Devserver enters rebuilding phase with browser refresh
  enabled.
- Expected Observable Behavior: `changeType=rebuilding` payload is broadcast
  only when refresh context is active.
- Protected User Interest: Browser overlay appears only for active rebuild
  sessions.
- Cancellation/Ordering/Race Constraints: Broadcast is canceled if refresh
  context closes.

### `WAVE-HL17-002`

- User Story: Reload orchestration respects app/vite readiness settings.
- Trigger Conditions: Reload plan is executed.
- Expected Observable Behavior: App readiness wait and Vite readiness wait are
  applied according to reload plan flags.
- Protected User Interest: Reload emits after required dependencies are ready.
- Cancellation/Ordering/Race Constraints: Readiness waits execute before payload
  broadcast decision.

### `WAVE-HL17-003`

- User Story: Invalidate-vite action has deterministic fallback semantics.
- Trigger Conditions: Browser decision selects invalidate-vite.
- Expected Observable Behavior: Vite invalidate is attempted only when Vite is
  enabled; on failure or unavailable Vite, action falls back to hard reload.
- Protected User Interest: Browser update proceeds even when invalidate endpoint
  is unavailable.
- Cancellation/Ordering/Race Constraints: Fallback decision is applied before
  execution category dispatch.

### `WAVE-HL17-004`

- User Story: Browser reload payloads map directly to resolved action category.
- Trigger Conditions: Browser execution category resolves to reload.
- Expected Observable Behavior: Hard reload uses `changeType=other`; revalidate
  uses `changeType=revalidate`.
- Protected User Interest: Client-side script behavior remains action-aligned.
- Cancellation/Ordering/Race Constraints: Payload mapping is derived before
  broadcast.

### `WAVE-HL17-005`

- User Story: Hot-reload CSS payload generation is freshness-aware.
- Trigger Conditions: Browser execution category resolves to hot-reload-css.
- Expected Observable Behavior: Critical and normal CSS payloads are emitted
  only when corresponding fresh outputs are available; unavailable outputs are
  skipped with warning.
- Protected User Interest: Browser does not receive stale CSS payloads when
  freshness is required.
- Cancellation/Ordering/Race Constraints: Availability checks run before payload
  append.

### `WAVE-HL17-006`

- User Story: Refresh script preserves scroll position across hard reloads.
- Trigger Conditions: Refresh script handles `changeType=other`.
- Expected Observable Behavior: Current scroll position is saved before reload
  and restored after reconnect using session storage key.
- Protected User Interest: Hard reload keeps user viewport continuity.
- Cancellation/Ordering/Race Constraints: Stored scroll state is removed after
  restore attempt.

### `WAVE-HL17-007`

- User Story: Normal CSS updates hot-swap stylesheet link in-place.
- Trigger Conditions: Refresh script handles `changeType=normal`.
- Expected Observable Behavior: New stylesheet link is inserted and old link is
  removed after load completion when present.
- Protected User Interest: Non-critical CSS changes apply without full page
  reload and with minimal flash.
- Cancellation/Ordering/Race Constraints: Old-link removal is deferred until
  new-link load callback.

### `WAVE-HL17-008`

- User Story: Critical CSS updates replace the critical style element in-place.
- Trigger Conditions: Refresh script handles `changeType=critical`.
- Expected Observable Behavior: Base64 payload is decoded and written to a new
  critical style element replacing existing element when present.
- Protected User Interest: Critical-style updates apply immediately without full
  reload.
- Cancellation/Ordering/Race Constraints: Replacement preserves element ID
  contract.

### `WAVE-HL17-009`

- User Story: Revalidate payload invokes configured client revalidate hook.
- Trigger Conditions: Refresh script handles `changeType=revalidate`.
- Expected Observable Behavior: Script calls resolved browser revalidate
  function when present and removes rebuilding overlay after resolution or
  failure.
- Protected User Interest: App can perform data revalidation without full
  reload.
- Cancellation/Ordering/Race Constraints: Overlay cleanup executes in both
  success and failure branches.

### `WAVE-HL17-010`

- User Story: Refresh websocket disconnection falls back to hard reload.
- Trigger Conditions: Websocket error or close events fire.
- Expected Observable Behavior: Browser reloads page on websocket close/error;
  beforeunload disables auto-reload by overriding close handler.
- Protected User Interest: Browser resynchronizes with devserver after refresh
  channel loss.
- Cancellation/Ordering/Race Constraints: Beforeunload close path suppresses
  duplicate reload during intentional navigation.

## HL-18 Readiness/Wait/Timeout Policy Behavior

### `WAVE-HL18-001`

- User Story: Readiness polling uses explicit bounded retry policy.
- Trigger Conditions: App or Vite readiness wait is requested.
- Expected Observable Behavior: Polling uses bounded attempts, linear delay
  growth, maximum total wait budget, and per-request timeout.
- Protected User Interest: Readiness checks are predictable and bounded.
- Cancellation/Ordering/Race Constraints: Delay accumulation gates continuation.

### `WAVE-HL18-002`

- User Story: Multi-endpoint readiness checks avoid redundant probes.
- Trigger Conditions: Readiness wait receives multiple candidate URLs.
- Expected Observable Behavior: Empty URLs are dropped and duplicates are
  removed before probe loop.
- Protected User Interest: Readiness probing avoids unnecessary duplicate
  network calls.
- Cancellation/Ordering/Race Constraints: URL normalization occurs before
  polling.

### `WAVE-HL18-003`

- User Story: Readiness success is strict and explicit.
- Trigger Conditions: Probe responses are evaluated.
- Expected Observable Behavior: Readiness returns success only when an endpoint
  responds with HTTP 200.
- Protected User Interest: Rebuild sequencing waits for healthy runtime state.
- Cancellation/Ordering/Race Constraints: Response bodies are always closed
  after probe attempts.

### `WAVE-HL18-004`

- User Story: App readiness probe URL composition is deterministic.
- Trigger Conditions: App readiness URL is resolved.
- Expected Observable Behavior: App probe URL uses loopback host, resolved app
  port, and configured healthcheck endpoint path.
- Protected User Interest: App-ready polling targets the intended local runtime.
- Cancellation/Ordering/Race Constraints: URL composition precedes probe loop.

### `WAVE-HL18-005`

- User Story: Vite readiness probe supports host alias compatibility.
- Trigger Conditions: Vite readiness URLs are resolved.
- Expected Observable Behavior: Probe candidate list includes both loopback IP
  and localhost hostnames.
- Protected User Interest: Vite readiness remains stable across host binding
  differences.
- Cancellation/Ordering/Race Constraints: Candidate list generation is pure and
  deterministic.

### `WAVE-HL18-006`

- User Story: Hook timeout resolution follows stage-policy with per-hook
  overrides.
- Trigger Conditions: Hook execution plan computes timeout durations.
- Expected Observable Behavior: Stage timeout defaults apply unless per-hook
  override is provided; disable-stage flags suppress stage timeout.
- Protected User Interest: Hook latency bounds are configurable without
  ambiguity.
- Cancellation/Ordering/Race Constraints: Timeout resolution happens per
  execution plan at runtime.

### `WAVE-HL18-007`

- User Story: Build-hook timeout policy is mode-specific.
- Trigger Conditions: Build hooks execute in dev or production mode.
- Expected Observable Behavior: Build hook timeout derives from corresponding
  mode-specific core timeout field.
- Protected User Interest: Dev/prod hook timing control remains explicit.
- Cancellation/Ordering/Race Constraints: Timeout derivation occurs before hook
  execution context creation.

### `WAVE-HL18-008`

- User Story: Optional-timeout execution context behavior is explicit.
- Trigger Conditions: Execution context is derived with optional timeout.
- Expected Observable Behavior: Zero/disabled timeout reuses parent context;
  positive timeout creates timeout-bound derived context and cancel function.
- Protected User Interest: Context cancellation semantics are predictable.
- Cancellation/Ordering/Race Constraints: Timeout-wrapped context is scoped to
  execution boundary.

## HL-19 Static Serving and Middleware Behavior

### `WAVE-HL19-001`

- User Story: Public URL resolver leaves passthrough URLs untouched.
- Trigger Conditions: Public URL resolution receives absolute/protocol URL
  forms.
- Expected Observable Behavior: Passthrough schemes/prefixes are returned
  without filemap rewrite.
- Protected User Interest: External/data/blob URLs remain valid.
- Cancellation/Ordering/Race Constraints: Passthrough check runs before filemap
  lookup.

### `WAVE-HL19-002`

- User Story: Public-asset path classification is prefix-aware and path-cleaned.
- Trigger Conditions: Public-asset check runs for request path.
- Expected Observable Behavior: Request path is cleaned, validated under public
  prefix, and rejects root/empty/dot asset paths.
- Protected User Interest: Static serving only targets explicit public assets.
- Cancellation/Ordering/Race Constraints: Prefix gate precedes filesystem stat.

### `WAVE-HL19-003`

- User Story: Public-asset existence requires concrete file presence.
- Trigger Conditions: Public-asset existence check probes filesystem.
- Expected Observable Behavior: Only existing non-directory entries in public FS
  are treated as assets; missing/stat errors return false.
- Protected User Interest: Middleware does not claim non-existent asset routes.
- Cancellation/Ordering/Race Constraints: Directory results are rejected after
  stat.

### `WAVE-HL19-004`

- User Story: Static middleware pass-through semantics are explicit.
- Trigger Conditions: ServeStatic middleware handles request.
- Expected Observable Behavior: Asset requests are served by static handler;
  non-asset requests are delegated to next handler.
- Protected User Interest: Application routes are not shadowed by static
  handler.
- Cancellation/Ordering/Race Constraints: Asset predicate is evaluated per
  request before delegation.

### `WAVE-HL19-005`

- User Story: Immutable static-handler mode applies deterministic cache header.
- Trigger Conditions: Static handler is requested with `immutable=true`.
- Expected Observable Behavior: Static responses include
  `Cache-Control: public, max-age=31536000, immutable`.
- Protected User Interest: Browser caching behavior is explicit for hashed
  assets.
- Cancellation/Ordering/Race Constraints: Header injection wraps underlying file
  server handler.

### `WAVE-HL19-006`

- User Story: Static file serving strips configured public prefix.
- Trigger Conditions: Static handler serves a request path.
- Expected Observable Behavior: Request path is stripped by configured public
  prefix before filesystem lookup.
- Protected User Interest: Asset URLs map correctly under non-root prefixes.
- Cancellation/Ordering/Race Constraints: Prefix strip occurs in handler adapter
  before file-server lookup.

### `WAVE-HL19-007`

- User Story: Favicon middleware uses hashed public filemap contract.
- Trigger Conditions: Favicon middleware handles `GET`/`HEAD /favicon.ico`.
- Expected Observable Behavior: Middleware resolves `favicon.ico` via public
  filemap and redirects when found; otherwise returns `404`.
- Protected User Interest: Favicon behavior matches hashed/unhashed asset map.
- Cancellation/Ordering/Race Constraints: Missing filemap/read failures are
  treated as not found.

## HL-20 Lock/Single-Runner and Command Serialization Behavior

### `WAVE-HL20-001`

- User Story: Dev mode enforces one running Wave instance per project dist root.
- Trigger Conditions: `RunDev` starts.
- Expected Observable Behavior: Dev lock is acquired before startup proceeds and
  released on function exit.
- Protected User Interest: Concurrent dev instances cannot corrupt shared dist
  outputs.
- Cancellation/Ordering/Race Constraints: Lock acquisition precedes build/watch
  initialization.

### `WAVE-HL20-002`

- User Story: Lock location is deterministic and project-scoped.
- Trigger Conditions: Dev lock path is derived.
- Expected Observable Behavior: Lock file path resolves under
  `dist/static/.wave-dev.lock`.
- Protected User Interest: Lock scope matches build artifact scope.
- Cancellation/Ordering/Race Constraints: Dist static path resolution happens
  before lock creation.

### `WAVE-HL20-003`

- User Story: Lock acquisition is race-safe under concurrent starters.
- Trigger Conditions: Multiple processes attempt lock acquisition.
- Expected Observable Behavior: Lock file creation uses exclusive create
  semantics and writes current PID to lock file.
- Protected User Interest: Lock ownership is unambiguous.
- Cancellation/Ordering/Race Constraints: PID write/close complete before lock
  is considered acquired.

### `WAVE-HL20-004`

- User Story: Active lock owner detection surfaces actionable error details.
- Trigger Conditions: Existing lock file PID resolves to running process.
- Expected Observable Behavior: Acquisition fails with lock-held error including
  held PID metadata.
- Protected User Interest: Operators can identify conflicting process instance.
- Cancellation/Ordering/Race Constraints: Running-process check precedes stale
  lock recovery.

### `WAVE-HL20-005`

- User Story: Stale lock recovery avoids deleting newly replaced lock files.
- Trigger Conditions: Lock recovery identifies stale/non-running lock owner.
- Expected Observable Behavior: Lock file contents are re-read for equality
  before removal; changed content aborts deletion.
- Protected User Interest: Recovery path does not remove another process's fresh
  lock.
- Cancellation/Ordering/Race Constraints: Equality check gates stale-file
  remove.

### `WAVE-HL20-006`

- User Story: Invalid lock content is treated conservatively before stale age
  threshold.
- Trigger Conditions: Lock file PID cannot be parsed.
- Expected Observable Behavior: Non-parseable lock is treated as held until
  stale threshold is exceeded; only stale invalid lock is recoverable.
- Protected User Interest: Corrupt lock files do not cause unsafe concurrent
  runs.
- Cancellation/Ordering/Race Constraints: Stale-threshold check gates
  invalid-lock recovery.

### `WAVE-HL20-007`

- User Story: Lock release is idempotent for already-removed files.
- Trigger Conditions: Release runs when lock file is missing.
- Expected Observable Behavior: Missing lock-file removal does not fail release.
- Protected User Interest: Cleanup paths remain robust across partial teardown.
- Cancellation/Ordering/Race Constraints: Not-exist errors are ignored on
  release.

### `WAVE-HL20-008`

- User Story: Dist static cleanup preserves lock files.
- Trigger Conditions: Full file processing cleans dist/static contents.
- Expected Observable Behavior: Cleanup removes static entries except wave lock
  files.
- Protected User Interest: Active dev lock coordination survives full file
  processing.
- Cancellation/Ordering/Race Constraints: Lock-file predicate is checked per
  static entry before removal.

### `WAVE-HL20-009`

- User Story: Restart intent queue merges non-config restart requests safely.
- Trigger Conditions: New restart request arrives while another request is
  already queued.
- Expected Observable Behavior: Pending and incoming non-config requests are
  merged with OR semantics for recompile-go; config restart takes precedence.
- Protected User Interest: Restart execution reflects strongest requested action
  without queue growth.
- Cancellation/Ordering/Race Constraints: Merge resolution occurs under queue
  mutex.

### `WAVE-HL20-010`

- User Story: Build-retry wait mode preserves first restart intent determinism.
- Trigger Conditions: Restart requests arrive while waiting for build retry.
- Expected Observable Behavior: First queued request controls next retry pass
  and is not upgraded by later requests in retry-wait mode.
- Protected User Interest: Retry behavior remains deterministic while build is
  already broken.
- Cancellation/Ordering/Race Constraints: Retry-wait mode bypasses merge logic.

## HL-21 CLI and Run-Mode Contracts

### `WAVE-HL21-001`

- User Story: CLI argument parsing for Wave run modes is explicit.
- Trigger Conditions: CLI options are parsed.
- Expected Observable Behavior: `-dev`, `-hook`, and `-no-binary` flags are
  recognized and parse errors are surfaced.
- Protected User Interest: Run-mode selection is predictable from explicit
  flags.
- Cancellation/Ordering/Race Constraints: Parse failure aborts run-mode
  dispatch.

### `WAVE-HL21-002`

- User Story: Hook-only mode runs caller-provided hook without invoking
  build/dev loops.
- Trigger Conditions: Hook-only flag is set and hook callback is provided.
- Expected Observable Behavior: Hook callback executes with current dev-flag
  value and normal build/dev paths are skipped.
- Protected User Interest: Custom hook workflows run in isolation.
- Cancellation/Ordering/Race Constraints: Hook execution branch precedes mode
  dispatch.

### `WAVE-HL21-003`

- User Story: Dev mode CLI path delegates to devserver runtime.
- Trigger Conditions: Dev flag is set.
- Expected Observable Behavior: CLI dispatch invokes `RunDev` and does not run
  production build path.
- Protected User Interest: Dev and build modes are clearly separated.
- Cancellation/Ordering/Race Constraints: Dev branch is evaluated before
  production build branch.

### `WAVE-HL21-004`

- User Story: Non-dev CLI path runs build with explicit compile-go toggle.
- Trigger Conditions: Dev flag is unset.
- Expected Observable Behavior: Build executes with `CompileGo` enabled unless
  `-no-binary` is set.
- Protected User Interest: Users can choose artifact-only build vs binary build.
- Cancellation/Ordering/Race Constraints: Compile-go option is resolved at
  dispatch time.

### `WAVE-HL21-005`

- User Story: Default CLI entrypoint uses process arg vector.
- Trigger Conditions: `BuildWaveWithHook` convenience function is used.
- Expected Observable Behavior: CLI options are parsed from `os.Args[1:]`.
- Protected User Interest: Standard CLI invocation behavior is preserved.
- Cancellation/Ordering/Race Constraints: Convenience wrapper delegates directly
  to option-based implementation.

### `WAVE-HL21-006`

- User Story: Build-only convenience entrypoint has no custom hook side effects.
- Trigger Conditions: `BuildWave` convenience function is used.
- Expected Observable Behavior: Build dispatch behaves as
  `BuildWaveWithHook(..., nil)`.
- Protected User Interest: Convenience entrypoint stays behaviorally minimal.
- Cancellation/Ordering/Race Constraints: Wrapper does not alter parsed options.

### `WAVE-HL21-007`

- User Story: Missing logger input receives deterministic default logger.
- Trigger Conditions: CLI/build/dev entrypoints receive nil logger.
- Expected Observable Behavior: Colorlog-based default logger instance is used.
- Protected User Interest: Diagnostic output exists even without caller logger.
- Cancellation/Ordering/Race Constraints: Logger normalization occurs before run
  operations start.

## HL-22 Error Handling and Non-Mutation Safety Guarantees

### `WAVE-HL22-001`

- User Story: Config reload never mutates active runtime config on parse/load
  failure.
- Trigger Conditions: Config reload attempts to load or parse updated config
  file.
- Expected Observable Behavior: Reload returns error and keeps existing config
  state unchanged.
- Protected User Interest: Temporary invalid file edits do not corrupt live
  runtime behavior.
- Cancellation/Ordering/Race Constraints: Config assignment occurs only after
  successful load/parse.

### `WAVE-HL22-002`

- User Story: Config reload never mutates active runtime config on validation
  failure.
- Trigger Conditions: New parsed config fails full validation.
- Expected Observable Behavior: Validation error is returned and previous config
  remains active.
- Protected User Interest: Invalid semantic edits do not poison running dev
  session config state.
- Cancellation/Ordering/Race Constraints: Validation gate precedes config swap.

### `WAVE-HL22-003`

- User Story: Reloaded config preserves framework-injected runtime-only fields.
- Trigger Conditions: Config reload succeeds.
- Expected Observable Behavior: Framework watch patterns, ignored patterns,
  schema extensions, hooks, overlays, and runtime browser naming fields are
  copied from prior config into new config.
- Protected User Interest: Framework integrations continue working across
  reloads.
- Cancellation/Ordering/Race Constraints: Field copy occurs before new config is
  installed.

### `WAVE-HL22-004`

- User Story: Event processing safely no-ops when critical runtime dependencies
  are absent.
- Trigger Conditions: Event processing executes while watcher or builder is nil.
- Expected Observable Behavior: Processing returns without attempting
  classification/build/hook execution.
- Protected User Interest: Transitional lifecycle states avoid nil-driven
  mutation or panic.
- Cancellation/Ordering/Race Constraints: Dependency guard runs before all plan
  stages.

### `WAVE-HL22-005`

- User Story: Build-phase failures stop downstream pipeline actions.
- Trigger Conditions: Build phase returns error in combined build/concurrent
  stage.
- Expected Observable Behavior: Pipeline logs stop reason and does not execute
  subsequent post-build browser phase.
- Protected User Interest: Browser refresh is not emitted for failed builds.
- Cancellation/Ordering/Race Constraints: Build failure cancels shared stage
  context.

### `WAVE-HL22-006`

- User Story: Public parsed-config exposure is defensive and stable.
- Trigger Conditions: Runtime tooling requests parsed config snapshot.
- Expected Observable Behavior: Public accessor returns cloned config snapshot
  without unstable internal callback/schema fields.
- Protected User Interest: External callers cannot mutate live runtime config.
- Cancellation/Ordering/Race Constraints: Clone happens per accessor call.

### `WAVE-HL22-007`

- User Story: Buildtime parsed-config accessor includes required framework
  internals while remaining defensive.
- Trigger Conditions: Build/dev tooling requests buildtime config snapshot.
- Expected Observable Behavior: Snapshot includes framework schema extensions
  and build callbacks while remaining a cloned object.
- Protected User Interest: Tooling can access build internals without mutating
  runtime-owned config object.
- Cancellation/Ordering/Race Constraints: Buildtime clone path is explicit and
  isolated from public clone path.

### `WAVE-HL22-008`

- User Story: Raw config byte accessor is mutation-safe.
- Trigger Conditions: `RawConfigJSON` is called.
- Expected Observable Behavior: Returned byte slice is a cloned copy of stored
  config bytes.
- Protected User Interest: Caller mutation cannot alter runtime config bytes.
- Cancellation/Ordering/Race Constraints: Clone is returned on each call.

### `WAVE-HL22-009`

- User Story: Public filemap accessor is mutation-safe.
- Trigger Conditions: `GetPublicFileMap` is called.
- Expected Observable Behavior: Returned map is cloned from internal cached map.
- Protected User Interest: Caller mutation cannot corrupt shared URL lookup map.
- Cancellation/Ordering/Race Constraints: Clone is created per call.

### `WAVE-HL22-010`

- User Story: Hook callback panics are converted to surfaced execution errors.
- Trigger Conditions: Hook callback panics during execution.
- Expected Observable Behavior: Panic is recovered and returned as callback
  execution error with stage attribution behavior.
- Protected User Interest: Hook failures remain visible and do not crash
  devserver.
- Cancellation/Ordering/Race Constraints: Recovery wrapper encloses callback
  execution boundary.

## HL-23 Cross-Platform Path and OS Behavior

### `WAVE-HL23-001`

- User Story: Dist binary path respects operating-system executable naming.
- Trigger Conditions: Dist binary path is derived.
- Expected Observable Behavior: Binary name is `main.exe` on Windows and `main`
  on non-Windows.
- Protected User Interest: Build/run commands target valid platform binary path.
- Cancellation/Ordering/Race Constraints: OS check occurs during path
  derivation.

### `WAVE-HL23-002`

- User Story: Shared relative-path constants remain fs.Sub-compatible.
- Trigger Conditions: `RelPaths` helpers are used by runtime/tooling.
- Expected Observable Behavior: Relative paths use forward slashes and omit
  leading slash.
- Protected User Interest: Path constants work consistently with `fs.FS`
  operations across platforms.
- Cancellation/Ordering/Race Constraints: Path helpers are pure constants.

### `WAVE-HL23-003`

- User Story: Trim/clean path normalization removes incidental formatting noise.
- Trigger Conditions: Path normalization helpers run on user-provided paths.
- Expected Observable Behavior: Leading/trailing whitespace is trimmed and empty
  inputs normalize to empty path sentinel.
- Protected User Interest: Equivalent path inputs map to same normalized form.
- Cancellation/Ordering/Race Constraints: Trim occurs before clean/abs
  resolution.

### `WAVE-HL23-004`

- User Story: Location-equivalence checks handle symlinks and missing-file
  aliases.
- Trigger Conditions: Path-equivalence is used for config-file event detection.
- Expected Observable Behavior: Paths are canonicalized with symlink resolution,
  including parent-symlink fallback for missing targets.
- Protected User Interest: Config-change detection remains stable across path
  spelling variants.
- Cancellation/Ordering/Race Constraints: Canonicalization runs before equality
  compare.

### `WAVE-HL23-005`

- User Story: Absolute directory derivation handles both file and directory
  inputs.
- Trigger Conditions: Config directory watch path is derived from config file
  location.
- Expected Observable Behavior: Existing directory input returns itself; file or
  non-existing path returns parent directory.
- Protected User Interest: Watcher can subscribe to config directory reliably.
- Cancellation/Ordering/Race Constraints: Directory stat check precedes
  parent-dir fallback.

### `WAVE-HL23-006`

- User Story: Process-running checks use platform-specific system calls.
- Trigger Conditions: Lock ownership validation checks a stored PID.
- Expected Observable Behavior: Unix uses signal-0 check; Windows uses process
  handle open query.
- Protected User Interest: Lock ownership detection behaves correctly on each
  OS.
- Cancellation/Ordering/Race Constraints: Platform selection is compile-time via
  build tags.

### `WAVE-HL23-007`

- User Story: Watch-root and config path normalization is explicit.
- Trigger Conditions: Watch patterns/config locations are normalized.
- Expected Observable Behavior: Relative patterns are anchored at normalized
  watch root, and absolute paths are slash-normalized for matching.
- Protected User Interest: Watch matching is stable across path separators and
  input styles.
- Cancellation/Ordering/Race Constraints: Normalization runs before pattern
  match indexing.

## HL-24 Framework Integration Runtime Contracts

### `WAVE-HL24-001`

- User Story: Framework watch pattern registration is isolated from caller
  mutation.
- Trigger Conditions: Framework adds watch patterns at runtime.
- Expected Observable Behavior: Watch-pattern definitions are defensively cloned
  into runtime config state.
- Protected User Interest: Framework integration remains stable despite caller
  slice mutation.
- Cancellation/Ordering/Race Constraints: Clone occurs before append.

### `WAVE-HL24-002`

- User Story: Framework ignored patterns and public filemap output dir are
  runtime-configurable.
- Trigger Conditions: Framework sets ignored patterns or filemap output dir.
- Expected Observable Behavior: Values are persisted in parsed config and used
  by watcher/build phases.
- Protected User Interest: Frameworks can extend watch/build behavior without
  forking core defaults.
- Cancellation/Ordering/Race Constraints: Runtime fields are read during watcher
  plan/build decision generation.

### `WAVE-HL24-003`

- User Story: Framework build hooks and overlay providers are first-class build
  pipeline inputs.
- Trigger Conditions: Framework sets dev/prod hook commands, hook runner, or Go
  overlay provider.
- Expected Observable Behavior: Build pipeline executes configured framework
  hooks and overlay lifecycle per mode.
- Protected User Interest: Framework-generated artifacts and compile overlays
  are integrated deterministically.
- Cancellation/Ordering/Race Constraints: Framework hook stage follows user hook
  stage in run order.

### `WAVE-HL24-004`

- User Story: Browser runtime namespace and function IDs are configurable with
  safe defaults.
- Trigger Conditions: Framework sets browser runtime naming overrides.
- Expected Observable Behavior: Public-filemap bootstrap and refresh script use
  resolved naming accessors (default when unset, override when set).
- Protected User Interest: Integrations avoid global-symbol collisions while
  preserving stable defaults.
- Cancellation/Ordering/Race Constraints: Accessors are pure and non-mutating.

### `WAVE-HL24-005`

- User Story: Framework schema extensions flow into generated config schema.
- Trigger Conditions: Builder writes schema after framework sections are
  registered.
- Expected Observable Behavior: Registered framework schema entries are merged
  into top-level schema properties output.
- Protected User Interest: Framework config fields remain discoverable/typed.
- Cancellation/Ordering/Race Constraints: Merge occurs before schema marshaling.

### `WAVE-HL24-006`

- User Story: Reload preserves framework runtime fields not persisted in base
  config file.
- Trigger Conditions: Config reload installs a newly parsed config.
- Expected Observable Behavior: Framework runtime fields are copied from
  existing config into new config before assignment.
- Protected User Interest: Reload does not silently drop framework integrations.
- Cancellation/Ordering/Race Constraints: Copy operation precedes config swap.

### `WAVE-HL24-007`

- User Story: Framework callbacks receive deterministic hook context contracts.
- Trigger Conditions: Callback hooks execute through event pipeline.
- Expected Observable Behavior: Hook context includes normalized file path and
  batch-aware changed-path metadata under predictable cloning semantics.
- Protected User Interest: Framework callback behavior is reproducible and safe.
- Cancellation/Ordering/Race Constraints: Context clone occurs per callback
  execution path.

### `WAVE-HL24-008`

- User Story: Framework integration semantics remain adapter-generic.
- Trigger Conditions: Framework integration surfaces are consumed by any current
  or future UI adapter/framework layer.
- Expected Observable Behavior: Observable refresh/revalidate/public-URL runtime
  semantics remain equivalent across integrations unless explicitly intended
  otherwise.
- Protected User Interest: Application behavior does not vary unexpectedly by
  adapter choice.
- Cancellation/Ordering/Race Constraints: Integration differences must not
  change user-observable semantics without explicit contract change.

## HL-25 Development vs Production Behavioral Deltas

### `WAVE-HL25-001`

- User Story: Runtime mode switching is explicit and environment-based.
- Trigger Conditions: Mode helpers are used.
- Expected Observable Behavior: Dev mode is active only when
  `WAVE_MODE=development`; dev entrypoints set this mode explicitly.
- Protected User Interest: Mode-dependent behavior is deterministic and
  inspectable.
- Cancellation/Ordering/Race Constraints: Mode flag is set before dev lifecycle
  startup.

### `WAVE-HL25-002`

- User Story: Cache policy differs intentionally between dev and production.
- Trigger Conditions: Cache and cache-map accessors are invoked.
- Expected Observable Behavior: Dev mode recomputes on each access; production
  memoizes successful values.
- Protected User Interest: Dev reflects latest files, production minimizes
  overhead.
- Cancellation/Ordering/Race Constraints: Mode check occurs per cache read.

### `WAVE-HL25-003`

- User Story: Port resolution policy differs between dev auto-allocation and
  fixed mode.
- Trigger Conditions: Port resolver resolves app port.
- Expected Observable Behavior: Dev resolves/sets a free port once when needed;
  fixed/non-dev paths use configured port or fallback default.
- Protected User Interest: Dev avoids port conflicts while production stays
  deterministic.
- Cancellation/Ordering/Race Constraints: Resolution is once-per-resolver.

### `WAVE-HL25-004`

- User Story: Base filesystem source differs by mode.
- Trigger Conditions: Runtime base FS initializes.
- Expected Observable Behavior: Dev uses live dist directory filesystem;
  production uses provided dist static filesystem and errors when absent.
- Protected User Interest: Dev hot outputs are visible while production serves
  shipped assets.
- Cancellation/Ordering/Race Constraints: Mode gate precedes FS selection.

### `WAVE-HL25-005`

- User Story: Dev run loop and production build loop are separated execution
  paths.
- Trigger Conditions: CLI dispatch selects dev or non-dev mode.
- Expected Observable Behavior: Dev path starts long-lived watch/restart/refresh
  loop; non-dev path runs bounded build process.
- Protected User Interest: Operational expectations differ clearly by mode.
- Cancellation/Ordering/Race Constraints: Dev/non-dev branching occurs before
  builder invocation.

### `WAVE-HL25-006`

- User Story: Go build tags differ by mode.
- Trigger Conditions: Go binary compilation runs.
- Expected Observable Behavior: Production compilation includes `-tags=prod`;
  dev compilation omits production tag.
- Protected User Interest: Prod-only code paths are correctly included/excluded.
- Cancellation/Ordering/Race Constraints: Tag decision is applied during go
  build command construction.

### `WAVE-HL25-007`

- User Story: Dev-only refresh script surfaces are suppressed in production.
- Trigger Conditions: Refresh script/hashes are requested at runtime.
- Expected Observable Behavior: Refresh script HTML and hash outputs are empty
  when not in dev mode.
- Protected User Interest: Production responses omit dev websocket tooling.
- Cancellation/Ordering/Race Constraints: Dev-mode gate runs before script
  generation.

### `WAVE-HL25-008`

- User Story: Browser-mode disabled configuration skips browser asset/reload
  work in both build and dev pipelines.
- Trigger Conditions: Core server-only mode is enabled.
- Expected Observable Behavior: Public/private static processing, CSS hot-reload
  broadcast, refresh server startup, and browser reload orchestration are
  skipped.
- Protected User Interest: Server-only applications avoid unnecessary browser
  pipeline cost.
- Cancellation/Ordering/Race Constraints: Browser-mode checks gate each
  browser-specific stage.

## HL-26 Public API Behavior Surface

### `WAVE-HL26-001`

- User Story: Runtime constructor fails fast for missing/invalid config input.
- Trigger Conditions: `wave.New` is called with empty or invalid config bytes.
- Expected Observable Behavior: Constructor panics with clear initialization
  error.
- Protected User Interest: Invalid runtime setup cannot proceed silently.
- Cancellation/Ordering/Race Constraints: Parse/validation gates object
  creation.

### `WAVE-HL26-002`

- User Story: Constructor isolates internal config bytes from caller mutation.
- Trigger Conditions: `wave.New` receives config byte slice.
- Expected Observable Behavior: Input config bytes are cloned before storage and
  parse.
- Protected User Interest: Post-construction caller mutations cannot alter
  runtime.
- Cancellation/Ordering/Race Constraints: Clone occurs before parse invocation.

### `WAVE-HL26-003`

- User Story: Runtime logger accessor and default logger behavior are explicit.
- Trigger Conditions: Logger is requested or constructor receives nil logger.
- Expected Observable Behavior: Runtime exposes active logger and initializes
  default logger when nil is provided.
- Protected User Interest: Logging behavior is consistent without extra setup.
- Cancellation/Ordering/Race Constraints: Logger resolution occurs during
  construction.

### `WAVE-HL26-004`

- User Story: Filesystem access APIs provide paired error-returning and panic
  variants.
- Trigger Conditions: Filesystem accessors are called.
- Expected Observable Behavior: `Get*FS` methods return `(fs, error)` and
  `Must*` methods panic on failure.
- Protected User Interest: Caller chooses strict/error-propagating access style.
- Cancellation/Ordering/Race Constraints: Must-variants delegate through cached
  accessors.

### `WAVE-HL26-005`

- User Story: Public URL and filemap APIs provide stable fallback semantics.
- Trigger Conditions: Public URL/filemap accessors are used.
- Expected Observable Behavior: `GetPublicURL` resolves hashed URL when known
  and prefixed fallback when missing; `GetPublicFileMap` returns defensive
  snapshot.
- Protected User Interest: Asset URL resolution remains usable under partial map
  states.
- Cancellation/Ordering/Race Constraints: Filemap lookup fallback does not
  throw.

### `WAVE-HL26-006`

- User Story: CSS runtime helper APIs gracefully degrade when CSS outputs are
  absent.
- Trigger Conditions: CSS accessor methods are called with missing entry/output.
- Expected Observable Behavior: Critical CSS content/style/hash and stylesheet
  URL/link helpers return empty values when unavailable.
- Protected User Interest: Templates can call helpers safely without nil checks.
- Cancellation/Ordering/Race Constraints: Missing-file detection occurs before
  render/hash generation.

### `WAVE-HL26-007`

- User Story: Public filemap bootstrap APIs expose both element markup and CSP
  hash outputs.
- Trigger Conditions: Public filemap helper methods are called.
- Expected Observable Behavior: Filemap element markup and script SHA-256 hash
  are returned from cached details object.
- Protected User Interest: Templates can wire CSP-compliant filemap bootstrap
  with stable API calls.
- Cancellation/Ordering/Race Constraints: Empty-filemap URL yields empty details
  object.

### `WAVE-HL26-008`

- User Story: Static serving API surface provides both handler and middleware
  forms.
- Trigger Conditions: Static serving APIs are called.
- Expected Observable Behavior: Handler form returns a static handler;
  middleware form wraps next handler with asset-path gating.
- Protected User Interest: Integrators can choose composition style without
  changing behavior.
- Cancellation/Ordering/Race Constraints: Middleware checks asset predicate per
  request.

### `WAVE-HL26-009`

- User Story: Favicon redirect API remains stable middleware contract.
- Trigger Conditions: Favicon middleware is installed and receives favicon
  request.
- Expected Observable Behavior: Middleware redirects to resolved hashed favicon
  when available or returns 404 fallback.
- Protected User Interest: Default favicon serving works with hashed asset map.
- Cancellation/Ordering/Race Constraints: Redirect decision uses current filemap
  lookup result.

### `WAVE-HL26-010`

- User Story: Refresh script APIs expose both inline markup and hash primitives.
- Trigger Conditions: Refresh script accessor methods are called.
- Expected Observable Behavior: APIs return inline script HTML and associated
  SHA-256 hash in dev mode, empty values otherwise.
- Protected User Interest: Integrators can satisfy CSP while embedding dev
  refresh.
- Cancellation/Ordering/Race Constraints: Dev-mode gate precedes script/hash
  generation.

### `WAVE-HL26-011`

- User Story: Framework/runtime setter APIs mutate only explicit framework-owned
  fields.
- Trigger Conditions: Framework setter methods are called on `Wave`.
- Expected Observable Behavior: Setter methods update corresponding framework
  integration fields without mutating unrelated core config fields.
- Protected User Interest: Integration customization remains targeted and
  predictable.
- Cancellation/Ordering/Race Constraints: Setters are direct field updates with
  no hidden side effects.

### `WAVE-HL26-012`

- User Story: Config accessor APIs expose stable path/prefix/runtime metadata.
- Trigger Conditions: Getter methods for dist/static/config/prefix/vite paths
  are called.
- Expected Observable Behavior: Accessors return values derived from parsed
  config and dist layout without additional mutation.
- Protected User Interest: Build/runtime consumers can query canonical paths via
  stable API surface.
- Cancellation/Ordering/Race Constraints: Accessors are pure reads.
