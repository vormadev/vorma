---
title: tasks
description:
    Memoized task execution with once-per-context semantics, TTL support, and
    parallel execution.
---

```go
import "github.com/vormadev/vorma/kit/tasks"
```

## Context

Execution context that caches task results:

```go
func NewCtx(parent context.Context) *Ctx
func NewCtxWithTTL(parent context.Context, ttl time.Duration) *Ctx
func (c *Ctx) NativeContext() context.Context
func (c *Ctx) RunParallel(tasks ...BoundTask) error
```

## Task

Define a task (typically as package-level variable for circular dependency
protection):

```go
func NewTask[I comparable, O any](fn func(ctx *Ctx, input I) (O, error)) *Task[I, O]
```

Execute a task (runs once per ctx+input, subsequent calls return cached result):

```go
func (t *Task[I, O]) Run(ctx *Ctx, input I) (O, error)
func (t *Task[I, O]) RunWithAnyInput(ctx *Ctx, input any) (any, error)
```

## Bound Tasks (for parallel execution)

Bind task to input and destination pointer:

```go
func (t *Task[I, O]) Bind(input I, dest *O) BoundTask
```

Run bound tasks in parallel (maximizes parallelism based on dependency DAG):

```go
ctx.RunParallel(
    taskA.Bind(inputA, &resultA),
    taskB.Bind(inputB, &resultB),
)
```

## TaskResult

```go
type TaskResult struct {
    Data any
    Err  error
}
func (r *TaskResult) OK() bool
```

## Example

```go
// Define task (package level)
var GetUser = tasks.NewTask(func(ctx *tasks.Ctx, userID string) (*User, error) {
    return db.FetchUser(ctx.NativeContext(), userID)
})

// Execute (runs once per ctx+userID, even if called multiple times)
func handler(ctx *tasks.Ctx) {
    user, err := GetUser.Run(ctx, "user123")

    // Parallel execution
    var user *User
    var posts []Post
    err := ctx.RunParallel(
        GetUser.Bind("user123", &user),
        GetPosts.Bind("user123", &posts),
    )
}
```

## Task Composition

Tasks can call other tasks. Shared dependencies across the entire call graph are
automatically deduplicated:

```go
var EnrichedUser = tasks.NewTask(func(ctx *tasks.Ctx, userID string) (*Enriched, error) {
    // Safe to call in every task - only runs once per ctx+input
    sub, err := CheckSubscription.Run(ctx, userID)
    if err != nil { return nil, err }

    var user *User
    var posts []Post
    ctx.RunParallel(
        GetUser.Bind(userID, &user),
        GetPosts.Bind(userID, &posts),
    )
    return &Enriched{user, posts, sub}, nil
})
```

## TTL Behavior

- `NewCtx`: Results cached indefinitely within context lifetime
- `NewCtxWithTTL`: Results expire after TTL, re-executed on next access
- Expired entries lazily cleaned up (at most once per TTL period)
- **Error retry**: After TTL expiry, failed tasks can retry (new `TaskResult`
  created)

## Global Context + TTL for Thundering Herd Prevention

Use a long-lived `Ctx` with TTL to prevent all requests from hammering expensive
upstream resources:

```go
var globalCtx = tasks.NewCtxWithTTL(context.Background(), 5*time.Minute)

var FetchAPIConfig = tasks.NewTask(func(ctx *tasks.Ctx, _ struct{}) (*Config, error) {
    return expensiveExternalAPICall()
})

// In request handlers - all requests share cached result, refreshed every 5 min
func handler(w http.ResponseWriter, r *http.Request) {
    config, err := FetchAPIConfig.Run(globalCtx, struct{}{})
    // ...
}
```

## Important

**Define tasks at package level** to get Go's compile-time circular dependency
protection.
