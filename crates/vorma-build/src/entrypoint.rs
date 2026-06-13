//! Public build entrypoint facade.

use std::io::Write;
use std::sync::mpsc::{self, Receiver, Sender, TryRecvError};
use std::thread;
use std::time::Duration;

use vorma::AppConfig;
use vorma::build_interface::AppBuildContract;
use vorma::build_interface::assets::ManifestMode;

use crate::app_server_build::build_app_server_and_read_live_state_until_cancelled;
use crate::app_server_process::PrebuiltAppServer;
use crate::build_plan::BuildDevWatchIntent;
use crate::dev_build::{
	DevBuildError, DevStaticGenerationKind, DevStaticGenerationUpdate, StartedDevGeneration,
	start_and_activate_dev_generation_on_loopback,
};
use crate::dev_refresh::ChangeType;
use crate::dev_signal::start_dev_shutdown_signal_listener;
use crate::dev_watcher::{
	DevFileChange, DevFileWatchPlan, DevWatcherError, start_dev_file_watcher,
};
use crate::generation_epoch::EpochSupervisor;
use crate::live_state_command::read_live_build_state_from_executable_until_cancelled;
use crate::process_runner::{BuildProcessCancel, StdBuildProcessRunner};
use crate::production_build::{
	ProductionBuildError, ProductionBuildReport,
	build_and_activate_production_generation_on_loopback,
};

const APP_SERVER_READY_PREFIX: &str = "App server ready: ";
const DEV_WATCHER_SETTLE_DEBOUNCE_INTERVAL: Duration = Duration::from_millis(10);

enum DevLoopEvent {
	FileChange(DevFileChange),
	FileWatcherFailed(DevWatcherError),
	ChildProcessExited(&'static str),
	Shutdown,
	ShutdownSignalFailed(String),
	RebuildFinished(Box<FinishedDevRebuild>),
}

enum DevLoopAction {
	Rebuild(DevFileChange),
	Shutdown,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum DevRebuildWork {
	ClientRevalidation,
	StaticGeneration(DevStaticGenerationUpdate),
	AppServerGeneration,
}

struct FinishedDevRebuild {
	started: StartedDevGeneration,
	runner: StdBuildProcessRunner,
	action: FinishedDevRebuildAction,
}

enum FinishedDevRebuildAction {
	WaitForNextChange,
	Rebuild(DevFileChange),
	Shutdown,
}

/// Run the Vorma build entry in production-build or dev-server mode.
pub fn run<S, F>(app_config: F) -> Result<(), String>
where
	F: FnOnce() -> vorma::Result<AppConfig<S>>,
	S: Send + Sync + 'static,
{
	run_with_args(app_config, std::env::args().skip(1)).map_err(|source| source.to_string())
}

/// Build one production generation.
pub fn build_production<S, F>(app_config: F) -> Result<ProductionBuildReport, BuildEntrypointError>
where
	F: FnOnce() -> vorma::Result<AppConfig<S>>,
	S: Send + Sync + 'static,
{
	let app = build_contract(app_config)?;
	current_thread_runtime()?.block_on(async move {
		let mut supervisor = EpochSupervisor::default();
		let (_, report) = build_and_activate_production_generation_on_loopback(
			&mut supervisor,
			app,
			ManifestMode::Prod,
		)
		.await
		.map_err(|source| BuildEntrypointError::ProductionBuild { source })?;
		Ok(report)
	})
}

/// Start one dev server generation and return the live process handle.
pub fn start_dev_server<S, F>(app_config: F) -> Result<StartedDevGeneration, BuildEntrypointError>
where
	F: FnOnce() -> vorma::Result<AppConfig<S>>,
	S: Send + Sync + 'static,
{
	let app = build_contract(app_config)?;
	current_thread_runtime()?.block_on(async move {
		start_and_activate_dev_generation_on_loopback(app)
			.await
			.map_err(|source| BuildEntrypointError::DevBuild { source })
	})
}

fn run_with_args<S, F, I>(app_config: F, args: I) -> Result<(), BuildEntrypointError>
where
	F: FnOnce() -> vorma::Result<AppConfig<S>>,
	S: Send + Sync + 'static,
	I: IntoIterator<Item = String>,
{
	match parse_build_command(args)? {
		BuildCommand::Build => {
			build_production(app_config)?;
			Ok(())
		}
		BuildCommand::Dev => {
			let started = start_dev_server(app_config)?;
			run_dev_forever(started)
		}
	}
}

fn build_contract<S, F>(app_config: F) -> Result<AppBuildContract, BuildEntrypointError>
where
	F: FnOnce() -> vorma::Result<AppConfig<S>>,
	S: Send + Sync + 'static,
{
	vorma::build_interface::env::with_build_mode(|| {
		let app_config = app_config().map_err(|source| BuildEntrypointError::AppConfig {
			message: source.to_string(),
		})?;
		vorma::build_interface::app_build_contract(app_config).map_err(|source| {
			BuildEntrypointError::AppConfig {
				message: source.to_string(),
			}
		})
	})
}

fn current_thread_runtime() -> Result<tokio::runtime::Runtime, BuildEntrypointError> {
	tokio::runtime::Builder::new_current_thread()
		.enable_io()
		.build()
		.map_err(|source| BuildEntrypointError::Runtime {
			message: source.to_string(),
		})
}

fn run_dev_forever(mut started: StartedDevGeneration) -> Result<(), BuildEntrypointError> {
	println!(
		"{APP_SERVER_READY_PREFIX}http://localhost:{}",
		started.report().dev_mux_port()
	);
	std::io::stdout()
		.flush()
		.map_err(|source| BuildEntrypointError::Output {
			message: source.to_string(),
		})?;
	let watcher = start_dev_file_watcher(compile_dev_watch_plan(&started)?)
		.map_err(|source| BuildEntrypointError::DevWatcher { source })?;
	let watch_plan_handle = watcher.plan_handle();
	let (dev_event_tx, dev_event_rx) = mpsc::channel();
	let file_event_tx = dev_event_tx.clone();
	let _ = thread::spawn(move || {
		loop {
			let event = match watcher.recv_settled(DEV_WATCHER_SETTLE_DEBOUNCE_INTERVAL) {
				Ok(change) => DevLoopEvent::FileChange(change),
				Err(source) => DevLoopEvent::FileWatcherFailed(source),
			};
			let terminal = matches!(event, DevLoopEvent::FileWatcherFailed(_));
			if file_event_tx.send(event).is_err() || terminal {
				return;
			}
		}
	});
	/*
	A dead app server or Vite is fatal for the whole dev session (Go
	parity): the loop surfaces it as an error instead of serving nothing.
	*/
	let child_exit_tx = dev_event_tx.clone();
	started.watch_child_exits(std::sync::Arc::new(move |name| {
		let _ = child_exit_tx.send(DevLoopEvent::ChildProcessExited(name));
	}));
	let shutdown_event_tx = dev_event_tx.clone();
	let shutdown_signal = start_dev_shutdown_signal_listener();
	let _ = thread::spawn(move || {
		let event = match shutdown_signal.recv() {
			Ok(()) => DevLoopEvent::Shutdown,
			Err(message) => DevLoopEvent::ShutdownSignalFailed(message),
		};
		let _ = shutdown_event_tx.send(event);
	});
	let mut runner = StdBuildProcessRunner;
	let mut pending_rebuild = None;
	loop {
		let change = match pending_rebuild.take() {
			Some(change) => change,
			None => match next_dev_loop_action(&dev_event_rx)? {
				DevLoopAction::Rebuild(change) => change,
				DevLoopAction::Shutdown => return Ok(()),
			},
		};
		let cancel = BuildProcessCancel::new();
		spawn_dev_rebuild(
			started,
			runner,
			change,
			cancel.clone(),
			dev_event_tx.clone(),
		);
		let finished = wait_for_inflight_dev_rebuild(&dev_event_rx, &cancel)?;
		started = finished.started;
		runner = finished.runner;
		/*
		Generation facts the classifier depends on — view modules, declared
		assets, critical-CSS imports — may have changed; swap the watcher's
		classification plan without touching the OS watch roots.
		*/
		watch_plan_handle.replace(compile_dev_watch_plan(&started)?);
		match finished.action {
			FinishedDevRebuildAction::WaitForNextChange => {}
			FinishedDevRebuildAction::Rebuild(change) => {
				pending_rebuild = Some(change);
			}
			FinishedDevRebuildAction::Shutdown => return Ok(()),
		}
	}
}

fn compile_dev_watch_plan(
	started: &StartedDevGeneration,
) -> Result<DevFileWatchPlan, BuildEntrypointError> {
	let plan = started.committed_generation().build_plan();
	DevFileWatchPlan::compile(
		plan.workspace().root_dir(),
		&plan
			.dev_watch()
			.extended_with_critical_css_imports(started.critical_css_import_paths()),
	)
	.map_err(|source| BuildEntrypointError::DevWatcher { source })
}

fn classify_dev_rebuild_work(change: &DevFileChange) -> Option<DevRebuildWork> {
	if !change.requires_vorma_generation() {
		return None;
	}
	if change.requires_app_server_generation() {
		return Some(DevRebuildWork::AppServerGeneration);
	}
	if change.requires_static_output_generation() {
		let kind = if change
			.intents()
			.contains(&BuildDevWatchIntent::PublicStaticInput)
		{
			DevStaticGenerationKind::PublicStaticAndCriticalCss
		} else {
			DevStaticGenerationKind::CriticalCss
		};
		return Some(DevRebuildWork::StaticGeneration(
			DevStaticGenerationUpdate::new(kind, change.requires_client_revalidation()),
		));
	}
	if change.requires_client_revalidation() {
		return Some(DevRebuildWork::ClientRevalidation);
	}
	None
}

fn spawn_dev_rebuild(
	mut started: StartedDevGeneration,
	mut runner: StdBuildProcessRunner,
	change: DevFileChange,
	cancel: BuildProcessCancel,
	dev_event_tx: Sender<DevLoopEvent>,
) {
	thread::spawn(move || {
		let Some(work) = classify_dev_rebuild_work(&change) else {
			let _ = dev_event_tx.send(DevLoopEvent::RebuildFinished(Box::new(
				FinishedDevRebuild {
					started,
					runner,
					action: FinishedDevRebuildAction::WaitForNextChange,
				},
			)));
			return;
		};
		if matches!(work, DevRebuildWork::ClientRevalidation) {
			started.broadcast_client_revalidation();
			let _ = dev_event_tx.send(DevLoopEvent::RebuildFinished(Box::new(
				FinishedDevRebuild {
					started,
					runner,
					action: FinishedDevRebuildAction::WaitForNextChange,
				},
			)));
			return;
		}
		if let DevRebuildWork::StaticGeneration(update) = work {
			let result = started
				.activate_next_static_generation(update)
				.map_err(|source| source.to_string());
			if let Err(message) = result {
				started.broadcast_build_error(message);
			}
			let _ = dev_event_tx.send(DevLoopEvent::RebuildFinished(Box::new(
				FinishedDevRebuild {
					started,
					runner,
					action: FinishedDevRebuildAction::WaitForNextChange,
				},
			)));
			return;
		}

		started.broadcast_rebuilding();
		let root_dir = started
			.committed_generation()
			.build_plan()
			.workspace()
			.root_dir()
			.to_owned();
		let server_cargo_package = started
			.committed_generation()
			.build_plan()
			.server_target()
			.cargo_package()
			.to_owned();
		let server_cargo_bin = started
			.committed_generation()
			.build_plan()
			.server_target()
			.cargo_bin()
			.to_owned();
		/*
		The app-server binary is the only app-linked binary: one cargo
		invocation builds it, the same executable answers live-state and then
		serves as the prebuilt dev server.
		*/
		let result = (|| {
			let (server_executable, live_state) =
				build_app_server_and_read_live_state_until_cancelled(
					&root_dir,
					&server_cargo_package,
					&server_cargo_bin,
					&mut runner,
					&cancel,
					|executable, live_state_cancel| {
						read_live_build_state_from_executable_until_cancelled(
							executable,
							&root_dir,
							&mut StdBuildProcessRunner,
							live_state_cancel,
						)
					},
				)
				.map_err(|source| source.to_string())?;
			/*
			The session bootstrapped on one server target; a mid-session
			target change means this rebuild compiled the OLD binary, so the
			only correct move is a dev-server restart.
			*/
			if let Some(declared) = live_state.server_build_target()
				&& (declared.cargo_package() != server_cargo_package
					|| declared.cargo_bin() != server_cargo_bin)
			{
				return Err(format!(
					"server cargo target changed (was {server_cargo_package}/{server_cargo_bin}, \
					now {}/{}); restart the dev server to pick it up",
					declared.cargo_package(),
					declared.cargo_bin()
				));
			}
			let prebuilt_app_server = PrebuiltAppServer::new(
				&server_cargo_package,
				&server_cargo_bin,
				&server_executable,
			);
			let report = if change.requires_static_output_generation() {
				started.activate_next_live_state_generation_until_cancelled(
					live_state,
					prebuilt_app_server,
					&mut runner,
					&cancel,
				)
			} else {
				started
					.activate_next_live_state_generation_reusing_committed_static_outputs_until_cancelled(
						live_state,
						prebuilt_app_server,
						&mut runner,
						&cancel,
					)
			}
			.map_err(|source| source.to_string())?;
			if change.requires_client_revalidation()
				&& !report.refresh_payloads().iter().any(|payload| {
					matches!(
						payload.change_type(),
						ChangeType::ClientRevalidate | ChangeType::HardReload
					)
				}) {
				started.broadcast_client_revalidation();
			}
			if let Some(message) = report.retired_app_server_termination_error() {
				eprintln!("Retired app server termination failed: {message}");
			}
			Ok::<(), String>(())
		})();
		if let Err(message) = result
			&& !cancel.is_cancelled()
		{
			started.broadcast_build_error(message);
		}
		let _ = dev_event_tx.send(DevLoopEvent::RebuildFinished(Box::new(
			FinishedDevRebuild {
				started,
				runner,
				action: FinishedDevRebuildAction::WaitForNextChange,
			},
		)));
	});
}

fn wait_for_inflight_dev_rebuild(
	dev_event_rx: &Receiver<DevLoopEvent>,
	cancel: &BuildProcessCancel,
) -> Result<FinishedDevRebuild, BuildEntrypointError> {
	let mut queued_change = None;
	let mut shutdown_requested = false;
	let mut deferred_error = None;
	loop {
		let event = dev_event_rx
			.recv()
			.map_err(|_| BuildEntrypointError::DevWatcher {
				source: DevWatcherError::Stopped,
			})?;
		match event {
			DevLoopEvent::RebuildFinished(mut finished) => {
				if let Some(error) = deferred_error {
					// Dropping `finished` terminates the app server and Vite
					// processes owned by the in-flight generation.
					return Err(error);
				}
				finished.action = if shutdown_requested {
					FinishedDevRebuildAction::Shutdown
				} else if let Some(change) = queued_change {
					FinishedDevRebuildAction::Rebuild(change)
				} else {
					FinishedDevRebuildAction::WaitForNextChange
				};
				return Ok(*finished);
			}
			event => {
				if let Err(error) = collect_inflight_dev_loop_event(
					event,
					&mut queued_change,
					&mut shutdown_requested,
					cancel,
				) {
					// Returning before RebuildFinished would leak the in-flight
					// generation's child processes inside the rebuild thread;
					// cancel it and surface the error once it hands back state.
					cancel.cancel();
					deferred_error.get_or_insert(error);
				}
			}
		}
	}
}

fn collect_inflight_dev_loop_event(
	event: DevLoopEvent,
	queued_change: &mut Option<DevFileChange>,
	shutdown_requested: &mut bool,
	cancel: &BuildProcessCancel,
) -> Result<(), BuildEntrypointError> {
	let Some(change) = dev_loop_file_change_or_return(event)? else {
		*shutdown_requested = true;
		cancel.cancel();
		return Ok(());
	};
	if !change.requires_vorma_generation() {
		return Ok(());
	}
	let Some(work) = classify_dev_rebuild_work(&change) else {
		return Ok(());
	};
	if matches!(
		work,
		DevRebuildWork::ClientRevalidation | DevRebuildWork::StaticGeneration(_)
	) {
		if let Some(queued) = queued_change {
			queued.extend(change);
		} else {
			*queued_change = Some(change);
		}
		return Ok(());
	}
	cancel.cancel();
	if let Some(queued) = queued_change {
		queued.extend(change);
	} else {
		*queued_change = Some(change);
	}
	Ok(())
}

fn next_dev_loop_action(
	dev_event_rx: &Receiver<DevLoopEvent>,
) -> Result<DevLoopAction, BuildEntrypointError> {
	loop {
		let event = dev_event_rx
			.recv()
			.map_err(|_| BuildEntrypointError::DevWatcher {
				source: DevWatcherError::Stopped,
			})?;
		let Some(change) = dev_loop_file_change_or_return(event)? else {
			return Ok(DevLoopAction::Shutdown);
		};
		if !change.requires_vorma_generation() {
			continue;
		}
		return drain_pending_dev_loop_action(change, dev_event_rx);
	}
}

fn drain_pending_dev_loop_action(
	mut change: DevFileChange,
	dev_event_rx: &Receiver<DevLoopEvent>,
) -> Result<DevLoopAction, BuildEntrypointError> {
	loop {
		match dev_event_rx.try_recv() {
			Ok(event) => {
				let Some(next_change) = dev_loop_file_change_or_return(event)? else {
					return Ok(DevLoopAction::Shutdown);
				};
				change.extend(next_change);
			}
			Err(TryRecvError::Empty) => return Ok(DevLoopAction::Rebuild(change)),
			Err(TryRecvError::Disconnected) => {
				return Err(BuildEntrypointError::DevWatcher {
					source: DevWatcherError::Stopped,
				});
			}
		}
	}
}

fn dev_loop_file_change_or_return(
	event: DevLoopEvent,
) -> Result<Option<DevFileChange>, BuildEntrypointError> {
	match event {
		DevLoopEvent::FileChange(change) => Ok(Some(change)),
		DevLoopEvent::FileWatcherFailed(source) => Err(BuildEntrypointError::DevWatcher { source }),
		DevLoopEvent::ChildProcessExited(name) => {
			Err(BuildEntrypointError::ChildProcessExited { name })
		}
		DevLoopEvent::Shutdown => Ok(None),
		DevLoopEvent::ShutdownSignalFailed(message) => {
			Err(BuildEntrypointError::DevShutdownSignal { message })
		}
		DevLoopEvent::RebuildFinished(_) => Err(BuildEntrypointError::DevLoop {
			message: "rebuild finished with no rebuild in flight".to_owned(),
		}),
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum BuildCommand {
	Build,
	Dev,
}

fn parse_build_command(
	args: impl IntoIterator<Item = String>,
) -> Result<BuildCommand, BuildEntrypointError> {
	let args = args.into_iter().collect::<Vec<_>>();
	match args.as_slice() {
		[] => Ok(BuildCommand::Build),
		[arg] if arg == "dev" => Ok(BuildCommand::Dev),
		_ => Err(BuildEntrypointError::InvalidArgs),
	}
}

/// Build entrypoint error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BuildEntrypointError {
	/// Command line arguments were invalid.
	InvalidArgs,
	/// A dev child process exited unexpectedly.
	ChildProcessExited {
		/// Child process display name.
		name: &'static str,
	},
	/// App configuration failed.
	AppConfig {
		/// Error message.
		message: String,
	},
	/// Tokio runtime creation failed.
	Runtime {
		/// Error message.
		message: String,
	},
	/// Production build failed.
	ProductionBuild {
		/// Source production build error.
		source: ProductionBuildError,
	},
	/// Dev build failed.
	DevBuild {
		/// Source dev build error.
		source: DevBuildError,
	},
	/// Dev filesystem watcher failed.
	DevWatcher {
		/// Source watcher error.
		source: DevWatcherError,
	},
	/// Dev shutdown signal listener failed.
	DevShutdownSignal {
		/// Error message.
		message: String,
	},
	/// Dev loop internal coordination failed.
	DevLoop {
		/// Error message.
		message: String,
	},
	/// Output write failed.
	Output {
		/// Error message.
		message: String,
	},
}

impl std::fmt::Display for BuildEntrypointError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidArgs => f.write_str("usage: build-entry [dev]"),
			Self::ChildProcessExited { name } => {
				write!(f, "{name} process exited unexpectedly")
			}
			Self::AppConfig { message } => write!(f, "{message}"),
			Self::Runtime { message } => write!(f, "create build runtime: {message}"),
			Self::ProductionBuild { source } => write!(f, "{source}"),
			Self::DevBuild { source } => write!(f, "{source}"),
			Self::DevWatcher { source } => write!(f, "{source}"),
			Self::DevShutdownSignal { message } => {
				write!(f, "dev shutdown signal listener failed: {message}")
			}
			Self::DevLoop { message } => write!(f, "dev loop failed: {message}"),
			Self::Output { message } => write!(f, "write build-entry output: {message}"),
		}
	}
}

impl std::error::Error for BuildEntrypointError {}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::build_plan::BuildDevWatchIntent;

	#[test]
	fn dev_watcher_settle_debounce_stays_below_local_latency_budget() {
		assert!(DEV_WATCHER_SETTLE_DEBOUNCE_INTERVAL <= std::time::Duration::from_millis(10));
	}

	#[test]
	fn dev_loop_action_coalesces_queued_generation_file_changes() {
		let (tx, rx) = mpsc::channel();
		tx.send(DevLoopEvent::FileChange(
			DevFileChange::from_intents_for_test([BuildDevWatchIntent::ServerRecompile]),
		))
		.unwrap();
		tx.send(DevLoopEvent::FileChange(
			DevFileChange::from_intents_for_test([BuildDevWatchIntent::CriticalCssInput]),
		))
		.unwrap();

		let action = next_dev_loop_action(&rx).unwrap();

		let DevLoopAction::Rebuild(change) = action else {
			panic!("expected rebuild action");
		};
		assert!(
			change
				.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)
		);
		assert!(
			change
				.intents()
				.contains(&BuildDevWatchIntent::CriticalCssInput)
		);
	}

	#[test]
	fn dev_loop_action_queued_shutdown_prevents_next_rebuild() {
		let (tx, rx) = mpsc::channel();
		tx.send(DevLoopEvent::FileChange(
			DevFileChange::from_intents_for_test([BuildDevWatchIntent::ServerRecompile]),
		))
		.unwrap();
		tx.send(DevLoopEvent::Shutdown).unwrap();

		assert!(matches!(
			next_dev_loop_action(&rx).unwrap(),
			DevLoopAction::Shutdown
		));
	}

	#[test]
	fn dev_rebuild_work_classifies_static_updates_without_app_server_generation() {
		let critical_css =
			DevFileChange::from_intents_for_test([BuildDevWatchIntent::CriticalCssInput]);
		let public_static_with_revalidation = DevFileChange::from_intents_for_test([
			BuildDevWatchIntent::PublicStaticInput,
			BuildDevWatchIntent::ClientRevalidate,
		]);
		let server_recompile =
			DevFileChange::from_intents_for_test([BuildDevWatchIntent::ServerRecompile]);

		assert_eq!(
			classify_dev_rebuild_work(&critical_css),
			Some(DevRebuildWork::StaticGeneration(
				DevStaticGenerationUpdate::new(DevStaticGenerationKind::CriticalCss, false)
			))
		);
		assert_eq!(
			classify_dev_rebuild_work(&public_static_with_revalidation),
			Some(DevRebuildWork::StaticGeneration(
				DevStaticGenerationUpdate::new(
					DevStaticGenerationKind::PublicStaticAndCriticalCss,
					true
				)
			))
		);
		assert_eq!(
			classify_dev_rebuild_work(&server_recompile),
			Some(DevRebuildWork::AppServerGeneration)
		);
	}

	#[test]
	fn inflight_generation_change_cancels_current_rebuild_and_queues_next_rebuild() {
		let cancel = BuildProcessCancel::new();
		let mut queued_change = None;
		let mut shutdown_requested = false;

		collect_inflight_dev_loop_event(
			DevLoopEvent::FileChange(DevFileChange::from_intents_for_test([
				BuildDevWatchIntent::ServerRecompile,
			])),
			&mut queued_change,
			&mut shutdown_requested,
			&cancel,
		)
		.unwrap();

		assert!(cancel.is_cancelled());
		assert!(!shutdown_requested);
		assert!(
			queued_change
				.unwrap()
				.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)
		);
	}

	#[test]
	fn inflight_non_generation_change_does_not_cancel_current_rebuild() {
		let cancel = BuildProcessCancel::new();
		let mut queued_change = None;
		let mut shutdown_requested = false;

		collect_inflight_dev_loop_event(
			DevLoopEvent::FileChange(DevFileChange::from_intents_for_test([
				BuildDevWatchIntent::FrontendBuildInput,
			])),
			&mut queued_change,
			&mut shutdown_requested,
			&cancel,
		)
		.unwrap();

		assert!(!cancel.is_cancelled());
		assert!(!shutdown_requested);
		assert!(queued_change.is_none());
	}

	#[test]
	fn inflight_static_generation_queues_without_canceling_current_rebuild() {
		let cancel = BuildProcessCancel::new();
		let mut queued_change = None;
		let mut shutdown_requested = false;

		collect_inflight_dev_loop_event(
			DevLoopEvent::FileChange(DevFileChange::from_intents_for_test([
				BuildDevWatchIntent::CriticalCssInput,
			])),
			&mut queued_change,
			&mut shutdown_requested,
			&cancel,
		)
		.unwrap();

		assert!(!cancel.is_cancelled());
		assert!(!shutdown_requested);
		let queued_change = queued_change.unwrap();
		assert!(
			queued_change
				.intents()
				.contains(&BuildDevWatchIntent::CriticalCssInput)
		);
		assert!(!queued_change.requires_app_server_generation());
		assert!(queued_change.requires_static_output_generation());
	}

	#[test]
	fn inflight_client_revalidation_queues_without_canceling_current_rebuild() {
		let cancel = BuildProcessCancel::new();
		let mut queued_change = None;
		let mut shutdown_requested = false;

		collect_inflight_dev_loop_event(
			DevLoopEvent::FileChange(DevFileChange::from_intents_for_test([
				BuildDevWatchIntent::ClientRevalidate,
			])),
			&mut queued_change,
			&mut shutdown_requested,
			&cancel,
		)
		.unwrap();

		assert!(!cancel.is_cancelled());
		assert!(!shutdown_requested);
		let queued_change = queued_change.unwrap();
		assert!(
			queued_change
				.intents()
				.contains(&BuildDevWatchIntent::ClientRevalidate)
		);
		assert!(queued_change.requires_only_client_revalidation());
	}

	#[test]
	fn inflight_shutdown_cancels_current_rebuild() {
		let cancel = BuildProcessCancel::new();
		let mut queued_change = None;
		let mut shutdown_requested = false;

		collect_inflight_dev_loop_event(
			DevLoopEvent::Shutdown,
			&mut queued_change,
			&mut shutdown_requested,
			&cancel,
		)
		.unwrap();

		assert!(cancel.is_cancelled());
		assert!(shutdown_requested);
		assert!(queued_change.is_none());
	}

	#[test]
	fn parse_build_command_accepts_empty_and_dev_only() {
		assert_eq!(
			parse_build_command(Vec::<String>::new()).unwrap(),
			BuildCommand::Build
		);
		assert_eq!(
			parse_build_command(["dev".to_owned()]).unwrap(),
			BuildCommand::Dev
		);
	}

	#[test]
	fn parse_build_command_rejects_unknown_args() {
		let error = parse_build_command(["serve".to_owned()]).unwrap_err();

		assert_eq!(error, BuildEntrypointError::InvalidArgs);
	}
}
