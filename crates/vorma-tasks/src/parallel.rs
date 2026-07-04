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
///
/// This is the crate's one collection type for running independent work
/// concurrently — there is deliberately no "spawn one task and await it
/// separately" idiom to reach for instead. Build a batch with
/// [`new`](Self::new), register each task/input pair with
/// [`add`](Self::add) (keeping the typed [`ParallelBatchOutputHandle`] it
/// returns), then consume the batch with [`run`](Self::run) and read each
/// result back out of the returned [`ParallelBatchOutputs`] with
/// [`take`](ParallelBatchOutputs::take).
///
/// **Parallel here means real spawned parallelism**, not merely concurrent
/// polling on one task: two or more siblings each become their own
/// `tokio::spawn`ed task (through an internal `JoinSet`), so CPU-bound
/// sibling work actually scales across executor threads — the same
/// contract Go's goroutine fan-out gives, which this crate is designed to
/// match rather than merely approximate with single-threaded interleaving.
/// That means a batch pays a real per-sibling task-spawn cost; for
/// sub-microsecond bodies with no I/O and no independent blocking, plain
/// `tokio::join!` of the task `run` calls directly may outperform a batch of
/// that size — reach for `ParallelBatch` when the siblings do enough
/// independent work (I/O, a task body with real CPU cost, or you want them
/// to share this batch's coalesced cancellation) that the spawn cost is
/// worth paying. A batch of exactly one task skips spawning entirely and
/// runs inline, so adding tasks to a batch one at a time in code that
/// sometimes ends up with only one is never a tax.
///
/// Every task in one batch shares the [`ExecCtx`] passed to
/// [`run`](Self::run) (through a [`child`](ExecCtx::child) context), so
/// siblings that happen to depend on the same underlying task/input pair
/// still coalesce through ordinary execution-context memoization. If any
/// sibling fails, [`run`](Self::run) cancels every other sibling still in
/// flight and returns that first failure — a batch is all-or-nothing, never
/// a partial-results collection.
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
	///
	/// Returns a typed handle: not the output itself (the batch has not run
	/// yet), but the key [`ParallelBatchOutputs::take`] uses after
	/// [`run`](Self::run) completes to hand back this specific task's
	/// `Arc<O>` result with its real type intact.
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
	///
	/// An empty batch (nothing was ever [`add`](Self::add)ed) succeeds
	/// immediately with an empty [`ParallelBatchOutputs`]. A batch of
	/// exactly one task runs it inline against a child of `ctx`, with no
	/// spawn. Two or more tasks each become their own spawned task racing
	/// to completion together.
	///
	/// If `ctx` is already cancelled, or becomes cancelled while siblings
	/// are running, every sibling still in flight is cancelled and this
	/// returns [`Error::Cancelled`](crate::Error::Cancelled). If a sibling
	/// task fails with an application error, every other sibling is
	/// cancelled immediately, and the returned error is the first
	/// non-cancellation failure observed, in completion order (not
	/// registration order) — a cancellation any other sibling reports as
	/// its own outcome is superseded by any application error, but among
	/// multiple application-error failures, whichever completed first
	/// wins. A panic inside any sibling's task body propagates out of
	/// `run` instead of being converted to an [`Error`](crate::Error).
	///
	/// ```
	/// use vorma_tasks::{ParallelBatch, Tasks, TasksOptions};
	///
	/// vorma_tasks::task! {
	///     static DOUBLE: Task<u32, u32, &'static str> =
	///         memoized(|_ctx, input: u32| async move { Ok(input * 2) });
	/// }
	/// vorma_tasks::task! {
	///     static SQUARE: Task<u32, u32, &'static str> =
	///         memoized(|_ctx, input: u32| async move { Ok(input * input) });
	/// }
	///
	/// # #[tokio::main(flavor = "current_thread")]
	/// # async fn main() {
	/// let tasks: Tasks<&'static str> = Tasks::new(TasksOptions::default());
	/// let ctx = tasks.exec_ctx(vorma_tasks::CancelToken::new());
	///
	/// let mut batch = ParallelBatch::new();
	/// let doubled = batch.add(DOUBLE, 21);
	/// let squared = batch.add(SQUARE, 5);
	/// let outputs = batch.run(&ctx).await.unwrap();
	///
	/// assert_eq!(*outputs.take(doubled), 42);
	/// assert_eq!(*outputs.take(squared), 25);
	/// # }
	/// ```
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
///
/// Returned by [`ParallelBatch::add`]; carries no value itself until the
/// batch has run — it is the key [`ParallelBatchOutputs::take`] exchanges
/// for the actual `Arc<O>` result once [`ParallelBatch::run`] succeeds.
pub struct ParallelBatchOutputHandle<O: 'static> {
	batch_id: Arc<()>,
	slot: Arc<Mutex<Option<Arc<O>>>>,
}

/// Output values produced by a successfully completed [`ParallelBatch`].
///
/// Returned by [`ParallelBatch::run`] on success; read individual outputs
/// back out with [`take`](Self::take), passing each
/// [`ParallelBatchOutputHandle`] the corresponding [`ParallelBatch::add`]
/// call returned.
#[derive(Debug)]
pub struct ParallelBatchOutputs {
	batch_id: Arc<()>,
}

impl ParallelBatchOutputs {
	/// Take one output by consuming the handle returned by [`ParallelBatch::add`].
	///
	/// Each handle can be taken exactly once — `take` consumes it by value,
	/// so there is no way to call it twice on the same handle from safe
	/// code.
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
