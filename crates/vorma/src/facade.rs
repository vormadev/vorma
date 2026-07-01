//! Clean Rust declaration facade for lowering public API facts into the framework graph.

use std::collections::{BTreeMap, BTreeSet};
use std::future::Future;
use std::sync::Arc;

use crate::contracts::{DocumentContract, RouteTypeContract, TypeDef};
use crate::execution_engine::{HandlerRegistry, RuntimeHandler};
use crate::form_data::FormData;
use crate::framework_graph::{
	FrameworkConfig, FrameworkDeclarations, FrameworkGraph, GraphError, HandlerId,
	MiddlewareDeclaration, ResourceDeclaration, ResourceKind, StaticAssetDeclaration,
	ViewDeclaration,
};
use crate::resource_body::ResourceOutput;
use crate::route_input::{
	ResourceInput, RouteContractFacts, ViewInput, route_contract_from_resolvers,
	search_schema_resolver, type_resolver,
};
use crate::runtime_app::CommittedRuntimeApp;
use crate::runtime_host::{CommittedRuntimeHost, RuntimeHostError};
use crate::runtime_manifest::RuntimeManifest;
use crate::runtime_snapshot::{RuntimeSnapshot, RuntimeSnapshotError, RuntimeSnapshotInput};
use crate::tsgen::Type;
use crate::typed_handler::{
	TypedHandlerContext, form_data_runtime_handler, typed_resource_runtime_handler,
	typed_runtime_handler,
};
use vorma_tasks::{Tasks, TasksOptions};

/// Facade-owned application declarations and runtime handler bindings.
pub struct AppFacade {
	declarations: FrameworkDeclarations,
	handlers: HandlerRegistry,
	handler_ids: BTreeSet<HandlerId>,
	type_defs_by_key: BTreeMap<String, TypeDef>,
	type_names_by_key: BTreeMap<String, String>,
}

/// Resource route declaration facts before binding a runtime handler.
pub struct ResourceRouteSpec {
	method: http::Method,
	pattern: String,
	kind: Option<ResourceKind>,
	input_schema: Option<serde_json::Value>,
	type_contract: RouteTypeContract,
	handler_id: HandlerId,
}

impl ResourceRouteSpec {
	/// Create resource route declaration facts.
	pub fn new(
		method: http::Method,
		pattern: impl Into<String>,
		kind: Option<ResourceKind>,
		input_schema: Option<serde_json::Value>,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
	) -> Self {
		Self {
			method,
			pattern: pattern.into(),
			kind,
			input_schema,
			type_contract,
			handler_id,
		}
	}

	fn into_declaration(self) -> ResourceDeclaration {
		ResourceDeclaration::new(
			self.method,
			self.pattern,
			self.kind,
			self.input_schema,
			self.type_contract,
			self.handler_id,
		)
	}
}

impl AppFacade {
	/// Create a facade with explicit framework configuration.
	pub fn new(config: FrameworkConfig) -> Self {
		Self::new_with_tasks(config, Tasks::new(TasksOptions::default()))
	}

	/// Create a facade with explicit framework configuration and task runtime.
	pub fn new_with_tasks(config: FrameworkConfig, tasks: Tasks<crate::Error>) -> Self {
		Self {
			declarations: FrameworkDeclarations::new(config),
			handlers: HandlerRegistry::new(tasks),
			handler_ids: BTreeSet::new(),
			type_defs_by_key: BTreeMap::new(),
			type_names_by_key: BTreeMap::new(),
		}
	}

	/// Read declarations accumulated by this facade.
	pub fn declarations(&self) -> &FrameworkDeclarations {
		&self.declarations
	}

	/// Read handlers accumulated by this facade.
	pub fn handlers(&self) -> &HandlerRegistry {
		&self.handlers
	}

	/// Set the document contract.
	pub fn set_document(&mut self, document: DocumentContract) -> &mut Self {
		self.declarations.set_document(document);
		self
	}

	/// Add a generated TypeScript definition contract.
	pub fn add_type_def(&mut self, type_def: TypeDef) -> Result<&mut Self, FacadeError> {
		self.register_type_def(type_def)?;
		Ok(self)
	}

	/// Add a static asset declaration.
	pub fn add_static_asset(&mut self, asset: StaticAssetDeclaration) -> &mut Self {
		self.declarations.add_static_asset(asset);
		self
	}

	/// Add a static asset declaration from source and public paths.
	pub fn add_static_asset_paths(
		&mut self,
		source_path: impl Into<String>,
		public_path: impl Into<String>,
	) -> &mut Self {
		self.add_static_asset(StaticAssetDeclaration::new(source_path, public_path))
	}

	/// Add a middleware declaration and its runtime handler.
	pub fn add_middleware<H>(
		&mut self,
		patterns: Vec<String>,
		methods: Vec<http::Method>,
		handler_id: HandlerId,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.register_handler(handler_id.clone(), handler)?;
		self.declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id)
				.with_patterns(patterns)
				.with_methods(methods),
		);
		Ok(self)
	}

	/// Add a view declaration and its runtime handler.
	pub fn add_view<H>(
		&mut self,
		view: ViewDeclaration,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.register_handler(view.handler_id().clone(), handler)?;
		self.declarations.add_view(view);
		Ok(self)
	}

	/// Add a view route declaration and its runtime handler.
	pub fn add_view_route<H>(
		&mut self,
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		search_schema: serde_json::Value,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.add_view(
			ViewDeclaration::new(
				pattern,
				client_file,
				search_schema,
				type_contract,
				handler_id,
			),
			handler,
		)
	}

	/// Add a resource declaration and its runtime handler.
	pub fn add_resource<H>(
		&mut self,
		resource: ResourceDeclaration,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.register_handler(resource.handler_id().clone(), handler)?;
		self.declarations.add_resource(resource);
		Ok(self)
	}

	/// Add a resource route declaration and its runtime handler.
	pub fn add_resource_route<H>(
		&mut self,
		spec: ResourceRouteSpec,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.add_resource(spec.into_declaration(), handler)
	}

	/// Compile the facade into a graph plus the corresponding runtime handlers.
	pub fn compile(self) -> Result<CompiledAppFacade, FacadeError> {
		let graph = FrameworkGraph::compile(self.declarations)
			.map_err(|source| FacadeError::Graph { source })?;
		Ok(CompiledAppFacade {
			graph,
			handlers: self.handlers,
		})
	}

	fn register_handler<H>(&mut self, handler_id: HandlerId, handler: H) -> Result<(), FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.ensure_handler_id_available(&handler_id)?;
		self.handler_ids.insert(handler_id.clone());
		self.handlers.insert(handler_id, handler);
		Ok(())
	}

	fn ensure_handler_id_available(&self, handler_id: &HandlerId) -> Result<(), FacadeError> {
		if self.handler_ids.contains(handler_id) {
			return Err(FacadeError::DuplicateHandlerId {
				handler_id: handler_id.clone(),
			});
		}
		Ok(())
	}

	fn register_type_def(&mut self, type_def: TypeDef) -> Result<(), FacadeError> {
		let key = type_def.key().to_owned();
		let name = type_def.name().to_owned();
		if let Some(existing) = self.type_defs_by_key.get(&key) {
			if existing == &type_def {
				return Ok(());
			}
			return Err(FacadeError::ConflictingTypeDef { key });
		}
		if let Some(existing_key) = self.type_names_by_key.get(&name)
			&& existing_key != &key
		{
			return Err(FacadeError::DuplicateTypeName { name });
		}
		self.type_defs_by_key.insert(key.clone(), type_def.clone());
		self.type_names_by_key.insert(name, key);
		self.declarations.add_type_def(type_def);
		Ok(())
	}
}

impl Default for AppFacade {
	fn default() -> Self {
		Self::new(FrameworkConfig::default())
	}
}

/// Stateful facade for public-style typed handler declarations.
pub struct StatefulAppFacade<S> {
	state: Arc<S>,
	facade: AppFacade,
}

impl<S> StatefulAppFacade<S>
where
	S: Send + Sync + 'static,
{
	/// Create a stateful facade from framework configuration and shared state.
	pub fn new(config: FrameworkConfig, state: S) -> Self {
		Self::new_with_tasks(config, state, Tasks::new(TasksOptions::default()))
	}

	/// Create a stateful facade from framework configuration, shared state, and task runtime.
	pub fn new_with_tasks(config: FrameworkConfig, state: S, tasks: Tasks<crate::Error>) -> Self {
		Self {
			state: Arc::new(state),
			facade: AppFacade::new_with_tasks(config, tasks),
		}
	}

	/// Shared application state.
	pub fn state(&self) -> &Arc<S> {
		&self.state
	}

	/// Read declarations accumulated by this facade.
	pub fn declarations(&self) -> &FrameworkDeclarations {
		self.facade.declarations()
	}

	/// Set the document contract.
	pub fn set_document(&mut self, document: DocumentContract) -> &mut Self {
		self.facade.set_document(document);
		self
	}

	/// Add a generated TypeScript definition contract.
	pub fn add_type_def(&mut self, type_def: TypeDef) -> Result<&mut Self, FacadeError> {
		self.facade.add_type_def(type_def)?;
		Ok(self)
	}

	/// Add a static asset declaration from source and public paths.
	pub fn add_static_asset_paths(
		&mut self,
		source_path: impl Into<String>,
		public_path: impl Into<String>,
	) -> &mut Self {
		self.facade.add_static_asset_paths(source_path, public_path);
		self
	}

	/// Add a typed middleware declaration and handler.
	pub fn add_typed_middleware<F, Fut>(
		&mut self,
		patterns: Vec<String>,
		methods: Vec<http::Method>,
		handler_id: HandlerId,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		F: Fn(TypedHandlerContext<S, ()>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<(), crate::Error>> + Send + 'static,
	{
		self.facade.add_middleware(
			patterns,
			methods,
			handler_id,
			typed_runtime_handler(Arc::clone(&self.state), handler),
		)?;
		Ok(self)
	}

	/// Add a middleware declaration with an already-adapted runtime handler.
	pub fn add_runtime_middleware<H>(
		&mut self,
		patterns: Vec<String>,
		methods: Vec<http::Method>,
		handler_id: HandlerId,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.facade
			.add_middleware(patterns, methods, handler_id, handler)?;
		Ok(self)
	}

	/// Add a view route declaration with an already-adapted runtime handler.
	pub fn add_runtime_view_route<H>(
		&mut self,
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		search_schema: serde_json::Value,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.facade.add_view_route(
			pattern,
			client_file,
			search_schema,
			type_contract,
			handler_id,
			handler,
		)?;
		Ok(self)
	}

	/// Add a resource route declaration with an already-adapted runtime handler.
	pub fn add_runtime_resource_route<H>(
		&mut self,
		spec: ResourceRouteSpec,
		handler: H,
	) -> Result<&mut Self, FacadeError>
	where
		H: RuntimeHandler + 'static,
	{
		self.facade.add_resource_route(spec, handler)?;
		Ok(self)
	}

	/// Add a public-style view route declaration and handler.
	pub fn add_view_route<I, O, F, Fut>(
		&mut self,
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		handler_id: HandlerId,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		I: ViewInput,
		O: serde::Serialize + Type + Send + Sync + 'static,
		F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.ensure_handler_id_available(&handler_id)?;
		let route_contract = route_contract_for::<I, O>()?;
		let search_schema = search_schema_resolver::<I>()
			.map_err(|message| FacadeError::RouteContract { message })?;
		self.register_type_defs(route_contract.type_defs)?;
		self.add_typed_view_route::<I, O, F, Fut>(
			pattern,
			client_file,
			search_schema,
			route_contract.type_contract,
			handler_id,
			handler,
		)
	}

	/// Add a public-style resource route declaration and handler.
	pub fn add_resource_route<I, O, F, Fut>(
		&mut self,
		method: http::Method,
		pattern: impl Into<String>,
		kind: Option<ResourceKind>,
		handler_id: HandlerId,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		I: ResourceInput + serde::de::DeserializeOwned + Type,
		O: ResourceOutput + Type,
		F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.ensure_handler_id_available(&handler_id)?;
		let route_contract = route_contract_for::<I, O>()?;
		let input_schema = search_schema_resolver::<I>().ok();
		self.register_type_defs(route_contract.type_defs)?;
		self.add_typed_resource_route::<I, O, F, Fut>(
			ResourceRouteSpec::new(
				method,
				pattern,
				kind,
				input_schema,
				route_contract.type_contract,
				handler_id,
			),
			handler,
		)
	}

	/// Add a public-style `FormData` resource route declaration and handler.
	pub fn add_form_data_resource<O, F, Fut>(
		&mut self,
		method: http::Method,
		pattern: impl Into<String>,
		kind: Option<ResourceKind>,
		handler_id: HandlerId,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		O: ResourceOutput + Type,
		F: Fn(TypedHandlerContext<S, FormData>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.ensure_handler_id_available(&handler_id)?;
		let route_contract = route_contract_for::<FormData, O>()?;
		self.register_type_defs(route_contract.type_defs)?;
		self.add_form_data_resource_route::<O, F, Fut>(
			ResourceRouteSpec::new(
				method,
				pattern,
				kind,
				None,
				route_contract.type_contract,
				handler_id,
			),
			handler,
		)
	}

	/// Add a typed view route declaration and handler.
	pub fn add_typed_view_route<I, O, F, Fut>(
		&mut self,
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		search_schema: serde_json::Value,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		I: serde::de::DeserializeOwned + Send + Sync + 'static,
		O: serde::Serialize + Send + Sync + 'static,
		F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.add_view_route(
			pattern,
			client_file,
			search_schema,
			type_contract,
			handler_id,
			typed_runtime_handler(Arc::clone(&self.state), handler),
		)?;
		Ok(self)
	}

	/// Add a typed resource route declaration and handler.
	pub fn add_typed_resource_route<I, O, F, Fut>(
		&mut self,
		spec: ResourceRouteSpec,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		I: serde::de::DeserializeOwned + Send + Sync + 'static,
		O: ResourceOutput,
		F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.add_resource_route(
			spec,
			typed_resource_runtime_handler(Arc::clone(&self.state), handler),
		)?;
		Ok(self)
	}

	/// Add a `FormData` resource route declaration and handler.
	pub fn add_form_data_resource_route<O, F, Fut>(
		&mut self,
		spec: ResourceRouteSpec,
		handler: F,
	) -> Result<&mut Self, FacadeError>
	where
		O: ResourceOutput,
		F: Fn(TypedHandlerContext<S, FormData>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
	{
		self.facade.add_resource_route(
			spec,
			form_data_runtime_handler(Arc::clone(&self.state), handler),
		)?;
		Ok(self)
	}

	/// Compile the facade into a graph plus the corresponding runtime handlers.
	pub fn compile(self) -> Result<CompiledAppFacade, FacadeError> {
		self.facade.compile()
	}

	fn register_type_defs(&mut self, type_defs: Vec<TypeDef>) -> Result<(), FacadeError> {
		for type_def in type_defs {
			self.facade.add_type_def(type_def)?;
		}
		Ok(())
	}
}

fn route_contract_for<I, O>() -> Result<RouteContractFacts, FacadeError>
where
	I: Type,
	O: Type,
{
	route_contract_from_resolvers(type_resolver::<I>, type_resolver::<O>)
		.map_err(|message| FacadeError::RouteContract { message })
}

/// Compiled framework graph and the runtime handler registry it references.
#[derive(Clone)]
pub struct CompiledAppFacade {
	graph: FrameworkGraph,
	handlers: HandlerRegistry,
}

impl CompiledAppFacade {
	/// Compiled framework graph.
	pub fn graph(&self) -> &FrameworkGraph {
		&self.graph
	}

	/// Runtime handlers referenced by the compiled graph.
	pub fn handlers(&self) -> &HandlerRegistry {
		&self.handlers
	}

	/// Consume the compiled facade into its graph and handlers.
	pub fn into_graph_and_handlers(self) -> (FrameworkGraph, HandlerRegistry) {
		(self.graph, self.handlers)
	}

	/// Consume the compiled facade into a committed runtime application.
	pub fn into_runtime_app(
		self,
		manifest: RuntimeManifest,
	) -> Result<CommittedRuntimeApp, RuntimeSnapshotError> {
		let snapshot = RuntimeSnapshot::compile(RuntimeSnapshotInput::new(self.graph, manifest))?;
		Ok(CommittedRuntimeApp::new(snapshot, self.handlers))
	}

	/// Consume the compiled facade into a committed runtime host.
	pub fn into_runtime_host<P>(
		self,
		manifest: RuntimeManifest,
		asset_provider: P,
		request_body_limit: usize,
	) -> Result<CommittedRuntimeHost<P>, FacadeRuntimeHostError>
	where
		P: crate::asset_body_provider::PublicAssetBodyProvider + 'static,
	{
		let app = self
			.into_runtime_app(manifest)
			.map_err(|source| FacadeRuntimeHostError::RuntimeSnapshot { source })?;
		CommittedRuntimeHost::new(app, asset_provider, request_body_limit)
			.map_err(|source| FacadeRuntimeHostError::RuntimeHost { source })
	}
}

/// Runtime host assembly error from a compiled facade.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FacadeRuntimeHostError {
	/// Runtime snapshot compilation failed.
	RuntimeSnapshot {
		/// Source runtime snapshot error.
		source: RuntimeSnapshotError,
	},
	/// Runtime host assembly failed.
	RuntimeHost {
		/// Source runtime host error.
		source: RuntimeHostError,
	},
}

impl std::fmt::Display for FacadeRuntimeHostError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::RuntimeSnapshot { source } => write!(f, "compile runtime snapshot: {source:?}"),
			Self::RuntimeHost { source } => write!(f, "assemble runtime host: {source}"),
		}
	}
}

impl std::error::Error for FacadeRuntimeHostError {
	fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
		match self {
			Self::RuntimeSnapshot { .. } => None,
			Self::RuntimeHost { source } => Some(source),
		}
	}
}

/// Facade declaration conversion error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FacadeError {
	/// Multiple declarations tried to bind the same handler identifier.
	DuplicateHandlerId {
		/// Reused handler identifier.
		handler_id: HandlerId,
	},
	/// Multiple declarations tried to bind the same type key to different definitions.
	ConflictingTypeDef {
		/// Conflicting type key.
		key: String,
	},
	/// Multiple declarations tried to export different type keys under one TypeScript name.
	DuplicateTypeName {
		/// Reused TypeScript name.
		name: String,
	},
	/// Route input/output type contract resolution failed.
	RouteContract {
		/// Contract resolution error.
		message: String,
	},
	/// Graph compilation failed.
	Graph {
		/// Source graph error.
		source: GraphError,
	},
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use bytes::Bytes;
	use http::header::{HeaderName, HeaderValue};
	use http::{Method, StatusCode};
	use http_body_util::{BodyExt, Full};
	use serde::{Deserialize, Serialize};
	use tower_service::Service;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyError;
	use crate::contracts::{DocumentAttributeContract, RouteTypeContract, TypeRefContract};
	use crate::execution_engine::{HandlerOutput, RequestInput};
	use crate::response_finalizer::ResponseEffects;
	use crate::runtime_app::RuntimeAppRequest;
	use crate::runtime_manifest::RuntimeManifest;
	use crate::test_support::route_type_contract;
	use crate::tsgen::{FieldDef, Result as TsResult, Type, TypeDef, TypeRef, TypeRegistry};

	const TEST_ASSET_PROVIDER_ERROR: &str = "asset provider should not run";
	const TEST_CLIENT_BUILD_ID: &str = "build-id";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_INPUT_TYPE_KEY: &str = "TestInput";
	const TEST_INPUT_TYPE_NAME: &str = "TestInput";
	const TEST_MIDDLEWARE_HEADER: &str = "x-stateful-middleware";
	const TEST_MIDDLEWARE_HEADER_VALUE: &str = "yes";
	const TEST_MIDDLEWARE_HANDLER_ID: &str = "middleware";
	const TEST_OUTPUT_TYPE_KEY: &str = "TestOutput";
	const TEST_OUTPUT_TYPE_NAME: &str = "TestOutput";
	const TEST_RESOURCE_HANDLER_ID: &str = "hello";
	const TEST_PUBLIC_RESOURCE_HANDLER_ID: &str = "public_hello";
	const TEST_RESOURCE_PATTERN: &str = "/api/hello";
	const TEST_RESOURCE_URI: &str = "/api/hello";

	#[derive(Clone, Debug, Deserialize)]
	struct TestInput {
		name: String,
	}

	impl Type for TestInput {
		fn type_ref() -> TypeRef {
			TypeRef::named_with_key(TEST_INPUT_TYPE_KEY, TEST_INPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record_with_key(
				TEST_INPUT_TYPE_KEY,
				TEST_INPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("name")],
			));
			Ok(())
		}
	}

	#[derive(Clone, Debug, Serialize)]
	struct TestOutput {
		message: String,
	}

	impl Type for TestOutput {
		fn type_ref() -> TypeRef {
			TypeRef::named_with_key(TEST_OUTPUT_TYPE_KEY, TEST_OUTPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record_with_key(
				TEST_OUTPUT_TYPE_KEY,
				TEST_OUTPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("message")],
			));
			Ok(())
		}
	}

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn test_input_type_contract() -> RouteTypeContract {
		RouteTypeContract::new(
			TypeRefContract::Named {
				key: TEST_INPUT_TYPE_KEY.to_owned(),
				name: TEST_INPUT_TYPE_NAME.to_owned(),
			},
			TypeRefContract::Unknown,
		)
	}

	fn test_input_type_def() -> TypeDef {
		TypeDef::Record {
			key: TEST_INPUT_TYPE_KEY.to_owned(),
			name: TEST_INPUT_TYPE_NAME.to_owned(),
			fields: vec![FieldDef::new("name", TypeRefContract::String, false)],
		}
	}

	#[tokio::test]
	async fn facade_compiles_graph_and_matching_handler_registry() {
		let mut facade: AppFacade = AppFacade::new(FrameworkConfig::default());
		facade
			.set_document(DocumentContract::new(
				vec![DocumentAttributeContract::new("lang", "en", false, false)],
				Vec::new(),
				Vec::new(),
				Vec::new(),
				Vec::new(),
			))
			.add_static_asset_paths("app.css", "/static/app.hash.css");
		facade
			.add_type_def(TypeDef::Record {
				key: "User".to_owned(),
				name: "User".to_owned(),
				fields: vec![FieldDef::new("name", TypeRefContract::String, false)],
			})
			.unwrap();
		facade
			.add_resource_route(
				ResourceRouteSpec::new(
					Method::GET,
					"/api/ping",
					None,
					Some(serde_json::json!({})),
					route_type_contract(),
					handler_id("resource"),
				),
				|_| async {
					let mut effects = ResponseEffects::default();
					effects.set_status(StatusCode::CREATED);
					Ok(HandlerOutput::body(Bytes::from_static(b"pong")).with_effects(effects))
				},
			)
			.unwrap();

		let compiled = facade.compile().unwrap();

		assert_eq!(
			compiled.graph().document().html_attributes()[0].name(),
			"lang"
		);
		assert_eq!(compiled.graph().type_defs().len(), 1);
		assert_eq!(
			compiled.graph().resources()[0].handler_id().as_str(),
			"resource"
		);

		let app = compiled
			.into_runtime_app(RuntimeManifest::new(
				TEST_CLIENT_BUILD_ID,
				TEST_PUBLIC_STATIC_BASE,
				"",
				BTreeMap::new(),
				vec!["/static/app.hash.css".to_owned()],
				BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
				Vec::new(),
			))
			.unwrap();
		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&|_| async { Err(PublicAssetBodyError::new(TEST_ASSET_PROVIDER_ERROR)) },
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::CREATED);
		assert_eq!(response.body(), &Bytes::from_static(b"pong"));
	}

	#[tokio::test]
	async fn stateful_facade_compiles_typed_routes_into_runtime_app() {
		let mut facade: StatefulAppFacade<String> =
			StatefulAppFacade::new(FrameworkConfig::default(), "hello ".to_owned());
		facade.add_type_def(test_input_type_def()).unwrap();
		facade
			.add_typed_resource_route::<TestInput, TestOutput, _, _>(
				ResourceRouteSpec::new(
					Method::POST,
					TEST_RESOURCE_PATTERN,
					None,
					Some(serde_json::json!({"type": "object"})),
					test_input_type_contract(),
					handler_id(TEST_RESOURCE_HANDLER_ID),
				),
				|ctx| async move {
					ctx.resource_response().set_status(StatusCode::CREATED);
					Ok(TestOutput {
						message: format!("{}{}", ctx.state(), ctx.input().name),
					})
				},
			)
			.unwrap();

		let mut service = facade
			.compile()
			.unwrap()
			.into_runtime_host(
				RuntimeManifest::new(
					TEST_CLIENT_BUILD_ID,
					TEST_PUBLIC_STATIC_BASE,
					"",
					BTreeMap::new(),
					Vec::new(),
					BTreeMap::new(),
					Vec::new(),
				),
				|_| async { Err(PublicAssetBodyError::new(TEST_ASSET_PROVIDER_ERROR)) },
				1024,
			)
			.unwrap()
			.into_service();
		let request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_RESOURCE_URI)
			.body(Full::new(Bytes::from_static(br#"{"name":"Ada"}"#)))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let status = response.status();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let body: serde_json::Value = serde_json::from_slice(&body).unwrap();

		assert_eq!(status, StatusCode::CREATED);
		assert_eq!(body["message"], "hello Ada");
	}

	#[tokio::test]
	async fn stateful_facade_derives_public_route_contracts_into_runtime_host() {
		let mut facade: StatefulAppFacade<String> =
			StatefulAppFacade::new(FrameworkConfig::default(), "hello ".to_owned());
		facade
			.add_resource_route::<TestInput, TestOutput, _, _>(
				Method::POST,
				TEST_RESOURCE_PATTERN,
				None,
				handler_id(TEST_PUBLIC_RESOURCE_HANDLER_ID),
				|ctx| async move {
					Ok(TestOutput {
						message: format!("{}{}", ctx.state(), ctx.input().name),
					})
				},
			)
			.unwrap();

		let compiled = facade.compile().unwrap();
		assert_eq!(compiled.graph().type_defs().len(), 2);
		assert_eq!(
			compiled.graph().resources()[0].input_schema(),
			Some(&serde_json::json!({"name": "s"}))
		);
		assert_eq!(
			compiled.graph().resources()[0].type_contract().input(),
			&TypeRefContract::Named {
				key: TEST_INPUT_TYPE_KEY.to_owned(),
				name: TEST_INPUT_TYPE_NAME.to_owned(),
			}
		);

		let mut service = compiled
			.into_runtime_host(
				RuntimeManifest::new(
					TEST_CLIENT_BUILD_ID,
					TEST_PUBLIC_STATIC_BASE,
					"",
					BTreeMap::new(),
					Vec::new(),
					BTreeMap::new(),
					Vec::new(),
				),
				|_| async { Err(PublicAssetBodyError::new(TEST_ASSET_PROVIDER_ERROR)) },
				1024,
			)
			.unwrap()
			.into_service();
		let request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_RESOURCE_URI)
			.body(Full::new(Bytes::from_static(br#"{"name":"Ada"}"#)))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let body: serde_json::Value = serde_json::from_slice(&body).unwrap();

		assert_eq!(body["message"], "hello Ada");
	}

	#[tokio::test]
	async fn stateful_facade_compiles_typed_middleware_into_runtime_host() {
		let mut facade: StatefulAppFacade<String> =
			StatefulAppFacade::new(FrameworkConfig::default(), "middleware".to_owned());
		facade
			.add_typed_middleware::<_, _>(
				Vec::new(),
				Vec::new(),
				handler_id(TEST_MIDDLEWARE_HANDLER_ID),
				|ctx| async move {
					ctx.response().set_header(
						HeaderName::from_static(TEST_MIDDLEWARE_HEADER),
						HeaderValue::from_static(TEST_MIDDLEWARE_HEADER_VALUE),
					);
					Ok(())
				},
			)
			.unwrap()
			.add_typed_resource_route::<(), TestOutput, _, _>(
				ResourceRouteSpec::new(
					Method::GET,
					TEST_RESOURCE_PATTERN,
					None,
					None,
					route_type_contract(),
					handler_id(TEST_RESOURCE_HANDLER_ID),
				),
				|ctx| async move {
					Ok(TestOutput {
						message: ctx.state().clone(),
					})
				},
			)
			.unwrap();

		let mut service = facade
			.compile()
			.unwrap()
			.into_runtime_host(
				RuntimeManifest::new(
					TEST_CLIENT_BUILD_ID,
					TEST_PUBLIC_STATIC_BASE,
					"",
					BTreeMap::new(),
					Vec::new(),
					BTreeMap::new(),
					Vec::new(),
				),
				|_| async { Err(PublicAssetBodyError::new(TEST_ASSET_PROVIDER_ERROR)) },
				1024,
			)
			.unwrap()
			.into_service();
		let request = http::Request::builder()
			.method(Method::GET)
			.uri(TEST_RESOURCE_URI)
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		assert_eq!(
			response.headers().get(TEST_MIDDLEWARE_HEADER).unwrap(),
			TEST_MIDDLEWARE_HEADER_VALUE
		);
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let body: serde_json::Value = serde_json::from_slice(&body).unwrap();
		assert_eq!(body["message"], "middleware");
	}

	#[test]
	fn facade_rejects_duplicate_handler_ids_without_adding_second_declaration() {
		let mut facade: AppFacade = AppFacade::default();
		facade
			.add_resource(
				ResourceDeclaration::new(
					Method::GET,
					"/api/one",
					None,
					None,
					route_type_contract(),
					handler_id("shared"),
				),
				|_| async { Ok(HandlerOutput::empty()) },
			)
			.unwrap();

		let error = match facade.add_view(
			ViewDeclaration::new(
				"/two",
				"two.tsx",
				serde_json::json!({}),
				route_type_contract(),
				handler_id("shared"),
			),
			|_| async { Ok(HandlerOutput::empty()) },
		) {
			Ok(_) => panic!("expected duplicate handler id rejection"),
			Err(error) => error,
		};

		assert!(matches!(error, FacadeError::DuplicateHandlerId { .. }));
		assert_eq!(facade.declarations().resources().len(), 1);
		assert_eq!(facade.declarations().views().len(), 0);
	}

	#[test]
	fn facade_dedupes_identical_type_defs_and_rejects_conflicts() {
		let mut facade: AppFacade = AppFacade::default();
		let type_def = test_input_type_def();

		facade.add_type_def(type_def.clone()).unwrap();
		facade.add_type_def(type_def).unwrap();
		let conflicting_key = match facade.add_type_def(TypeDef::Alias {
			key: TEST_INPUT_TYPE_KEY.to_owned(),
			name: TEST_INPUT_TYPE_NAME.to_owned(),
			target: TypeRefContract::Integer,
		}) {
			Ok(_) => panic!("expected conflicting type key"),
			Err(error) => error,
		};
		let duplicate_name = match facade.add_type_def(TypeDef::Alias {
			key: "OtherInput".to_owned(),
			name: TEST_INPUT_TYPE_NAME.to_owned(),
			target: TypeRefContract::String,
		}) {
			Ok(_) => panic!("expected duplicate type name"),
			Err(error) => error,
		};

		assert_eq!(facade.declarations().type_defs().len(), 1);
		assert!(matches!(
			conflicting_key,
			FacadeError::ConflictingTypeDef { .. }
		));
		assert!(matches!(
			duplicate_name,
			FacadeError::DuplicateTypeName { .. }
		));
	}

	#[test]
	fn facade_surfaces_graph_errors() {
		let mut facade: AppFacade = AppFacade::default();
		facade
			.add_view(
				ViewDeclaration::new(
					"",
					"bad.tsx",
					serde_json::json!({}),
					route_type_contract(),
					handler_id("bad"),
				),
				|_| async { Ok(HandlerOutput::empty()) },
			)
			.unwrap();

		let error = match facade.compile() {
			Ok(_) => panic!("expected graph error"),
			Err(error) => error,
		};

		assert!(matches!(error, FacadeError::Graph { .. }));
	}
}
