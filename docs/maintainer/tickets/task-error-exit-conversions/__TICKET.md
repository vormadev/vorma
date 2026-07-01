# Task Error Exit Conversions

Status: open

Make task runs compose cleanly with Vorma handler exits while preserving error sources.

Current task-package baseline:

- Tasks are stable process-lifetime nodes declared with `vorma_tasks::task!`.
- Cache policies are `memoized`, `extended_cache`, and `single_flight`.
- Parallel task execution uses `ParallelBatch`: add task/input pairs, call
  `batch.run(ctx).await`, then consume typed output handles with `outputs.take(handle)`.
- `Task::new`, `Duration::ZERO` sentinel construction, tuple parallel APIs,
  `PreparedTask`, and public bound-item APIs are not the intended public surface.
- `vorma-tasks` remains sovereign. This ticket must not change task cache policy,
  parallelism, hashing, or constructor shape.

Current Board friction:

- Handler call sites map task errors by hand with
  `.map_err(|error| ViewExit::err(error.to_string()))` or the `HttpExit` equivalent.
- That loses the source chain that `ViewExit` and `HttpExit` are supposed to preserve.
- Plain `vorma::Error` and `BoxError` already convert into exit types; task runtime errors
  should compose with the same `?` ergonomics.
- Current Board code still contains those manual string mappings in
  `examples/board/src/views.rs` and `examples/board/src/resources.rs`.
- `crates/vorma/src/execution_engine.rs` has internal
  `HandlerExecutionError::from_task_error`, but that does not make `?` work in user
  handlers returning `ViewExit` or `HttpExit`.

Desired result:

- `?` works on `Task::run` and `ParallelBatch::run` inside view/resource/middleware
  handlers returning `ViewExit` or `HttpExit`.
- Task application failures preserve the original application error source chain.
- Task runtime failures such as cancellation, cycle detection, missing overrides, or type
  mismatches produce clear server-side exit records.
- Board task call sites stop flattening task errors to strings.

Constraints:

- `vorma-tasks` is a sovereign crate. Do not add Vorma-framework semantics to the task
  crate merely to make Vorma handlers convenient.
- If the conversion belongs in `vorma`, put it there. Keep the standalone tasks crate
  general-purpose.
- Do not change task cache policy, parallelism, constructor shape, or hashing while doing
  this ticket unless a separate, real issue is discovered and handled as its own work.

Verification expectations:

- Add focused tests proving task errors convert into `ViewExit` and `HttpExit` while
  preserving sources.
- Migrate Board call sites away from manual string mapping.
- Run the affected `vorma`, `vorma-tasks`, and Board request tests.

Done means:

- Handler task runs read like normal fallible operations.
- The error chain survives.
- The fix is layered in the right crate and does not disturb task-runtime semantics.
