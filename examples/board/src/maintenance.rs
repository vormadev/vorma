//! Background maintenance: the app's own task runtime, outside any request.
/*
`vorma-tasks` is a sovereign crate, not a request helper: handlers get an
`ExecCtx` from the framework's runtime because HTTP requests happen to be
one common shape of work, but the same primitive is just as usable for a
daemon loop, a CLI, or a scheduled job. This module is that second shape:
a task runtime this app constructs and owns end to end, with its own
`Tasks`, its own execution contexts, and its own cancellation wiring, none
of it borrowed from the framework's request-handling machinery.
*/

use std::sync::Arc;
use std::time::Duration;

use rusqlite::params;
use vorma::tasks::{CancelToken, Tasks, TasksOptions};

use crate::repo::{self, DbInput};
use crate::store::Db;

/// How long a session may sit unused before the maintenance worker prunes it.
const SESSION_RETENTION: Duration = Duration::from_secs(30 * 24 * 60 * 60);
/// How often the worker wakes up to check for expired sessions.
const SWEEP_INTERVAL: Duration = Duration::from_secs(60 * 60);
/// Runs slower than this are logged by [`SlowTaskObserver`].
const SLOW_TASK_THRESHOLD: Duration = Duration::from_millis(50);

// Real domain work, not a demo no-op: expired sessions are exactly the
// kind of upkeep a long-running app needs regardless of request traffic,
// so this earns its place as the worker's job instead of an invented
// counter.
vorma::tasks::task! {
	static PRUNE_EXPIRED_SESSIONS: vorma::tasks::Task<DbInput<()>, usize, vorma::Error> =
		memoized(|_ctx, arg: DbInput<()>| async move {
			let cutoff = unix_now() - SESSION_RETENTION.as_secs() as i64;
			repo::blocking_task(arg.db, move |conn| {
				conn.execute(
					"DELETE FROM sessions WHERE created_at < ?1",
					params![cutoff],
				)
			})
			.await
		});
}

fn unix_now() -> i64 {
	let since_epoch = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH);
	#[expect(
		clippy::unwrap_used,
		reason = "the system clock is never before the Unix epoch on any platform this app targets"
	)]
	let secs = since_epoch.unwrap().as_secs();
	secs as i64
}

/// Run the session-pruning worker until `shutdown` is cancelled.
///
/// This is plain application code: a `tokio::time::interval` loop racing a shutdown
/// signal via `select!`. Real apps run their own loops like this all the time; Vorma does
/// not need to know about it, because nothing here touches request handling. The teaching
/// point is upstream of the loop shape: the worker builds its own [`Tasks`] runtime with
/// [`Tasks::new`], and opens one [`vorma::tasks::ExecCtx`] per iteration with
/// [`Tasks::exec_ctx`] so each sweep gets fresh memoization state, exactly like a fresh
/// request would. `observer` is the same slow-task sink the server main wires into the
/// framework's own task runtime (see [`slow_task_observer`]), so one log stream covers
/// task latency everywhere in the process, not just inside requests.
pub async fn run_session_pruner(
	db: Arc<Db>,
	shutdown: CancelToken,
	observer: Arc<dyn vorma::tasks::TaskObserver>,
) {
	let tasks: Tasks<vorma::Error> = Tasks::new(TasksOptions {
		observer: Some(observer),
		..TasksOptions::default()
	});
	let mut interval = tokio::time::interval(SWEEP_INTERVAL);
	// The first tick fires immediately; a freshly booted server should not
	// wait a full sweep interval before its first prune.
	loop {
		tokio::select! {
			_ = interval.tick() => {}
			() = shutdown.cancelled() => {
				tracing::info!("session pruner stopping: shutdown signal received");
				return;
			}
		}

		/*
		Each iteration is its own execution context, cancelled the instant
		shutdown fires so an in-flight sweep does not keep the process alive
		past its graceful-shutdown window. `child()` here (rather than handing
		out `shutdown` directly) keeps this iteration's cancellation state
		independent of any other consumer of the same shutdown token.
		*/
		let ctx = tasks.exec_ctx(shutdown.child());
		let arg = DbInput {
			db: Arc::clone(&db),
			input: (),
		};
		match PRUNE_EXPIRED_SESSIONS.run(&ctx, arg).await {
			Ok(pruned) if *pruned > 0 => {
				tracing::info!(pruned = *pruned, "session pruner: removed expired sessions");
			}
			Ok(_) => {}
			Err(error) if error.is_cancelled() => {
				tracing::info!("session pruner: sweep cancelled by shutdown");
			}
			Err(error) => {
				tracing::error!("session pruner: sweep failed: {error}");
			}
		}
	}
}

/// Build the shared slow-task observer.
///
/// `TasksOptions.observer` is how an app gets production task-runtime observability
/// without touching handler code or task bodies: every task run through a [`Tasks`]
/// runtime configured with this observer gets logged when it crosses the slow-task
/// threshold, regardless of which view, resource, or background job asked for it. The
/// server main wires the same instance into both the framework's request-serving task
/// runtime and this module's standalone worker runtime.
pub fn slow_task_observer() -> Arc<dyn vorma::tasks::TaskObserver> {
	Arc::new(SlowTaskObserver)
}

struct SlowTaskObserver;

impl vorma::tasks::TaskObserver for SlowTaskObserver {
	fn observe(&self, event: vorma::tasks::TaskEvent) {
		let vorma::tasks::TaskEventKind::RunCompleted { duration, .. } = &event.kind else {
			return;
		};
		if *duration < SLOW_TASK_THRESHOLD {
			return;
		}
		tracing::warn!(
			task_name = event.task_name,
			task_id = ?event.task_id,
			duration_ms = duration.as_millis(),
			"slow task run"
		);
	}
}
