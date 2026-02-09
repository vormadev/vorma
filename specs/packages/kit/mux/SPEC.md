# kit/mux Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/mux`

## Scope

Package-owned HTTP and nested routing behavior for `Router` and `NestedRouter`, including middleware orchestration, task execution pipeline, context plumbing, and nested task fan-out.

## Terminology

- "Fast path" means `Router.ServeHTTP` handling for HTTP routes without task middleware and without `TasksCtxRequirer`.
- "Slow path" means `Router.ServeHTTP` handling that builds a `tasks.Ctx` and full request transport state.
- "Task middleware" means `TaskMiddleware` attached at global/method/pattern levels.
- "Merged proxy" means the result of `response.MergeProxyResponses(...)` across task middleware proxies.

## Requirements

- `KIT-MUX-001` Router default matcher option behavior.
`NewRouter` MUST default dynamic param prefix rune to `':'` and splat segment rune to `'*'` when not configured.
- `KIT-MUX-002` Router mount-root normalization behavior.
`NewRouter` MUST normalize `MountRoot` to either empty string or leading+trailing slash form, with `"/"` normalized to empty.
- `KIT-MUX-003` `MountRoot()` method contract.
`Router.MountRoot()` with no args MUST return normalized mount root; with args it MUST join mount root and only the first provided pattern argument.
- `KIT-MUX-004` Method matcher construction semantics.
Per-method matchers MUST be lazily created and MUST share the router's matcher option set.
- `KIT-MUX-005` Method-scoped route registration semantics.
Route registration MUST be method-scoped; same pattern under different methods MUST remain distinct registrations.
- `KIT-MUX-006` HTTP handler registration semantics.
`RegisterHandler` MUST register handler type as HTTP and MUST mark `needsTasksCtx` when handler implements `TasksCtxRequirer`.
- `KIT-MUX-007` Task handler registration semantics.
`RegisterTaskHandler` MUST register handler type as task and MUST attach a request-data getter for the pattern.
- `KIT-MUX-008` Router route listing semantics.
`Router.AllRoutes()` MUST expose the live underlying route slice in registration order.
- `KIT-MUX-009` Pattern/path semantics delegation.
Path pattern semantics (static/dynamic/splat and precedence) MUST delegate to `kit/matcher` behavior.
- `KIT-MUX-010` Mount-root stripping at dispatch.
`ServeHTTP` MUST strip configured mount-root prefix from incoming path before matching when prefix matches.
- `KIT-MUX-011` `HEAD` explicit-handler precedence.
When handling `HEAD`, router MUST prefer explicit `HEAD` route matches before fallback.
- `KIT-MUX-012` `HEAD` fallback-to-`GET` semantics.
When explicit `HEAD` route is absent and `GET` matches, router MUST execute `GET` handler semantics while suppressing body bytes, preserving headers and status.
- `KIT-MUX-013` Not-found behavior.
Unmatched requests MUST use configured global not-found handler when present; otherwise MUST return default HTTP 404.
- `KIT-MUX-014` Fast-path gating behavior.
Fast path MUST only apply to HTTP handlers with no task middleware at route/method/global levels and no `needsTasksCtx` flag.
- `KIT-MUX-015` Fast-path request data injection behavior.
Fast path MUST only attach request-store transport when params or splat values are non-empty.
- `KIT-MUX-016` Slow-path transport initialization behavior.
Slow path MUST create a `tasks.Ctx`, request transport, and new response proxy for the request.
- `KIT-MUX-017` Parse-input invocation guard.
Route `ParseInput` MUST be invoked only when configured and when task input type is not `None`.
- `KIT-MUX-018` Parse-input mutation adoption.
Parsed/mutated input pointer value MUST become `ReqData.Input()` delivered to task handlers.
- `KIT-MUX-019` Parse-input validation error mapping.
Validation errors (`validate.IsValidationError`) from parse-input MUST map to HTTP 400.
- `KIT-MUX-020` Parse-input non-validation error mapping.
Non-validation parse-input errors MUST map to HTTP 500.
- `KIT-MUX-021` Task-handler success response behavior.
Successful task-handler outputs MUST be serialized as JSON.
- `KIT-MUX-022` Task-handler error mapping.
Task-handler returned errors MUST map to HTTP 500 and MUST stop normal success response flow.
- `KIT-MUX-023` Task-handler proxy application behavior.
Task-handler response proxy MUST be applied before JSON write; if proxy is error or redirect, JSON write MUST be skipped.
- `KIT-MUX-024` Nil-ish task output warning behavior.
Nil-ish task outputs MUST trigger warning logging and MUST still proceed through JSON response path.
- `KIT-MUX-025` HTTP middleware layering behavior.
HTTP middleware MUST execute with global outermost, method-level middle, pattern-level innermost nesting.
- `KIT-MUX-026` HTTP middleware registration-order behavior.
Within each HTTP middleware layer, registration order MUST be preserved.
- `KIT-MUX-027` HTTP middleware conditional behavior.
When `MiddlewareOptions.If` is provided and returns false, HTTP middleware MUST be skipped and downstream handler invoked directly.
- `KIT-MUX-028` Task middleware gathering order behavior.
Applicable task middleware MUST be gathered in global then method then pattern order.
- `KIT-MUX-029` Task middleware conditional behavior.
When task middleware `MiddlewareOptions.If` returns false, that middleware MUST be skipped.
- `KIT-MUX-030` Task middleware execution model behavior.
Applicable task middleware MUST run via `tasks.Ctx.RunParallel`; each middleware MUST receive its own `ReqData[None]` instance and isolated response proxy.
- `KIT-MUX-031` Task middleware error short-circuit behavior.
If any task middleware returns non-nil Go error, response MUST be HTTP 500 and downstream handler MUST NOT execute.
- `KIT-MUX-032` Task middleware proxy-merge behavior.
After task middleware success, middleware proxies MUST be merged and applied; if merged proxy is error or redirect, downstream handler MUST NOT execute.
- `KIT-MUX-033` Task-route HTTP middleware composition behavior.
For task routes, HTTP middleware chain MUST wrap the final task handler.
- `KIT-MUX-034` HTTP-route-with-task-middleware composition behavior.
For HTTP routes with task middleware, the route HTTP chain MUST run as downstream of task middleware gating.
- `KIT-MUX-035` HTTP chain caching behavior.
`Route.httpChain` MUST cache compiled HTTP middleware chain on first use; subsequent requests MUST reuse cached chain.
- `KIT-MUX-036` Slow-path `tasks.Ctx` availability behavior.
Task handlers, task middleware, and slow-path HTTP handlers MUST observe non-nil `tasks.Ctx` via request context helpers.
- `KIT-MUX-037` `TasksCtxRequirer` behavior.
Handlers implementing `TasksCtxRequirer` MUST force slow-path handling and MUST receive non-nil `tasks.Ctx`.
- `KIT-MUX-038` Regular-handler `tasks.Ctx` absence behavior.
Regular fast-path HTTP handlers without task middleware and without `TasksCtxRequirer` MUST not receive `tasks.Ctx`.
- `KIT-MUX-039` Request helper fallback behavior.
`GetTasksCtx`, `GetParams`, `GetParam`, and `GetSplatValues` MUST safely return nil/empty defaults when request-store transport is absent.
- `KIT-MUX-040` ReqData response-helper forwarding behavior.
`ReqData` response helper methods (`HeadEls`, redirect/status/header/cookie setters/getters, redirect/error/success predicates) MUST forward to underlying response proxy behavior.
- `KIT-MUX-041` `InjectTasksCtxMiddleware` behavior.
`InjectTasksCtxMiddleware` MUST inject tasks context only when absent; if tasks context already exists, it MUST pass through without replacement.
- `KIT-MUX-042` Nested router options behavior.
`NewNestedRouter` MUST default dynamic/splat/index options consistently (`:`, `*`, empty explicit-index) and honor configured overrides.
- `KIT-MUX-043` Nested route registration behavior.
Nested route registration MUST support routes with handlers and pattern-only routes; duplicate pattern registration MUST panic.
- `KIT-MUX-044` Nested matching wrapper behavior.
`FindNestedMatches` and `FindNestedMatchesAndRunTasks` MUST preserve matcher found/not-found contract and return `(nil, false)` when no nested match exists.
- `KIT-MUX-045` Nested task-run prerequisites and result-shape behavior.
`RunNestedTasks` MUST require request `tasks.Ctx`, return nil on zero matches, and otherwise return `NestedTasksResults` with shared params/splat, per-pattern map/slice, and task participation metadata.
- `KIT-MUX-046` Nested task execution and proxy allocation behavior.
`RunNestedTasks` MUST execute matched task handlers in parallel, capture per-pattern data/error, and allocate response proxies per match (including handler-less matches).
- `KIT-MUX-047` Nested live-state and rebuild behavior.
`NestedRouter` live-state accessors (`AllRoutes`, `GetMatcher`) MUST expose live internal structures; rebuild APIs (`ReplaceRoutes`, `RebuildPreservingHandlers`) MUST atomically rebuild matcher/compiled state while preserving matcher option runes.
- `KIT-MUX-048` Nested pooling safety dependency behavior.
Nested req-data pooling safety MUST depend on `tasks.Ctx.RunParallel` remaining blocking-until-complete.
- `KIT-MUX-049` Middleware/registration helper wrapper equivalence behavior.
Receiver-method wrappers (`Router.SetGlobalHTTPMiddleware`, `Router.SetMethodLevelHTTPMiddleware`, `Route.SetPatternLevelHTTPMiddleware`, `Router.SetGlobalNotFoundHTTPHandler`, receiver `RegisterHandler*`) and package-level registration helpers MUST delegate to the same underlying behavior as their package-function counterparts.
- `KIT-MUX-050` ReqData accessor and request helper behavior.
`ReqData` accessors (`Params`, `Param`, `SplatValues`, `TasksCtx`, `Request`, `ResponseProxy`, `Input`) and request helpers (`GetTasksCtx`, `GetParams`, `GetParam`, `GetSplatValues`) MUST return stored transport data when present and safe zero/nil defaults when absent.
- `KIT-MUX-051` Nested registration-state query behavior.
`NestedRouter.IsRegistered` MUST report route-key presence and `NestedRouter.HasTaskHandler` MUST return true only when the route exists and has a non-nil task handler.
- `KIT-MUX-052` Nested task-result accessor behavior.
`NestedTasksResult` accessors (`Pattern`, `OK`, `Data`, `Err`, `RanTask`) MUST reflect stored execution state, and `NestedTasksResults.GetHasTaskHandler(i)` MUST return false for out-of-range indices.
- `KIT-MUX-053` Task adapter wrapper behavior.
`TaskHandlerFromFunc` and `TaskMiddlewareFromFunc` MUST wrap user functions in `tasks.Task` adapters that invoke the user function with the supplied `ReqData` and propagate return values/errors unchanged.
- `KIT-MUX-054` Router explicit-index getter behavior.
`Router.GetExplicitIndexSegment()` MUST return the router matcher explicit-index segment value; for `NewRouter` this value MUST default to empty string because `Options` does not expose explicit-index configuration.
- `KIT-MUX-055` `TasksCtxRequirer` adapter/marker behavior.
`TasksCtxRequirerFunc` MUST satisfy `TasksCtxRequirer` (`ServeHTTP` + marker `NeedsTasksCtx`), and `HandlerNeedsTasksCtxImplReflectType` MUST represent the interface type used by `RegisterHandler` for `needsTasksCtx` detection.
