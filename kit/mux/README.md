# kit/mux

`github.com/vormadev/vorma/kit/mux`

`mux` is Vorma's HTTP routing layer with two execution models:

- classic `http.Handler` routes
- typed task routes (`TaskHandler`) with `ReqData[I]` inputs and structured
  outputs

It also provides a `NestedRouter` for hierarchical pattern matching + parallel
nested task execution.

## Import

```go
import "github.com/vormadev/vorma/kit/mux"
```

## Start Here

### Standard HTTP routes

```go
router := mux.NewRouter(nil)

mux.RegisterHandlerFunc(router, http.MethodGet, "/healthz", func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})
```

### Typed task routes

```go
type CreateUserInput struct {
	Name string `json:"name"`
}

type CreateUserOutput struct {
	ID string `json:"id"`
}

router := mux.NewRouter(&mux.Options{
	ParseInput: func(r *http.Request, inputPtr any) error {
		return json.NewDecoder(r.Body).Decode(inputPtr)
	},
})

mux.RegisterTaskHandler(router, http.MethodPost, "/users",
	mux.TaskHandlerFromFunc(func(rd *mux.ReqData[CreateUserInput]) (CreateUserOutput, error) {
		return CreateUserOutput{ID: "user-" + rd.Input().Name}, nil
	}),
)
```

## Matching And Dispatch

- method + pattern matching is per-method
- `HEAD` falls back to `GET` when no explicit `HEAD` route exists
- `MountRoot` strips configured prefix before route matching
- dynamic params and splat are powered by `kit/matcher`

Helpers for handlers:

- `mux.GetParams(r)`
- `mux.GetParam(r, "id")`
- `mux.GetSplatValues(r)`

## Middleware Model

HTTP middleware can be attached at three levels:

- global
- method
- route/pattern

Execution order is:

- global outermost
- method middle
- pattern innermost

Task middleware (`TaskMiddleware`) also supports global/method/pattern levels.

Task middleware behavior:

- all applicable task middlewares run in parallel
- response side effects are merged via `response.MergeProxyResponses`
- if merged proxy is error or redirect, main handler is skipped
- if any task middleware returns a non-nil Go error, request returns `500`

## `ReqData` In Task Handlers

`ReqData[I]` gives task handlers:

- typed input via `Input()`
- route params/splat
- request reference
- `tasks.Ctx`
- `response.Proxy` controls

You can set response state from tasks with wrappers like:

- `SetResponseStatus`
- `SetResponseHeader` / `AddResponseHeader`
- `SetResponseCookie`
- `Redirect`
- `HeadEls`

## `tasks.Ctx` Availability

- task routes and any route with task middleware run with a `tasks.Ctx`
- HTTP handlers can get it via `mux.GetTasksCtx(r)` when present
- for non-task stacks, use `mux.InjectTasksCtxMiddleware` to force-inject a
  `tasks.Ctx`
- handlers implementing `TasksCtxRequirer` are run on the slow path so
  `tasks.Ctx` is available

## Nested Router

Use `NestedRouter` when one URL should match multiple nested patterns and
optionally run multiple nested tasks.

```go
nr := mux.NewNestedRouter(nil)

mux.RegisterNestedPatternWithoutHandler(nr, "")
mux.RegisterNestedTaskHandler(nr, "/users",
	mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (string, error) {
		return "users", nil
	}),
)
mux.RegisterNestedTaskHandler(nr, "/users/:id",
	mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (string, error) {
		return rd.Param("id"), nil
	}),
)

results, ok := mux.FindNestedMatchesAndRunTasks(nr, r)
if ok {
	_ = results.Map
	_ = results.Slice
}
```

`NestedTasksResults` includes:

- shared params/splat for the matched path
- per-pattern task result (`Map`, `Slice`)
- per-pattern response proxies for downstream merge/application

## Lifecycle And Concurrency Assumptions

- Register routes and middleware before serving traffic.
- `Route.httpChain` is compiled and cached on first use, so adding HTTP
  middleware after first hit will not affect that route's cached chain.
- `Router.AllRoutes()` returns the underlying route slice.
- `NestedRouter.AllRoutes()` returns the underlying route map.
- `NestedRouter.GetMatcher()` returns the live matcher pointer.

Treat returned slices/maps/matcher as internal live state; avoid mutating them
from request paths.

## Public API Reference

### Core Type Aliases

- `type None = genericsutil.None`
- `type Params = matcher.Params`
- `type NestedReqData = ReqData[None]`
- `type HTTPMiddleware = func(http.Handler) http.Handler`
- `type TaskHandler[I any, O any] = tasks.Task[*ReqData[I], O]`
- `type TaskMiddleware[O any] = tasks.Task[*ReqData[None], O]`

### Core Types

- `type MiddlewareOptions struct`
- `type Options struct`
- `type Router struct`
- `type Route[I, O any] struct`
- `type ReqData[I any] struct`
- `type TaskHandlerFunc[I any, O any] func(*ReqData[I]) (O, error)`
- `type TaskMiddlewareFunc[O any] func(*ReqData[None]) (O, error)`
- `type TasksCtxRequirer interface { http.Handler; NeedsTasksCtx() }`
- `type TasksCtxRequirerFunc func(http.ResponseWriter, *http.Request)`
- `type AnyRoute interface`

### Nested Types

- `type NestedOptions struct`
- `type NestedRouter struct`
- `type NestedRoute[O any] struct`
- `type AnyNestedRoute interface`
- `type NestedTasksResult struct`
- `type NestedTasksResults struct`

### Exported Struct Fields

`MiddlewareOptions`:

- `If func(r *http.Request) bool`

`Options`:

- `MountRoot string`
- `DynamicParamPrefixRune rune`
- `SplatSegmentRune rune`
- `ParseInput func(r *http.Request, inputPtr any) error`

`NestedOptions`:

- `DynamicParamPrefixRune rune`
- `SplatSegmentRune rune`
- `ExplicitIndexSegment string`

`NestedTasksResults`:

- `Params Params`
- `SplatValues []string`
- `Map map[string]*NestedTasksResult`
- `Slice []*NestedTasksResult`
- `ResponseProxies []*response.Proxy`

### Functions

- `func NewRouter(options ...*Options) *Router`
- `func NewNestedRouter(opts *NestedOptions) *NestedRouter`
- `func TaskHandlerFromFunc[I any, O any](taskHandlerFunc TaskHandlerFunc[I, O]) *TaskHandler[I, O]`
- `func TaskMiddlewareFromFunc[O any](userFunc TaskMiddlewareFunc[O]) *TaskMiddleware[O]`
- `func RegisterHandler(router *Router, method, pattern string, httpHandler http.Handler) *Route[any, any]`
- `func RegisterHandlerFunc(router *Router, method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]`
- `func RegisterTaskHandler[I any, O any](router *Router, method, pattern string, taskHandler *TaskHandler[I, O]) *Route[I, O]`
- `func SetGlobalHTTPMiddleware(router *Router, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func SetMethodLevelHTTPMiddleware(router *Router, method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func SetPatternLevelHTTPMiddleware[I any, O any](route *Route[I, O], httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func SetGlobalTaskMiddleware[O any](router *Router, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)`
- `func SetMethodLevelTaskMiddleware[O any](router *Router, method string, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)`
- `func SetPatternLevelTaskMiddleware[PI any, PO any, MWO any](route *Route[PI, PO], taskMw *TaskMiddleware[MWO], opts ...*MiddlewareOptions)`
- `func SetGlobalNotFoundHTTPHandler(router *Router, httpHandler http.Handler)`
- `func InjectTasksCtxMiddleware(next http.Handler) http.Handler`
- `func GetTasksCtx(r *http.Request) *tasks.Ctx`
- `func GetParams(r *http.Request) Params`
- `func GetParam(r *http.Request, key string) string`
- `func GetSplatValues(r *http.Request) []string`
- `func RegisterNestedTaskHandler[O any](router *NestedRouter, pattern string, taskHandler *TaskHandler[None, O]) *NestedRoute[O]`
- `func RegisterNestedPatternWithoutHandler(router *NestedRouter, pattern string)`
- `func FindNestedMatches(nestedRouter *NestedRouter, r *http.Request) (*matcher.FindNestedMatchesResults, bool)`
- `func FindNestedMatchesAndRunTasks(nestedRouter *NestedRouter, r *http.Request) (*NestedTasksResults, bool)`
- `func RunNestedTasks(nestedRouter *NestedRouter, r *http.Request, findNestedMatchesResults *matcher.FindNestedMatchesResults) *NestedTasksResults`

### Methods

`Router`:

- `func (rt *Router) AllRoutes() []AnyRoute`
- `func (rt *Router) GetExplicitIndexSegment() string`
- `func (rt *Router) GetDynamicParamPrefixRune() rune`
- `func (rt *Router) GetSplatSegmentRune() rune`
- `func (rt *Router) MountRoot(optionalPatternToAppend ...string) string`
- `func (rt *Router) RegisterHandler(method, pattern string, httpHandler http.Handler) *Route[any, any]`
- `func (rt *Router) RegisterHandlerFunc(method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]`
- `func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)`
- `func (rt *Router) SetGlobalHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) SetMethodLevelHTTPMiddleware(method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) SetGlobalNotFoundHTTPHandler(httpHandler http.Handler)`

`Route[I,O]`:

- `func (route *Route[I, O]) OriginalPattern() string`
- `func (route *Route[I, O]) Method() string`
- `func (route *Route[I, O]) SetPatternLevelHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`

`ReqData[I]`:

- `func (rd *ReqData[I]) Params() Params`
- `func (rd *ReqData[I]) Param(key string) string`
- `func (rd *ReqData[I]) SplatValues() []string`
- `func (rd *ReqData[I]) TasksCtx() *tasks.Ctx`
- `func (rd *ReqData[I]) Request() *http.Request`
- `func (rd *ReqData[I]) ResponseProxy() *response.Proxy`
- `func (rd *ReqData[I]) Input() I`
- `func (rd *ReqData[I]) HeadEls() *headels.HeadEls`
- `func (rd *ReqData[I]) Redirect(url string, code ...int)`
- `func (rd *ReqData[I]) SetResponseStatus(status int, errorText ...string)`
- `func (rd *ReqData[I]) SetResponseCookie(cookie *http.Cookie)`
- `func (rd *ReqData[I]) SetResponseHeader(key, value string)`
- `func (rd *ReqData[I]) AddResponseHeader(key, value string)`
- `func (rd *ReqData[I]) GetResponseStatus() (int, string)`
- `func (rd *ReqData[I]) GetResponseHeader(key string) string`
- `func (rd *ReqData[I]) GetResponseHeaders(key string) []string`
- `func (rd *ReqData[I]) GetResponseCookies() []*http.Cookie`
- `func (rd *ReqData[I]) GetResponseLocation() string`
- `func (rd *ReqData[I]) IsResponseError() bool`
- `func (rd *ReqData[I]) IsResponseRedirect() bool`
- `func (rd *ReqData[I]) IsResponseSuccess() bool`

`TasksCtxRequirerFunc`:

- `func (h TasksCtxRequirerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request)`
- `func (h TasksCtxRequirerFunc) NeedsTasksCtx()`

`NestedRouter`:

- `func (nr *NestedRouter) AllRoutes() map[string]AnyNestedRoute`
- `func (nr *NestedRouter) IsRegistered(originalPattern string) bool`
- `func (nr *NestedRouter) HasTaskHandler(originalPattern string) bool`
- `func (nr *NestedRouter) GetExplicitIndexSegment() string`
- `func (nr *NestedRouter) GetDynamicParamPrefixRune() rune`
- `func (nr *NestedRouter) GetSplatSegmentRune() rune`
- `func (nr *NestedRouter) GetMatcher() *matcher.Matcher`
- `func (nr *NestedRouter) ReplaceRoutes(newRoutes map[string]AnyNestedRoute)`
- `func (nr *NestedRouter) RebuildPreservingHandlers(patterns []string)`

`NestedRoute[O]`:

- `func (route *NestedRoute[O]) OriginalPattern() string`

`NestedTasksResult`:

- `func (ntr *NestedTasksResult) Pattern() string`
- `func (ntr *NestedTasksResult) OK() bool`
- `func (ntr *NestedTasksResult) Data() any`
- `func (ntr *NestedTasksResult) Err() error`
- `func (ntr *NestedTasksResult) RanTask() bool`

`NestedTasksResults`:

- `func (ntr *NestedTasksResults) GetHasTaskHandler(i int) bool`
