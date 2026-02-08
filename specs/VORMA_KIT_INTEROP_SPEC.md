# Vorma and Kit Interop Specification

Status: Draft  
Last Updated: 2026-02-08  
Applies To: Vorma only
Document Class: Supporting interop reference (not primary black-box conformance spec)

## 0. How To Use This Doc

This document is a dependency-boundary map and interop contract reference.

It is intentionally not the primary requirement-by-requirement conformance spec.
Primary conformance specs (for refactor-safe black-box tests) live in docs like:

- `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`

Use this doc to identify which kit behavior changes require Vorma review, and
promote specific interop items into conformance requirements when they become
externally observable and test-critical.

## 1. Purpose

This document defines which dependency behaviors are **normative for Vorma**,
primarily from `kit/*` plus a small set of directly consumed supporting utility
packages.

It does **not** attempt to fully spec each kit package. Instead, it specifies the
interop boundaries where Vorma either:

- inherits behavior from kit, or
- must remain compatible with kit semantics to function correctly.

## 2. Scope and Non-Goals

In scope:

- Backend interop used by Vorma runtime/build.
- Frontend interop used by Vorma TypeScript runtime.
- Cross-layer invariants and change-management rules.

Out of scope:

- Full standalone specs for every dependency package.
- Wave-only behavior not surfaced through Vorma.

## 3. Normative Language

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

## 4. Interop Surface Map

### 4.1 Backend (Go)

- `kit/mux`: routing, nested routing, task handlers, middleware ordering, task context.
- `kit/matcher`: pattern normalization and route matching semantics used by mux.
- `kit/response`: response proxy model, redirect/header/status merge semantics.
- `kit/headels`: head element dedupe/sort/render behavior used by SSR/head updates.
- `kit/validate`: action/query input parsing and validation error typing.
- `kit/htmlutil`: safe/trusted HTML element rendering used by SSR/head/style/script helpers.
- `kit/envutil` + `kit/netutil`: environment parsing + port selection semantics used by Wave/Vorma mode and dev-port behavior.
- `kit/reflectutil`: interface/nil-shape reflection semantics used by loader/error/validation helper paths.
- `kit/middleware`: endpoint/method-gated middleware composition used by embedded Wave helpers (for example `FaviconRedirect`).
- `kit/bytesutil` + `kit/cryptoutil`: base64/base64url/hash/key-shape helpers used by script-integrity, filemap/build-id, and manifest hashing paths.
- `kit/fsutil`: filesystem copy/gob-decode/panic-helper semantics used by static artifact and embedded-FS paths.
- `kit/executil`: command/shell execution semantics used by build hooks and framework scripts.
- `kit/id`: cryptographically random ID generation semantics used by dev/fast-build IDs.
- `kit/lru`: watcher match-cache recency/eviction semantics used by dev file-classification paths.
- `kit/colorlog`: default logger behavior used when Vorma/Wave callers omit explicit logger injection.
- `kit/contextutil` + `kit/genericsutil`: typed context-store and zero/assert helpers used by mux/task/runtime context plumbing.
- `kit/set`: nil-safe generic set helper used by validator rule evaluation paths.
- `kit/grace`: process orchestration/termination helper used by Vite dev-process lifecycle paths.

### 4.2 Frontend (TypeScript)

- `vorma/kit/matcher/*`: client pattern registry and nested match lookup.
- `vorma/kit/url`: href classification and click interception helpers.
- `vorma/kit/json`: deterministic query serialization and deep equality helpers.
- `vorma/kit/debounce`: debounced async control flow for client events.
- `vorma/kit/listeners`: focus/visibility listener composition.

### 4.3 Supporting Utility Packages (Non-`kit/*`, Vorma-Bound)

- `lab/tsgen` + `lab/tsgen/tsgencore`: reflection-driven TypeScript generation, deterministic ordering, and Go->TS type-shape mapping used by Vorma generated artifacts.
- `lab/viteutil`: Vite manifest/dependency helpers and dev/prod command orchestration used by Wave/Vorma build/runtime paths.
- `lab/jsonschema`: JSON-schema helper constructors/description shaping used by Wave and Vorma schema emission.
- `lab/stringsutil`: builder/line-collection primitives used in generation/schema helper paths.
- `lab/parseutil`: package manifest parsing helper used by Vorma version accessor/export paths.
- `lab/esbuildutil`: esbuild result/metafile helper behavior used by Wave CSS build and dependency graph helper paths.

## 5. Backend Interop Contracts

### 5.1 Router and Matcher Contract (`kit/mux` + `kit/matcher`)

1. Vorma route behavior MUST follow matcher segment semantics:
- Static segment
- Dynamic segment (default `:` prefix)
- Splat segment (default `*`)
- Optional explicit index segment behavior when configured

Nested-router default compatibility:

- when Vorma does not provide explicit nested matcher runes/segments, effective
  nested-router defaults inherited from kit MUST remain:
  `DynamicParamPrefixRune=':'`, `SplatSegmentRune='*'`,
  `ExplicitIndexSegment=""`.

2. Nested loader matching in Vorma MUST use `FindNestedMatches` semantics:
- Root-to-deep match list
- Deterministic ordering
- Index route ordering and splat handling as implemented by matcher
- Stable precedence where static routes beat param routes at same depth
- Deterministic param binding in ambiguous dynamic-pattern cases

Matcher edge semantics inherited by Vorma also include:

- path segment normalization collapsing repeated interior slashes while
  preserving root/trailing-slash semantics (`/` root as empty segment,
  `/users/` trailing-empty segment),
- explicit-index matcher configuration MUST reject slash-bearing index tokens
  (index segment containing `/`) as fail-fast panic-class construction error,
- UTF-8 path segment text MUST be preserved through parse/match flow (no
  normalization that drops or rewrites non-ASCII segment content),
- dynamic segments do not match empty trailing segments,
- non-root splat routes do not match base path without trailing slash (for
  example `/users/*` does not match `/users`, but can match `/users/` with
  empty remainder),
- with explicit index mode enabled, explicit index routes (for example
  `/users/_index`) match both `/users` and `/users/`,
- explicit trailing-slash pattern variants remain distinct candidates when
  registered,
- duplicate dynamic-param names in the same pattern resolve by last-write
  binding for that param key,
- root candidate interplay among root, root splat, and explicit index route
  remains deterministic,
- full-match validity is required for final leaf acceptance (deeper routes do
  not satisfy unmatched intermediate targets),
- parent+leaf stacks can include non-contiguous registered depths when the
  requested leaf itself is fully matched,
- in nested matching, root-only candidates can appear as parent context when a
  deeper candidate fully matches; root alone MUST NOT become a false-positive
  full match for multi-segment paths,
- nested-match ordering tie-break at equivalent depth MUST remain deterministic
  by normalized pattern after precedence filtering, with index routes ordered
  after non-index routes.

Registration safety compatibility:

- duplicate nested-pattern registration is panic-class behavior in kit mux;
  Vorma route-registration/reconciliation flows MUST preserve duplicate-guard
  behavior (`register-if-needed`) and MUST NOT rely on duplicate registration
  being a safe no-op.
- nested-router rebuild compatibility for route-sync flows MUST preserve all
  already-registered handler-backed patterns, replace handler-less patterns
  with the newly supplied pattern set, and keep matcher option semantics
  (`DynamicParamPrefixRune`, `SplatSegmentRune`, `ExplicitIndexSegment`)
  unchanged across rebuild publication.

3. Loader task execution for matched nested routes MUST run in parallel using
`mux.RunNestedTasks`, and Vorma MUST treat result ordering as index-aligned with
matched patterns.

`RunNestedTasks` result-shape compatibility also includes:

- `results.Slice`, `results.ResponseProxies`, and match ordering remain
  index-aligned to nested match order,
- matched patterns without task handlers still produce result entries with
  `RanTask=false` semantics while preserving aligned response-proxy slots,
- per-pattern `results.Map` keys remain keyed by original pattern string.

4. Action routing MUST follow `mux.Router` semantics including:
- Method-specific route resolution
- HEAD fallback behavior
- Mount root behavior
- Mount-root normalization behavior (leading slash + trailing slash canonical form;
  root-only mount resolves to empty prefix)
- `MountRoot(...)` helper argument semantics:
  zero args returns canonical mount root, one arg appends via path join, and
  additional args beyond the first are ignored
- Static-over-dynamic precedence for equivalent candidates
- Splat decoding behavior (URL-decoded segments; empty splat represented as `[""]`)
- HTTP middleware nesting order: global (outermost), then method-level, then
  pattern-level (innermost)
- middleware `If` predicate semantics for both HTTP/task middleware:
  false predicate skips that middleware while preserving downstream chain
  execution
- panic-recovery middleware compatibility: recovery-capable HTTP middleware MUST
  be able to wrap downstream handler execution and recover downstream panics
  (including handler panics) before response is finalized
- Task-middleware execution as parallel fan-out with merged proxy semantics
  applied before final handler emission

This includes preserving halt semantics where:

- unexpected task-middleware/task-handler Go errors return internal-server-error
  class responses and skip final handler emission,
- proxy-class error status or redirect set by task middleware also suppresses
  final handler emission.

Task-handler robustness compatibility also includes:

- a registered route whose task handler is unexpectedly nil MUST resolve as
  internal-server-error class response rather than panicking the process.

Task-middleware fan-out semantics also include:

- applicable task middlewares (after per-request `If` predicate filtering) run
  in parallel fan-out without sibling short-circuit,
- response-proxy merge/halt decision is made after that fan-out completes,
- if any task middleware returns a Go error, runtime returns internal-server-error
  class response even when a middleware also set proxy status/headers.

Task-runtime inheritance through mux/task execution also includes:

- `AnyTask.RunWithAnyInput` type mismatch errors MUST be treated as task
  execution errors (not silently coerced),
- `RunWithAnyInput` typed-nil pointer inputs MUST remain accepted for
  pointer-typed tasks, while untyped `nil` remains a mismatch error,
- per-request `tasks.Ctx` memoization semantics: same task pointer + input value
  pair executes at most once per request context and reuses cached result for
  dependent fan-out calls in that context,
- Vorma’s default `tasks.NewCtx(r.Context())` path (TTL disabled) means
  per-request task results and task errors are sticky for the full request
  context lifetime (no in-request cache expiry/retry window),
- memoization scope MUST remain context-local: identical task/input pairs in
  different `tasks.Ctx` instances execute independently,
- `RunParallel` fan-out MUST share the same underlying `tasks.Ctx` cache so
  sibling branches depending on the same task/input pair collapse to one
  execution,
- `RunParallel` MUST treat nil bound tasks as ignorable entries and MUST be a
  no-op success for an empty effective task set,
- context cancellation/error checks around task execution MUST remain
  cancellation-sensitive both before task entry and after task function return.

5. Vorma input parsing for actions/queries MUST stay compatible with mux parse
entry points (`ParseInput`) and their method/content-type branching.

Parse-input edge compatibility also includes:

- absent/`nil` parser with non-`None` typed input yielding zero-value input
  payloads (rather than panicking),
- parser-driven pointer mutation being reflected in typed handler input.

6. HEAD fallback compatibility MUST preserve GET-equivalent status/header
semantics while suppressing response body bytes when no HEAD-specific route is
registered.

7. Third-party router interop through `InjectTasksCtxMiddleware` MUST preserve
availability of mux task context, params/splat defaults, and response proxy
objects for downstream handlers expecting mux request context helpers.

Tasks-context exposure compatibility includes:

- handlers implementing `TasksCtxRequirer` receiving task context without
  requiring injected middleware,
- regular `http.Handler` paths requiring injected middleware when they depend on
  mux task context helpers,
- regular `http.Handler` paths also receiving task context when route execution
  already enters mux slow-path due to applicable task middleware.

### 5.2 Response Proxy Contract (`kit/response`)

1. Vorma MUST assume proxy objects are per-task/per-handler scoped and merged
after parallel work.

2. When Vorma merges response proxies:
- Headers are merged in order with set/add semantics preserved.
- Header operation precedence MUST remain operation-aware across proxy boundaries:
  a later `SetHeader(key, v)` MUST clear prior accumulated values for `key`
  from earlier proxies before later `AddHeader` values append.
- Cookies are deduped by name, with later proxy value winning.
- Status precedence is: first error status wins; otherwise last success wins.
- Redirect precedence is: first redirect wins if merged status is not an error.
- `SetHeader` MUST clear prior values for the key before subsequent `AddHeader`s.
- Client redirect header emission MUST remain single-valued at apply time.
- repeated client-redirect writes on a single proxy MUST override to last value.
- Redirect URL validation semantics from kit/response MUST remain compatible:
  relative URLs are allowed; absolute URLs are limited to `http`/`https`.
- Client redirect branch selection MUST remain compatible with
  `X-Accepts-Client-Redirect` boolean parsing semantics (only parseable true
  values opt into client redirect behavior).

Operational edge compatibility includes:

- `SetStatus` without explicit text clearing stale prior error text,
- `clientRedirect(url)` setting status `200` when status is unset, while not
  overriding an already-explicit status code,
- `serverRedirect(url, code)` behaving as no-op when proxy already has error
  status (redirect MUST NOT overwrite existing error status),
- merge behavior ignoring `nil` proxies and `nil` cookie entries,
- redirect invocation with `nil` request taking server-redirect path without
  panic,
- redirect status normalization using `303 See Other` as default/fallback:
  redirect helpers receiving omitted or non-`3xx` status codes MUST coerce to
  `303`,
- empty client-redirect URL rejection at API boundary.
- apply-time header semantics preserving writer merge behavior:
  `SetHeader` operations replace existing `ResponseWriter` header values for
  that key, while `AddHeader` operations append to existing values.
- apply-time redirect/error precedence preserving body/header safety:
  if merged status resolves to error, redirect location MUST NOT be emitted;
  if merged status resolves to redirect, redirect status/location MUST override
  prior non-error success status.

3. Vorma MUST short-circuit normal JSON/SSR body emission when merged proxy
indicates redirect or error.

4. Client redirect headers (`X-Client-Redirect`, `X-Accepts-Client-Redirect`)
are part of the Vorma redirect protocol contract and MUST remain compatible with
frontend redirect handling.

5. Response-helper compatibility inherited from `kit/response.Response`
includes:

- `SetHeader`/`AddHeader` operations remain non-committing (safe before final
  status/body decision),
- `Redirect(nil, url, code)` MUST take non-panic server-redirect path by
  setting `Location` + redirect status on the response writer,
- `ClientRedirect(url)` MUST reject empty/invalid URLs and MUST reject calls
  after response has been committed.

### 5.3 Head Elements Contract (`kit/headels`)

1. Vorma head composition MUST use headels dedupe/sort behavior as normative:
- Rule-based uniqueness (including defaults like `title` and
  `meta[name="description"]`)
- Content-hash dedupe fallback
- Last-wins replacement within a duplicate class
- Deterministic output ordering under mixed add/update/remove/reorder operations
- Nil-element tolerance: dedupe paths MUST ignore `nil` elements without panic.
- Content-distinct preservation: non-identical elements (for example script tags
  with different `DangerousInnerHTML`, or script tags differing by required
  boolean attributes) MUST both be preserved.
- Rule-matching strictness: boolean-attribute uniqueness rules MUST require all
  rule-declared booleans to be present (not partial-match).
- Input safety: unique-rule initialization and element collection semantics MUST
  preserve non-mutating behavior for source collections used by Vorma callers.
- Concurrency + collection safety: `HeadEls` mutation/merge APIs
  (`Add`, `AddElements`) MUST remain safe under concurrent use, and `Collect()`
  MUST return clone-safe snapshots so caller mutations cannot alter internal
  stored state.

2. The SSR-rendered head marker boundaries for meta/rest blocks are normative
for frontend head patching behavior.

### 5.4 Validation and Request Input Contract (`kit/validate`)

1. Vorma action/query parsing MUST remain compatible with:
- JSON decoding + validation (`JSONBodyInto`)
- URL query decoding + validation (`URLSearchParamsInto`)

2. Query parsing compatibility includes dotted-path flattening/expansion and
array handling behavior used by `validate.parseURLValues`.

This includes preserving behavior for repeated keys and nested object recovery
from dotted query keys at action/query boundaries.

Parse edge compatibility also includes:

- destination input for URL-value parsing MUST be a non-nil pointer-to-struct
  (invalid destination shape returns parser error),
- scalar parse behavior for `int`/`uint`/`float`/`bool` MUST remain strict
  (unparseable values return parser error rather than silent coercion),
- empty string values for pointer scalar fields MUST clear/set-`nil`,
- empty string values for non-pointer scalar fields MUST preserve existing
  zero/default value (no coercion from empty input),
- slice parsing from repeated keys MUST preserve value order while filtering
  empty-string elements,
- unsupported destination field kinds MUST return parser error.

3. Validation failures surfaced as `ValidationError` MUST continue mapping to
client-visible request errors (e.g. 400 behavior in mux slow-path handling).

4. Parse entry-point guard behavior inherited from `kit/validate` is normative:

- `JSONBodyInto` MUST fail as `ValidationError` for nil request or nil request
  body.
- `URLSearchParamsInto` MUST fail as `ValidationError` for nil request or nil
  request URL.
- `JSONBytesInto`, `JSONStrInto`, and `URLSearchParamsInto` MUST fail as
  `ValidationError` for nil destination.
- successful decode/parse paths MUST execute validator pass
  (`attemptValidation`) on the decoded destination payload before returning.

5. Recursive validator traversal compatibility is normative for parsed
action/query payloads:

- both direct and pointer-receiver `Validate()` implementations MUST be honored,
  including pointer-receiver validators on non-pointer struct values,
- traversal MUST recursively validate exported struct fields, map keys, map
  values, and slice/array elements,
- nil pointers, nil maps/slices, and nil slice elements MUST be tolerated
  without panic,
- non-`ValidationError` failures returned from nested `Validate()` calls MUST be
  path-labeled with validation context, while wrapped `ValidationError` values
  remain class-preserving for request-error mapping.

6. Object/Any helper semantics used inside app-defined validators are part of
Vorma parse/validation interop:

- `Object(...)` MUST accept only struct-like targets or maps keyed by `string`
  (other shapes fail validation),
- unknown or unexported struct fields referenced via `Object.Required(...)` or
  `Object.Optional(...)` MUST fail as validation errors (not panic),
- checker short-circuit behavior (`done` semantics) MUST remain sticky across
  chained calls (first required/optional terminal outcome is preserved),
- repeated `Error()` calls MUST be idempotent (no duplicate-error growth).

7. URL search-param decode shape behavior inherited by Vorma includes:

- embedded struct fields are traversed as part of dotted-key expansion,
- dotted keys under map fields preserve the remainder path as map key text
  (including keys containing dots),
- pointer fields to struct/map/slice destinations are auto-initialized before
  assignment when needed by incoming keys,
- slice fields decoded from repeated keys preserve encounter order, drop empty
  string elements, and resolve to an empty slice when no valid elements remain.

### 5.5 HTML Rendering and CSP Helper Contract (`kit/htmlutil`)

1. Vorma SSR/head/script helper rendering MUST remain compatible with
`htmlutil.Element` rendering semantics:
- tag and unsafe-attribute keys are HTML-escaped before rendering,
- escaped attribute output order is deterministic (sorted by escaped key),
- `AttributesKnownSafe` values override same-key entries from escaped
  `Attributes`,
- boolean attributes are emitted as bare escaped keys.

2. Inner-content precedence and escaping inherited by Vorma MUST remain:
- `DangerousInnerHTML` takes precedence and is emitted verbatim,
- otherwise `TextContent` is HTML-escaped and emitted,
- otherwise empty content is emitted.

3. Self-closing behavior MUST remain compatible:
- HTML void tags self-close by default,
- `SelfClosing=true` forces self-closing for non-void tags.

4. CSP/integrity helper behavior inherited by Vorma includes:
- `ComputeContentSha256` hashes `DangerousInnerHTML` bytes and MUST NOT mutate
  element attributes,
- `SetSha256Integrity` requires non-empty external hash and sets
  `integrity=sha256-<hash>` in known-safe attributes,
- `AddNonce` defaults nonce length to `16` when caller passes `0`, stores nonce
  in known-safe attributes, and surfaces generation errors.

5. Module script helper compatibility:
- `RenderModuleScriptToBuilder(src, b)` MUST render via element renderer with
  tag `script` and known-safe attributes `type=module` and `src=<src>`.

### 5.6 Environment and Port Helper Contract (`kit/envutil` + `kit/netutil`)

1. Environment helper fallback behavior inherited by Vorma/Wave MUST remain:
- `GetStr` returns default when env key is absent,
- `GetInt`/`GetBool` parse env string and return default when parse fails.

2. Free-port selection behavior inherited by Vorma/Wave MUST remain:
- invalid default port (`<=0` or `>65535`) normalizes to `8080`,
- if normalized default is available, it is used,
- otherwise helper probes up to next `1024` ports in ascending order,
- if scan fails, helper attempts random free port allocation,
- if random allocation fails, helper returns normalized default plus error.

3. Port-availability probe compatibility includes:
- explicit invalid probe ports return unavailable,
- probe uses TCP listener checks across `tcp`/`tcp4`/`tcp6` and
  `:<port>` + `localhost:<port>` forms,
- address-family/protocol-unavailable style listen errors
  (`EAFNOSUPPORT`/`EPROTONOSUPPORT`/`EADDRNOTAVAIL`) are ignored for that probe
  attempt.

4. Random-port helper compatibility:
- first attempt uses `tcp4` on `127.0.0.1:0`,
- fallback attempt uses `tcp` on `localhost:0`,
- non-TCP listener-address shapes are error-class outcomes.

5. Localhost guard compatibility (`IsLocalhost`) used by Vorma-adjacent tooling:
- case-insensitive `localhost` hostnames are local,
- loopback IPv4/IPv6 hosts are local,
- non-loopback/private/public hosts and malformed URL/host strings are not local.

### 5.7 Reflection Helper Contract (`kit/reflectutil`)

1. Interface implementation checks inherited by Vorma MUST remain:
- `ImplementsInterface` returns false for nil type inputs,
- it checks both direct implementation and pointer-to-value implementation for
  non-pointer concrete types,
- passing a non-interface target as `iface` is panic-class misuse.

2. Nil-shape detection helper behavior (`ExcludingNoneGetIsNilOrUltimatelyPointsToNil`)
used by loader diagnostics MUST remain:
- nil pointer/interface/map/slice chains resolve as true,
- non-nil map/slice and non-pointer concrete values resolve as false,
- generic `None` sentinel shapes are explicitly excluded from nil-class results.

3. JSON-field-name helper behavior inherited for reflective validation paths:
- explicit `json:"name,...` uses `name`,
- `json:"-"` excludes field (empty name),
- empty-name tag (`json:",omitempty"`) falls back to struct field name.

### 5.8 Endpoint-Gated Middleware Contract (`kit/middleware`)

1. `ToHandlerMiddleware(endpoint, methods, handlerFunc)` compatibility inherited
by Vorma/Wave MUST remain:
- gate condition is exact `r.URL.Path == endpoint` plus method membership in
  `methods`,
- matching requests execute `handlerFunc` and short-circuit downstream handler,
- non-matching requests pass through to downstream handler unchanged.

2. Endpoint gate behavior is exact-string and MUST NOT introduce implicit path
normalization (no slash-trim/case folding).

### 5.9 Crypto and Byte-Codec Helper Contract (`kit/cryptoutil` + `kit/bytesutil`)

1. Hash/encoding primitives inherited by Vorma/Wave MUST remain:
- `cryptoutil.Sha256Hash(msg)` returns raw SHA-256 digest bytes (length 32),
- `bytesutil.ToBase64` uses standard base64 encoding,
- `bytesutil.ToBase64URLRaw` uses URL-safe base64 without padding.

2. Build/runtime hash-bearing surfaces that rely on this contract include:
- SSR script and critical/filemap helper CSP hash materialization,
- refresh-script hash helper values,
- build artifact hash-derived naming and build-id composition paths.

3. Byte/key conversion guard behavior inherited by hash/signature helper callers:
- `bytesutil` gob decode helpers fail on nil/empty-invalid inputs rather than
  silently succeeding with garbage state,
- `cryptoutil.ToKey32` MUST reject non-32-byte input,
- `cryptoutil.FromKey32` MUST reject nil key input.

4. Input-validation behavior for security-sensitive helper APIs is normative:
- nil/empty key guards in `HmacSha256`/`ValidateHmacSha256`,
- nil secret-key/ciphertext-length guards in symmetric decrypt/encrypt helpers,
- signature/public-key shape guards in asymmetric verification helpers.

### 5.10 Filesystem Helper Contract (`kit/fsutil`)

1. Directory helper semantics inherited by Vorma/Wave MUST remain:
- `EnsureDir(path)` creates missing directories with mode `0755`,
- `EnsureDirs(paths...)` applies `EnsureDir` per path and fails on first error.

2. File-copy semantics inherited by artifact/static flows MUST remain:
- `CopyFile(src, dest)` creates missing destination parent directories with mode
  `0755`,
- destination file is created/truncated, payload bytes are copied, and `Sync()`
  is called before success return,
- destination permission bits match source permission bits.

3. Directory-copy semantics inherited by build paths MUST remain:
- `CopyDir(src, dst)` recursively copies nested files/directories,
- destination root directory mode follows source directory mode.

4. Gob decode helper semantics inherited by filemap/runtime decode paths:
- `FromGobInto(file, destPtr)` fails for nil file or nil destination pointer,
- `FromGob[T](file)` fails for nil file and returns decoded typed value on
  success,
- decode failures are surfaced as error-class outcomes (not silent partial
  decode success).

5. Panic helper semantics inherited by embed/static bootstrap paths:
- `MustSub(...)` panics on sub-FS failure,
- `MustReadFile(...)` panics on read failure.

### 5.11 Command Execution Helper Contract (`kit/executil`)

1. `MakeCmdRunner(commands...)` inherited behavior MUST remain:
- no command arguments is error-class outcome (`no commands provided`),
- command execution binds stdout/stderr to process stdout/stderr.

2. `RunCmd(commands...)` MUST remain a direct execution wrapper over
`MakeCmdRunner`.

3. Shell execution helper behavior inherited by build hooks/events:
- `RunShell(cmd)` uses `cmd /C` on Windows and `sh -c` on non-Windows targets,
- stdout/stderr are passed through to process stdout/stderr,
- command/process failure returns error-class outcome.

4. Executable-location helper behavior:
- `GetExecutableDir()` returns current executable directory and surfaces path
  resolution failures.

### 5.12 ID Generation Contract (`kit/id`)

1. Default ID helper behavior inherited by Vorma build-id generation MUST remain:
- `New(idLen)` uses mixed-case alphanumeric charset by default,
- `idLen=0` returns empty string on success.

2. Charset validation semantics inherited by callers:
- at most one optional charset argument is allowed,
- charset length must be `[1,255]`,
- charset must be single-byte ASCII only.

3. Distribution/entropy behavior inherited by build-id paths:
- byte-to-charset mapping uses rejection-sampling semantics (no modulo-bias
  truncation).

4. Multi-ID helper semantics:
- `NewMulti(idLen, quantity, ...)` returns `quantity` IDs in order and fails on
  first generation error.

### 5.13 LRU/TTL Cache Contract (`kit/lru`)

1. Construction semantics inherited by watcher cache paths MUST remain:
- negative `maxItems` coerces to `0`,
- `maxItems<=0` cache stores no entries.

2. Recency and spam semantics inherited by classification-cache behavior:
- non-spam `Get`/`Set` moves entry to MRU/front,
- spam entries do not move on `Get`,
- overflow eviction removes LRU/back entry.

3. TTL semantics inherited by optional TTL consumers:
- `SetWithTTL(..., ttl=0)` creates non-expiring entries,
- expired entries are treated as misses and removed on access,
- `CleanupExpired()` removes all currently expired entries.

4. Background cleanup loop semantics (when default TTL > 0):
- cleanup ticker interval is bounded to `[1s,1m]` around half default TTL,
- `Close()` terminates cleanup loop and is idempotent.

5. Concurrency semantics:
- cache operations remain safe under concurrent access.

### 5.14 Logger Helper Contract (`kit/colorlog`)

1. Default logger bootstrap semantics inherited by Vorma/Wave:
- `New(label, opts...)` defaults output to stdout when unset,
- color mode is auto-detected only for tty-like `*os.File` outputs when
  `UseColor` is nil.

2. Level gating semantics:
- `Enabled(level)` is threshold check `level >= configuredLevel`.

3. Output shape semantics inherited by operational logs:
- each log line includes timestamp (`YYYY/MM/DD HH:MM:SS`), label, level prefix,
  message, and optional attrs,
- level prefixes remain `DEBUG  `, `WARNING  `, `ERROR  `, or empty for info.

4. Attribute/group semantics inherited by structured logging:
- `WithGroup` prefixes keys with dot-joined group path,
- `WithAttrs`/record attrs render as bracketed key/value pairs.

5. Handler clone/thread-safety semantics:
- `WithAttrs`/`WithGroup` clones share a write mutex for non-interleaved output,
- empty `WithAttrs`/empty-group `WithGroup` are identity no-op semantics.

6. Write failure semantics:
- handler write failures are surfaced as error-class outcomes from `Handle`.

### 5.15 Type Generation Contract (`lab/tsgen` + `lab/tsgen/tsgencore`)

1. Deterministic output structure inherited by generated Vorma TS artifacts:
- top-level section order remains: generated header, optional collection block,
  ad-hoc exported types, optional extra TS code block,
- exported type lines are sorted deterministically,
- collection item property lines and final collection item ordering are sorted
  deterministically.

2. Collection/phantom semantics inherited by route artifact generation:
- default collection var name is `tsgenCollection` when unset,
- `ExportCollectionArray=true` prefixes collection with `export`,
- phantom type fields emit `null as unknown as <Type>` for resolved named/inline
  TS shapes,
- unresolved/unknown phantom type shapes emit `null as unknown`,
- null-equivalent phantom type shapes are omitted.

3. Go->TS mapping semantics inherited from `tsgencore`:
- bool->`boolean`, numeric kinds->`number`, string->`string`,
- `time.Time`->`string`, `time.Duration`->`number`,
- `[]byte`->`string`,
- slices/arrays->`Array<T>`,
- maps->`Record<K,V>`,
- empty objects->`Record<never, never>`,
- nil type instance->`null`, interface-like unknown->`unknown`.

4. Struct/tag/embedding override precedence semantics:
- precedence is `TSType()` override > `ts_type` struct tag > reflection-derived
  type,
- untagged anonymous embeds flatten fields depth-first in deterministic order,
- tagged embeds remain nested under tag-derived field key,
- pointer fields and `omitempty`/`omitzero` fields are optional in TS output.

5. Name/reference resolution semantics:
- conflicting requested export names are disambiguated with numeric suffixes
  (`_2`, `_3`, ...),
- unresolved internal type IDs in final replacement pass degrade to `unknown`
  with warning-class diagnostic output.

6. File writer helper semantics:
- `GenerateTSToFile` requires non-empty output path,
- destination directory is ensured before write,
- generated output write failures are surfaced as error-class outcomes.

### 5.16 Vite Utility Contract (`lab/viteutil`)

1. Manifest helper semantics inherited by build/runtime paths:
- `ReadManifest(path)` returns decoded manifest map or error,
- `FindRelativeEntrypointPath` resolves entrypoint by matching `IsEntry` chunk
  and basename,
- `FindAllDependencies` performs recursive import traversal with cycle guard and
  returns deduped dependency basenames including entry chunk basename.

2. Dev-script generation semantics inherited by HTML response path:
- `ToDevScripts` always emits module script tags for `@vite/client` and client
  entrypoint using `http://localhost:<__VITE_PORT>/...`,
- React variant also emits React-refresh preamble module script,
- render failures are surfaced as error-class outcomes.

3. Port helper semantics inherited by dev orchestration:
- `InitPort(...)` selects a free port via netutil helper and stores it in
  `__VITE_PORT`,
- `GetVitePortStr()` reads `__VITE_PORT` environment value as-is.

4. Build-context command orchestration semantics:
- base command is tokenized with whitespace split (`strings.Fields`),
- dev mode executes `vite --port <p> --clearScreen false --strictPort true`
  plus optional config flag,
- prod mode executes `vite build` with configured outdir/assetsDir and temp
  manifest filename, then renames temp manifest to configured destination,
- existing dev process is terminated before restart attempts,
- wait/cleanup paths terminate/wait only for live processes.

### 5.17 JSON Schema Helper Contract (`lab/jsonschema`)

1. Entry-constructor helper semantics inherited by Wave/Vorma schema emission:
- `Required*` helpers set `required=true` and corresponding type,
- `Optional*` helpers set `required=false` and corresponding type,
- `ObjectWithOverride` forces object type and uses override description text.

2. Description synthesis semantics inherited by generated schema docs:
- without override, descriptions are prefixed with `Required|Optional <type>.`,
- optional default/example prose is appended with deterministic text layout,
- string defaults are quoted for description rendering.

3. Listing/uniqueness prose helper semantics:
- example/default list formatting uses quoted Oxford-list style,
- `UniqueFrom(...)` prepends `Must be unique from ...` phrasing.

4. Structural serialization semantics:
- `ToJSONSchema` carries `Items` only when item type is present.

### 5.18 String and Package Parse Utility Contract (`lab/stringsutil` + `lab/parseutil`)

1. String helper semantics inherited by generation/schema helpers:
- `stringsutil.Builder` methods are fluent append operations over internal
  `strings.Builder`,
- `CollectLines` returns scanner-split lines in order and returns `(nil, nil)`
  for empty input.

2. Package manifest parser semantics inherited by version-accessor paths:
- `PackageJSONFromString` and `PackageJSONFromFile` are panic-class helpers on
  malformed/unreadable inputs,
- parser returns source lines, line index for first `"version":` entry, and
  JSON-decoded version string.

### 5.19 Context, Generic, and Set Helper Contract (`kit/contextutil` + `kit/genericsutil` + `kit/set`)

1. Typed-context helper semantics inherited by mux/task context plumbing:
- `contextutil.NewStore[T](key)` creates key-space isolated store instances
  (same key name across store instances remains non-colliding),
- `GetContextWithValue` stores typed value in context,
- `GetValueFromContext` returns typed value or zero-value when missing/mismatched,
- `GetRequestWithContext` wraps request context with stored typed value.

2. Generic helper semantics inherited by runtime helper layers:
- `genericsutil.AssertOrZero[T](v)` returns asserted `T` value when type matches,
  otherwise zero-value `T`,
- `genericsutil.Zero[T]()` returns zero-value `T`,
- `genericsutil.IsNone(v)` is true only for direct empty-struct alias values or
  pointers (named non-alias empty structs are not treated as `None`),
- `OrDefault(field, defaultVal)` uses default when field equals type zero value.

3. Generic-set helper semantics inherited by validator/rule internals:
- `set.New[T]()` returns empty set,
- `Set.Add` on nil set initializes set before insert and returns updated set,
- `Set.Contains` is membership check with false for absent values.

### 5.20 Process Graceful-Shutdown Helper Contract (`kit/grace`)

1. Orchestration defaults inherited by app/process lifecycle helpers:
- default shutdown timeout is `30s` when unset,
- default signals are platform-specific (`Interrupt` on Windows; `SIGHUP`,
  `SIGINT`, `SIGTERM`, `SIGQUIT` on non-Windows),
- default logger is `colorlog.New("grace")` when unset.

2. Startup/shutdown orchestration behavior:
- startup callback error triggers shutdown path via context cancellation,
- shutdown callback executes with timeout-bounded context,
- shutdown timeout logs warning and allows orchestrator return even when callback
  is non-cooperative.

3. Process termination helper semantics used by Vite dev lifecycle:
- `TerminateProcess(process, wait, logger)` sends graceful terminate signal
  (`SIGTERM` non-Windows, kill on Windows),
- waits up to `wait` for process exit,
- on timeout force-kills process and logs warning,
- signal/kill failures are error-class outcomes.

### 5.21 Esbuild Helper Contract (`lab/esbuildutil`)

1. Build-result error extraction semantics inherited by Wave CSS build:
- `CollectErrors(result)` aggregates esbuild error texts and returns error when
  any error entries exist.

2. Metafile helper semantics:
- `UnmarshalOutput(result)` decodes `result.Metafile` JSON into helper subset
  struct and surfaces parse failures.

3. Dependency graph helper semantics:
- `FindAllDependencies(metafile, importPath)` recursively follows output imports
  with cycle guard,
- imports with kind `dynamic-import` are excluded from traversal,
- returned dependencies are basename-normalized and include basename of
  `importPath`.

4. Entrypoint lookup helper semantics:
- `FindRelativeEntrypointPath` resolves output key by exact `EntryPoint` match,
  otherwise returns error.

## 6. Frontend Interop Contracts

### 6.1 Client Matcher Parity (`vorma/kit/matcher/*`)

1. Client-side matching MUST be semantically aligned with backend matcher rules
for dynamic params, splat, and index behavior.

This parity includes trailing-slash and empty-segment eligibility, root/index
precedence behavior, and deterministic ordering under ambiguous pattern sets.

2. Vorma-generated app config values for matcher runes/segments MUST be treated
as the canonical bridge between backend and frontend matching behavior.

3. Client partial-match fallback helper behavior MUST remain compatible with
Vorma runtime assumptions:
- full match first,
- then longest-to-shortest prefix probing,
- deterministic longest successful parent selection,
- null result when no prefix match exists.

Additional matcher semantics inherited by Vorma frontend runtime include:

- `parseSegments` normalization parity:
  empty path (`""`) yields no segments, root path (`"/"`) yields a single empty
  segment, repeated interior slashes collapse, trailing slash contributes a
  trailing empty segment, and UTF-8 segment text is preserved.
- explicit-index registration guard parity:
  explicit index token containing `/` MUST fail fast during registry creation.
- explicit-index normalization parity:
  in explicit-index mode, index-route patterns normalize to trailing-empty
  segment form (for example `/_index`-style patterns), and non-root trailing
  slash registration forms are rejected as invalid.
- best-match precedence parity:
  exact static match wins before dynamic traversal; dynamic candidates score
  lower than static candidates; splat fallback is only selected when no better
  candidate exists.
- dynamic-empty guard parity:
  dynamic segments MUST NOT match empty trailing segments.
- trailing-slash static parity:
  lookup of a trailing-slash path MUST still allow static match against the
  slash-trimmed static pattern variant.
- parameter extraction parity:
  dynamic params are extracted positionally from normalized segments; duplicate
  dynamic param names resolve by last-write assignment in final params map.
- splat extraction parity:
  root and non-root splat matches expose remaining real-path segments as
  `splatValues` from the splat position onward.
- nested-match root-guard parity:
  root-only match sets MUST NOT be treated as full matches for non-root paths.
- nested conflict-resolution parity:
  when longest candidates conflict, index candidates are deprioritized; dynamic
  vs splat longest-candidate tie-break depends on path depth compatibility
  (dynamic favored on equal depth, splat favored when real path is deeper).
- nested parent/leaf gap parity:
  nested result stacks may include parent and deep leaf matches even when
  intermediate depths are unregistered, but unmatched intermediate request paths
  still return null for that path.
- nested ordering parity:
  final nested match list remains deterministic by depth with index patterns
  ordered after non-index peers.

### 6.2 URL and Link Semantics (`vorma/kit/url`)

1. Internal vs external URL classification MUST remain consistent with current
`getHrefDetails` behavior.

2. Link interception eligibility (new tab modifiers, download links, hash-only,
etc.) MUST remain compatible with Vorma link handling assumptions.

3. GET/HEAD detection behavior from `getIsGETRequest` is normative for fetch and
redirect flow branching in Vorma client runtime.

Additional inherited URL/link helper semantics include:

- `getIsErrorRes` classifies response errors by status-class prefix (`4xx`,
  `5xx`), and non-error classes remain false.
- `getIsGETRequest` is case-insensitive for method checks and treats absent
  method as GET-compatible.
- `getHrefDetails` contract:
  empty/invalid href yields `isHTTP: false`; non-empty parseable hrefs resolve
  against `window.location.href`; internal HTTP(S) hrefs expose both
  `absoluteURL` and origin-stripped `relativeURL`; external HTTP(S) hrefs expose
  empty `relativeURL`.
- non-HTTP protocols (`mailto:`, `tel:`, and similar) are explicitly non-HTTP
  and MUST not be routed through internal navigation/prefetch paths.
- `getAnchorDetailsFromEvent` contract:
  non-anchor targets resolve to null; returned anchor metadata includes
  `isInternal` derived from `getHrefDetails`; default-prevention eligibility is
  disabled for `_blank`, middle-click, hash-only hrefs, download links, and
  modifier-key clicks (`ctrl`/`shift`/`meta`/`alt`).
- `getPrefetchHandlers` contract:
  only internal HTTP hrefs produce handlers; `start()` schedules delayed prefetch
  (default 50ms); `stop()` cancels timer and removes matching prefetch link.
- `prefetch(relativeURL)` behavior:
  existing `link[href="<relativeURL>"]` is removed before insertion, then a
  single `<link rel="prefetch">` with matching `href` is appended.

### 6.3 Query Serialization Contract (`vorma/kit/json`)

1. Client query serialization MUST remain compatible with backend query parsing:
- Deterministic key ordering
- Dot-path flattening for nested objects
- Repeated keys for arrays
- Empty values encoded as empty strings where applicable

Serializer behavior inherited by Vorma also includes:

- top-level and nested object keys are alphabetically sorted before emission,
- `null`/`undefined` values serialize as empty-string query values,
- arrays preserve item order as repeated keys; empty arrays serialize as a
  single empty-string value,
- empty objects serialize as an empty-string value for that object key,
- nested object properties flatten using `.` path joining (including keys that
  themselves contain dots),
- scalar values serialize via `String(value)` and final wire encoding follows
  `URLSearchParams` encoding semantics.

2. Deep-equality helper semantics (`jsonDeepEquals`) are normative for client
change-detection flows:

- strict type mismatch returns false,
- arrays compare by length and index order (order-sensitive),
- objects compare by own-key presence and recursively equal values
  (key-order-insensitive),
- `null` vs `undefined` remain distinct,
- `NaN` remains not-equal to `NaN` (strict-equality semantics).

3. Stable JSON stringification helper semantics (`jsonStringifyStable`) are
normative when deterministic payload hashing/string forms are required:

- object keys are recursively sorted while array order is preserved,
- circular references throw explicit errors (no silent cycle handling),
- primitive/sparse/`undefined`/`NaN`/`Infinity` handling remains aligned with
  standard `JSON.stringify` output semantics after structural stabilization.

4. Any change to serializer/equality/stringification shape is a cross-layer
contract change and MUST be
handled as a breaking interop review item.

### 6.4 Event Timing Helpers (`vorma/kit/debounce`, `vorma/kit/listeners`)

1. Debounce semantics are normative for client status/event timing where used by
navigation, HMR, and focus revalidation paths.

Current inherited debounce helper behavior includes:

- each invocation resets the pending timer window for that debounced function,
- only the latest invocation in a debounce window executes the wrapped callback,
- returned Promise resolves with the wrapped callback return value after delay,
- superseded invocations are canceled by timer reset and do not execute callback.

2. Focus/visibility listener behavior MUST remain compatible with Vorma’s stale
revalidation assumptions.

Current inherited focus-listener contract used by Vorma helper paths:

- registration MUST include window `focus` and `visibilitychange` listeners,
- `visibilitychange` path MUST gate callback execution on
  `document.visibilityState === "visible"`,
- callback delivery MUST be debounced using kit listener helper window
  (currently 30ms),
- returned cleanup callback MUST remove both listeners.

## 7. Cross-Layer Invariants

The following invariants are required:

1. Pattern invariant: backend and frontend route matching semantics MUST match.

2. Query invariant: TS query serializer output MUST round-trip through backend
query parser/validator in expected shape.

3. Redirect invariant: backend redirect headers/status behavior MUST match
frontend redirect interpreter logic.

4. Head invariant: backend head dedupe/render semantics MUST remain compatible
with frontend incremental head update logic.

## 8. Change Management Policy

Any PR that changes one of the interop surfaces above MUST:

1. Update this spec (or explicitly state no interop impact).
2. Update/add tests at the relevant boundary:
- Backend unit/integration tests
- Frontend unit tests (for TS interop)
- End-to-end tests when wire behavior changes
3. Document user-facing impact in release notes/changelog if behavior changes.

Changes are classified as:

- Interop-preserving refactor: no external behavior change; tests prove parity.
- Interop-affecting change: behavior change at Vorma boundary; requires explicit
compatibility decision and release documentation.

## 9. Primary Code References

Backend touchpoints:

- `vormaruntime/glue.go`
- `vormaruntime/gmpd.go`
- `vormaruntime/get_root_handler.go`
- `vormaruntime/route_registry.go`
- `vormabuild/vorma_gen_ts.go`
- `vorma.go`

Frontend touchpoints:

- `internal/framework/_typescript/client/src/client.ts`
- `internal/framework/_typescript/client/src/client_loaders.ts`
- `internal/framework/_typescript/client/src/init_client.ts`
- `internal/framework/_typescript/client/src/links.ts`
- `internal/framework/_typescript/client/src/redirects/redirects.ts`
- `internal/framework/_typescript/client/src/vorma_app_helpers/vorma_app_helpers.ts`
