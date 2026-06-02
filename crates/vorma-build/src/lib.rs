mod activation;
mod browser_sync;
mod build_cancel;
mod config;
mod constants;
mod cssbundle;
mod dev_loop;
mod dev_mux;
mod dev_watcher;
mod document_hash;
mod fswatcher;
mod generation;
mod globset;
mod live_refresh;
mod live_state;
mod local_cargo;
mod manifest;
mod pipeline;
mod process_wait;
mod runtime;
mod session;
mod signals;
mod static_build;
mod staticproc;
mod supervisor;
#[cfg(test)]
mod test_support;
mod ts_gen;
mod ts_modules;
mod utils;
mod vite_plugin;
mod viteutil;
mod watch_plan;
mod work_queue;

use std::process;

use vorma::{AppConfig, ViewErrorClientMsg};

use crate::config::CargoBinTarget;
use crate::session::BuildSession;
use crate::signals::start_signal_thread;

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct BuildOptions {
	// Build entry identity is startup/orchestrator state, not live app config. Do not
	// move it into `vorma::App` or normalized app config: the orchestrator needs to know
	// which Cargo target to execute before it can re-run the build entry in live-state
	// mode and read the app's live config. If the build-entry target changes, the outer
	// build/dev process must be restarted. Treating this as live config would create a
	// chicken/egg bug where the framework has to run the build entry in order to discover
	// which build entry it should have run.
	pub cargo_package: &'static str,
	pub cargo_bin: &'static str,
}

impl From<&BuildOptions> for CargoBinTarget {
	fn from(build_options: &BuildOptions) -> Self {
		Self {
			cargo_package: build_options.cargo_package.to_owned(),
			cargo_bin: build_options.cargo_bin.to_owned(),
		}
	}
}

pub fn run<S, E, F>(app_config: F, build_options: BuildOptions) -> Result<(), String>
where
	F: FnOnce() -> vorma::Result<AppConfig<S, E>>,
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	run_with_args(app_config, build_options, std::env::args().skip(1))
}

fn run_with_args<S, E, F, I>(
	app_config: F,
	build_options: BuildOptions,
	args: I,
) -> Result<(), String>
where
	F: FnOnce() -> vorma::Result<AppConfig<S, E>>,
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
	I: IntoIterator<Item = String>,
{
	set_build_env();

	let app_config = app_config().map_err(|err| err.to_string())?;
	let (config, views, resources, document) = vorma::__private::app_live_state_parts(app_config);

	if is_live_state_mode() {
		match live_state::get_live_state_from_app(&config, &views, &resources, &document) {
			Ok(live_state) => print_json_and_exit(&live_state, 0),
			Err(err) => print_err_json_and_exit("error getting live state", &err),
		}
	}

	let command = parse_build_command(args)?;
	let mut session =
		BuildSession::new(config, CargoBinTarget::from(&build_options), command.mode());
	let mut signal_thread = start_signal_thread(session.runtime().stop_handles());
	session.runtime().reset_build_cancel();
	if signal_thread.shutdown_requested()? {
		return Ok(());
	}
	let candidate = match match command {
		BuildCommand::Build => pipeline::prepare_generation_candidate(&mut session),
		BuildCommand::Dev => dev_loop::prepare_stable_server_refresh_candidate(&mut session),
	} {
		Ok(candidate) => candidate,
		Err(err) => {
			if signal_thread.shutdown_requested()? {
				return Ok(());
			}
			return Err(format!("Initialization error: {err}"));
		}
	};
	session.commit_generation(candidate);
	if signal_thread.shutdown_requested()? {
		return Ok(());
	}
	let committed = session
		.committed()
		.cloned()
		.ok_or_else(|| "committed generation not available".to_owned())?;

	let manifest =
		match activation::activate_generation(&committed, command.mode(), session.runtime_mut()) {
			Ok(manifest) => manifest,
			Err(err) => {
				if signal_thread.shutdown_requested()? {
					return Ok(());
				}
				return Err(format!("Initialization error: {err}"));
			}
		};
	let Some(manifest) = manifest else {
		return Ok(());
	};
	session.set_committed_manifest(manifest)?;
	if signal_thread.shutdown_requested()? {
		return Ok(());
	}
	if command == BuildCommand::Dev {
		return dev_loop::run_dev_loop(&mut session, &mut signal_thread);
	}
	session.runtime_mut().stop_dev_mux_server()?;
	Ok(())
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum BuildCommand {
	Build,
	Dev,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum RunMode {
	Build,
	Dev,
}

impl BuildCommand {
	fn mode(self) -> RunMode {
		match self {
			Self::Build => RunMode::Build,
			Self::Dev => RunMode::Dev,
		}
	}
}

fn parse_build_command(args: impl IntoIterator<Item = String>) -> Result<BuildCommand, String> {
	let args = args.into_iter().collect::<Vec<_>>();
	match args.as_slice() {
		[] => Ok(BuildCommand::Build),
		[arg] if arg == "dev" => Ok(BuildCommand::Dev),
		_ => Err("usage: build-entry [dev]".to_owned()),
	}
}

fn set_build_env() {
	// Rust 2024 marks environment mutation unsafe because concurrent env access can race.
	// Vorma's build entry calls this before the framework starts any worker threads.
	unsafe {
		std::env::set_var(vorma::__private::constants::ENV_KEY_IS_BUILD, "1");
	}
}

fn is_live_state_mode() -> bool {
	std::env::var_os("__VORMA_LIVE_STATE_MODE").is_some_and(|value| value == "1")
}

fn print_err_json_and_exit(outer_msg: &str, err: &str) -> ! {
	let error = serde_json::json!({
		"error": format!("{outer_msg}: {err}"),
	});
	print_json_and_exit(&error, 1)
}

fn print_json_and_exit<T: serde::Serialize>(value: &T, code: i32) -> ! {
	match utils::serialize_pretty_json(value)
		.and_then(|bytes| String::from_utf8(bytes).map_err(|err| err.to_string()))
	{
		Ok(json) => println!("{json}"),
		Err(_) => println!(r#"{{"error":"unknown error"}}"#),
	}
	process::exit(code);
}

#[cfg(test)]
mod tests {
	use super::*;
	use vorma::{
		DevWatchConfig, DocumentBuilder, FrontendConfig, Middlewares, PathConfig, Resources,
		ServerConfig, TasksOptions, TsGenConfig, Views,
	};

	#[test]
	fn parse_build_command_accepts_exact_empty_or_dev_argv() {
		assert_eq!(parse_build_command([]).unwrap(), BuildCommand::Build);
		assert_eq!(
			parse_build_command(["dev".to_owned()]).unwrap(),
			BuildCommand::Dev
		);
	}

	#[test]
	fn parse_build_command_rejects_dev_as_incidental_argument() {
		assert_eq!(
			parse_build_command(["--mode".to_owned(), "dev".to_owned()]).unwrap_err(),
			"usage: build-entry [dev]"
		);
		assert_eq!(
			parse_build_command(["prod".to_owned()]).unwrap_err(),
			"usage: build-entry [dev]"
		);
	}

	#[test]
	fn run_returns_initialization_errors_instead_of_panicking() {
		let error = run_with_args(
			|| {
				Ok(vorma::AppConfig::<(), String> {
					root_dir: crate::test_support::root_dir(),
					server_config: ServerConfig {
						cargo_package: "example-app".to_owned(),
						cargo_bin: "example-server".to_owned(),
					},
					dist_dir: "../outside-dist".to_owned(),
					path_config: PathConfig::default(),
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
					dev_watch_config: DevWatchConfig::default(),
					state: (),
					views: Views::new(),
					resources: Resources::new(),
					middlewares: Middlewares::new(),
					tasks_options: TasksOptions::default(),
					document: DocumentBuilder::default(),
					request_body_limit: 1024,
				})
			},
			BuildOptions {
				cargo_package: "example-app",
				cargo_bin: "example-build",
			},
			std::iter::empty(),
		)
		.unwrap_err();

		assert_eq!(
			error,
			"Initialization error: error with dist dir: dist_dir must be inside root_dir"
		);
	}
}
