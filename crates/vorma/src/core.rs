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
pub use context::{HeadHandle, MiddlewareCtx, Params, ResourceCtx, ResponseHandle, ViewCtx};
use contract::RouteCollector;
pub use contract::{Contract, ResourceEntry, ViewEntry, contract_for};
#[cfg(test)]
use pattern::default_resource_kind_for_method;
use pattern::validate_declared_route_pattern;
pub use pattern::{
	default_resource_kind, params_for_pattern, pattern_is_splat, view_parents_for_patterns,
};
mod pattern;
pub(crate) use runtime::{RuntimeRoutes, runtime_routes_for};
mod runtime;
#[cfg(test)]
use runner::serialize_route_output;
pub use runner::{
	ErasedRequestCtx, ErasedRouteFuture, ErasedRouteHandler, PathParams, RouteFuture,
	run_static_resource, run_static_view,
};
use runner::{RouteRunner, run_route_runner};
mod runner;

#[doc(hidden)]
pub type TypeResolver = fn(TypePhase, &mut TypeRegistry) -> Result<TypeRef, String>;
#[doc(hidden)]
pub type SearchSchemaResolver = fn() -> Result<Value, String>;
#[cfg(test)]
type ResourceHandler<S, E, I, P, O> =
	dyn Fn(ResourceCtx<S, E, I, P>) -> RouteFuture<O, E> + Send + Sync;
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

/// Request-scoped middleware declaration shared by views and resources.
pub struct Middleware<S, E = Box<dyn std::error::Error + Send + Sync>> {
	mw: mux::Middleware<S, E>,
}

impl<S, E> Middleware<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create middleware from an async handler.
	pub fn new<F, Fut, O>(handler: F) -> Self
	where
		F: Fn(MiddlewareCtx<S, E>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = vorma_tasks::Result<O, E>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		Self {
			mw: mux::Middleware::new(move |ctx| {
				let future = handler(MiddlewareCtx::new(ctx));
				async move { future.await.map(|_| ()) }
			}),
		}
	}

	fn register_resource(&self, r: &mut mux::Router<S, E>) -> Result<(), mux::Error> {
		r.use_middleware_entry(&self.mw);
		Ok(())
	}

	fn register_view(&self, r: &mut NestedRouter<S, E>) -> Result<(), mux::Error> {
		r.use_middleware_entry(&self.mw);
		Ok(())
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		self.register_resource(&mut routes.resources)?;
		self.register_view(&mut routes.views)?;
		Ok(())
	}
}

/// Generated-client classification for a resource.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ResourceKind {
	/// Read-shaped resource. GET/HEAD resources default to this when no explicit kind is set.
	Query,
	/// Write-shaped resource. Non-GET/HEAD resources default to this when no explicit kind is set.
	Mutation,
}

impl ResourceKind {
	/// Return the generated TypeScript literal for this resource kind.
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
	handler: RouteRunner<S, E>,
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
		handler: ErasedRouteHandler<S, E>,
	) -> Self {
		Self {
			pattern: Cow::Borrowed(pattern),
			handler: RouteRunner::Static(handler),
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
		handler: F,
	) -> Self
	where
		F: Fn(ViewCtx<S, E, I, P>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		I: Type + api::ViewInput,
		P: PathParams,
		O: Type + Serialize + Send + Sync + 'static,
	{
		let handler: Arc<ViewHandler<S, E, I, P, O>> = Arc::new(move |ctx| Box::pin(handler(ctx)));
		Self {
			pattern: Cow::Owned(pattern.into()),
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
					let output = handler(ViewCtx::new(ctx.with_input(input), params))
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

/// Collection of middleware declarations.
#[derive(Default)]
pub struct Middlewares<S, E = Box<dyn std::error::Error + Send + Sync>> {
	middlewares: Vec<Middleware<S, E>>,
}

impl<S, E> Middlewares<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create an empty middleware collection.
	pub fn new() -> Self {
		Self {
			middlewares: Vec::new(),
		}
	}

	/// Append a middleware declaration.
	pub fn push(&mut self, middleware: Middleware<S, E>) -> &mut Self {
		self.middlewares.push(middleware);
		self
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		for middleware in &self.middlewares {
			middleware.register_runtime(routes)?;
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
		let handler = self.handler.clone();
		r.add_handler_entry(
			self.pattern.to_string(),
			mux::erased_handler(move |ctx| {
				let handler = handler.clone();
				async move { run_route_runner(handler, ctx).await }
			}),
		)
	}
}

/// Resource declaration.
pub struct Resource<S, E = Box<dyn std::error::Error + Send + Sync>> {
	method: Method,
	pattern: Cow<'static, str>,
	kind: Option<ResourceKind>,
	handler: RouteRunner<S, E>,
	input_type: TypeResolver,
	output_type: TypeResolver,
}

impl<S, E> Resource<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	#[doc(hidden)]
	pub const fn from_static(
		method: Method,
		pattern: &'static str,
		kind: Option<ResourceKind>,
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
		kind: Option<ResourceKind>,
		input_parser: InputParser<I>,
		handler: F,
	) -> Self
	where
		F: Fn(ResourceCtx<S, E, I, P>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		I: Type + api::ResourceInput,
		P: PathParams,
		O: Type + Serialize + Send + Sync + 'static,
	{
		let handler: Arc<ResourceHandler<S, E, I, P, O>> =
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
					let output = handler(ResourceCtx::new(ctx.with_input(input), params))
						.await
						.map_err(RouteExecutionError::Task)?;
					serialize_route_output(output)
				})
			})),
			input_type: type_resolver::<I>,
			output_type: type_resolver::<O>,
		}
	}

	/// HTTP method registered for this resource.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// Resource pattern relative to the configured API mount root.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Explicit generated-client kind override, when one was declared.
	pub fn kind(&self) -> Option<ResourceKind> {
		self.kind
	}
}

impl<S, E> Resource<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	#[cfg(test)]
	pub(crate) fn without_handler(
		method: Method,
		pattern: impl Into<String>,
		kind: Option<ResourceKind>,
	) -> Self {
		Self::new(
			method,
			pattern,
			kind,
			InputParser::default_input(),
			|_: ResourceCtx<S, E, (), ()>| async { Ok(None) },
		)
	}
}

impl<S, E> Resource<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		let input = (self.input_type)(TypePhase::Deserialize, &mut collector.types)?;
		let output = (self.output_type)(TypePhase::Serialize, &mut collector.types)?;
		collector.resources.push(ResourceEntry {
			method: self.method.as_str().to_owned(),
			pattern: self.pattern.to_string(),
			kind: self.kind,
			input,
			output,
		});
		Ok(())
	}

	fn register_to_mux(&self, r: &mut mux::Router<S, E>) -> Result<(), mux::Error> {
		validate_declared_route_pattern("resource", &self.pattern)
			.map_err(mux::Error::InvalidPattern)?;
		let handler = self.handler.clone();
		r.add_handler_entry(
			self.method.clone(),
			self.pattern.to_string(),
			mux::erased_handler(move |ctx| {
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

/// Collection of resource declarations.
#[derive(Default)]
pub struct Resources<S, E = Box<dyn std::error::Error + Send + Sync>> {
	resources: Vec<Resource<S, E>>,
}

impl<S, E> Resources<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	/// Create an empty resource collection.
	pub fn new() -> Self {
		Self {
			resources: Vec::new(),
		}
	}

	/// Append a resource declaration.
	pub fn push(&mut self, resource: Resource<S, E>) -> &mut Self {
		self.resources.push(resource);
		self
	}

	fn register_contract(&self, collector: &mut RouteCollector) -> Result<(), String> {
		for resource in &self.resources {
			resource.register_contract(collector)?;
		}
		Ok(())
	}

	fn register_runtime(&self, routes: &mut RuntimeRoutes<S, E>) -> Result<(), mux::Error> {
		for resource in &self.resources {
			resource.register_to_mux(&mut routes.resources)?;
		}
		Ok(())
	}
}

#[cfg(test)]
#[path = "core_tests.rs"]
mod core_tests;
