# kit/tasks

`github.com/vormadev/vorma/kit/tasks`

The `tasks` package is the core primitive for orchestrating dependency-heavy
workflows.

A "task" is a function that takes input, returns data (or error), and runs at
most once per `(Ctx, task, input)` key, even if requested repeatedly during the
same execution context.

This gives you two major guarantees:

- Automatic deduplication for shared dependencies in a task DAG.
- Thundering-herd control in long-lived/shared contexts by collapsing concurrent
  calls onto one execution per key.

## Import

```go
import "github.com/vormadev/vorma/kit/tasks"
```

## Key Features

1. Automatic memoization per `(Ctx, task, input)`.
2. Strong typing with generics (`Task[I comparable, O any]`).
3. Concurrent-safe shared execution via `Ctx`.
4. First-class parallel execution (`RunParallel` + `Bind`).
5. Task composition with deduped shared dependencies.
6. Optional context-level TTL for cache expiration.
7. Context cancellation propagation through task execution.

## Mental Model

- A task is `func(*Ctx, I) (O, error)`.
- Input is part of the cache key (so input stability matters).
- Same task + same input + same `Ctx` runs once.
- Different input or different `Ctx` is a different execution key.

If tasks are defined as package-level variables, accidental task-dependency
cycles are usually caught by Go initialization-cycle checks at compile time.

## Defining Tasks

Always prefer package-level task definitions so dependency graphs are explicit.

```go
var FetchUserTask = tasks.NewTask(func(ctx *tasks.Ctx, id int) (*User, error) {
	return db.GetUser(ctx.NativeContext(), id)
})
```

`NewTask(nil)` returns `nil`.

## Running A Task

```go
ctx := tasks.NewCtx(context.Background())

user, err := FetchUserTask.Run(ctx, 123)
if err != nil {
	return err
}

// same key => cached result
user2, err := FetchUserTask.Run(ctx, 123)
if err != nil {
	return err
}

// different input => separate execution
user3, err := FetchUserTask.Run(ctx, 456)
if err != nil {
	return err
}

_ = user
_ = user2
_ = user3
```

## Parallel Task Execution

```go
var user *User
var orders []*Order
var profile *Profile

err := ctx.RunParallel(
	FetchUserTask.Bind(123, &user),
	FetchOrdersTask.Bind(123, &orders),
	FetchProfileTask.Bind(123, &profile),
)
if err != nil {
	return err
}
```

Behavior:

- Nil bound tasks are ignored.
- First error cancels sibling work through `errgroup` context cancellation.
- Shared dependencies still execute once per key.

## Composing Tasks

Tasks can call other tasks. Shared dependencies are automatically deduplicated
across the full composed graph.

```go
var EnrichedUserTask = tasks.NewTask(func(ctx *tasks.Ctx, userID int) (*EnrichedUser, error) {
	isSubscribed, err := FetchUserSubscriptionStatus.Run(ctx, userID)
	if err != nil || !isSubscribed {
		return nil, errors.New("subscription error")
	}

	var user *User
	var orders []*Order

	if err := ctx.RunParallel(
		FetchUserTask.Bind(userID, &user),
		FetchOrdersTask.Bind(userID, &orders),
	); err != nil {
		return nil, err
	}

	return &EnrichedUser{
		User:   user,
		Orders: orders,
	}, nil
})
```

## Time-To-Live (TTL) And Thundering-Herd Control

By default, a `Ctx` caches indefinitely for that context lifetime:

```go
ctx := tasks.NewCtx(parentCtx)
```

For long-lived/shared contexts, use TTL:

```go
ctx := tasks.NewCtxWithTTL(parentCtx, 5*time.Minute)
```

Primary use case:

- Reduce upstream load spikes by allowing callers to share cached results for a
  bounded period, then refresh.

When TTL is usually helpful:

- Background workers or services that reuse a `Ctx` beyond a single request.
- Expensive upstream calls where slightly stale data is acceptable.
- Global-level dedupe windows to reduce herd behavior.

TTL behavior details:

- `ttl == 0`: no expiration.
- `ttl < 0`: normalized to `0`.
- Expiration is per input key.
- Cleanup is lazy and runs at most once per TTL period during access.
- Errors are cached per key the same way as successes; after expiry, a new entry
  is created so retry can happen.

`NewCtx(nil)` and `NewCtxWithTTL(nil, ttl)` use `context.Background()`.

## Error And Cancellation Semantics

- If context is already canceled, run returns that context error.
- Task execution result (including error) is cached per key.
- `RunParallel` returns the first error encountered.

## `RunWithAnyInput`

`RunWithAnyInput` is for erased dispatch paths (`AnyTask`):

- If `input` has type `I`, that value is used.
- If `input` is not type `I`, it returns a type-mismatch error.

Prefer typed `Run` whenever possible.

## Practical Guidance

- Keep task inputs small, comparable, and semantically stable.
- Keep `Ctx` scope explicit (typically one request, job, or operation).
- Use TTL intentionally; for request-scoped contexts, no TTL is often enough.
- Prefer task composition over ad-hoc shared mutable caches.

## API Reference

- `type AnyTask`
- `type BoundTask`
- `type Ctx`
- `type Task[I comparable, O any]`
- `func NewTask[I comparable, O any](fn func(ctx *Ctx, input I) (O, error)) *Task[I, O]`
- `func NewCtx(parent context.Context) *Ctx`
- `func NewCtxWithTTL(parent context.Context, ttl time.Duration) *Ctx`
- `func (t *Task[I, O]) Run(ctx *Ctx, input I) (O, error)`
- `func (t *Task[I, O]) RunWithAnyInput(ctx *Ctx, input any) (any, error)`
- `func (t *Task[I, O]) Bind(input I, dest *O) BoundTask`
- `func (c *Ctx) NativeContext() context.Context`
- `func (c *Ctx) RunParallel(tasks ...BoundTask) error`
- `BoundTask` interface method: `Run(ctx *Ctx) error`
