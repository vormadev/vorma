//! Development generation assembly and activation.
//!
//! Where dev builds diverge from production: view/entry client modules
//! resolve to `http://127.0.0.1:<vite_server_port>/...` URLs served by the
//! running Vite dev server instead of production's built/hashed bundle
//! URLs, and the runtime manifest carries dev-only metadata (the Vite and
//! dev-mux ports, the dev refresh token) production manifests never have.
//! See [`crate::production_generation`] for the production counterpart.

use std::collections::BTreeMap;
use std::path::PathBuf;

use vorma::build_interface::AppBuildContract;
use vorma::build_interface::assets::ManifestMode;

use crate::build_output::{
	BuildOutputWriteError, BuildOutputWriteReport, write_generation_candidate_outputs,
};
use crate::build_plan::BuildProjectionPlan;
use crate::generation_epoch::{
	CommittedGeneration, EpochSupervisor, GenerationCandidate, GenerationError,
};
use crate::generation_inputs::PreparedBuildInputs;
use crate::projection_compiler::{
	ClientModuleArtifacts, CompletedBuildArtifacts, DevManifestMetadata, GenerationArtifacts,
	ProjectionBundle, ViewModuleContract,
};
use crate::static_outputs::{
	PreparedPublicStaticOutputs, PublicStaticOutputError, PublicStaticPublishReport,
	publish_public_static_outputs,
};
use crate::typescript_contracts::GeneratedTypeScriptContracts;
use crate::vite_plugin_contract::{VitePluginConfigError, package_manager_relative_path};

const DEV_LOOPBACK_HOST: &str = "127.0.0.1";

/// Dev runtime ports and refresh token: the live facts a generation needs
/// to project dev-only client module URLs and manifest metadata (see the
/// module docs above). Ports are `i32` rather than `u16` because they carry
/// straight into [`vorma_contract::runtime_manifest::RuntimeManifest`]'s
/// own `i32` port fields, which the browser-facing JSON manifest and
/// generated TypeScript read as plain numbers.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevRuntimeInputs {
	vite_server_port: i32,
	dev_mux_port: i32,
	dev_refresh_token: String,
}

impl DevRuntimeInputs {
	/// Create dev runtime inputs.
	pub fn new(
		vite_server_port: i32,
		dev_mux_port: i32,
		dev_refresh_token: impl Into<String>,
	) -> Self {
		Self {
			vite_server_port,
			dev_mux_port,
			dev_refresh_token: dev_refresh_token.into(),
		}
	}

	/// Dev Vite server port.
	pub fn vite_server_port(&self) -> i32 {
		self.vite_server_port
	}

	/// Dev mux server port.
	pub fn dev_mux_port(&self) -> i32 {
		self.dev_mux_port
	}

	/// Dev browser refresh token.
	pub fn dev_refresh_token(&self) -> &str {
		&self.dev_refresh_token
	}
}

/// Prepared dev generation artifacts before activation: everything a
/// generation candidate needs, assembled but not yet built into a candidate
/// or published to disk. Kept as a distinct step from activation so a dev
/// session can prepare once and activate multiple times against different
/// live app-build contracts (used when the app server binary itself did
/// not need to rebuild, only the graph reread).
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedDevGeneration {
	public_static_outputs: PreparedPublicStaticOutputs,
	projection_bundle: ProjectionBundle,
	build_plan: BuildProjectionPlan,
	typescript_contracts: GeneratedTypeScriptContracts,
	artifacts: CompletedBuildArtifacts,
}

impl PreparedDevGeneration {
	/// Prepared public static outputs.
	pub fn public_static_outputs(&self) -> &PreparedPublicStaticOutputs {
		&self.public_static_outputs
	}

	/// Compiled projection bundle used to assemble this generation.
	pub fn projection_bundle(&self) -> &ProjectionBundle {
		&self.projection_bundle
	}

	/// Build projection plan used to assemble this generation.
	pub fn build_plan(&self) -> &BuildProjectionPlan {
		&self.build_plan
	}

	/// Generated TypeScript contracts used to assemble this generation.
	pub fn typescript_contracts(&self) -> &GeneratedTypeScriptContracts {
		&self.typescript_contracts
	}

	/// Completed build artifacts to activate with the app build contract.
	pub fn artifacts(&self) -> &CompletedBuildArtifacts {
		&self.artifacts
	}
}

/// Report returned after publishing a committed dev generation's public
/// static files and runtime manifest/generated-TypeScript outputs to disk.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevOutputPublishReport {
	public_static: PublicStaticPublishReport,
	generation_outputs: BuildOutputWriteReport,
}

impl DevOutputPublishReport {
	/// Public static publication report.
	#[cfg(test)]
	pub fn public_static(&self) -> &PublicStaticPublishReport {
		&self.public_static
	}

	/// Runtime manifest and generated TypeScript write report.
	#[cfg(test)]
	pub fn generation_outputs(&self) -> &BuildOutputWriteReport {
		&self.generation_outputs
	}
}

/// Prepare a dev generation from precomputed graph/static inputs and live dev runtime data.
pub(crate) fn prepare_dev_generation_from_build_inputs(
	build_inputs: PreparedBuildInputs,
	runtime: DevRuntimeInputs,
) -> Result<PreparedDevGeneration, DevGenerationError> {
	validate_dev_runtime_inputs(&runtime)?;
	let client_entry = dev_client_module(
		build_inputs.plan(),
		build_inputs.plan().vite_inputs().entry_file(),
		runtime.vite_server_port(),
	)?;
	let view_module_outputs = build_inputs
		.bundle()
		.view_modules()
		.iter()
		.map(|view| dev_view_module_output(build_inputs.plan(), view, runtime.vite_server_port()))
		.collect::<Result<BTreeMap<_, _>, _>>()?;
	let public_filepaths = build_inputs
		.public_static_outputs()
		.public_filepaths()
		.to_vec();
	let public_filemap = build_inputs
		.public_static_outputs()
		.public_filemap()
		.clone();
	let artifacts = CompletedBuildArtifacts::new(
		build_inputs.critical_css().css(),
		public_filepaths,
		public_filemap,
		view_module_outputs,
	)
	.with_client_entry(client_entry)
	.with_dev_metadata(DevManifestMetadata::new(
		runtime.vite_server_port(),
		runtime.dev_mux_port(),
		runtime.dev_refresh_token(),
	));
	let projection_bundle = build_inputs.bundle.clone();
	let build_plan = build_inputs.plan.clone();
	let typescript_contracts = build_inputs.typescript_contracts.clone();
	Ok(PreparedDevGeneration {
		public_static_outputs: build_inputs.public_static_outputs,
		projection_bundle,
		build_plan,
		typescript_contracts,
		artifacts,
	})
}

pub(crate) fn prepare_dev_generation_from_committed_static_outputs(
	bundle: &ProjectionBundle,
	plan: &BuildProjectionPlan,
	artifacts: &GenerationArtifacts,
	runtime: DevRuntimeInputs,
) -> Result<CompletedBuildArtifacts, DevGenerationError> {
	prepare_dev_generation_from_static_outputs(
		bundle,
		plan,
		artifacts.critical_css(),
		artifacts.public_filepaths().to_vec(),
		artifacts.public_filemap().clone(),
		artifacts,
		runtime,
	)
}

pub(crate) fn prepare_dev_generation_from_static_outputs(
	bundle: &ProjectionBundle,
	plan: &BuildProjectionPlan,
	critical_css: impl Into<String>,
	public_filepaths: Vec<String>,
	public_filemap: BTreeMap<String, String>,
	artifacts: &GenerationArtifacts,
	runtime: DevRuntimeInputs,
) -> Result<CompletedBuildArtifacts, DevGenerationError> {
	validate_dev_runtime_inputs(&runtime)?;
	let client_entry = dev_client_module(
		plan,
		plan.vite_inputs().entry_file(),
		runtime.vite_server_port(),
	)?;
	let view_module_outputs = bundle
		.view_modules()
		.iter()
		.map(|view| dev_view_module_output(plan, view, runtime.vite_server_port()))
		.collect::<Result<BTreeMap<_, _>, _>>()?;
	let mut completed = CompletedBuildArtifacts::new(
		critical_css,
		public_filepaths,
		public_filemap,
		view_module_outputs,
	)
	.with_client_entry(client_entry)
	.with_dev_metadata(DevManifestMetadata::new(
		runtime.vite_server_port(),
		runtime.dev_mux_port(),
		runtime.dev_refresh_token(),
	));
	if let Some(client_core_assets) = artifacts.client_core_assets().cloned() {
		completed = completed.with_client_core_assets(client_core_assets);
	}
	Ok(completed)
}

/// Publish and activate an already-prepared dev generation.
pub(crate) async fn publish_and_activate_prepared_dev_generation<'a>(
	supervisor: &'a mut EpochSupervisor,
	app: AppBuildContract,
	prepared: &PreparedDevGeneration,
) -> Result<(&'a CommittedGeneration, DevOutputPublishReport), DevGenerationError> {
	let candidate = build_prepared_dev_generation_candidate(supervisor, app, prepared).await?;
	let report = publish_prepared_candidate_outputs(&candidate, prepared)?;
	let committed = supervisor.activate(candidate);
	Ok((committed, report))
}

pub(crate) async fn build_prepared_dev_generation_candidate(
	supervisor: &EpochSupervisor,
	app: AppBuildContract,
	prepared: &PreparedDevGeneration,
) -> Result<GenerationCandidate, DevGenerationError> {
	supervisor
		.build_next_app_candidate_with_precomputed_projections(
			app,
			prepared.projection_bundle().clone(),
			prepared.build_plan().clone(),
			prepared.typescript_contracts().clone(),
			prepared.artifacts().clone(),
		)
		.await
		.map_err(|source| DevGenerationError::Generation { source })
}

pub(crate) fn publish_prepared_candidate_outputs(
	candidate: &GenerationCandidate,
	prepared: &PreparedDevGeneration,
) -> Result<DevOutputPublishReport, DevGenerationError> {
	let public_static =
		publish_public_static_outputs(candidate.build_plan(), prepared.public_static_outputs())
			.map_err(|source| DevGenerationError::PublicStatic { source })?;
	let generation_outputs = write_generation_candidate_outputs(candidate, ManifestMode::Dev)
		.map_err(|source| DevGenerationError::BuildOutput { source })?;
	Ok(DevOutputPublishReport {
		public_static,
		generation_outputs,
	})
}

pub(crate) fn publish_cached_static_candidate_outputs(
	candidate: &GenerationCandidate,
) -> Result<DevOutputPublishReport, DevGenerationError> {
	let public_static = PublicStaticPublishReport::unchanged(candidate.build_plan())
		.map_err(|source| DevGenerationError::PublicStatic { source })?;
	let generation_outputs = write_generation_candidate_outputs(candidate, ManifestMode::Dev)
		.map_err(|source| DevGenerationError::BuildOutput { source })?;
	Ok(DevOutputPublishReport {
		public_static,
		generation_outputs,
	})
}

/// Error from this module's dev generation preparation and publication
/// functions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevGenerationError {
	/// Dev runtime inputs were invalid.
	InvalidRuntimeInput {
		/// Rejected field.
		field: &'static str,
	},
	/// Dev module URL projection failed.
	DevModuleUrl {
		/// Source Vite path projection error.
		source: VitePluginConfigError,
	},
	/// Public static publication failed.
	PublicStatic {
		/// Source public static output error.
		source: PublicStaticOutputError,
	},
	/// Candidate creation failed.
	Generation {
		/// Source generation error.
		source: GenerationError,
	},
	/// Runtime manifest or generated TypeScript write failed.
	BuildOutput {
		/// Source output write error.
		source: BuildOutputWriteError,
	},
}

impl std::fmt::Display for DevGenerationError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidRuntimeInput { field } => {
				write!(f, "invalid dev runtime input {field}")
			}
			Self::DevModuleUrl { source } => write!(f, "{source}"),
			Self::PublicStatic { source } => write!(f, "{source}"),
			Self::Generation { source } => write!(f, "{source}"),
			Self::BuildOutput { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for DevGenerationError {}

fn validate_dev_runtime_inputs(runtime: &DevRuntimeInputs) -> Result<(), DevGenerationError> {
	if runtime.vite_server_port() <= 0 {
		return Err(DevGenerationError::InvalidRuntimeInput {
			field: "vite_server_port",
		});
	}
	if runtime.dev_mux_port() <= 0 {
		return Err(DevGenerationError::InvalidRuntimeInput {
			field: "dev_mux_port",
		});
	}
	if runtime.dev_refresh_token().is_empty() {
		return Err(DevGenerationError::InvalidRuntimeInput {
			field: "dev_refresh_token",
		});
	}
	Ok(())
}

fn dev_view_module_output(
	plan: &BuildProjectionPlan,
	view: &ViewModuleContract,
	vite_server_port: i32,
) -> Result<(String, ClientModuleArtifacts), DevGenerationError> {
	Ok((
		view.pattern().to_owned(),
		dev_client_module(plan, view.client_file(), vite_server_port)?,
	))
}

fn dev_client_module(
	plan: &BuildProjectionPlan,
	path: &str,
	vite_server_port: i32,
) -> Result<ClientModuleArtifacts, DevGenerationError> {
	let root_dir = PathBuf::from(plan.workspace().root_dir());
	let js_package_manager_dir = crate::build_output::resolve_workspace_path(
		&root_dir,
		plan.vite_inputs().js_package_manager_dir(),
	);
	let path = package_manager_relative_path(&root_dir, &js_package_manager_dir, path)
		.map_err(|source| DevGenerationError::DevModuleUrl { source })?;
	Ok(ClientModuleArtifacts::new(
		format!("http://{DEV_LOOPBACK_HOST}:{vite_server_port}/{path}"),
		Vec::new(),
		Vec::new(),
	))
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::path::PathBuf;

	use super::*;
	use crate::generation_inputs::prepare_build_inputs;
	use crate::test_support::unique_temp_root_with_dirs;

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_ENTRY_FILE: &str = "src/entry.tsx";
	const TEST_CRITICAL_CSS_FILE: &str = "src/critical.css";
	const TEST_VIEW_PATTERN: &str = "/";
	const TEST_VIEW_CLIENT_FILE: &str = "src/root.tsx";

	fn temp_root() -> PathBuf {
		unique_temp_root_with_dirs("vorma-build-dev", &[TEST_PUBLIC_STATIC_SOURCE_DIR, "src"])
	}

	fn app_build_contract(root_dir: PathBuf) -> AppBuildContract {
		let mut views = vorma::Views::new();
		views.push(vorma::View::from_static(
			TEST_VIEW_PATTERN,
			TEST_VIEW_CLIENT_FILE,
			vorma::build_interface::route_input::type_resolver::<()>,
			vorma::build_interface::route_input::type_resolver::<()>,
			vorma::build_interface::route_input::search_schema_resolver::<()>,
			root_view_handler,
		));
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
			views,
			..vorma::AppConfig::<()>::default()
		})
		.unwrap()
	}

	fn root_view_handler(
		ctx: vorma::build_interface::ErasedRequestCtx<()>,
	) -> vorma::build_interface::ErasedRouteFuture {
		vorma::build_interface::run_static_view::<(), (), (), ()>(ctx, root_view_inner)
	}

	fn root_view_inner(
		_ctx: vorma::ViewCtx<(), ()>,
	) -> vorma::build_interface::RouteFuture<(), vorma::ViewExit> {
		Box::pin(async { Ok(()) })
	}

	#[tokio::test]
	async fn dev_generation_projects_vite_urls_metadata_and_publishes_dev_manifest() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg/>",
		)
		.unwrap();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let build_inputs = prepare_build_inputs(&app).unwrap();
		let prepared = prepare_dev_generation_from_build_inputs(
			build_inputs,
			DevRuntimeInputs::new(5173, 4173, "refresh-token"),
		)
		.unwrap();
		let mut supervisor = EpochSupervisor::default();

		let (committed, report) =
			publish_and_activate_prepared_dev_generation(&mut supervisor, app, &prepared)
				.await
				.unwrap();

		assert_eq!(committed.manifest().dev_vite_server_port(), 5173);
		assert_eq!(committed.manifest().dev_mux_port(), 4173);
		assert_eq!(committed.manifest().dev_refresh_token(), "refresh-token");
		assert_eq!(
			committed.manifest().client_entry().url(),
			"http://127.0.0.1:5173/src/entry.tsx"
		);
		assert_eq!(
			committed.manifest().view_module("/").unwrap().import_url(),
			"http://127.0.0.1:5173/src/root.tsx"
		);
		assert!(committed.runtime_snapshot().is_ok());
		assert!(report.generation_outputs().manifest_path().exists());

		fs::remove_dir_all(root_dir).unwrap();
	}
}
