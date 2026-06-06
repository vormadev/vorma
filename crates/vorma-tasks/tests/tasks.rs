use std::hash::{Hash, Hasher};
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use tokio::sync::{Notify, watch};
use vorma_tasks::{
	CancelToken, Clock, ClockInstant, Error, Task, TaskEvent, TaskEventKind, TaskEventOutcome,
	TaskOverrideMode, TaskOverrides, TaskRunSource, Tasks, TasksOptions,
};

#[derive(Clone, Default)]
struct ManualClock {
	nanos: Arc<AtomicU64>,
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

fn counted_task(
	runs: Arc<AtomicUsize>,
	cross_exec_ctx_cache_ttl: Duration,
) -> Task<u32, u32, &'static str> {
	Task::new(cross_exec_ctx_cache_ttl, move |_ctx, input| {
		let runs = runs.clone();
		async move {
			runs.fetch_add(1, Ordering::SeqCst);
			Ok(input * 2)
		}
	})
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
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> = Task::new(Duration::ZERO, move |_ctx, ()| {
		let runs = task_runs.clone();
		async move {
			runs.fetch_add(1, Ordering::SeqCst);
			Ok(1)
		}
	});
	let overrides = TaskOverrides::new(TaskOverrideMode::RequireOverride);
	let tasks = tasks_with_overrides(overrides);
	let err = task.run(&exec_ctx(&tasks), ()).await.unwrap_err();

	assert!(matches!(err, Error::MissingOverride { .. }));
	assert_eq!(runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn run_unmatched_overrides_allow_original_task_bodies() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task = counted_task(runs.clone(), Duration::ZERO);
	let overrides = TaskOverrides::new(TaskOverrideMode::RunUnmatched);
	let tasks = tasks_with_overrides(overrides);

	assert_eq!(*task.run(&exec_ctx(&tasks), 4).await.unwrap(), 8);
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_overrides_replace_task_bodies_and_memoize_inside_exec_ctx() {
	let body_runs = Arc::new(AtomicUsize::new(0));
	let override_runs = Arc::new(AtomicUsize::new(0));
	let task_body_runs = body_runs.clone();
	let task: Task<u32, u32, &'static str> = Task::new(Duration::ZERO, move |_ctx, input| {
		let body_runs = task_body_runs.clone();
		async move {
			body_runs.fetch_add(1, Ordering::SeqCst);
			Ok(input)
		}
	});
	let task_override_runs = override_runs.clone();
	let overrides =
		TaskOverrides::new(TaskOverrideMode::RequireOverride).replace(&task, move |_ctx, input| {
			let override_runs = task_override_runs.clone();
			async move {
				override_runs.fetch_add(1, Ordering::SeqCst);
				Ok(input * 10)
			}
		});
	let tasks = tasks_with_overrides(overrides);
	let ctx = exec_ctx(&tasks);

	let (a, b) = tokio::join!(task.run(&ctx, 3), task.run(&ctx, 3));

	assert_eq!(*a.unwrap(), 30);
	assert_eq!(*b.unwrap(), 30);
	assert_eq!(body_runs.load(Ordering::SeqCst), 0);
	assert_eq!(override_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn task_overrides_participate_in_cross_exec_ctx_cache() {
	let body_runs = Arc::new(AtomicUsize::new(0));
	let override_runs = Arc::new(AtomicUsize::new(0));
	let task_body_runs = body_runs.clone();
	let task: Task<u32, u32, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, input| {
			let body_runs = task_body_runs.clone();
			async move {
				body_runs.fetch_add(1, Ordering::SeqCst);
				Ok(input)
			}
		});
	let task_override_runs = override_runs.clone();
	let overrides =
		TaskOverrides::new(TaskOverrideMode::RequireOverride).replace(&task, move |_ctx, input| {
			let override_runs = task_override_runs.clone();
			async move {
				override_runs.fetch_add(1, Ordering::SeqCst);
				Ok(input * 10)
			}
		});
	let tasks = tasks_with_overrides(overrides);

	assert_eq!(*task.run(&exec_ctx(&tasks), 3).await.unwrap(), 30);
	assert_eq!(*task.run(&exec_ctx(&tasks), 3).await.unwrap(), 30);
	assert_eq!(body_runs.load(Ordering::SeqCst), 0);
	assert_eq!(override_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_exec_ctx_memo_and_run_events() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task = counted_task(runs.clone(), Duration::ZERO);
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let ctx = exec_ctx(&tasks);

	assert_eq!(*task.run(&ctx, 5).await.unwrap(), 10);
	assert_eq!(*task.run(&ctx, 5).await.unwrap(), 10);

	{
		let events = events.lock().expect("task event log lock poisoned");
		assert!(events.iter().all(|event| event.task_id == task.id()));
		assert!(
			events
				.iter()
				.all(|event| event.task_input_type == std::any::type_name::<u32>())
		);
	}

	let kinds = event_kinds(&events);
	assert_eq!(
		kinds,
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
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_cross_exec_ctx_cache_and_duration_events() {
	let runs = Arc::new(AtomicUsize::new(0));
	let clock = ManualClock::default();
	let task_clock = clock.clone();
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, ()| {
			let runs = task_runs.clone();
			let clock = task_clock.clone();
			async move {
				runs.fetch_add(1, Ordering::SeqCst);
				clock.advance(Duration::from_millis(7));
				Ok(1)
			}
		});
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(clock.clone(), observer);

	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 1);
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 1);

	let kinds = event_kinds(&events);
	assert_eq!(
		kinds,
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
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_cross_exec_ctx_in_flight_waits() {
	let runs = Arc::new(AtomicUsize::new(0));
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task = gated_task(
		runs.clone(),
		started,
		release.clone(),
		Duration::from_secs(60),
	);
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(task.run(&first_ctx, ()));
	let mut second = Box::pin(task.run(&second_ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}
	release.notify_one();

	assert_eq!(*first.await.unwrap(), 1);
	assert_eq!(*second.await.unwrap(), 1);

	let kinds = event_kinds(&events);
	assert!(kinds.contains(&TaskEventKind::CrossExecCtxInFlightWait));
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn observer_reports_stale_cross_exec_ctx_slot_cleanup() {
	let runs = Arc::new(AtomicUsize::new(0));
	let clock = ManualClock::default();
	let task = counted_task(runs.clone(), Duration::from_millis(10));
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(clock.clone(), observer);

	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	clock.advance(Duration::from_millis(10));
	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);

	let kinds = event_kinds(&events);
	assert!(kinds.contains(&TaskEventKind::CrossExecCtxStaleSlotRemoved { count: 1 }));
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn observer_reports_cancellation_events() {
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task = gated_task(
		Arc::new(AtomicUsize::new(0)),
		started,
		release,
		Duration::ZERO,
	);
	let (events, observer) = event_log();
	let tasks = tasks_with_clock_observer(ManualClock::default(), observer);
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut pending = Box::pin(task.run(&ctx, ()));

	tokio::select! {
		result = &mut pending => panic!("task finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	assert!(pending.await.unwrap_err().is_cancelled());

	let kinds = event_kinds(&events);
	assert!(kinds.contains(&TaskEventKind::Cancelled));
}

#[tokio::test]
async fn task_runs_once_per_exec_ctx() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task = counted_task(runs.clone(), Duration::ZERO);
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	let (a, b) = tokio::join!(task.run(&ctx, 7), task.run(&ctx, 7));

	assert_eq!(*a.unwrap(), 14);
	assert_eq!(*b.unwrap(), 14);
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn separate_exec_ctx_values_do_not_share_without_cross_exec_ctx_cache() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task = counted_task(runs.clone(), Duration::ZERO);
	let tasks = tasks();

	assert_eq!(*task.run(&exec_ctx(&tasks), 1).await.unwrap(), 2);
	assert_eq!(*task.run(&exec_ctx(&tasks), 1).await.unwrap(), 2);

	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_reuses_completed_values_until_expiration() {
	let runs = Arc::new(AtomicUsize::new(0));
	let clock = ManualClock::default();
	let task = counted_task(runs.clone(), Duration::from_millis(10));
	let tasks = tasks_with_clock(clock.clone());

	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	clock.advance(Duration::from_millis(9));
	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	assert_eq!(runs.load(Ordering::SeqCst), 1);

	clock.advance(Duration::from_millis(1));
	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_ttl_starts_after_task_completion() {
	let runs = Arc::new(AtomicUsize::new(0));
	let clock = ManualClock::default();
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task = gated_task(
		runs.clone(),
		started,
		release.clone(),
		Duration::from_millis(50),
	);
	let tasks = tasks_with_clock(clock.clone());
	let ctx = exec_ctx(&tasks);
	let mut first = Box::pin(task.run(&ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	clock.advance(Duration::from_millis(60));
	release.notify_one();
	assert_eq!(*first.await.unwrap(), 1);

	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 1);
	assert_eq!(runs.load(Ordering::SeqCst), 1);

	clock.advance(Duration::from_millis(50));
	let third_ctx = exec_ctx(&tasks);
	let mut third = Box::pin(task.run(&third_ctx, ()));

	tokio::select! {
		result = &mut third => panic!("third run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}

	release.notify_one();
	assert_eq!(*third.await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn task_composition_shares_one_exec_ctx() {
	let runs = Arc::new(AtomicUsize::new(0));
	let child = counted_task(runs.clone(), Duration::ZERO);
	let parent: Task<u32, u32, &'static str> = Task::new(Duration::ZERO, move |ctx, input| {
		let child = child.clone();
		async move {
			let (left, right) = tokio::try_join!(child.run(&ctx, input), child.run(&ctx, input))?;
			Ok(*left + *right)
		}
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(*parent.run(&ctx, 3).await.unwrap(), 12);
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn run_parallel_runs_bound_tasks_concurrently_and_stores_results() {
	let (started, mut started_rx) = watch::channel(0usize);
	let started_count = Arc::new(AtomicUsize::new(0));
	let release = Arc::new(Notify::new());
	let first_runs = Arc::new(AtomicUsize::new(0));
	let second_runs = Arc::new(AtomicUsize::new(0));

	let first: Task<u32, u32, &'static str> = {
		let started = started.clone();
		let started_count = started_count.clone();
		let release = release.clone();
		let first_runs = first_runs.clone();
		Task::new(Duration::ZERO, move |_ctx, input| {
			let started = started.clone();
			let started_count = started_count.clone();
			let release = release.clone();
			let first_runs = first_runs.clone();
			async move {
				first_runs.fetch_add(1, Ordering::SeqCst);
				let count = started_count.fetch_add(1, Ordering::SeqCst) + 1;
				let _ = started.send(count);
				release.notified().await;
				Ok(input * 2)
			}
		})
	};
	let second: Task<&'static str, String, &'static str> = {
		let started = started.clone();
		let started_count = started_count.clone();
		let release = release.clone();
		let second_runs = second_runs.clone();
		Task::new(Duration::ZERO, move |_ctx, input| {
			let started = started.clone();
			let started_count = started_count.clone();
			let release = release.clone();
			let second_runs = second_runs.clone();
			async move {
				second_runs.fetch_add(1, Ordering::SeqCst);
				let count = started_count.fetch_add(1, Ordering::SeqCst) + 1;
				let _ = started.send(count);
				release.notified().await;
				Ok(format!("{input}-done"))
			}
		})
	};

	let first_output = Arc::new(Mutex::new(None));
	let second_output = Arc::new(Mutex::new(None));
	let first_output_sink = first_output.clone();
	let second_output_sink = second_output.clone();
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);
	let mut pending = Box::pin(ctx.run_parallel(vec![
		first.bind_input_with_result(5, move |output| {
			*first_output_sink
				.lock()
				.expect("first output lock poisoned") = Some(output);
		}),
		second.bind_input_with_result("ok", move |output| {
			*second_output_sink
				.lock()
				.expect("second output lock poisoned") = Some(output);
		}),
	]));

	tokio::select! {
		result = &mut pending => panic!("parallel run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 2) => {}
	}

	release.notify_waiters();
	pending.await.unwrap();

	assert_eq!(
		**first_output
			.lock()
			.expect("first output lock poisoned")
			.as_ref()
			.expect("first output set"),
		10
	);
	assert_eq!(
		second_output
			.lock()
			.expect("second output lock poisoned")
			.as_ref()
			.expect("second output set")
			.as_str(),
		"ok-done"
	);
	assert_eq!(*first.run(&ctx, 5).await.unwrap(), 10);
	assert_eq!(second.run(&ctx, "ok").await.unwrap().as_str(), "ok-done");
	assert_eq!(first_runs.load(Ordering::SeqCst), 1);
	assert_eq!(second_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn run_parallel_returns_original_error_and_cancels_siblings() {
	let (started, started_rx) = watch::channel(0usize);
	let wait_runs = Arc::new(AtomicUsize::new(0));
	let wait_task: Task<(), usize, &'static str> = {
		let started = started.clone();
		let wait_runs = wait_runs.clone();
		Task::new(Duration::ZERO, move |ctx, ()| {
			let started = started.clone();
			let wait_runs = wait_runs.clone();
			async move {
				let run = wait_runs.fetch_add(1, Ordering::SeqCst) + 1;
				let _ = started.send(run);
				if run == 1 {
					ctx.cancel_token().cancelled().await;
				}
				Ok(run)
			}
		})
	};
	let fail_task: Task<(), (), &'static str> = {
		let started_rx = started_rx.clone();
		Task::new(Duration::ZERO, move |_ctx, ()| {
			let mut started_rx = started_rx.clone();
			async move {
				wait_for_started(&mut started_rx, 1).await;
				Err("boom".into())
			}
		})
	};
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	let err = ctx
		.run_parallel(vec![wait_task.bind_input(()), fail_task.bind_input(())])
		.await
		.unwrap_err();

	assert!(matches!(err, Error::Failed(error) if *error == "boom"));
	assert_eq!(*wait_task.run(&ctx, ()).await.unwrap(), 2);
	assert_eq!(wait_runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn run_parallel_parent_cancellation_reaches_running_tasks() {
	let (started, mut started_rx) = watch::channel(0usize);
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let wait_task: Task<(), (), &'static str> = Task::new(Duration::ZERO, move |ctx, ()| {
		let started = started.clone();
		let runs = task_runs.clone();
		async move {
			let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = started.send(run);
			ctx.cancel_token().cancelled().await;
			Err(Error::Cancelled)
		}
	});
	let tasks = tasks();
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut pending = Box::pin(ctx.run_parallel(vec![wait_task.bind_input(())]));

	tokio::select! {
		result = &mut pending => panic!("parallel run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	assert!(pending.await.unwrap_err().is_cancelled());
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct Collision(u8);

impl Hash for Collision {
	fn hash<H: Hasher>(&self, state: &mut H) {
		1u8.hash(state);
	}
}

#[tokio::test]
async fn equal_hashes_still_use_input_equality() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<Collision, u8, &'static str> =
		Task::new(Duration::ZERO, move |_ctx, input: Collision| {
			let runs = task_runs.clone();
			async move {
				runs.fetch_add(1, Ordering::SeqCst);
				Ok(input.0)
			}
		});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(*task.run(&ctx, Collision(1)).await.unwrap(), 1);
	assert_eq!(*task.run(&ctx, Collision(2)).await.unwrap(), 2);
	assert_eq!(*task.run(&ctx, Collision(1)).await.unwrap(), 1);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn different_tasks_with_same_input_do_not_share() {
	let runs = Arc::new(AtomicUsize::new(0));
	let first = counted_task(runs.clone(), Duration::from_secs(60));
	let second = counted_task(runs.clone(), Duration::from_secs(60));
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert_eq!(*first.run(&ctx, 1).await.unwrap(), 2);
	assert_eq!(*second.run(&ctx, 1).await.unwrap(), 2);
	assert_eq!(*first.run(&ctx, 1).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cycles_are_reported_instead_of_deadlocking() {
	type RecursiveTask = Task<(), (), &'static str>;

	let task: Arc<std::sync::Mutex<Option<RecursiveTask>>> = Arc::new(std::sync::Mutex::new(None));
	let task_ref = task.clone();
	let self_cycle: RecursiveTask = Task::new(Duration::ZERO, move |ctx, ()| {
		let task_ref = task_ref.clone();
		async move {
			let task = task_ref
				.lock()
				.expect("task lock poisoned")
				.as_ref()
				.expect("task initialized")
				.clone();
			task.run(&ctx, ()).await?;
			Ok(())
		}
	});
	*task.lock().expect("task lock poisoned") = Some(self_cycle.clone());

	let tasks = tasks();
	let err = self_cycle.run(&exec_ctx(&tasks), ()).await.unwrap_err();

	assert!(err.to_string().contains("task cycle detected"));
}

#[tokio::test]
async fn non_cancellation_errors_are_memoized_inside_one_exec_ctx() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), (), &'static str> = Task::new(Duration::ZERO, move |_ctx, ()| {
		let runs = task_runs.clone();
		async move {
			runs.fetch_add(1, Ordering::SeqCst);
			Err("boom".into())
		}
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(task.run(&ctx, ()).await.is_err());
	assert!(task.run(&ctx, ()).await.is_err());
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_does_not_retain_non_cancellation_errors() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, ()| {
			let runs = task_runs.clone();
			async move {
				let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
				if run == 1 {
					return Err("boom".into());
				}
				Ok(run)
			}
		});
	let tasks = tasks();

	assert!(task.run(&exec_ctx(&tasks), ()).await.is_err());
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_shares_in_flight_non_cancellation_errors() {
	let runs = Arc::new(AtomicUsize::new(0));
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task_runs = runs.clone();
	let task_release = release.clone();
	let task: Task<(), usize, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, ()| {
			let runs = task_runs.clone();
			let started = started.clone();
			let release = task_release.clone();
			async move {
				let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
				let _ = started.send(run);
				if run == 1 {
					release.notified().await;
					return Err("boom".into());
				}
				Ok(run)
			}
		});
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(task.run(&first_ctx, ()));
	let mut second = Box::pin(task.run(&second_ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}
	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}

	assert_eq!(runs.load(Ordering::SeqCst), 1);
	release.notify_one();

	let (first_result, second_result) = tokio::join!(first, second);

	assert!(first_result.is_err());
	assert!(second_result.is_err());
	assert_eq!(runs.load(Ordering::SeqCst), 1);
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_retains_success_after_non_cancellation_error_retry() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, ()| {
			let runs = task_runs.clone();
			async move {
				let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
				if run == 1 {
					return Err("boom".into());
				}
				Ok(run)
			}
		});
	let tasks = tasks();

	assert!(task.run(&exec_ctx(&tasks), ()).await.is_err());
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn cross_exec_ctx_cache_capacity_bypasses_new_entries_without_eviction() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task = counted_task(runs.clone(), Duration::from_secs(60));
	let (events, observer) = event_log();
	let tasks = tasks_with_cache_capacity_observer(1, observer);

	assert_eq!(*task.run(&exec_ctx(&tasks), 1).await.unwrap(), 2);
	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	assert_eq!(*task.run(&exec_ctx(&tasks), 2).await.unwrap(), 4);
	assert_eq!(*task.run(&exec_ctx(&tasks), 1).await.unwrap(), 2);

	assert_eq!(runs.load(Ordering::SeqCst), 3);
	assert!(
		event_kinds(&events)
			.contains(&TaskEventKind::CrossExecCtxCacheCapacityBypass { max_entries: 1 })
	);
}

#[tokio::test]
async fn same_exec_ctx_still_memoizes_non_cancellation_errors_with_cross_exec_ctx_cache() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), (), &'static str> = Task::new(Duration::from_secs(60), move |_ctx, ()| {
		let runs = task_runs.clone();
		async move {
			runs.fetch_add(1, Ordering::SeqCst);
			Err("boom".into())
		}
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(task.run(&ctx, ()).await.is_err());
	assert!(task.run(&ctx, ()).await.is_err());
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cancellation_results_are_not_memoized_inside_one_exec_ctx() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), (), &'static str> = Task::new(Duration::ZERO, move |_ctx, ()| {
		let runs = task_runs.clone();
		async move {
			runs.fetch_add(1, Ordering::SeqCst);
			Err(Error::Cancelled)
		}
	});
	let tasks = tasks();
	let ctx = exec_ctx(&tasks);

	assert!(task.run(&ctx, ()).await.is_err());
	assert!(task.run(&ctx, ()).await.is_err());
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn waiting_on_local_work_respects_cancellation() {
	let runs = Arc::new(AtomicUsize::new(0));
	let release = Arc::new(Notify::new());
	let (started, mut started_rx) = watch::channel(0usize);
	let task_runs = runs.clone();
	let task_release = release.clone();
	let task: Task<(), usize, &'static str> = Task::new(Duration::ZERO, move |_ctx, ()| {
		let runs = task_runs.clone();
		let started = started.clone();
		let release = task_release.clone();
		async move {
			let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = started.send(run);
			release.notified().await;
			Ok(run)
		}
	});
	let tasks = tasks();
	let cancel = CancelToken::new();
	let ctx = tasks.exec_ctx(cancel.clone());
	let mut first = Box::pin(task.run(&ctx, ()));
	let second = Box::pin(task.run(&ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	cancel.cancel();
	let err = second.await.unwrap_err();

	assert!(err.to_string().contains("cancelled"));
	release.notify_one();
	assert!(first.await.unwrap_err().is_cancelled());
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_coalesces_concurrent_work_across_contexts() {
	let runs = Arc::new(AtomicUsize::new(0));
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task = gated_task(
		runs.clone(),
		started,
		release.clone(),
		Duration::from_secs(60),
	);
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(task.run(&first_ctx, ()));
	let mut second = Box::pin(task.run(&second_ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}
	tokio::select! {
		result = &mut second => panic!("second run finished unexpectedly: {result:?}"),
		_ = tokio::task::yield_now() => {}
	}

	assert_eq!(runs.load(Ordering::SeqCst), 1);
	release.notify_one();

	assert_eq!(*first.await.unwrap(), 1);
	assert_eq!(*second.await.unwrap(), 1);
	assert_eq!(runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn cancelled_waiter_does_not_cancel_cross_exec_ctx_cache_work_for_other_waiters() {
	let runs = Arc::new(AtomicUsize::new(0));
	let (started, mut started_rx) = watch::channel(0usize);
	let release = Arc::new(Notify::new());
	let task = gated_task(
		runs.clone(),
		started,
		release.clone(),
		Duration::from_secs(60),
	);
	let tasks = tasks();
	let first_cancel = CancelToken::new();
	let first_ctx = tasks.exec_ctx(first_cancel.clone());
	let second_ctx = exec_ctx(&tasks);
	let mut first = Box::pin(task.run(&first_ctx, ()));
	let second = Box::pin(task.run(&second_ctx, ()));

	tokio::select! {
		result = &mut first => panic!("first run finished unexpectedly: {result:?}"),
		_ = wait_for_started(&mut started_rx, 1) => {}
	}

	first_cancel.cancel();
	assert!(first.await.unwrap_err().is_cancelled());

	release.notify_one();
	assert_eq!(*second.await.unwrap(), 1);
	assert_eq!(runs.load(Ordering::SeqCst), 1);

	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 1);
}

#[tokio::test]
async fn cross_exec_ctx_cache_does_not_retain_cancellation_results() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> =
		Task::new(Duration::from_secs(60), move |_ctx, ()| {
			let runs = task_runs.clone();
			async move {
				let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
				if run == 1 {
					return Err(Error::Cancelled);
				}
				Ok(run)
			}
		});
	let tasks = tasks();

	assert!(
		task.run(&exec_ctx(&tasks), ())
			.await
			.unwrap_err()
			.is_cancelled()
	);
	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

#[tokio::test]
async fn panicking_runner_reopens_slot_for_retry() {
	let runs = Arc::new(AtomicUsize::new(0));
	let task_runs = runs.clone();
	let task: Task<(), usize, &'static str> = Task::new(Duration::ZERO, move |_ctx, ()| {
		let runs = task_runs.clone();
		async move {
			let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
			if run == 1 {
				panic!("intentional test panic");
			}
			Ok(run)
		}
	});
	let tasks = tasks();
	let first_ctx = exec_ctx(&tasks);
	let task_for_spawn = task.clone();
	let first = tokio::spawn(async move { task_for_spawn.run(&first_ctx, ()).await });

	let panic = first.await.expect_err("first run should panic");
	assert!(panic.is_panic());

	assert_eq!(*task.run(&exec_ctx(&tasks), ()).await.unwrap(), 2);
	assert_eq!(runs.load(Ordering::SeqCst), 2);
}

fn gated_task(
	runs: Arc<AtomicUsize>,
	started: watch::Sender<usize>,
	release: Arc<Notify>,
	cross_exec_ctx_cache_ttl: Duration,
) -> Task<(), usize, &'static str> {
	Task::new(cross_exec_ctx_cache_ttl, move |_ctx, ()| {
		let runs = runs.clone();
		let started = started.clone();
		let release = release.clone();
		async move {
			let run = runs.fetch_add(1, Ordering::SeqCst) + 1;
			let _ = started.send(run);
			release.notified().await;
			Ok(run)
		}
	})
}

async fn wait_for_started(rx: &mut watch::Receiver<usize>, expected: usize) {
	while *rx.borrow_and_update() < expected {
		rx.changed().await.expect("started sender dropped");
	}
}
