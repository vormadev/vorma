use std::future::Future;
use std::hash::Hash;
use std::panic;
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use tokio::task::{JoinError, JoinSet};

use crate::cancel::CancelToken;
use crate::error::{Error, Result};
use crate::task::{ExecCtx, Task};

type PreparedParallelFuture<E> = Pin<Box<dyn Future<Output = Result<(), E>> + Send + 'static>>;
type PreparedParallelRun<E> =
	Box<dyn FnOnce(ExecCtx<E>, CancelToken) -> PreparedParallelFuture<E> + Send>;

/// Collection of task/input pairs that run in parallel.
pub struct ParallelBatch<E = Box<dyn std::error::Error + Send + Sync>> {
	id: Arc<()>,
	tasks: Vec<PreparedParallelTask<E>>,
}

impl<E> ParallelBatch<E>
where
	E: Send + Sync + 'static,
{
	/// Create an empty parallel batch.
	pub fn new() -> Self {
		Self {
			id: Arc::new(()),
			tasks: Vec::new(),
		}
	}

	/// Add one task/input pair to this batch.
	pub fn add<I, O>(&mut self, task: Task<I, O, E>, input: I) -> ParallelBatchOutputHandle<O>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let output = ParallelBatchOutputHandle {
			batch_id: self.id.clone(),
			slot: Arc::new(Mutex::new(None)),
		};
		let output_for_run = output.slot.clone();
		self.tasks.push(PreparedParallelTask {
			run: Box::new(move |ctx, cancel| {
				Box::pin(async move {
					let result = task.run(&ctx, input).await;
					if result.is_err() {
						cancel.cancel();
					}
					let value = result?;
					*output_for_run
						.lock()
						.expect("parallel task output lock poisoned") = Some(value);
					Ok(())
				})
			}),
		});
		output
	}

	/// Run every task in this batch in parallel.
	pub async fn run(self, ctx: &ExecCtx<E>) -> Result<ParallelBatchOutputs, E> {
		if ctx.is_cancelled() {
			return Err(Error::Cancelled);
		}

		let outputs = ParallelBatchOutputs { batch_id: self.id };
		let mut tasks = self.tasks;
		match tasks.len() {
			0 => return Ok(outputs),
			1 => {
				let task = tasks.pop().expect("parallel task count checked");
				let run_ctx = ctx.child();
				let cancel = run_ctx.cancel_token().clone();
				(task.run)(run_ctx, cancel).await?;
				if ctx.is_cancelled() {
					return Err(Error::Cancelled);
				}
				return Ok(outputs);
			}
			_ => {}
		}

		let run_ctx = ctx.child();
		let cancel = run_ctx.cancel_token().clone();
		let mut cancel_on_drop = CancelOnDrop::new(cancel.clone());
		let mut set = JoinSet::new();

		for task in tasks {
			let task_ctx = run_ctx.clone();
			let task_cancel = cancel.clone();
			set.spawn((task.run)(task_ctx, task_cancel));
		}

		let mut first_error = None;
		while let Some(joined) = set.join_next().await {
			match joined_result(joined) {
				Ok(()) => {}
				Err(error) => remember_first_error(&mut first_error, &cancel, error),
			}
		}

		cancel_on_drop.disarm();
		if let Some(error) = first_error {
			return Err(error);
		}
		if ctx.is_cancelled() {
			return Err(Error::Cancelled);
		}

		Ok(outputs)
	}
}

impl<E> Default for ParallelBatch<E>
where
	E: Send + Sync + 'static,
{
	fn default() -> Self {
		Self::new()
	}
}

/// Typed handle for one output produced by a [`ParallelBatch`].
pub struct ParallelBatchOutputHandle<O: 'static> {
	batch_id: Arc<()>,
	slot: Arc<Mutex<Option<Arc<O>>>>,
}

/// Output values produced by a successfully completed [`ParallelBatch`].
#[derive(Debug)]
pub struct ParallelBatchOutputs {
	batch_id: Arc<()>,
}

impl ParallelBatchOutputs {
	/// Take one output by consuming the handle returned by [`ParallelBatch::add`].
	///
	/// # Panics
	///
	/// Panics if the handle came from a different batch.
	/// Panics if the batch reports success without storing the corresponding output.
	pub fn take<O>(&self, handle: ParallelBatchOutputHandle<O>) -> Arc<O>
	where
		O: Send + Sync + 'static,
	{
		assert!(
			Arc::ptr_eq(&self.batch_id, &handle.batch_id),
			"parallel batch output handle used with a different batch"
		);
		handle
			.slot
			.lock()
			.expect("parallel task output lock poisoned")
			.take()
			.expect("parallel batch output already taken")
	}
}

struct PreparedParallelTask<E> {
	run: PreparedParallelRun<E>,
}

fn joined_result<T, E>(joined: std::result::Result<Result<T, E>, JoinError>) -> Result<T, E>
where
	E: Send + Sync + 'static,
{
	match joined {
		Ok(result) => result,
		Err(error) if error.is_panic() => panic::resume_unwind(error.into_panic()),
		Err(_) => Err(Error::Cancelled),
	}
}

fn remember_first_error<E>(
	first_error: &mut Option<Error<E>>,
	cancel: &CancelToken,
	error: Error<E>,
) {
	let replace_first_error = first_error
		.as_ref()
		.is_none_or(|first| first.is_cancelled() && !error.is_cancelled());
	if replace_first_error {
		*first_error = Some(error);
	}
	cancel.cancel();
}

struct CancelOnDrop {
	cancel: CancelToken,
	active: bool,
}

impl CancelOnDrop {
	fn new(cancel: CancelToken) -> Self {
		Self {
			cancel,
			active: true,
		}
	}

	fn disarm(&mut self) {
		self.active = false;
	}
}

impl Drop for CancelOnDrop {
	fn drop(&mut self) {
		if self.active {
			self.cancel.cancel();
		}
	}
}
