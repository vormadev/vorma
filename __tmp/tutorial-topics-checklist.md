# Vorma HN Clone Tutorial — Topics Checklist

Topics already covered are marked [x]. Topics that need to be added are marked [
].

---

## Core Concepts

- [x] Project scaffolding (`npm create vorma@latest`)
- [x] Project structure walkthrough
- [x] VormaAppConfig and `app.go`
- [x] `context.go` — LoaderCtx, ActionCtx, DefineLoader, DefineAction
- [x] `init.go` — Router initialization, middleware setup
- [x] Loaders (data fetching, patterns, params, `c.Param()`)
- [x] Actions (mutations, method + pattern, input/output types)
- [x] `vorma.None` for empty inputs/outputs
- [x] `vorma.LoaderError` (client vs server error messages)
- [x] Frontend route declarations (`route()` — pattern, import, component name)
- [x] `vorma.entry.tsx` — client init
- [x] `vorma.bindings.ts` — typed helpers
- [x] `useLoaderData()` and `useRouterData()`
- [x] `Link` component (typed patterns, params)
- [x] `navigate()` for client-side navigation
- [x] `api.mutate()` — calling actions, typed input/result
- [x] Routing in depth — pattern types (static, dynamic, index, splat), nested
      matching, `_index` convention, deeper nesting, gaps, what doesn't match,
      parallel execution & error propagation, navigation & data refetching
- [x] `vorma.gen/` — what it contains, why not to edit it
- [x] `HeadEls` — per-route `<title>` and meta overrides via `c.HeadEls()`
- [x] Data flow diagrams (loader flow, mutation flow)

## Tasks

- [x] What a task is, `tasks.NewTask`, `tasks.Ctx`
- [x] Request-scoped caching (same task + same input + same ctx = runs once)
- [x] `c.TasksCtx()` — accessing the per-request task context
- [x] Task composition — tasks calling other tasks
- [x] `RunParallel` + `Bind` — concurrent execution with output binding
- [x] Global thundering-herd protection with `NewCtxWithTTL`
- [x] `WithNativeContext` — bridging global cache with per-request cancellation
- [x] When to use request-scoped vs global tasks
- [x] How Vorma uses tasks internally (loaders as tasks, parallel middleware)

## Validation

- [x] The `validate` package — what it is, import path
- [x] The `Validator` interface (`Validate() error`) — implement on input
      structs
- [x] Automatic validation pipeline: actions parse JSON body then call
      `Validate()` automatically
- [x] Why this is basically built-in tRPC — validation on the Go side, type
      safety on the TS side
- [x] `validate.Object(input)` — fluent ObjectChecker for struct validation
- [x] `.Required("Field")` / `.Optional("Field")` — field-level checks
- [x] Chaining validators: `.Email()`, `.Min()`, `.Max()`, `.RangeInclusive()`,
      `.URL()`, `.In()`, `.Regex()`
- [x] `.MutuallyExclusive()` / `.MutuallyRequired()` — field group constraints
- [x] `.Error()` — terminal method that returns `*ValidationError` or nil
- [x] `validate.Any("label", value)` — standalone value validation
- [x] How validation errors are returned to the frontend (400 status, error
      message)

## TypeScript Generation Customization

- [x] `AdHocType` — registering custom Go types for TypeScript generation
- [x] `ts_type` struct tag — override a field's TypeScript type
- [x] `TSTyper` interface — override field types via method
      (`TSType() map[string]string`)
- [x] `TSTyperRaw` interface — provide a raw TypeScript type string (e.g.,
      `FormData`)
- [x] `ExtraTSCode` — inject raw TypeScript into `vorma.gen`
- [x] `vorma.FormData` — using browser FormData for file uploads in actions

## Client-Side APIs

- [x] `setupGlobalLoadingIndicator({ start, stop, isRunning })` — show/hide
      NProgress or similar
- [x] `revalidateOnWindowFocus({ staleTimeMS })` — auto-refresh data on tab
      focus
- [x] `revalidate()` — manually refresh current route data
- [x] `getStatus()` — check `isNavigating`, `isSubmitting`, `isRevalidating`
- [x] `addClientLoader()` — client-side data fetching layer (receives
      `serverDataPromise`)
- [x] Error boundaries — 4th argument to `route()`, error boundary components
- [x] `prefetch: "intent"` — intent-based prefetching (already set as default,
      but explain it)
- [x] Link lifecycle hooks — `beforeBegin`, `beforeRender`, `afterRender`
- [x] `api.query()` — calling GET actions (vs `api.mutate()` for POST)
- [x] `submit()` — low-level action submission with options (`dedupeKey`,
      `skipGlobalLoadingIndicator`)

## Response Control

- [x] `c.ResponseProxy()` — the Proxy object for deferred response building
- [x] `c.Redirect(url, statusCode)` — redirects from loaders/actions
- [x] `c.SetResponseStatus(code)` — set HTTP status
- [x] `c.SetResponseHeader()` / `c.AddResponseHeader()` — custom headers
- [x] `c.SetResponseCookie()` — set cookies from loaders
- [x] Client-side vs server-side redirects (AJAX -> `X-Client-Redirect` header)

## Middleware

- [x] Adding middleware in `init.go` via `r.AddGlobalHTTPMiddleware()`

## Production & Deployment

- [x] `pnpm build` — production build with embedded assets
- [x] Dev vs prod build tags (`wave.dev.go` vs `wave.prod.go` with
      `//go:build prod`)
- [x] `go:embed` — how assets are embedded into a single binary
- [x] Dockerfile — multi-stage build pattern
- [x] Vercel deployment option

## Advanced Patterns

- [x] Splat routes (`/*`) for catch-all patterns (covered in Routing In Depth)
- [x] `wave.config.json` — build pipeline configuration options
- [x] Custom context methods on `LoaderCtx`/`ActionCtx`
- [x] `HeadDedupeKeysFunc` — head element deduplication rules
- [x] `RootTemplateDataFunc` — passing data to the HTML template
- [x] `useViewTransitions` option in `initClient`
