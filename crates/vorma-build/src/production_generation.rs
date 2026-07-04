//! Production generation assembler.
//!
//! Where production builds diverge from dev: view/entry client modules
//! resolve to the finished, hashed bundle URLs a completed Vite production
//! build produced (projected from the Vite manifest via
//! [`crate::vite_manifest::project_vite_manifest`]), instead of the
//! dev-server-proxied URLs [`crate::dev_generation`] projects. See
//! [`crate::dev_generation`] for the dev counterpart.

use std::collections::BTreeMap;

use vorma::build_interface::AppBuildContract;
use vorma::build_interface::assets::ManifestMode;

use crate::build_output::{
	BuildOutputWriteError, BuildOutputWriteReport, write_generation_candidate_outputs,
};
use crate::build_plan::BuildProjectionPlan;
use crate::generation_epoch::{CommittedGeneration, EpochSupervisor, GenerationError};
use crate::generation_inputs::PreparedBuildInputs;
use crate::projection_compiler::{CompletedBuildArtifacts, ProjectionBundle};
use crate::static_outputs::{
	PreparedPublicStaticOutputs, PublicStaticOutputError, PublicStaticPublishReport,
	publish_public_static_outputs,
};
use crate::typescript_contracts::GeneratedTypeScriptContracts;
use crate::vite_manifest::{ViteManifest, ViteManifestError, project_vite_manifest};

/// Prepared production generation artifacts before epoch activation and filesystem publication.
#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct PreparedProductionGeneration {
	public_static_outputs: PreparedPublicStaticOutputs,
	projection_bundle: ProjectionBundle,
	build_plan: BuildProjectionPlan,
	typescript_contracts: GeneratedTypeScriptContracts,
	artifacts: CompletedBuildArtifacts,
}

impl PreparedProductionGeneration {
	/// Prepared public static outputs.
	pub(crate) fn public_static_outputs(&self) -> &PreparedPublicStaticOutputs {
		&self.public_static_outputs
	}

	/// Completed build artifacts to activate with the app build contract.
	pub(crate) fn artifacts(&self) -> &CompletedBuildArtifacts {
		&self.artifacts
	}
}

/// Report returned after publishing a committed production generation's
/// public static files and runtime manifest/generated-TypeScript outputs to
/// disk.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProductionOutputPublishReport {
	public_static: PublicStaticPublishReport,
	generation_outputs: BuildOutputWriteReport,
}

impl ProductionOutputPublishReport {
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

/// Prepare one complete production generation from pre-Vite inputs and completed Vite outputs.
pub(crate) fn prepare_production_generation_from_build_inputs(
	build_inputs: PreparedBuildInputs,
	vite_manifest: &ViteManifest,
) -> Result<PreparedProductionGeneration, ProductionGenerationError> {
	let vite = project_vite_manifest(build_inputs.bundle(), build_inputs.plan(), vite_manifest)
		.map_err(|source| ProductionGenerationError::ViteManifest { source })?;

	let public_filepaths = build_inputs
		.public_static_outputs()
		.public_filepaths()
		.iter()
		.cloned()
		.chain(vite.public_filepaths().iter().cloned())
		.collect::<Vec<_>>();
	let public_filemap = build_inputs
		.public_static_outputs()
		.public_filemap()
		.iter()
		.map(|(source_path, public_path)| (source_path.clone(), public_path.clone()))
		.collect::<BTreeMap<_, _>>();
	let artifacts = CompletedBuildArtifacts::new(
		build_inputs.critical_css().css(),
		public_filepaths,
		public_filemap,
		vite.view_module_outputs().clone(),
	)
	.with_client_entry(vite.client_entry().clone())
	.with_client_core_assets(vite.client_core_assets().clone());

	Ok(PreparedProductionGeneration {
		public_static_outputs: build_inputs.public_static_outputs,
		projection_bundle: build_inputs.bundle,
		build_plan: build_inputs.plan,
		typescript_contracts: build_inputs.typescript_contracts,
		artifacts,
	})
}

/// Publish and activate an already-prepared production generation.
pub(crate) async fn publish_and_activate_prepared_production_generation<'a>(
	supervisor: &'a mut EpochSupervisor,
	app: AppBuildContract,
	prepared: &PreparedProductionGeneration,
	mode: ManifestMode,
) -> Result<(&'a CommittedGeneration, ProductionOutputPublishReport), ProductionGenerationError> {
	let candidate = supervisor
		.build_next_app_candidate_with_precomputed_projections(
			app,
			prepared.projection_bundle.clone(),
			prepared.build_plan.clone(),
			prepared.typescript_contracts.clone(),
			prepared.artifacts().clone(),
		)
		.await
		.map_err(|source| ProductionGenerationError::Generation { source })?;
	let public_static =
		publish_public_static_outputs(candidate.build_plan(), prepared.public_static_outputs())
			.map_err(|source| ProductionGenerationError::PublicStatic { source })?;
	let generation_outputs = write_generation_candidate_outputs(&candidate, mode)
		.map_err(|source| ProductionGenerationError::BuildOutput { source })?;
	let report = ProductionOutputPublishReport {
		public_static,
		generation_outputs,
	};
	let committed = supervisor.activate(candidate);
	Ok((committed, report))
}

/// Error from this module's production generation preparation and
/// publication functions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProductionGenerationError {
	/// Public static publication failed.
	PublicStatic {
		/// Source static output error.
		source: PublicStaticOutputError,
	},
	/// Vite manifest projection failed.
	ViteManifest {
		/// Source Vite manifest error.
		source: ViteManifestError,
	},
	/// Generation activation failed.
	Generation {
		/// Source generation error.
		source: GenerationError,
	},
	/// Runtime manifest or generated TypeScript write failed.
	BuildOutput {
		/// Source build output write error.
		source: BuildOutputWriteError,
	},
}

impl std::fmt::Display for ProductionGenerationError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::PublicStatic { source } => write!(f, "{source}"),
			Self::ViteManifest { source } => write!(f, "{source}"),
			Self::Generation { source } => write!(f, "{source}"),
			Self::BuildOutput { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for ProductionGenerationError {}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::PathBuf;

	use super::*;
	use crate::build_output::write_typescript_contracts;
	use crate::generation_inputs::prepare_build_inputs;
	use crate::test_support::unique_temp_root;
	use crate::vite_manifest::{CLIENT_CORE_WASM_SOURCE_FILENAME, ViteManifestChunk};

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_ENTRY_FILE: &str = "src/entry.tsx";
	const TEST_CRITICAL_CSS_FILE: &str = "src/critical.css";

	fn temp_root() -> PathBuf {
		unique_temp_root("vorma-build-production")
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
				js_package_manager_base_cmd: "pnpm".to_owned(),
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

	fn vite_manifest() -> ViteManifest {
		let wasm_file = format!("assets/{CLIENT_CORE_WASM_SOURCE_FILENAME}");
		ViteManifest::from(BTreeMap::from([
			(
				TEST_ENTRY_FILE.to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					css: vec!["assets/entry.css".to_owned()],
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
		]))
	}

	#[tokio::test]
	async fn production_generation_assembles_static_vite_document_and_manifest_facts() {
		let root_dir = temp_root();
		fs::create_dir_all(root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR)).unwrap();
		fs::create_dir_all(root_dir.join("src")).unwrap();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg></svg>",
		)
		.unwrap();
		fs::write(
			root_dir.join(TEST_CRITICAL_CSS_FILE),
			r#".hero { background: url("@public/logo.svg"); }"#,
		)
		.unwrap();
		let mut supervisor = EpochSupervisor::default();
		let app = app_build_contract(root_dir.clone());
		let build_inputs = prepare_build_inputs(&app).unwrap();
		let typescript_report =
			write_typescript_contracts(build_inputs.plan(), build_inputs.typescript_contracts())
				.unwrap();

		assert_eq!(
			build_inputs.vite_plugin_config().entry_module,
			TEST_ENTRY_FILE
		);
		assert!(typescript_report.path().exists());

		let prepared =
			prepare_production_generation_from_build_inputs(build_inputs, &vite_manifest())
				.unwrap();

		let (committed, publish_report) = publish_and_activate_prepared_production_generation(
			&mut supervisor,
			app,
			&prepared,
			ManifestMode::Prod,
		)
		.await
		.unwrap();

		assert!(committed.manifest().critical_css().contains("/static/"));
		assert!(
			committed
				.manifest()
				.public_filepaths()
				.iter()
				.any(|path| path == "/static/assets/entry.js")
		);
		assert!(
			committed
				.manifest()
				.public_filepaths()
				.iter()
				.any(|path| path.contains("vorma_out_logo_"))
		);
		assert_eq!(
			committed.manifest().client_entry().url(),
			"/static/assets/entry.js"
		);
		assert_eq!(
			committed
				.manifest()
				.client_core_assets()
				.unwrap()
				.wasm_url(),
			"/static/assets/vorma_client_wasm_bg.wasm"
		);
		assert!(
			publish_report
				.public_static()
				.public_output_dir()
				.join(prepared.public_static_outputs().files()[0].output_name())
				.exists()
		);
		assert!(publish_report.generation_outputs().manifest_path().exists());
		assert!(
			publish_report
				.generation_outputs()
				.generated_typescript_path()
				.exists()
		);

		fs::remove_dir_all(root_dir).unwrap();
	}
}
