# vorma-tasks

Standalone async task runtime with stable task declarations.

Tasks are process-lifetime nodes declared with `task!`. A task can memoize completed
results inside one execution context, extend successful results across execution contexts
for an explicit TTL, or run in `single_flight` mode where only in-flight duplicate work is
coalesced.

```rust
vorma_tasks::task! {
    static USER_TASK: vorma_tasks::Task<UserId, User, Error> =
        memoized(|ctx, user_id| async move {
            load_user(ctx, user_id).await
        });
}

vorma_tasks::task! {
    static STATS_TASK: vorma_tasks::Task<(), Stats, Error> =
        extended_cache(Duration::from_secs(30), |_ctx, ()| async move {
            load_stats().await
        });
}
```

Parallel execution uses one collection type:

```rust
let mut batch = vorma_tasks::ParallelBatch::new();
let user = batch.add(USER_TASK, user_id);
let stats = batch.add(STATS_TASK, ());
let outputs = batch.run(&ctx).await?;

let user = outputs.take(user);
let stats = outputs.take(stats);
```

All tasks in a `ParallelBatch` share execution-context memoization. The first failing
sibling cancels the remaining siblings.
