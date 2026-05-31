use std::borrow::Cow;
use std::future::Future;
#[cfg(test)]
use std::sync::Arc;

use http::Method;
#[cfg(test)]
use serde::Serialize;
use serde_json::Value;
#[cfg(test)]
use vorma_tasks::Result as TaskResult;

#[cfg(test)]
use crate::api;
#[cfg(test)]
use crate::mux::RouteExecutionError;
use crate::mux::{self, NestedRouter};
#[cfg(test)]
use crate::mux::{InputParser, None};
use crate::searchparams;
use crate::tsgen::{Type, TypePhase, TypeRef, TypeRegistry};

mod context;
mod contract;
#[cfg(test)]
use context::accepts_client_redirect;
pub use context::{ApiCtx, HeadHandle, ResponseHandle, RouteParams, TaskMiddlewareCtx, ViewCtx};
use contract::RouteCollector;
pub use contract::{ApiRouteEntry, Contract, ViewEntry, contract_for};
#[cfg(test)]
use pattern::default_api_route_kind_for_method;
use pattern::validate_declared_route_pattern;
pub use pattern::{
	default_api_route_kind, route_is_splat_pattern, route_params_for_pattern,
	view_parents_for_patterns,
};
mod pattern;
pub(crate) use runtime::{RuntimeRoutes, runtime_routes_for};
mod runtime;
#[cfg(test)]
use runner::serialize_route_output;
pub use runner::{
	ErasedRequestCtx, ErasedRouteFuture, ErasedRouteHandler, PathParams, RouteFuture,
	run_static_api_route, run_static_view,
};
use runner::{RouteRunner, run_route_runner};
mod runner;

#[doc(hidden)]
pub type TypeResolver = fn(TypePhase, &mut TypeRegistry) -> Result<TypeRef, String>;
#[doc(hidden)]
pub type SearchSchemaResolver = fn() -> Result<Value, String>;
#[cfg(test)]
type ApiRouteHandler<S, E, I, P, O> = dyn Fn(ApiCtx<S, E, I, P>) -> RouteFuture<O, E> + Send + Sync;
#[cfg(test)]
type ViewHandler<S, E, I, P, O> = dyn Fn(ViewCtx<S, E, I, P>) -> RouteFuture<O, E> + Send + Sync;

#[doc(hidden)]
pub fn type_resolver<T>(phase: TypePhase, registry: &mut TypeRegistry) -> Result<TypeRef, String>
where
	T: Type,
{
	T::collect_type_defs_for(phase, registry).map_err(|err| err.to_string())?;
	Ok(T::type_ref_for(phase))
}

#[doc(hidden)]
pub fn search_schema_resolver<T>() -> Result<Value, String>
where
	T: Type,
{
	searchparams::schema_for_type::<T>().map_err(|err| err.to_string())
}

/// Request-scoped task middleware declaration shared by views and API-routes.
pub struct TaskMiddleware<S, E = Box<dyn std::error::Error + Send + Sync>> {
	mw: mux::TaskMw<S, E>,
}

impl<S, E> TaskMiddleware<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create task middleware from an async handler.
	pub fn new<F, Fut, O>(handler: F) -> Self
	where
		F: Fn(TaskMiddlewareCtx<S, E>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = vorma_tasks::Result<O, E>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		Self {
			mw: mux::TaskMw::new(move |ctx| {
				let future = handler(TaskMiddlewareCtx::new(ctx));
				async move { future.await.map(|_| ()) }
			}),
		}
	}

	fn register_api(&self, r: &mut mux::Router<S, E>) -> Result<(), mux::Error> {
		r.use_task_middleware_entry(&self.mw);
		Ok(())
	}

	fn register_view(&self, r: &mut NestedRouter<S, E>) -> Result<(), mux::Error> {
		r.use_task_middleware_entry(&self.mw);
		Ok(())
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		self.register_api(&mut routes.api)?;
		self.register_view(&mut routes.views)?;
		Ok(())
	}
}

/// Generated-client classification for an API-route.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ApiRouteKind {
	/// Read-shaped route. GET/HEAD routes default to this when no explicit kind is set.
	Query,
	/// Write-shaped route. Non-GET/HEAD routes default to this when no explicit kind is set.
	Mutation,
}

impl ApiRouteKind {
	/// Return the generated TypeScript literal for this API-route kind.
	pub fn as_str(self) -> &'static str {
		match self {
			Self::Query => "query",
			Self::Mutation => "mutation",
		}
	}
}

/// Nested view declaration.
pub struct View<S, E = Box<dyn std::error::Error + Send + Sync>> {
	pattern: Cow<'static, str>,
	loader: RouteRunner<S, E>,
	client_file: Cow<'static, str>,
	input_type: TypeResolver,
	output_type: TypeResolver,
	search_schema: SearchSchemaResolver,
}

impl<S, E> View<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	#[doc(hidden)]
	pub const fn from_static(
		pattern: &'static str,
		client_file: &'static str,
		input_type: TypeResolver,
		output_type: TypeResolver,
		search_schema: SearchSchemaResolver,
		loader: ErasedRouteHandler<S, E>,
	) -> Self {
		Self {
			pattern: Cow::Borrowed(pattern),
			loader: RouteRunner::Static(loader),
			client_file: Cow::Borrowed(client_file),
			input_type,
			output_type,
			search_schema,
		}
	}

	#[cfg(test)]
	pub(crate) fn new<I, P, O, F, Fut>(
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		input_parser: InputParser<I>,
		loader: F,
	) -> Self
	where
		F: Fn(ViewCtx<S, E, I, P>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		I: Type + api::ViewInput,
		P: PathParams,
		O: Type + Serialize + Send + Sync + 'static,
	{
		let loader: Arc<ViewHandler<S, E, I, P, O>> = Arc::new(move |ctx| Box::pin(loader(ctx)));
		Self {
			pattern: Cow::Owned(pattern.into()),
			loader: RouteRunner::Dynamic(Arc::new(move |ctx| {
				let input_parser = input_parser.clone();
				let loader = loader.clone();
				Box::pin(async move {
					let input = input_parser
						.parse(ctx.request())
						.await
						.map_err(RouteExecutionError::Input)?;
					let params = P::from_raw_path_params(ctx.params())
						.map_err(RouteExecutionError::Input)?;
					let output = loader(ViewCtx::new(ctx.with_input(input), params))
						.await
						.map_err(RouteExecutionError::Task)?;
					serialize_route_output(output)
				})
			})),
			client_file: Cow::Owned(client_file.into()),
			input_type: type_resolver::<I>,
			output_type: type_resolver::<O>,
			search_schema: search_schema_resolver::<I>,
		}
	}

	/// Registered view pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Frontend client module associated with this view.
	pub fn client_file(&self) -> &str {
		&self.client_file
	}
}

/// Collection of task middleware declarations.
#[derive(Default)]
pub struct TaskMiddlewares<S, E = Box<dyn std::error::Error + Send + Sync>> {
	task_mws: Vec<TaskMiddleware<S, E>>,
}

impl<S, E> TaskMiddlewares<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create an empty task middleware collection.
	pub fn new() -> Self {
		Self {
			task_mws: Vec::new(),
		}
	}

	/// Append a task middleware declaration.
	pub fn push(&mut self, task_middleware: TaskMiddleware<S, E>) -> &mut Self {
		self.task_mws.push(task_middleware);
		self
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		for task_middleware in &self.task_mws {
			task_middleware.register_runtime(routes)?;
		}
		Ok(())
	}
}

impl<S, E> View<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		let input = (self.input_type)(TypePhase::Deserialize, &mut collector.types)?;
		let output = (self.output_type)(TypePhase::Serialize, &mut collector.types)?;
		collector.views.push(ViewEntry {
			pattern: self.pattern.to_string(),
			client_file: self.client_file.to_string(),
			input,
			output,
			search_schema: (self.search_schema)()
				.map_err(|err| format!("view {} input: {err}", self.pattern))?,
		});
		Ok(())
	}

	fn register_to_mux(&self, r: &mut NestedRouter<S, E>) -> Result<(), mux::Error> {
		validate_declared_route_pattern("view", &self.pattern)
			.map_err(mux::Error::InvalidPattern)?;
		let loader = self.loader.clone();
		r.add_task_handler_entry(
			self.pattern.to_string(),
			mux::erased_task_handler(move |ctx| {
				let loader = loader.clone();
				async move { run_route_runner(loader, ctx).await }
			}),
		)
	}
}

/// API-route declaration.
pub struct ApiRoute<S, E = Box<dyn std::error::Error + Send + Sync>> {
	method: Method,
	pattern: Cow<'static, str>,
	kind: Option<ApiRouteKind>,
	handler: RouteRunner<S, E>,
	input_type: TypeResolver,
	output_type: TypeResolver,
}

impl<S, E> ApiRoute<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	#[doc(hidden)]
	pub const fn from_static(
		method: Method,
		pattern: &'static str,
		kind: Option<ApiRouteKind>,
		input_type: TypeResolver,
		output_type: TypeResolver,
		handler: ErasedRouteHandler<S, E>,
	) -> Self {
		Self {
			method,
			pattern: Cow::Borrowed(pattern),
			kind,
			handler: RouteRunner::Static(handler),
			input_type,
			output_type,
		}
	}

	#[cfg(test)]
	pub(crate) fn new<I, P, O, F, Fut>(
		method: Method,
		pattern: impl Into<String>,
		kind: Option<ApiRouteKind>,
		input_parser: InputParser<I>,
		handler: F,
	) -> Self
	where
		F: Fn(ApiCtx<S, E, I, P>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		I: Type + api::ApiInput,
		P: PathParams,
		O: Type + Serialize + Send + Sync + 'static,
	{
		let handler: Arc<ApiRouteHandler<S, E, I, P, O>> =
			Arc::new(move |ctx| Box::pin(handler(ctx)));
		Self {
			method,
			pattern: Cow::Owned(pattern.into()),
			kind,
			handler: RouteRunner::Dynamic(Arc::new(move |ctx| {
				let input_parser = input_parser.clone();
				let handler = handler.clone();
				Box::pin(async move {
					let input = input_parser
						.parse(ctx.request())
						.await
						.map_err(RouteExecutionError::Input)?;
					let params = P::from_raw_path_params(ctx.params())
						.map_err(RouteExecutionError::Input)?;
					let output = handler(ApiCtx::new(ctx.with_input(input), params))
						.await
						.map_err(RouteExecutionError::Task)?;
					serialize_route_output(output)
				})
			})),
			input_type: type_resolver::<I>,
			output_type: type_resolver::<O>,
		}
	}

	/// HTTP method registered for this API-route.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// API-route pattern relative to the configured API mount root.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Explicit generated-client kind override, when one was declared.
	pub fn kind(&self) -> Option<ApiRouteKind> {
		self.kind
	}
}

impl<S, E> ApiRoute<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	#[cfg(test)]
	pub(crate) fn without_handler(
		method: Method,
		pattern: impl Into<String>,
		kind: Option<ApiRouteKind>,
	) -> Self {
		Self::new(
			method,
			pattern,
			kind,
			InputParser::default_input(),
			|_: ApiCtx<S, E, (), ()>| async { Ok(None) },
		)
	}
}

impl<S, E> ApiRoute<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		let input = (self.input_type)(TypePhase::Deserialize, &mut collector.types)?;
		let output = (self.output_type)(TypePhase::Serialize, &mut collector.types)?;
		collector.api_routes.push(ApiRouteEntry {
			method: self.method.as_str().to_owned(),
			pattern: self.pattern.to_string(),
			kind: self.kind,
			input,
			output,
		});
		Ok(())
	}

	fn register_to_mux(&self, r: &mut mux::Router<S, E>) -> Result<(), mux::Error> {
		validate_declared_route_pattern("API-route", &self.pattern)
			.map_err(mux::Error::InvalidPattern)?;
		let handler = self.handler.clone();
		r.add_task_handler_entry(
			self.method.clone(),
			self.pattern.to_string(),
			mux::erased_task_handler(move |ctx| {
				let handler = handler.clone();
				async move { run_route_runner(handler, ctx).await }
			}),
		)?;
		Ok(())
	}
}

/// Collection of nested view declarations.
#[derive(Default)]
pub struct Views<S, E = Box<dyn std::error::Error + Send + Sync>> {
	views: Vec<View<S, E>>,
}

impl<S, E> Views<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create an empty view collection.
	pub fn new() -> Self {
		Self { views: Vec::new() }
	}

	/// Append a view declaration.
	pub fn push(&mut self, view: View<S, E>) -> &mut Self {
		self.views.push(view);
		self
	}

	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		for view in &self.views {
			view.register_contract(collector)?;
		}
		Ok(())
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		for view in &self.views {
			view.register_to_mux(&mut routes.views)?;
		}
		Ok(())
	}
}

/// Collection of API-route declarations.
#[derive(Default)]
pub struct ApiRoutes<S, E = Box<dyn std::error::Error + Send + Sync>> {
	api_routes: Vec<ApiRoute<S, E>>,
}

impl<S, E> ApiRoutes<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create an empty API-route collection.
	pub fn new() -> Self {
		Self {
			api_routes: Vec::new(),
		}
	}

	/// Append an API-route declaration.
	pub fn push(&mut self, api_route: ApiRoute<S, E>) -> &mut Self {
		self.api_routes.push(api_route);
		self
	}

	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		for api_route in &self.api_routes {
			api_route.register_contract(collector)?;
		}
		Ok(())
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		for api_route in &self.api_routes {
			api_route.register_to_mux(&mut routes.api)?;
		}
		Ok(())
	}
}

#[cfg(test)]
#[path = "core_tests.rs"]
mod core_tests;
