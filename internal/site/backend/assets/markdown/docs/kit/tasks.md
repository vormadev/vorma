---
title: Tasks
description: vorma/kit/tasks
---

```go
import "github.com/vormadev/vorma/kit/tasks"
```

The `tasks` package is the single most important and innovative primitive on
which the entire Vorma framework is built.

A "Task", as used in this package, is simply a function that takes in input,
returns data (or an error), and runs a maximum of one time per
execution-context/input-value pairing (typically, but not necessarily, a web
request lifecycle), even if invoked repeatedly during the lifetime of the
execution context.

Tasks are automatically protected from circular deps by Go's compile-time
"initialization cycle" errors (assuming they are defined as package-level
variables).

When called with `ctx.RunParallel(...boundTasks)`, tasks run with the maximum
parallelism and deduplication possible based on the directed acyclic graph of
the task set.

## Key Features

1. **Automatic Memoization**: Tasks cache results per input within an execution
   context
2. **Circular Dependency Protection**: Go's compile-time checks prevent cycles
   (if tasks are defined at package level)
3. **Thread-Safe**: Safe for concurrent use
4. **Context Cancellation**: Respects Go context cancellation
5. **Type-Safe**: Generic types ensure compile-time safety
6. **Parallel Execution**: First-class ergonomic support for running multiple
   tasks concurrently
7. **Optional Ctx-Level TTL**: Time-based expiration for cached results to
   prevent thundering herd at a global level

## Usage Examples

### Defining Task

<lightbulb>
Always define tasks as package-level variables to ensure a safe dependency graph
guaranteed by the Go compiler.
</lightbulb>

```go
// Define a task at package level
var FetchUserTask = tasks.NewTask(func(ctx *tasks.Ctx, id int) (*User, error) {
	return db.GetUser(ctx.NativeContext(), id)
})
```

### Running a Task

```go
// Use the task
ctx := tasks.NewCtx(context.Background())
user, err := FetchUserTask.Run(ctx, 123) // user variable strongly typed

// Calling again with SAME input
// Will not run (instantly returns cached result)
user2, err := FetchUserTask.Run(ctx, 123)

// Calling again with DIFFERENT input
// Will run (if not yet run with this input value)
user3, err := FetchUserTask.Run(ctx, 456)
```

### Parallel Task Execution

```go
// Define multiple tasks (always as package-level variables)
var FetchUserTask = tasks.NewTask(fetchUser)
var FetchOrdersTask = tasks.NewTask(fetchOrders)
var FetchProfileTask = tasks.NewTask(fetchProfile)

func doUserStuff(userID int) {
	ctx := tasks.NewCtx(context.Background())

	var user *User
	var orders []*Order
	var profile *Profile

	// Run them in parallel
	err := ctx.RunParallel(
		FetchUserTask.Bind(userID, &user),
		FetchOrdersTask.Bind(userID, &orders),
		FetchProfileTask.Bind(userID, &profile),
	)
}
```

### Composing Tasks

Tasks can call other tasks, which can call other tasks, which can call other
tasks, and so on. Shared task dependencies across the entire task set will be
automatically deduplicated and only run once. The level of safety, efficiency,
and composability this gives you is extremely powerful.

For example, if you really wanted to, you could double check a user's
subscription status (or authentication status or what-have-you) inside of every
single other task across your entire app, and you'd never need to worry about it
running more than once per request (or other applicable execution context).

```go
// Task composition example
var EnrichedUserTask = tasks.NewTask(func(ctx *tasks.Ctx, userID int) (*EnrichedUser, error) {
	isSubscribed, err := FetchUserSubscriptionStatus.Run(ctx, userID)
	if err != nil || !isSubscribed {
		return nil, errors.New("subscription error")
	}

	var user *User
	var orders *Orders

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

## Time-To-Live (TTL) for Cache Expiration

By default, task results are cached indefinitely within a context's lifetime
(typically a single request). For longer-lived contexts or global-level caching,
you can use TTL to expire cached results after a specified duration.

### When to Use TTL

TTL is primarily useful for preventing thundering herd problems at a global
level:

- **Longer-lived contexts**: Background workers, daemons, or services that reuse
  the same task context across multiple operations
- **Rate limiting**: Preventing excessive calls to external APIs or databases
- **Stale data tolerance**: When slightly outdated data is acceptable and you
  want to reduce load on upstream services

<lightbulb>
You can vary the TTL by creating purpose-specific task context instances (e.g., a different TTL for user profiles than for API key validity, or whatever).
</lightbulb>

### Using TTL

```go
// Create a context with 5-minute TTL for cached results
ctx := tasks.NewCtxWithTTL(context.Background(), 5*time.Minute)

// First call executes the task
user, err := FetchUserTask.Run(ctx, 123)

// Within 5 minutes: uses cached result
user, err = FetchUserTask.Run(ctx, 123)

// After 5 minutes: re-executes the task with fresh data
time.Sleep(6 * time.Minute)
user, err = FetchUserTask.Run(ctx, 123)
```

### TTL Behavior Details

- **Lazy cleanup**: Expired entries are removed from memory at most once per TTL
  period during cache access
- **Per-input expiration**: Each unique input has its own expiration timer
- **Error caching**: Errors are cached via `sync.Once` within a `TaskResult`.
  After TTL expiration, a new `TaskResult` is created, allowing the task to
  retry
- **Thread-safe**: All TTL operations are safe for concurrent access
