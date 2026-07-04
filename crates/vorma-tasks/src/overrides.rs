use std::any::Any;
use std::collections::HashMap;
use std::future::Future;
use std::hash::Hash;
use std::marker::PhantomData;
use std::sync::Arc;

use crate::error::Result;
use crate::key::TaskId;
use crate::store::StoredOutcome;
use crate::task::{BoxFuture, ExecCtx, Task};

type OverrideFn<I, O, E> = dyn Fn(ExecCtx<E>, I) -> BoxFuture<Result<O, E>> + Send + Sync;

/// Policy for tasks that do not have an explicit test override.
///
/// Set on [`TaskOverrides::new`]; applies to every task reachable through a
/// [`Tasks`](crate::Tasks) runtime built with these overrides that has no
/// matching [`TaskOverrides::replace`] entry.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskOverrideMode {
	/// Run the original task body when no override is registered.
	///
	/// The natural default: only the tasks you explicitly stub with
	/// [`TaskOverrides::replace`] change behavior, everything else runs for
	/// real.
	RunUnmatched,
	/// Return an error before running any unmatched task body.
	///
	/// A dry-run task graph's safety net: if a test intends to stub every
	/// task an application flow reaches (so nothing accidentally hits a
	/// real database or network call), `RequireOverride` turns "forgot to
	/// stub one" into an immediate [`Error::MissingOverride`](crate::Error::MissingOverride)
	/// instead of a silent real execution.
	RequireOverride,
}

/// Typed task-body substitutions for tests and dry-run task graphs.
///
/// Built once with [`new`](Self::new) and a [`TaskOverrideMode`], then
/// chained with [`replace`](Self::replace) calls (each one consumes and
/// returns `Self`, so calls compose fluently), and finally supplied to
/// [`TasksOptions::overrides`](crate::TasksOptions::overrides) when
/// constructing a [`Tasks`](crate::Tasks) runtime. Every substitution is
/// checked at replacement time to be that exact task's own input/output
/// types — there is no way to register a substitute with the wrong shape
/// for the task it replaces.
///
/// ```
/// use vorma_tasks::{TaskOverrideMode, TaskOverrides, Tasks, TasksOptions};
///
/// vorma_tasks::task! {
///     static GREETING: Task<String, String, &'static str> =
///         memoized(|_ctx, name: String| async move { Ok(format!("Hello, {name}!")) });
/// }
///
/// # #[tokio::main(flavor = "current_thread")]
/// # async fn main() {
/// let overrides = TaskOverrides::new(TaskOverrideMode::RunUnmatched)
///     .replace(&GREETING, |_ctx, name: String| async move { Ok(format!("Hi {name}")) });
/// let tasks: Tasks<&'static str> = Tasks::new(TasksOptions {
///     overrides: Some(overrides),
///     ..TasksOptions::default()
/// });
/// let ctx = tasks.exec_ctx(vorma_tasks::CancelToken::new());
///
/// let greeting = GREETING.run(&ctx, "Ferris".to_owned()).await.unwrap();
/// assert_eq!(*greeting, "Hi Ferris");
/// # }
/// ```
pub struct TaskOverrides<E = Box<dyn std::error::Error + Send + Sync>> {
	mode: TaskOverrideMode,
	replacements: HashMap<TaskId, Arc<dyn Any + Send + Sync>>,
	_error: PhantomData<fn() -> E>,
}

impl<E> TaskOverrides<E>
where
	E: Send + Sync + 'static,
{
	/// Create a set of typed task-body substitutions.
	pub fn new(mode: TaskOverrideMode) -> Self {
		Self {
			mode,
			replacements: HashMap::new(),
			_error: PhantomData,
		}
	}

	/// Replace one task body with a typed substitute.
	///
	/// The substitute participates in caching exactly like the original
	/// body would: a `memoized` task's substitute result is still memoized
	/// for the execution context, and an `extended_cache` task's
	/// substitute result still populates the cross-execution-context
	/// cache. Calling `replace` again for the same task overwrites the
	/// earlier substitute rather than stacking both.
	pub fn replace<I, O, F, Fut>(mut self, task: &Task<I, O, E>, f: F) -> Self
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
		F: Fn(ExecCtx<E>, I) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, E>> + Send + 'static,
	{
		self.replacements.insert(
			task.id(),
			Arc::new(TaskReplacement {
				f: Box::new(move |ctx, input| Box::pin(f(ctx, input))),
			}),
		);
		self
	}

	pub(crate) fn resolve<I, O>(&self, task: &Task<I, O, E>) -> TaskOverride<I, O, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		if let Some(replacement) = self.replacements.get(&task.id()) {
			let replacement = replacement
				.clone()
				.downcast::<TaskReplacement<I, O, E>>()
				.expect("task override stored with mismatched input/output type");
			return TaskOverride::Replace(replacement);
		}

		match self.mode {
			TaskOverrideMode::RunUnmatched => TaskOverride::RunOriginal,
			TaskOverrideMode::RequireOverride => TaskOverride::Missing,
		}
	}
}

impl<E> Clone for TaskOverrides<E> {
	fn clone(&self) -> Self {
		Self {
			mode: self.mode,
			replacements: self.replacements.clone(),
			_error: PhantomData,
		}
	}
}

pub(crate) enum TaskOverride<I, O, E> {
	RunOriginal,
	Replace(Arc<TaskReplacement<I, O, E>>),
	Missing,
}

pub(crate) struct TaskReplacement<I, O, E> {
	f: Box<OverrideFn<I, O, E>>,
}

impl<I, O, E> TaskReplacement<I, O, E>
where
	I: Send + 'static,
	O: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub(crate) async fn call(&self, ctx: ExecCtx<E>, input: I) -> StoredOutcome<E> {
		match (self.f)(ctx, input).await {
			Ok(output) => StoredOutcome::Ok(Arc::new(output)),
			Err(error) => StoredOutcome::Err(error),
		}
	}
}
