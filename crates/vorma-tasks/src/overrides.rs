use std::any::Any;
use std::collections::HashMap;
use std::future::Future;
use std::hash::Hash;
use std::marker::PhantomData;
use std::pin::Pin;
use std::sync::Arc;

use crate::error::Result;
use crate::key::TaskId;
use crate::store::StoredOutcome;
use crate::task::{ExecCtx, Task};

type BoxFuture<T> = Pin<Box<dyn Future<Output = T> + Send + 'static>>;
type OverrideFn<I, O, E> = dyn Fn(ExecCtx<E>, I) -> BoxFuture<Result<O, E>> + Send + Sync;

/// Policy for tasks that do not have an explicit test override.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskOverrideMode {
	/// Run the original task body when no override is registered.
	RunUnmatched,
	/// Return an error before running any unmatched task body.
	RequireOverride,
}

/// Typed task-body substitutions for tests and dry-run task graphs.
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
