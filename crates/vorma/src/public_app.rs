//! Public app declaration containers.

use std::future::Future;
use std::path::PathBuf;
use std::sync::Arc;

use crate::app_assembly::{AppAssembly, DEFAULT_REQUEST_BODY_LIMIT};
use crate::config::{Config, DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig};
use crate::document_builder::DocumentBuilder;
use crate::error::Error;
use crate::facade::{FacadeError, ResourceRouteSpec, StatefulAppFacade};
use crate::framework_graph::{HandlerId, ResourceKind};
use crate::route_input::{SearchSchemaResolver, TypeResolver, route_contract_from_resolvers};
use crate::static_route::{
	ErasedRouteHandler, MiddlewareCtx, middleware_runtime_handler, static_route_runtime_handler,
};

use vorma_tasks::Tasks;

const VIEW_HANDLER_ID_PREFIX: &str = "view:";
const RESOURCE_HANDLER_ID_PREFIX: &str = "resource:";
const MIDDLEWARE_HANDLER_ID_PREFIX: &str = "middleware:";

/// Complete app declaration consumed by both build and runtime host assembly.
pub struct AppConfig<S> {
	/*
	Canonical field order (and the order the example teaches): filesystem
	anchors → cargo target → public URL base → domain groups → app values →
	tunables. Path rule: `root_dir` is the one absolute anchor (PathBuf);
	every other path-ish field is a root-relative String fragment.
	*/
	/// Absolute application root used to resolve all relative config paths.
	pub root_dir: PathBuf,
	/// Build output directory, relative to [`Self::root_dir`] unless absolute.
	pub dist_dir: String,
	/// Cargo package/bin identity of the app server binary.
	pub server_target: ServerTarget,
	/// Public static asset base URL path.
	pub public_static_base: String,
	/// Frontend entry, Vite, package-manager, static, and critical-CSS config.
	pub frontend_config: FrontendConfig,
	/// Generated TypeScript output and supplemental declaration config.
	pub ts_gen_config: TsGenConfig,
	/// Dev watcher include/classification patterns.
	pub dev_watch_config: DevWatchConfig,
	/// Application state shared by handlers.
	pub state: S,
	/// Registered nested views.
	pub views: Views<S>,
	/// Registered resources.
	pub resources: Resources<S>,
	/// Registered middleware.
	pub middlewares: Middlewares<S>,
	/// Task runtime options.
	pub tasks_options: vorma_tasks::TasksOptions<crate::Error>,
	/// Document shell/default-head builder.
	pub document: DocumentBuilder,
	/// Maximum request body bytes Vorma will collect for its own handlers.
	pub request_body_limit: usize,
}

/// Normalized app ready to compile into runtime/build assembly.
pub struct App<S> {
	config: Config,
	assembly: AppAssembly<S>,
	_tasks: Tasks<crate::Error>,
}

/// Hidden build-facing app contract.
#[doc(hidden)]
pub struct AppBuildContract {
	graph: crate::framework_graph::FrameworkGraph,
	document_builder: DocumentBuilder,
}

impl AppBuildContract {
	/// Canonical framework graph.
	pub fn graph(&self) -> &crate::framework_graph::FrameworkGraph {
		&self.graph
	}

	/// Runtime document builder used to derive build document identity.
	pub fn document_builder(&self) -> &DocumentBuilder {
		&self.document_builder
	}

	/// Consume this contract into its owned parts.
	pub fn into_parts(self) -> (crate::framework_graph::FrameworkGraph, DocumentBuilder) {
		(self.graph, self.document_builder)
	}
}

impl<S> App<S>
where
	S: Send + Sync + 'static,
{
	/// Normalize public app configuration into fresh app assembly.
	pub(crate) fn from_app_config(app_config: AppConfig<S>) -> crate::Result<Self> {
		let AppConfig {
			root_dir,
			dist_dir,
			server_target,
			public_static_base,
			frontend_config,
			ts_gen_config,
			dev_watch_config,
			state,
			views,
			resources,
			middlewares,
			tasks_options,
			document,
			request_body_limit,
		} = app_config;
		let extra_type_defs = ts_gen_config
			.extra_types
			.iter()
			.flat_map(|extra_type| extra_type.__type_defs().iter().cloned())
			.collect::<Vec<_>>();
		let config = Config {
			root_dir,
			dist_dir,
			server_target,
			public_static_base,
			frontend_config,
			ts_gen_config,
			dev_watch_config,
		};
		let tasks = Tasks::new(tasks_options);
		let mut assembly = AppAssembly::from_config_with_tasks(&config, state, tasks.clone())
			.map_err(|source| Error::new(source.to_string()))?;
		assembly.set_runtime_document_builder(document);
		assembly.set_request_body_limit(request_body_limit);
		for type_def in extra_type_defs {
			assembly
				.facade_mut()
				.add_type_def(type_def)
				.map_err(facade_error)?;
		}
		middlewares.register_into(assembly.facade_mut())?;
		views.register_into(assembly.facade_mut())?;
		resources.register_into(assembly.facade_mut())?;
		Ok(Self {
			config,
			assembly,
			_tasks: tasks,
		})
	}

	/// Build a runtime host from public configuration.
	///
	/// When the process was started in live build-state mode (the build's
	/// env key is set), this emits the serialized build contract to stdout
	/// and exits instead of constructing a host — the app binary doubles as
	/// the build's graph emitter.
	pub fn from_config(app_config: AppConfig<S>) -> crate::Result<crate::RuntimeHost<S>> {
		if crate::live_state_emit::is_live_build_state_mode() {
			crate::live_state_emit::emit_live_build_state_and_exit(app_config);
		}
		crate::RuntimeHost::new(Self::from_app_config(app_config)?)
	}

	/// Consume the app into its declaration assembly.
	pub(crate) fn into_assembly(self) -> AppAssembly<S> {
		self.assembly
	}

	/// Consume the app into build-facing graph and document-builder facts.
	#[doc(hidden)]
	pub(crate) fn into_build_contract(self) -> crate::Result<AppBuildContract> {
		let (graph, document_builder) = self
			.into_assembly()
			.compile()
			.map_err(|source| Error::new(format!("{source:?}")))?
			.into_build_parts();
		Ok(AppBuildContract {
			graph,
			document_builder,
		})
	}

	/// Consume the app into the normalized config and declaration assembly.
	pub(crate) fn into_config_and_assembly(self) -> (Config, AppAssembly<S>) {
		(self.config, self.assembly)
	}
}

/// Request-scoped middleware declaration shared by views and resources.
type MiddlewareRegistrar<S> = Box<
	dyn FnOnce(
			usize,
			&mut StatefulAppFacade<S>,
			Vec<String>,
			Vec<http::Method>,
		) -> Result<(), FacadeError>
		+ Send,
>;

/// Request-scoped middleware declaration shared by views and resources.
/*
Filters AND together; an omitted filter means unrestricted. Scope
patterns use the same grammar as routes (flat matching, no `_index`):
subtree gating is written in the pattern language ("/admin" plus the
"/admin" splat form), and the run decision is a plain path match — scope
patterns never contribute params.
*/
pub struct Middleware<S> {
	register: MiddlewareRegistrar<S>,
	patterns: Vec<String>,
	methods: Vec<http::Method>,
}

impl<S> Middleware<S>
where
	S: Send + Sync + 'static,
{
	/// Create middleware from an async handler; runs for every request.
	pub fn new<F, Fut, O>(handler: F) -> Self
	where
		F: Fn(MiddlewareCtx<S>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, crate::HttpExit>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		Self {
			register: Box::new(move |index, facade, patterns, methods| {
				facade.add_runtime_middleware(
					patterns,
					methods,
					middleware_handler_id(index),
					middleware_runtime_handler(Arc::clone(facade.state()), handler),
				)?;
				Ok(())
			}),
			patterns: Vec::new(),
			methods: Vec::new(),
		}
	}

	/// Run only for requests whose path matches one of these patterns.
	pub fn with_patterns<I>(mut self, patterns: I) -> Self
	where
		I: IntoIterator,
		I::Item: Into<String>,
	{
		self.patterns = patterns.into_iter().map(Into::into).collect();
		self
	}

	/// Run only for requests using one of these methods.
	pub fn with_methods<I>(mut self, methods: I) -> Self
	where
		I: IntoIterator<Item = http::Method>,
	{
		self.methods = methods.into_iter().collect();
		self
	}
}

/// Nested view declaration.
pub struct View<S> {
	pattern: &'static str,
	client_file: &'static str,
	input_type: TypeResolver,
	output_type: TypeResolver,
	search_schema: SearchSchemaResolver,
	handler: ErasedRouteHandler<S>,
}

impl<S> View<S>
where
	S: Send + Sync + 'static,
{
	/// Create a macro-compatible static view declaration.
	#[doc(hidden)]
	pub const fn from_static(
		pattern: &'static str,
		client_file: &'static str,
		input_type: TypeResolver,
		output_type: TypeResolver,
		search_schema: SearchSchemaResolver,
		handler: ErasedRouteHandler<S>,
	) -> Self {
		Self {
			pattern,
			client_file,
			input_type,
			output_type,
			search_schema,
			handler,
		}
	}

	/// Registered view pattern.
	pub fn pattern(&self) -> &str {
		self.pattern
	}

	/// Frontend client module associated with this view.
	pub fn client_file(&self) -> &str {
		self.client_file
	}

	fn register_into(self, facade: &mut StatefulAppFacade<S>) -> Result<(), FacadeError> {
		register_static_view(
			facade,
			self.pattern,
			self.client_file,
			self.input_type,
			self.output_type,
			self.search_schema,
			self.handler,
		)
	}
}

/// Resource declaration.
pub struct Resource<S> {
	method: http::Method,
	pattern: &'static str,
	kind: Option<ResourceKind>,
	input_type: TypeResolver,
	output_type: TypeResolver,
	handler: ErasedRouteHandler<S>,
}

impl<S> Resource<S>
where
	S: Send + Sync + 'static,
{
	/// Create a macro-compatible static resource declaration.
	#[doc(hidden)]
	pub const fn from_static(
		method: http::Method,
		pattern: &'static str,
		kind: Option<ResourceKind>,
		input_type: TypeResolver,
		output_type: TypeResolver,
		handler: ErasedRouteHandler<S>,
	) -> Self {
		Self {
			method,
			pattern,
			kind,
			input_type,
			output_type,
			handler,
		}
	}

	/// HTTP method registered for this resource.
	pub fn method(&self) -> &http::Method {
		&self.method
	}

	/// Declared resource URL pattern.
	pub fn pattern(&self) -> &str {
		self.pattern
	}

	/// Explicit generated-client kind override, when one was declared.
	pub fn kind(&self) -> Option<ResourceKind> {
		self.kind
	}

	fn register_into(self, facade: &mut StatefulAppFacade<S>) -> Result<(), FacadeError> {
		register_static_resource(
			facade,
			self.method,
			self.pattern,
			self.kind,
			self.input_type,
			self.output_type,
			self.handler,
		)
	}
}

/// Collection of middleware declarations.
#[derive(Default)]
pub struct Middlewares<S> {
	middlewares: Vec<Middleware<S>>,
}

impl<S> Middlewares<S>
where
	S: Send + Sync + 'static,
{
	/// Create an empty middleware collection.
	pub fn new() -> Self {
		Self {
			middlewares: Vec::new(),
		}
	}

	/// Append a middleware declaration.
	pub fn push(&mut self, middleware: Middleware<S>) -> &mut Self {
		self.middlewares.push(middleware);
		self
	}

	fn register_into(self, facade: &mut StatefulAppFacade<S>) -> crate::Result<()> {
		for (index, middleware) in self.middlewares.into_iter().enumerate() {
			let Middleware {
				register,
				patterns,
				methods,
			} = middleware;
			register(index, facade, patterns, methods).map_err(facade_error)?;
		}
		Ok(())
	}
}

/// Collection of nested view declarations.
#[derive(Default)]
pub struct Views<S> {
	views: Vec<View<S>>,
}

impl<S> Views<S>
where
	S: Send + Sync + 'static,
{
	/// Create an empty view collection.
	pub fn new() -> Self {
		Self { views: Vec::new() }
	}

	/// Append a view declaration.
	pub fn push(&mut self, view: View<S>) -> &mut Self {
		self.views.push(view);
		self
	}

	fn register_into(self, facade: &mut StatefulAppFacade<S>) -> crate::Result<()> {
		for view in self.views {
			view.register_into(facade).map_err(facade_error)?;
		}
		Ok(())
	}
}

/// Collection of resource declarations.
#[derive(Default)]
pub struct Resources<S> {
	resources: Vec<Resource<S>>,
}

impl<S> Resources<S>
where
	S: Send + Sync + 'static,
{
	/// Create an empty resource collection.
	pub fn new() -> Self {
		Self {
			resources: Vec::new(),
		}
	}

	/// Append a resource declaration.
	pub fn push(&mut self, resource: Resource<S>) -> &mut Self {
		self.resources.push(resource);
		self
	}

	fn register_into(self, facade: &mut StatefulAppFacade<S>) -> crate::Result<()> {
		for resource in self.resources {
			resource.register_into(facade).map_err(facade_error)?;
		}
		Ok(())
	}
}

fn register_static_view<S>(
	facade: &mut StatefulAppFacade<S>,
	pattern: &'static str,
	client_file: &'static str,
	input_type: TypeResolver,
	output_type: TypeResolver,
	search_schema: SearchSchemaResolver,
	handler: ErasedRouteHandler<S>,
) -> Result<(), FacadeError>
where
	S: Send + Sync + 'static,
{
	let route_contract = route_contract_from_resolvers(input_type, output_type)
		.map_err(|message| FacadeError::RouteContract { message })?;
	let search_schema = search_schema().map_err(|message| FacadeError::RouteContract {
		message: format!("view {pattern} input: {message}"),
	})?;
	register_type_defs(facade, route_contract.type_defs)?;
	facade.add_runtime_view_route(
		pattern,
		client_file,
		search_schema,
		route_contract.type_contract,
		view_handler_id(pattern),
		static_route_runtime_handler(Arc::clone(facade.state()), handler),
	)?;
	Ok(())
}

fn register_static_resource<S>(
	facade: &mut StatefulAppFacade<S>,
	method: http::Method,
	pattern: &'static str,
	kind: Option<ResourceKind>,
	input_type: TypeResolver,
	output_type: TypeResolver,
	handler: ErasedRouteHandler<S>,
) -> Result<(), FacadeError>
where
	S: Send + Sync + 'static,
{
	let route_contract = route_contract_from_resolvers(input_type, output_type)
		.map_err(|message| FacadeError::RouteContract { message })?;
	register_type_defs(facade, route_contract.type_defs)?;
	facade.add_runtime_resource_route(
		ResourceRouteSpec::new(
			method.clone(),
			pattern,
			kind,
			None,
			route_contract.type_contract,
			resource_handler_id(&method, pattern),
		),
		static_route_runtime_handler(Arc::clone(facade.state()), handler),
	)?;
	Ok(())
}

fn register_type_defs<S>(
	facade: &mut StatefulAppFacade<S>,
	type_defs: Vec<crate::tsgen::TypeDef>,
) -> Result<(), FacadeError>
where
	S: Send + Sync + 'static,
{
	for type_def in type_defs {
		facade.add_type_def(type_def)?;
	}
	Ok(())
}

fn view_handler_id(pattern: &str) -> HandlerId {
	HandlerId::new(format!("{VIEW_HANDLER_ID_PREFIX}{pattern}"))
		.expect("generated view handler id should be nonempty")
}

fn resource_handler_id(method: &http::Method, pattern: &str) -> HandlerId {
	HandlerId::new(format!("{RESOURCE_HANDLER_ID_PREFIX}{method}:{pattern}"))
		.expect("generated resource handler id should be nonempty")
}

fn middleware_handler_id(index: usize) -> HandlerId {
	HandlerId::new(format!("{MIDDLEWARE_HANDLER_ID_PREFIX}{index}"))
		.expect("generated middleware handler id should be nonempty")
}

fn facade_error(error: FacadeError) -> Error {
	Error::new(format!("{error:?}"))
}

impl<S> Default for AppConfig<S>
where
	S: Default + Send + Sync + 'static,
{
	fn default() -> Self {
		Self {
			root_dir: PathBuf::new(),
			dist_dir: String::new(),
			server_target: ServerTarget::default(),
			public_static_base: String::new(),
			frontend_config: FrontendConfig::default(),
			ts_gen_config: TsGenConfig::default(),
			dev_watch_config: DevWatchConfig::default(),
			state: S::default(),
			views: Views::new(),
			resources: Resources::new(),
			middlewares: Middlewares::new(),
			tasks_options: vorma_tasks::TasksOptions::default(),
			document: DocumentBuilder::default(),
			request_body_limit: DEFAULT_REQUEST_BODY_LIMIT,
		}
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use bytes::Bytes;
	use http::Method;
	use http_body_util::{BodyExt, Full};
	use serde::{Deserialize, Serialize};
	use tower_service::Service;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyError;
	use crate::runtime_manifest::RuntimeManifest;
	use crate::tsgen::{FieldDef, Result as TsResult, Type, TypeDef, TypeRef, TypeRegistry};

	const TEST_CLIENT_BUILD_ID: &str = "build-id";
	const TEST_INPUT_TYPE_NAME: &str = "AppInput";
	const TEST_OUTPUT_TYPE_NAME: &str = "AppOutput";
	const TEST_RESOURCE_PATTERN: &str = "/api/hello";
	const TEST_RESOURCE_URI: &str = "/api/hello";
	const TEST_STATIC_PATTERN: &str = "/static/:id";
	const TEST_STATIC_RESOURCE_PATTERN: &str = "/api/static/:id";
	const TEST_STATIC_RESOURCE_URI: &str = "/api/static/7";
	const TEST_STATIC_FORM_PATTERN: &str = "/api/static-form/:id";
	const TEST_STATIC_FORM_RESOURCE_URI: &str = "/api/static-form/7";
	const TEST_STATIC_ERROR_PATTERN: &str = "/api/static-error/:id";
	const TEST_STATIC_ERROR_RESOURCE_URI: &str = "/api/static-error/7";
	const TEST_STATIC_ERROR_BODY: &str = "static resource failed";
	const TEST_STATIC_VIEW_CLIENT_FILE: &str = "static.tsx";
	const TEST_STATIC_VIEW_MODULE: &str = "/static/static.js";

	#[derive(Clone, Debug, Default)]
	struct TestState {
		prefix: String,
	}

	#[derive(Clone, Debug, Deserialize)]
	struct AppInput {
		name: String,
	}

	impl Type for AppInput {
		fn type_ref() -> TypeRef {
			TypeRef::named(TEST_INPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record(
				TEST_INPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("name")],
			));
			Ok(())
		}
	}

	#[derive(Clone, Debug, Serialize)]
	struct AppOutput {
		message: String,
	}

	impl Type for AppOutput {
		fn type_ref() -> TypeRef {
			TypeRef::named(TEST_OUTPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record(
				TEST_OUTPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("message")],
			));
			Ok(())
		}
	}

	#[derive(Clone, Debug, Eq, PartialEq)]
	struct StaticParams {
		id: String,
	}

	impl crate::static_route::PathParams for StaticParams {
		fn from_raw_path_params(
			params: &crate::__private::Params,
		) -> std::result::Result<Self, crate::static_route::InputError> {
			Ok(Self {
				id: params
					.get("id")
					.map(|value| value.to_string())
					.ok_or_else(|| crate::static_route::InputError::bad_request("missing id"))?,
			})
		}
	}

	fn static_view_handler(
		ctx: crate::static_route::ErasedRequestCtx<TestState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_view::<TestState, AppInput, StaticParams, AppOutput>(
			ctx,
			static_view_inner,
		)
	}

	fn static_view_inner(
		ctx: crate::ViewCtx<TestState, AppInput, StaticParams>,
	) -> crate::static_route::RouteFuture<AppOutput, crate::ViewExit> {
		Box::pin(async move {
			ctx.head().title("Static View");
			Ok(AppOutput {
				message: format!(
					"{}{}:{}",
					ctx.state().prefix,
					ctx.params().id,
					ctx.input().name
				),
			})
		})
	}

	fn static_resource_handler(
		ctx: crate::static_route::ErasedRequestCtx<TestState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_resource::<TestState, AppInput, StaticParams, AppOutput>(
			ctx,
			static_resource_inner,
		)
	}

	fn static_resource_inner(
		ctx: crate::ResourceCtx<TestState, AppInput, StaticParams>,
	) -> crate::static_route::RouteFuture<AppOutput, crate::HttpExit> {
		Box::pin(async move {
			ctx.response().set_status(http::StatusCode::CREATED);
			Ok(AppOutput {
				message: format!(
					"{}{}:{}:{}",
					ctx.state().prefix,
					ctx.params().id,
					ctx.param("id"),
					ctx.input().name
				),
			})
		})
	}

	fn static_form_resource_handler(
		ctx: crate::static_route::ErasedRequestCtx<TestState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_resource::<
			TestState,
			crate::FormData,
			StaticParams,
			AppOutput,
		>(ctx, static_form_resource_inner)
	}

	fn static_form_resource_inner(
		ctx: crate::ResourceCtx<TestState, crate::FormData, StaticParams>,
	) -> crate::static_route::RouteFuture<AppOutput, crate::HttpExit> {
		Box::pin(async move {
			ctx.response().set_status(http::StatusCode::CREATED);
			Ok(AppOutput {
				message: format!(
					"{}{}:{}:{}",
					ctx.state().prefix,
					ctx.params().id,
					ctx.param("id"),
					ctx.input().text("name").unwrap_or("")
				),
			})
		})
	}

	fn static_error_resource_handler(
		ctx: crate::static_route::ErasedRequestCtx<TestState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_resource::<TestState, AppInput, StaticParams, AppOutput>(
			ctx,
			static_error_resource_inner,
		)
	}

	fn static_error_resource_inner(
		_ctx: crate::ResourceCtx<TestState, AppInput, StaticParams>,
	) -> crate::static_route::RouteFuture<AppOutput, crate::HttpExit> {
		Box::pin(async move {
			Err(
				crate::HttpExit::err("static resource rejected for the error pin")
					.with_status(http::StatusCode::BAD_REQUEST)
					.with_client_msg(TEST_STATIC_ERROR_BODY),
			)
		})
	}

	const STATIC_VIEW: View<TestState> = View::from_static(
		TEST_STATIC_PATTERN,
		TEST_STATIC_VIEW_CLIENT_FILE,
		crate::route_input::type_resolver::<AppInput>,
		crate::route_input::type_resolver::<AppOutput>,
		crate::route_input::search_schema_resolver::<AppInput>,
		static_view_handler,
	);

	const STATIC_RESOURCE: Resource<TestState> = Resource::from_static(
		Method::POST,
		TEST_STATIC_RESOURCE_PATTERN,
		Some(ResourceKind::Mutation),
		crate::route_input::type_resolver::<AppInput>,
		crate::route_input::type_resolver::<AppOutput>,
		static_resource_handler,
	);

	const STATIC_FORM_RESOURCE: Resource<TestState> = Resource::from_static(
		Method::POST,
		TEST_STATIC_FORM_PATTERN,
		Some(ResourceKind::Mutation),
		crate::route_input::type_resolver::<crate::FormData>,
		crate::route_input::type_resolver::<AppOutput>,
		static_form_resource_handler,
	);

	const STATIC_ERROR_RESOURCE: Resource<TestState> = Resource::from_static(
		Method::POST,
		TEST_STATIC_ERROR_PATTERN,
		Some(ResourceKind::Mutation),
		crate::route_input::type_resolver::<AppInput>,
		crate::route_input::type_resolver::<AppOutput>,
		static_error_resource_handler,
	);

	const APP_RESOURCE: Resource<TestState> = Resource::from_static(
		Method::POST,
		TEST_RESOURCE_PATTERN,
		None,
		crate::route_input::type_resolver::<AppInput>,
		crate::route_input::type_resolver::<AppOutput>,
		static_app_resource_handler,
	);

	fn static_app_resource_handler(
		ctx: crate::static_route::ErasedRequestCtx<TestState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_resource::<TestState, AppInput, (), AppOutput>(
			ctx,
			static_app_resource_inner,
		)
	}

	fn static_app_resource_inner(
		ctx: crate::ResourceCtx<TestState, AppInput>,
	) -> crate::static_route::RouteFuture<AppOutput, crate::HttpExit> {
		Box::pin(async move {
			Ok(AppOutput {
				message: format!("{}{}", ctx.state().prefix, ctx.input().name),
			})
		})
	}

	fn app_config() -> AppConfig<TestState> {
		let mut resources: Resources<TestState> = Resources::new();
		resources.push(APP_RESOURCE);
		AppConfig {
			root_dir: "app".into(),
			server_target: ServerTarget {
				cargo_package: "server-package".to_owned(),
				cargo_bin: "server-bin".to_owned(),
			},
			dist_dir: "dist".to_owned(),
			public_static_base: String::new(),
			frontend_config: FrontendConfig {
				ui_variant: crate::UiVariant::React,
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "src/critical.css".to_owned(),
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				extra_types: Vec::new(),
				extra_ts: crate::tsgen::TsDrafter::new(),
			},
			dev_watch_config: DevWatchConfig::default(),
			state: TestState {
				prefix: "hello ".to_owned(),
			},
			views: Views::new(),
			resources,
			middlewares: Middlewares::new(),
			tasks_options: vorma_tasks::TasksOptions::default(),
			document: DocumentBuilder::default(),
			request_body_limit: 1024,
		}
	}

	#[tokio::test]
	async fn app_config_lowers_public_resources_into_runtime_assembly() {
		let app = App::from_app_config(app_config()).unwrap();
		let mut service = app
			.into_assembly()
			.compile()
			.unwrap()
			.into_runtime_host(
				RuntimeManifest::new(
					TEST_CLIENT_BUILD_ID,
					"/",
					"",
					BTreeMap::new(),
					Vec::new(),
					BTreeMap::new(),
					Vec::new(),
				),
				|_| async { Err(PublicAssetBodyError::new("asset provider should not run")) },
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
	async fn static_public_declarations_lower_into_runtime_assembly() {
		let mut views = Views::new();
		views.push(STATIC_VIEW);
		let mut resources = Resources::new();
		resources.push(STATIC_RESOURCE);
		resources.push(STATIC_FORM_RESOURCE);
		resources.push(STATIC_ERROR_RESOURCE);
		let config = AppConfig {
			public_static_base: "/static/".to_owned(),
			views,
			resources,
			..app_config()
		};
		let mut service = App::from_app_config(config)
			.unwrap()
			.into_assembly()
			.compile()
			.unwrap()
			.into_runtime_host(
				RuntimeManifest::new(
					TEST_CLIENT_BUILD_ID,
					"/static/",
					"",
					BTreeMap::from([(
						TEST_STATIC_PATTERN.to_owned(),
						serde_json::json!({"name": "s"}),
					)]),
					vec![TEST_STATIC_VIEW_MODULE.to_owned()],
					BTreeMap::new(),
					vec![crate::runtime_manifest::RuntimeViewModule::new(
						TEST_STATIC_PATTERN,
						TEST_STATIC_VIEW_MODULE,
						Vec::new(),
						Vec::new(),
					)],
				),
				|_| async { Err(PublicAssetBodyError::new("asset provider should not run")) },
			)
			.unwrap()
			.into_service();
		let request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_STATIC_RESOURCE_URI)
			.body(Full::new(Bytes::from_static(br#"{"name":"Ada"}"#)))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let status = response.status();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let body: serde_json::Value = serde_json::from_slice(&body).unwrap();

		assert_eq!(status, http::StatusCode::CREATED);
		assert_eq!(body["message"], "hello 7:7:Ada");

		let form_request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_STATIC_FORM_RESOURCE_URI)
			.header(
				http::header::CONTENT_TYPE,
				http::HeaderValue::from_static("application/x-www-form-urlencoded"),
			)
			.body(Full::new(Bytes::from_static(b"name=Ada")))
			.unwrap();

		let form_response = service.call(form_request).await.unwrap();
		let form_status = form_response.status();
		let form_body = form_response
			.into_body()
			.collect()
			.await
			.unwrap()
			.to_bytes();
		let form_body: serde_json::Value = serde_json::from_slice(&form_body).unwrap();

		assert_eq!(form_status, http::StatusCode::CREATED);
		assert_eq!(form_body["message"], "hello 7:7:Ada");

		let error_request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_STATIC_ERROR_RESOURCE_URI)
			.body(Full::new(Bytes::from_static(br#"{"name":"Ada"}"#)))
			.unwrap();

		let error_response = service.call(error_request).await.unwrap();
		let error_status = error_response.status();
		let error_body = error_response
			.into_body()
			.collect()
			.await
			.unwrap()
			.to_bytes();

		assert_eq!(error_status, http::StatusCode::BAD_REQUEST);
		assert_eq!(
			error_body,
			Bytes::from(format!(r#"{{"error":"{TEST_STATIC_ERROR_BODY}"}}"#))
		);
	}
}
