//! Complete production build orchestration.

use vorma::build_interface::AppBuildContract;
use vorma::build_interface::assets::ManifestMode;

use crate::build_output::{
	BuildOutputWriteError, TypeScriptOutputWriteReport, write_typescript_contracts,
};
use crate::generation_epoch::{CommittedGeneration, EpochSupervisor};
use crate::generation_inputs::{BuildInputError, prepare_build_inputs};
use crate::output_lock::{OutputLayoutLock, OutputLayoutLockError};
use crate::process_runner::{BuildProcessRunner, StdBuildProcessRunner};
use crate::production_generation::{
	ProductionGenerationError, ProductionOutputPublishReport,
	prepare_production_generation_from_build_inputs,
	publish_and_activate_prepared_production_generation,
};
use crate::production_vite::{
	ProductionViteError, ViteProductionBuildInput, run_vite_production_build,
};
use crate::tokens::{TokenGenerationError, generate_vite_plugin_token};
use crate::vite_plugin_rpc::{
	LoopbackVitePluginRpcServer, VitePluginRpcServer, VitePluginRpcState, VitePluginServerError,
};

/// Complete production build report.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProductionBuildReport {
	prebuild_typescript: TypeScriptOutputWriteReport,
	output_publish: ProductionOutputPublishReport,
}

impl ProductionBuildReport {
	/// Generated TypeScript written before Vite ran.
	#[cfg(test)]
	pub fn prebuild_typescript(&self) -> &TypeScriptOutputWriteReport {
		&self.prebuild_typescript
	}

	/// Final static/runtime output publication report.
	#[cfg(test)]
	pub fn output_publish(&self) -> &ProductionOutputPublishReport {
		&self.output_publish
	}
}

/// Run a complete production build using Vorma's standard loopback Vite plugin server.
pub async fn build_and_activate_production_generation_on_loopback(
	supervisor: &mut EpochSupervisor,
	app: AppBuildContract,
	mode: ManifestMode,
) -> Result<(&CommittedGeneration, ProductionBuildReport), ProductionBuildError> {
	let token = generate_vite_plugin_token()
		.map_err(|source| ProductionBuildError::RandomToken { source })?;
	let mut server = LoopbackVitePluginRpcServer;
	let mut runner = StdBuildProcessRunner;
	build_and_activate_production_generation(supervisor, app, token, &mut server, &mut runner, mode)
		.await
}

/// Run a complete production build and commit the resulting generation.
pub async fn build_and_activate_production_generation<'a>(
	supervisor: &'a mut EpochSupervisor,
	app: AppBuildContract,
	vite_plugin_token: impl Into<String>,
	vite_plugin_server: &mut impl VitePluginRpcServer,
	process_runner: &mut impl BuildProcessRunner,
	mode: ManifestMode,
) -> Result<(&'a CommittedGeneration, ProductionBuildReport), ProductionBuildError> {
	let vite_plugin_token = vite_plugin_token.into();
	if vite_plugin_token.is_empty() {
		return Err(ProductionBuildError::EmptyVitePluginToken);
	}
	let build_inputs = prepare_build_inputs(&app)
		.map_err(|source| ProductionBuildError::BuildInputs { source })?;
	let output_layout_lock = OutputLayoutLock::acquire_for_workspace_output_layout(
		build_inputs.plan().workspace().root_dir(),
		build_inputs.plan().workspace().dist_dir(),
	)
	.map_err(|source| ProductionBuildError::OutputLayoutLock { source })?;
	let prebuild_typescript =
		write_typescript_contracts(build_inputs.plan(), build_inputs.typescript_contracts())
			.map_err(|source| ProductionBuildError::TypeScriptOutput { source })?;
	let rpc_state =
		VitePluginRpcState::from_prepared_build_inputs(&vite_plugin_token, &build_inputs);
	let rpc_server = vite_plugin_server
		.start_vite_plugin_rpc_server(rpc_state)
		.map_err(|source| ProductionBuildError::VitePluginServer { source })?;
	let vite_manifest = run_vite_production_build(
		build_inputs.plan(),
		&ViteProductionBuildInput::new(rpc_server.port(), &vite_plugin_token),
		process_runner,
	)
	.map_err(|source| ProductionBuildError::Vite { source })?;
	let prepared = prepare_production_generation_from_build_inputs(build_inputs, &vite_manifest)
		.map_err(|source| ProductionBuildError::Generation { source })?;
	let (committed, output_publish) =
		publish_and_activate_prepared_production_generation(supervisor, app, &prepared, mode)
			.await
			.map_err(|source| ProductionBuildError::Generation { source })?;
	output_layout_lock
		.release()
		.map_err(|source| ProductionBuildError::OutputLayoutLock { source })?;
	Ok((
		committed,
		ProductionBuildReport {
			prebuild_typescript,
			output_publish,
		},
	))
}

/// Complete production build error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProductionBuildError {
	/// Secure Vite plugin token generation failed.
	RandomToken {
		/// Source token generation error.
		source: TokenGenerationError,
	},
	/// Vite plugin token was empty.
	EmptyVitePluginToken,
	/// Output-layout lock acquisition or release failed.
	OutputLayoutLock {
		/// Source output-layout lock error.
		source: OutputLayoutLockError,
	},
	/// Shared build input preparation failed.
	BuildInputs {
		/// Source build input error.
		source: BuildInputError,
	},
	/// Generated TypeScript prebuild output failed.
	TypeScriptOutput {
		/// Source output write error.
		source: BuildOutputWriteError,
	},
	/// Vite plugin RPC server could not start.
	VitePluginServer {
		/// Source Vite plugin server error.
		source: VitePluginServerError,
	},
	/// Vite production build failed.
	Vite {
		/// Source Vite production error.
		source: ProductionViteError,
	},
	/// Generation preparation, publication, or activation failed.
	Generation {
		/// Source production generation error.
		source: ProductionGenerationError,
	},
}

impl std::fmt::Display for ProductionBuildError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::RandomToken { source } => write!(f, "{source}"),
			Self::EmptyVitePluginToken => f.write_str("Vite plugin token cannot be empty"),
			Self::OutputLayoutLock { source } => write!(f, "{source}"),
			Self::BuildInputs { source } => write!(f, "{source}"),
			Self::TypeScriptOutput { source } => write!(f, "{source}"),
			Self::VitePluginServer { source } => write!(f, "{source}"),
			Self::Vite { source } => write!(f, "{source}"),
			Self::Generation { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for ProductionBuildError {}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::PathBuf;

	use http::StatusCode;

	use super::*;
	use crate::process_runner::{BuildProcessCommand, BuildProcessError};
	use crate::production_vite::production_vite_manifest_temp_path;
	use crate::test_support::unique_temp_root_with_dirs;
	use crate::vite_manifest::{CLIENT_CORE_WASM_SOURCE_FILENAME, ViteManifestChunk};
	use crate::vite_plugin_contract::{
		VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, VITE_PLUGIN_TOKEN_HEADER,
	};
	use crate::vite_plugin_rpc::VitePluginRpcServerHandle;

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_ENTRY_FILE: &str = "src/entry.tsx";
	const TEST_CRITICAL_CSS_FILE: &str = "src/critical.css";
	const TEST_TOKEN: &str = "token";

	fn temp_root() -> PathBuf {
		unique_temp_root_with_dirs("vorma-build-prod", &[TEST_PUBLIC_STATIC_SOURCE_DIR, "src"])
	}

	fn app_build_contract(root_dir: PathBuf) -> AppBuildContract {
		vorma::build_interface::app_build_contract(vorma::AppConfig {
			root_dir,
			server_target: vorma::ServerTarget {
				cargo_package: TEST_CARGO_PACKAGE.to_owned(),
				cargo_bin: TEST_CARGO_BIN.to_owned(),
			},
			dist_dir: TEST_DIST_DIR.to_owned(),
			public_static_base: TEST_PUBLIC_STATIC_BASE.to_owned(),
			frontend_config: vorma::FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: TEST_ENTRY_FILE.to_owned(),
				public_static_src_dir: TEST_PUBLIC_STATIC_SOURCE_DIR.to_owned(),
				critical_css_file: TEST_CRITICAL_CSS_FILE.to_owned(),
			},
			ts_gen_config: vorma::TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				..vorma::TsGenConfig::default()
			},
			..vorma::AppConfig::<()>::default()
		})
		.unwrap()
	}

	#[derive(Debug, Default)]
	struct FakeVitePluginServer {
		state: Option<VitePluginRpcState>,
	}

	impl VitePluginRpcServer for FakeVitePluginServer {
		fn start_vite_plugin_rpc_server(
			&mut self,
			state: VitePluginRpcState,
		) -> Result<VitePluginRpcServerHandle, VitePluginServerError> {
			self.state = Some(state);
			Ok(VitePluginRpcServerHandle::new(4321))
		}
	}

	#[derive(Debug)]
	struct FakeProcessRunner {
		command: Option<BuildProcessCommand>,
		manifest_path: PathBuf,
	}

	impl BuildProcessRunner for FakeProcessRunner {
		fn run_inheriting_stdio(
			&mut self,
			command: &BuildProcessCommand,
		) -> Result<(), BuildProcessError> {
			self.command = Some(command.clone());
			fs::create_dir_all(self.manifest_path.parent().unwrap()).unwrap();
			let wasm_file = format!("assets/{CLIENT_CORE_WASM_SOURCE_FILENAME}");
			let manifest = BTreeMap::from([
				(
					TEST_ENTRY_FILE.to_owned(),
					ViteManifestChunk {
						file: "assets/entry.js".to_owned(),
						..ViteManifestChunk::default()
					},
				),
				(
					"src/wasm-wrapper.ts".to_owned(),
					ViteManifestChunk {
						file: "assets/wasm-wrapper.js".to_owned(),
						assets: vec![wasm_file.clone()],
						..ViteManifestChunk::default()
					},
				),
				(
					"pkg/vorma_client_wasm_bg.wasm".to_owned(),
					ViteManifestChunk {
						src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
						file: wasm_file,
						..ViteManifestChunk::default()
					},
				),
			]);
			fs::write(&self.manifest_path, serde_json::to_vec(&manifest).unwrap()).unwrap();
			Ok(())
		}
	}

	#[tokio::test]
	async fn production_build_writes_typescript_runs_vite_publishes_and_commits() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg></svg>",
		)
		.unwrap();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let preflight_inputs = prepare_build_inputs(&app).unwrap();
		let manifest_path = production_vite_manifest_temp_path(preflight_inputs.plan()).unwrap();
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner {
			command: None,
			manifest_path,
		};
		let mut supervisor = EpochSupervisor::default();

		let (committed, report) = build_and_activate_production_generation(
			&mut supervisor,
			app,
			TEST_TOKEN,
			&mut server,
			&mut runner,
			ManifestMode::Prod,
		)
		.await
		.unwrap();

		let state = server.state.as_ref().unwrap();
		let mut headers = http::HeaderMap::new();
		headers.insert(VITE_PLUGIN_TOKEN_HEADER, TEST_TOKEN.parse().unwrap());
		let hash_response =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);
		let command = runner.command.as_ref().unwrap();
		let committed_id = committed.id();
		let committed_ui_variant = committed.manifest().ui_variant().to_owned();

		assert_eq!(hash_response.status(), StatusCode::OK);
		assert_eq!(
			command.env().get(VITE_PLUGIN_SERVER_PORT_ENV_KEY).unwrap(),
			"4321"
		);
		assert_eq!(
			command.env().get(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY).unwrap(),
			TEST_TOKEN
		);
		assert!(report.prebuild_typescript().path().exists());
		assert!(
			report
				.output_publish()
				.generation_outputs()
				.manifest_path()
				.exists()
		);
		assert_eq!(committed_ui_variant, "react");
		assert_eq!(supervisor.committed().unwrap().id(), committed_id);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn vite_plugin_token_generation_returns_lowercase_opaque_token() {
		let token = generate_vite_plugin_token().unwrap();

		assert_eq!(token.len(), 52);
		assert!(
			token
				.chars()
				.all(|value| value.is_ascii_lowercase() || value.is_ascii_digit())
		);
	}
}
