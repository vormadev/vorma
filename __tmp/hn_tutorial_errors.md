# HN Tutorial Audit: Errors and Unverified Claims

Full sentence-by-sentence audit of `vorma-hn-clone-react-tutorial.draft.md`
against Vorma source code.

---

## CONFIRMED ERRORS

### 1. Section 11.10: Navigation and Data Refetching — FABRICATED BEHAVIOR

**Lines 1589-1603**

The entire section 11.10 contains fabricated claims about loader re-execution
behavior during client-side navigation.

**What the tutorial says:**

> - If you go from `/_index` to `/item/42`, the root loader (`/`) **doesn't**
>   re-run -- it already has the data, and nothing changed. Only the new child
>   loader (`/item/:id`) runs.
> - If you go from `/item/42` to `/item/99`, the root loader still doesn't
>   re-run. The item loader re-runs with the new `id`.
> - If you go from `/item/42` back to `/_index`, the root loader still doesn't
>   re-run. The index loader runs fresh.

**What the code actually does:**

The server **always runs ALL matched loaders** for every navigation. There is no
mechanism to skip loaders that "haven't changed."

- The client sends a normal request with the target URL + `?vorma_json=buildID`
  — no information about which loaders to skip
  (`fetch_route_data_server.ts:116-134`)
- The server calls `nestedmux.FindMatches()` and runs ALL matched loaders in
  parallel (`runtimehttp.go:329-332`, `routepipeline.go:318-323`)
- There is zero caching or selective execution logic on the server side

**Verdict:** This is completely made up. Every navigation triggers full
server-side execution of all matched route loaders.

---

### 2. Section 13.1: Task Deduplication Key — INCORRECT

**Line 1868**

**What the tutorial says:**

> The deduplication key is `(task pointer, input value, context)`.

**What the code actually does:**

The dedup key is `(task pointer, input value)` — the context is NOT part of the
key. Different contexts have independent caches (because each `Ctx` has its own
`results` map), so the dedup is per-context by construction, but the context is
not part of the `taskKey` struct.

From `kit/tasks/tasks.go:59-63`:

```go
type taskKey struct {
    taskPtr uintptr
    input   any
}
```

**Verdict:** Misleading. The sentence implies context is part of a composite
key, but context isolation comes from each `Ctx` having its own map — it's not a
field in the key. The behavior described is correct (same context + same task +
same input = one execution), but the description of the mechanism is wrong.

---

### 3. Section 16.1: Loading Indicator `include` Description — IMPRECISE

**Line 2408-2409**

**What the tutorial says:**

> `include`: Filter which operations trigger the indicator. Defaults to `"all"`,
> but you can pass `["navigations", "submissions", "revalidations"]` to be
> selective.

**What the code actually does:**

The type is `"all" | Array<"navigations" | "submissions" | "revalidations">`.
The default when `include` is omitted is effectively `"all"` (line 279:
`!config.include || config.include === "all"`).

**Verdict:** The description is functionally correct but doesn't make clear that
`include` accepts either the string `"all"` or an array of specific types. Minor
issue.

---

## UNVERIFIABLE CLAIMS (could not confirm or deny from source)

### 4. Section 15.5: Proxy Merging — "The first error status wins"

**Lines 2347-2353**

The proxy merging rules are claimed as:

> - **Status:** The first error status (4xx/5xx) wins. Otherwise the last
>   success status wins.
> - **Redirects:** The first redirect wins (unless an error status is also set,
>   in which case the error takes precedence).

**Verification:** CORRECT — confirmed at `kit/response/response.go:600-634`. The
merge loop takes the first 4xx/5xx status found, and only updates with 2xx if no
error has been found. Redirects are taken from the first proxy that has one,
unless an error status is already set.

---

### 5. Section 18: Middleware Order

**Line 2908-2909**

> Order matters: first registered = outermost (runs first on request, last on
> response)

**Verification:** The code appends middleware to a slice
(`kit/mux/mux.go:227-237`), but the exact wrapping order (outermost-first vs
innermost-first) was not definitively confirmed from the examined code alone.
Functionally likely correct but not verified.

---

### 6. Section 19.2: Critical CSS Inlining

**Line 3203-3205**

> CSS in this file is inlined directly into the HTML for fast first-paint, while
> `NonCritical` CSS is loaded asynchronously.

**Verification:** The schema defines `CSSEntryFiles.Critical` and
`CSSEntryFiles.NonCritical` fields, but the actual inlining behavior was not
traced through the full build pipeline. Likely correct based on naming, but not
confirmed.

---

## ALL OTHER CLAIMS — VERIFIED CORRECT

Everything below was verified against actual source code and found to be
accurate.

### Section 2: Project Structure

- File layout, naming conventions — matches scaffolder templates in
  `internal/site/`
- `VormaAppConfig` fields (`Wave`, `HeadDedupeKeysFunc`, `DefaultHeadElsFunc`,
  `RootTemplateDataFunc`) — verified at `vorma.go:50-80`

### Section 2: context.go — DefineLoader/DefineAction

- `LoaderCtx` and `ActionCtx` wrapper types, `DefineLoader`/`DefineAction`
  helper pattern — verified matches scaffolder output

### Section 2: init.go

- `MustInitWithDefaultRouter()`, `MustStaticMiddleware()`, `App.ServerAddr()` —
  verified
- `healthcheck.Healthz` middleware — verified at
  `kit/middleware/healthcheck/healthcheck.go:12`

### Section 2: vorma.entry.tsx

- `initClient({ vormaAppConfig, renderFn })` signature — verified at
  `app/init.ts:19-22`
- `VormaRootOutlet` component — verified exists

### Section 2: vorma.app.tsx

- `makeTypedLink`, `makeTypedNavigate`, `makeTypedUseRouterData`,
  `makeTypedUseLoaderData` — verified
- `api = { query, mutate }` pattern — verified in
  `internal/site/frontend/src/vorma.app.tsx:62-71`

### Section 3: Route Planning

- Loaders = data fetching, Actions = mutations — correct
- Actions support any HTTP method (POST, PUT, DELETE, etc.) — verified at
  `vorma.go` and action registration code

### Section 6: Backend Routes

- `DefineLoader(pattern, handler)` returns typed loader — verified
- `DefineAction(method, pattern, handler)` — verified
- `vorma.None` is an empty struct — verified at `vorma.go:35`, aliases
  `genericsutil.None`
- `vorma.LoaderError{Client, Server}` — verified at `vormaruntime.go:1087-1090`
- Client field sent to user, Server field logged — verified at
  `vormaruntime.go:690-717`
- `c.Param("id")` for URL params — verified at `kit/mux/mux.go:426`
- `c.HeadEls()` for per-route head elements — verified at
  `kit/mux/mux.go:1087-1091`

### Section 7: Frontend Routes

- `route(pattern, import, componentName, errorBoundaryName?)` — verified at
  `client/buildtime.ts:4-13`
- Pattern must match backend DefineLoader pattern — correct by design

### Section 8: React Components

- `RouteProps<"/">` includes `Outlet` component — verified
- `useRouterData(props)` returns root loader data — verified
- `useLoaderData(props)` returns route-specific loader data — verified
- `Link` with `pattern` and `params` props — verified at `ui/helpers.ts`
- `api.mutate()` returns
  `{ success: true, data: T } | { success: false, error: string }` — verified
  via `SubmitResult<T>` at `navigation/types.ts:95-97`
- `api.mutate()` for non-POST requires `requestInit: { method: "DELETE" }` —
  verified at `app/helpers.ts:218-223` (type enforces it)

### Section 9: Build Pipeline

- AST analysis discovers DefineLoader/DefineAction calls — verified at
  `vormabuild/backendroutes/registrationgraph/registrationgraph.go`
- Generates TypeScript types in `vorma.gen/` — verified
- Starts Vite for HMR — verified at
  `wave/wavedev/devserver/devserver.go:1066-1082`
- Starts Go server — verified at `wave/wavedev/devserver/devserver.go:347-360`

### Section 11.1: Pattern Types

- Static, Dynamic, Index, Splat — verified at
  `kit/internal/matchercore/matchercore.go:271-281`

### Section 11.2: Nested Matching

- Matches ALL ancestor patterns, not just one — verified in `nestedmatcher`
  package
- Matched loaders run in parallel — verified at `nestedmux.go:509-528`
  (goroutines + WaitGroup)

### Section 11.3: \_index Convention

- `/dashboard/_index` is the index page, `/dashboard` is the layout — verified

### Section 11.4: Deeper Nesting

- 6-deep nesting all match simultaneously — verified

### Section 11.5: Gaps Are Fine

- Missing intermediates are skipped — verified by tests in
  `find_nested_matches.partial_matching_with_gaps.test.ts`

### Section 11.6: Splat Routes

- `c.SplatValues()` returns captured segments — verified at
  `matchercore.go:86-94`
- Dynamic params beat splats for single segments — verified by test at
  `find_matches_test.go:854-863`
- `/`, `/*`, `/_index` registered: `/` matches `/` and `/_index` but not `/*` —
  verified at `find_matches_test.go:553-558`

### Section 11.7: What Doesn't Match

- Dynamic params require non-empty segment — verified at `matchercore.go:664`
- Unregistered paths return 404 — correct

### Section 11.8: Error Propagation

- Parent failure cancels children — verified at `nestedmux.go:415-430, 503-504`
  and tests
- Child failure doesn't affect parent — verified by test
  `Task_Error_Does_Not_Cancel_Siblings`
- Sibling loaders independent — verified; code intentionally avoids errgroup
  fail-fast (`nestedmux.go:408-411`)

### Section 11.9: Frontend Route Declarations

- All claims verified

### Section 12: vorma.gen

- Generated types, routes constant, VormaApp type, vormaAppConfig, RouteProps —
  all verified

### Section 13: Tasks

- `tasks.NewTask` with comparable input — verified (`tasks.go:28`)
- `tasks.NewCtx(r.Context())` — verified (`tasks.go:78-82`)
- `tasks.NewCtxWithTTL` with TTL expiry — verified (`tasks.go:84-110`)
- `ctx.WithNativeContext(r)` shares cache, uses request cancellation — verified
  (`tasks.go:116-132`)
- `RunParallel` uses errgroup, cancels on first error — verified
  (`tasks.go:326-343`)
- `Bind` pairs input with destination variable — verified
  (`tasks.go:55-57, 272-288`)
- `c.TasksCtx()` available in loaders and actions — verified

### Section 14: Validation

- `validate.Object(input)` fluent chain — verified (`validate.go:227-248`)
- `Required("Field")` / `Optional("Field")` — verified (`validate.go:108-118`)
- All chain validators (Email, URL, Min, Max, RangeInclusive, In, Regex,
  MutuallyExclusive, MutuallyRequired) — verified with line numbers
- `.Error()` returns `*ValidationError` or nil — verified (`validate.go:62-67`)
- Required short-circuits — verified (sets `c.done = true`, all validators check
  `if c.done { return c }`)
- `validate.Any("label", value)` — verified (`validate.go:223-225`)
- `Validator` interface auto-called after JSON parsing — verified
  (`validate.go:435-462, 1168`)
- Returns 400 if validation fails, handler never runs — verified
  (`kit/mux/mux.go:558-586`)

### Section 15: Response Control

- `c.ResponseProxy()` returns `*response.Proxy` — verified
- `Redirect(r, url, code)` defaults to 303 — verified (`response.go:183-192`)
- X-Accepts-Client-Redirect / X-Client-Redirect headers — verified
  (`response.go:176-181`)
- `SetStatus`, `SetHeader`, `AddHeader`, `SetCookie` — all verified
- Proxy merging rules — all verified

### Section 16: Client-Side APIs

- `setupGlobalLoadingIndicator` with start/stop/isRunning — verified
  (`extras.ts:258-261`)
- Default delays 12ms — verified (`extras.ts:251`)
- `revalidateOnWindowFocus({ staleTimeMS: 5000 })` default 5000ms — verified
  (`extras.ts:379-380`)
- `revalidate()` — verified (`client.ts:213-217`)
- `getStatus()` with isNavigating/isSubmitting/isRevalidating — verified
- Error boundary 4th arg to route() — verified (`buildtime.ts:4-13`)
- Error boundary receives `{ error: string }` — verified (`context.ts:70`)
- `defaultErrorBoundary` in initClient — verified (`init.ts:80-88`)
- Intent-based prefetching on hover/touch — verified
  (`links_prefetch_lifecycle.ts:148, 164`)
- Default prefetch delay 100ms — verified (`links_prefetch_lifecycle.ts:114`)
- Link lifecycle hooks beforeBegin/beforeRender/afterRender — verified
  (`links_click_lifecycle.ts:11-15`)
- Can be async (return `void | Promise<void>`) — verified
- Link/navigate options: replace, scrollToTop, state — verified
  (`ui/helpers.ts:237-277`)
- navigate() additional: search, hash — verified
- `api.query()` sends as URL search params, `api.mutate()` sends as JSON body —
  verified in `vorma.app.tsx`
- `submit()` with dedupeKey, skipGlobalLoadingIndicator, revalidate — verified
  (`navigation/types.ts:89-93`)
- `addClientLoader` with params, serverDataPromise, signal — verified
  (`typed_adapter_helpers_runtime.ts:14-29`)
- Client loaders run during prefetch — verified

### Section 17: TypeScript Generation

- `AdHocType` with TypeInstance and TSTypeName — verified (`tsgencore.go:27-31`)
- `ts_type` struct tag — verified (`tsgencore.go:619`)
- `TSTyper` interface with `TSType() map[string]string` — verified
  (`tsgencore.go:33-36`)
- `TSTyperRaw` interface with `TSTypeRaw() string` — verified
  (`tsgen.go:129-131`)
- Precedence: TSType() > ts_type tag > reflection — verified
  (`tsgencore.go:392-400`)
- `vorma.FormData` uses TSTyperRaw — verified (`vormaruntime.go:412-416`)
- `ExtraTSCode` appended to generated file — verified
  (`tsartifactgen.go:480-483`)

### Section 18: Middleware

- `AddGlobalHTTPMiddleware` accepts `func(http.Handler) http.Handler` — verified
  (`kit/mux/mux.go:227-237`)
- Standard Go middleware signature works — correct

### Section 19: Production

- Build steps (pnpm build + go build -tags prod) — verified
- wave.dev.go / wave.prod.go build tag pattern — verified in
  `internal/site/backend/`
- `//go:embed` for production binary — verified
- `wave.New(wave.Config{WaveConfigJSON, DistStaticFS})` — verified
  (`wave.go:1145-1160`)
- `fsutil.MustReadFile` / `fsutil.MustSub` — verified
  (`kit/fsutil/fsutil.go:185-212`)
- Vercel uses Rust serverless proxy — verified (`internal/site/api/proxy.rs`)
- Hashed assets get immutable cache headers — verified (`wave.go:1805-1808`)

### Section 20: Configuration

- Custom context methods pattern — correct by design
- `HeadDedupeKeysFunc` dedup behavior (deepest route wins) — verified
  (`kit/headels/headblocks.go:217-260`)
- `DefaultHeadElsFunc` — verified (`vorma.go:54-55`)
- `RootTemplateDataFunc` — verified (`vorma.go:56-57`)
- `useViewTransitions` wraps in `document.startViewTransition()` — verified
  (`render_commit_runtime.ts:248-262`)
- Only for user navigations, not prefetch/revalidation — verified (conditions at
  lines 251-252)
- wave.config.json fields — all verified against schema at
  `wave/wavebuild/builder/internal/schema/schema.go`
