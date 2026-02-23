# kit/nestedmux

`github.com/vormadev/vorma/kit/nestedmux`

Nested router that builds on `kit/nestedmatcher` and runs matched task handlers.

Use this package when a single request path should resolve to multiple nested
patterns and optionally execute handlers for each matched pattern.

## Import

```go
import "github.com/vormadev/vorma/kit/nestedmux"
```

## Quick Start

```go
router := nestedmux.NewRouter(nil)

nestedmux.AddPatternWithoutHandler(router, "")
nestedmux.AddTaskHandler(
	router,
	"/users/:id",
	mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (string, error) {
		return rd.Param("id"), nil
	}),
)

req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
req = mux.RequestWithTasksCtx(req, tasks.NewCtx(context.Background()))

results, ok := nestedmux.FindMatchesAndRunTasks(router, req)
if ok {
	_ = results.Map["/users/:id"].Data() // "42"
}
```

## Router Options

Defaults:

- `DynamicParamPrefix`: `':'`
- `SplatSegmentIdentifier`: `'*'`
- `ExplicitIndexSegmentIdentifier`: `""` (disabled)

## Core Behavior

- `FindMatches` returns nested matcher results for `r.URL.Path`.
- `FindMatchesAndRunTasks` runs task handlers for matched patterns.
- `TasksResults.Slice` preserves match order.
- `TasksResults.Map` gives direct lookup by pattern.
- `TasksResult.RanTask()` is `false` for matched patterns without handlers.

## Task Execution Notes

- Requests must carry a `tasks.Ctx` (`mux.RequestWithTasksCtx` is the helper).
- `RunTasks` runs matched tasks in parallel.
- On task error, descendant task contexts are canceled while already-running
  sibling tasks can still complete.
- Task results always include per-pattern response proxies for downstream merge.

## Lifecycle And Concurrency Notes

- Register routes before serving traffic.
- `AllRoutes` and `Matcher` return defensive copies.
- `AddPatternWithoutHandlerIfMissing` is safe to call concurrently for the same
  router instance.
- `ReplaceRoutes` and `RebuildPreservingHandlers` support atomic route table
  replacement for dynamic rebuild flows.

## Public API

### Types

- `type ReqData`
- `type Options`
- `type Router`
- `type Route[O any]`
- `type AnyRoute`
- `type TasksResult`
- `type TasksResults`

### Functions

- `func NewRouter(opts *Options) *Router`
- `func AddTaskHandler[O any](router *Router, pattern string, taskHandler *mux.TaskHandler[mux.None, O]) *Route[O]`
- `func AddPatternWithoutHandler(router *Router, pattern string)`
- `func FindMatches(nestedRouter *Router, r *http.Request) (*nestedmatcher.Results, bool)`
- `func FindMatchesAndRunTasks(nestedRouter *Router, r *http.Request) (*TasksResults, bool)`
- `func RunTasks(nestedRouter *Router, r *http.Request, findNestedMatchesResults *nestedmatcher.Results) *TasksResults`

### `Router` methods

- `func (nr *Router) AllRoutes() map[string]AnyRoute`
- `func (nr *Router) IsRegistered(originalPattern string) bool`
- `func (nr *Router) HasTaskHandler(originalPattern string) bool`
- `func (nr *Router) ExplicitIndexSegmentIdentifier() string`
- `func (nr *Router) DynamicParamPrefix() rune`
- `func (nr *Router) SplatSegmentIdentifier() rune`
- `func (nr *Router) Matcher() *nestedmatcher.Matcher`
- `func (nr *Router) AddPatternWithoutHandlerIfMissing(pattern string) bool`
- `func (nr *Router) ReplaceRoutes(newRoutes map[string]AnyRoute)`
- `func (nr *Router) RebuildPreservingHandlers(patterns []string)`

### `Route` methods

- `func (route *Route[O]) OriginalPattern() string`

### `TasksResult` methods

- `func (ntr *TasksResult) Pattern() string`
- `func (ntr *TasksResult) OK() bool`
- `func (ntr *TasksResult) Data() any`
- `func (ntr *TasksResult) Err() error`
- `func (ntr *TasksResult) RanTask() bool`

### `TasksResults` methods

- `func (ntr *TasksResults) HasTaskHandlerAt(i int) bool`
