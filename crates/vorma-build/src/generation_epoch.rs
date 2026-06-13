//! Transactional generation epoch model.

use vorma::build_interface::AppBuildContract;
use vorma::build_interface::runtime::{
	RuntimeSnapshot, RuntimeSnapshotError, RuntimeSnapshotInput,
};
use vorma_contract::framework_graph::FrameworkGraph;
use vorma_contract::runtime_manifest::RuntimeManifest;

#[cfg(test)]
use crate::build_plan::BuildPlanError;
use crate::build_plan::BuildProjectionPlan;
use crate::projection_compiler::{
	CompletedBuildArtifacts, GenerationArtifactError, GenerationArtifacts, ProjectionBundle,
	ProjectionError,
};
use crate::typescript_contracts::GeneratedTypeScriptContracts;
#[cfg(test)]
use crate::typescript_contracts::{TypeScriptContractError, render_typescript_contracts};

/// Complete generation candidate before activation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GenerationCandidate {
	id: u64,
	graph: FrameworkGraph,
	projections: ProjectionBundle,
	build_plan: BuildProjectionPlan,
	typescript_contracts: GeneratedTypeScriptContracts,
	artifacts: GenerationArtifacts,
	manifest: RuntimeManifest,
}

impl GenerationCandidate {
	/// Build a complete generation candidate.
	#[cfg(test)]
	pub fn new(
		id: u64,
		graph: FrameworkGraph,
		artifacts: GenerationArtifacts,
	) -> Result<Self, GenerationError> {
		let projections = ProjectionBundle::compile(&graph);
		let build_plan = BuildProjectionPlan::compile(&projections)
			.map_err(|source| GenerationError::BuildPlan { source })?;
		let typescript_contracts =
			render_typescript_contracts(&projections, artifacts.public_filemap())
				.map_err(|source| GenerationError::TypeScriptContracts { source })?;
		let manifest = projections
			.runtime_manifest(&artifacts)
			.map_err(|source| GenerationError::Projection { source })?;
		RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph.clone(), manifest.clone()))
			.map_err(|source| GenerationError::RuntimeSnapshot { source })?;
		Ok(Self {
			id,
			graph,
			projections,
			build_plan,
			typescript_contracts,
			artifacts,
			manifest,
		})
	}

	/// Build a complete generation candidate from precomputed graph projections.
	pub fn from_precomputed_projections(
		id: u64,
		graph: FrameworkGraph,
		projections: ProjectionBundle,
		build_plan: BuildProjectionPlan,
		typescript_contracts: GeneratedTypeScriptContracts,
		artifacts: GenerationArtifacts,
	) -> Result<Self, GenerationError> {
		let manifest = projections
			.runtime_manifest(&artifacts)
			.map_err(|source| GenerationError::Projection { source })?;
		RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph.clone(), manifest.clone()))
			.map_err(|source| GenerationError::RuntimeSnapshot { source })?;
		Ok(Self {
			id,
			graph,
			projections,
			build_plan,
			typescript_contracts,
			artifacts,
			manifest,
		})
	}

	/// Build a complete generation candidate from the app build-facing contract.
	#[cfg(test)]
	pub async fn from_app_build_contract(
		id: u64,
		app: AppBuildContract,
		artifacts: CompletedBuildArtifacts,
	) -> Result<Self, GenerationError> {
		let (graph, document_builder) = app.into_parts();
		let artifacts = artifacts
			.into_generation_artifacts(&document_builder)
			.await
			.map_err(|source| GenerationError::Artifacts { source })?;
		Self::new(id, graph, artifacts)
	}

	/// Build a complete generation candidate from the app build-facing contract and
	/// precomputed graph projections.
	pub async fn from_app_build_contract_with_precomputed_projections(
		id: u64,
		app: AppBuildContract,
		projections: ProjectionBundle,
		build_plan: BuildProjectionPlan,
		typescript_contracts: GeneratedTypeScriptContracts,
		artifacts: CompletedBuildArtifacts,
	) -> Result<Self, GenerationError> {
		let (graph, document_builder) = app.into_parts();
		let artifacts = artifacts
			.into_generation_artifacts(&document_builder)
			.await
			.map_err(|source| GenerationError::Artifacts { source })?;
		Self::from_precomputed_projections(
			id,
			graph,
			projections,
			build_plan,
			typescript_contracts,
			artifacts,
		)
	}

	/// Build a complete generation candidate from live graph facts, precomputed graph
	/// projections, and a precomputed root document hash source.
	pub fn from_live_graph_with_precomputed_projections(
		id: u64,
		graph: FrameworkGraph,
		projections: ProjectionBundle,
		build_plan: BuildProjectionPlan,
		typescript_contracts: GeneratedTypeScriptContracts,
		artifacts: CompletedBuildArtifacts,
		root_document_hash_source: impl Into<String>,
	) -> Result<Self, GenerationError> {
		Self::from_precomputed_projections(
			id,
			graph,
			projections,
			build_plan,
			typescript_contracts,
			artifacts.into_generation_artifacts_with_root_document_hash_source(
				root_document_hash_source,
			),
		)
	}

	/// Candidate build/dev projection plan.
	pub fn build_plan(&self) -> &BuildProjectionPlan {
		&self.build_plan
	}

	/// Candidate generated TypeScript contracts.
	pub fn typescript_contracts(&self) -> &GeneratedTypeScriptContracts {
		&self.typescript_contracts
	}

	/// Candidate runtime manifest projection.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}

	/// Commit this candidate.
	pub fn commit(self, effects: GenerationEffects) -> CommittedGeneration {
		CommittedGeneration {
			id: self.id,
			graph: self.graph,
			projections: self.projections,
			build_plan: self.build_plan,
			typescript_contracts: self.typescript_contracts,
			artifacts: self.artifacts,
			manifest: self.manifest,
			effects,
		}
	}
}

/// Effects derived from one generation.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct GenerationEffects {
	/// Public asset table changed.
	pub public_assets_changed: bool,
	/// Critical CSS changed.
	pub critical_css_changed: bool,
	/// Browser route revalidation is required.
	pub client_revalidate_required: bool,
}

impl GenerationEffects {
	/// Derive committed generation effects from the previous committed generation.
	pub fn derive(previous: Option<&CommittedGeneration>, candidate: &GenerationCandidate) -> Self {
		let Some(previous) = previous else {
			return Self {
				public_assets_changed: true,
				critical_css_changed: true,
				client_revalidate_required: true,
			};
		};
		Self {
			public_assets_changed: previous.manifest.public_filepaths()
				!= candidate.manifest.public_filepaths()
				|| previous.manifest.public_filemap() != candidate.manifest.public_filemap()
				|| previous.artifacts.view_module_outputs()
					!= candidate.artifacts.view_module_outputs(),
			critical_css_changed: previous.artifacts.critical_css()
				!= candidate.artifacts.critical_css(),
			client_revalidate_required: previous.projections.view_payloads()
				!= candidate.projections.view_payloads()
				|| previous.projections.resource_contracts()
					!= candidate.projections.resource_contracts(),
		}
	}

	/// Project committed generation effects into browser refresh facts.
	pub fn browser_refresh_effects(&self, generation_id: u64) -> BrowserRefreshEffects {
		BrowserRefreshEffects {
			generation_id,
			public_assets_changed: self.public_assets_changed,
			critical_css_changed: self.critical_css_changed,
			client_revalidate_required: self.client_revalidate_required,
		}
	}
}

/// Browser refresh facts derived from a committed generation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BrowserRefreshEffects {
	generation_id: u64,
	public_assets_changed: bool,
	critical_css_changed: bool,
	client_revalidate_required: bool,
}

impl BrowserRefreshEffects {
	/// Committed generation identifier.
	#[cfg(test)]
	pub fn generation_id(&self) -> u64 {
		self.generation_id
	}

	/// Whether the committed public asset table changed.
	pub fn public_assets_changed(&self) -> bool {
		self.public_assets_changed
	}

	/// Whether committed critical CSS changed.
	pub fn critical_css_changed(&self) -> bool {
		self.critical_css_changed
	}

	/// Whether browser route revalidation is required.
	pub fn client_revalidate_required(&self) -> bool {
		self.client_revalidate_required
	}
}

/// Committed immutable generation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedGeneration {
	id: u64,
	graph: FrameworkGraph,
	projections: ProjectionBundle,
	build_plan: BuildProjectionPlan,
	typescript_contracts: GeneratedTypeScriptContracts,
	artifacts: GenerationArtifacts,
	manifest: RuntimeManifest,
	effects: GenerationEffects,
}

impl CommittedGeneration {
	/// Committed generation id.
	#[cfg(test)]
	pub fn id(&self) -> u64 {
		self.id
	}

	/// Committed framework graph.
	pub fn graph(&self) -> &FrameworkGraph {
		&self.graph
	}

	/// Committed projections.
	pub fn projections(&self) -> &ProjectionBundle {
		&self.projections
	}

	/// Committed build/dev projection plan.
	pub fn build_plan(&self) -> &BuildProjectionPlan {
		&self.build_plan
	}

	/// Committed generated TypeScript contracts.
	#[cfg(test)]
	pub fn typescript_contracts(&self) -> &GeneratedTypeScriptContracts {
		&self.typescript_contracts
	}

	/// Completed generation artifacts.
	pub fn artifacts(&self) -> &GenerationArtifacts {
		&self.artifacts
	}

	/// Committed runtime manifest projection.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}

	/// Committed effects.
	#[cfg(test)]
	pub fn effects(&self) -> &GenerationEffects {
		&self.effects
	}

	/// Build the runtime snapshot input for this committed generation.
	#[cfg(test)]
	pub fn runtime_snapshot_input(&self) -> RuntimeSnapshotInput {
		RuntimeSnapshotInput::new(self.graph.clone(), self.manifest.clone())
	}

	/// Compile the immutable runtime snapshot for this committed generation.
	#[cfg(test)]
	pub fn runtime_snapshot(&self) -> Result<RuntimeSnapshot, RuntimeSnapshotError> {
		RuntimeSnapshot::compile(self.runtime_snapshot_input())
	}

	/// Browser refresh facts for this committed generation.
	pub fn browser_refresh_effects(&self) -> BrowserRefreshEffects {
		self.effects.browser_refresh_effects(self.id)
	}
}

/// Generation candidate construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum GenerationError {
	/// Completed build artifact construction failed.
	Artifacts {
		/// Source generation artifact error.
		source: GenerationArtifactError,
	},
	/// Build/dev plan projection failed.
	#[cfg(test)]
	BuildPlan {
		/// Source build plan error.
		source: BuildPlanError,
	},
	/// Projection validation failed.
	Projection {
		/// Source projection error.
		source: ProjectionError,
	},
	/// Generated TypeScript contract rendering failed.
	#[cfg(test)]
	TypeScriptContracts {
		/// Source generated TypeScript rendering error.
		source: TypeScriptContractError,
	},
	/// Runtime snapshot compilation failed.
	RuntimeSnapshot {
		/// Source runtime snapshot error.
		source: RuntimeSnapshotError,
	},
}

impl std::fmt::Display for GenerationError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Artifacts { source } => write!(f, "{source}"),
			#[cfg(test)]
			Self::BuildPlan { source } => write!(f, "{source}"),
			Self::Projection { source } => write!(f, "{source}"),
			#[cfg(test)]
			Self::TypeScriptContracts { source } => write!(f, "{source}"),
			Self::RuntimeSnapshot { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for GenerationError {}

/// In-memory epoch supervisor for tested transactional commits.
#[derive(Clone, Debug)]
pub struct EpochSupervisor {
	committed: Option<CommittedGeneration>,
	next_candidate_id: u64,
}

impl Default for EpochSupervisor {
	fn default() -> Self {
		Self {
			committed: None,
			next_candidate_id: 1,
		}
	}
}

impl EpochSupervisor {
	/// Return committed generation, if any.
	#[cfg(test)]
	pub fn committed(&self) -> Option<&CommittedGeneration> {
		self.committed.as_ref()
	}

	/// Build the next complete generation candidate without committing it.
	#[cfg(test)]
	pub fn build_next_candidate(
		&self,
		graph: FrameworkGraph,
		artifacts: GenerationArtifacts,
	) -> Result<GenerationCandidate, GenerationError> {
		GenerationCandidate::new(self.next_candidate_id, graph, artifacts)
	}

	/// Build the next complete app generation candidate without committing it.
	#[cfg(test)]
	pub async fn build_next_app_candidate(
		&self,
		app: AppBuildContract,
		artifacts: CompletedBuildArtifacts,
	) -> Result<GenerationCandidate, GenerationError> {
		GenerationCandidate::from_app_build_contract(self.next_candidate_id, app, artifacts).await
	}

	/// Build the next complete app generation candidate from precomputed projections.
	pub async fn build_next_app_candidate_with_precomputed_projections(
		&self,
		app: AppBuildContract,
		projections: ProjectionBundle,
		build_plan: BuildProjectionPlan,
		typescript_contracts: GeneratedTypeScriptContracts,
		artifacts: CompletedBuildArtifacts,
	) -> Result<GenerationCandidate, GenerationError> {
		GenerationCandidate::from_app_build_contract_with_precomputed_projections(
			self.next_candidate_id,
			app,
			projections,
			build_plan,
			typescript_contracts,
			artifacts,
		)
		.await
	}

	/// Build the next complete live graph candidate from precomputed projections.
	pub fn build_next_live_graph_candidate_with_precomputed_projections(
		&self,
		graph: FrameworkGraph,
		projections: ProjectionBundle,
		build_plan: BuildProjectionPlan,
		typescript_contracts: GeneratedTypeScriptContracts,
		artifacts: CompletedBuildArtifacts,
		root_document_hash_source: impl Into<String>,
	) -> Result<GenerationCandidate, GenerationError> {
		GenerationCandidate::from_live_graph_with_precomputed_projections(
			self.next_candidate_id,
			graph,
			projections,
			build_plan,
			typescript_contracts,
			artifacts,
			root_document_hash_source,
		)
	}

	/// Build and atomically activate the next complete generation candidate.
	#[cfg(test)]
	pub fn activate_next_complete_candidate(
		&mut self,
		graph: FrameworkGraph,
		artifacts: GenerationArtifacts,
	) -> Result<&CommittedGeneration, GenerationError> {
		let candidate = self.build_next_candidate(graph, artifacts)?;
		Ok(self.activate(candidate))
	}

	/// Build and atomically activate the next complete app generation candidate.
	#[cfg(test)]
	pub async fn activate_next_complete_app_candidate(
		&mut self,
		app: AppBuildContract,
		artifacts: CompletedBuildArtifacts,
	) -> Result<&CommittedGeneration, GenerationError> {
		let candidate = self.build_next_app_candidate(app, artifacts).await?;
		Ok(self.activate(candidate))
	}

	/// Atomically activate a complete candidate.
	pub fn activate(&mut self, candidate: GenerationCandidate) -> &CommittedGeneration {
		self.next_candidate_id = self.next_candidate_id.max(candidate.id + 1);
		let effects = GenerationEffects::derive(self.committed.as_ref(), &candidate);
		self.committed = Some(candidate.commit(effects));
		self.committed
			.as_ref()
			.expect("generation was just committed")
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;
	use vorma_contract::contracts::{TypeDef, TypeRefContract};
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, GraphError, HandlerId, ResourceDeclaration, ServerBuildTarget,
		ViewDeclaration,
	};

	use super::*;
	use crate::projection_compiler::ClientModuleArtifacts;
	use crate::test_support::route_type_contract;
	use crate::typescript_contracts::PRIVATE_RESOURCE_CONTRACTS_IDENTIFIER;

	const TEST_CRITICAL_CSS: &str = "critical-css";
	const TEST_CRITICAL_CSS_2: &str = "critical-css-2";
	const TEST_ROOT_DOCUMENT_HASH_SOURCE: &str = "document-hash-source";
	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_ROOT_DIR: &str = ".";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_JS_PACKAGE_MANAGER_BASE_CMD: &str = "pnpm exec";
	const TEST_JS_PACKAGE_MANAGER_DIR: &str = "web";
	const TEST_VITE_CONFIG_FILE: &str = "vite.config.ts";
	const TEST_ENTRY_FILE: &str = "src/client/entry.tsx";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_CRITICAL_CSS_FILE: &str = "src/client/critical.css";
	const TEST_GENERATED_TYPESCRIPT_FILE: &str = "src/client/vorma.gen.ts";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn framework_config() -> FrameworkConfig {
		FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new(TEST_CARGO_PACKAGE, TEST_CARGO_BIN),
			TEST_ROOT_DIR,
			TEST_DIST_DIR,
			FrontendBuildInputs::new(
				"react",
				TEST_JS_PACKAGE_MANAGER_BASE_CMD,
				TEST_JS_PACKAGE_MANAGER_DIR,
				TEST_VITE_CONFIG_FILE,
				TEST_ENTRY_FILE,
				TEST_PUBLIC_STATIC_SOURCE_DIR,
				TEST_CRITICAL_CSS_FILE,
			),
			TEST_GENERATED_TYPESCRIPT_FILE,
			DevWatchConfig::default(),
		))
	}

	fn graph() -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::new(framework_config());
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	fn graph_with_extra_resource() -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::new(framework_config());
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/pong",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("pong"),
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	fn complete_artifacts() -> GenerationArtifacts {
		complete_artifacts_with(TEST_CRITICAL_CSS, "/static/root.js")
	}

	fn complete_artifacts_with(critical_css: &str, root_import_url: &str) -> GenerationArtifacts {
		GenerationArtifacts::new(
			critical_css,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec![root_import_url.to_owned()],
			BTreeMap::new(),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new(root_import_url, Vec::new(), Vec::new()),
			)]),
		)
	}

	fn app_build_contract() -> AppBuildContract {
		let document = vorma::DocumentBuilder::new(|context| async move {
			let mut document = vorma::Document::new();
			let app_css = context.public_url(" app.css ")?;
			let head = document.head();
			head.link([head.rel("stylesheet"), head.href(app_css)]);
			head.meta_property_content("og:url", context.request().path());
			Ok(document)
		});
		vorma::build_interface::app_build_contract(vorma::AppConfig {
			root_dir: TEST_ROOT_DIR.into(),
			server_target: vorma::ServerTarget {
				cargo_package: TEST_CARGO_PACKAGE.to_owned(),
				cargo_bin: TEST_CARGO_BIN.to_owned(),
			},
			dist_dir: TEST_DIST_DIR.to_owned(),
			public_static_base: "/static/".to_owned(),
			frontend_config: vorma::FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: TEST_JS_PACKAGE_MANAGER_BASE_CMD.to_owned(),
				js_package_manager_dir: TEST_JS_PACKAGE_MANAGER_DIR.to_owned(),
				vite_config_file: TEST_VITE_CONFIG_FILE.to_owned(),
				entry_file: TEST_ENTRY_FILE.to_owned(),
				public_static_src_dir: TEST_PUBLIC_STATIC_SOURCE_DIR.to_owned(),
				critical_css_file: TEST_CRITICAL_CSS_FILE.to_owned(),
			},
			ts_gen_config: vorma::TsGenConfig {
				out_file: TEST_GENERATED_TYPESCRIPT_FILE.to_owned(),
				..vorma::TsGenConfig::default()
			},
			document,
			..vorma::AppConfig::<()>::default()
		})
		.unwrap()
	}

	#[test]
	fn candidate_commit_preserves_complete_epoch() {
		let candidate = GenerationCandidate::new(7, graph(), complete_artifacts()).unwrap();
		let mut supervisor = EpochSupervisor::default();

		let committed = supervisor.activate(candidate);
		let expected_client_build_id = committed.manifest().client_build_id().to_owned();

		assert_eq!(committed.id(), 7);
		assert_eq!(
			committed.projections().resource_contracts()[0].pattern(),
			"/ping"
		);
		assert_eq!(
			committed.build_plan().server_target().cargo_package(),
			TEST_CARGO_PACKAGE
		);
		assert_eq!(
			committed.build_plan().vite_inputs().entry_file(),
			TEST_ENTRY_FILE
		);
		assert_eq!(committed.manifest().ui_variant(), "react");
		assert_eq!(
			committed.manifest().client_build_id(),
			committed.manifest().to_client_build_id().unwrap()
		);
		assert_eq!(expected_client_build_id.len(), 24);
		assert!(committed.typescript_contracts().source().contains(&format!(
			"const {PRIVATE_RESOURCE_CONTRACTS_IDENTIFIER} = ["
		)));
		assert_eq!(
			committed
				.runtime_snapshot_input()
				.manifest()
				.client_build_id(),
			expected_client_build_id
		);
		assert_eq!(
			committed.runtime_snapshot().unwrap().client_build_id(),
			expected_client_build_id
		);
		assert!(committed.effects().public_assets_changed);
		assert!(committed.effects().critical_css_changed);
		assert!(committed.effects().client_revalidate_required);
		assert!(committed.browser_refresh_effects().public_assets_changed());
		assert_eq!(committed.browser_refresh_effects().generation_id(), 7);
	}

	#[tokio::test]
	async fn supervisor_activates_app_build_contract_with_document_derived_artifacts() {
		let mut supervisor = EpochSupervisor::default();

		let committed = supervisor
			.activate_next_complete_app_candidate(
				app_build_contract(),
				CompletedBuildArtifacts::new(
					TEST_CRITICAL_CSS,
					Vec::new(),
					BTreeMap::new(),
					BTreeMap::new(),
				),
			)
			.await
			.unwrap();
		let source: serde_json::Value =
			serde_json::from_str(committed.artifacts().root_document_hash_source()).unwrap();

		assert_eq!(committed.id(), 1);
		assert_eq!(
			committed.build_plan().server_target().cargo_package(),
			TEST_CARGO_PACKAGE
		);
		assert_eq!(source["head_defaults"][0]["attributes"]["href"], "/app.css");
		assert_eq!(source["head_defaults"][1]["attributes"]["content"], "/");
		assert_ne!(
			committed.artifacts().root_document_hash_source(),
			TEST_ROOT_DOCUMENT_HASH_SOURCE
		);
	}

	#[test]
	fn candidate_creation_rejects_missing_build_inputs() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();

		let error = GenerationCandidate::new(7, graph, complete_artifacts()).unwrap_err();

		assert!(matches!(
			error,
			GenerationError::BuildPlan {
				source: BuildPlanError::MissingBuildInputs
			}
		));
	}

	#[test]
	fn candidate_creation_rejects_incomplete_epoch() {
		let error = GenerationCandidate::new(
			7,
			graph(),
			GenerationArtifacts::new(
				TEST_CRITICAL_CSS,
				TEST_ROOT_DOCUMENT_HASH_SOURCE,
				Vec::new(),
				BTreeMap::new(),
				BTreeMap::new(),
			),
		)
		.unwrap_err();

		assert!(matches!(
			error,
			GenerationError::Projection {
				source: ProjectionError::MissingViewModuleOutput { .. }
			}
		));
	}

	#[test]
	fn candidate_creation_rejects_invalid_public_output_capabilities() {
		let error = GenerationCandidate::new(
			7,
			graph(),
			GenerationArtifacts::new(
				TEST_CRITICAL_CSS,
				TEST_ROOT_DOCUMENT_HASH_SOURCE,
				vec!["/outside/root.js".to_owned()],
				BTreeMap::new(),
				BTreeMap::from([(
					"/".to_owned(),
					ClientModuleArtifacts::new("/outside/root.js", Vec::new(), Vec::new()),
				)]),
			),
		)
		.unwrap_err();

		assert!(matches!(
			error,
			GenerationError::Projection {
				source: ProjectionError::InvalidPublicAssetCapabilities { .. }
			}
		));
	}

	#[test]
	fn supervisor_failed_candidate_does_not_replace_committed_generation() {
		let mut supervisor = EpochSupervisor::default();
		let committed_id = supervisor
			.activate_next_complete_candidate(graph(), complete_artifacts())
			.unwrap()
			.id();
		let committed_client_build_id = supervisor
			.committed()
			.unwrap()
			.manifest()
			.client_build_id()
			.to_owned();

		let error = supervisor
			.activate_next_complete_candidate(
				graph(),
				GenerationArtifacts::new(
					TEST_CRITICAL_CSS,
					TEST_ROOT_DOCUMENT_HASH_SOURCE,
					Vec::new(),
					BTreeMap::new(),
					BTreeMap::new(),
				),
			)
			.unwrap_err();
		let still_committed = supervisor.committed().unwrap();

		assert_eq!(committed_id, 1);
		assert!(matches!(
			error,
			GenerationError::Projection {
				source: ProjectionError::MissingViewModuleOutput { .. }
			}
		));
		assert_eq!(still_committed.id(), 1);
		assert_eq!(
			still_committed.manifest().client_build_id(),
			committed_client_build_id
		);
		assert!(still_committed.effects().public_assets_changed);
		assert!(still_committed.effects().client_revalidate_required);

		let next_committed_id = supervisor
			.activate_next_complete_candidate(graph(), complete_artifacts())
			.unwrap()
			.id();

		assert_eq!(next_committed_id, 2);
	}

	#[test]
	fn supervisor_derives_generation_effects_from_committed_facts() {
		let mut supervisor = EpochSupervisor::default();
		let first = supervisor
			.activate_next_complete_candidate(graph(), complete_artifacts())
			.unwrap();

		assert!(first.effects().public_assets_changed);
		assert!(first.effects().critical_css_changed);
		assert!(first.effects().client_revalidate_required);

		let unchanged = supervisor
			.activate_next_complete_candidate(graph(), complete_artifacts())
			.unwrap();

		assert!(!unchanged.effects().public_assets_changed);
		assert!(!unchanged.effects().critical_css_changed);
		assert!(!unchanged.effects().client_revalidate_required);

		let critical_css = supervisor
			.activate_next_complete_candidate(
				graph(),
				complete_artifacts_with(TEST_CRITICAL_CSS_2, "/static/root.js"),
			)
			.unwrap();

		assert!(!critical_css.effects().public_assets_changed);
		assert!(critical_css.effects().critical_css_changed);
		assert!(!critical_css.effects().client_revalidate_required);

		let public_assets = supervisor
			.activate_next_complete_candidate(
				graph(),
				complete_artifacts_with(TEST_CRITICAL_CSS_2, "/static/root.2.js"),
			)
			.unwrap();

		assert!(public_assets.effects().public_assets_changed);
		assert!(!public_assets.effects().critical_css_changed);
		assert!(!public_assets.effects().client_revalidate_required);

		let route_contracts = supervisor
			.activate_next_complete_candidate(
				graph_with_extra_resource(),
				complete_artifacts_with(TEST_CRITICAL_CSS_2, "/static/root.2.js"),
			)
			.unwrap();

		assert!(!route_contracts.effects().public_assets_changed);
		assert!(!route_contracts.effects().critical_css_changed);
		assert!(route_contracts.effects().client_revalidate_required);
	}

	#[test]
	fn graph_rejects_invalid_generated_typescript_contracts_before_candidate_creation() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Alias {
			key: "bad".to_owned(),
			name: "bad name".to_owned(),
			target: TypeRefContract::String,
		});

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(error, GraphError::InvalidTypeDef { .. }));
	}
}
