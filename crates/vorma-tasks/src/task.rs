use std::fmt;
use std::future::Future;
use std::hash::Hash;
use std::panic;
use std::pin::Pin;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::task::{Context, Poll, Waker};
use std::time::Duration;

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
type TaskFn<I, O, E> = fn(ExecCtx<E>, I) -> BoxFuture<Result<O, E>>;

/// Stable typed unit of async work.
pub struct Task<I: 'static, O: 'static, E: 'static = Box<dyn std::error::Error + Send + Sync>> {
	id: &'static AtomicU64,
	name: &'static str,
	cache_policy: TaskCachePolicy,
	f: TaskFn<I, O, E>,
}

impl<I, O, E> Task<I, O, E>
where
	I: Clone + Eq + Hash + Send + Sync + 'static,
	O: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Opaque identity for this task definition inside the current process.
	pub fn id(&self) -> TaskId {
		let id = self.id.load(Ordering::Acquire);
		if id != 0 {
			return TaskId(id);
		}

		let next_id = NEXT_TASK_ID.fetch_add(1, Ordering::Relaxed);
		match self
			.id
			.compare_exchange(0, next_id, Ordering::AcqRel, Ordering::Acquire)
		{
			Ok(_) => TaskId(next_id),
			Err(existing) => TaskId(existing),
		}
	}

	fn input_type(&self) -> &'static str {
		std::any::type_name::<I>()
	}

	/// Run this task in an execution context.
	pub async fn run(&self, ctx: &ExecCtx<E>, input: I) -> Result<Arc<O>, E> {
		ctx.resolve(self, input).await
	}

	async fn call(&self, ctx: ExecCtx<E>, input: I) -> StoredOutcome<E> {
		let task_override = ctx.tasks.task_override_for(self);
		match task_override {
			TaskOverride::RunOriginal => self.call_original(ctx, input).await,
			TaskOverride::Replace(replacement) => replacement.call(ctx, input).await,
			TaskOverride::Missing => StoredOutcome::Err(Error::MissingOverride {
				task_name: self.name,
			}),
		}
	}

	async fn call_original(&self, ctx: ExecCtx<E>, input: I) -> StoredOutcome<E> {
		match (self.f)(ctx, input).await {
			Ok(output) => StoredOutcome::Ok(Arc::new(output)),
			Err(error) => StoredOutcome::Err(error),
		}
	}
}

impl<I, O, E> Clone for Task<I, O, E> {
	fn clone(&self) -> Self {
		*self
	}
}

impl<I, O, E> Copy for Task<I, O, E> {}

#[derive(Clone, Copy)]
enum TaskCachePolicy {
	Memoized,
	ExtendedCache { ttl: Duration },
	SingleFlight,
}

#[doc(hidden)]
pub mod __macro_support {
	use std::hash::Hash;
	use std::sync::atomic::AtomicU64;
	use std::time::Duration;

	use super::{Task, TaskCachePolicy, TaskFn};

	#[doc(hidden)]
	pub const fn memoized<I, O, E>(
		id: &'static AtomicU64,
		name: &'static str,
		f: TaskFn<I, O, E>,
	) -> Task<I, O, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
		E: Send + Sync + 'static,
	{
		Task {
			id,
			name,
			cache_policy: TaskCachePolicy::Memoized,
			f,
		}
	}

	#[doc(hidden)]
	pub const fn extended_cache<I, O, E>(
		id: &'static AtomicU64,
		name: &'static str,
		ttl: Duration,
		f: TaskFn<I, O, E>,
	) -> Task<I, O, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
		E: Send + Sync + 'static,
	{
		assert!(
			!ttl.is_zero(),
			"extended task cache TTL must be greater than zero"
		);
		Task {
			id,
			name,
			cache_policy: TaskCachePolicy::ExtendedCache { ttl },
			f,
		}
	}

	#[doc(hidden)]
	pub const fn single_flight<I, O, E>(
		id: &'static AtomicU64,
		name: &'static str,
		f: TaskFn<I, O, E>,
	) -> Task<I, O, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
		E: Send + Sync + 'static,
	{
		Task {
			id,
			name,
			cache_policy: TaskCachePolicy::SingleFlight,
			f,
		}
	}
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

	fn observe(
		&self,
		task_id: TaskId,
		task_name: &'static str,
		task_input_type: &'static str,
		kind: TaskEventKind,
	) {
		if let Some(observer) = &self.inner.observer {
			observer.observe(TaskEvent {
				at: self.now(),
				task_id,
				task_name,
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
			local: Arc::new(Store::new(None)),
			cancel: CancelToken::new(),
			path: Some(Arc::new(PathNode {
				key,
				parent: self.path.clone(),
			})),
		}
	}

	pub(crate) async fn resolve<I, O>(&self, task: &Task<I, O, E>, input: I) -> Result<Arc<O>, E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
		O: Send + Sync + 'static,
	{
		let task_id = task.id();
		let task_name = task.name;
		let task_input_type = task.input_type();
		if self.cancel.is_cancelled() {
			self.tasks.observe(
				task_id,
				task_name,
				task_input_type,
				TaskEventKind::Cancelled,
			);
			return Err(Error::Cancelled);
		}
		let fingerprint = fingerprint_for(task_id, &input);
		if path_contains(self.path.as_deref(), fingerprint, &input) {
			return Err(Error::Cycle { task_name });
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
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::ExecCtxMemoHit,
				);
				if self.cancel.is_cancelled() {
					self.tasks.observe(
						task_id,
						task_name,
						task_input_type,
						TaskEventKind::Cancelled,
					);
					return Err(Error::Cancelled);
				}
				return decode_outcome::<O, E>(outcome, task_name);
			}
			SlotClaim::Wait => {
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::ExecCtxMemoWait,
				);
				return self
					.wait_for_local::<O>(&local_slot, task_id, task_name, task_input_type)
					.await;
			}
			SlotClaim::Run => {
				let guard = RunningGuard::new(self.local.clone(), local_slot.clone());
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::ExecCtxMemoMiss,
				);
				guard
			}
		};

		let outcome = match task.cache_policy {
			TaskCachePolicy::Memoized | TaskCachePolicy::SingleFlight => {
				let path_key = PathKey::from_dyn(local_slot.shared_key());
				self.run_in_exec_ctx(task, input, path_key).await?
			}
			TaskCachePolicy::ExtendedCache { ttl } => {
				match self.resolve_shared(task, input, fingerprint, ttl).await {
					Ok(outcome) => outcome,
					Err(error) => {
						local_guard.remove_and_abandon();
						return Err(error);
					}
				}
			}
		};

		if outcome.is_cancelled() {
			local_guard.remove_and_abandon();
			return decode_outcome::<O, E>(outcome, task_name);
		}

		if matches!(task.cache_policy, TaskCachePolicy::SingleFlight) {
			self.local.remove_slot(&local_slot);
		}
		local_slot.finish(outcome.clone(), None);
		local_guard.disarm();
		decode_outcome::<O, E>(outcome, task_name)
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
		let task_id = task.id();
		let task_name = task.name;
		let task_input_type = task.input_type();
		let child_ctx = self.child_for_task(path_key);
		let observing = self.tasks.has_observer();
		let started_at = observing.then(|| self.tasks.now());
		if observing {
			self.tasks.observe(
				task_id,
				task_name,
				task_input_type,
				TaskEventKind::RunStarted {
					source: TaskRunSource::ExecCtx,
				},
			);
		}

		/*
		Poll-once fast path: memo-dependency and pure-compute bodies are
		ready on their first poll, so wiring the cancellation subscription
		(the select machinery plus, for child tokens, a boxed parent wait
		inside CancelToken::cancelled) is pure overhead for them. Poll the
		call future once with a noop waker; a body that finishes here never
		suspended, so it had no window to observe a cancellation the
		before-run and after-outcome checks do not already cover. Only a
		body that returns Pending — one that actually suspends — falls into
		the select and gains preemptive cancellation, and it does so on the
		same future, so no work is repeated and no poll is lost (the select
		re-registers the real waker, superseding the noop registration).
		*/
		let call = task.call(child_ctx, input);
		tokio::pin!(call);
		// Bind the first poll to its own statement so the noop Context —
		// which is !Send — is dropped before the select's await point and
		// does not taint the surrounding future's Send bound.
		let first_poll = call.as_mut().poll(&mut Context::from_waker(Waker::noop()));
		let outcome = match first_poll {
			Poll::Ready(result) => result,
			Poll::Pending => tokio::select! {
				result = &mut call => result,
				_ = self.cancel.cancelled() => {
					self.tasks.observe(task_id, task_name, task_input_type, TaskEventKind::Cancelled);
					return Err(Error::Cancelled);
				},
			},
		};

		if observing && let Some(started_at) = started_at {
			self.tasks.observe(
				task_id,
				task_name,
				task_input_type,
				TaskEventKind::RunCompleted {
					source: TaskRunSource::ExecCtx,
					outcome: task_event_outcome(&outcome),
					duration: self.tasks.now().saturating_duration_since(started_at),
				},
			);
		}
		if self.cancel.is_cancelled() {
			self.tasks.observe(
				task_id,
				task_name,
				task_input_type,
				TaskEventKind::Cancelled,
			);
			return Err(Error::Cancelled);
		}
		Ok(outcome)
	}

	async fn wait_for_local<O>(
		&self,
		slot: &Arc<Slot<E>>,
		task_id: TaskId,
		task_name: &'static str,
		task_input_type: &'static str,
	) -> Result<Arc<O>, E>
	where
		O: Send + Sync + 'static,
	{
		let outcome = self
			.wait_for_slot(slot, task_id, task_name, task_input_type)
			.await?;
		decode_outcome::<O, E>(outcome, task_name)
	}

	/*
	Lost-wakeup safety: a waiter must register with the notifier BEFORE
	re-checking slot state. notify_waiters stores no permit, so checking
	first would leave a window — runner finishes and notifies between
	the check and the await — that strands the waiter forever.
	*/
	async fn wait_for_slot(
		&self,
		slot: &Arc<Slot<E>>,
		task_id: TaskId,
		task_name: &'static str,
		task_input_type: &'static str,
	) -> Result<StoredOutcome<E>, E> {
		slot.register_waiter();
		let guard = WaiterGuard { slot };
		loop {
			if self.cancel.is_cancelled() {
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::Cancelled,
				);
				return Err(Error::Cancelled);
			}
			let notified = slot.signal.notified();
			tokio::pin!(notified);
			notified.as_mut().enable();
			match slot.claim() {
				SlotClaim::Ready(outcome) => {
					if self.cancel.is_cancelled() {
						self.tasks.observe(
							task_id,
							task_name,
							task_input_type,
							TaskEventKind::Cancelled,
						);
						return Err(Error::Cancelled);
					}
					drop(guard);
					return Ok(outcome);
				}
				SlotClaim::Run => {
					slot.abandon();
					return Err(Error::Cancelled);
				}
				SlotClaim::Wait => {
					tokio::select! {
						_ = &mut notified => {}
						_ = self.cancel.cancelled() => {
							self.tasks.observe(task_id, task_name, task_input_type, TaskEventKind::Cancelled);
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
		let task_id = task.id();
		let task_name = task.name;
		let task_input_type = task.input_type();
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
						task_id,
						task_name,
						task_input_type,
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
						task_id,
						task_name,
						task_input_type,
						TaskEventKind::CrossExecCtxStaleSlotRemoved {
							count: stale_slots_removed,
						},
					);
				}
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::CrossExecCtxCacheCapacityBypass { max_entries },
				);
				let path_key = PathKey::new(task_id, &input);
				return self.run_in_exec_ctx(task, input, path_key).await;
			}
		};

		match shared_slot.claim() {
			SlotClaim::Ready(outcome) => {
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::CrossExecCtxCacheHit,
				);
				if self.cancel.is_cancelled() {
					self.tasks.observe(
						task_id,
						task_name,
						task_input_type,
						TaskEventKind::Cancelled,
					);
					return Err(Error::Cancelled);
				}
				return Ok(outcome);
			}
			SlotClaim::Wait => {
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::CrossExecCtxInFlightWait,
				);
			}
			SlotClaim::Run => {
				let tasks = self.tasks.clone();
				let guard = SharedRunningGuard::new(tasks.clone(), shared_slot.clone());
				self.tasks.observe(
					task_id,
					task_name,
					task_input_type,
					TaskEventKind::CrossExecCtxCacheMiss,
				);
				let task_for_spawn = *task;
				let input_for_spawn = input.clone();
				let run_ctx = self.shared_run_child(PathKey::from_dyn(shared_slot.shared_key()));
				let shared_slot_for_run = shared_slot.clone();
				let join = tokio::spawn(async move {
					let observing = tasks.has_observer();
					let started_at = observing.then(|| tasks.now());
					if observing {
						tasks.observe(
							task_id,
							task_name,
							task_input_type,
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
							task_input_type,
							TaskEventKind::RunCompleted {
								source: TaskRunSource::CrossExecCtx,
								outcome: task_event_outcome(&outcome),
								duration: tasks.now().saturating_duration_since(started_at),
							},
						);
					}
					if outcome.is_cancelled() {
						guard.remove_and_abandon();
						return outcome;
					}
					let expires_at = if outcome.is_ok() {
						Some(tasks.now().saturating_add_duration(ttl))
					} else {
						Some(tasks.now())
					};
					if outcome.is_ok() {
						tasks.observe(
							task_id,
							task_name,
							task_input_type,
							TaskEventKind::CrossExecCtxCacheInserted,
						);
					}
					shared_slot_for_run.finish(outcome.clone(), expires_at);
					guard.disarm();
					outcome
				});
				return self
					.wait_for_shared_runner(join, task_id, task_name, task_input_type)
					.await;
			}
		}

		self.wait_for_slot(&shared_slot, task_id, task_name, task_input_type)
			.await
	}

	async fn wait_for_shared_runner(
		&self,
		join: tokio::task::JoinHandle<StoredOutcome<E>>,
		task_id: TaskId,
		task_name: &'static str,
		task_input_type: &'static str,
	) -> Result<StoredOutcome<E>, E> {
		let joined = tokio::select! {
			result = join => result,
			_ = self.cancel.cancelled() => {
				self.tasks.observe(task_id, task_name, task_input_type, TaskEventKind::Cancelled);
				return Err(Error::Cancelled);
			},
		};

		match joined {
			Ok(outcome) => {
				if self.cancel.is_cancelled() {
					self.tasks.observe(
						task_id,
						task_name,
						task_input_type,
						TaskEventKind::Cancelled,
					);
					return Err(Error::Cancelled);
				}
				Ok(outcome)
			}
			Err(error) if error.is_panic() => panic::resume_unwind(error.into_panic()),
			Err(_) => Err(Error::Cancelled),
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

struct SharedRunningGuard<E> {
	tasks: Tasks<E>,
	slot: Arc<Slot<E>>,
	active: bool,
}

impl<E> SharedRunningGuard<E> {
	fn new(tasks: Tasks<E>, slot: Arc<Slot<E>>) -> Self {
		Self {
			tasks,
			slot,
			active: true,
		}
	}

	fn disarm(mut self) {
		self.active = false;
	}

	fn remove_and_abandon(mut self) {
		self.tasks.inner.shared.remove_slot(&self.slot);
		self.slot.abandon();
		self.active = false;
	}
}

impl<E> Drop for SharedRunningGuard<E> {
	fn drop(&mut self) {
		if self.active {
			self.tasks.inner.shared.remove_slot(&self.slot);
			self.slot.abandon();
		}
	}
}

fn task_event_outcome<E>(outcome: &StoredOutcome<E>) -> TaskEventOutcome {
	match outcome {
		StoredOutcome::Ok(_) => TaskEventOutcome::Success,
		StoredOutcome::Err(error) if error.is_cancelled() => TaskEventOutcome::Cancelled,
		StoredOutcome::Err(_) => TaskEventOutcome::Error,
	}
}
