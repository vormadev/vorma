use std::collections::{BTreeSet, VecDeque};
use std::path::{Path, PathBuf};
use std::sync::atomic::Ordering;
use std::sync::mpsc::{self, Sender};
use std::sync::{Arc, Mutex};
use std::thread;

use vorma::__private::Config;

use crate::build_cancel::BuildCancel;
use crate::config::{VormaCfg, to_cfg};
use crate::fswatcher::{Evt, Watcher, WatcherOptions};
use crate::globset;
use crate::work_queue::DevWorkQueueSender;

#[derive(Debug, Default)]
pub(crate) struct DevWatcher {
	thread: Option<WatcherThread>,
}

#[derive(Debug)]
struct WatcherThread {
	stop_tx: Option<Sender<()>>,
	thread: Option<thread::JoinHandle<Result<(), String>>>,
}

#[derive(Clone)]
struct WatchEventSink {
	root_dir: String,
	server_watch_set: globset::Set,
	client_revalidate_watch_set: globset::Set,
	pub_src_pattern: String,
	css_files_to_watch: Arc<Mutex<BTreeSet<PathBuf>>>,
	dev_work: DevWorkQueueSender,
	build_cancel: Arc<BuildCancel>,
}

pub(crate) struct DevWatcherStartArgs {
	pub(crate) config: Config,
	pub(crate) fatal_events: Arc<Mutex<VecDeque<String>>>,
	pub(crate) dev_work: DevWorkQueueSender,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) css_files_to_watch: Arc<Mutex<BTreeSet<PathBuf>>>,
}

impl DevWatcher {
	pub(crate) fn restart(&mut self, args: DevWatcherStartArgs) -> Result<(), String> {
		self.stop()?;
		let cfg = to_cfg(&args.config).map_err(|err| format!("error converting config: {err}"))?;
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: PathBuf::from(cfg.root_dir()),
			watch_patterns: cfg.watch_patterns(),
			..WatcherOptions::default()
		})?;
		let event_sink = watch_event_sink(
			&cfg,
			args.dev_work.clone(),
			Arc::clone(&args.build_cancel),
			Arc::clone(&args.css_files_to_watch),
		)?;
		let (stop_tx, stop_rx) = mpsc::channel();
		let (ready_tx, ready_rx) = mpsc::channel();
		let fatal_events = args.fatal_events;
		let wake_run_loop = args.dev_work;
		let thread = thread::spawn(move || {
			let result = watcher.watch(stop_rx, ready_tx, |evts| {
				event_sink.on_evt_batch(&evts);
				Ok(())
			});
			if let Err(err) = &result {
				fatal_events
					.lock()
					.expect("fatal events lock poisoned")
					.push_back(format!("filesystem watcher failed: {err}"));
				wake_run_loop.wake();
			}
			result
		});
		match ready_rx
			.recv()
			.map_err(|err| format!("filesystem watcher startup failed: {err}"))?
		{
			Ok(()) => {}
			Err(err) => {
				let _ = stop_tx.send(());
				let _ = thread.join();
				return Err(err);
			}
		}
		self.thread = Some(WatcherThread {
			stop_tx: Some(stop_tx),
			thread: Some(thread),
		});
		Ok(())
	}

	pub(crate) fn stop(&mut self) -> Result<(), String> {
		let Some(mut thread) = self.thread.take() else {
			return Ok(());
		};
		thread.stop()
	}
}

impl Drop for DevWatcher {
	fn drop(&mut self) {
		let _ = self.stop();
	}
}

impl WatcherThread {
	fn stop(&mut self) -> Result<(), String> {
		if let Some(stop_tx) = self.stop_tx.take() {
			let _ = stop_tx.send(());
		}
		let Some(thread) = self.thread.take() else {
			return Ok(());
		};
		thread
			.join()
			.map_err(|_| "filesystem watcher thread panicked".to_owned())?
	}
}

fn watch_event_sink(
	cfg: &VormaCfg<'_>,
	dev_work: DevWorkQueueSender,
	build_cancel: Arc<BuildCancel>,
	css_files_to_watch: Arc<Mutex<BTreeSet<PathBuf>>>,
) -> Result<WatchEventSink, String> {
	let server_watch_set = globset::compile(&cfg.server_watch_patterns())
		.map_err(|err| format!("compile server watch patterns: {err}"))?;
	let client_revalidate_watch_set = globset::compile(&cfg.client_revalidate_on_change_patterns())
		.map_err(|err| format!("compile client revalidate watch patterns: {err}"))?;

	Ok(WatchEventSink {
		root_dir: cfg.root_dir(),
		server_watch_set,
		client_revalidate_watch_set,
		pub_src_pattern: cfg.pub_src_pattern(),
		css_files_to_watch,
		dev_work,
		build_cancel,
	})
}

impl WatchEventSink {
	fn on_evt_batch(&self, evts: &[Evt]) {
		let mut static_implicated = false;
		let mut server_implicated = false;
		let mut client_revalidate_implicated = false;
		let css_files_to_watch = self
			.css_files_to_watch
			.lock()
			.expect("css_files_to_watch lock poisoned")
			.clone();

		for evt in evts {
			let rel_path = self.logical_path(&evt.path);
			let physical_path = self.physical_path(&rel_path, &evt.path);
			if !server_implicated && self.server_watch_set.is_match(&rel_path) {
				server_implicated = true;
				break;
			}
			if !static_implicated
				&& (globset::path_match_unvalidated(&self.pub_src_pattern, &physical_path)
					|| css_files_to_watch.contains(Path::new(&physical_path)))
			{
				static_implicated = true;
			}
			if !client_revalidate_implicated && self.client_revalidate_watch_set.is_match(&rel_path)
			{
				client_revalidate_implicated = true;
			}
		}

		if server_implicated {
			let paths = evts
				.iter()
				.map(|evt| self.logical_path(&evt.path))
				.collect::<Vec<_>>();
			eprintln!(
				"Detected server-related file change; cancelling current build and queueing rebuild: {}",
				paths.join(", ")
			);
			self.build_cancel.store(true, Ordering::SeqCst);
			self.dev_work.queue_server_refresh();
			return;
		}

		if static_implicated || client_revalidate_implicated {
			self.dev_work
				.queue_static_build(client_revalidate_implicated);
		}
	}

	fn logical_path(&self, raw_path: &str) -> String {
		let path = crate::config::sys_norm(raw_path);
		crate::config::relative_path(&self.root_dir, &path)
			.map(|path| {
				let path = crate::config::sys_norm(path.to_string_lossy().as_ref());
				if path.is_empty() {
					".".to_owned()
				} else {
					path
				}
			})
			.unwrap_or(path)
	}

	fn physical_path(&self, logical_path: &str, raw_path: &str) -> String {
		let raw_path = crate::config::sys_norm(raw_path);
		if Path::new(&raw_path).is_absolute() {
			return raw_path;
		}
		crate::config::sys_norm(
			Path::new(&self.root_dir)
				.join(logical_path)
				.to_string_lossy()
				.as_ref(),
		)
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeSet;
	use std::path::PathBuf;

	use vorma::__private::Config;
	use vorma::{DevWatchConfig, FrontendConfig, ServerConfig};

	use super::*;
	use crate::work_queue::{DevWork, DevWorkQueue};

	#[test]
	fn watch_event_sink_preserves_client_revalidate_across_static_work_coalescing() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				public_static_src_dir: "public".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			dev_watch_config: DevWatchConfig {
				on_change_recompile_server: vec!["**/*.rs".to_owned()],
				on_change_client_revalidate: vec!["src/**/*.tsx".to_owned()],
				..DevWatchConfig::default()
			},
		};
		let cfg = to_cfg(&config).unwrap();
		let dev_work = DevWorkQueue::new();
		let build_cancel = Arc::new(BuildCancel::default());
		let css_files_to_watch = Arc::new(Mutex::new(BTreeSet::new()));
		let event_sink = watch_event_sink(
			&cfg,
			dev_work.sender(),
			Arc::clone(&build_cancel),
			Arc::clone(&css_files_to_watch),
		)
		.unwrap();

		event_sink.on_evt_batch(&[Evt {
			path: "src/page.tsx".to_owned(),
			op: crate::fswatcher::Op::Edit,
			is_known_dir: false,
		}]);
		event_sink.on_evt_batch(&[Evt {
			path: "public/logo.svg".to_owned(),
			op: crate::fswatcher::Op::Edit,
			is_known_dir: false,
		}]);

		assert_eq!(
			dev_work.claim_work(),
			Some(DevWork::StaticBuild {
				includes_client_revalidate: true,
			}),
		);
	}

	#[test]
	fn watch_event_sink_reads_current_css_files_to_watch_per_batch() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				public_static_src_dir: "public".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let dev_work = DevWorkQueue::new();
		let build_cancel = Arc::new(BuildCancel::default());
		let css_files_to_watch = Arc::new(Mutex::new(BTreeSet::new()));
		let event_sink = watch_event_sink(
			&cfg,
			dev_work.sender(),
			Arc::clone(&build_cancel),
			Arc::clone(&css_files_to_watch),
		)
		.unwrap();
		*css_files_to_watch
			.lock()
			.expect("css_files_to_watch lock poisoned") =
			BTreeSet::from([PathBuf::from(cfg.root_dir()).join("styles/imported.css")]);

		event_sink.on_evt_batch(&[Evt {
			path: "styles/imported.css".to_owned(),
			op: crate::fswatcher::Op::Edit,
			is_known_dir: false,
		}]);

		assert_eq!(
			dev_work.claim_work(),
			Some(DevWork::StaticBuild {
				includes_client_revalidate: false,
			}),
		);
	}

	#[test]
	fn server_change_cancels_current_build_and_wins_over_static_work() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				public_static_src_dir: "public".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			dev_watch_config: DevWatchConfig {
				on_change_recompile_server: vec!["**/*.rs".to_owned()],
				on_change_client_revalidate: vec!["src/**/*.tsx".to_owned()],
				..DevWatchConfig::default()
			},
		};
		let cfg = to_cfg(&config).unwrap();
		let dev_work = DevWorkQueue::new();
		let build_cancel = Arc::new(BuildCancel::default());
		let css_files_to_watch = Arc::new(Mutex::new(BTreeSet::new()));
		let event_sink = watch_event_sink(
			&cfg,
			dev_work.sender(),
			Arc::clone(&build_cancel),
			Arc::clone(&css_files_to_watch),
		)
		.unwrap();

		event_sink.on_evt_batch(&[
			Evt {
				path: "public/logo.svg".to_owned(),
				op: crate::fswatcher::Op::Edit,
				is_known_dir: false,
			},
			Evt {
				path: "src/client.tsx".to_owned(),
				op: crate::fswatcher::Op::Edit,
				is_known_dir: false,
			},
			Evt {
				path: "src/server.rs".to_owned(),
				op: crate::fswatcher::Op::Edit,
				is_known_dir: false,
			},
		]);

		assert!(build_cancel.load(Ordering::SeqCst));
		assert_eq!(dev_work.claim_work(), Some(DevWork::RefreshServer));
	}
}
