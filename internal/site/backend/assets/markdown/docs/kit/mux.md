---
title: mux
description:
    HTTP router with task handlers, middleware, and nested layout matching.
---

```go
import "github.com/vormadev/vorma/kit/mux"
```

## Types

```go
type None = genericsutil.None
type Params = matcher.Params
type TaskHandler[I any, O any] = tasks.Task[*ReqData[I], O]
type TaskMiddleware[O any] = tasks.Task[*ReqData[None], O]
type HTTPMiddleware = func(http.Handler) http.Handler
type TaskHandlerFunc[I any, O any] func(*ReqData[I]) (O, error)
type TaskMiddlewareFunc[O any] func(*ReqData[None]) (O, error)
```

## Router

Standard HTTP router with task handlers and middleware support.

```go
type Options struct {
    MountRoot              string // Strip prefix from incoming paths
    DynamicParamPrefixRune rune   // Default: ':'
    SplatSegmentRune       rune   // Default: '*'
    ParseInput             func(r *http.Request, inputPtr any) error // Required for task handlers
}

func NewRouter(options ...*Options) *Router

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (rt *Router) AllRoutes() []AnyRoute
func (rt *Router) MountRoot(optionalPatternToAppend ...string) string
func (rt *Router) GetExplicitIndexSegment() string
func (rt *Router) GetDynamicParamPrefixRune() rune
func (rt *Router) GetSplatSegmentRune() rune
```

### Route Registration

Task handlers return JSON responses. Use standard `http.Handler` for other
content types.

```go
func RegisterTaskHandler[I any, O any](
    router *Router, method, pattern string, taskHandler *TaskHandler[I, O],
) *Route[I, O]

func RegisterHandler(
    router *Router, method, pattern string, httpHandler http.Handler,
) *Route[any, any]

func RegisterHandlerFunc(
    router *Router, method, pattern string, httpHandlerFunc http.HandlerFunc,
) *Route[any, any]

// Method variants
func (rt *Router) RegisterHandler(method, pattern string, httpHandler http.Handler) *Route[any, any]
func (rt *Router) RegisterHandlerFunc(method, pattern string, httpHandlerFunc http.HandlerFunc) *Route[any, any]
```

### Task Handler Creation

```go
func TaskHandlerFromFunc[I any, O any](taskHandlerFunc TaskHandlerFunc[I, O]) *TaskHandler[I, O]
func TaskMiddlewareFromFunc[O any](userFunc TaskMiddlewareFunc[O]) *TaskMiddleware[O]
```

### Middleware

Middleware levels (outer to inner): global → method → pattern.

```go
type MiddlewareOptions struct {
    If func(r *http.Request) bool // Conditional execution
}

func SetGlobalHTTPMiddleware(router *Router, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
func SetGlobalTaskMiddleware[O any](router *Router, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)
func SetMethodLevelHTTPMiddleware(router *Router, method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
func SetMethodLevelTaskMiddleware[O any](router *Router, method string, taskMw *TaskMiddleware[O], opts ...*MiddlewareOptions)
func SetPatternLevelHTTPMiddleware[I any, O any](route *Route[I, O], httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
func SetPatternLevelTaskMiddleware[PI any, PO any, MWO any](route *Route[PI, PO], taskMw *TaskMiddleware[MWO], opts ...*MiddlewareOptions)

func SetGlobalNotFoundHTTPHandler(router *Router, httpHandler http.Handler)

// Method variants
func (rt *Router) SetGlobalHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
func (rt *Router) SetMethodLevelHTTPMiddleware(method string, httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
func (rt *Router) SetGlobalNotFoundHTTPHandler(httpHandler http.Handler)
func (route *Route[I, O]) SetPatternLevelHTTPMiddleware(httpMw HTTPMiddleware, opts ...*MiddlewareOptions)
```

### Route

```go
type Route[I, O any] struct { ... }

func (route *Route[I, O]) OriginalPattern() string
func (route *Route[I, O]) Method() string
```

## ReqData

Request context for task handlers.

```go
type ReqData[I any] struct { ... }

func (rd *ReqData[I]) Params() Params
func (rd *ReqData[I]) Param(key string) string
func (rd *ReqData[I]) SplatValues() []string
func (rd *ReqData[I]) TasksCtx() *tasks.Ctx
func (rd *ReqData[I]) Request() *http.Request
func (rd *ReqData[I]) ResponseProxy() *response.Proxy
func (rd *ReqData[I]) Input() I

// Response helpers
func (rd *ReqData[I]) HeadEls() *headels.HeadEls
func (rd *ReqData[I]) Redirect(url string, code ...int)
func (rd *ReqData[I]) SetResponseStatus(status int, errorText ...string)
func (rd *ReqData[I]) SetResponseCookie(cookie *http.Cookie)
func (rd *ReqData[I]) SetResponseHeader(key, value string)
func (rd *ReqData[I]) AddResponseHeader(key, value string)
func (rd *ReqData[I]) GetResponseStatus() (int, string)
func (rd *ReqData[I]) GetResponseHeader(key string) string
func (rd *ReqData[I]) GetResponseHeaders(key string) []string
func (rd *ReqData[I]) GetResponseCookies() []*http.Cookie
func (rd *ReqData[I]) GetResponseLocation() string
func (rd *ReqData[I]) IsResponseError() bool
func (rd *ReqData[I]) IsResponseRedirect() bool
func (rd *ReqData[I]) IsResponseSuccess() bool
```

### Request Context Accessors

For use in standard `http.Handler` when registered via mux.

```go
func GetTasksCtx(r *http.Request) *tasks.Ctx
func GetParams(r *http.Request) Params
func GetParam(r *http.Request, key string) string
func GetSplatValues(r *http.Request) []string
```

### TasksCtx Injection

For handlers that need `*tasks.Ctx` but aren't task handlers.

```go
type TasksCtxRequirer interface {
    http.Handler
    NeedsTasksCtx()
}

type TasksCtxRequirerFunc func(http.ResponseWriter, *http.Request)
func (h TasksCtxRequirerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (h TasksCtxRequirerFunc) NeedsTasksCtx()

func InjectTasksCtxMiddleware(next http.Handler) http.Handler
```

---

## NestedRouter

Router for nested/layout-based matching (Remix/Next.js style). Returns all
matching patterns in depth order.

```go
type NestedOptions struct {
    DynamicParamPrefixRune rune
    SplatSegmentRune       rune
    ExplicitIndexSegment   string // e.g., "_index"
}

func NewNestedRouter(opts *NestedOptions) *NestedRouter

func (nr *NestedRouter) AllRoutes() map[string]AnyNestedRoute
func (nr *NestedRouter) IsRegistered(originalPattern string) bool
func (nr *NestedRouter) HasTaskHandler(originalPattern string) bool
func (nr *NestedRouter) GetExplicitIndexSegment() string
func (nr *NestedRouter) GetDynamicParamPrefixRune() rune
func (nr *NestedRouter) GetSplatSegmentRune() rune
func (nr *NestedRouter) GetMatcher() *matcher.Matcher
```

### Nested Route Registration

```go
type NestedReqData = ReqData[None]

func RegisterNestedTaskHandler[O any](
    router *NestedRouter, pattern string, taskHandler *TaskHandler[None, O],
) *NestedRoute[O]

func RegisterNestedPatternWithoutHandler(router *NestedRouter, pattern string)
```

### Matching and Execution

```go
func FindNestedMatches(nestedRouter *NestedRouter, r *http.Request) (*matcher.FindNestedMatchesResults, bool)
func FindNestedMatchesAndRunTasks(nestedRouter *NestedRouter, r *http.Request) (*NestedTasksResults, bool)
func RunNestedTasks(
    nestedRouter *NestedRouter,
    r *http.Request,
    findNestedMatchesResults *matcher.FindNestedMatchesResults,
) *NestedTasksResults
```

### Results

```go
type NestedTasksResults struct {
    Params          Params
    SplatValues     []string
    Map             map[string]*NestedTasksResult
    Slice           []*NestedTasksResult
    ResponseProxies []*response.Proxy
}

func (ntr *NestedTasksResults) GetHasTaskHandler(i int) bool

type NestedTasksResult struct { ... }

func (ntr *NestedTasksResult) Pattern() string
func (ntr *NestedTasksResult) OK() bool
func (ntr *NestedTasksResult) Data() any
func (ntr *NestedTasksResult) Err() error
func (ntr *NestedTasksResult) RanTask() bool
```

### Route Replacement (Dev Mode)

```go
func (nr *NestedRouter) ReplaceRoutes(newRoutes map[string]AnyNestedRoute)
func (nr *NestedRouter) RebuildPreservingHandlers(patterns []string)
```

`RebuildPreservingHandlers` atomically rebuilds the router, keeping routes with
task handlers and replacing handler-less routes with the provided patterns.
Intended for dev-time hot reload.

---

## Example

```go
r := mux.NewRouter(&mux.Options{
    ParseInput: func(r *http.Request, iPtr any) error {
        return json.NewDecoder(r.Body).Decode(iPtr)
    },
})

type CreateUserInput struct {
    Name string `json:"name"`
}
type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

handler := mux.TaskHandlerFromFunc(func(rd *mux.ReqData[CreateUserInput]) (*User, error) {
    return &User{ID: "123", Name: rd.Input().Name}, nil
})

mux.RegisterTaskHandler(r, "POST", "/users", handler)

// Standard handler with params
r.RegisterHandlerFunc("GET", "/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := mux.GetParam(r, "id")
    fmt.Fprintf(w, "User: %s", id)
})

http.ListenAndServe(":8080", r)
```
