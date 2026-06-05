use std::fmt;
use std::io;
use std::process::Command;
use std::sync::Arc;
use std::sync::atomic::Ordering;

use path_slash::PathExt;
use vorma::__private::manifest::Manifest;

use crate::RunMode;
use crate::build_layout::{RetainedViteManifest, promote_prod_tmp_vite_manifest};
use crate::command_runner::{CommandRunError, run_command_inheriting_stdio};
use crate::config::{ConfigError, VormaCfg, relative_path, to_cfg};
use crate::constants::{VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY};
use crate::generation::CommittedGeneration;
use crate::manifest::{
	DevManifestInput, ProdManifestInput, write_dev_manifest, write_prod_manifest,
};
use crate::runtime::DevRuntime;
use crate::supervisor::clear_vorma_runtime_env;

pub(crate) fn activate_generation(
	committed: &CommittedGeneration,
	mode: RunMode,
	runtime: &mut DevRuntime,
) -> Result<ActivationOutcome, ActivationError> {
	match mode {
		RunMode::Build => activate_prod_generation(committed, runtime),
		RunMode::Dev => activate_dev_generation(committed, runtime)
			.map(|manifest| ActivationOutcome::Published(Box::new(manifest))),
	}
}

#[derive(Debug)]
pub(crate) enum ActivationOutcome {
	Published(Box<Manifest>),
	Cancelled,
}

#[derive(Debug)]
pub(crate) enum ActivationError {
	Config {
		phase: &'static str,
		source: ConfigError,
	},
	DevRuntime {
		phase: &'static str,
		source: String,
	},
	ManifestWrite {
		phase: &'static str,
		source: String,
	},
	AppServerStart {
		source: String,
	},
	ViteBuildCommand {
		source: String,
	},
	ViteBuild {
		source: CommandRunError,
	},
	RetainViteManifest {
		source: io::Error,
	},
}

impl fmt::Display for ActivationError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Config { phase, source } => {
				write!(f, "error converting config for {phase}: {source}")
			}
			Self::DevRuntime { phase, source } => {
				write!(f, "error preparing dev runtime for {phase}: {source}")
			}
			Self::ManifestWrite { phase, source } => {
				write!(f, "error writing {phase} manifest: {source}")
			}
			Self::AppServerStart { source } => write!(f, "error starting app server: {source}"),
			Self::ViteBuildCommand { source } => {
				write!(f, "error preparing Vite production build command: {source}")
			}
			Self::ViteBuild { source } => {
				write!(f, "error running Vite production build: {source}")
			}
			Self::RetainViteManifest { source } => {
				write!(f, "error retaining Vite manifest: {source}")
			}
		}
	}
}

impl std::error::Error for ActivationError {}

fn activate_dev_generation(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<Manifest, ActivationError> {
	let dev_runtime = runtime
		.prepare_dev_generation_runtime_for_manifest(committed)
		.map_err(|source| ActivationError::DevRuntime {
			phase: "dev manifest",
			source,
		})?;
	let cfg = to_cfg(committed.config()).map_err(|source| ActivationError::Config {
		phase: "dev activation",
		source,
	})?;
	let manifest = write_dev_manifest(
		&cfg,
		&DevManifestInput::from_generation_metadata(
			dev_runtime.vite_server_port,
			dev_runtime.dev_mux_port,
			dev_runtime.dev_refresh_token,
			committed.live(),
			committed.static_metadata(),
		),
	)
	.map_err(|source| ActivationError::ManifestWrite {
		phase: "dev",
		source,
	})?;
	runtime
		.start_app_server(committed)
		.map_err(|source| ActivationError::AppServerStart { source })?;
	Ok(manifest)
}

fn activate_prod_generation(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<ActivationOutcome, ActivationError> {
	activate_prod_generation_with(committed, runtime, run_cmd)
}

fn activate_prod_generation_with<F>(
	committed: &CommittedGeneration,
	runtime: &mut DevRuntime,
	run_command: F,
) -> Result<ActivationOutcome, ActivationError>
where
	F: FnOnce(Command, Arc<crate::build_cancel::BuildCancel>) -> Result<(), CommandRunError>,
{
	let cfg = publish_generation_for_vite_config(committed, runtime)?;
	match run_vite_prod_build(&cfg, runtime, run_command)? {
		ProdViteBuildOutcome::Cancelled => Ok(ActivationOutcome::Cancelled),
		ProdViteBuildOutcome::Succeeded => {
			let prod_vite_manifest = promote_prod_vite_manifest(&cfg)?;
			let manifest = write_prod_generation_manifest(&cfg, committed, prod_vite_manifest)?;
			Ok(ActivationOutcome::Published(Box::new(manifest)))
		}
	}
}

fn publish_generation_for_vite_config<'a>(
	committed: &'a CommittedGeneration,
	runtime: &mut DevRuntime,
) -> Result<VormaCfg<'a>, ActivationError> {
	let cfg = to_cfg(committed.config()).map_err(|source| ActivationError::Config {
		phase: "Vite config publication",
		source,
	})?;
	runtime
		.ensure_dev_mux_server()
		.map_err(|source| ActivationError::DevRuntime {
			phase: "Vite config publication",
			source,
		})?;
	runtime.publish_dev_mux_generation(committed.dev_mux_generation());
	Ok(cfg)
}

fn run_vite_prod_build<F>(
	cfg: &VormaCfg<'_>,
	runtime: &mut DevRuntime,
	run_command: F,
) -> Result<ProdViteBuildOutcome, ActivationError>
where
	F: FnOnce(Command, Arc<crate::build_cancel::BuildCancel>) -> Result<(), CommandRunError>,
{
	let dev_mux_port = runtime
		.dev_mux_port()
		.map_err(|source| ActivationError::DevRuntime {
			phase: "Vite production build",
			source,
		})?;
	let vite_plugin_token =
		runtime
			.vite_plugin_token()
			.map_err(|source| ActivationError::DevRuntime {
				phase: "Vite production build",
				source,
			})?;
	let command = vite_prod_build_cmd(cfg, dev_mux_port, &vite_plugin_token)?;
	match run_command(command, runtime.build_cancel()) {
		Ok(()) => {}
		Err(CommandRunError::Cancelled) => return Ok(ProdViteBuildOutcome::Cancelled),
		Err(source) => {
			return Err(ActivationError::ViteBuild { source });
		}
	}
	if runtime.build_cancel().load(Ordering::SeqCst) {
		return Ok(ProdViteBuildOutcome::Cancelled);
	}
	Ok(ProdViteBuildOutcome::Succeeded)
}

fn promote_prod_vite_manifest(cfg: &VormaCfg<'_>) -> Result<RetainedViteManifest, ActivationError> {
	promote_prod_tmp_vite_manifest(cfg.build_layout())
		.map_err(|source| ActivationError::RetainViteManifest { source })
}

fn write_prod_generation_manifest(
	cfg: &VormaCfg<'_>,
	committed: &CommittedGeneration,
	prod_vite_manifest: RetainedViteManifest,
) -> Result<Manifest, ActivationError> {
	let input = ProdManifestInput::from_generation_metadata(
		prod_vite_manifest,
		committed.live(),
		committed.static_metadata(),
	);
	write_prod_manifest(cfg, &input).map_err(|source| ActivationError::ManifestWrite {
		phase: "production",
		source,
	})
}

#[derive(Debug, Eq, PartialEq)]
enum ProdViteBuildOutcome {
	Succeeded,
	Cancelled,
}

fn vite_prod_build_cmd(
	cfg: &VormaCfg<'_>,
	internal_dev_server_port: u16,
	vite_plugin_token: &str,
) -> Result<Command, ActivationError> {
	let args = vite_prod_build_args(cfg)?;
	let mut command = Command::new(&args[0]);
	clear_vorma_runtime_env(&mut command);
	command
		.args(&args[1..])
		.current_dir(cfg.js_package_manager_dir())
		.env(
			VITE_PLUGIN_SERVER_PORT_ENV_KEY,
			internal_dev_server_port.to_string(),
		)
		.env(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, vite_plugin_token);
	Ok(command)
}

fn vite_prod_build_args(cfg: &VormaCfg<'_>) -> Result<Vec<String>, ActivationError> {
	let out_dir = relative_path(cfg.js_package_manager_dir(), cfg.pub_out()).ok_or_else(|| {
		ActivationError::ViteBuildCommand {
			source: "failed to get relative path for Vite outDir".to_owned(),
		}
	})?;
	let vite_manifest =
		relative_path(cfg.pub_out(), cfg.prod_tmp_vite_manifest_out()).ok_or_else(|| {
			ActivationError::ViteBuildCommand {
				source: "failed to get relative path for Vite manifest".to_owned(),
			}
		})?;
	let mut args = cfg.js_package_manager_cmd_base();
	args.extend([
		"vite".to_owned(),
		"build".to_owned(),
		"--outDir".to_owned(),
		out_dir.to_string_lossy().into_owned(),
		"--assetsDir".to_owned(),
		".".to_owned(),
		"--manifest".to_owned(),
		vite_manifest.to_slash_lossy().into_owned(),
		"--emptyOutDir".to_owned(),
		"false".to_owned(),
	]);
	let cfg_file = cfg.vite_config_file();
	if !cfg_file.is_empty() {
		args.extend(["--config".to_owned(), cfg_file]);
	}
	Ok(args)
}

fn run_cmd(
	command: Command,
	build_cancel: Arc<crate::build_cancel::BuildCancel>,
) -> Result<(), CommandRunError> {
	run_command_inheriting_stdio(command, &build_cancel)
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
				ui_variant: vorma::UiVariant::React,
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
		let tmp = cfg.prod_tmp_vite_manifest_out();
		fs::create_dir_all(Path::new(&tmp).parent().unwrap()).unwrap();
		fs::write(tmp, serde_json::to_vec(&vite_manifest).unwrap()).unwrap();
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
				"tmp/vorma_internal_tmp_vite_manifest.json",
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
				ui_variant: vorma::UiVariant::React,
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
				OsStr::new("tmp/vorma_internal_tmp_vite_manifest.json"),
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
		let vite_manifest_out = cfg.build_layout().prod_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let result = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			Err(CommandRunError::Cancelled)
		})
		.unwrap();

		assert!(matches!(result, ActivationOutcome::Cancelled));
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		assert!(!vite_manifest_out.exists());
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
		let vite_manifest_out = cfg.build_layout().prod_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let result = activate_prod_generation_with(&committed, &mut runtime, |_cmd, cancel| {
			cancel.store(true, Ordering::SeqCst);
			Ok(())
		})
		.unwrap();

		assert!(matches!(result, ActivationOutcome::Cancelled));
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		assert!(!vite_manifest_out.exists());
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
		let vite_manifest_out = cfg.build_layout().prod_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();

		let error = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			Err(CommandRunError::CommandWait {
				source: std::io::Error::other("vite exploded"),
			})
		})
		.unwrap_err();

		assert_eq!(
			error.to_string(),
			"error running Vite production build: wait for command: vite exploded"
		);
		assert!(!Path::new(&manifest_out).exists());
		assert!(Path::new(&tmp_manifest_out).exists());
		assert!(!vite_manifest_out.exists());
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
		let tmp_manifest_dir = cfg.build_layout().prod_tmp_vite_manifest_dir();
		let vite_manifest_out = cfg.build_layout().prod_vite_manifest_out();
		let committed = committed_generation(config);
		let mut runtime = DevRuntime::new();
		let mut ran_vite = false;

		let result = activate_prod_generation_with(&committed, &mut runtime, |_cmd, _cancel| {
			ran_vite = true;
			Ok(())
		})
		.unwrap();
		let ActivationOutcome::Published(manifest) = result else {
			panic!("successful prod activation should publish manifest");
		};
		let manifest = *manifest;

		assert!(ran_vite);
		assert_eq!(manifest.client_entry.url, "/static/assets/entry.js");
		assert!(
			!manifest
				.public_filepaths
				.iter()
				.any(|path| path.contains("vorma_internal_tmp_vite_manifest.json"))
		);
		assert!(Path::new(&manifest_out).exists());
		assert!(!Path::new(&tmp_manifest_out).exists());
		assert!(vite_manifest_out.exists());
		assert!(!tmp_manifest_dir.exists());
		fs::remove_dir_all(root).unwrap();
	}
}
