//! Standalone task runtime and cache primitive.
//!
//! Tasks are stable process-lifetime nodes declared with [`task!`].
//! `memoized` tasks retain results by task/input inside one [`ExecCtx`].
//! `extended_cache` tasks also retain successful results for later execution contexts
//! created from the same [`Tasks`] runtime. `single_flight` tasks coalesce concurrent
//! duplicate calls without retaining completed results.

#![deny(missing_docs)]
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
