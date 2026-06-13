//! Ports of the Go kit/tasks benchmark suite.
//!
//! Each benchmark mirrors its Go counterpart's shape: fresh execution
//! contexts model per-request use, the repeated-calls case pins the
//! memo-hit hot path, and the scaling family stresses run_parallel
//! fan-out. Two deliberate deviations from the Go originals: the
//! high-contention task spin-waits one microsecond because the tokio
//! timer cannot sleep that precisely, and the cancellation case cancels
//! via a spawned task rather than a timer for the same reason.

use std::hint::black_box;
use std::sync::Arc;
use std::time::{Duration, Instant};

// This crate does not link the vorma server crate, so the bench
// binary declares the allocator vorma apps run by default.
#[global_allocator]
static GLOBAL_ALLOCATOR: mimalloc::MiMalloc = mimalloc::MiMalloc;

use vorma_bench::Bench;
use vorma_tasks::{CancelToken, ExecCtx, PreparedTask, Task, Tasks, TasksOptions};

type BenchError = Box<dyn std::error::Error + Send + Sync>;

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
		let task: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 2) });
		let tasks = &tasks;
		let task = &task;
		let mut i = 0i64;
		bench.bench_async("single_task", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				black_box(task.run(&ctx, input).await.expect("task succeeds"));
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let task_1: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 2) });
		let task_2: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 3) });
		let task_3: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 4) });
		let tasks = &tasks;
		let task_1 = &task_1;
		let task_2 = &task_2;
		let task_3 = &task_3;
		let mut i = 0i64;
		bench.bench_async("parallel_independent_tasks", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				ctx.run_parallel([
					task_1.bind_input_with_result(input, |value| {
						black_box(*value);
					}),
					task_2.bind_input_with_result(input, |value| {
						black_box(*value);
					}),
					task_3.bind_input_with_result(input, |value| {
						black_box(*value);
					}),
				])
				.await
				.expect("parallel tasks succeed");
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let shared: Task<(), String, BenchError> = Task::new(Duration::ZERO, |_, ()| async move {
			spin_one_microsecond();
			Ok("result".to_owned())
		});
		let tasks = &tasks;
		let shared = &shared;
		bench.bench_async("high_contention", &rt, move || async move {
			{
				let ctx = exec_ctx(tasks);
				let mut joined = tokio::task::JoinSet::new();
				for _ in 0..10 {
					let ctx = ctx.clone();
					let shared = shared.clone();
					joined.spawn(async move {
						shared.run(&ctx, ()).await.expect("shared task succeeds");
					});
				}
				while let Some(result) = joined.join_next().await {
					result.expect("no panics");
				}
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let base: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 2) });
		let dependent: Task<i64, i64, BenchError> = {
			let base = base.clone();
			Task::new(Duration::ZERO, move |ctx, input| {
				let base = base.clone();
				async move {
					let value = base.run(&ctx, input).await?;
					Ok(*value + 10)
				}
			})
		};
		let tasks = &tasks;
		let dependent = &dependent;
		let mut i = 0i64;
		bench.bench_async("task_with_dependencies", &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks);
				black_box(
					dependent
						.run(&ctx, input)
						.await
						.expect("dependent task succeeds"),
				);
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let task: Task<String, String, BenchError> =
			Task::new(Duration::ZERO, |_, input: String| async move {
				Ok(format!("Hello, {input}"))
			});
		let tasks = &tasks;
		let task = &task;
		bench.bench_async("allocations", &rt, move || async move {
			{
				let ctx = exec_ctx(tasks);
				black_box(
					task.run(&ctx, "World".to_owned())
						.await
						.expect("task succeeds"),
				);
			}
		});
	}

	for count in [1usize, 2, 5, 10, 20, 50] {
		let tasks = tasks_runtime();
		let task_list: Arc<Vec<Task<i64, i64, BenchError>>> = Arc::new(
			(0..count)
				.map(|id| {
					let id = id as i64;
					Task::new(
						Duration::ZERO,
						move |_, input| async move { Ok(input + id) },
					)
				})
				.collect(),
		);
		let tasks_ref = &tasks;
		let task_list = &task_list;
		let mut i = 0i64;
		bench.bench_async(&format!("parallel_scaling/tasks-{count}"), &rt, move || {
			i += 1;
			let input = i;
			async move {
				let ctx = exec_ctx(tasks_ref);
				let bound: Vec<PreparedTask<BenchError>> = task_list
					.iter()
					.map(|task| {
						task.bind_input_with_result(input, |value| {
							black_box(*value);
						})
					})
					.collect();
				ctx.run_parallel(bound)
					.await
					.expect("parallel tasks succeed");
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let task: Task<i64, i64, BenchError> = Task::new(Duration::ZERO, |_, input| async move {
			tokio::time::sleep(Duration::from_millis(10)).await;
			Ok(input * 2)
		});
		let tasks = &tasks;
		let task = &task;
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
				let _ = black_box(task.run(&ctx, input).await);
				canceller.await.expect("canceller completes");
			}
		});
	}

	{
		let tasks = tasks_runtime();
		let task: Task<i64, i64, BenchError> =
			Task::new(Duration::ZERO, |_, input| async move { Ok(input * 2) });
		let ctx = rt.block_on(async {
			let ctx = exec_ctx(&tasks);
			task.run(&ctx, 42).await.expect("warm-up run succeeds");
			ctx
		});
		let ctx = &ctx;
		let task = &task;
		bench.bench_async("repeated_task_calls", &rt, move || async move {
			black_box(task.run(ctx, 42).await.expect("memo hit succeeds"));
		});
	}
}
