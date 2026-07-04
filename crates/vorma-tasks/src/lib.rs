//! Standalone async task runtime with stable task declarations, execution-context
//! memoization, optional cross-context caching, true spawned parallelism, and
//! cooperative cancellation. No HTTP types, no request/response assumptions — a
//! task can model a build step or a CLI computation exactly as naturally as a
//! request handler's dependency.
//!
//! # Getting started
//!
//! Declare one stable task node with [`task!`], then resolve it through a
//! [`Tasks`] runtime and an [`ExecCtx`]:
//!
//! ```
//! use vorma_tasks::{Tasks, TasksOptions};
//!
//! vorma_tasks::task! {
//!     static GREET: Task<String, String, &'static str> =
//!         memoized(|_ctx, name: String| async move { Ok(format!("Hello, {name}!")) });
//! }
//!
//! # #[tokio::main(flavor = "current_thread")]
//! # async fn main() {
//! let tasks: Tasks<&'static str> = Tasks::new(TasksOptions::default());
//! let ctx = tasks.exec_ctx(vorma_tasks::CancelToken::new());
//!
//! let greeting = GREET.run(&ctx, "Ferris".to_owned()).await.unwrap();
//! assert_eq!(*greeting, "Hello, Ferris!");
//! # }
//! ```
//!
//! [`Tasks`] owns the process-lifetime (or application-lifetime) shared
//! state; construct one per application and clone it wherever it is needed
//! (it is `Arc`-backed). [`Tasks::exec_ctx`] then creates one [`ExecCtx`]
//! per independent unit of work — one per HTTP request, one per CLI
//! invocation, one per build — and every [`Task::run`] call for that unit
//! of work goes through it.
//!
//! # The `task!` static-node model
//!
//! A task is not a value application code builds at runtime — it is a
//! `static` declared once with [`task!`], macro-expanded into a
//! [`Task<Input, Output, Error>`](Task) whose body is a capture-free
//! function pointer (the macro's expansion enforces this: nothing a
//! closure literal captures can survive into the declared body). `Task` is
//! `Copy`: passing one around, storing it, or capturing it in another
//! closure is just copying a name, a cache policy, and a function
//! pointer — it never clones any accumulated state, because a `Task`
//! handle carries none. Every declared task lazily receives a
//! process-unique [`TaskId`] on first access ([`Task::id`]), stable for
//! the life of the process but never guaranteed stable across restarts.
//!
//! # Three cache policies, chosen at declaration
//!
//! Every task declares exactly one of these, and the choice is part of the
//! task's identity — it cannot be changed per call:
//!
//! - **`memoized(body)`** retains a completed result — success *or*
//!   application error — for the rest of the [`ExecCtx`] it ran in.
//!   Resolving the same task with an equal input again inside that same
//!   context returns the retained outcome without re-running the body.
//!   Different contexts never share this state, even if built from the
//!   same [`Tasks`] runtime.
//! - **`extended_cache(ttl, body)`** does everything `memoized` does
//!   *inside* one context, and additionally publishes a **successful**
//!   result to the shared cache owned by the [`Tasks`] runtime, where it
//!   remains available to every future [`ExecCtx`] created from that same
//!   runtime until `ttl` elapses. A zero `Duration` is rejected at
//!   **compile time**: the task's constructor is a `const fn`, so
//!   declaring `extended_cache(Duration::ZERO, ...)` fails to compile with
//!   a const-evaluation error rather than merely panicking somewhere at
//!   runtime, because a zero-TTL "cache" is not a cache. Only successes
//!   are retained across contexts — an application error is never cached
//!   at this layer (though the same run's error is still `memoized` for
//!   the context that triggered it, per the rule above).
//!   The clock starts on completion, not on when the run began. A run that
//!   was cancelled mid-flight publishes nothing, even if the body had
//!   already produced `Ok` — a cancelled run's result, success or not, is
//!   always discarded rather than trusted.
//! - **`single_flight(body)`** coalesces concurrent duplicate calls — every
//!   caller racing the same task/input pair while a run is already in
//!   flight receives that one run's result — but retains nothing once the
//!   run completes. The very next call for the same input runs the body
//!   again from scratch. Reach for this when a result is only useful to
//!   callers racing *right now* (deduplicating a burst of identical
//!   in-flight work) and stale results would be actively wrong to serve
//!   later, unlike `memoized`, which is the right default whenever a
//!   retained result inside one execution context is safe to reuse.
//!
//! Both `memoized` and `extended_cache` are "shared" in the sense that
//! concurrent callers coalesce onto one run and all receive the same
//! outcome; what actually distinguishes them is cache **extent** — one
//! context's lifetime versus the runtime's — which is why the
//! cross-context policy is named for its extension, not called "shared."
//!
//! # Parallelism is real, not just concurrency
//!
//! [`ParallelBatch`] is the one collection type for running independent
//! task/input pairs together. Two or more tasks in a batch each become
//! their own spawned task (a real `tokio::spawn`, not merely a polled
//! future sharing one task slot), so CPU-bound sibling work genuinely
//! scales across executor threads — the crate's own contract (general
//! purpose: builds and CLIs need CPU-bound fan-out just as much as
//! requests do), not an implementation detail borrowed from how the
//! framework happens to use it. That means a batch pays a real per-sibling
//! spawn cost, so it earns its keep once siblings do enough independent
//! work (I/O, real per-task computation, or a shared coalesced
//! cancellation scope) to be worth it; a batch of exactly one task skips
//! spawning and runs inline. If any sibling fails, every other sibling in
//! flight is cancelled and the batch fails with that error; a batch is
//! all-or-nothing.
//!
//! # Cooperative, preemptive-at-boundaries cancellation
//!
//! [`CancelToken`] is host-agnostic: a build, a CLI, or a request handler
//! constructs one from whatever its own shutdown signal is and hands it to
//! [`Tasks::exec_ctx`]. Cancellation is observed at task-resolution
//! boundaries (before a run starts, immediately after one completes) and,
//! for a body that actually suspends instead of finishing synchronously,
//! races the body's own future so a genuinely long-running task is cut off
//! promptly rather than always running to completion. A result — even a
//! successful one — produced by a run that raced a cancellation is always
//! discarded, never memoized, never cached, and never handed to the
//! caller: cancellation always wins that race. [`ExecCtx::child`] and
//! [`ParallelBatch`] build a cancellation hierarchy from one token —
//! cancelling a parent cancels every descendant, cancelling a child never
//! reaches its parent or siblings.
//!
//! # Dependency cycles fail instead of deadlocking
//!
//! If resolving a task requires (transitively) resolving itself again on
//! the same call path, that resolution returns [`Error::Cycle`]
//! immediately instead of hanging forever waiting for a run that can never
//! complete — an improvement over designs that only prevent this by
//! relying on compile-time initialization ordering, which cannot protect
//! against cycles introduced purely by runtime input-dependent branching.
//!
//! # Passive observation
//!
//! Implement [`TaskObserver`] (or supply a plain closure, which implements
//! it automatically) and register it on [`TasksOptions::observer`] to
//! receive a [`TaskEvent`] for every cache hit/miss, run start/completion,
//! stale-entry cleanup, and cancellation — the shape a metrics exporter or
//! structured logger wants. Observation is genuinely free when nothing is
//! registered: event construction (including the clock read a duration
//! needs) is skipped entirely rather than merely discarded.

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
#![forbid(unsafe_code)]
// Loom builds compile only the store protocol; the async layer above it
// is gated out, leaving its helpers intentionally unreferenced.
#![cfg_attr(loom, allow(dead_code))]

#[cfg(not(loom))]
mod cancel;
mod clock;
mod error;
mod key;
#[cfg(not(loom))]
mod observer;
#[cfg(not(loom))]
mod overrides;
#[cfg(not(loom))]
mod parallel;
mod store;
mod sync;
#[cfg(not(loom))]
mod task;

#[cfg(all(loom, test))]
mod loom_tests;

#[cfg(not(loom))]
pub use cancel::CancelToken;
pub use clock::{Clock, ClockInstant, SystemClock};
pub use error::{Error, Result};
pub use key::TaskId;
#[cfg(not(loom))]
pub use observer::{TaskEvent, TaskEventKind, TaskEventOutcome, TaskObserver, TaskRunSource};
#[cfg(not(loom))]
pub use overrides::{TaskOverrideMode, TaskOverrides};
#[cfg(not(loom))]
pub use parallel::{ParallelBatch, ParallelBatchOutputHandle, ParallelBatchOutputs};
#[cfg(not(loom))]
#[doc(hidden)]
pub use task::__macro_support as __task_macro;
#[cfg(not(loom))]
pub use task::{ExecCtx, Task, Tasks, TasksOptions};

/// Declare one stable task node.
///
/// Expands to a `static` binding of type [`Task<Input, Output, Error>`
/// ](crate::Task) (or a caller-specified alias of it — see below). The
/// general shape:
///
/// ```text
/// task! {
///     static NAME: Task<Input, Output, Error> =
///         policy(body);
/// }
/// ```
///
/// `policy` is exactly one of `memoized`, `extended_cache(ttl, ...)`, or
/// `single_flight` — see the crate root docs for what each one retains and
/// for how long. `body` is `|ctx: ExecCtx<Error>, input: Input| async move
/// { ... }`, returning `Result<Output, Error>`; the closure's parameter
/// pattern can destructure the input directly (`|_ctx, (left, right):
/// (u32, u32)|`) exactly as an ordinary closure parameter would.
///
/// ```
/// use std::time::Duration;
///
/// vorma_tasks::task! {
///     static LOAD_USER: Task<u64, String, &'static str> =
///         memoized(|_ctx, user_id: u64| async move { Ok(format!("user-{user_id}")) });
/// }
///
/// vorma_tasks::task! {
///     static LOAD_STATS: Task<(), usize, &'static str> =
///         extended_cache(Duration::from_secs(30), |_ctx, ()| async move { Ok(42) });
/// }
///
/// vorma_tasks::task! {
///     static DEDUPED_FETCH: Task<u64, String, &'static str> =
///         single_flight(|_ctx, id: u64| async move { Ok(format!("fetched-{id}")) });
/// }
/// ```
///
/// The declared body is a plain `fn` under the macro's expansion, not a
/// closure literal that captures its environment — nothing outside the
/// macro invocation can be captured, so every task body's dependencies
/// must arrive through its `ctx` and `input` parameters. This is what
/// keeps a [`Task`](crate::Task) handle `Copy` and free of any hidden
/// captured state.
///
/// A task can depend on other tasks by calling
/// [`Task::run`](crate::Task::run) on them with the `ctx` its own body
/// received:
///
/// ```
/// vorma_tasks::task! {
///     static DOUBLE: Task<u32, u32, &'static str> =
///         memoized(|_ctx, input: u32| async move { Ok(input * 2) });
/// }
///
/// vorma_tasks::task! {
///     static DOUBLE_THEN_ADD_ONE: Task<u32, u32, &'static str> =
///         memoized(|ctx, input: u32| async move {
///             let doubled = DOUBLE.run(&ctx, input).await?;
///             Ok(*doubled + 1)
///         });
/// }
/// ```
///
/// `Task` in the declaration can also be any in-scope path or type alias
/// that resolves to [`Task`](crate::Task) itself (`::vorma_tasks::Task`,
/// or `type MyTask<I, O, E> = vorma_tasks::Task<I, O, E>;` and then
/// `MyTask<...>` in the declaration) — the expanded `static` is always a
/// real [`Task`](crate::Task) value underneath; only its spelling in the
/// declaration is flexible, to fit however an application chooses to
/// import or re-export the type.
#[macro_export]
macro_rules! task {
	(
		$(#[$meta:meta])*
		$vis:vis static $name:ident :
			$($declaration:tt)+
	) => {
		$crate::task! {
			@task_type
			[$( #[$meta] )*]
			$vis static $name :
				$($declaration)+
		}
	};
	(
		@task_type
		[$( #[$meta:meta] )*]
		$vis:vis static $name:ident :
			Task <$input:ty, $output:ty, $error:ty> =
			$($policy:tt)+
	) => {
		$crate::task! {
			@policy
			[$( #[$meta] )*]
			[$crate::Task]
			$vis static $name : Task<$input, $output, $error> =
				$($policy)+
		}
	};
	(
		@task_type
		[$( #[$meta:meta] )*]
		$vis:vis static $name:ident :
			$task_type:ident <$input:ty, $output:ty, $error:ty> =
			$($policy:tt)+
	) => {
		$crate::task! {
			@policy
			[$( #[$meta] )*]
			[$task_type]
			$vis static $name : Task<$input, $output, $error> =
				$($policy)+
		}
	};
	(
		@task_type
		[$( #[$meta:meta] )*]
		$vis:vis static $name:ident :
			$namespace:tt :: $($rest:tt)+
	) => {
		$crate::task! {
			@task_type_with_prefix
			[$( #[$meta] )*]
			[$namespace ::]
			$vis static $name:
				$($rest)+
		}
	};
	(
		@task_type
		[$( #[$meta:meta] )*]
		$vis:vis static $name:ident :
			:: $($rest:tt)+
	) => {
		$crate::task! {
			@task_type_with_prefix
			[$( #[$meta] )*]
			[::]
			$vis static $name:
				$($rest)+
		}
	};
	(
		@task_type_with_prefix
		[$( #[$meta:meta] )*]
		[$($task_type_prefix:tt)+]
		$vis:vis static $name:ident:
			Task <$input:ty, $output:ty, $error:ty> =
			$($policy:tt)+
	) => {
		$crate::task! {
			@policy
			[$( #[$meta] )*]
			[$($task_type_prefix)+ Task]
			$vis static $name : Task<$input, $output, $error> =
				$($policy)+
		}
	};
	(
		@task_type_with_prefix
		[$( #[$meta:meta] )*]
		[$($task_type_prefix:tt)+]
		$vis:vis static $name:ident:
			$task_type:ident <$input:ty, $output:ty, $error:ty> =
			$($policy:tt)+
	) => {
		$crate::task! {
			@policy
			[$( #[$meta] )*]
			[$($task_type_prefix)+ $task_type]
			$vis static $name : Task<$input, $output, $error> =
				$($policy)+
		}
	};
	(
		@task_type_with_prefix
		[$( #[$meta:meta] )*]
		[$($task_type_prefix:tt)+]
		$vis:vis static $name:ident:
			$namespace:tt :: $($rest:tt)+
	) => {
		$crate::task! {
			@task_type_with_prefix
			[$( #[$meta] )*]
			[$($task_type_prefix)+ $namespace ::]
			$vis static $name:
				$($rest)+
		}
	};
	(
		@policy
		[$( #[$meta:meta] )*]
		[$($task_type:tt)+]
		$vis:vis static $name:ident : Task<$input:ty, $output:ty, $error:ty> =
			memoized($task_fn:expr);
	) => {
		$crate::task! {
			@define
			[$( #[$meta] )*]
			[$($task_type)+]
			$vis static $name : Task<$input, $output, $error> =
				$crate::__task_macro::memoized,
				[],
				$task_fn;
		}
	};
	(
		@policy
		[$( #[$meta:meta] )*]
		[$($task_type:tt)+]
		$vis:vis static $name:ident : Task<$input:ty, $output:ty, $error:ty> =
			extended_cache($ttl:expr, $task_fn:expr);
	) => {
		$crate::task! {
			@define
			[$( #[$meta] )*]
			[$($task_type)+]
			$vis static $name : Task<$input, $output, $error> =
				$crate::__task_macro::extended_cache,
				[$ttl,],
				$task_fn;
		}
	};
	(
		@policy
		[$( #[$meta:meta] )*]
		[$($task_type:tt)+]
		$vis:vis static $name:ident : Task<$input:ty, $output:ty, $error:ty> =
			single_flight($task_fn:expr);
	) => {
		$crate::task! {
			@define
			[$( #[$meta] )*]
			[$($task_type)+]
			$vis static $name : Task<$input, $output, $error> =
				$crate::__task_macro::single_flight,
				[],
				$task_fn;
		}
	};
	(
		@define
		[$( #[$meta:meta] )*]
		[$($task_type:tt)+]
		$vis:vis static $name:ident : Task<$input:ty, $output:ty, $error:ty> =
			$constructor:path,
			[$($constructor_arg:expr,)*],
			$task_fn:expr;
	) => {
		$(#[$meta])*
		$vis static $name: $($task_type)+<$input, $output, $error> = {
			fn __vorma_task_call(
				ctx: $crate::ExecCtx<$error>,
				input: $input,
			) -> ::std::pin::Pin<
				::std::boxed::Box<
					dyn ::std::future::Future<
						Output = $crate::Result<$output, $error>,
					> + ::std::marker::Send + 'static,
				>,
			> {
				fn __vorma_task_invoke<F, Fut>(
					f: F,
					ctx: $crate::ExecCtx<$error>,
					input: $input,
				) -> Fut
				where
					F: ::std::ops::FnOnce($crate::ExecCtx<$error>, $input) -> Fut,
					Fut: ::std::future::Future<
						Output = $crate::Result<$output, $error>,
					> + ::std::marker::Send + 'static,
				{
					f(ctx, input)
				}

				::std::boxed::Box::pin(__vorma_task_invoke($task_fn, ctx, input))
			}
			static ID: ::std::sync::atomic::AtomicU64 =
				::std::sync::atomic::AtomicU64::new(0);
			$constructor(
				&ID,
				::std::concat!(::std::module_path!(), "::", ::std::stringify!($name)),
				$($constructor_arg,)*
				__vorma_task_call,
			)
		};
	};
}
