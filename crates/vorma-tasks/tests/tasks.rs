use std::hash::{Hash, Hasher};
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use tokio::sync::{Notify, watch};
use tokio::time;
use vorma_tasks::{
	CancelToken, Clock, ClockInstant, Error, ParallelBatch, TaskEvent, TaskEventKind,
	TaskEventOutcome, TaskOverrideMode, TaskOverrides, TaskRunSource, Tasks, TasksOptions,
};

struct CaseInput<V, S> {
	value: V,
	state: Arc<S>,
}

impl<V, S> Clone for CaseInput<V, S>
where
	V: Clone,
{
	fn clone(&self) -> Self {
		Self {
			value: self.value.clone(),
			state: self.state.clone(),
		}
	}
}

impl<V, S> CaseInput<V, S> {
	fn new(value: V, state: Arc<S>) -> Self {
		Self { value, state }
	}
}

impl<V, S> PartialEq for CaseInput<V, S>
where
	V: PartialEq,
{
	fn eq(&self, other: &Self) -> bool {
		self.value == other.value && Arc::ptr_eq(&self.state, &other.state)
	}
}

impl<V, S> Eq for CaseInput<V, S> where V: Eq {}

impl<V, S> Hash for CaseInput<V, S>
where
	V: Hash,
{
	fn hash<H: Hasher>(&self, state: &mut H) {
		self.value.hash(state);
		(Arc::as_ptr(&self.state) as usize).hash(state);
	}
}

#[derive(Default)]
struct CountState {
	runs: AtomicUsize,
}

struct ClockState {
	runs: AtomicUsize,
	clock: ManualClock,
}

struct GatedState {
	runs: AtomicUsize,
	started: watch::Sender<usize>,
	release: Notify,
}

struct ParallelState {
	started: watch::Sender<usize>,
	started_count: AtomicUsize,
	release: Notify,
	first_runs: AtomicUsize,
	second_runs: AtomicUsize,
}

struct ParallelErrorState {
	started: watch::Sender<usize>,
	wait_runs: AtomicUsize,
}

struct ParallelCompetingErrorsState {
	started: watch::Sender<usize>,
	started_count: AtomicUsize,
	release_added_first: Notify,
	release_added_second: Notify,
}

struct CancelOnSuccessState {
	runs: AtomicUsize,
	cancel: CancelToken,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct Collision(u8);

#[derive(Clone, Eq, Hash, PartialEq)]
struct PatternInput {
	left: u32,
	right: u32,
}

type AliasTask<I, O, E> = vorma_tasks::Task<I, O, E>;

mod task_aliases {
	pub type ModuleAliasTask<I, O, E> = vorma_tasks::Task<I, O, E>;
}

struct PanicEqInput {
	value: u8,
	panic_on_eq: Arc<AtomicBool>,
	state: Arc<CountState>,
}

#[derive(Clone, Eq, PartialEq)]
struct PanicHashInput;

impl PanicEqInput {
	fn new(value: u8, panic_on_eq: Arc<AtomicBool>, state: Arc<CountState>) -> Self {
		Self {
			value,
			panic_on_eq,
			state,
		}
	}
}

impl Clone for PanicEqInput {
	fn clone(&self) -> Self {
		Self {
			value: self.value,
			panic_on_eq: self.panic_on_eq.clone(),
			state: self.state.clone(),
		}
	}
}

impl PartialEq for PanicEqInput {
	fn eq(&self, other: &Self) -> bool {
		if self.panic_on_eq.swap(false, Ordering::SeqCst) {
			panic!("intentional equality panic");
		}
		self.value == other.value && Arc::ptr_eq(&self.state, &other.state)
	}
}

impl Eq for PanicEqInput {}

impl Hash for Collision {
	fn hash<H: Hasher>(&self, state: &mut H) {
		1u8.hash(state);
	}
}

impl Hash for PanicEqInput {
	fn hash<H: Hasher>(&self, state: &mut H) {
		1u8.hash(state);
	}
}

impl Hash for PanicHashInput {
	fn hash<H: Hasher>(&self, _state: &mut H) {
		panic!("intentional hash panic");
	}
}

#[derive(Clone, Default)]
struct ManualClock {
	nanos: Arc<AtomicU64>,
}

#[derive(Default)]
struct PanicOnceClock {
	panicked: AtomicBool,
}

impl ManualClock {
	fn advance(&self, duration: Duration) {
		self.nanos
			.fetch_add(duration.as_nanos() as u64, Ordering::SeqCst);
	}
}

impl Clock for ManualClock {
	fn now(&self) -> ClockInstant {
		ClockInstant::from_duration_since_origin(Duration::from_nanos(
			self.nanos.load(Ordering::SeqCst),
		))
	}
}

impl Clock for PanicOnceClock {
	fn now(&self) -> ClockInstant {
		if !self.panicked.swap(true, Ordering::SeqCst) {
			panic!("intentional clock panic");
		}
		ClockInstant::from_duration_since_origin(Duration::ZERO)
	}
}

vorma_tasks::task! {
	static COUNTED_MEMOIZED: Task<CaseInput<u32, CountState>, u32, &'static str> =
		memoized(|_ctx, input: CaseInput<u32, CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value * 2)
		});
}

vorma_tasks::task! {
	static COUNTED_EXTENDED_10MS: Task<CaseInput<u32, CountState>, u32, &'static str> =
		extended_cache(Duration::from_millis(10), |_ctx, input: CaseInput<u32, CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value * 2)
		});
}

vorma_tasks::task! {
	static COUNTED_EXTENDED_60S: Task<CaseInput<u32, CountState>, u32, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<u32, CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value * 2)
		});
}

vorma_tasks::task! {
	static SECOND_COUNTED_EXTENDED_60S: Task<CaseInput<u32, CountState>, u32, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<u32, CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value * 2)
		});
}

vorma_tasks::task! {
	static CLOCK_EXTENDED_60S: Task<CaseInput<(), ClockState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), ClockState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			input.state.clock.advance(Duration::from_millis(7));
			Ok(1)
		});
}

vorma_tasks::task! {
	static GATED_MEMOIZED: Task<CaseInput<(), GatedState>, usize, &'static str> =
		memoized(|_ctx, input: CaseInput<(), GatedState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			input.state.release.notified().await;
			Ok(run)
		});
}

vorma_tasks::task! {
	static GATED_EXTENDED_50MS: Task<CaseInput<(), GatedState>, usize, &'static str> =
		extended_cache(Duration::from_millis(50), |_ctx, input: CaseInput<(), GatedState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			input.state.release.notified().await;
			Ok(run)
		});
}

vorma_tasks::task! {
	static GATED_EXTENDED_60S: Task<CaseInput<(), GatedState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), GatedState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			input.state.release.notified().await;
			Ok(run)
		});
}

vorma_tasks::task! {
	static GATED_EXTENDED_ERROR_THEN_SUCCESS: Task<CaseInput<(), GatedState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), GatedState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			if run == 1 {
				input.state.release.notified().await;
				return Err("boom".into());
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static EXTENDED_THROUGH_GATED_MEMOIZED: Task<CaseInput<(), GatedState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |ctx, input: CaseInput<(), GatedState>| async move {
			let value = GATED_MEMOIZED.run(&ctx, input).await?;
			Ok(*value)
		});
}

vorma_tasks::task! {
	static EXTENDED_CANCEL_CALLER_THEN_SUCCESS: Task<CaseInput<(), CancelOnSuccessState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), CancelOnSuccessState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			input.state.cancel.cancel();
			Ok(run)
		});
}

vorma_tasks::task! {
	static PARENT_COUNTED: Task<CaseInput<u32, CountState>, u32, &'static str> =
		memoized(|ctx, input: CaseInput<u32, CountState>| async move {
			let left_input = input.clone();
			let right_input = input;
			let (left, right) = tokio::try_join!(
				COUNTED_MEMOIZED.run(&ctx, left_input),
				COUNTED_MEMOIZED.run(&ctx, right_input),
			)?;
			Ok(*left + *right)
		});
}

vorma_tasks::task! {
	static PARALLEL_FIRST: Task<CaseInput<u32, ParallelState>, u32, &'static str> =
		memoized(|_ctx, input: CaseInput<u32, ParallelState>| async move {
			input.state.first_runs.fetch_add(1, Ordering::SeqCst);
			let count = input.state.started_count.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(count);
			input.state.release.notified().await;
			Ok(input.value * 2)
		});
}

vorma_tasks::task! {
	static PARALLEL_SECOND: Task<CaseInput<&'static str, ParallelState>, String, &'static str> =
		memoized(|_ctx, input: CaseInput<&'static str, ParallelState>| async move {
			input.state.second_runs.fetch_add(1, Ordering::SeqCst);
			let count = input.state.started_count.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(count);
			input.state.release.notified().await;
			Ok(format!("{}-done", input.value))
		});
}

vorma_tasks::task! {
	static PARALLEL_WAIT: Task<CaseInput<(), ParallelErrorState>, usize, &'static str> =
		memoized(|ctx, input: CaseInput<(), ParallelErrorState>| async move {
			let run = input.state.wait_runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			if run == 1 {
				ctx.cancel_token().cancelled().await;
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static PARALLEL_FAIL: Task<CaseInput<(), ParallelErrorState>, (), &'static str> =
		memoized(|_ctx, input: CaseInput<(), ParallelErrorState>| async move {
			let mut started = input.state.started.subscribe();
			wait_for_started(&mut started, 1).await;
			Err("boom".into())
		});
}

vorma_tasks::task! {
	static PARALLEL_FAIL_ADDED_FIRST: Task<CaseInput<(), ParallelCompetingErrorsState>, (), &'static str> =
		memoized(|_ctx, input: CaseInput<(), ParallelCompetingErrorsState>| async move {
			let count = input.state.started_count.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(count);
			input.state.release_added_first.notified().await;
			Err("error-from-task-added-first".into())
		});
}

vorma_tasks::task! {
	static PARALLEL_FAIL_ADDED_SECOND: Task<CaseInput<(), ParallelCompetingErrorsState>, (), &'static str> =
		memoized(|_ctx, input: CaseInput<(), ParallelCompetingErrorsState>| async move {
			let count = input.state.started_count.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(count);
			input.state.release_added_second.notified().await;
			Err("error-from-task-added-second".into())
		});
}

vorma_tasks::task! {
	static COLLISION_TASK: Task<CaseInput<Collision, CountState>, u8, &'static str> =
		memoized(|_ctx, input: CaseInput<Collision, CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value.0)
		});
}

vorma_tasks::task! {
	static PANIC_EQ_TASK: Task<PanicEqInput, u8, &'static str> =
		memoized(|_ctx, input: PanicEqInput| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Ok(input.value)
		});
}

vorma_tasks::task! {
	static PANIC_HASH_TASK: Task<PanicHashInput, (), &'static str> =
		memoized(|_ctx, _input: PanicHashInput| async move { Ok(()) });
}

vorma_tasks::task! {
	static DESTRUCTURED_INPUT: Task<(u32, u32), u32, &'static str> =
		memoized(|_ctx, (left, right)| async move { Ok(left + right) });
}

vorma_tasks::task! {
	static TYPED_PATTERN_INPUT: Task<(u32, u32), u32, &'static str> =
		memoized(|_ctx, (left, right): (u32, u32)| async move { Ok(left * right) });
}

vorma_tasks::task! {
	static MUT_TYPED_INPUT: Task<u32, u32, &'static str> =
		memoized(|_ctx, mut input: u32| async move {
			input += 1;
			Ok(input)
		});
}

vorma_tasks::task! {
	static STRUCT_TYPED_PATTERN_INPUT: Task<PatternInput, u32, &'static str> =
		memoized(|_ctx, PatternInput { left, right }: PatternInput| async move {
			Ok(left + right)
		});
}

vorma_tasks::task! {
	static ABSOLUTE_PATH_TASK: ::vorma_tasks::Task<(), (), &'static str> =
		memoized(|_ctx, _input| async move { Ok(()) });
}

vorma_tasks::task! {
	static ALIAS_PATH_TASK: AliasTask<(), (), &'static str> =
		memoized(|_ctx, _input| async move { Ok(()) });
}

vorma_tasks::task! {
	static MODULE_ALIAS_PATH_TASK: task_aliases::ModuleAliasTask<(), (), &'static str> =
		memoized(|_ctx, _input| async move { Ok(()) });
}

vorma_tasks::task! {
	static SELF_CYCLE: Task<(), (), &'static str> =
		memoized(|ctx, _input: ()| async move {
			SELF_CYCLE.run(&ctx, ()).await?;
			Ok(())
		});
}

vorma_tasks::task! {
	static MEMOIZED_ERROR: Task<CaseInput<(), CountState>, (), &'static str> =
		memoized(|_ctx, input: CaseInput<(), CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Err("boom".into())
		});
}

vorma_tasks::task! {
	static EXTENDED_ERROR_THEN_SUCCESS: Task<CaseInput<(), CountState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), CountState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			if run == 1 {
				return Err("boom".into());
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static MEMOIZED_CANCEL: Task<CaseInput<(), CountState>, (), &'static str> =
		memoized(|_ctx, input: CaseInput<(), CountState>| async move {
			input.state.runs.fetch_add(1, Ordering::SeqCst);
			Err(Error::Cancelled)
		});
}

vorma_tasks::task! {
	static EXTENDED_CANCEL_THEN_SUCCESS: Task<CaseInput<(), CountState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), CountState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			if run == 1 {
				return Err(Error::Cancelled);
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static PANIC_THEN_SUCCESS: Task<CaseInput<(), CountState>, usize, &'static str> =
		memoized(|_ctx, input: CaseInput<(), CountState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			if run == 1 {
				panic!("intentional test panic");
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static EXTENDED_PANIC_THEN_SUCCESS: Task<CaseInput<(), CountState>, usize, &'static str> =
		extended_cache(Duration::from_secs(60), |_ctx, input: CaseInput<(), CountState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			if run == 1 {
				panic!("intentional test panic");
			}
			Ok(run)
		});
}

vorma_tasks::task! {
	static SINGLE_FLIGHT_GATED: Task<CaseInput<(), GatedState>, usize, &'static str> =
		single_flight(|_ctx, input: CaseInput<(), GatedState>| async move {
			let run = input.state.runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = input.state.started.send(run);
			input.state.release.notified().await;
			Ok(run)
		});
}

fn count_state() -> Arc<CountState> {
	Arc::new(CountState::default())
}

fn gated_state() -> (Arc<GatedState>, watch::Receiver<usize>) {
	let (started, started_rx) = watch::channel(0usize);
	(
		Arc::new(GatedState {
			runs: AtomicUsize::new(0),
			started,
			release: Notify::new(),
		}),
		started_rx,
	)
}

fn exec_ctx<E>(tasks: &Tasks<E>) -> vorma_tasks::ExecCtx<E>
where
	E: Send + Sync + 'static,
{
	tasks.exec_ctx(CancelToken::new())
}

fn tasks<E>() -> Tasks<E>
where
	E: Send + Sync + 'static,
{
	Tasks::new(TasksOptions::default())
}

fn tasks_with_cache_capacity<E>(max_entries: usize) -> Tasks<E>
where
	E: Send + Sync + 'static,
{
	Tasks::new(TasksOptions {
		max_cross_exec_ctx_cache_entries: max_entries,
		..TasksOptions::default()
	})
}

fn tasks_with_clock<E>(clock: ManualClock) -> Tasks<E>
where
	E: Send + Sync + 'static,
{
	Tasks::new(TasksOptions {
		clock: Arc::new(clock),
		..TasksOptions::default()
	})
}

fn tasks_with_overrides<E>(overrides: TaskOverrides<E>) -> Tasks<E>
where
	E: Send + Sync + 'static,
{
	Tasks::new(TasksOptions {
		overrides: Some(overrides),
		..TasksOptions::default()
	})
}

fn tasks_with_clock_observer<E, F>(clock: ManualClock, observer: F) -> Tasks<E>
where
	E: Send + Sync + 'static,
	F: vorma_tasks::TaskObserver,
{
	Tasks::new(TasksOptions {
		clock: Arc::new(clock),
		observer: Some(Arc::new(observer)),
		..TasksOptions::default()
	})
}

fn tasks_with_cache_capacity_observer<E, F>(max_entries: usize, observer: F) -> Tasks<E>
where
	E: Send + Sync + 'static,
	F: vorma_tasks::TaskObserver,
{
	Tasks::new(TasksOptions {
		max_cross_exec_ctx_cache_entries: max_entries,
		observer: Some(Arc::new(observer)),
		..TasksOptions::default()
	})
}

fn event_log() -> (
	Arc<Mutex<Vec<TaskEvent>>>,
	impl Fn(TaskEvent) + Send + Sync + 'static,
) {
	let events = Arc::new(Mutex::new(Vec::new()));
	let observer_events = events.clone();
	let observer = move |event| {
		observer_events
			.lock()
			.expect("task event log lock poisoned")
			.push(event);
	};
	(events, observer)
}

fn event_kinds(events: &Arc<Mutex<Vec<TaskEvent>>>) -> Vec<TaskEventKind> {
	events
		.lock()
		.expect("task event log lock poisoned")
		.iter()
		.map(|event| event.kind.clone())
		.collect()
}

#[tokio::test]
async fn child_cancel_token_follows_parent_without_cancelling_parent() {
	let parent = CancelToken::new();
	let child = parent.child();

	child.cancel();
	assert!(child.is_cancelled());
	assert!(!parent.is_cancelled());

	let inherited = parent.child();
	parent.cancel();
	inherited.cancelled().await;
	assert!(inherited.is_cancelled());
}

#[tokio::test]
async fn required_overrides_do_not_run_unmatched_task_bodies() {
	let state = count_state();
	let task_input = CaseInput::new(1, state.clone());
	let overrides = TaskOverrides::new(TaskOverrideMode::RequireOverride);
	let tasks = tasks_with_overrides(overrides);
	let err = COUNTED_MEMOIZED
		.run(&exec_ctx(&tasks), task_input)
		.await
		.unwrap_err();

	assert!(matches!(err, Error::MissingOverride { .. }));
	assert_eq!(state.runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn run_unmatched_overrides_allow_original_task_bodies() {
	let state = count_state();
	let overrides = TaskOverrides::new(TaskOverrideMode::RunUnmatched);
	let tasks = tasks_with_overrides(overrides);

	assert_eq!(
		*COUNTED_MEMOIZED
			.run(&exec_ctx(&tasks), CaseInput::new(4, state.clone()))
			.await
			.unwrap(),
		8
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_overrides_replace_task_bodies_and_memoize_inside_exec_ctx() {
	let body_state = count_state();
	let override_runs = Arc::new(AtomicUsize::new(0));
	let task_override_runs = override_runs.clone();
	let overrides = TaskOverrides::new(TaskOverrideMode::RequireOverride).replace(
		&COUNTED_MEMOIZED,
		move |_ctx, input| {
			let override_runs = task_override_runs.clone();
			async move {
				override_runs.fetch_add(1, Ordering::SeqCst);
				Ok(input.value * 10)
			}
		},
	);
	let tasks = tasks_with_overrides(overrides);
	let ctx = exec_ctx(&tasks);
	let input = CaseInput::new(3, body_state.clone());

	let (a, b) = tokio::join!(
		COUNTED_MEMOIZED.run(&ctx, input.clone()),
		COUNTED_MEMOIZED.run(&ctx, input),
	);

	assert_eq!(*a.unwrap(), 30);
	assert_eq!(*b.unwrap(), 30);
	assert_eq!(body_state.runs.load(Ordering::SeqCst), 0);
	assert_eq!(override_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_overrides_participate_in_cross_exec_ctx_cache() {
	let body_state = count_state();
	let override_runs = Arc::new(AtomicUsize::new(0));
	let task_override_runs = override_runs.clone();
	let overrides = TaskOverrides::new(TaskOverrideMode::RequireOverride).replace(
		&COUNTED_EXTENDED_60S,
		move |_ctx, input| {
			let override_runs = task_override_runs.clone();
			async move {
				override_runs.fetch_add(1, Ordering::SeqCst);
				Ok(input.value * 10)
			}
		},
	);
	let tasks = tasks_with_overrides(overrides);
	let input = CaseInput::new(3, body_state.clone());

	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		30
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		30
	);
	assert_eq!(body_state.runs.load(Ordering::SeqCst), 0);
	assert_eq!(override_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_exec_ctx_memo_and_run_events() {
	let state = count_state();
	let input = CaseInput::new(5, state.clone());
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let ctx = exec_ctx(&tasks);

	assert_eq!(
		*COUNTED_MEMOIZED.run(&ctx, input.clone()).await.unwrap(),
		10
	);
	assert_eq!(*COUNTED_MEMOIZED.run(&ctx, input).await.unwrap(), 10);

	{
		let events = events.lock().expect("task event log lock poisoned");
		assert!(
			events
				.iter()
				.all(|event| event.task_id == COUNTED_MEMOIZED.id())
		);
		assert!(
			events
				.iter()
				.all(|event| event.task_name == concat!(module_path!(), "::COUNTED_MEMOIZED"))
		);
		assert!(
			events.iter().all(|event| event.task_input_type
				== std::any::type_name::<CaseInput<u32, CountState>>())
		);
	}

	assert_eq!(
		event_kinds(&events),
		vec![
			TaskEventKind::ExecCtxMemoMiss,
			TaskEventKind::RunStarted {
				source: TaskRunSource::ExecCtx
			},
			TaskEventKind::RunCompleted {
				source: TaskRunSource::ExecCtx,
				outcome: TaskEventOutcome::Success,
				duration: Duration::ZERO
			},
			TaskEventKind::ExecCtxMemoHit,
		],
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_cross_exec_ctx_cache_and_duration_events() {
	let clock = ManualClock::default();
	let state = Arc::new(ClockState {
		runs: AtomicUsize::new(0),
		clock: clock.clone(),
	});
	let input = CaseInput::new((), state.clone());
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(clock, observer);

	assert_eq!(
		*CLOCK_EXTENDED_60S
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		1
	);
	assert_eq!(
		*CLOCK_EXTENDED_60S
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		1
	);

	assert_eq!(
		event_kinds(&events),
		vec![
			TaskEventKind::ExecCtxMemoMiss,
			TaskEventKind::CrossExecCtxCacheMiss,
			TaskEventKind::RunStarted {
				source: TaskRunSource::CrossExecCtx
			},
			TaskEventKind::RunCompleted {
				source: TaskRunSource::CrossExecCtx,
				outcome: TaskEventOutcome::Success,
				duration: Duration::from_millis(7)
			},
			TaskEventKind::CrossExecCtxCacheInserted,
			TaskEventKind::ExecCtxMemoMiss,
			TaskEventKind::CrossExecCtxCacheHit,
		],
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_cross_exec_ctx_in_flight_waits() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(GATED_EXTENDED_60S.run(&first_ctx, input.clone()));
	let mut second = Box::pin(GATED_EXTENDED_60S.run(&second_ctx, input));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}
	state.release.notify_one();

	assert_eq!(*first.await.unwrap(), 1);
	assert_eq!(*second.await.unwrap(), 1);
	assert!(event_kinds(&events).contains(&TaskEventKind::CrossExecCtxInFlightWait));
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_stale_cross_exec_ctx_slot_cleanup() {
	let state = count_state();
	let input = CaseInput::new(2, state.clone());
	let clock = ManualClock::default();
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(clock.clone(), observer);

	assert_eq!(
		*COUNTED_EXTENDED_10MS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		4
	);
	clock.advance(Duration::from_millis(10));
	assert_eq!(
		*COUNTED_EXTENDED_10MS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		4
	);

	assert!(
		event_kinds(&events).contains(&TaskEventKind::CrossExecCtxStaleSlotRemoved { count: 1 })
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn observer_reports_cancellation_events() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut pending = Box::pin(GATED_MEMOIZED.run(&ctx, input));

	tokio::select! {
		result = &mut pending => panic!("task finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	assert!(pending.await.unwrap_err().is_cancelled());
	assert!(event_kinds(&events).contains(&TaskEventKind::Cancelled));
}

#[tokio::test]
async fn cancelled_context_does_not_hash_input() {
	let tasks = tasks();
	let cancel = CancelToken::new();
	cancel.cancel();
	let ctx = tasks.exec_ctx(cancel);

	assert!(
		PANIC_HASH_TASK
			.run(&ctx, PanicHashInput)
			.await
			.unwrap_err()
			.is_cancelled()
	);
}

#[tokio::test]
async fn panicking_clock_does_not_poison_cross_exec_ctx_store() {
	let state = count_state();
	let tasks = Tasks::new(TasksOptions {
		clock: Arc::new(PanicOnceClock::default()),
		..TasksOptions::default()
	});
	let first_tasks = tasks.clone();
	let first_state = state.clone();
	let first = tokio::spawn(async move {
		COUNTED_EXTENDED_60S
			.run(&exec_ctx(&first_tasks), CaseInput::new(1, first_state))
			.await
	});

	assert!(first.await.expect_err("clock should panic").is_panic());
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(1, state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_panic_after_exec_ctx_slot_claim_does_not_strand_local_slot() {
	let state = count_state();
	let input = CaseInput::new(5, state.clone());
	let panicked = Arc::new(AtomicBool::new(false));
	let observer_panicked = panicked.clone();
	let tasks = Tasks::new(TasksOptions {
		observer: Some(Arc::new(move |event: TaskEvent| {
			if matches!(event.kind, TaskEventKind::ExecCtxMemoMiss)
				&& !observer_panicked.swap(true, Ordering::SeqCst)
			{
				panic!("intentional observer panic");
			}
		})),
		..TasksOptions::default()
	});
	let ctx = exec_ctx(&tasks);
	let first_ctx = ctx.clone();
	let first = tokio::spawn(async move { COUNTED_MEMOIZED.run(&first_ctx, input.clone()).await });

	assert!(first.await.expect_err("observer should panic").is_panic());
	assert_eq!(
		*time::timeout(
			Duration::from_secs(1),
			COUNTED_MEMOIZED.run(&ctx, CaseInput::new(5, state.clone()))
		)
		.await
		.expect("local slot should reopen after observer panic")
		.unwrap(),
		10
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_panic_after_cross_exec_ctx_slot_claim_does_not_strand_shared_slot() {
	let state = count_state();
	let input = CaseInput::new(5, state.clone());
	let panicked = Arc::new(AtomicBool::new(false));
	let observer_panicked = panicked.clone();
	let tasks = Tasks::new(TasksOptions {
		observer: Some(Arc::new(move |event: TaskEvent| {
			if matches!(event.kind, TaskEventKind::CrossExecCtxCacheMiss)
				&& !observer_panicked.swap(true, Ordering::SeqCst)
			{
				panic!("intentional observer panic");
			}
		})),
		..TasksOptions::default()
	});
	let first_ctx = exec_ctx(&tasks);
	let first =
		tokio::spawn(async move { COUNTED_EXTENDED_60S.run(&first_ctx, input.clone()).await });

	assert!(first.await.expect_err("observer should panic").is_panic());
	assert_eq!(
		*time::timeout(
			Duration::from_secs(1),
			COUNTED_EXTENDED_60S.run(&exec_ctx(&tasks), CaseInput::new(5, state.clone()))
		)
		.await
		.expect("shared slot should reopen after observer panic")
		.unwrap(),
		10
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_runs_once_per_exec_ctx() {
	let state = count_state();
	let input = CaseInput::new(7, state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	let (a, b) = tokio::join!(
		COUNTED_MEMOIZED.run(&ctx, input.clone()),
		COUNTED_MEMOIZED.run(&ctx, input),
	);

	assert_eq!(*a.unwrap(), 14);
	assert_eq!(*b.unwrap(), 14);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn separate_exec_ctx_values_do_not_share_without_cross_exec_ctx_cache() {
	let state = count_state();
	let input = CaseInput::new(1, state.clone());
	let tasks = tasks();

	assert_eq!(
		*COUNTED_MEMOIZED
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		2
	);
	assert_eq!(
		*COUNTED_MEMOIZED
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_reuses_completed_values_until_expiration() {
	let state = count_state();
	let input = CaseInput::new(2, state.clone());
	let clock = ManualClock::default();
	let tasks = tasks_with_clock(clock.clone());

	assert_eq!(
		*COUNTED_EXTENDED_10MS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		4
	);
	clock.advance(Duration::from_millis(9));
	assert_eq!(
		*COUNTED_EXTENDED_10MS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		4
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);

	clock.advance(Duration::from_millis(1));
	assert_eq!(
		*COUNTED_EXTENDED_10MS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		4
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_ttl_starts_after_task_completion() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let clock = ManualClock::default();
	let tasks = tasks_with_clock(clock.clone());
	let ctx = exec_ctx(&tasks);
	let mut first = Box::pin(GATED_EXTENDED_50MS.run(&ctx, input.clone()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	clock.advance(Duration::from_millis(60));
	state.release.notify_one();
	assert_eq!(*first.await.unwrap(), 1);

	assert_eq!(
		*GATED_EXTENDED_50MS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		1
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);

	clock.advance(Duration::from_millis(50));
	let third_ctx = exec_ctx(&tasks);
	let mut third = Box::pin(GATED_EXTENDED_50MS.run(&third_ctx, input));

	tokio::select! {
		result = &mut third => panic!("third run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}

	state.release.notify_one();
	assert_eq!(*third.await.unwrap(), 2);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn task_composition_shares_one_exec_ctx() {
	let state = count_state();
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(
		*PARENT_COUNTED
			.run(&ctx, CaseInput::new(3, state.clone()))
			.await
			.unwrap(),
		12
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_macro_accepts_destructured_input_pattern() {
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(*DESTRUCTURED_INPUT.run(&ctx, (2, 5)).await.unwrap(), 7);
	assert_eq!(*TYPED_PATTERN_INPUT.run(&ctx, (2, 5)).await.unwrap(), 10);
	assert_eq!(*MUT_TYPED_INPUT.run(&ctx, 9).await.unwrap(), 10);
	assert_eq!(
		*STRUCT_TYPED_PATTERN_INPUT
			.run(&ctx, PatternInput { left: 3, right: 4 })
			.await
			.unwrap(),
		7
	);
}

#[tokio::test]
async fn task_macro_accepts_absolute_task_type_path() {
	let tasks = tasks();

	ABSOLUTE_PATH_TASK.run(&exec_ctx(&tasks), ()).await.unwrap();
}

#[tokio::test]
async fn task_macro_accepts_task_type_alias() {
	let tasks = tasks();

	ALIAS_PATH_TASK.run(&exec_ctx(&tasks), ()).await.unwrap();
	MODULE_ALIAS_PATH_TASK
		.run(&exec_ctx(&tasks), ())
		.await
		.unwrap();
}

#[tokio::test]
async fn parallel_runs_added_tasks_concurrently_and_stores_results() {
	let (started, mut started_rx) = watch::channel(0usize);
	let state = Arc::new(ParallelState {
		started,
		started_count: AtomicUsize::new(0),
		release: Notify::new(),
		first_runs: AtomicUsize::new(0),
		second_runs: AtomicUsize::new(0),
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);
	let mut batch = ParallelBatch::new();
	let first_output = batch.add(PARALLEL_FIRST, CaseInput::new(5, state.clone()));
	let second_output = batch.add(PARALLEL_SECOND, CaseInput::new("ok", state.clone()));
	let mut pending = Box::pin(batch.run(&ctx));

	tokio::select! {
		result = &mut pending => panic!("parallel run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}

	state.release.notify_waiters();
	let outputs = pending.await.unwrap();

	assert_eq!(*outputs.take(first_output), 10);
	assert_eq!(outputs.take(second_output).as_str(), "ok-done");
	assert_eq!(
		*PARALLEL_FIRST
			.run(&ctx, CaseInput::new(5, state.clone()))
			.await
			.unwrap(),
		10
	);
	assert_eq!(
		PARALLEL_SECOND
			.run(&ctx, CaseInput::new("ok", state.clone()))
			.await
			.unwrap()
			.as_str(),
		"ok-done"
	);
	assert_eq!(state.first_runs.load(Ordering::SeqCst), 1);
	assert_eq!(state.second_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn parallel_empty_batch_respects_parent_cancellation() {
	let tasks = tasks();
	let cancel = CancelToken::new();
	cancel.cancel();
	let ctx = tasks.exec_ctx(cancel);
	let batch: ParallelBatch<&'static str> = ParallelBatch::new();

	assert!(batch.run(&ctx).await.unwrap_err().is_cancelled());
}

#[tokio::test]
async fn parallel_returns_original_error_and_cancels_siblings() {
	let (started, _started_rx) = watch::channel(0usize);
	let state = Arc::new(ParallelErrorState {
		started,
		wait_runs: AtomicUsize::new(0),
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);
	let mut batch = ParallelBatch::new();
	batch.add(PARALLEL_WAIT, CaseInput::new((), state.clone()));
	batch.add(PARALLEL_FAIL, CaseInput::new((), state.clone()));

	let err = batch.run(&ctx).await.unwrap_err();

	assert!(matches!(err, Error::Failed(error) if *error == "boom"));
	assert_eq!(
		*PARALLEL_WAIT
			.run(&ctx, CaseInput::new((), state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.wait_runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn parallel_error_tie_break_is_completion_order_not_registration_order() {
	let (started, mut started_rx) = watch::channel(0usize);
	let state = Arc::new(ParallelCompetingErrorsState {
		started,
		started_count: AtomicUsize::new(0),
		release_added_first: Notify::new(),
		release_added_second: Notify::new(),
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);
	let mut batch = ParallelBatch::new();
	batch.add(PARALLEL_FAIL_ADDED_FIRST, CaseInput::new((), state.clone()));
	batch.add(
		PARALLEL_FAIL_ADDED_SECOND,
		CaseInput::new((), state.clone()),
	);
	let mut pending = Box::pin(batch.run(&ctx));

	tokio::select! {
		result = &mut pending => panic!("parallel run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}

	// Release the task added SECOND first, so its error completes before
	// the task added FIRST's error, even though it was registered later.
	state.release_added_second.notify_one();
	tokio::task::yield_now().await;
	tokio::task::yield_now().await;
	state.release_added_first.notify_one();

	let err = pending.await.unwrap_err();

	assert!(
		matches!(err, Error::Failed(error) if *error == "error-from-task-added-second"),
		"the error that completed first should win regardless of registration order"
	);
}

#[tokio::test]
async fn parallel_parent_cancellation_reaches_running_tasks() {
	let (started, mut started_rx) = watch::channel(0usize);
	let state = Arc::new(ParallelErrorState {
		started,
		wait_runs: AtomicUsize::new(0),
	});
	let tasks = tasks();
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut batch = ParallelBatch::new();
	batch.add(PARALLEL_WAIT, CaseInput::new((), state.clone()));
	let mut pending = Box::pin(batch.run(&ctx));

	tokio::select! {
		result = &mut pending => panic!("parallel run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	assert!(pending.await.unwrap_err().is_cancelled());
	assert_eq!(state.wait_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn equal_hashes_still_use_input_equality() {
	let state = count_state();
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(
		*COLLISION_TASK
			.run(&ctx, CaseInput::new(Collision(1), state.clone()))
			.await
			.unwrap(),
		1
	);
	assert_eq!(
		*COLLISION_TASK
			.run(&ctx, CaseInput::new(Collision(2), state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(
		*COLLISION_TASK
			.run(&ctx, CaseInput::new(Collision(1), state.clone()))
			.await
			.unwrap(),
		1
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn panicking_input_equality_does_not_poison_task_store() {
	let state = count_state();
	let panic_on_eq = Arc::new(AtomicBool::new(false));
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(
		*PANIC_EQ_TASK
			.run(
				&ctx,
				PanicEqInput::new(1, panic_on_eq.clone(), state.clone())
			)
			.await
			.unwrap(),
		1
	);
	panic_on_eq.store(true, Ordering::SeqCst);
	let second_ctx = ctx.clone();
	let second_panic_on_eq = panic_on_eq.clone();
	let second_state = state.clone();
	let second = tokio::spawn(async move {
		PANIC_EQ_TASK
			.run(
				&second_ctx,
				PanicEqInput::new(2, second_panic_on_eq, second_state),
			)
			.await
	});

	assert!(second.await.expect_err("equality should panic").is_panic());
	assert_eq!(
		*PANIC_EQ_TASK
			.run(&ctx, PanicEqInput::new(1, panic_on_eq, state.clone()))
			.await
			.unwrap(),
		1
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn different_tasks_with_same_input_do_not_share() {
	let state = count_state();
	let input = CaseInput::new(1, state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(
		*COUNTED_EXTENDED_60S.run(&ctx, input.clone()).await.unwrap(),
		2
	);
	assert_eq!(
		*SECOND_COUNTED_EXTENDED_60S
			.run(&ctx, input.clone())
			.await
			.unwrap(),
		2
	);
	assert_eq!(*COUNTED_EXTENDED_60S.run(&ctx, input).await.unwrap(), 2);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cycles_are_reported_instead_of_deadlocking() {
	let tasks = tasks();
	let err = SELF_CYCLE.run(&exec_ctx(&tasks), ()).await.unwrap_err();

	assert!(err.to_string().contains("task cycle detected"));
}

#[tokio::test]
async fn non_cancellation_errors_are_memoized_inside_one_exec_ctx() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(MEMOIZED_ERROR.run(&ctx, input.clone()).await.is_err());
	assert!(MEMOIZED_ERROR.run(&ctx, input).await.is_err());
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_does_not_retain_non_cancellation_errors() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();

	assert!(
		EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.is_err()
	);
	assert_eq!(
		*EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_shares_in_flight_non_cancellation_errors() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(GATED_EXTENDED_ERROR_THEN_SUCCESS.run(&first_ctx, input.clone()));
	let mut second = Box::pin(GATED_EXTENDED_ERROR_THEN_SUCCESS.run(&second_ctx, input.clone()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}
	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}

	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
	state.release.notify_one();

	let (first_result, second_result) = tokio::join!(first, second);

	assert!(first_result.is_err());
	assert!(second_result.is_err());
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
	assert_eq!(
		*GATED_EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_retains_success_after_non_cancellation_error_retry() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();

	assert!(
		EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.is_err()
	);
	assert_eq!(
		*EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap(),
		2
	);
	assert_eq!(
		*EXTENDED_ERROR_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_capacity_bypasses_new_entries_without_eviction() {
	let state = count_state();
	let (events, observer) = event_log();
	let tasks = tasks_with_cache_capacity_observer(1, observer);

	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(1, state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(2, state.clone()))
			.await
			.unwrap(),
		4
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(2, state.clone()))
			.await
			.unwrap(),
		4
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(1, state.clone()))
			.await
			.unwrap(),
		2
	);

	assert_eq!(state.runs.load(Ordering::SeqCst), 3);
	assert!(
		event_kinds(&events)
			.contains(&TaskEventKind::CrossExecCtxCacheCapacityBypass { max_entries: 1 })
	);
}

#[tokio::test]
async fn cross_exec_ctx_cancelled_runner_does_not_consume_cache_capacity() {
	let state = count_state();
	let tasks = tasks_with_cache_capacity(1);

	assert!(
		EXTENDED_CANCEL_THEN_SUCCESS
			.run(&exec_ctx(&tasks), CaseInput::new((), state.clone()))
			.await
			.unwrap_err()
			.is_cancelled()
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(10, state.clone()))
			.await
			.unwrap(),
		20
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(10, state.clone()))
			.await
			.unwrap(),
		20
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn same_exec_ctx_still_memoizes_non_cancellation_errors_with_cross_exec_ctx_cache() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(
		EXTENDED_ERROR_THEN_SUCCESS
			.run(&ctx, input.clone())
			.await
			.is_err()
	);
	assert!(EXTENDED_ERROR_THEN_SUCCESS.run(&ctx, input).await.is_err());
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cancellation_results_are_not_memoized_inside_one_exec_ctx() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(MEMOIZED_CANCEL.run(&ctx, input.clone()).await.is_err());
	assert!(MEMOIZED_CANCEL.run(&ctx, input).await.is_err());
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn waiting_on_local_work_respects_cancellation() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut first = Box::pin(GATED_MEMOIZED.run(&ctx, input.clone()));
	let second = Box::pin(GATED_MEMOIZED.run(&ctx, input));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	let err = second.await.unwrap_err();

	assert!(err.to_string().contains("cancelled"));
	state.release.notify_one();
	assert!(first.await.unwrap_err().is_cancelled());
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_coalesces_concurrent_work_across_contexts() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(GATED_EXTENDED_60S.run(&first_ctx, input.clone()));
	let mut second = Box::pin(GATED_EXTENDED_60S.run(&second_ctx, input));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}
	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}

	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
	state.release.notify_one();

	assert_eq!(*first.await.unwrap(), 1);
	assert_eq!(*second.await.unwrap(), 1);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cancelled_waiter_does_not_cancel_cross_exec_ctx_cache_work_for_other_waiters() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let first_cancel = CancelToken::new();
	let first_ctx = tasks.exec_ctx(first_cancel.clone());
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(GATED_EXTENDED_60S.run(&first_ctx, input.clone()));
	let second = Box::pin(GATED_EXTENDED_60S.run(&second_ctx, input.clone()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	first_cancel.cancel();
	assert!(first.await.unwrap_err().is_cancelled());

	state.release.notify_one();
	assert_eq!(*second.await.unwrap(), 1);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
	assert_eq!(
		*GATED_EXTENDED_60S
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		1
	);
}

#[tokio::test]
async fn cross_exec_ctx_runner_uses_independent_local_memoization_for_dependencies() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let caller_cancel = CancelToken::new();
	let caller_ctx = tasks.exec_ctx(caller_cancel.clone());
	let mut caller_dependency = Box::pin(GATED_MEMOIZED.run(&caller_ctx, input.clone()));

	tokio::select! {
		result = &mut caller_dependency => panic!("caller dependency finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	let mut caller_extended =
		Box::pin(EXTENDED_THROUGH_GATED_MEMOIZED.run(&caller_ctx, input.clone()));
	tokio::select! {
		result = &mut caller_extended => panic!("extended task finished unexpectedly: {result:?}"),
		result = time::timeout(Duration::from_secs(1), wait_for_started(&mut started_rx, 2)) => {
			result.expect("shared runner should start its own dependency");
		}
	}

	caller_cancel.cancel();
	assert!(caller_dependency.await.unwrap_err().is_cancelled());
	assert!(caller_extended.await.unwrap_err().is_cancelled());

	state.release.notify_waiters();
	assert_eq!(
		*EXTENDED_THROUGH_GATED_MEMOIZED
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_runner_respects_cancellation_set_before_success_return() {
	let cancel = CancelToken::new();
	let state = Arc::new(CancelOnSuccessState {
		runs: AtomicUsize::new(0),
		cancel: cancel.clone(),
	});
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();

	assert!(
		EXTENDED_CANCEL_CALLER_THEN_SUCCESS
			.run(&tasks.exec_ctx(cancel), input.clone())
			.await
			.unwrap_err()
			.is_cancelled()
	);
	assert_eq!(
		*EXTENDED_CANCEL_CALLER_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		1
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_does_not_retain_cancellation_results() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();

	assert!(
		EXTENDED_CANCEL_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input.clone())
			.await
			.unwrap_err()
			.is_cancelled()
	);
	assert_eq!(
		*EXTENDED_CANCEL_THEN_SUCCESS
			.run(&exec_ctx(&tasks), input)
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn panicking_runner_reopens_slot_for_retry() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let first = tokio::spawn(async move { PANIC_THEN_SUCCESS.run(&first_ctx, input).await });

	let panic = first.await.expect_err("first run should panic");
	assert!(panic.is_panic());

	assert_eq!(
		*PANIC_THEN_SUCCESS
			.run(&exec_ctx(&tasks), CaseInput::new((), state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_panicking_runner_reopens_slot_for_retry_and_propagates_to_runner() {
	let state = count_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let first =
		tokio::spawn(async move { EXTENDED_PANIC_THEN_SUCCESS.run(&first_ctx, input).await });

	let panic = first.await.expect_err("first run should panic");
	assert!(panic.is_panic());

	assert_eq!(
		*EXTENDED_PANIC_THEN_SUCCESS
			.run(&exec_ctx(&tasks), CaseInput::new((), state.clone()))
			.await
			.unwrap(),
		2
	);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_panicking_runner_does_not_consume_cache_capacity() {
	let panic_state = count_state();
	let tasks = tasks_with_cache_capacity(1);
	let first_ctx = exec_ctx(&tasks);
	let first = tokio::spawn(async move {
		EXTENDED_PANIC_THEN_SUCCESS
			.run(&first_ctx, CaseInput::new((), panic_state.clone()))
			.await
	});

	let panic = first.await.expect_err("first run should panic");
	assert!(panic.is_panic());
	let cache_state = count_state();
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(10, cache_state.clone()))
			.await
			.unwrap(),
		20
	);
	assert_eq!(
		*COUNTED_EXTENDED_60S
			.run(&exec_ctx(&tasks), CaseInput::new(10, cache_state.clone()))
			.await
			.unwrap(),
		20
	);
	assert_eq!(cache_state.runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn single_flight_coalesces_concurrent_calls_without_memoizing_completed_results() {
	let (state, mut started_rx) = gated_state();
	let input = CaseInput::new((), state.clone());
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);
	let mut first = Box::pin(SINGLE_FLIGHT_GATED.run(&ctx, input.clone()));
	let mut second = Box::pin(SINGLE_FLIGHT_GATED.run(&ctx, input.clone()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}
	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}

	state.release.notify_one();
	assert_eq!(*first.await.unwrap(), 1);
	assert_eq!(*second.await.unwrap(), 1);
	assert_eq!(state.runs.load(Ordering::SeqCst), 1);

	let mut third = Box::pin(SINGLE_FLIGHT_GATED.run(&ctx, input));
	tokio::select! {
		result = &mut third => panic!("third run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}
	state.release.notify_one();
	assert_eq!(*third.await.unwrap(), 2);
	assert_eq!(state.runs.load(Ordering::SeqCst), 2);
}

async fn wait_for_started(rx: &mut watch::Receiver<usize>, expected: usize) {
	while *rx.borrow_and_update() < expected {
		rx.changed().await.expect("started sender dropped");
	}
}
