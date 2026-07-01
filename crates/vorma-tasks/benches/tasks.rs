//! Ports of the Go kit/tasks benchmark suite.
//!
//! Each benchmark mirrors its Go counterpart's shape: fresh execution
//! contexts model isolated execution scopes, the repeated-calls case
//! pins the memo-hit hot path, and the scaling family stresses parallel
//! fan-out across distinct task nodes.
//! Two deliberate deviations from the Go originals: the high-contention
//! task spin-waits one microsecond because the tokio timer cannot sleep
//! that precisely, and the cancellation case cancels via a spawned task
//! rather than a timer for the same reason.

use std::hint::black_box;
use std::time::{Duration, Instant};

// This crate does not link the vorma server crate, so the bench
// binary declares the allocator vorma apps run by default.
#[global_allocator]
static GLOBAL_ALLOCATOR: mimalloc::MiMalloc = mimalloc::MiMalloc;

use vorma_bench::Bench;
use vorma_tasks::{CancelToken, ExecCtx, ParallelBatch, Tasks, TasksOptions};

type BenchError = Box<dyn std::error::Error + Send + Sync>;

vorma_tasks::task! {
	static DOUBLE_TASK: Task<i64, i64, BenchError> =
		memoized(|_, input: i64| async move { Ok(input * 2) });
}

vorma_tasks::task! {
	static TRIPLE_TASK: Task<i64, i64, BenchError> =
		memoized(|_, input: i64| async move { Ok(input * 3) });
}

vorma_tasks::task! {
	static QUADRUPLE_TASK: Task<i64, i64, BenchError> =
		memoized(|_, input: i64| async move { Ok(input * 4) });
}

vorma_tasks::task! {
	static SPIN_TASK: Task<(), String, BenchError> =
		memoized(|_, _input: ()| async move {
			spin_one_microsecond();
			Ok("result".to_owned())
		});
}

vorma_tasks::task! {
	static DEPENDENT_TASK: Task<i64, i64, BenchError> =
		memoized(|ctx, input: i64| async move {
			let value = DOUBLE_TASK.run(&ctx, input).await?;
			Ok(*value + 10)
		});
}

vorma_tasks::task! {
	static ALLOCATION_TASK: Task<String, String, BenchError> =
		memoized(|_, input: String| async move { Ok(format!("Hello, {input}")) });
}

vorma_tasks::task! {
	static SLEEP_TASK: Task<i64, i64, BenchError> =
		memoized(|_, input: i64| async move {
			tokio::time::sleep(Duration::from_millis(10)).await;
			Ok(input * 2)
		});
}

macro_rules! declare_scaling_tasks {
	($(($task:ident, $offset:expr)),+ $(,)?) => {
		$(
			vorma_tasks::task! {
				static $task: Task<i64, i64, BenchError> =
					memoized(|_, input: i64| async move { Ok(input + $offset) });
			}
		)+

		const SCALING_TASKS: &[vorma_tasks::Task<i64, i64, BenchError>] = &[$($task),+];
	};
}

declare_scaling_tasks!(
	(SCALING_TASK_00, 0),
	(SCALING_TASK_01, 1),
	(SCALING_TASK_02, 2),
	(SCALING_TASK_03, 3),
	(SCALING_TASK_04, 4),
	(SCALING_TASK_05, 5),
	(SCALING_TASK_06, 6),
	(SCALING_TASK_07, 7),
	(SCALING_TASK_08, 8),
	(SCALING_TASK_09, 9),
	(SCALING_TASK_10, 10),
	(SCALING_TASK_11, 11),
	(SCALING_TASK_12, 12),
	(SCALING_TASK_13, 13),
	(SCALING_TASK_14, 14),
	(SCALING_TASK_15, 15),
	(SCALING_TASK_16, 16),
	(SCALING_TASK_17, 17),
	(SCALING_TASK_18, 18),
	(SCALING_TASK_19, 19),
	(SCALING_TASK_20, 20),
	(SCALING_TASK_21, 21),
	(SCALING_TASK_22, 22),
	(SCALING_TASK_23, 23),
	(SCALING_TASK_24, 24),
	(SCALING_TASK_25, 25),
	(SCALING_TASK_26, 26),
	(SCALING_TASK_27, 27),
	(SCALING_TASK_28, 28),
	(SCALING_TASK_29, 29),
	(SCALING_TASK_30, 30),
	(SCALING_TASK_31, 31),
	(SCALING_TASK_32, 32),
	(SCALING_TASK_33, 33),
	(SCALING_TASK_34, 34),
	(SCALING_TASK_35, 35),
	(SCALING_TASK_36, 36),
	(SCALING_TASK_37, 37),
	(SCALING_TASK_38, 38),
	(SCALING_TASK_39, 39),
	(SCALING_TASK_40, 40),
	(SCALING_TASK_41, 41),
	(SCALING_TASK_42, 42),
	(SCALING_TASK_43, 43),
	(SCALING_TASK_44, 44),
	(SCALING_TASK_45, 45),
	(SCALING_TASK_46, 46),
	(SCALING_TASK_47, 47),
	(SCALING_TASK_48, 48),
	(SCALING_TASK_49, 49),
);

fn runtime() -> tokio::runtime::Runtime {
	tokio::runtime::Builder::new_multi_thread()
		.enable_all()
		.build()
		.expect("benchmark runtime builds")
}

fn tasks_runtime() -> Tasks<BenchError> {
	Tasks::new(TasksOptions::default())
}

fn exec_ctx(tasks: &Tasks<BenchError>) -> ExecCtx<BenchError> {
	tasks.exec_ctx(CancelToken::new())
}

fn spin_one_microsecond() {
	let start = Instant::now();
	while start.elapsed() < Duration::from_micros(1) {
		std::hint::spin_loop();
	}
}

fn main() {
	let mut bench = Bench::new(env!("CARGO_PKG_NAME"));
	let rt = runtime();

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		let mut i = 0i64;
		bench.bench_async("single_task", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				black_box(DOUBLE_TASK.run(&ctx, input).await.expect("task succeeds"));
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		let mut i = 0i64;
		bench.bench_async("parallel_independent_tasks", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				let mut batch = ParallelBatch::new();
				let double = batch.add(DOUBLE_TASK, input);
				let triple = batch.add(TRIPLE_TASK, input);
				let quadruple = batch.add(QUADRUPLE_TASK, input);
				let outputs = batch.run(&ctx).await.expect("parallel tasks succeed");
				black_box(*outputs.take(double));
				black_box(*outputs.take(triple));
				black_box(*outputs.take(quadruple));
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		bench.bench_async("high_contention", &rt, move || async move {
			let ctx = exec_ctx(tasks);
			let mut joined = tokio::task::JoinSet::new();
			for _ in 0..10 {
				let ctx = ctx.clone();
				joined.spawn(async move {
					SPIN_TASK.run(&ctx, ()).await.expect("shared task succeeds");
				});
			}
			while let Some(result) = joined.join_next().await {
				result.expect("no panics");
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		let mut i = 0i64;
		bench.bench_async("task_with_dependencies", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				black_box(
					DEPENDENT_TASK
						.run(&ctx, input)
						.await
						.expect("dependent task succeeds"),
				);
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		bench.bench_async("allocations", &rt, move || async move {
			let ctx = exec_ctx(tasks);
			black_box(
				ALLOCATION_TASK
					.run(&ctx, "World".to_owned())
					.await
					.expect("task succeeds"),
			);
		});
	}

	for count in [1usize, 2, 5, 10, 20, 50] {
		let tasks = tasks_runtime();
		let tasks_ref = &tasks;
		let mut i = 0i64;
		bench.bench_async(&format!("parallel_scaling/tasks-{count}"), &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks_ref);
				let mut batch = ParallelBatch::new();
				let outputs = SCALING_TASKS
					.iter()
					.take(count)
					.map(|task| batch.add(*task, input))
					.collect::<Vec<_>>();
				let batch_outputs = batch.run(&ctx).await.expect("parallel tasks succeed");
				for output in outputs {
					black_box(*batch_outputs.take(output));
				}
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let tasks = &tasks;
		let mut i = 0i64;
		bench.bench_async("context_cancellation", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let cancel = CancelToken::new();
				let ctx = tasks.exec_ctx(cancel.clone());
				let canceller = tokio::spawn(async move {
					cancel.cancel();
				});
				let _ = black_box(SLEEP_TASK.run(&ctx, input).await);
				canceller.await.expect("canceller completes");
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let ctx = rt.block_on(async {
			let ctx = exec_ctx(&tasks);
			DOUBLE_TASK
				.run(&ctx, 42)
				.await
				.expect("warm-up run succeeds");
			ctx
		});
		let ctx = &ctx;
		bench.bench_async("repeated_task_calls", &rt, move || async move {
			black_box(DOUBLE_TASK.run(ctx, 42).await.expect("memo hit succeeds"));
		});
	}
}
