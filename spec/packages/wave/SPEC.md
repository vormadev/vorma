# wave Specification

Status: Active  
Last Updated: 2026-02-09  
Applies To: `wave` runtime package behavior (`wave/*.go`)

## 1. Purpose

This is the canonical contract for Wave runtime/public-asset behavior exposed by
the `wave` package.

Build/dev control-plane behavior lives in `wave/tooling` and is intentionally
not duplicated here.

## 2. Ownership Boundary

`wave` owns:

- runtime config parsing/accessor defaults,
- runtime env helpers and port selection helpers,
- runtime FS/filemap/public-URL resolution,
- runtime static-serving helpers,
- runtime CSS/filemap/refresh script HTML helper output.

`wave/tooling` owns:

- build/dev CLI and orchestration,
- watcher/event-loop control-plane semantics,
- static/CSS/schema build-time pipelines.

## 3. Requirement Catalog

### 3.1 Config and Construction

#### WAVE-RT-001: ParseConfig Minimal Safety Contract

Given `ParseConfig(data)` is called  
When JSON is invalid  
Then it MUST return an error prefixed with `parse config:`.

Given parsed config omits `Core`  
When parsing completes  
Then it MUST return an error (`config: Core section is required`) rather than
allowing nil-dereference behavior.

Given parsing succeeds  
When return values are observed  
Then `Dist.Root` MUST be populated as `filepath.Clean(Core.DistDir)`.

Given `ParseConfigFile(path)` is called  
When file read fails  
Then it MUST return an error prefixed with `read config file:`.

Given `ParseConfigFile(path)` reads file content successfully  
When parsing is delegated  
Then it MUST return exactly `ParseConfig(fileBytes)` results.

#### WAVE-RT-002: Constructor Panic-and-Default Contract

Given `New(Config)` receives nil `WaveConfigJSON`  
When called  
Then it MUST panic.

Given `ParseConfig` fails during `New`  
When called  
Then `New` MUST panic with that parse failure.

Given `Logger` is nil  
When `New` initializes runtime instance  
Then it MUST default to `colorlog.New("wave")`.

Given construction succeeds  
When instance is created  
Then cache-backed runtime helpers (`baseFS`, `publicFS`, `privateFS`, `fileMap`,
CSS/filemap helpers) MUST be initialized.

#### WAVE-RT-003: Raw Config and Parsed Config Accessors

Given a `Wave` instance  
When `RawConfigJSON()` is called  
Then it MUST return the raw bytes passed into constructor config.

Given a `Wave` instance  
When `GetParsedConfig()` is called  
Then it MUST return the parsed config pointer used by the runtime instance.

Given a `Wave` instance  
When `Logger()` is called  
Then it MUST return the logger pointer held by the runtime instance.

Given runtime config accessor helpers (`GetPublicPathPrefix`, `GetDistDir`,
`GetPublicStaticDir`, `GetPrivateStaticDir`, `GetConfigFile`,
`GetViteManifestLocation`, `GetViteOutDir`, `GetStaticPrivateOutDir`,
`GetStaticPublicOutDir`)  
When called  
Then each helper MUST return the corresponding parsed-config/dist-layout value
without additional mutation.

### 3.2 Runtime Cache Semantics

#### WAVE-RT-004: Dev-vs-Prod Cache Behavior

Given runtime cache wrappers (`cache`, `cacheMap`)  
When Wave is in dev mode  
Then cached values MUST be recomputed on each access.

Given Wave is not in dev mode  
When cache access succeeds  
Then results MUST be memoized for subsequent calls.

### 3.3 Runtime FS, Filemap, and URL Resolution

#### WAVE-RT-005: Base/Public/Private FS Selection Contract

Given runtime is in dev mode  
When base FS is initialized  
Then it MUST use `os.DirFS(cfg.Dist.Static())`.

Given runtime is not in dev mode  
When `distStaticFS` is nil  
Then base FS initialization MUST fail.

Given base FS resolves  
When public/private FS are initialized  
Then they MUST resolve via `fs.Sub(base, RelPaths.AssetsPublic())` and
`fs.Sub(base, RelPaths.AssetsPrivate())`.

#### WAVE-RT-006: MustGet FS Panic Contract

Given `MustGetPublicFS()` / `MustGetPrivateFS()`  
When underlying getter returns error  
Then helper MUST panic.

#### WAVE-RT-007: Public Filemap Decode Contract

Given `GetPublicFileMap()`  
When runtime filemap blob exists and decodes  
Then decoded map MUST be returned.

Given file open/decode fails  
When called  
Then error MUST be returned (not silently swallowed).

#### WAVE-RT-008: Public URL Resolution Contract

Given `GetPublicURL(original)`  
When input starts with `data:`  
Then output MUST return the original data URL unchanged.

Given filemap lookup finds a hashed output  
When resolved  
Then output MUST use that hashed URL.

Given lookup misses or filemap load fails  
When resolved  
Then runtime MUST return fallback prefixed URL and keep execution running.

Given `FileMap.Lookup(original, prefix)`  
When lookup hits a hashed entry  
Then it MUST return `EnsureLeadingSlash(path.Join(prefix, entry.DistName))` with
`found=true`.

Given `FileMap.Lookup(original, prefix)`  
When lookup misses  
Then it MUST return `EnsureLeadingSlash(path.Join(prefix, original))` with
`found=false`.

#### WAVE-RT-009: Public Asset Detection Contract

Given `IsPublicAsset(urlPath)` and configured prefix is empty or `/`  
When called  
Then runtime MUST perform FS-backed existence check under public FS.

Given configured prefix is non-root  
When called  
Then runtime MUST treat prefix match as public-asset check result.

### 3.4 Static Serving and Redirect Helpers

#### WAVE-RT-010: Static Handler Construction Contract

Given `GetServeStaticHandler(immutable=false)`  
When called  
Then returned handler MUST serve from public FS under configured
`PublicPathPrefix`.

Given `immutable=true`  
When serving response  
Then handler MUST set `Cache-Control: public, max-age=31536000, immutable`.

Given public FS initialization fails  
When called  
Then error MUST be returned.

Given `MustGetServeStaticHandler(immutable)`  
When static handler creation fails  
Then helper MUST panic.

#### WAVE-RT-011: ServeStatic Middleware Gate Contract

Given middleware from `ServeStatic(immutable)`  
When request path is public asset  
Then middleware MUST serve static response through static handler.

Given request path is not public asset  
When called  
Then middleware MUST delegate to `next` handler.

#### WAVE-RT-012: Favicon Redirect Contract

Given `FaviconRedirect()` middleware for `GET`/`HEAD /favicon.ico`  
When hashed/public URL differs from fallback path  
Then middleware MUST issue `302 Found` redirect.

Given hashed/public URL equals fallback path (no mapped favicon)  
When called  
Then middleware MUST return `404 Not Found`.

### 3.5 Runtime CSS and Filemap HTML Helpers

#### WAVE-RT-013: Critical CSS Helper Contract

Given no critical CSS entry or missing critical CSS artifact  
When critical CSS helpers are called  
Then helper outputs MUST be empty values (`""`).

Given critical CSS exists  
When helpers are called  
Then runtime MUST provide:

- inline CSS content,
- rendered style element with `id="wave-critical-css"`,
- sha256 hash derived from rendered style element content.

Given `GetCriticalCSSElementID()`  
When called  
Then it MUST return `wave-critical-css`.

#### WAVE-RT-014: Normal Stylesheet Helper Contract

Given no non-critical CSS entry  
When stylesheet helpers are called  
Then URL/link outputs MUST be empty.

Given non-critical CSS reference resolves  
When helpers are called  
Then runtime MUST provide prefixed stylesheet URL and rendered link element with
`id="wave-normal-css"`.

Given `GetStyleSheetElementID()`  
When called  
Then it MUST return `wave-normal-css`.

#### WAVE-RT-015: Public Filemap HTML Helper Contract

Given filemap reference URL resolves  
When filemap helpers are called  
Then runtime MUST provide:

- modulepreload + module script elements,
- script body that defines `window.__wave.getPublicURL(...)`,
- sha256 hash for generated module script content.

Given filemap URL is empty  
When called  
Then helper outputs MUST be empty values.

Given `GetPublicFileMapURL()`  
When public filemap ref lookup fails  
Then helper MUST return empty URL value (`""`) rather than panicking.

#### WAVE-RT-016: Refresh Script Helper Contract

Given runtime is not in dev mode  
When refresh script helpers are called  
Then returned script and hash MUST be empty values.

Given runtime is in dev mode  
When refresh script helpers are called  
Then helpers MUST use refresh port from environment (`WAVE_REFRESH_SERVER_PORT`)
or default (`10000`) and return:

- `<script>` wrapper around `RefreshScriptInner(port)`,
- base64-encoded sha256 hash of refresh script body.

Given `RefreshScriptInner(port)`  
When called  
Then it MUST return the refresh JavaScript template with provided port
interpolated into websocket endpoint URL.

### 3.6 Runtime Env and ParsedConfig Helpers

#### WAVE-RT-017: Env Mode and Port Helper Contract

Given `SetModeToDev()`  
When called  
Then `WAVE_MODE` MUST be set to `development` and `GetIsDev()` MUST reflect
that.

Given `GetPort()` or `GetRefreshServerPort()`  
When corresponding environment value is absent or not an integer  
Then helper MUST return `0`.

Given `SetPort(port)` or `SetRefreshServerPort(port)`  
When called  
Then helper MUST write decimal string value into corresponding environment key.

Given instance wrapper helpers `Wave.GetIsDev()`, `Wave.MustGetPort()`, and
`Wave.SetModeToDev()`  
When called  
Then behavior MUST delegate to package-level env helpers (`GetIsDev`,
`MustGetPort`, `SetModeToDev`) without additional policy.

Given `MustGetPort()` first call  
When not in dev mode (or when `WAVE_PORT_HAS_BEEN_SET=true`)  
Then helper MUST use configured `PORT`, defaulting to `8080` if absent/invalid.

Given `MustGetPort()` first call in dev mode without prior port-set flag  
When selecting port  
Then helper MUST request free port near configured/default port and set `PORT`
and `WAVE_PORT_HAS_BEEN_SET=true`.

Given subsequent `MustGetPort()` calls  
When called repeatedly  
Then helper MUST return memoized port value for process lifetime.

Given `MustGetAppPort` alias  
When called  
Then it MUST behave identically to `MustGetPort`.

#### WAVE-RT-018: ParsedConfig Helper Defaults Contract

Given parsed config helper methods  
When corresponding fields are absent  
Then defaults MUST be:

- `PublicPathPrefix()`: `/`,
- `WatchRoot()`: `.`,
- `HealthcheckEndpoint()`: `/`,
- `UsingBrowser()`: inverse of `ServerOnlyMode`,
- `UsingVite()`: true only when `Vite != nil`.

Given `PublicPathPrefix()`  
When configured prefix is non-empty and not `/`  
Then helper MUST normalize with both leading and trailing slash.

Given CSS entry helper methods (`CriticalCSSEntry`, `NonCriticalCSSEntry`)  
When values are non-empty  
Then returned paths MUST be `filepath.Clean(...)`.

Given `ViteManifestPath()`  
When called  
Then helper MUST return
`filepath.Join(Dist.StaticPrivate(), "vorma_out", "vorma_vite_manifest.json")`.

#### WAVE-RT-019: Runtime Framework Extension Mutator Contract

Given runtime mutators `AddFrameworkWatchPatterns`, `AddIgnoredPatterns`, and
`SetPublicFileMapOutDir`  
When called  
Then runtime config MUST reflect appended watch patterns, appended ignored
patterns, and overwritten public filemap output directory value.

### 3.7 Runtime Path and Shared Helper Models

#### WAVE-RT-020: RelPaths and DistLayout Path-Shaping Contract

Given `RelPaths` helpers  
When called  
Then returned relative paths/file names MUST be:

- `Internal()`: `internal`,
- `AssetsPublic()`: `assets/public`,
- `AssetsPrivate()`: `assets/private`,
- `CriticalCSS()`: `internal/critical.css`,
- `NormalCSSRef()`: `internal/normal_css_file_ref.txt`,
- `PublicFileMapRef()`: `internal/public_file_map_file_ref.txt`,
- `PublicFileMapGob()`: `internal/public_filemap.gob`,
- `PublicFileMapGobName()`: `public_filemap.gob`,
- `PrivateFileMapGobName()`: `private_filemap.gob`,
- `PublicFileMapJSName()`: `vorma_internal_public_filemap.js`,
- `PublicFileMapTSName()`: `filemap.ts`,
- `PublicFileMapJSONName()`: `filemap.json`.

Given `DistLayout` helpers  
When called  
Then each method MUST derive paths via `filepath.Join` from `Root` and segment
constants so derived outputs compose consistently (`Static`, `StaticAssets`,
`StaticPublic`, `StaticPrivate`, `Internal`, `CriticalCSS`, `NormalCSSRef`,
`PublicFileMapRef`, `PublicFileMapGob`, `PrivateFileMapGob`, `KeepFile`).

Given `DistLayout.Binary()`  
When runtime OS is windows  
Then output file name MUST be `main.exe`; otherwise it MUST be `main`.

#### WAVE-RT-021: Watch/Refresh Helper Struct Semantics Contract

Given `WatchedFile.Sort()` is called and `SortedHooks` is nil  
When hooks are classified  
Then `SortedHooks` MUST be initialized and hooks MUST be bucketed by timing:
`post`, `concurrent`, `concurrent-no-wait`, and default-to-`pre` for all other
values.

Given `WatchedFile.Sort()` is called and `SortedHooks` is already non-nil  
When called repeatedly  
Then helper MUST return without re-sorting or appending.

Given `RefreshAction.Merge(other)`  
When combining actions  
Then result fields MUST be per-field boolean OR.

Given `RefreshAction.IsZero()`  
When evaluated  
Then it MUST return true only when all action flags are false.

## 4. Scenario Catalog

### WRC-RT-001 (covers WAVE-RT-001)

Parse-config fixtures MUST verify invalid-JSON wrapping, missing-core rejection,
dist-root cleaning, and parse-file read/delegation behavior.

### WRC-RT-002 (covers WAVE-RT-002)

Constructor fixtures MUST verify panic-on-invalid-input behavior, default logger
selection, and runtime cache/helper initialization.

### WRC-RT-003 (covers WAVE-RT-003)

Accessor fixtures MUST verify raw/parsed/logger passthrough and runtime config
helper passthrough semantics.

### WRC-RT-004 (covers WAVE-RT-004)

Cache fixtures MUST verify dev-time recompute behavior and prod-time memoized
behavior.

### WRC-RT-005 (covers WAVE-RT-005)

Filesystem initialization fixtures MUST verify dev/prod base-FS selection and
public/private sub-FS resolution behavior.

### WRC-RT-006 (covers WAVE-RT-006)

Must-get FS fixtures MUST verify panic propagation when underlying getter calls
fail.

### WRC-RT-007 (covers WAVE-RT-007)

Filemap fixtures MUST verify decode success behavior and open/decode error
propagation.

### WRC-RT-008 (covers WAVE-RT-008)

Public URL fixtures MUST verify data-URL passthrough, hashed lookup behavior,
fallback behavior, and `FileMap.Lookup` hit/miss contracts.

### WRC-RT-009 (covers WAVE-RT-009)

Asset detection fixtures MUST verify root-prefix FS existence checks and
non-root prefix-match behavior.

### WRC-RT-010 (covers WAVE-RT-010)

Static handler fixtures MUST verify handler creation, immutable cache-header
behavior, and must-get panic semantics.

### WRC-RT-011 (covers WAVE-RT-011)

ServeStatic middleware fixtures MUST verify public-asset serving and non-asset
delegation paths.

### WRC-RT-012 (covers WAVE-RT-012)

Favicon redirect fixtures MUST verify mapped-icon redirect and fallback 404
behavior.

### WRC-RT-013 (covers WAVE-RT-013)

Critical CSS fixtures MUST verify empty outputs for absent assets, populated
outputs for present assets, and critical-style element ID behavior.

### WRC-RT-014 (covers WAVE-RT-014)

Normal stylesheet fixtures MUST verify empty outputs when non-critical entry is
absent, populated URL/link output behavior, and stylesheet element ID behavior.

### WRC-RT-015 (covers WAVE-RT-015)

Public filemap HTML fixtures MUST verify filemap URL/elements/script-hash
behavior and empty-on-missing-reference behavior.

### WRC-RT-016 (covers WAVE-RT-016)

Refresh script fixtures MUST verify dev-only output gating, default-port
fallback, hash output behavior, and `RefreshScriptInner` template interpolation.

### WRC-RT-017 (covers WAVE-RT-017)

Env helper fixtures MUST verify mode/port get-set behavior, wrapper delegation,
port allocation/memoization behavior, and `MustGetAppPort` aliasing.

### WRC-RT-018 (covers WAVE-RT-018)

Parsed-config helper fixtures MUST verify defaulting, prefix normalization, CSS
entry cleaning, and Vite manifest path shaping.

### WRC-RT-019 (covers WAVE-RT-019)

Runtime mutator fixtures MUST verify watch-pattern append semantics,
ignore-pattern append semantics, and public-filemap-out-dir overwrite behavior.

### WRC-RT-020 (covers WAVE-RT-020)

Path helper fixtures MUST verify `RelPaths` and `DistLayout` path-shaping
contracts, including OS-sensitive binary naming.

### WRC-RT-021 (covers WAVE-RT-021)

Watch/refresh helper fixtures MUST verify `WatchedFile.Sort`
bucketing/idempotent behavior and `RefreshAction` merge/zero semantics.

## 5. Relation to Other Specs

- Wave build/dev owner contracts: `spec/packages/wave/tooling/SPEC.md`
- Wave build/dev traceability:
  `spec/packages/wave/tooling/TRACEABILITY_MATRIX.md`
- Wave build/dev conformance issues:
  `spec/packages/wave/tooling/CONFORMANCE_ISSUES.md`
