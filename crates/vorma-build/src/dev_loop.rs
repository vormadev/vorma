use std::sync::mpsc::TryRecvError;

use crate::browser_sync::ChangeType;
use crate::dev_lifecycle::DevLifecycle;
use crate::generation::{GenerationCandidate, StaticEffects};
use crate::session::BuildSession;
use crate::signals::SignalThread;
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
	match DevLifecycle::new(session).prepare_refresh_with_app_server_stopped() {
		Ok(candidate) => {
			let published = DevLifecycle::new(session)
				.activate_generation(candidate)
				.map_err(|err| err.to_string())?;
			broadcast_static_effects(
				session.runtime(),
				&published.effects,
				&published.static_metadata,
			);
			Ok(())
		}
		Err(err) => {
			let err = err.to_string();
			handle_build_error(session, "refresh", &err);
			Ok(())
		}
	}
}

pub(crate) fn prepare_stable_server_refresh_candidate(
	session: &mut BuildSession,
) -> Result<GenerationCandidate, String> {
	DevLifecycle::new(session)
		.prepare_refresh()
		.map_err(|err| err.to_string())
}

fn process_static_build(
	session: &mut BuildSession,
	includes_client_revalidate: bool,
) -> Result<(), String> {
	session
		.runtime()
		.broadcast_refresh(ChangeType::ShowRebuildingOverlay, "", "");
	let published =
		match DevLifecycle::new(session).publish_static_update(includes_client_revalidate) {
			Ok(published) => published,
			Err(err) => {
				let err = err.to_string();
				handle_build_error(session, "static build", &err);
				return Ok(());
			}
		};
	broadcast_static_effects(
		session.runtime(),
		&published.effects,
		&published.static_metadata,
	);
	Ok(())
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
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::sync::atomic::Ordering;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use crate::RunMode;
	use crate::browser_sync::ChangeType;
	use crate::cargo_target::CargoBinTarget;
	use crate::generation::{StaticEffects, StaticMetadata};
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
	fn server_refresh_prepare_error_reports_build_error_without_committing_generation() {
		let root = temp_root("refresh-prepare-error");
		let mut bad_config = config(&root, "dist");
		bad_config.frontend_config.public_static_src_dir = ".".to_owned();
		let mut session = BuildSession::new(bad_config, build_entry(), RunMode::Dev);
		let mut rx = session.runtime().add_client_for_test();

		process_server_refresh(&mut session).unwrap();

		let overlay = rx.try_recv().unwrap();
		assert_eq!(overlay.change_type, ChangeType::ShowRebuildingOverlay);
		let build_error = rx.try_recv().unwrap();
		assert_eq!(build_error.change_type, ChangeType::ShowBuildError);
		assert!(
			build_error
				.build_error
				.contains("frontend_config.public_static_src_dir cannot be .")
		);
		assert!(session.committed().is_none());
		fs::remove_dir_all(root).unwrap();
	}
}
