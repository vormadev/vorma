# kit/mux

`github.com/vormadev/vorma/kit/mux`

`mux` is Vorma's typed HTTP router.

Use it when you want method+path routing with:

- standard `http.Handler` routes
- typed task routes (`TaskHandler[I, O]`) backed by `tasks.Ctx`
- HTTP middleware and task middleware at global/method/pattern scope

Nested routing is not in this package anymore. Use
`github.com/vormadev/vorma/kit/nestedmux`.

## Import

```go
import "github.com/vormadev/vorma/kit/mux"
```

## Quick Start

### HTTP route

```go
router := mux.NewRouter()
router.AddHTTPHandlerFunc(http.MethodGet, "/healthz", func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})
```

### Typed task route

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

mux.AddTaskHandler(
	router,
	http.MethodPost,
	"/users",
	mux.TaskHandlerFromFunc(
		func(rd *mux.ReqData[CreateUserInput]) (CreateUserOutput, error) {
			return CreateUserOutput{ID: "user-" + rd.Input().Name}, nil
		},
	),
)
```

## Matching And Dispatch

- matching is per HTTP method
- `HEAD` falls back to `GET` if no explicit `HEAD` route exists
- `Options.MountRoot` strips a configured URL prefix before matching
- dynamic params and splats are handled by `kit/matcher`

Request lookup helpers:

- `mux.GetParams(r)`
- `mux.GetParam(r, "id")`
- `mux.GetSplatValues(r)`

## Middleware Model

HTTP middleware and task middleware each support:

- global scope
- method scope
- route/pattern scope

HTTP middleware execution order:

- global outermost
- method middle
- pattern innermost

Task middleware behavior:

- middleware tasks run in parallel through the request `tasks.Ctx`
- middleware response proxies are merged before route handler execution
- merged error/redirect proxy short-circuits the route handler
- middleware execution error returns HTTP 500

## `ReqData[I]` In Task Handlers

`ReqData[I]` exposes:

- `Input()`
- `Params()` and `Param(key)`
- `SplatValues()`
- `Request()`
- `TasksCtx()`
- `ResponseProxy()`

It also exposes response helpers:

- `SetResponseStatus`
- `SetResponseHeader` and `AddResponseHeader`
- `SetResponseCookie`
- `Redirect`
- `HeadEls`

## `tasks.Ctx` Availability

- task routes and requests with task middleware use a `tasks.Ctx`
- use `mux.GetTasksCtx(r)` to read it from request context
- use `mux.RequestWithTasksCtx(r, tasksCtx)` to inject it
- use `mux.InjectTasksCtxMiddleware(next)` to force injection for handler stacks
- handlers implementing `TasksCtxRequirer` are run on the tasks-context path

## Lifecycle And Concurrency Notes

- register routes and middleware before serving traffic
- `Route.httpChain` is cached; adding HTTP middleware after first hit on a route
  does not update that route's cached chain
- `Router.AllRoutes()` returns a defensive copy

## Public API

### Type aliases

- `type None = genericsutil.None`
- `type Params = matcher.Params`
- `type TaskHandler[I any, O any] = tasks.Task[*ReqData[I], O]`
- `type TaskMiddleware[O any] = tasks.Task[*ReqData[None], O]`
- `type HTTPMiddleware = func(http.Handler) http.Handler`

### Types

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

### Functions

- `func NewRouter(options ...*Options) *Router`
- `func TaskHandlerFromFunc[I any, O any](taskHandlerFunc TaskHandlerFunc[I, O]) *TaskHandler[I, O]`
- `func TaskMiddlewareFromFunc[O any](userFunc TaskMiddlewareFunc[O]) *TaskMiddleware[O]`
- `func AddHTTPHandler(router *Router, method, pattern string, httpHandler http.Handler) *Route[any, any]`
- `func AddHTTPHandlerFunc(router *Router, method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]`
- `func AddTaskHandler[I any, O any](router *Router, method, pattern string, taskHandler *TaskHandler[I, O]) *Route[I, O]`
- `func AddGlobalHTTPMiddleware(router *Router, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func AddMethodLevelHTTPMiddleware(router *Router, method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func AddPatternLevelHTTPMiddleware[I any, O any](route *Route[I, O], httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func AddGlobalTaskMiddleware[O any](router *Router, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)`
- `func AddMethodLevelTaskMiddleware[O any](router *Router, method string, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)`
- `func AddPatternLevelTaskMiddleware[PI any, PO any, MWO any](route *Route[PI, PO], taskMw *TaskMiddleware[MWO], opts ...*MiddlewareOptions)`
- `func SetGlobalNotFoundHTTPHandler(router *Router, httpHandler http.Handler)`
- `func InjectTasksCtxMiddleware(next http.Handler) http.Handler`
- `func GetTasksCtx(r *http.Request) *tasks.Ctx`
- `func RequestWithTasksCtx(request *http.Request, tasksCtx *tasks.Ctx) *http.Request`
- `func GetParam(r *http.Request, key string) string`
- `func GetParams(r *http.Request) Params`
- `func GetSplatValues(r *http.Request) []string`

### `Router` methods

- `func (rt *Router) AllRoutes() []AnyRoute`
- `func (rt *Router) DynamicParamPrefix() rune`
- `func (rt *Router) SplatSegmentIdentifier() rune`
- `func (rt *Router) MountRoot(optionalPatternToAppend ...string) string`
- `func (rt *Router) AddHTTPHandler(method, pattern string, httpHandler http.Handler) *Route[any, any]`
- `func (rt *Router) AddHTTPHandlerFunc(method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]`
- `func (rt *Router) AddGlobalHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) AddMethodLevelHTTPMiddleware(method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) SetGlobalNotFoundHTTPHandler(httpHandler http.Handler)`
- `func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)`

### `Route` methods

- `func (route *Route[I, O]) OriginalPattern() string`
- `func (route *Route[I, O]) Method() string`
- `func (route *Route[I, O]) AddPatternLevelHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`

### `ReqData` methods

- `func (rd *ReqData[I]) Params() Params`
- `func (rd *ReqData[I]) Param(key string) string`
- `func (rd *ReqData[I]) SplatValues() []string`
- `func (rd *ReqData[I]) TasksCtx() *tasks.Ctx`
- `func (rd *ReqData[I]) Request() *http.Request`
- `func (rd *ReqData[I]) ResponseProxy() *response.Proxy`
- `func (rd *ReqData[I]) Input() I`
- `func (rd *ReqData[I]) SetTasksCtx(tasksCtx *tasks.Ctx)`
- `func (rd *ReqData[I]) ResetForReuse(params Params, splatValues []string, input I, req *http.Request, responseProxy *response.Proxy)`
- `func (rd *ReqData[I]) ClearForPool()`
- `func (rd *ReqData[I]) HeadEls() *headels.HeadEls`
- `func (rd *ReqData[I]) Redirect(url string, code ...int) (bool, error)`
- `func (rd *ReqData[I]) SetResponseStatus(status int, errorText ...string)`
- `func (rd *ReqData[I]) SetResponseCookie(cookie *http.Cookie)`
- `func (rd *ReqData[I]) SetResponseHeader(key, value string)`
- `func (rd *ReqData[I]) AddResponseHeader(key, value string)`
- `func (rd *ReqData[I]) ResponseStatus() (int, string)`
- `func (rd *ReqData[I]) ResponseHeader(key string) string`
- `func (rd *ReqData[I]) ResponseHeaders(key string) []string`
- `func (rd *ReqData[I]) ResponseCookies() []*http.Cookie`
- `func (rd *ReqData[I]) ResponseLocation() string`
- `func (rd *ReqData[I]) IsResponseError() bool`
- `func (rd *ReqData[I]) IsResponseRedirect() bool`
- `func (rd *ReqData[I]) IsResponseSuccess() bool`
- `func (rt *Router) AddHTTPHandlerFunc(method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]`
- `func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)`
- `func (rt *Router) AddGlobalHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) AddMethodLevelHTTPMiddleware(method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`
- `func (rt *Router) SetGlobalNotFoundHTTPHandler(httpHandler http.Handler)`

`Route[I,O]`:

- `func (route *Route[I, O]) OriginalPattern() string`
- `func (route *Route[I, O]) Method() string`
- `func (route *Route[I, O]) AddPatternLevelHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)`

`ReqData[I]`:

- `func (rd *ReqData[I]) Params() Params`
- `func (rd *ReqData[I]) Param(key string) string`
- `func (rd *ReqData[I]) SplatValues() []string`
- `func (rd *ReqData[I]) TasksCtx() *tasks.Ctx`
- `func (rd *ReqData[I]) Request() *http.Request`
- `func (rd *ReqData[I]) ResponseProxy() *response.Proxy`
- `func (rd *ReqData[I]) Input() I`
- `func (rd *ReqData[I]) HeadEls() *headels.HeadEls`
- `func (rd *ReqData[I]) Redirect(url string, code ...int) (bool, error)`
- `func (rd *ReqData[I]) SetResponseStatus(status int, errorText ...string)`
- `func (rd *ReqData[I]) SetResponseCookie(cookie *http.Cookie)`
- `func (rd *ReqData[I]) SetResponseHeader(key, value string)`
- `func (rd *ReqData[I]) AddResponseHeader(key, value string)`
- `func (rd *ReqData[I]) ResponseStatus() (int, string)`
- `func (rd *ReqData[I]) ResponseHeader(key string) string`
- `func (rd *ReqData[I]) ResponseHeaders(key string) []string`
- `func (rd *ReqData[I]) ResponseCookies() []*http.Cookie`
- `func (rd *ReqData[I]) ResponseLocation() string`
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
- `func (nr *NestedRouter) ExplicitIndexSegmentIdentifier() string`
- `func (nr *NestedRouter) DynamicParamPrefix() rune`
- `func (nr *NestedRouter) SplatSegmentIdentifier() rune`
- `func (nr *NestedRouter) Matcher() *matcher.Matcher`
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

- `func (ntr *NestedTasksResults) HasTaskHandlerAt(i int) bool`
