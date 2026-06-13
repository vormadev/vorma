use std::fmt;
use std::future::Future;
use std::hash::Hash;
use std::pin::Pin;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use futures_util::stream::FuturesUnordered;
use futures_util::stream::StreamExt;

use crate::cancel::CancelToken;
use crate::clock::{Clock, ClockInstant, SystemClock};
use crate::error::{Error, Result};
use crate::key::{KeyFingerprint, PathKey, TaskId, fingerprint_for};
use crate::observer::{TaskEvent, TaskEventKind, TaskEventOutcome, TaskObserver, TaskRunSource};
use crate::overrides::{TaskOverride, TaskOverrides};
use crate::store::{
	RunningGuard, Slot, SlotClaim, SlotLookup, Store, StoredOutcome, decode_outcome,
};

static NEXT_TASK_ID: AtomicU64 = AtomicU64::new(1);

// One link in an execution path: the task/input pair being run, plus
// the chain it was reached through. Extending a path for one run costs
// a single allocation; sharing it costs a refcount bump.
struct PathNode {
	key: PathKey,
	parent: Option<Arc<PathNode>>,
}

fn path_contains<I>(mut node: Option<&PathNode>, fingerprint: KeyFingerprint, input: &I) -> bool
where
	I: Eq + Hash + Send + Sync + 'static,
{
	while let Some(current) = node {
		if current.key.matches(fingerprint, input) {
			return true;
		}
		node = current.parent.as_deref();
	}
	false
}

type BoxFuture<T> = Pin<Box<dyn Future<Output = T> + Send + 'static>>;
type TaskFn<I, O, E> = dyn Fn(ExecCtx<E>, I) -> BoxFuture<Result<O, E>> + Send + Sync;
type PreparedTaskFn<E> = dyn FnOnce(ExecCtx<E>) -> BoxFuture<Result<(), E>> + Send;

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
		ctx.resolve(self, input).await
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
		PreparedTask::new(move |ctx| async move {
			let output = task.run(&ctx, input).await?;
			result(output);
			Ok(())
		})
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
	/// Maximum number of entries retained by the cross-execution-context cache.
	///
	/// When this limit is reached, existing entries remain usable and new keys run
	/// without shared caching.
	pub max_cross_exec_ctx_cache_entries: usize,
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
				shared: Store::new(Some(options.max_cross_exec_ctx_cache_entries)),
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
			local: Arc::new(Store::new(None)),
			cancel,
			path: None,
		}
	}

	fn now(&self) -> ClockInstant {
		self.inner.clock.now()
	}

	// Observer presence, checked before any event construction that
	// would itself cost work (clock reads, durations).
	fn has_observer(&self) -> bool {
		self.inner.observer.is_some()
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
			max_cross_exec_ctx_cache_entries: 4096,
		}
	}
}

/// One execution context with task memoization, cancellation, and dependency coalescing.
pub struct ExecCtx<E = Box<dyn std::error::Error + Send + Sync>> {
	tasks: Tasks<E>,
	local: Arc<Store<E>>,
	cancel: CancelToken,
	path: Option<Arc<PathNode>>,
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

		/*
		Siblings run concurrently on one poller rather than as spawned
		runtime tasks: prepared work interleaves at await points, the
		first failure cancels the rest, and no per-sibling scheduler
		round-trip is paid.
		*/
		let shared = self.child();
		let cancel = shared.cancel.clone();
		let mut futures: FuturesUnordered<_> = tasks
			.into_iter()
			.map(|task| {
				let ctx = shared.clone();
				task.run(ctx)
			})
			.collect();

		let mut first_error: Option<Error<E>> = None;
		while let Some(result) = futures.next().await {
			if let Err(error) = result {
				let replace_first_error = first_error
					.as_ref()
					.is_none_or(|first| first.is_cancelled() && !error.is_cancelled());
				if replace_first_error {
					first_error = Some(error);
				}
				cancel.cancel();
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
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: self.cancel.clone(),
			path: Some(Arc::new(PathNode {
				key,
				parent: self.path.clone(),
			})),
		}
	}

	fn shared_run_child(&self, key: PathKey) -> Self {
		Self {
			tasks: self.tasks.clone(),
			local: self.local.clone(),
			cancel: CancelToken::new(),
			path: Some(Arc::new(PathNode {
				key,
				parent: self.path.clone(),
			})),
		}
	}

	async fn resolve<I, O>(&self, task: &Task<I, O, E>, input: I) -> Result<Arc<O>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let fingerprint = fingerprint_for(task.inner.id, &input);
		if path_contains(self.path.as_deref(), fingerprint, &input) {
			return Err(Error::Cycle {
				task: task.inner.name,
			});
		}
		if self.cancel.is_cancelled() {
			self.tasks
				.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
			return Err(Error::Cancelled);
		}

		let local_lookup = self
			.local
			.slot_for(fingerprint, &input, None, || self.tasks.now());
		let local_slot = match local_lookup {
			SlotLookup::Found { slot, .. } => slot,
			SlotLookup::CapacityBypass { .. } => unreachable!("local task store is unbounded"),
		};
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
					task,
					input,
					fingerprint,
					task.inner.cross_exec_ctx_cache_ttl,
				)
				.await
			{
				Ok(outcome) => outcome,
				Err(error) => return Err(error),
			}
		} else {
			let path_key = PathKey::from_dyn(local_slot.shared_key());
			self.run_in_exec_ctx(task, input, path_key).await?
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

	async fn run_in_exec_ctx<I, O>(
		&self,
		task: &Task<I, O, E>,
		input: I,
		path_key: PathKey,
	) -> Result<StoredOutcome<E>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let child_ctx = self.child_for_task(path_key);
		let observing = self.tasks.has_observer();
		let started_at = observing.then(|| self.tasks.now());
		if observing {
			self.tasks.observe(
				task.inner.id,
				task.inner.name,
				TaskEventKind::RunStarted {
					source: TaskRunSource::ExecCtx,
				},
			);
		}

		/*
		Fast path: poll the task body once before wiring up any
		cancellation machinery. Bodies that complete immediately —
		memoized dependencies, pure computation — never pay for a
		cancellation subscription. Cancellation was checked just before
		this, and a body that never suspends has no await point at
		which prompt cancellation could matter.
		*/
		let call = task.call(child_ctx, input);
		let mut call = std::pin::pin!(call);
		let first_poll = {
			let mut poll_ctx = std::task::Context::from_waker(std::task::Waker::noop());
			Future::poll(call.as_mut(), &mut poll_ctx)
		};
		let outcome = match first_poll {
			std::task::Poll::Ready(outcome) => outcome,
			std::task::Poll::Pending => {
				tokio::select! {
					result = call.as_mut() => result,
					_ = self.cancel.cancelled() => {
						self.tasks.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
						return Err(Error::Cancelled);
					},
				}
			}
		};

		if observing && let Some(started_at) = started_at {
			self.tasks.observe(
				task.inner.id,
				task.inner.name,
				TaskEventKind::RunCompleted {
					source: TaskRunSource::ExecCtx,
					outcome: task_event_outcome(&outcome),
					duration: self.tasks.now().saturating_duration_since(started_at),
				},
			);
		}
		if self.cancel.is_cancelled() {
			self.tasks
				.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
			return Err(Error::Cancelled);
		}
		Ok(outcome)
	}

	/*
	Lost-wakeup safety: a waiter must register with the notifier BEFORE
	re-checking slot state. notify_waiters stores no permit, so checking
	first would leave a window — runner finishes and notifies between
	the check and the await — that strands the waiter forever.
	*/
	async fn wait_for_local<O>(
		&self,
		slot: &Arc<Slot<E>>,
		task_id: TaskId,
		task_name: &'static str,
	) -> Result<Arc<O>, E>
	where
		O: Send + Sync + 'static,
	{
		slot.register_waiter();
		let guard = WaiterGuard { slot };
		loop {
			if self.cancel.is_cancelled() {
				self.tasks
					.observe(task_id, task_name, TaskEventKind::Cancelled);
				return Err(Error::Cancelled);
			}
			let notified = slot.signal.notified();
			tokio::pin!(notified);
			notified.as_mut().enable();
			match slot.claim() {
				SlotClaim::Ready(outcome) => {
					if self.cancel.is_cancelled() {
						self.tasks
							.observe(task_id, task_name, TaskEventKind::Cancelled);
						return Err(Error::Cancelled);
					}
					drop(guard);
					return decode_outcome::<O, E>(outcome, task_name);
				}
				SlotClaim::Run => {
					slot.abandon();
					return Err(Error::Cancelled);
				}
				SlotClaim::Wait => {
					tokio::select! {
						_ = &mut notified => {}
						_ = self.cancel.cancelled() => {
							self.tasks.observe(task_id, task_name, TaskEventKind::Cancelled);
							return Err(Error::Cancelled);
						},
					}
				}
			}
		}
	}

	async fn resolve_shared<I, O>(
		&self,
		task: &Task<I, O, E>,
		input: I,
		fingerprint: KeyFingerprint,
		ttl: std::time::Duration,
	) -> Result<StoredOutcome<E>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let shared_lookup =
			self.tasks
				.inner
				.shared
				.slot_for(fingerprint, &input, Some(ttl), || self.tasks.now());
		let shared_slot = match shared_lookup {
			SlotLookup::Found {
				slot,
				stale_slots_removed,
			} => {
				if stale_slots_removed > 0 {
					self.tasks.observe(
						task.inner.id,
						task.inner.name,
						TaskEventKind::CrossExecCtxStaleSlotRemoved {
							count: stale_slots_removed,
						},
					);
				}
				slot
			}
			SlotLookup::CapacityBypass {
				stale_slots_removed,
				max_entries,
			} => {
				if stale_slots_removed > 0 {
					self.tasks.observe(
						task.inner.id,
						task.inner.name,
						TaskEventKind::CrossExecCtxStaleSlotRemoved {
							count: stale_slots_removed,
						},
					);
				}
				self.tasks.observe(
					task.inner.id,
					task.inner.name,
					TaskEventKind::CrossExecCtxCacheCapacityBypass { max_entries },
				);
				let path_key = PathKey::new(task.inner.id, &input);
				return self.run_in_exec_ctx(task, input, path_key).await;
			}
		};

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
				let run_ctx = self.shared_run_child(PathKey::from_dyn(shared_slot.shared_key()));
				let tasks = self.tasks.clone();
				let task_id = task.inner.id;
				let task_name = task.inner.name;
				let shared_slot_for_run = shared_slot.clone();
				tokio::spawn(async move {
					let observing = tasks.has_observer();
					let started_at = observing.then(|| tasks.now());
					if observing {
						tasks.observe(
							task_id,
							task_name,
							TaskEventKind::RunStarted {
								source: TaskRunSource::CrossExecCtx,
							},
						);
					}
					let outcome = task_for_spawn.call(run_ctx, input_for_spawn).await;
					if observing && let Some(started_at) = started_at {
						tasks.observe(
							task_id,
							task_name,
							TaskEventKind::RunCompleted {
								source: TaskRunSource::CrossExecCtx,
								outcome: task_event_outcome(&outcome),
								duration: tasks.now().saturating_duration_since(started_at),
							},
						);
					}
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

		shared_slot.register_waiter();
		let guard = WaiterGuard { slot: &shared_slot };
		loop {
			if self.cancel.is_cancelled() {
				self.tasks
					.observe(task.inner.id, task.inner.name, TaskEventKind::Cancelled);
				return Err(Error::Cancelled);
			}
			let notified = shared_slot.signal.notified();
			tokio::pin!(notified);
			notified.as_mut().enable();
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
					drop(guard);
					return Ok(outcome);
				}
				SlotClaim::Run => {
					shared_slot.abandon();
					return Err(Error::Cancelled);
				}
				SlotClaim::Wait => {
					tokio::select! {
						_ = &mut notified => {}
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

// Balances waiter registration on every exit path, including
// cancellation returns.
struct WaiterGuard<'s, E> {
	slot: &'s Slot<E>,
}

impl<E> Drop for WaiterGuard<'_, E> {
	fn drop(&mut self) {
		self.slot.unregister_waiter();
	}
}

fn task_event_outcome<E>(outcome: &StoredOutcome<E>) -> TaskEventOutcome {
	match outcome {
		StoredOutcome::Ok(_) => TaskEventOutcome::Success,
		StoredOutcome::Err(error) if error.is_cancelled() => TaskEventOutcome::Cancelled,
		StoredOutcome::Err(_) => TaskEventOutcome::Error,
	}
}
