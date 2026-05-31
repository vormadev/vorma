use std::process::{Command, Stdio};
use std::sync::Arc;
use std::sync::atomic::Ordering;

use vorma::__private::constants::PROD_TMP_VITE_MANIFEST_FILENAME;
use vorma::__private::manifest::Manifest;

use crate::RunMode;
use crate::config::{VormaCfg, relative_path, to_cfg};
use crate::constants::{VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY};
use crate::generation::CommittedGeneration;
use crate::manifest::{ManifestInput, write_manifest};
use crate::process_wait::{ChildWaitError, current_thread_runtime, wait_child_or_cancel};
use crate::runtime::DevRuntime;
use crate::supervisor::{clear_vorma_runtime_env, prepare_child_process};

pub(crate) fn activate_generation(
	committed: &CommittedGeneration,
	mode: RunMode,
	runtime: &mut DevRuntime,
) -> Result<Option<Manifest>, String> {
	match mode {
		RunMode::Build => activate_prod_generation(committed, runtime),
		RunMode::Dev => activate_dev_generation(committed, runtime).map(Some),
	}
}

pub(crate) fn publish_dev_generation_to_mux(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<(), String> {
	runtime.ensure_dev_mux_server()?;
	runtime.publish_dev_mux_generation(Some(committed.dev_mux_generation()));
	Ok(())
}

fn activate_dev_generation(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<Manifest, String> {
	publish_dev_generation_to_mux(committed, runtime)?;
	runtime
		.restart_watcher(committed)
		.map_err(|err| format!("error restarting filesystem watcher: {err}"))?;
	if runtime.vite_server_running() {
		runtime
			.send_vite_plugin_restart()
			.map_err(|err| format!("error sending restart command to Vite plugin: {err}"))?;
	}
	let vite_port = runtime
		.start_vite_server(committed)
		.map_err(|err| format!("error starting Vite server: {err}"))?;
	let cfg =
		to_cfg(committed.config()).map_err(|err| format!("error converting config: {err}"))?;
	let manifest = write_manifest(
		&cfg,
		&ManifestInput::from_generation_metadata(
			true,
			i32::from(vite_port),
			runtime.dev_mux_port_i32()?,
			runtime.dev_refresh_token()?,
			committed.live(),
			committed.static_metadata(),
		),
	)?;
	runtime
		.start_app_server(committed)
		.map_err(|err| format!("error starting app server: {err}"))?;
	Ok(manifest)
}

fn activate_prod_generation(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<Option<Manifest>, String> {
	activate_prod_generation_with(committed, runtime, run_cmd)
}

fn activate_prod_generation_with<F>(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
	run_command: F,
) -> Result<Option<Manifest>, String>
where
	F: FnOnce(Command, Arc<crate::build_cancel::BuildCancel>) -> Result<(), CommandRunError>,
{
	let cfg =
		to_cfg(committed.config()).map_err(|err| format!("error converting config: {err}"))?;
	runtime.ensure_dev_mux_server()?;
	runtime.publish_dev_mux_generation(Some(committed.dev_mux_generation()));
	let command =
		vite_prod_build_cmd(&cfg, runtime.dev_mux_port()?, &runtime.vite_plugin_token()?)?;
	match run_command(command, runtime.build_cancel()) {
		Ok(()) => {}
		Err(CommandRunError::Cancelled) => return Ok(None),
		Err(CommandRunError::Failed(err)) => {
			return Err(format!("Error running Vite production build: {err}"));
		}
	}
	if runtime.build_cancel().load(Ordering::SeqCst) {
		return Ok(None);
	}
	let manifest = write_manifest(
		&cfg,
		&ManifestInput::from_generation_metadata(
			false,
			0,
			0,
			String::new(),
			committed.live(),
			committed.static_metadata(),
		),
	)
	.map_err(|err| format!("Error writing manifest: {err}"))?;
	Ok(Some(manifest))
}

fn vite_prod_build_cmd(
	cfg: &VormaCfg<'_>,
	internal_dev_server_port: u16,
	vite_plugin_token: &str,
) -> Result<Command, String> {
	let args = vite_prod_build_args(cfg)?;
	let mut command = Command::new(&args[0]);
	clear_vorma_runtime_env(&mut command);
	command
		.args(&args[1..])
		.current_dir(cfg.js_package_manager_dir())
		.stdout(Stdio::inherit())
		.stderr(Stdio::inherit())
		.env(
			VITE_PLUGIN_SERVER_PORT_ENV_KEY,
			internal_dev_server_port.to_string(),
		)
		.env(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, vite_plugin_token);
	Ok(command)
}

fn vite_prod_build_args(cfg: &VormaCfg<'_>) -> Result<Vec<String>, String> {
	let out_dir = relative_path(cfg.js_package_manager_dir(), cfg.pub_out())
		.ok_or_else(|| "failed to get relative path for Vite outDir".to_owned())?;
	let mut args = cfg.js_package_manager_cmd_base();
	args.extend([
		"vite".to_owned(),
		"build".to_owned(),
		"--outDir".to_owned(),
		out_dir.to_string_lossy().into_owned(),
		"--assetsDir".to_owned(),
		".".to_owned(),
		"--manifest".to_owned(),
		PROD_TMP_VITE_MANIFEST_FILENAME.to_owned(),
		"--emptyOutDir".to_owned(),
		"false".to_owned(),
	]);
	let cfg_file = cfg.vite_config_file();
	if !cfg_file.is_empty() {
		args.extend(["--config".to_owned(), cfg_file]);
	}
	Ok(args)
}

#[derive(Debug)]
enum CommandRunError {
	Cancelled,
	Failed(String),
}

fn run_cmd(
	mut command: Command,
	build_cancel: Arc<crate::build_cancel::BuildCancel>,
) -> Result<(), CommandRunError> {
	prepare_child_process(&mut command);
	let runtime =
		current_thread_runtime().map_err(|err| CommandRunError::Failed(err.to_string()))?;
	runtime.block_on(async {
		let mut command = tokio::process::Command::from(command);
		let mut child = command
			.spawn()
			.map_err(|err| CommandRunError::Failed(err.to_string()))?;
		match wait_child_or_cancel(&mut child, &build_cancel).await {
			Ok(status) if status.success() => Ok(()),
			Ok(status) => Err(CommandRunError::Failed(format!("status {status}"))),
			Err(ChildWaitError::Cancelled) => Err(CommandRunError::Cancelled),
			Err(ChildWaitError::Wait(err)) => Err(CommandRunError::Failed(err.to_string())),
		}
	})
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::ffi::OsStr;
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::sync::atomic::Ordering;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::generation::{
		BuildArtifactMode, BuildArtifacts, GenerationCandidate, LiveMetadata, StaticEffects,
		StaticMetadata,
	};
	use crate::runtime::DevRuntime;
	use crate::viteutil::{ViteManifest, ViteManifestChunk};

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-activation-{name}-{nonce}"));
		fs::create_dir_all(root.join("frontend")).unwrap();
		fs::create_dir_all(root.join("public")).unwrap();
		root
	}

	fn server_config() -> ServerConfig {
		ServerConfig {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-server".to_owned(),
		}
	}

	fn config(root: PathBuf) -> Config {
		Config {
			root_dir: root,
			dist_dir: "dist".to_owned(),
			server_config: server_config(),
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

	fn committed_generation(config: Config) -> crate::generation::CommittedGeneration {
		GenerationCandidate {
			config,
			live: LiveMetadata {
				root_document_hash_source: "document-hash-source".to_owned(),
				..LiveMetadata::default()
			},
			static_metadata: StaticMetadata::default(),
			manifest: None,
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/example-build"),
				mode: BuildArtifactMode::Prod,
			},
			static_effects: StaticEffects::default(),
		}
		.commit()
	}

	fn write_tmp_vite_manifest(cfg: &VormaCfg<'_>) {
		let tmp = cfg.prod_tmp_vite_manifest_out();
		fs::create_dir_all(Path::new(&tmp).parent().unwrap()).unwrap();
		fs::write(tmp, "{}").unwrap();
	}

	fn write_prod_public_outputs_and_manifest(cfg: &VormaCfg<'_>) {
		let public_dir = PathBuf::from(cfg.pub_out());
		fs::create_dir_all(public_dir.join("assets")).unwrap();
		fs::write(public_dir.join("assets/entry.js"), "entry").unwrap();
		fs::write(public_dir.join("assets/wasm-wrapper.js"), "wrapper").unwrap();
		fs::write(public_dir.join("assets/vorma_client_wasm_bg.wasm"), "wasm").unwrap();
		let vite_manifest = ViteManifest::from(BTreeMap::from([
			(
				"src/client/entry.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
			(
				"src/wasm-wrapper.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/wasm-wrapper.js".to_owned(),
					assets: vec!["assets/vorma_client_wasm_bg.wasm".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"pkg/vorma_client_wasm_bg.wasm".to_owned(),
				ViteManifestChunk {
					src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
					file: "assets/vorma_client_wasm_bg.wasm".to_owned(),
					..ViteManifestChunk::default()
				},
			),
		]));
		fs::write(
			cfg.prod_tmp_vite_manifest_out(),
			serde_json::to_vec(&vite_manifest).unwrap(),
		)
		.unwrap();
	}

	#[test]
	fn vite_prod_build_args_use_expected_static_output_shape() {
		let root = temp_root("vite-prod-args");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();

		assert_eq!(
			vite_prod_build_args(&cfg).unwrap(),
			vec![
				"pnpm",
				"exec",
				"vite",
				"build",
				"--outDir",
				"dist/.vorma/static/public",
				"--assetsDir",
				".",
				"--manifest",
				PROD_TMP_VITE_MANIFEST_FILENAME,
				"--emptyOutDir",
				"false",
			]
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn vite_prod_build_cmd_uses_dir_args_and_internal_server_env() {
		let root = temp_root("vite-prod-cmd");
		let config = Config {
			root_dir: root.clone(),
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: "frontend".to_owned(),
				vite_config_file: "frontend/vite.config.ts".to_owned(),
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			..config(root.clone())
		};
		let cfg = to_cfg(&config).unwrap();

		let command = vite_prod_build_cmd(&cfg, 4321, "secret").unwrap();

		assert_eq!(command.get_program(), OsStr::new("pnpm"));
		assert_eq!(
			command.get_args().collect::<Vec<_>>(),
			vec![
				OsStr::new("exec"),
				OsStr::new("vite"),
				OsStr::new("build"),
				OsStr::new("--outDir"),
				OsStr::new("../dist/.vorma/static/public"),
				OsStr::new("--assetsDir"),
				OsStr::new("."),
				OsStr::new("--manifest"),
				OsStr::new(PROD_TMP_VITE_MANIFEST_FILENAME),
				OsStr::new("--emptyOutDir"),
				OsStr::new("false"),
				OsStr::new("--config"),
				OsStr::new("vite.config.ts"),
			],
		);
		assert_eq!(
			command.get_current_dir(),
			Some(Path::new(&cfg.js_package_manager_dir()))
		);
		assert_eq!(
			command
				.get_envs()
				.find(|(key, _)| *key == VITE_PLUGIN_SERVER_PORT_ENV_KEY)
				.and_then(|(_, value)| value)
				.unwrap(),
			"4321",
		);
		assert_eq!(
			command
				.get_envs()
				.find(|(key, _)| *key == VITE_PLUGIN_SERVER_TOKEN_ENV_KEY)
				.and_then(|(_, value)| value)
				.unwrap(),
			"secret",
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn prod_activation_skips_manifest_when_vite_is_cancelled() {
		let root = temp_root("prod-cancelled");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		write_tmp_vite_manifest(&cfg);
		let manifest_out = cfg.manifest_json_out(false);
		let tmp_manifest_out = cfg.prod_tmp_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let result = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			Err(CommandRunError::Cancelled)
		})
		.unwrap();

		assert!(result.is_none());
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn prod_activation_skips_manifest_when_cancelled_after_vite_success() {
		let root = temp_root("prod-cancelled-after-success");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		write_tmp_vite_manifest(&cfg);
		let manifest_out = cfg.manifest_json_out(false);
		let tmp_manifest_out = cfg.prod_tmp_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let result = activate_prod_generation_with(&committed, &mut runtime, |_cmd, cancel| {
			cancel.store(true, Ordering::SeqCst);
			Ok(())
		})
		.unwrap();

		assert!(result.is_none());
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn prod_activation_keeps_manifest_unpublished_when_vite_fails() {
		let root = temp_root("prod-vite-fails");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		write_tmp_vite_manifest(&cfg);
		let manifest_out = cfg.manifest_json_out(false);
		let tmp_manifest_out = cfg.prod_tmp_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let error = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			Err(CommandRunError::Failed("vite exploded".to_owned()))
		})
		.unwrap_err();

		assert_eq!(error, "Error running Vite production build: vite exploded");
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn prod_activation_runs_vite_then_publishes_manifest() {
		let root = temp_root("prod-publishes-manifest");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		write_prod_public_outputs_and_manifest(&cfg);
		let manifest_out = cfg.manifest_json_out(false);
		let tmp_manifest_out = cfg.prod_tmp_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();
		let mut ran_vite = false;

		let manifest = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			ran_vite = true;
			Ok(())
		})
		.unwrap()
		.expect("successful prod activation should publish manifest");

		assert!(ran_vite);
		assert_eq!(manifest.client_entry.url, "/static/assets/entry.js");
		assert!(Path::new(&manifest_out).exists());
		assert!(!Path::new(&tmp_manifest_out).exists());
		fs::remove_dir_all(root).unwrap();
	}
}
