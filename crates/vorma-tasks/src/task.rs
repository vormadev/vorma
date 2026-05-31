use std::fmt;
use std::future::Future;
use std::hash::Hash;
use std::pin::Pin;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use crate::cancel::CancelToken;
use crate::clock::{Clock, ClockInstant, SystemClock};
use crate::error::{Error, Result};
use crate::key::{KeyData, PathKey, TaskId};
use crate::observer::{TaskEvent, TaskEventKind, TaskEventOutcome, TaskObserver, TaskRunSource};
use crate::overrides::{TaskOverride, TaskOverrides};
use crate::store::{RunningGuard, Slot, SlotClaim, Store, StoredOutcome, decode_outcome};

static NEXT_TASK_ID: AtomicU64 = AtomicU64::new(1);

type BoxFuture<T> = Pin<Box<dyn Future<Output = T> + Send + 'static>>;
type TaskFn<I, O, E> = dyn Fn(ExecCtx<E>, I) -> BoxFuture<Result<O, E>> + Send + Sync;
type PreparedTaskFn<E> = dyn FnOnce(ExecCtx<E>) -> BoxFuture<Result<(), E>> + Send;
type PreparedTaskResultFn<O> = dyn FnOnce(Arc<O>) + Send;

/// Stable typed unit of async work.
pub struct Task<I, O, E = Box<dyn std::error::Error + Send + Sync>> {
	inner: Arc<TaskInner<I, O, E>>,
}

impl<I, O, E> Task<I, O, E>
where
	I: Clone + Eq + Hash + Send + Sync + 'static,
	O: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create a task.
	///
	/// `Duration::ZERO` disables cross-execution-context caching. A nonzero TTL retains
	/// successful results across execution contexts created from the same [`Tasks`]
	/// runtime.
	pub fn new<F, Fut>(cross_exec_ctx_cache_ttl: Duration, f: F) -> Self
	where
		F: Fn(ExecCtx<E>, I) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, E>> + Send + 'static,
	{
		Self {
			inner: Arc::new(TaskInner {
				id: TaskId(NEXT_TASK_ID.fetch_add(1, Ordering::Relaxed)),
				cross_exec_ctx_cache_ttl,
				name: std::any::type_name::<I>(),
				f: Box::new(move |ctx, input| Box::pin(f(ctx, input))),
			}),
		}
	}

	/// Opaque identity for this task definition inside the current process.
	pub fn id(&self) -> TaskId {
		self.inner.id
	}

	/// Run this task in an execution context.
	pub async fn run(&self, ctx: &ExecCtx<E>, input: I) -> Result<Arc<O>, E> {
		ctx.resolve(self.clone(), input).await
	}

	/// Bind input for later execution through [`ExecCtx::run_parallel`].
	pub fn bind_input(&self, input: I) -> PreparedTask<E> {
		self.bind_input_with_result(input, |_| {})
	}

	/// Bind input and a result sink for later execution through [`ExecCtx::run_parallel`].
	pub fn bind_input_with_result<F>(&self, input: I, result: F) -> PreparedTask<E>
	where
		F: FnOnce(Arc<O>) + Send + 'static,
	{
		let task = self.clone();
		let mut result = Some(Box::new(result) as Box<PreparedTaskResultFn<O>>);
		PreparedTask::new(move |ctx| async move {
			let output = task.run(&ctx, input).await?;
			if let Some(result) = result.take() {
				result(output);
			}
			Ok(())
		})
	}

	fn key(&self, input: &I) -> KeyData<I> {
		KeyData::new(self.inner.id, input)
	}

	fn child_ctx(&self, ctx: &ExecCtx<E>, input: &I) -> ExecCtx<E> {
		ctx.child_for_task(PathKey::new(self.inner.id, input))
	}

	fn shared_run_child(&self, ctx: &ExecCtx<E>, input: &I) -> ExecCtx<E> {
		ctx.shared_run_child(PathKey::new(self.inner.id, input))
	}

	async fn call(&self, ctx: ExecCtx<E>, input: I) -> StoredOutcome<E> {
		let task_override = ctx.tasks.task_override_for(self);
		match task_override {
			TaskOverride::RunOriginal => self.call_original(ctx, input).await,
			TaskOverride::Replace(replacement) => replacement.call(ctx, input).await,
			TaskOverride::Missing => StoredOutcome::Err(Error::MissingOverride {
				task: self.inner.name,
			}),
		}
	}

	async fn call_original(&self, ctx: ExecCtx<E>, input: I) -> StoredOutcome<E> {
		match (self.inner.f)(ctx, input).await {
			Ok(output) => StoredOutcome::Ok(Arc::new(output)),
			Err(error) => StoredOutcome::Err(error),
		}
	}
}

/// A task with input bound, ready for [`ExecCtx::run_parallel`].
pub struct PreparedTask<E = Box<dyn std::error::Error + Send + Sync>> {
	run: Box<PreparedTaskFn<E>>,
}

impl<E> PreparedTask<E>
where
	E: Send + Sync + 'static,
{
	fn new<F, Fut>(run: F) -> Self
	where
		F: FnOnce(ExecCtx<E>) -> Fut + Send + 'static,
		Fut: Future<Output = Result<(), E>> + Send + 'static,
	{
		Self {
			run: Box::new(move |ctx| Box::pin(run(ctx))),
		}
	}

	async fn run(self, ctx: ExecCtx<E>) -> Result<(), E> {
		(self.run)(ctx).await
	}
}

impl<I, O, E> Clone for Task<I, O, E> {
	fn clone(&self) -> Self {
		Self {
			inner: self.inner.clone(),
		}
	}
}

struct TaskInner<I, O, E> {
	id: TaskId,
	cross_exec_ctx_cache_ttl: Duration,
	name: &'static str,
	f: Box<TaskFn<I, O, E>>,
}

/// Owns shared cross-execution-context task state.
pub struct Tasks<E = Box<dyn std::error::Error + Send + Sync>> {
	inner: Arc<TasksInner<E>>,
}

/// Configuration for a [`Tasks`] runtime.
pub struct TasksOptions<E = Box<dyn std::error::Error + Send + Sync>> {
	/// Monotonic clock used for cross-execution-context cache expiry and event timing.
	pub clock: Arc<dyn Clock>,
	/// Passive observer for task runtime events.
	pub observer: Option<Arc<dyn TaskObserver>>,
	/// Typed task-body substitutions for test and dry-run runtimes.
	pub overrides: Option<TaskOverrides<E>>,
}

impl<E> Tasks<E>
where
	E: Send + Sync + 'static,
{
	/// Create a long-lived task runtime with explicit options.
	pub fn new(options: TasksOptions<E>) -> Self {
		Self {
			inner: Arc::new(TasksInner {
				clock: options.clock,
				observer: options.observer,
				shared: Store::new(),
				overrides: options.overrides,
			}),
		}
	}

	fn task_override_for<I, O>(&self, task: &Task<I, O, E>) -> TaskOverride<I, O, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		if let Some(overrides) = &self.inner.overrides {
			overrides.resolve(task)
		} else {
			TaskOverride::RunOriginal
		}
	}

	/// Create one execution context with its own memoization state.
	pub fn exec_ctx(&self, cancel: CancelToken) -> ExecCtx<E> {
		ExecCtx {
			tasks: self.clone(),
			local: Arc::new(Store::new()),
			cancel,
			path: Arc::new(Vec::new()),
		}
	}

	fn now(&self) -> ClockInstant {
		self.inner.clock.now()
	}

	fn observe(&self, task_id: TaskId, task_input_type: &'static str, kind: TaskEventKind) {
		if let Some(observer) = &self.inner.observer {
			observer.observe(TaskEvent {
				at: self.now(),
				task_id,
				task_input_type,
				kind,
			});
		}
	}
}

impl<E> Clone for Tasks<E> {
	fn clone(&self) -> Self {
		Self {
			inner: self.inner.clone(),
		}
	}
}

impl<E> fmt::Debug for Tasks<E> {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_struct("Tasks").finish_non_exhaustive()
	}
}

struct TasksInner<E> {
	clock: Arc<dyn Clock>,
	observer: Option<Arc<dyn TaskObserver>>,
	shared: Store<E>,
	overrides: Option<TaskOverrides<E>>,
}

impl<E> Default for TasksOptions<E>
where
	E: 'static,
{
	fn default() -> Self {
		Self {
			clock: Arc::new(SystemClock::new()),
			observer: None,
			overrides: None,
		}
	}
}

/// One execution context with task memoization, cancellation, and dependency coalescing.
pub struct ExecCtx<E = Box<dyn std::error::Error + Send + Sync>> {
	tasks: Tasks<E>,
	local: Arc<Store<E>>,
	cancel: CancelToken,
	path: Arc<Vec<PathKey>>,
}

impl<E> ExecCtx<E>
where
	E: Send + Sync + 'static,
{
	/// Cancellation token associated with this execution context.
	pub fn cancel_token(&self) -> &CancelToken {
		&self.cancel
	}

	/// Whether this execution context has been cancelled.
	pub fn is_cancelled(&self) -> bool {
		self.cancel.is_cancelled()
	}

	/// Create a child execution context with shared memoization and child cancellation.
	pub fn child(&self) -> Self {
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: self.cancel.child(),
			path: self.path.clone(),
		}
	}

	/// Run prepared tasks concurrently in a shared child execution context.
	///
	/// All prepared tasks share one execution context for memoization, and the first
	/// failing sibling cancels the remaining siblings.
	pub async fn run_parallel(
		&self,
		tasks: impl IntoIterator<Item = PreparedTask<E>>,
	) -> Result<(), E> {
		if self.cancel.is_cancelled() {
			return Err(Error::Cancelled);
		}

		let tasks = tasks.into_iter().collect::<Vec<_>>();
		match tasks.len() {
			0 => return Ok(()),
			1 => {
				let mut tasks = tasks;
				let task = tasks.pop().expect("length checked");
				return task.run(self.child()).await;
			}
			_ => {}
		}

		let shared = self.child();
		let cancel = shared.cancel.clone();
		let mut joined = tokio::task::JoinSet::new();
		for task in tasks {
			let ctx = shared.clone();
			joined.spawn(async move { task.run(ctx).await });
		}

		let mut first_error = None;
		while let Some(result) = joined.join_next().await {
			match result {
				Ok(Ok(())) => {}
				Ok(Err(error)) => {
					let replace_first_error =
						first_error.as_ref().is_none_or(|first: &Error<E>| {
							first.is_cancelled() && !error.is_cancelled()
						});
					if replace_first_error {
						first_error = Some(error);
					}
					cancel.cancel();
				}
				Err(join_error) if join_error.is_panic() => {
					std::panic::resume_unwind(join_error.into_panic());
				}
				Err(_) => {
					if first_error.is_none() {
						first_error = Some(Error::Cancelled);
					}
					cancel.cancel();
				}
			}
		}

		if let Some(error) = first_error {
			return Err(error);
		}
		if self.cancel.is_cancelled() {
			return Err(Error::Cancelled);
		}
		Ok(())
	}

	fn child_for_task(&self, key: PathKey) -> Self {
		let mut path = Vec::with_capacity(self.path.len() + 1);
		path.extend(self.path.iter().cloned());
		path.push(key);
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: self.cancel.clone(),
			path: Arc::new(path),
		}
	}

	fn shared_run_child(&self, key: PathKey) -> Self {
		let mut path = Vec::with_capacity(self.path.len() + 1);
		path.extend(self.path.iter().cloned());
		path.push(key);
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: CancelToken::new(),
			path: Arc::new(path),
		}
	}

	async fn resolve<I, O>(&self, task: Task<I, O, E>, input: I) -> Result<Arc<O>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let key = task.key(&input);
		if self.path.iter().any(|path_key| path_key.matches(&key)) {
			return Err(Error::Cycle {
				task: task.inner.name,
			});
		}
		if self.cancel.is_cancelled() {
			self.tasks
				.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
			return Err(Error::Cancelled);
		}

		let local_slot = self.local.slot_for(&key, None, self.tasks.now()).slot;
		let local_guard = match local_slot.claim() {
			SlotClaim::Ready(outcome) => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::ExecCtxMemoHit,
				);
				if self.cancel.is_cancelled() {
					self.tasks
						.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
					return Err(Error::Cancelled);
				}
				return decode_outcome::<O, E>(outcome, task.inner.name);
			}
			SlotClaim::Wait => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::ExecCtxMemoWait,
				);
				return self
					.wait_for_local::<O>(&local_slot, task.inner.id, task.inner.name)
					.await;
			}
			SlotClaim::Run => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::ExecCtxMemoMiss,
				);
				RunningGuard::new(local_slot.clone())
			}
		};

		let outcome = if !task.inner.cross_exec_ctx_cache_ttl.is_zero() {
			match self
				.resolve_shared(
					task.clone(),
					input,
					&key,
					task.inner.cross_exec_ctx_cache_ttl,
				)
				.await
			{
				Ok(outcome) => outcome,
				Err(error) => return Err(error),
			}
		} else {
			let child_ctx = task.child_ctx(self, &input);
			let started_at = self.tasks.now();
			self.tasks.observe(
				task.inner.id,
				task.inner.name,
				TaskEventKind::RunStarted {
					source: TaskRunSource::ExecCtx,
				},
			);
			let outcome = tokio::select! {
				result = task.call(child_ctx, input) => result,
				_ = self.cancel.cancelled() => {
					self.tasks.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
					return Err(Error::Cancelled);
				},
			};
			self.tasks.observe(
				task.inner.id,
				task.inner.name,
				TaskEventKind::RunCompleted {
					source: TaskRunSource::ExecCtx,
					outcome: task_event_outcome(&outcome),
					duration: self.tasks.now().saturating_duration_since(started_at),
				},
			);
			if self.cancel.is_cancelled() {
				self.tasks
					.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
				return Err(Error::Cancelled);
			}
			outcome
		};

		if outcome.is_cancelled() {
			local_slot.abandon();
			local_guard.disarm();
			return decode_outcome::<O, E>(outcome, task.inner.name);
		}

		local_slot.finish(outcome.clone(), None);
		local_guard.disarm();
		decode_outcome::<O, E>(outcome, task.inner.name)
	}

	async fn wait_for_local<O>(
		&self,
		slot: &Arc<Slot<E>>,
		task_id: TaskId,
		task_name: &'static str,
	) -> Result<Arc<O>, E>
	where
		O: Send + Sync + 'static,
	{
		loop {
			tokio::select! {
				_ = slot.notify.notified() => {}
				_ = self.cancel.cancelled() => {
					self.tasks.observe(task_id, task_name, TaskEventKind::Cancelled);
					return Err(Error::Cancelled);
				},
			}
			match slot.claim() {
				SlotClaim::Ready(outcome) => {
					if self.cancel.is_cancelled() {
						self.tasks
							.observe(task_id, task_name, TaskEventKind::Cancelled);
						return Err(Error::Cancelled);
					}
					return decode_outcome::<O, E>(outcome, task_name);
				}
				SlotClaim::Wait => {}
				SlotClaim::Run => {
					slot.abandon();
					return Err(Error::Cancelled);
				}
			}
		}
	}

	async fn resolve_shared<I, O>(
		&self,
		task: Task<I, O, E>,
		input: I,
		key: &KeyData<I>,
		ttl: std::time::Duration,
	) -> Result<StoredOutcome<E>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let shared_lookup = self
			.tasks
			.inner
			.shared
			.slot_for(key, Some(ttl), self.tasks.now());
		let shared_slot = shared_lookup.slot;
		if shared_lookup.stale_slots_removed > 0 {
			self.tasks.observe(
				task.inner.id,
				task.inner.name,
				TaskEventKind::CrossExecCtxStaleSlotRemoved {
					count: shared_lookup.stale_slots_removed,
				},
			);
		}

		match shared_slot.claim() {
			SlotClaim::Ready(outcome) => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::CrossExecCtxCacheHit,
				);
				if self.cancel.is_cancelled() {
					self.tasks
						.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
					return Err(Error::Cancelled);
				}
				return Ok(outcome);
			}
			SlotClaim::Wait => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::CrossExecCtxInFlightWait,
				);
			}
			SlotClaim::Run => {
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::CrossExecCtxCacheMiss,
				);
				let guard = RunningGuard::new(shared_slot.clone());
				let task_for_spawn = task.clone();
				let input_for_spawn = input.clone();
				let run_ctx = task.shared_run_child(self, &input_for_spawn);
				let tasks = self.tasks.clone();
				let task_id = task.inner.id;
				let task_name = task.inner.name;
				let shared_slot_for_run = shared_slot.clone();
				tokio::spawn(async move {
					let started_at = tasks.now();
					tasks.observe(
						task_id,
						task_name,
						TaskEventKind::RunStarted {
							source: TaskRunSource::CrossExecCtx,
						},
					);
					let outcome = task_for_spawn.call(run_ctx, input_for_spawn).await;
					tasks.observe(
						task_id,
						task_name,
						TaskEventKind::RunCompleted {
							source: TaskRunSource::CrossExecCtx,
							outcome: task_event_outcome(&outcome),
							duration: tasks.now().saturating_duration_since(started_at),
						},
					);
					if outcome.is_cancelled() {
						shared_slot_for_run.abandon();
						guard.disarm();
						return;
					}
					let expires_at = if outcome.is_ok() {
						Some(tasks.now().saturating_add_duration(ttl))
					} else {
						Some(tasks.now())
					};
					if outcome.is_ok() {
						tasks.observe(task_id, task_name, TaskEventKind::CrossExecCtxCacheInserted);
					}
					shared_slot_for_run.finish(outcome, expires_at);
					guard.disarm();
				});
			}
		}

		loop {
			if self.cancel.is_cancelled() {
				self.tasks
					.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
				return Err(Error::Cancelled);
			}
			match shared_slot.claim() {
				SlotClaim::Ready(outcome) => {
					if self.cancel.is_cancelled() {
						self.tasks.observe(
							task.inner.id,
							task.inner.name,
							TaskEventKind::Cancelled,
						);
						return Err(Error::Cancelled);
					}
					return Ok(outcome);
				}
				SlotClaim::Run => {
					shared_slot.abandon();
					return Err(Error::Cancelled);
				}
				SlotClaim::Wait => {
					tokio::select! {
						_ = shared_slot.notify.notified() => {}
						_ = self.cancel.cancelled() => {
							self.tasks.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
							return Err(Error::Cancelled);
						},
					}
				}
			}
		}
	}
}

impl<E> Clone for ExecCtx<E> {
	fn clone(&self) -> Self {
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: self.cancel.clone(),
			path: self.path.clone(),
		}
	}
}

impl<E> fmt::Debug for ExecCtx<E> {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_struct("ExecCtx")
			.field("cancelled", &self.cancel.is_cancelled())
			.finish_non_exhaustive()
	}
}

fn task_event_outcome<E>(outcome: &StoredOutcome<E>) -> TaskEventOutcome {
	match outcome {
		StoredOutcome::Ok(_) => TaskEventOutcome::Success,
		StoredOutcome::Err(error) if error.is_cancelled() => TaskEventOutcome::Cancelled,
		StoredOutcome::Err(_) => TaskEventOutcome::Error,
	}
}
