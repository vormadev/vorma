//! Immutable runtime generation snapshots.
//!
//! Contract layer of the declare → contract → serve pipeline: compiles one
//! committed `FrameworkGraph` + `RuntimeManifest` pair into the immutable
//! `RuntimeSnapshot` (execution plan, asset capabilities, document facts)
//! that every serving layer above reads and never mutates.

use std::collections::BTreeSet;

use url::Url;

use crate::asset_capabilities::{AssetCapabilities, AssetCapabilityError};
use crate::execution_engine::{
	ExecutionEngine, HandlerRegistry, RequestExecutionReport, RequestInput,
};
use crate::execution_plan::{ExecutionPlan, PlanError};
use crate::framework_graph::FrameworkGraph;
use crate::runtime_manifest::RuntimeManifest;

/// Inputs required to compile one runtime generation snapshot.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RuntimeSnapshotInput {
	graph: FrameworkGraph,
	manifest: RuntimeManifest,
}

impl RuntimeSnapshotInput {
	/// Create runtime snapshot inputs.
	pub fn new(graph: FrameworkGraph, manifest: RuntimeManifest) -> Self {
		Self { graph, manifest }
	}

	/// Framework graph.
	pub fn graph(&self) -> &FrameworkGraph {
		&self.graph
	}

	/// Runtime manifest.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}
}

/// Immutable committed runtime generation snapshot.
#[derive(Clone, Debug)]
pub struct RuntimeSnapshot {
	graph: FrameworkGraph,
	manifest: RuntimeManifest,
	client_build_id: String,
	engine: ExecutionEngine,
}

impl RuntimeSnapshot {
	/// Compile an immutable runtime snapshot.
	pub fn compile(input: RuntimeSnapshotInput) -> Result<Self, RuntimeSnapshotError> {
		if input.manifest.client_build_id().trim().is_empty() {
			return Err(RuntimeSnapshotError::MissingClientBuildId);
		}
		validate_manifest_view_contracts(&input)?;
		let plan = ExecutionPlan::compile(&input.graph)
			.map_err(|source| RuntimeSnapshotError::Plan { source })?;
		let assets = AssetCapabilities::new(
			input.manifest.public_static_base(),
			input.manifest.public_filepaths().to_vec(),
			input.manifest.public_filemap().clone(),
		)
		.map_err(|source| RuntimeSnapshotError::Assets { source })?;
		let client_build_id = input.manifest.client_build_id().to_owned();
		Ok(Self {
			graph: input.graph,
			manifest: input.manifest,
			client_build_id,
			engine: ExecutionEngine::new(plan, assets),
		})
	}

	/// Framework graph committed with this snapshot.
	pub fn graph(&self) -> &FrameworkGraph {
		&self.graph
	}

	/// Runtime manifest committed with this snapshot.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}

	/// Client build identifier committed with this snapshot.
	pub fn client_build_id(&self) -> &str {
		&self.client_build_id
	}

	/// Runtime execution engine.
	pub fn engine(&self) -> &ExecutionEngine {
		&self.engine
	}

	/// Committed public asset capabilities.
	pub fn asset_capabilities(&self) -> &AssetCapabilities {
		self.engine.asset_capabilities()
	}

	/// Execute one request against this snapshot.
	pub async fn execute(
		&self,
		request: RequestInput,
		handlers: &HandlerRegistry,
	) -> Result<RequestExecutionReport, crate::execution_engine::ExecutionError>
where {
		self.engine.execute(request, handlers).await
	}
}

/// Runtime snapshot compile error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeSnapshotError {
	/// Runtime snapshot did not include a client build identifier.
	MissingClientBuildId,
	/// Runtime manifest configuration does not match the graph configuration.
	ManifestGraphMismatch {
		/// Mismatched field name.
		field: &'static str,
		/// Graph configuration value.
		graph_value: String,
		/// Manifest configuration value.
		manifest_value: String,
	},
	/// Execution plan compilation failed.
	Plan {
		/// Source execution plan error.
		source: PlanError,
	},
	/// Asset capability compilation failed.
	Assets {
		/// Source asset capability error.
		source: AssetCapabilityError,
	},
	/// Runtime manifest missed a search schema for a declared view.
	MissingViewSearchSchema {
		/// Declared view route pattern.
		pattern: String,
	},
	/// Runtime manifest included a search schema for no declared view.
	UnexpectedViewSearchSchema {
		/// Unexpected view route pattern.
		pattern: String,
	},
	/// Runtime manifest missed a browser module for a declared view.
	MissingViewModule {
		/// Declared view route pattern.
		pattern: String,
	},
	/// Runtime manifest included more than one browser module for a declared view.
	DuplicateViewModule {
		/// Duplicated view route pattern.
		pattern: String,
	},
	/// Runtime manifest included a browser module for no declared view.
	UnexpectedViewModule {
		/// Unexpected view route pattern.
		pattern: String,
	},
	/// Runtime manifest client entry URL was not a manifest-listed public output.
	UnlistedClientEntryPublicOutput {
		/// Unlisted module/dependency/CSS public path.
		public_path: String,
	},
	/// Runtime manifest client core URL was not a manifest-listed public output.
	UnlistedClientCorePublicOutput {
		/// Unlisted client core public path.
		public_path: String,
	},
	/// Runtime manifest view module URL was not a manifest-listed public output.
	UnlistedViewModulePublicOutput {
		/// Declared view route pattern.
		pattern: String,
		/// Unlisted module/dependency/CSS public path.
		public_path: String,
	},
}

impl std::fmt::Display for RuntimeSnapshotError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::MissingClientBuildId => {
				f.write_str("runtime snapshot requires a client build identifier")
			}
			Self::ManifestGraphMismatch {
				field,
				graph_value,
				manifest_value,
			} => write!(
				f,
				"runtime manifest {field} value {manifest_value:?} does not match graph value {graph_value:?}"
			),
			Self::Plan { source } => write!(f, "{source}"),
			Self::Assets { source } => write!(f, "{source}"),
			Self::MissingViewSearchSchema { pattern } => {
				write!(
					f,
					"runtime manifest is missing search schema for view {pattern:?}"
				)
			}
			Self::UnexpectedViewSearchSchema { pattern } => write!(
				f,
				"runtime manifest contains search schema for undeclared view {pattern:?}"
			),
			Self::MissingViewModule { pattern } => {
				write!(
					f,
					"runtime manifest is missing browser module for view {pattern:?}"
				)
			}
			Self::DuplicateViewModule { pattern } => {
				write!(
					f,
					"runtime manifest contains duplicate browser module for view {pattern:?}"
				)
			}
			Self::UnexpectedViewModule { pattern } => write!(
				f,
				"runtime manifest contains browser module for undeclared view {pattern:?}"
			),
			Self::UnlistedClientEntryPublicOutput { public_path } => write!(
				f,
				"runtime manifest client entry output {public_path:?} is not manifest-listed"
			),
			Self::UnlistedClientCorePublicOutput { public_path } => write!(
				f,
				"runtime manifest client core output {public_path:?} is not manifest-listed"
			),
			Self::UnlistedViewModulePublicOutput {
				pattern,
				public_path,
			} => write!(
				f,
				"runtime manifest view {pattern:?} module output {public_path:?} is not manifest-listed"
			),
		}
	}
}

impl std::error::Error for RuntimeSnapshotError {}

fn validate_manifest_view_contracts(
	input: &RuntimeSnapshotInput,
) -> Result<(), RuntimeSnapshotError> {
	let declared_patterns = input
		.graph
		.views()
		.iter()
		.map(|view| view.pattern().to_owned())
		.collect::<BTreeSet<_>>();
	for pattern in input.manifest.search_schemas().keys() {
		if !declared_patterns.contains(pattern) {
			return Err(RuntimeSnapshotError::UnexpectedViewSearchSchema {
				pattern: pattern.clone(),
			});
		}
	}
	let mut module_patterns = BTreeSet::new();
	for module in input.manifest.view_modules() {
		if !module_patterns.insert(module.pattern().to_owned()) {
			return Err(RuntimeSnapshotError::DuplicateViewModule {
				pattern: module.pattern().to_owned(),
			});
		}
		if !declared_patterns.contains(module.pattern()) {
			return Err(RuntimeSnapshotError::UnexpectedViewModule {
				pattern: module.pattern().to_owned(),
			});
		}
	}
	let public_filepaths = input
		.manifest
		.public_filepaths()
		.iter()
		.map(String::as_str)
		.collect::<BTreeSet<_>>();
	validate_manifest_client_outputs(input.manifest(), &public_filepaths)?;
	for view in input.graph.views() {
		if !input.manifest.search_schemas().contains_key(view.pattern()) {
			return Err(RuntimeSnapshotError::MissingViewSearchSchema {
				pattern: view.pattern().to_owned(),
			});
		}
		let module = input.manifest.view_module(view.pattern()).ok_or_else(|| {
			RuntimeSnapshotError::MissingViewModule {
				pattern: view.pattern().to_owned(),
			}
		})?;
		for public_path in std::iter::once(module.import_url())
			.chain(module.dep_urls().iter().map(String::as_str))
			.chain(module.css_bundle_urls().iter().map(String::as_str))
		{
			if !manifest_allows_module_url(&input.manifest, &public_filepaths, public_path) {
				return Err(RuntimeSnapshotError::UnlistedViewModulePublicOutput {
					pattern: view.pattern().to_owned(),
					public_path: public_path.to_owned(),
				});
			}
		}
	}
	Ok(())
}

fn validate_manifest_client_outputs(
	manifest: &RuntimeManifest,
	public_filepaths: &BTreeSet<&str>,
) -> Result<(), RuntimeSnapshotError> {
	for public_path in std::iter::once(manifest.client_entry().url())
		.chain(
			manifest
				.client_entry()
				.dep_urls()
				.iter()
				.map(String::as_str),
		)
		.chain(
			manifest
				.client_entry()
				.css_bundle_urls()
				.iter()
				.map(String::as_str),
		) {
		if !public_path.is_empty()
			&& !manifest_allows_module_url(manifest, public_filepaths, public_path)
		{
			return Err(RuntimeSnapshotError::UnlistedClientEntryPublicOutput {
				public_path: public_path.to_owned(),
			});
		}
	}
	if let Some(client_core_assets) = manifest.client_core_assets() {
		for public_path in [
			client_core_assets.module_url(),
			client_core_assets.wasm_url(),
		] {
			if !public_path.is_empty()
				&& !manifest_allows_module_url(manifest, public_filepaths, public_path)
			{
				return Err(RuntimeSnapshotError::UnlistedClientCorePublicOutput {
					public_path: public_path.to_owned(),
				});
			}
		}
	}
	Ok(())
}

fn manifest_allows_module_url(
	manifest: &RuntimeManifest,
	public_filepaths: &BTreeSet<&str>,
	module_url: &str,
) -> bool {
	public_filepaths.contains(module_url)
		|| manifest_allows_dev_vite_module_url(manifest, module_url)
}

fn manifest_allows_dev_vite_module_url(manifest: &RuntimeManifest, module_url: &str) -> bool {
	let port = manifest.dev_vite_server_port();
	if port <= 0 {
		return false;
	}
	let Ok(port) = u16::try_from(port) else {
		return false;
	};
	let Ok(url) = Url::parse(module_url) else {
		return false;
	};
	url.scheme() == "http" && url.host_str() == Some("127.0.0.1") && url.port() == Some(port)
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;

	use super::*;
	use crate::execution_engine::{HandlerOutput, RequestExecutionReport};
	use crate::framework_graph::{
		FrameworkConfig, FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration,
		StaticAssetDeclaration, ViewDeclaration,
	};
	use crate::runtime_manifest::{
		ClientCoreAssets, ClientModule, RuntimeManifest, RuntimeViewModule,
	};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn graph() -> FrameworkGraph {
		graph_with_config(FrameworkConfig::default())
	}

	fn resource_declarations(config: FrameworkConfig) -> FrameworkDeclarations {
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		declarations
	}

	fn graph_with_resource_config(config: FrameworkConfig) -> FrameworkGraph {
		FrameworkGraph::compile(resource_declarations(config)).unwrap()
	}

	fn graph_with_config(config: FrameworkConfig) -> FrameworkGraph {
		let mut declarations = resource_declarations(config);
		declarations.add_static_asset(StaticAssetDeclaration::new(
			"app.css",
			"/static/app.hash.css",
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	fn graph_with_view() -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	#[tokio::test]
	async fn snapshot_executes_against_committed_plan_and_assets() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph(), manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("ping"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!("pong")))
		});

		let resource = snapshot
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let asset = snapshot
			.execute(
				RequestInput::new(Method::GET, "/static/app.hash.css"),
				&handlers,
			)
			.await
			.unwrap();

		assert_eq!(snapshot.client_build_id(), "build-id");
		assert!(matches!(resource, RequestExecutionReport::Resource(_)));
		assert!(matches!(asset, RequestExecutionReport::PublicAsset(_)));
	}

	#[tokio::test]
	async fn snapshot_static_base_comes_from_manifest_not_app_config() {
		let graph = graph_with_resource_config(FrameworkConfig::new("/wrong-static/"));
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("ping"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!("pong")))
		});

		let resource = snapshot
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let unknown_resource = snapshot
			.execute(RequestInput::new(Method::GET, "/wrong-api/ping"), &handlers)
			.await
			.unwrap();
		let asset = snapshot
			.execute(
				RequestInput::new(Method::GET, "/static/app.hash.css"),
				&handlers,
			)
			.await
			.unwrap();

		assert!(matches!(resource, RequestExecutionReport::Resource(_)));
		assert!(matches!(unknown_resource, RequestExecutionReport::NotFound));
		assert!(matches!(asset, RequestExecutionReport::PublicAsset(_)));
	}

	#[test]
	fn snapshot_rejects_uncommitted_filemap_outputs() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/other.hash.css".to_owned())]),
			Vec::new(),
		);
		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph(), manifest)).unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::Assets {
				source: AssetCapabilityError::InvalidPublicFileMapOutput { .. }
			}
		));
	}

	#[test]
	fn snapshot_rejects_view_modules_not_listed_as_public_outputs() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			Vec::new(),
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnlistedViewModulePublicOutput { .. }
		));
	}

	#[test]
	fn snapshot_rejects_client_entry_not_listed_as_public_output() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		)
		.with_client_entry(ClientModule::new(
			"/static/entry.js",
			Vec::new(),
			Vec::new(),
		));

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnlistedClientEntryPublicOutput { public_path }
				if public_path == "/static/entry.js"
		));
	}

	#[test]
	fn snapshot_rejects_client_core_not_listed_as_public_output() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned(), "/static/entry.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		)
		.with_client_entry(ClientModule::new(
			"/static/entry.js",
			Vec::new(),
			Vec::new(),
		))
		.with_client_core_assets(Some(ClientCoreAssets::new(
			"/static/core.js",
			"/static/core.wasm",
		)));

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnlistedClientCorePublicOutput { public_path }
				if public_path == "/static/core.js"
		));
	}

	#[test]
	fn snapshot_accepts_dev_vite_client_entry_and_view_modules() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			Vec::new(),
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"http://127.0.0.1:5173/src/root.tsx",
				Vec::new(),
				Vec::new(),
			)],
		)
		.with_client_entry(ClientModule::new(
			"http://127.0.0.1:5173/src/entry.tsx",
			Vec::new(),
			Vec::new(),
		))
		.with_dev_metadata(5173, 4173, "refresh-token");

		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest));

		assert!(snapshot.is_ok());
	}

	#[test]
	fn snapshot_rejects_dev_vite_client_entry_on_wrong_port() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		)
		.with_client_entry(ClientModule::new(
			"http://127.0.0.1:5174/src/entry.tsx",
			Vec::new(),
			Vec::new(),
		))
		.with_dev_metadata(5173, 4173, "refresh-token");

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnlistedClientEntryPublicOutput { public_path }
				if public_path == "http://127.0.0.1:5174/src/entry.tsx"
		));
	}

	#[test]
	fn snapshot_rejects_missing_view_search_schema() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			Vec::new(),
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::MissingViewSearchSchema { .. }
		));
	}

	#[test]
	fn snapshot_rejects_missing_view_module() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			Vec::new(),
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::MissingViewModule { .. }
		));
	}

	#[test]
	fn snapshot_rejects_unexpected_view_search_schema() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([
				("/".to_owned(), serde_json::json!({})),
				("/extra".to_owned(), serde_json::json!({})),
			]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnexpectedViewSearchSchema { pattern } if pattern == "/extra"
		));
	}

	#[test]
	fn snapshot_rejects_duplicate_view_modules() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![
				RuntimeViewModule::new("/", "/static/root.js", Vec::new(), Vec::new()),
				RuntimeViewModule::new("/", "/static/root.js", Vec::new(), Vec::new()),
			],
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::DuplicateViewModule { pattern } if pattern == "/"
		));
	}

	#[test]
	fn snapshot_rejects_unexpected_view_modules() {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned(), "/static/extra.js".to_owned()],
			BTreeMap::new(),
			vec![
				RuntimeViewModule::new("/", "/static/root.js", Vec::new(), Vec::new()),
				RuntimeViewModule::new("/extra", "/static/extra.js", Vec::new(), Vec::new()),
			],
		);

		let error =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph_with_view(), manifest))
				.unwrap_err();

		assert!(matches!(
			error,
			RuntimeSnapshotError::UnexpectedViewModule { pattern } if pattern == "/extra"
		));
	}
}
