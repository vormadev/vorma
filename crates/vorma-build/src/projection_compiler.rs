//! Projection compiler from framework graph to build artifacts.

use std::collections::{BTreeMap, BTreeSet};

use serde_json::Value;
use vorma::build_interface::assets::{AssetCapabilities, AssetCapabilityError};
use vorma::build_interface::contracts::{DocumentBuilder, DocumentBuilderError};
use vorma_contract::contracts::{DocumentContract, RouteTypeContract, TypeDef};
use vorma_contract::framework_graph::{BuildInputConfig, FrameworkGraph, ResourceKind};
use vorma_contract::runtime_manifest::{
	ClientCoreAssets, ClientModule, RuntimeManifest, RuntimeViewModule,
};

/// Compiled projections derived from one framework graph.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProjectionBundle {
	public_static_base: String,
	ui_variant: String,
	build_inputs: Option<BuildInputConfig>,
	document: DocumentContract,
	type_defs: Vec<TypeDef>,
	generated_typescript_extra_source: String,
	view_payloads: Vec<ViewPayloadContract>,
	resource_contracts: Vec<ResourceContract>,
	view_modules: Vec<ViewModuleContract>,
	static_assets: Vec<StaticAssetContract>,
}

impl ProjectionBundle {
	/// Compile graph projections.
	pub fn compile(graph: &FrameworkGraph) -> Self {
		Self {
			public_static_base: graph.config().public_static_base().to_owned(),
			ui_variant: graph
				.config()
				.build_inputs()
				.map(|inputs| inputs.frontend_inputs().ui_variant().to_owned())
				.unwrap_or_default(),
			build_inputs: graph.config().build_inputs().cloned(),
			document: graph.document().clone(),
			type_defs: graph.type_defs().to_vec(),
			generated_typescript_extra_source: graph
				.config()
				.build_inputs()
				.map(|inputs| inputs.generated_typescript_extra_source().to_owned())
				.unwrap_or_default(),
			view_payloads: graph
				.views()
				.iter()
				.map(|view| ViewPayloadContract {
					pattern: view.pattern().to_owned(),
					parent_patterns: view.parent_patterns().to_vec(),
					params: view.params().to_vec(),
					search_schema: view.search_schema().clone(),
					type_contract: view.type_contract().clone(),
				})
				.collect(),
			resource_contracts: graph
				.resources()
				.iter()
				.map(|resource| ResourceContract {
					method: resource.method().as_str().to_owned(),
					pattern: resource.pattern().to_owned(),
					kind: resource.kind(),
					default_kind: resource.default_kind(),
					params: resource.params().to_vec(),
					input_schema: resource.input_schema().cloned(),
					type_contract: resource.type_contract().clone(),
				})
				.collect(),
			view_modules: graph
				.views()
				.iter()
				.map(|view| ViewModuleContract {
					pattern: view.pattern().to_owned(),
					client_file: view.client_file().to_owned(),
				})
				.collect(),
			static_assets: graph
				.static_assets()
				.iter()
				.map(|asset| StaticAssetContract {
					source_path: asset.source_path().to_owned(),
					public_path: asset.public_path().to_owned(),
				})
				.collect(),
		}
	}

	/// Normalized public static base.
	pub fn public_static_base(&self) -> &str {
		&self.public_static_base
	}

	/// Stable UI adapter variant label.
	pub fn ui_variant(&self) -> &str {
		&self.ui_variant
	}

	/// Build/dev inputs carried by the canonical graph.
	pub fn build_inputs(&self) -> Option<&BuildInputConfig> {
		self.build_inputs.as_ref()
	}

	/// Document contract used by generated SSR and build identity outputs.
	#[cfg(test)]
	pub fn document(&self) -> &DocumentContract {
		&self.document
	}

	/// Type definitions emitted into generated TypeScript contracts.
	pub fn type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}

	/// Raw supplemental generated TypeScript source.
	pub fn generated_typescript_extra_source(&self) -> &str {
		&self.generated_typescript_extra_source
	}

	/// View patterns needed by runtime payload projection.
	#[cfg(test)]
	pub fn view_payload_patterns(&self) -> impl Iterator<Item = &str> + '_ {
		self.view_payloads.iter().map(ViewPayloadContract::pattern)
	}

	/// View payload contracts needed by runtime route payloads.
	pub fn view_payloads(&self) -> &[ViewPayloadContract] {
		&self.view_payloads
	}

	/// Resource contracts needed by generated TypeScript and API callers.
	pub fn resource_contracts(&self) -> &[ResourceContract] {
		&self.resource_contracts
	}

	/// View module contracts needed by manifests and dev module serving.
	pub fn view_modules(&self) -> &[ViewModuleContract] {
		&self.view_modules
	}

	/// Static asset contracts needed by manifests and capability tables.
	pub fn static_assets(&self) -> &[StaticAssetContract] {
		&self.static_assets
	}

	/// Build the dev watch plan derived from graph-owned source paths.
	#[cfg(test)]
	pub fn dev_watch_plan(&self) -> DevWatchPlan {
		let source_paths = self
			.view_modules
			.iter()
			.map(|view| view.client_file().to_owned())
			.chain(
				self.static_assets
					.iter()
					.map(|asset| asset.source_path().to_owned()),
			)
			.collect::<BTreeSet<_>>()
			.into_iter()
			.collect();
		DevWatchPlan { source_paths }
	}

	/// Build the runtime manifest projection from completed build artifacts.
	pub fn runtime_manifest(
		&self,
		artifacts: &GenerationArtifacts,
	) -> Result<RuntimeManifest, ProjectionError> {
		let mut public_filepaths = BTreeSet::new();
		for public_path in &artifacts.public_filepaths {
			if !public_filepaths.insert(public_path.clone()) {
				return Err(ProjectionError::DuplicatePublicOutput {
					public_path: public_path.clone(),
				});
			}
		}
		for (source_path, public_path) in &artifacts.public_filemap {
			if !public_filepaths.contains(public_path) {
				return Err(ProjectionError::UnlistedPublicFilemapOutput {
					source_path: source_path.clone(),
					public_path: public_path.clone(),
				});
			}
		}
		AssetCapabilities::new(
			self.public_static_base.clone(),
			public_filepaths.iter().cloned(),
			artifacts.public_filemap.clone(),
		)
		.map_err(|source| ProjectionError::InvalidPublicAssetCapabilities { source })?;
		for asset in &self.static_assets {
			match artifacts.public_filemap.get(asset.source_path()) {
				Some(public_path) if public_path == asset.public_path() => {}
				_ => {
					return Err(ProjectionError::MissingStaticAssetOutput {
						source_path: asset.source_path().to_owned(),
					});
				}
			}
		}
		let is_dev_generation = artifacts.dev_metadata().is_some();
		for public_path in artifacts.client_entry.public_output_paths() {
			if !is_dev_generation
				&& !public_path.is_empty()
				&& !public_filepaths.contains(public_path)
			{
				return Err(ProjectionError::UnlistedClientEntryPublicOutput {
					public_path: public_path.to_owned(),
				});
			}
		}
		if let Some(assets) = &artifacts.client_core_assets {
			for public_path in [assets.module_url(), assets.wasm_url()] {
				if !is_dev_generation
					&& !public_path.is_empty()
					&& !public_filepaths.contains(public_path)
				{
					return Err(ProjectionError::UnlistedClientCorePublicOutput {
						public_path: public_path.to_owned(),
					});
				}
			}
		}
		let mut view_modules = Vec::with_capacity(self.view_modules.len());
		for view in &self.view_modules {
			let module = artifacts
				.view_module_outputs
				.get(view.pattern())
				.ok_or_else(|| ProjectionError::MissingViewModuleOutput {
					pattern: view.pattern().to_owned(),
				})?;
			for public_path in module.public_output_paths() {
				if !is_dev_generation && !public_filepaths.contains(public_path) {
					return Err(ProjectionError::UnlistedViewModulePublicOutput {
						pattern: view.pattern().to_owned(),
						public_path: public_path.to_owned(),
					});
				}
			}
			view_modules.push(RuntimeViewModule::new(
				view.pattern(),
				module.import_url(),
				module.dep_urls().to_vec(),
				module.css_bundle_urls().to_vec(),
			));
		}
		let manifest = RuntimeManifest::new(
			"",
			self.public_static_base.clone(),
			artifacts.critical_css.clone(),
			self.view_payloads
				.iter()
				.map(|view| (view.pattern().to_owned(), view.search_schema().clone()))
				.collect(),
			public_filepaths.into_iter().collect(),
			artifacts.public_filemap.clone(),
			view_modules,
		)
		.with_client_entry(ClientModule::new(
			artifacts.client_entry.import_url(),
			artifacts.client_entry.dep_urls().to_vec(),
			artifacts.client_entry.css_bundle_urls().to_vec(),
		))
		.with_client_core_assets(
			artifacts
				.client_core_assets
				.as_ref()
				.map(|assets| ClientCoreAssets::new(assets.module_url(), assets.wasm_url())),
		)
		.with_ui_variant(self.ui_variant.clone())
		.with_root_document_shell_hash(root_document_shell_hash(
			artifacts.root_document_hash_source(),
		));
		let manifest = match artifacts.dev_metadata() {
			Some(dev_metadata) => manifest.with_dev_metadata(
				dev_metadata.vite_server_port(),
				dev_metadata.dev_mux_port(),
				dev_metadata.dev_refresh_token(),
			),
			None => manifest,
		};
		Ok(manifest)
	}
}

/// Dev watcher plan derived from one projection bundle.
#[cfg(test)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevWatchPlan {
	source_paths: Vec<String>,
}

#[cfg(test)]
impl DevWatchPlan {
	/// Source paths watched by the dev generation supervisor.
	pub fn source_paths(&self) -> &[String] {
		&self.source_paths
	}
}

/// View payload projection contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewPayloadContract {
	pattern: String,
	parent_patterns: Vec<String>,
	params: Vec<String>,
	search_schema: Value,
	type_contract: RouteTypeContract,
}

impl ViewPayloadContract {
	/// View route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Parent route patterns from outermost to innermost.
	pub fn parent_patterns(&self) -> &[String] {
		&self.parent_patterns
	}

	/// Dynamic parameter names in declaration order.
	pub fn params(&self) -> &[String] {
		&self.params
	}

	/// Search schema projected into route payloads.
	pub fn search_schema(&self) -> &Value {
		&self.search_schema
	}

	/// View input/output type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}
}

/// Resource projection contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceContract {
	method: String,
	pattern: String,
	kind: Option<ResourceKind>,
	default_kind: ResourceKind,
	params: Vec<String>,
	input_schema: Option<Value>,
	type_contract: RouteTypeContract,
}

impl ResourceContract {
	/// HTTP method.
	pub fn method(&self) -> &str {
		&self.method
	}

	/// Route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Generated-client kind.
	#[cfg(test)]
	pub fn kind(&self) -> ResourceKind {
		self.kind.unwrap_or(self.default_kind)
	}

	/// Explicit generated-client kind override.
	pub fn kind_override(&self) -> Option<ResourceKind> {
		self.kind
	}

	/// Default generated-client kind.
	pub fn default_kind(&self) -> ResourceKind {
		self.default_kind
	}

	/// Dynamic parameter names in declaration order.
	pub fn params(&self) -> &[String] {
		&self.params
	}

	/// Resource input schema when projected for generated API contracts.
	#[cfg(test)]
	pub fn input_schema(&self) -> Option<&Value> {
		self.input_schema.as_ref()
	}

	/// Resource input/output type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}
}

/// View module projection contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewModuleContract {
	pattern: String,
	client_file: String,
}

impl ViewModuleContract {
	/// View route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Client module source path.
	pub fn client_file(&self) -> &str {
		&self.client_file
	}
}

/// Static asset projection contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticAssetContract {
	source_path: String,
	public_path: String,
}

impl StaticAssetContract {
	/// Framework-relative source path.
	pub fn source_path(&self) -> &str {
		&self.source_path
	}

	/// Public request path.
	pub fn public_path(&self) -> &str {
		&self.public_path
	}
}

/// Completed build outputs before app document identity is derived.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CompletedBuildArtifacts {
	critical_css: String,
	public_filepaths: Vec<String>,
	public_filemap: BTreeMap<String, String>,
	client_entry: ClientModuleArtifacts,
	client_core_assets: Option<ClientCoreArtifacts>,
	view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
	dev_metadata: Option<DevManifestMetadata>,
}

impl CompletedBuildArtifacts {
	/// Create completed build artifacts before app document identity is derived.
	pub fn new(
		critical_css: impl Into<String>,
		public_filepaths: Vec<String>,
		public_filemap: BTreeMap<String, String>,
		view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
	) -> Self {
		Self {
			critical_css: critical_css.into(),
			public_filepaths,
			public_filemap,
			client_entry: ClientModuleArtifacts::default(),
			client_core_assets: None,
			view_module_outputs,
			dev_metadata: None,
		}
	}

	/// Attach the browser entry module artifact.
	pub fn with_client_entry(mut self, client_entry: ClientModuleArtifacts) -> Self {
		self.client_entry = client_entry;
		self
	}

	/// Attach client core module and WASM artifacts.
	pub fn with_client_core_assets(mut self, client_core_assets: ClientCoreArtifacts) -> Self {
		self.client_core_assets = Some(client_core_assets);
		self
	}

	/// Attach dev runtime manifest metadata.
	pub fn with_dev_metadata(mut self, dev_metadata: DevManifestMetadata) -> Self {
		self.dev_metadata = Some(dev_metadata);
		self
	}

	/// Derive generation artifacts from completed build outputs and the app document builder.
	pub async fn into_generation_artifacts(
		self,
		document_builder: &DocumentBuilder,
	) -> Result<GenerationArtifacts, GenerationArtifactError> {
		let Self {
			critical_css,
			public_filepaths,
			public_filemap,
			client_entry,
			client_core_assets,
			view_module_outputs,
			dev_metadata,
		} = self;
		let mut artifacts = GenerationArtifacts::from_document_builder(
			critical_css,
			document_builder,
			public_filepaths,
			public_filemap,
			view_module_outputs,
		)
		.await?;
		artifacts.client_entry = client_entry;
		artifacts.client_core_assets = client_core_assets;
		artifacts.dev_metadata = dev_metadata;
		Ok(artifacts)
	}

	/// Derive generation artifacts from completed build outputs and a precomputed
	/// document hash source.
	pub fn into_generation_artifacts_with_root_document_hash_source(
		self,
		root_document_hash_source: impl Into<String>,
	) -> GenerationArtifacts {
		let Self {
			critical_css,
			public_filepaths,
			public_filemap,
			client_entry,
			client_core_assets,
			view_module_outputs,
			dev_metadata,
		} = self;
		let mut artifacts = GenerationArtifacts::new(
			critical_css,
			root_document_hash_source,
			public_filepaths,
			public_filemap,
			view_module_outputs,
		);
		artifacts.client_entry = client_entry;
		artifacts.client_core_assets = client_core_assets;
		artifacts.dev_metadata = dev_metadata;
		artifacts
	}
}

/// Completed artifacts from one build generation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GenerationArtifacts {
	critical_css: String,
	root_document_hash_source: String,
	public_filepaths: Vec<String>,
	public_filemap: BTreeMap<String, String>,
	client_entry: ClientModuleArtifacts,
	client_core_assets: Option<ClientCoreArtifacts>,
	view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
	dev_metadata: Option<DevManifestMetadata>,
}

impl GenerationArtifacts {
	/// Create completed generation artifacts.
	pub fn new(
		critical_css: impl Into<String>,
		root_document_hash_source: impl Into<String>,
		public_filepaths: Vec<String>,
		public_filemap: BTreeMap<String, String>,
		view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
	) -> Self {
		Self {
			critical_css: critical_css.into(),
			root_document_hash_source: root_document_hash_source.into(),
			public_filepaths,
			public_filemap,
			client_entry: ClientModuleArtifacts::default(),
			client_core_assets: None,
			view_module_outputs,
			dev_metadata: None,
		}
	}

	/// Create completed generation artifacts by deriving document identity from the app builder.
	pub async fn from_document_builder(
		critical_css: impl Into<String>,
		document_builder: &DocumentBuilder,
		public_filepaths: Vec<String>,
		public_filemap: BTreeMap<String, String>,
		view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
	) -> Result<Self, GenerationArtifactError> {
		let root_document_hash_source = document_builder
			.build_root_document_hash_source()
			.await
			.map_err(|source| GenerationArtifactError::RootDocumentHashSource { source })?;
		Ok(Self::new(
			critical_css,
			root_document_hash_source,
			public_filepaths,
			public_filemap,
			view_module_outputs,
		))
	}

	/// Attach the browser entry module artifact.
	#[cfg(test)]
	pub fn with_client_entry(mut self, client_entry: ClientModuleArtifacts) -> Self {
		self.client_entry = client_entry;
		self
	}

	/// Attach client core module and WASM artifacts.
	#[cfg(test)]
	pub fn with_client_core_assets(mut self, client_core_assets: ClientCoreArtifacts) -> Self {
		self.client_core_assets = Some(client_core_assets);
		self
	}

	/// Critical CSS content inserted into server-rendered HTML.
	pub fn critical_css(&self) -> &str {
		&self.critical_css
	}

	/// Build identity source for the root document shell.
	pub fn root_document_hash_source(&self) -> &str {
		&self.root_document_hash_source
	}

	/// Manifest-listed public file paths.
	pub fn public_filepaths(&self) -> &[String] {
		&self.public_filepaths
	}

	/// Public source-to-URL map.
	pub fn public_filemap(&self) -> &BTreeMap<String, String> {
		&self.public_filemap
	}

	/// View pattern-to-module-output map.
	pub fn view_module_outputs(&self) -> &BTreeMap<String, ClientModuleArtifacts> {
		&self.view_module_outputs
	}

	/// Client core artifacts.
	pub fn client_core_assets(&self) -> Option<&ClientCoreArtifacts> {
		self.client_core_assets.as_ref()
	}

	/// Dev runtime manifest metadata.
	pub fn dev_metadata(&self) -> Option<&DevManifestMetadata> {
		self.dev_metadata.as_ref()
	}
}

/// Dev runtime manifest metadata.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevManifestMetadata {
	vite_server_port: i32,
	dev_mux_port: i32,
	dev_refresh_token: String,
}

impl DevManifestMetadata {
	/// Create dev runtime manifest metadata.
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

/// Completed generation artifact construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum GenerationArtifactError {
	/// Root document hash source derivation failed.
	RootDocumentHashSource {
		/// Source document builder error.
		source: DocumentBuilderError,
	},
}

impl std::fmt::Display for GenerationArtifactError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::RootDocumentHashSource { source } => {
				write!(f, "build root document hash source: {source}")
			}
		}
	}
}

impl std::error::Error for GenerationArtifactError {}

/// Completed browser module artifact.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct ClientModuleArtifacts {
	import_url: String,
	dep_urls: Vec<String>,
	css_bundle_urls: Vec<String>,
}

impl ClientModuleArtifacts {
	/// Create completed view module artifacts.
	pub fn new(
		import_url: impl Into<String>,
		dep_urls: Vec<String>,
		css_bundle_urls: Vec<String>,
	) -> Self {
		Self {
			import_url: import_url.into(),
			dep_urls,
			css_bundle_urls,
		}
	}

	/// Browser import URL.
	pub fn import_url(&self) -> &str {
		&self.import_url
	}

	/// Module dependency URLs.
	pub fn dep_urls(&self) -> &[String] {
		&self.dep_urls
	}

	/// CSS bundle URLs.
	pub fn css_bundle_urls(&self) -> &[String] {
		&self.css_bundle_urls
	}
}

/// Completed client core assets.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ClientCoreArtifacts {
	module_url: String,
	wasm_url: String,
}

impl ClientCoreArtifacts {
	/// Create client core artifacts.
	pub fn new(module_url: impl Into<String>, wasm_url: impl Into<String>) -> Self {
		Self {
			module_url: module_url.into(),
			wasm_url: wasm_url.into(),
		}
	}

	/// Client core module URL.
	pub fn module_url(&self) -> &str {
		&self.module_url
	}

	/// Client core WASM URL.
	pub fn wasm_url(&self) -> &str {
		&self.wasm_url
	}
}

/// Projection compiler error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProjectionError {
	/// Build artifacts listed the same public output more than once.
	DuplicatePublicOutput {
		/// Duplicated public output path.
		public_path: String,
	},
	/// A declared static asset had no completed output.
	MissingStaticAssetOutput {
		/// Static asset source path.
		source_path: String,
	},
	/// Public file map pointed at an output not listed in the manifest public outputs.
	UnlistedPublicFilemapOutput {
		/// Static source path.
		source_path: String,
		/// Unlisted public path.
		public_path: String,
	},
	/// Public output capability validation failed.
	InvalidPublicAssetCapabilities {
		/// Source public asset capability error.
		source: AssetCapabilityError,
	},
	/// A declared view had no completed browser module output.
	MissingViewModuleOutput {
		/// View route pattern.
		pattern: String,
	},
	/// A browser-visible view module output was not listed as a public output.
	UnlistedViewModulePublicOutput {
		/// View route pattern.
		pattern: String,
		/// Unlisted public module/dependency/CSS path.
		public_path: String,
	},
	/// Browser entry output was not listed as a public output.
	UnlistedClientEntryPublicOutput {
		/// Unlisted public module/dependency/CSS path.
		public_path: String,
	},
	/// Client core output was not listed as a public output.
	UnlistedClientCorePublicOutput {
		/// Unlisted public module/WASM path.
		public_path: String,
	},
}

impl std::fmt::Display for ProjectionError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::DuplicatePublicOutput { public_path } => {
				write!(f, "duplicate public output {public_path:?}")
			}
			Self::MissingStaticAssetOutput { source_path } => {
				write!(f, "missing static asset output for source {source_path:?}")
			}
			Self::UnlistedPublicFilemapOutput {
				source_path,
				public_path,
			} => write!(
				f,
				"public file map source {source_path:?} points at unlisted output {public_path:?}"
			),
			Self::InvalidPublicAssetCapabilities { source } => write!(f, "{source}"),
			Self::MissingViewModuleOutput { pattern } => {
				write!(f, "missing browser module output for view {pattern:?}")
			}
			Self::UnlistedViewModulePublicOutput {
				pattern,
				public_path,
			} => write!(
				f,
				"view {pattern:?} module output {public_path:?} is not manifest-listed"
			),
			Self::UnlistedClientEntryPublicOutput { public_path } => write!(
				f,
				"client entry output {public_path:?} is not manifest-listed"
			),
			Self::UnlistedClientCorePublicOutput { public_path } => write!(
				f,
				"client core output {public_path:?} is not manifest-listed"
			),
		}
	}
}

impl std::error::Error for ProjectionError {}

impl ClientModuleArtifacts {
	fn public_output_paths(&self) -> impl Iterator<Item = &str> {
		std::iter::once(self.import_url())
			.chain(self.dep_urls().iter().map(String::as_str))
			.chain(self.css_bundle_urls().iter().map(String::as_str))
	}
}

fn root_document_shell_hash(root_document_hash_source: &str) -> String {
	blake3::hash(root_document_hash_source.trim().as_bytes())
		.to_hex()
		.to_string()
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;
	use vorma::build_interface::contracts::{Document, DocumentBuilder};
	use vorma_contract::contracts::{DocumentAttributeContract, DocumentContract, TypeRefContract};
	use vorma_contract::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration,
		StaticAssetDeclaration, ViewDeclaration,
	};

	use super::*;
	use crate::test_support::route_type_contract;

	const TEST_CRITICAL_CSS: &str = "critical-css";
	const TEST_ROOT_DOCUMENT_HASH_SOURCE: &str = "document-hash-source";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn graph_with_view_resource_and_asset() -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::default();
		declarations.set_document(DocumentContract::new(
			vec![DocumentAttributeContract::new("lang", "en", false, false)],
			Vec::new(),
			Vec::new(),
			Vec::new(),
			Vec::new(),
		));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/sessions",
			None,
			Some(serde_json::json!({"body": "session"})),
			route_type_contract(),
			handler_id("sessions"),
		));
		declarations.add_static_asset(StaticAssetDeclaration::new(
			"app.css",
			"/static/app.hash.css",
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	#[test]
	fn projection_bundle_derives_all_contract_views_from_graph() {
		let graph = graph_with_view_resource_and_asset();

		let bundle = ProjectionBundle::compile(&graph);

		assert_eq!(bundle.public_static_base(), "/static/");
		assert_eq!(bundle.document().html_attributes()[0].name(), "lang");
		assert_eq!(bundle.view_payload_patterns().collect::<Vec<_>>(), ["/"]);
		assert_eq!(
			bundle.view_payloads()[0].type_contract().output(),
			&TypeRefContract::Unknown
		);
		assert_eq!(bundle.view_modules()[0].client_file(), "root.tsx");
		assert_eq!(bundle.resource_contracts()[0].method(), "POST");
		assert_eq!(
			bundle.resource_contracts()[0].kind(),
			ResourceKind::Mutation
		);
		assert_eq!(
			bundle.resource_contracts()[0].input_schema(),
			Some(&serde_json::json!({"body": "session"}))
		);
		assert_eq!(
			bundle.resource_contracts()[0].type_contract().input(),
			&TypeRefContract::Unit
		);
		assert_eq!(
			bundle.static_assets()[0].public_path(),
			"/static/app.hash.css"
		);
		assert_eq!(
			bundle.dev_watch_plan().source_paths(),
			["app.css", "root.tsx"]
		);
	}

	#[test]
	fn runtime_manifest_requires_complete_public_and_module_outputs() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec![
				"/static/app.hash.css".to_owned(),
				"/static/core.js".to_owned(),
				"/static/core.wasm".to_owned(),
				"/static/entry.css".to_owned(),
				"/static/entry.js".to_owned(),
				"/static/root.css".to_owned(),
				"/static/root.js".to_owned(),
				"/static/shared.js".to_owned(),
			],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new(
					"/static/root.js",
					vec!["/static/shared.js".to_owned()],
					vec!["/static/root.css".to_owned()],
				),
			)]),
		)
		.with_client_entry(ClientModuleArtifacts::new(
			"/static/entry.js",
			vec!["/static/core.js".to_owned(), "/static/shared.js".to_owned()],
			vec!["/static/entry.css".to_owned()],
		))
		.with_client_core_assets(ClientCoreArtifacts::new(
			"/static/core.js",
			"/static/core.wasm",
		));

		let manifest = bundle.runtime_manifest(&artifacts).unwrap();

		assert_eq!(
			manifest.client_build_id(),
			manifest.to_client_build_id().unwrap()
		);
		assert_eq!(manifest.client_build_id().len(), 24);
		assert_eq!(
			manifest.root_document_shell_hash(),
			root_document_shell_hash(TEST_ROOT_DOCUMENT_HASH_SOURCE)
		);
		assert_eq!(manifest.client_entry().url(), "/static/entry.js");
		assert_eq!(
			manifest.client_entry().dep_urls(),
			["/static/core.js", "/static/shared.js"]
		);
		assert_eq!(
			manifest.client_entry().css_bundle_urls(),
			["/static/entry.css"]
		);
		assert_eq!(
			manifest.client_core_assets().unwrap().module_url(),
			"/static/core.js"
		);
		assert_eq!(
			manifest.client_core_assets().unwrap().wasm_url(),
			"/static/core.wasm"
		);
		assert_eq!(manifest.critical_css(), TEST_CRITICAL_CSS);
		assert_eq!(manifest.search_schemas()["/"], serde_json::json!({}));
		assert_eq!(
			manifest.public_filepaths(),
			[
				"/static/app.hash.css",
				"/static/core.js",
				"/static/core.wasm",
				"/static/entry.css",
				"/static/entry.js",
				"/static/root.css",
				"/static/root.js",
				"/static/shared.js"
			]
		);
		assert_eq!(manifest.public_filemap()["app.css"], "/static/app.hash.css");
		assert_eq!(manifest.view_modules()[0].import_url(), "/static/root.js");
		assert_eq!(manifest.view_modules()[0].dep_urls(), ["/static/shared.js"]);
		assert_eq!(
			manifest.view_modules()[0].css_bundle_urls(),
			["/static/root.css"]
		);
	}

	#[tokio::test]
	async fn generation_artifacts_derive_root_document_hash_source_from_document_builder() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let document_builder = DocumentBuilder::new(|context| async move {
			let mut document = Document::new();
			let app_css = context.public_url(" app.css ")?;
			let head = document.head();
			head.link([head.rel("stylesheet"), head.href(app_css)]);
			head.meta_property_content("og:url", context.request().path());
			Ok(document)
		});
		let artifacts = GenerationArtifacts::from_document_builder(
			TEST_CRITICAL_CSS,
			&document_builder,
			vec![
				"/static/app.hash.css".to_owned(),
				"/static/entry.js".to_owned(),
				"/static/root.js".to_owned(),
			],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		)
		.await
		.unwrap()
		.with_client_entry(ClientModuleArtifacts::new(
			"/static/entry.js",
			Vec::new(),
			Vec::new(),
		));

		let source: serde_json::Value =
			serde_json::from_str(artifacts.root_document_hash_source()).unwrap();
		let manifest = bundle.runtime_manifest(&artifacts).unwrap();

		assert_eq!(source["head_defaults"][0]["attributes"]["href"], "/app.css");
		assert_eq!(source["head_defaults"][1]["attributes"]["content"], "/");
		assert_eq!(
			manifest.root_document_shell_hash(),
			root_document_shell_hash(artifacts.root_document_hash_source())
		);
	}

	#[test]
	fn runtime_manifest_rejects_unlisted_public_filemap_outputs() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			Vec::new(),
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		);

		let error = bundle.runtime_manifest(&artifacts).unwrap_err();

		assert!(matches!(
			error,
			ProjectionError::UnlistedPublicFilemapOutput { .. }
		));
	}

	#[test]
	fn runtime_manifest_rejects_unlisted_view_module_public_outputs() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		);

		let error = bundle.runtime_manifest(&artifacts).unwrap_err();

		assert!(matches!(
			error,
			ProjectionError::UnlistedViewModulePublicOutput { .. }
		));
	}

	#[test]
	fn runtime_manifest_rejects_duplicate_public_outputs() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec![
				"/static/app.hash.css".to_owned(),
				"/static/app.hash.css".to_owned(),
			],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		);

		let error = bundle.runtime_manifest(&artifacts).unwrap_err();

		assert!(matches!(
			error,
			ProjectionError::DuplicatePublicOutput { .. }
		));
	}

	#[test]
	fn runtime_manifest_rejects_public_outputs_outside_static_base() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec![
				"/static/app.hash.css".to_owned(),
				"/outside/root.js".to_owned(),
			],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/outside/root.js", Vec::new(), Vec::new()),
			)]),
		);

		let error = bundle.runtime_manifest(&artifacts).unwrap_err();

		assert!(matches!(
			error,
			ProjectionError::InvalidPublicAssetCapabilities {
				source: AssetCapabilityError::InvalidManifestPublicPath { .. }
			}
		));
	}

	#[test]
	fn runtime_manifest_rejects_missing_view_module_outputs() {
		let graph = graph_with_view_resource_and_asset();
		let bundle = ProjectionBundle::compile(&graph);
		let artifacts = GenerationArtifacts::new(
			TEST_CRITICAL_CSS,
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			BTreeMap::new(),
		);

		let error = bundle.runtime_manifest(&artifacts).unwrap_err();

		assert!(matches!(
			error,
			ProjectionError::MissingViewModuleOutput { .. }
		));
	}
}
