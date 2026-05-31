use std::fs;
use std::sync::mpsc::TryRecvError;

use paranoid::local_lock::ProcessLock;
use vorma::__private::Config;
use vorma::__private::manifest::Manifest;

use crate::browser_sync::ChangeType;
use crate::config::to_cfg;
use crate::generation::{CommittedGeneration, GenerationCandidate, StaticEffects};
use crate::manifest::{ManifestInput, write_manifest};
use crate::session::BuildSession;
use crate::signals::SignalThread;
use crate::static_build::{StaticBuildInput, prepare_static_build, publish_static_outputs};
use crate::work_queue::DevWork;

pub(crate) fn run_dev_loop(
	session: &mut BuildSession,
	signal_thread: &mut SignalThread,
) -> Result<(), String> {
	let dev_work_rx = session.runtime_mut().take_dev_work_receiver();
	loop {
		check_fatal(session)?;
		if signal_thread.shutdown_requested()? {
			return Ok(());
		}

		match dev_work_rx.recv() {
			Ok(()) => loop {
				check_fatal(session)?;
				if signal_thread.shutdown_requested()? {
					return Ok(());
				}
				let work = session.runtime().claim_dev_work();
				if work.is_some() {
					session.runtime().reset_build_cancel();
				}
				if let Some(work) = work {
					process_dev_work(session, work)?;
				}
				match dev_work_rx.try_recv() {
					Ok(()) => continue,
					Err(TryRecvError::Empty) => break,
					Err(TryRecvError::Disconnected) => {
						return Err("dev work queue disconnected".to_owned());
					}
				}
			},
			Err(_) => return Err("dev work queue disconnected".to_owned()),
		}
	}
}

fn process_dev_work(session: &mut BuildSession, work: DevWork) -> Result<(), String> {
	match work {
		DevWork::RefreshServer => process_server_refresh(session),
		DevWork::StaticBuild {
			includes_client_revalidate,
		} => process_static_build(session, includes_client_revalidate),
	}
}

fn process_server_refresh(session: &mut BuildSession) -> Result<(), String> {
	session
		.runtime()
		.broadcast_refresh(ChangeType::ShowRebuildingOverlay, "", "");
	let stop_app_server = session.runtime_mut().begin_stop_app_server();
	let prepared = prepare_stable_server_refresh_candidate(session);
	session
		.runtime_mut()
		.finish_stop_app_server(stop_app_server, false);
	match prepared {
		Ok(candidate) => {
			let effects = candidate.static_effects.clone();
			session.commit_generation(candidate);
			let committed = session
				.committed()
				.cloned()
				.ok_or_else(|| "committed generation not available".to_owned())?;
			let manifest = crate::activation::activate_generation(
				&committed,
				crate::RunMode::Dev,
				session.runtime_mut(),
			)?
			.ok_or_else(|| "dev activation returned cancelled production outcome".to_owned())?;
			session.set_committed_manifest(manifest)?;
			broadcast_static_effects(session.runtime(), &effects, committed.static_metadata());
			Ok(())
		}
		Err(err) => {
			handle_build_error(session, "refresh", &err);
			Ok(())
		}
	}
}

pub(crate) fn prepare_stable_server_refresh_candidate(
	session: &mut BuildSession,
) -> Result<GenerationCandidate, String> {
	loop {
		let bootstrap_config = session.bootstrap_config()?;
		let candidate = crate::pipeline::prepare_generation_candidate(session)?;
		if candidate.config == bootstrap_config {
			return Ok(candidate);
		}
		apply_live_config_retry_transition(session, &bootstrap_config, &candidate.config)?;
		session.set_bootstrap_config_override(candidate.config);
	}
}

fn apply_live_config_retry_transition(
	session: &mut BuildSession,
	bootstrap_config: &Config,
	next_config: &Config,
) -> Result<(), String> {
	let bootstrap_cfg = to_cfg(bootstrap_config)
		.map_err(|err| format!("error converting bootstrap config: {err}"))?;
	let next_cfg =
		to_cfg(next_config).map_err(|err| format!("error converting live config: {err}"))?;
	if bootstrap_cfg.dist_dir() != next_cfg.dist_dir() {
		let mut next_dev_lock = ProcessLock::new(next_cfg.dev_lock_out());
		next_dev_lock
			.acquire()
			.map_err(|err| format!("error acquiring dev lock: {err}"))?;
		if let Some(mut old_dev_lock) = session.runtime_mut().replace_dev_lock(next_dev_lock) {
			old_dev_lock
				.release()
				.map_err(|err| format!("error releasing old dev lock: {err}"))?;
		}
		match fs::remove_dir_all(bootstrap_cfg.vorma_out()) {
			Ok(()) => {}
			Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
			Err(err) => return Err(format!("failed to clean up old .vorma directory: {err}")),
		}
	}
	session.runtime_mut().stop_vite_server(false);
	session.runtime_mut().stop_watcher()?;
	session.runtime().clear_dev_mux_generation();
	Ok(())
}

fn process_static_build(
	session: &mut BuildSession,
	includes_client_revalidate: bool,
) -> Result<(), String> {
	session
		.runtime()
		.broadcast_refresh(ChangeType::ShowRebuildingOverlay, "", "");
	let Some(committed) = session.committed().cloned() else {
		handle_build_error(
			session,
			"static build",
			"committed generation not available for static build",
		);
		return Ok(());
	};
	let build_cancel = session.runtime().build_cancel();
	let prepared = match prepare_static_build(&StaticBuildInput {
		config: committed.config(),
		previous_static: Some(committed.static_metadata()),
		build_cancel: build_cancel.clone(),
		includes_client_revalidate,
	}) {
		Ok(prepared) => prepared,
		Err(err) => {
			handle_build_error(session, "static build", &err);
			return Ok(());
		}
	};
	let (static_metadata, effects) = match publish_static_outputs(
		committed.config(),
		committed.live(),
		prepared,
		&build_cancel,
	) {
		Ok(result) => result,
		Err(err) => {
			handle_build_error(session, "static build", &err);
			return Ok(());
		}
	};
	let committed = session
		.committed_mut()
		.ok_or_else(|| "committed generation not available".to_owned())?;
	committed.replace_static_metadata(static_metadata);
	let committed = committed.clone();
	session.runtime_mut().restart_watcher(&committed)?;
	let manifest = write_dev_manifest(&committed, session.runtime())?;
	session.set_committed_manifest(manifest)?;
	broadcast_static_effects(session.runtime(), &effects, committed.static_metadata());
	Ok(())
}

fn write_dev_manifest(
	committed: &CommittedGeneration,
	runtime: &crate::runtime::DevRuntime,
) -> Result<Manifest, String> {
	let vite_server_port = runtime
		.vite_server_port()
		.ok_or_else(|| "Vite server port is not available for dev manifest".to_owned())?;
	let cfg =
		to_cfg(committed.config()).map_err(|err| format!("error converting config: {err}"))?;
	write_manifest(
		&cfg,
		&ManifestInput::from_generation_metadata(
			true,
			i32::from(vite_server_port),
			runtime.dev_mux_port_i32()?,
			runtime.dev_refresh_token()?,
			committed.live(),
			committed.static_metadata(),
		),
	)
}

fn broadcast_static_effects(
	runtime: &crate::runtime::DevRuntime,
	effects: &StaticEffects,
	static_metadata: &crate::generation::StaticMetadata,
) {
	for (change_type, critical_css, build_error) in
		static_refresh_messages(effects, static_metadata)
	{
		runtime.broadcast_refresh(change_type, &critical_css, &build_error);
	}
}

fn static_refresh_messages(
	effects: &StaticEffects,
	static_metadata: &crate::generation::StaticMetadata,
) -> Vec<(ChangeType, String, String)> {
	if effects.public_filemap_changed {
		return vec![(ChangeType::HardReload, String::new(), String::new())];
	}
	let mut messages = Vec::new();
	if effects.critical_css_changed {
		messages.push((
			ChangeType::UpdateCriticalCss,
			static_metadata.critical_css.clone(),
			String::new(),
		));
	}
	if effects.includes_client_revalidate {
		messages.push((ChangeType::ClientRevalidate, String::new(), String::new()));
	}
	if messages.is_empty() {
		messages.push((
			ChangeType::HideRebuildingOverlay,
			String::new(),
			String::new(),
		));
	}
	messages
}

fn handle_build_error(session: &BuildSession, context: &str, err: &str) {
	if session.runtime().build_cancel().is_cancelled() {
		eprintln!("Cancelled {context}: {err}");
		return;
	}
	eprintln!("Error during {context}: {err}");
	session
		.runtime()
		.broadcast_refresh(ChangeType::ShowBuildError, "", err);
}

fn check_fatal(session: &BuildSession) -> Result<(), String> {
	if let Some(err) = session.runtime().pop_fatal_event() {
		return Err(err);
	}
	if let Some(exit) = session.runtime().pop_child_process_exit() {
		return Err(format!("{} exited unexpectedly: {}", exit.name, exit.err));
	}
	Ok(())
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::sync::atomic::Ordering;
	use std::time::{SystemTime, UNIX_EPOCH};

	use paranoid::local_lock::ProcessLock;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use crate::RunMode;
	use crate::browser_sync::ChangeType;
	use crate::config::{CargoBinTarget, to_cfg};
	use crate::generation::{
		BuildArtifactMode, BuildArtifacts, GenerationCandidate, LiveMetadata, StaticEffects,
		StaticMetadata,
	};
	use crate::session::BuildSession;

	use super::*;

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-dev-loop-{name}-{nonce}"));
		fs::create_dir_all(&root).unwrap();
		root
	}

	fn config(root: &Path, dist_dir: &str) -> Config {
		Config {
			root_dir: root.to_path_buf(),
			dist_dir: dist_dir.to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/client/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..Config::default()
		}
	}

	fn build_entry() -> CargoBinTarget {
		CargoBinTarget {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-build".to_owned(),
		}
	}

	fn acquired_dev_lock(config: &Config) -> ProcessLock {
		let cfg = to_cfg(config).unwrap();
		let mut lock = ProcessLock::new(cfg.dev_lock_out());
		lock.acquire().unwrap();
		lock
	}

	fn committed_static_session(config: Config, static_metadata: StaticMetadata) -> BuildSession {
		let mut session = BuildSession::new(config.clone(), build_entry(), RunMode::Dev);
		session.commit_generation(GenerationCandidate {
			config,
			live: LiveMetadata {
				root_document_hash_source: "document-hash-source".to_owned(),
				..LiveMetadata::default()
			},
			static_metadata,
			manifest: None,
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/example-build"),
				mode: BuildArtifactMode::Dev {
					app_server_executable: PathBuf::from("/tmp/example-server"),
				},
			},
			static_effects: StaticEffects::default(),
		});
		session
	}

	#[test]
	fn static_refresh_messages_broadcast_critical_css_then_client_revalidate() {
		let messages = static_refresh_messages(
			&StaticEffects {
				public_filemap_changed: false,
				critical_css_changed: true,
				includes_client_revalidate: true,
			},
			&StaticMetadata {
				critical_css: "body{color:red}".to_owned(),
				..StaticMetadata::default()
			},
		);

		assert_eq!(
			messages,
			vec![
				(
					ChangeType::UpdateCriticalCss,
					"body{color:red}".to_owned(),
					String::new(),
				),
				(ChangeType::ClientRevalidate, String::new(), String::new(),),
			],
		);
	}

	#[test]
	fn static_refresh_messages_hard_reload_wins_over_weaker_actions() {
		let messages = static_refresh_messages(
			&StaticEffects {
				public_filemap_changed: true,
				critical_css_changed: true,
				includes_client_revalidate: true,
			},
			&StaticMetadata {
				critical_css: "body{color:red}".to_owned(),
				..StaticMetadata::default()
			},
		);

		assert_eq!(
			messages,
			vec![(ChangeType::HardReload, String::new(), String::new(),)],
		);
	}

	#[test]
	fn static_refresh_messages_hide_overlay_when_no_stronger_action_exists() {
		let messages =
			static_refresh_messages(&StaticEffects::default(), &StaticMetadata::default());

		assert_eq!(
			messages,
			vec![(
				ChangeType::HideRebuildingOverlay,
				String::new(),
				String::new(),
			)],
		);
	}

	#[test]
	fn handle_build_error_broadcasts_real_errors() {
		let root = temp_root("build-error-broadcast");
		let session = BuildSession::new(config(&root, "dist"), build_entry(), RunMode::Dev);
		let mut rx = session.runtime().add_client_for_test();

		handle_build_error(&session, "refresh", "boom");

		let msg = rx.try_recv().unwrap();
		assert_eq!(msg.change_type, ChangeType::ShowBuildError);
		assert_eq!(msg.build_error, "boom");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn handle_build_error_suppresses_cancelled_generations() {
		let root = temp_root("build-error-cancelled");
		let session = BuildSession::new(config(&root, "dist"), build_entry(), RunMode::Dev);
		let mut rx = session.runtime().add_client_for_test();

		session
			.runtime()
			.build_cancel()
			.store(true, Ordering::SeqCst);
		handle_build_error(&session, "refresh", "build cancelled");

		assert!(rx.try_recv().is_err());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn live_config_retry_transition_keeps_dev_lock_when_dist_dir_is_unchanged() {
		let root = temp_root("same-dist-lock");
		let bootstrap = config(&root, "dist");
		let mut next = config(&root, "dist");
		next.frontend_config.entry_file = "src/client/next-entry.tsx".to_owned();
		let mut session = BuildSession::new(bootstrap.clone(), build_entry(), RunMode::Dev);
		session
			.runtime_mut()
			.hold_dev_lock(acquired_dev_lock(&bootstrap));

		apply_live_config_retry_transition(&mut session, &bootstrap, &next).unwrap();

		assert!(session.runtime().has_dev_lock());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn live_config_retry_transition_swaps_dev_lock_and_removes_old_vorma_on_dist_change() {
		let root = temp_root("dist-lock-swap");
		let bootstrap = config(&root, "dist-a");
		let next = config(&root, "dist-b");
		let bootstrap_cfg = to_cfg(&bootstrap).unwrap();
		let old_vorma_out = PathBuf::from(bootstrap_cfg.vorma_out());
		fs::create_dir_all(&old_vorma_out).unwrap();
		fs::write(old_vorma_out.join("stale.txt"), "stale").unwrap();
		let mut session = BuildSession::new(bootstrap.clone(), build_entry(), RunMode::Dev);
		session
			.runtime_mut()
			.hold_dev_lock(acquired_dev_lock(&bootstrap));

		apply_live_config_retry_transition(&mut session, &bootstrap, &next).unwrap();

		assert!(session.runtime().has_dev_lock());
		assert!(!old_vorma_out.exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn static_build_failure_does_not_mutate_committed_generation() {
		let root = temp_root("static-failure-no-commit");
		fs::create_dir_all(root.join("public")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		fs::create_dir_all(root.join("src/client/vorma.gen.ts")).unwrap();
		let config = config(&root, "dist");
		let old_static = StaticMetadata {
			public_filemap: BTreeMap::from([("old.css".to_owned(), "/static/old.css".to_owned())]),
			critical_css: "old css".to_owned(),
			..StaticMetadata::default()
		};
		let mut session = committed_static_session(config, old_static.clone());

		process_static_build(&mut session, true).unwrap();

		assert_eq!(session.committed().unwrap().static_metadata(), &old_static);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cancelled_static_build_does_not_mutate_committed_generation() {
		let root = temp_root("static-cancel-no-commit");
		fs::create_dir_all(root.join("public")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		let config = config(&root, "dist");
		let old_static = StaticMetadata {
			public_filemap: BTreeMap::from([("old.css".to_owned(), "/static/old.css".to_owned())]),
			critical_css: "old css".to_owned(),
			..StaticMetadata::default()
		};
		let mut session = committed_static_session(config, old_static.clone());
		session
			.runtime()
			.build_cancel()
			.store(true, Ordering::SeqCst);

		process_static_build(&mut session, true).unwrap();

		assert_eq!(session.committed().unwrap().static_metadata(), &old_static);
		fs::remove_dir_all(root).unwrap();
	}
}
