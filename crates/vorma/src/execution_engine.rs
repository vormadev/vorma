//! Request classification and handler execution over immutable snapshots.

use std::cmp::Ordering;
use std::collections::{BTreeMap, HashMap};
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use bytes::Bytes;
use futures_util::stream::{FuturesUnordered, StreamExt};
use http::{Extensions, HeaderMap, Method, Uri};
use serde_json::Value;
use url::form_urlencoded;
use vorma_matcher::{Params, compare_specificity};
use vorma_tasks::{CancelToken, ExecCtx, Tasks, TasksOptions};

use crate::asset_capabilities::{AssetCapabilities, PublicAsset, PublicUrlError};
use crate::error::Error as AppError;
use crate::execution_plan::{ExecutionPlan, ResourceMatch, ViewMatches};
use crate::framework_graph::HandlerId;
use crate::input_decoder::{DecodedRouteInput, decode_resource_input, decode_view_input};
use crate::response_finalizer::{INTERNAL_SERVER_ERROR_STATUS_TEXT, ResponseEffects};

const DEFAULT_SERVER_ERROR_MESSAGE: &str = "An unexpected error occurred.";

/// Boxed future returned by a runtime handler.
pub type HandlerFuture =
	Pin<Box<dyn Future<Output = Result<HandlerOutput, HandlerExecutionError>> + Send + 'static>>;

/// Runtime handler callable stored in a committed snapshot.
pub trait RuntimeHandler: Send + Sync {
	/// Execute the handler for one request invocation.
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture;
}

impl<F, Fut> RuntimeHandler for F
where
	F: Fn(HandlerInput) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<HandlerOutput, HandlerExecutionError>> + Send + 'static,
{
	fn call(&self, input: HandlerInput, _exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		Box::pin(self(input))
	}
}

/// Runtime handler registry for one committed generation.
pub struct HandlerRegistry {
	tasks: Tasks<crate::Error>,
	handlers: HashMap<HandlerId, Arc<dyn RuntimeHandler>>,
}

impl Clone for HandlerRegistry {
	fn clone(&self) -> Self {
		Self {
			tasks: self.tasks.clone(),
			handlers: self.handlers.clone(),
		}
	}
}

impl HandlerRegistry {
	/// Create a handler registry backed by one task runtime.
	pub fn new(tasks: Tasks<crate::Error>) -> Self {
		Self {
			tasks,
			handlers: HashMap::new(),
		}
	}

	/// Register a runtime handler.
	pub fn insert<H>(&mut self, id: HandlerId, handler: H)
	where
		H: RuntimeHandler + 'static,
	{
		self.handlers.insert(id, Arc::new(handler));
	}

	fn handler(&self, id: &HandlerId) -> Result<Arc<dyn RuntimeHandler>, ExecutionError> {
		self.handlers
			.get(id)
			.cloned()
			.ok_or_else(|| ExecutionError::MissingHandler {
				handler_id: id.clone(),
			})
	}

	fn request_exec_ctx(&self) -> RequestExecCtx {
		let cancel = CancelToken::new();
		RequestExecCtx {
			exec_ctx: self.tasks.exec_ctx(cancel.clone()),
			_cancel_on_drop: CancelOnDrop(cancel),
		}
	}
}

impl Default for HandlerRegistry {
	fn default() -> Self {
		Self::new(Tasks::new(TasksOptions::default()))
	}
}

struct RequestExecCtx {
	exec_ctx: ExecCtx<crate::Error>,
	_cancel_on_drop: CancelOnDrop,
}

struct CancelOnDrop(CancelToken);

impl Drop for CancelOnDrop {
	fn drop(&mut self) {
		self.0.cancel();
	}
}

/// Request facts available to route handlers.
#[derive(Clone, Debug)]
pub struct RequestInput {
	method: Method,
	uri: Uri,
	path: String,
	query: Option<String>,
	headers: HeaderMap,
	extensions: Extensions,
	body: Bytes,
}

impl RequestInput {
	/// Create request input from method and path.
	pub fn new(method: Method, path: impl Into<String>) -> Self {
		let path = path.into();
		Self {
			method,
			uri: uri_from_path_and_query(&path, None),
			path,
			query: None,
			headers: HeaderMap::new(),
			extensions: Extensions::new(),
			body: Bytes::new(),
		}
	}

	/// Create request input from an HTTP URI while keeping a decoded routing path.
	pub(crate) fn from_http_uri(method: Method, uri: Uri, decoded_path: impl Into<String>) -> Self {
		Self {
			method,
			query: uri.query().map(ToOwned::to_owned),
			uri,
			path: decoded_path.into(),
			headers: HeaderMap::new(),
			extensions: Extensions::new(),
			body: Bytes::new(),
		}
	}

	/// Set the raw query string.
	pub fn with_query(mut self, query: impl Into<String>) -> Self {
		let query = query.into();
		self.uri = uri_from_path_and_query(&self.path, Some(&query));
		self.query = Some(query);
		self
	}

	/// Set request headers.
	pub fn with_headers(mut self, headers: HeaderMap) -> Self {
		self.headers = headers;
		self
	}

	/// Set request extensions.
	pub fn with_extensions(mut self, extensions: Extensions) -> Self {
		self.extensions = extensions;
		self
	}

	/// Set the raw request body.
	pub fn with_body(mut self, body: Bytes) -> Self {
		self.body = body;
		self
	}

	/// HTTP method.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// Original request URI.
	pub fn uri(&self) -> &Uri {
		&self.uri
	}

	/// Request path.
	pub fn path(&self) -> &str {
		&self.path
	}

	/// Raw query string.
	pub fn query(&self) -> Option<&str> {
		self.query.as_deref()
	}

	/// Request headers.
	pub fn headers(&self) -> &HeaderMap {
		&self.headers
	}

	/// Request extensions.
	pub fn extensions(&self) -> &Extensions {
		&self.extensions
	}

	/// Return one typed request extension.
	pub fn extension<T: Send + Sync + 'static>(&self) -> Option<&T> {
		self.extensions.get::<T>()
	}

	/// Raw request body.
	pub fn body(&self) -> &Bytes {
		&self.body
	}
}

fn uri_from_path_and_query(path: &str, query: Option<&str>) -> Uri {
	let uri = match query {
		Some(query) => format!("{path}?{query}"),
		None => path.to_owned(),
	};
	uri.parse()
		.expect("internal request input paths must form a valid URI")
}

/// Handler role in one request execution plan.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum HandlerRole {
	/// Middleware handler.
	Middleware,
	/// Resource handler.
	Resource,
	/// View handler.
	View,
}

/// Input passed to one runtime handler invocation.
#[derive(Clone, Debug)]
pub struct HandlerInput {
	request: Arc<RequestInput>,
	asset_capabilities: AssetCapabilities,
	role: HandlerRole,
	pattern: String,
	params: Params,
	splat_values: vorma_matcher::SplatValues,
	query_params: Arc<BTreeMap<String, Vec<String>>>,
	decoded_input: DecodedRouteInput,
}

impl HandlerInput {
	/// Original request facts.
	pub fn request(&self) -> &RequestInput {
		&self.request
	}

	/// Resolve a logical public source path to a committed public URL.
	pub fn public_url(&self, src_path: &str) -> Result<&str, PublicUrlError> {
		self.asset_capabilities.public_url(src_path)
	}

	/// Handler role.
	pub fn role(&self) -> HandlerRole {
		self.role
	}

	/// Matched route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Captured params.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Captured splat values.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		&self.splat_values
	}

	/// Decoded query values keyed by query parameter name.
	pub fn query_params(&self) -> &BTreeMap<String, Vec<String>> {
		self.query_params.as_ref()
	}

	/// Decoded route input value.
	pub fn decoded_input(&self) -> &DecodedRouteInput {
		&self.decoded_input
	}
}

/// Runtime handler output before request-level commit rules are applied.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct HandlerOutput {
	effects: ResponseEffects,
	data: Option<Value>,
	body: Option<Bytes>,
}

impl HandlerOutput {
	/// Create an empty handler output.
	pub fn empty() -> Self {
		Self::default()
	}

	/// Create a handler output containing route data.
	pub fn data(data: Value) -> Self {
		Self {
			effects: ResponseEffects::default(),
			data: Some(data),
			body: None,
		}
	}

	/// Create a handler output containing a resource body.
	pub fn body(body: Bytes) -> Self {
		Self {
			effects: ResponseEffects::default(),
			data: None,
			body: Some(body),
		}
	}

	/// Replace response effects on this output.
	pub fn with_effects(mut self, effects: ResponseEffects) -> Self {
		self.effects = effects;
		self
	}

	/// Response effects.
	pub fn effects(&self) -> &ResponseEffects {
		&self.effects
	}

	/// Route data.
	pub fn data_value(&self) -> Option<&Value> {
		self.data.as_ref()
	}

	/// Resource body.
	pub fn body_value(&self) -> Option<&Bytes> {
		self.body.as_ref()
	}
}

/// Error returned by an individual runtime handler.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HandlerExecutionError {
	message: String,
	status: Option<http::StatusCode>,
	client_message: Option<String>,
	effects: Box<ResponseEffects>,
}

impl HandlerExecutionError {
	/// Create a handler execution error with no client-visible facts.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
			status: None,
			client_message: None,
			effects: Box::default(),
		}
	}

	/// Create a handler execution error from a framework error.
	pub fn from_application_error(error: &AppError) -> Self {
		Self::new(error.to_string())
	}

	/// Set the response status this handler error carries.
	pub fn with_status(mut self, status: http::StatusCode) -> Self {
		self.status = Some(status);
		self
	}

	/// Create a handler execution error from a task runtime error.
	pub fn from_task_error(error: vorma_tasks::Error<AppError>) -> Self {
		match error {
			vorma_tasks::Error::Failed(error) => Self::from_application_error(error.as_ref()),
			error => Self::new(error.to_string()),
		}
	}

	/// Create a handler execution error with a browser-visible message.
	pub fn with_client_message(
		message: impl Into<String>,
		client_message: impl Into<String>,
	) -> Self {
		Self {
			message: message.into(),
			status: None,
			client_message: Some(client_message.into()),
			effects: Box::default(),
		}
	}

	/// Attach terminal response effects to this handler error.
	pub fn with_effects(mut self, effects: ResponseEffects) -> Self {
		self.effects = Box::new(effects);
		self
	}

	/// Error message.
	pub fn message(&self) -> &str {
		&self.message
	}

	/// Browser-visible error message, when the error carries one.
	pub fn client_message(&self) -> Option<&str> {
		self.client_message.as_deref()
	}

	/// Response status carried by the error, when it carries one.
	pub fn status(&self) -> Option<http::StatusCode> {
		self.status
	}

	/// Handler-owned error response effects.
	pub fn effects(&self) -> &ResponseEffects {
		&self.effects
	}
}

/// Classified request target.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RequestTarget {
	/// Manifest-listed public asset.
	PublicAsset(PublicAsset),
	/// Resource route match.
	Resource(ResourceMatch),
	/// Known path whose routes do not serve this method.
	MethodNotAllowed(Vec<Method>),
	/// Nested view route match.
	View(ViewMatches),
	/// No framework target matched.
	NotFound,
}

/// Runtime request classifier and executor.
#[derive(Clone, Debug)]
pub struct ExecutionEngine {
	plan: ExecutionPlan,
	assets: AssetCapabilities,
}

impl ExecutionEngine {
	/// Create a runtime request classifier.
	pub fn new(plan: ExecutionPlan, assets: AssetCapabilities) -> Self {
		Self { plan, assets }
	}

	/// Committed public asset capabilities.
	pub fn asset_capabilities(&self) -> &AssetCapabilities {
		&self.assets
	}

	/// Classify an HTTP method and path against the committed snapshot.
	/*
	Views and GET/HEAD resources share one URL space, adjudicated by the
	matcher's specificity order — the same order that picks among views.
	Graph validation rejects cross-table specificity ties, so the
	comparison below always has a winner. The static asset space stays a
	reserved prefix kept disjoint from GET/HEAD resources at graph
	compile, and views never serve other methods.
	*/
	pub fn classify(&self, method: &Method, path: &str) -> RequestTarget {
		if *method == Method::GET || *method == Method::HEAD {
			if let Some(asset) = self.assets.asset_for_request_path(path) {
				return RequestTarget::PublicAsset(asset);
			}
			match (
				self.plan.match_resource(method, path),
				self.plan.match_views(path),
			) {
				(Some(resource), Some(view)) => {
					// Ties cannot compile; Equal routes to the resource
					// purely for deterministic exhaustiveness.
					let view_wins =
						compare_specificity(view.leaf_pattern(), resource.matched_pattern())
							== Ordering::Greater;
					if view_wins {
						return RequestTarget::View(view);
					}
					return RequestTarget::Resource(resource);
				}
				(Some(resource), None) => return RequestTarget::Resource(resource),
				(None, Some(view)) => return RequestTarget::View(view),
				(None, None) => {}
			}
		} else if let Some(resource) = self.plan.match_resource(method, path) {
			return RequestTarget::Resource(resource);
		}
		let methods = self.allowed_methods(path);
		if !methods.is_empty() {
			// A request method in this list would have returned a target
			// above, so reaching here means the method is not allowed.
			return RequestTarget::MethodNotAllowed(methods);
		}
		RequestTarget::NotFound
	}

	// Methods that would succeed for this path: resource methods, plus
	// GET/HEAD when a view or public asset serves it.
	fn allowed_methods(&self, path: &str) -> Vec<Method> {
		let mut methods = self.plan.allowed_resource_methods(path);
		if !methods.contains(&Method::GET)
			&& (self.assets.asset_for_request_path(path).is_some()
				|| self.plan.match_views(path).is_some())
		{
			methods.push(Method::GET);
			if !methods.contains(&Method::HEAD) {
				methods.push(Method::HEAD);
			}
			methods.sort_by(|left, right| left.as_str().cmp(right.as_str()));
		}
		methods
	}

	/// Execute one request against a handler registry.
	pub async fn execute(
		&self,
		request: RequestInput,
		handlers: &HandlerRegistry,
	) -> Result<RequestExecutionReport, ExecutionError>
where {
		let request_exec_ctx = handlers.request_exec_ctx();
		match self.classify(request.method(), request.path()) {
			RequestTarget::PublicAsset(asset) => Ok(RequestExecutionReport::PublicAsset(asset)),
			RequestTarget::Resource(resource) => {
				let middleware_ids = self
					.plan
					.matching_middleware_ids(request.method(), request.path());
				let route =
					resource_execution(request, resource, middleware_ids, self.plan.type_defs())
						.await?;
				execute_invocations(
					route,
					handlers,
					self.assets.clone(),
					request_exec_ctx.exec_ctx,
				)
				.await
				.map(RequestExecutionReport::Resource)
			}
			RequestTarget::MethodNotAllowed(methods) => {
				Ok(RequestExecutionReport::MethodNotAllowed(methods))
			}
			RequestTarget::View(views) => {
				let middleware_ids = self
					.plan
					.matching_middleware_ids(request.method(), request.path());
				let route = view_execution(request, views, middleware_ids, self.plan.type_defs())?;
				execute_invocations(
					route,
					handlers,
					self.assets.clone(),
					request_exec_ctx.exec_ctx,
				)
				.await
				.map(RequestExecutionReport::View)
			}
			RequestTarget::NotFound => Ok(RequestExecutionReport::NotFound),
		}
	}
}

/// Completed request execution report.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RequestExecutionReport {
	/// Manifest-listed public asset.
	PublicAsset(PublicAsset),
	/// Completed resource execution.
	Resource(RouteExecutionReport),
	/// Known path whose routes do not serve this method.
	MethodNotAllowed(Vec<Method>),
	/// Completed view execution.
	View(RouteExecutionReport),
	/// No framework target matched.
	NotFound,
}

/// Route execution report after terminal-boundary commit rules.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct RouteExecutionReport {
	matched_patterns: Vec<String>,
	params: Params,
	splat_values: vorma_matcher::SplatValues,
	suppress_body: bool,
	committed: Vec<HandlerCommit>,
	suppressed: Vec<SuppressedHandler>,
	server_error: Option<RouteServerError>,
	effects: ResponseEffects,
	terminal_handler_id: Option<HandlerId>,
}

impl RouteExecutionReport {
	/// Matched route patterns from outermost to innermost.
	pub fn matched_patterns(&self) -> &[String] {
		&self.matched_patterns
	}

	/// Captured params.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Captured splat values.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		&self.splat_values
	}

	/// Whether HTTP finalization must suppress the response body.
	pub fn suppress_body(&self) -> bool {
		self.suppress_body
	}

	/// Handler outputs that committed in execution-plan order.
	pub fn committed(&self) -> &[HandlerCommit] {
		&self.committed
	}

	/// Handlers suppressed by an earlier terminal boundary.
	pub fn suppressed(&self) -> &[SuppressedHandler] {
		&self.suppressed
	}

	/// Browser-visible server error from the outermost failed view handler, if any.
	pub fn server_error(&self) -> Option<&RouteServerError> {
		self.server_error.as_ref()
	}

	/// Merged committed response effects.
	pub fn effects(&self) -> &ResponseEffects {
		&self.effects
	}

	/// Handler that established the terminal boundary, if any.
	pub fn terminal_handler_id(&self) -> Option<&HandlerId> {
		self.terminal_handler_id.as_ref()
	}
}

/// Browser-visible server error from a failed view handler.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RouteServerError {
	handler_id: HandlerId,
	pattern: String,
	view_index: usize,
	status: Option<http::StatusCode>,
	client_message: String,
}

impl RouteServerError {
	/// Failed view handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}

	/// Failed view route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Failed view index in the matched view chain.
	pub fn view_index(&self) -> usize {
		self.view_index
	}

	/// Browser-visible error message.
	pub fn client_message(&self) -> &str {
		&self.client_message
	}
}

/// Handler suppressed by an earlier terminal boundary.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SuppressedHandler {
	handler_id: HandlerId,
	role: HandlerRole,
	pattern: String,
}

impl SuppressedHandler {
	/// Runtime handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}

	/// Handler role.
	pub fn role(&self) -> HandlerRole {
		self.role
	}

	/// Matched route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}
}

/// Committed handler output.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HandlerCommit {
	handler_id: HandlerId,
	role: HandlerRole,
	pattern: String,
	output: HandlerOutput,
}

impl HandlerCommit {
	/// Runtime handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}

	/// Handler role.
	pub fn role(&self) -> HandlerRole {
		self.role
	}

	/// Matched route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Handler output.
	pub fn output(&self) -> &HandlerOutput {
		&self.output
	}
}

/// Request execution error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ExecutionError {
	/// A committed plan referenced a handler missing from the registry.
	MissingHandler {
		/// Missing runtime handler identifier.
		handler_id: HandlerId,
	},
	/// Route input decoding failed before handler invocation.
	InputFailed {
		/// Matched route pattern whose input failed to decode.
		pattern: String,
		/// Input decoding error message.
		message: String,
	},
}

#[derive(Clone)]
struct HandlerInvocation {
	handler_id: HandlerId,
	role: HandlerRole,
	pattern: String,
	params: Params,
	splat_values: vorma_matcher::SplatValues,
	request: Arc<RequestInput>,
	decoded_input: DecodedRouteInput,
}

struct RouteExecution {
	matched_patterns: Vec<String>,
	params: Params,
	splat_values: vorma_matcher::SplatValues,
	query_params: Arc<BTreeMap<String, Vec<String>>>,
	suppress_body: bool,
	invocations: Vec<HandlerInvocation>,
}

async fn resource_execution(
	request: RequestInput,
	resource: ResourceMatch,
	middleware_ids: Vec<HandlerId>,
	type_defs: &[crate::contracts::TypeDef],
) -> Result<RouteExecution, ExecutionError> {
	let query_params = Arc::new(decode_query_params(request.query()));
	let suppress_body = request.method() == Method::HEAD;
	let decoded_input = decode_resource_input(&request, resource.input_type(), type_defs)
		.await
		.map_err(|error| ExecutionError::InputFailed {
			pattern: resource.pattern().to_owned(),
			message: error.to_string(),
		})?;
	let request = Arc::new(request);
	let mut invocations = middleware_ids
		.into_iter()
		.map(|handler_id| HandlerInvocation {
			handler_id,
			role: HandlerRole::Middleware,
			pattern: resource.pattern().to_owned(),
			params: resource.params().clone(),
			splat_values: resource.splat_values().clone(),
			request: Arc::clone(&request),
			decoded_input: DecodedRouteInput::default(),
		})
		.collect::<Vec<_>>();
	invocations.push(HandlerInvocation {
		handler_id: resource.handler_id().clone(),
		role: HandlerRole::Resource,
		pattern: resource.pattern().to_owned(),
		params: resource.params().clone(),
		splat_values: resource.splat_values().clone(),
		request: Arc::clone(&request),
		decoded_input,
	});
	Ok(RouteExecution {
		matched_patterns: vec![resource.pattern().to_owned()],
		params: resource.params().clone(),
		splat_values: resource.splat_values().clone(),
		query_params,
		suppress_body,
		invocations,
	})
}

fn view_execution(
	request: RequestInput,
	views: ViewMatches,
	middleware_ids: Vec<HandlerId>,
	type_defs: &[crate::contracts::TypeDef],
) -> Result<RouteExecution, ExecutionError> {
	let query_params = Arc::new(decode_query_params(request.query()));
	let suppress_body = request.method() == Method::HEAD;
	let request = Arc::new(request);
	/*
	Middlewares are selected per REQUEST (declaration order, filtered by
	the plan); their route facts are request-wide. The pattern recorded
	on a middleware invocation is the leaf match, used only as internal
	log context.
	*/
	let leaf_pattern = views
		.nodes()
		.last()
		.map(|view| view.pattern().to_owned())
		.unwrap_or_default();
	let mut invocations = middleware_ids
		.into_iter()
		.map(|handler_id| HandlerInvocation {
			handler_id,
			role: HandlerRole::Middleware,
			pattern: leaf_pattern.clone(),
			params: views.params().clone(),
			splat_values: views.splat_values().clone(),
			request: Arc::clone(&request),
			decoded_input: DecodedRouteInput::default(),
		})
		.collect::<Vec<_>>();
	for view in views.nodes() {
		let decoded_input =
			decode_view_input(&request, view.input_type(), type_defs).map_err(|error| {
				ExecutionError::InputFailed {
					pattern: view.pattern().to_owned(),
					message: error.to_string(),
				}
			})?;
		invocations.push(HandlerInvocation {
			handler_id: view.handler_id().clone(),
			role: HandlerRole::View,
			pattern: view.pattern().to_owned(),
			params: views.params().clone(),
			splat_values: views.splat_values().clone(),
			request: Arc::clone(&request),
			decoded_input,
		});
	}
	Ok(RouteExecution {
		matched_patterns: views
			.nodes()
			.iter()
			.map(|view| view.pattern().to_owned())
			.collect(),
		params: views.params().clone(),
		splat_values: views.splat_values().clone(),
		query_params,
		suppress_body,
		invocations,
	})
}

async fn execute_invocations(
	route: RouteExecution,
	handlers: &HandlerRegistry,
	asset_capabilities: AssetCapabilities,
	exec_ctx: ExecCtx<crate::Error>,
) -> Result<RouteExecutionReport, ExecutionError>
where
{
	let prepared = route
		.invocations
		.iter()
		.map(|invocation| {
			handlers
				.handler(&invocation.handler_id)
				.map(|handler| (invocation.clone(), handler))
		})
		.collect::<Result<Vec<_>, _>>()?;
	let execution_contexts = ordered_execution_contexts(exec_ctx, prepared.len());
	let mut report = RouteExecutionReport {
		matched_patterns: route.matched_patterns,
		params: route.params,
		splat_values: route.splat_values,
		suppress_body: route.suppress_body,
		..RouteExecutionReport::default()
	};

	let middleware_indexes = prepared
		.iter()
		.enumerate()
		.filter_map(|(idx, (invocation, _))| {
			(invocation.role == HandlerRole::Middleware).then_some(idx)
		})
		.collect::<Vec<_>>();
	let handler_indexes = prepared
		.iter()
		.enumerate()
		.filter_map(|(idx, (invocation, _))| {
			(invocation.role != HandlerRole::Middleware).then_some(idx)
		})
		.collect::<Vec<_>>();

	if let Some(boundary) = execute_invocation_phase(
		&middleware_indexes,
		&prepared,
		&route.query_params,
		&asset_capabilities,
		&execution_contexts,
		&mut report,
	)
	.await?
	{
		let suppressed = middleware_indexes
			.iter()
			.skip(boundary.phase_position + 1)
			.chain(handler_indexes.iter())
			.copied()
			.collect::<Vec<_>>();
		cancel_execution_contexts(&execution_contexts, &suppressed);
		suppress_invocation_indexes(&mut report, &prepared, &suppressed);
		return Ok(report);
	}

	if let Some(boundary) = execute_invocation_phase(
		&handler_indexes,
		&prepared,
		&route.query_params,
		&asset_capabilities,
		&execution_contexts,
		&mut report,
	)
	.await?
	{
		let suppressed = handler_indexes
			.iter()
			.skip(boundary.phase_position + 1)
			.copied()
			.collect::<Vec<_>>();
		cancel_execution_contexts(&execution_contexts, &suppressed);
		suppress_invocation_indexes(&mut report, &prepared, &suppressed);
		return Ok(report);
	}

	Ok(report)
}

struct PhaseBoundary {
	phase_position: usize,
}

async fn execute_invocation_phase(
	phase_indexes: &[usize],
	prepared: &[(HandlerInvocation, Arc<dyn RuntimeHandler>)],
	route_query_params: &Arc<BTreeMap<String, Vec<String>>>,
	asset_capabilities: &AssetCapabilities,
	execution_contexts: &[ExecCtx<crate::Error>],
	report: &mut RouteExecutionReport,
) -> Result<Option<PhaseBoundary>, ExecutionError>
where
{
	let mut pending = FuturesUnordered::new();
	for (phase_position, idx) in phase_indexes.iter().copied().enumerate() {
		let (invocation, handler) = &prepared[idx];
		let query_params = Arc::clone(route_query_params);
		let asset_capabilities = asset_capabilities.clone();
		let exec_ctx = execution_contexts[idx].clone();
		pending.push(async move {
			let input = HandlerInput {
				request: Arc::clone(&invocation.request),
				asset_capabilities,
				role: invocation.role,
				pattern: invocation.pattern.clone(),
				params: invocation.params.clone(),
				splat_values: invocation.splat_values.clone(),
				query_params,
				decoded_input: invocation.decoded_input.clone(),
			};
			(phase_position, idx, handler.call(input, exec_ctx).await)
		})
	}
	let mut outputs = std::iter::repeat_with(|| None)
		.take(phase_indexes.len())
		.collect::<Vec<_>>();
	let mut next_to_commit = 0;
	while let Some((phase_position, _global_idx, output)) = pending.next().await {
		outputs[phase_position] = Some(output);
		while next_to_commit < outputs.len() {
			let Some(output) = outputs[next_to_commit].take() else {
				break;
			};
			let global_idx = phase_indexes[next_to_commit];
			let invocation = &prepared[global_idx].0;
			let output = match output {
				Ok(output) => output,
				Err(error) if invocation.role == HandlerRole::View => {
					if error.effects().is_terminal() {
						finish_terminal_report(report, invocation, error.effects());
						return Ok(Some(PhaseBoundary {
							phase_position: next_to_commit,
						}));
					}
					report.server_error = Some(RouteServerError {
						handler_id: invocation.handler_id.clone(),
						pattern: invocation.pattern.clone(),
						view_index: view_index_for_prepared_invocation(prepared, global_idx),
						status: error.status,
						client_message: error
							.client_message
							.unwrap_or_else(|| DEFAULT_SERVER_ERROR_MESSAGE.to_owned()),
					});
					return Ok(Some(PhaseBoundary {
						phase_position: next_to_commit,
					}));
				}
				Err(error) => {
					/*
					Terminal precedence: handler-owned terminal effects (e.g.
					a redirect) win; otherwise the response is built FROM the
					returned error — its status and client text, with the
					conventional fallbacks when the error carries neither.
					*/
					let mut internal_error_effects;
					let effects = if error.effects().is_terminal() {
						error.effects()
					} else {
						internal_error_effects = ResponseEffects::default();
						internal_error_effects.set_status_with_text(
							error
								.status
								.unwrap_or(http::StatusCode::INTERNAL_SERVER_ERROR),
							error
								.client_message
								.as_deref()
								.unwrap_or(INTERNAL_SERVER_ERROR_STATUS_TEXT),
						);
						&internal_error_effects
					};
					finish_terminal_report(report, invocation, effects);
					return Ok(Some(PhaseBoundary {
						phase_position: next_to_commit,
					}));
				}
			};
			let terminal = output.effects().is_terminal();
			let commit = HandlerCommit {
				handler_id: invocation.handler_id.clone(),
				role: invocation.role,
				pattern: invocation.pattern.clone(),
				output,
			};
			report.effects.merge_from(commit.output.effects());
			if terminal {
				report.terminal_handler_id = Some(commit.handler_id.clone());
			}
			report.committed.push(commit);
			next_to_commit += 1;
			if terminal {
				return Ok(Some(PhaseBoundary {
					phase_position: next_to_commit - 1,
				}));
			}
		}
	}
	Ok(None)
}

fn ordered_execution_contexts(
	exec_ctx: ExecCtx<crate::Error>,
	count: usize,
) -> Vec<ExecCtx<crate::Error>>
where
{
	(0..count).map(|_| exec_ctx.child()).collect()
}

fn cancel_execution_contexts(execution_contexts: &[ExecCtx<crate::Error>], indexes: &[usize]) {
	for index in indexes {
		if let Some(exec_ctx) = execution_contexts.get(*index) {
			exec_ctx.cancel_token().cancel();
		}
	}
}

fn suppress_invocation_indexes(
	report: &mut RouteExecutionReport,
	prepared: &[(HandlerInvocation, Arc<dyn RuntimeHandler>)],
	indexes: &[usize],
) {
	report.suppressed.extend(indexes.iter().map(|idx| {
		let invocation = &prepared[*idx].0;
		SuppressedHandler {
			handler_id: invocation.handler_id.clone(),
			role: invocation.role,
			pattern: invocation.pattern.clone(),
		}
	}));
}

fn finish_terminal_report(
	report: &mut RouteExecutionReport,
	invocation: &HandlerInvocation,
	effects: &ResponseEffects,
) {
	report.effects.merge_from(effects);
	report.terminal_handler_id = Some(invocation.handler_id.clone());
}

fn view_index_for_prepared_invocation(
	prepared: &[(HandlerInvocation, Arc<dyn RuntimeHandler>)],
	idx: usize,
) -> usize
where
{
	prepared
		.iter()
		.take(idx + 1)
		.filter(|(invocation, _)| invocation.role == HandlerRole::View)
		.count()
		.saturating_sub(1)
}

fn decode_query_params(query: Option<&str>) -> BTreeMap<String, Vec<String>> {
	let Some(query) = query else {
		return BTreeMap::new();
	};
	let mut decoded = BTreeMap::<String, Vec<String>>::new();
	for (key, value) in form_urlencoded::parse(query.as_bytes()) {
		decoded
			.entry(key.into_owned())
			.or_default()
			.push(value.into_owned());
	}
	decoded
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::sync::atomic::{AtomicUsize, Ordering};
	use std::sync::{Arc, Mutex};
	use std::time::Duration;

	use http::StatusCode;
	use tokio::sync::{Barrier, Notify, oneshot};
	use vorma_tasks::{CancelToken, ExecCtx, Task};

	use super::*;
	use crate::contracts::{FieldDef, RouteTypeContract, TypeDef, TypeRefContract};
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, MiddlewareDeclaration,
		ResourceDeclaration, ViewDeclaration,
	};
	use crate::response_finalizer::VORMA_JSON_QUERY_KEY;
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn named_type(key: &str, name: &str) -> TypeRefContract {
		TypeRefContract::Named {
			key: key.to_owned(),
			name: name.to_owned(),
		}
	}

	fn test_engine(declarations: FrameworkDeclarations) -> ExecutionEngine {
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.css".to_owned())]),
		)
		.unwrap();
		ExecutionEngine::new(plan, assets)
	}

	#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
	struct SharedInput;

	#[derive(Clone)]
	struct SharedTaskHandler {
		task: Task<SharedInput, String, crate::Error>,
		output: &'static str,
	}

	impl RuntimeHandler for SharedTaskHandler {
		fn call(&self, _input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
			let task = self.task.clone();
			let output = self.output;
			Box::pin(async move {
				let _ = task
					.run(&exec_ctx, SharedInput)
					.await
					.map_err(|source| HandlerExecutionError::new(source.to_string()))?;
				Ok(HandlerOutput::data(serde_json::json!(output)))
			})
		}
	}

	struct PendingTokenHandler {
		token_tx: Arc<Mutex<Option<oneshot::Sender<CancelToken>>>>,
		started: Option<Arc<Notify>>,
	}

	impl RuntimeHandler for PendingTokenHandler {
		fn call(&self, _input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
			let token_tx = Arc::clone(&self.token_tx);
			let started = self.started.clone();
			Box::pin(async move {
				if let Some(token_tx) = token_tx.lock().unwrap().take() {
					let _ = token_tx.send(exec_ctx.cancel_token().clone());
				}
				if let Some(started) = &started {
					started.notify_waiters();
				}
				std::future::pending::<Result<HandlerOutput, HandlerExecutionError>>().await
			})
		}
	}

	/*
	Views and GET resources adjudicate shared paths by specificity, the
	same order that picks among views: the more specific pattern owns the
	path, in either direction, and a view splat yields exactly the paths
	a more specific resource pins.
	*/
	#[test]
	fn classifier_adjudicates_view_resource_overlaps_by_specificity() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/:story_id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/s/export",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("export.resource"),
		));
		let engine = test_engine(declarations);
		assert!(matches!(
			engine.classify(&Method::GET, "/s/export"),
			RequestTarget::Resource(_)
		));
		assert!(matches!(
			engine.classify(&Method::GET, "/s/123"),
			RequestTarget::View(_)
		));

		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/export",
			"export.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("export.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/s/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("story.resource"),
		));
		let engine = test_engine(declarations);
		assert!(matches!(
			engine.classify(&Method::GET, "/s/export"),
			RequestTarget::View(_)
		));
		assert!(matches!(
			engine.classify(&Method::GET, "/s/123"),
			RequestTarget::Resource(_)
		));

		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/bob/*",
			"bob.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("bob.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/bob/sally",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("sally.resource"),
		));
		let engine = test_engine(declarations);
		assert!(matches!(
			engine.classify(&Method::GET, "/bob/sally"),
			RequestTarget::Resource(_)
		));
		assert!(matches!(
			engine.classify(&Method::GET, "/bob/anything-else"),
			RequestTarget::View(_)
		));
	}

	#[test]
	fn classifier_prefers_resources_then_assets_then_views() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		let engine = test_engine(declarations);

		assert!(matches!(
			engine.classify(&Method::GET, "/api/ping"),
			RequestTarget::Resource(_)
		));
		assert!(matches!(
			engine.classify(&Method::GET, "/static/app.css"),
			RequestTarget::PublicAsset(_)
		));
		assert!(matches!(
			engine.classify(&Method::GET, "/"),
			RequestTarget::View(_)
		));
	}

	#[tokio::test]
	async fn resource_execution_gates_handler_until_middleware_finishes() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let middleware_started = Arc::new(Notify::new());
		let middleware_can_finish = Arc::new(Notify::new());
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		{
			let middleware_started = Arc::clone(&middleware_started);
			let middleware_can_finish = Arc::clone(&middleware_can_finish);
			handlers.insert(handler_id("middleware"), move |_| {
				let middleware_started = Arc::clone(&middleware_started);
				let middleware_can_finish = Arc::clone(&middleware_can_finish);
				async move {
					middleware_started.notify_one();
					middleware_can_finish.notified().await;
					Ok(HandlerOutput::data(serde_json::json!("middleware")))
				}
			});
		}
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::data(serde_json::json!("resource")))
				}
			});
		}

		let execution = tokio::spawn(async move {
			engine
				.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
				.await
		});
		tokio::time::timeout(Duration::from_secs(1), middleware_started.notified())
			.await
			.unwrap();
		tokio::task::yield_now().await;
		assert_eq!(handler_runs.load(Ordering::SeqCst), 0);

		middleware_can_finish.notify_one();
		let report = tokio::time::timeout(Duration::from_secs(1), execution)
			.await
			.unwrap()
			.unwrap()
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report
				.committed()
				.iter()
				.map(|commit| commit.handler_id().as_str())
				.collect::<Vec<_>>(),
			["middleware", "resource"]
		);
	}

	#[tokio::test]
	async fn middleware_filters_select_by_pattern_and_method() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("everywhere")));
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("admin.gate"))
				.with_patterns(["/admin", "/admin/*"]),
		);
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("mutations.only")).with_methods([Method::POST]),
		);
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("admin.posts"))
				.with_patterns(["/admin/*"])
				.with_methods([Method::POST]),
		);
		for (pattern, file, id) in [
			("/", "root.tsx", "root"),
			("/admin", "admin.tsx", "admin"),
			("/admin/users/:id", "admin-user.tsx", "admin.user"),
		] {
			declarations.add_view(ViewDeclaration::new(
				pattern,
				file,
				serde_json::json!({}),
				route_type_contract(),
				handler_id(id),
			));
		}
		let engine = test_engine(declarations);

		/*
		Selection is by request path against each middleware's own flat
		matcher plus method membership; filters AND together and the
		declaration order is preserved.
		*/
		let ids = |method: Method, path: &str| {
			engine
				.plan
				.matching_middleware_ids(&method, path)
				.iter()
				.map(|id| id.as_str().to_owned())
				.collect::<Vec<_>>()
		};
		assert_eq!(ids(Method::GET, "/"), ["everywhere"]);
		assert_eq!(ids(Method::GET, "/admin"), ["everywhere", "admin.gate"]);
		assert_eq!(
			ids(Method::POST, "/admin/users/7"),
			["everywhere", "admin.gate", "mutations.only", "admin.posts"]
		);
		assert_eq!(
			ids(Method::GET, "/admin/users/7"),
			["everywhere", "admin.gate"]
		);
	}

	#[tokio::test]
	async fn scope_patterns_match_urls_with_no_hidden_rewrites() {
		/*
		Scope patterns are URL patterns: a "/mod" splat scope does NOT
		cover a resource whose URL carries the api mount, and an
		"/api/mod" splat scope does. The two spaces stay separately
		addressable — no stripping behind the author's back.
		*/
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("view_gate")).with_patterns(["/mod", "/mod/*"]),
		);
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("gate")).with_patterns(["/api/mod/*"]),
		);
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/mod/items/:id/kill",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("kill"),
		));
		let engine = test_engine(declarations);
		let view_gate_runs = Arc::new(AtomicUsize::new(0));
		let gate_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		{
			let view_gate_runs = Arc::clone(&view_gate_runs);
			handlers.insert(handler_id("view_gate"), move |_| {
				let view_gate_runs = Arc::clone(&view_gate_runs);
				async move {
					view_gate_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::empty())
				}
			});
		}
		{
			let gate_runs = Arc::clone(&gate_runs);
			handlers.insert(handler_id("gate"), move |_| {
				let gate_runs = Arc::clone(&gate_runs);
				async move {
					gate_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::empty())
				}
			});
		}
		handlers.insert(handler_id("kill"), move |_| async move {
			Ok(HandlerOutput::data(serde_json::json!({})))
		});

		engine
			.execute(
				RequestInput::new(Method::POST, "/api/mod/items/9/kill"),
				&handlers,
			)
			.await
			.unwrap();
		assert_eq!(view_gate_runs.load(Ordering::SeqCst), 0);
		assert_eq!(gate_runs.load(Ordering::SeqCst), 1);
	}

	#[tokio::test]
	async fn scoped_middleware_runs_only_inside_its_patterns() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("gate")).with_patterns(["/admin", "/admin/*"]),
		);
		for (pattern, file, id) in [("/", "root.tsx", "root"), ("/admin", "admin.tsx", "admin")] {
			declarations.add_view(ViewDeclaration::new(
				pattern,
				file,
				serde_json::json!({}),
				route_type_contract(),
				handler_id(id),
			));
		}
		let engine = test_engine(declarations);
		let gate_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		{
			let gate_runs = Arc::clone(&gate_runs);
			handlers.insert(handler_id("gate"), move |_| {
				let gate_runs = Arc::clone(&gate_runs);
				async move {
					gate_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::empty())
				}
			});
		}
		for id in ["root", "admin"] {
			handlers.insert(handler_id(id), move |_| async move {
				Ok(HandlerOutput::data(serde_json::json!({})))
			});
		}

		engine
			.execute(RequestInput::new(Method::GET, "/"), &handlers)
			.await
			.unwrap();
		assert_eq!(gate_runs.load(Ordering::SeqCst), 0);

		engine
			.execute(RequestInput::new(Method::GET, "/admin"), &handlers)
			.await
			.unwrap();
		assert_eq!(gate_runs.load(Ordering::SeqCst), 1);
	}

	#[tokio::test]
	async fn middleware_invocations_execute_in_parallel_before_handler_phase() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("first")));
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("second")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let barrier = Arc::new(Barrier::new(2));
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		for id in ["first", "second"] {
			let barrier = Arc::clone(&barrier);
			handlers.insert(handler_id(id), move |_| {
				let barrier = Arc::clone(&barrier);
				async move {
					barrier.wait().await;
					Ok(HandlerOutput::empty())
				}
			});
		}
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::data(serde_json::json!("resource")))
				}
			});
		}

		let report = tokio::time::timeout(
			Duration::from_secs(1),
			engine.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers),
		)
		.await
		.unwrap()
		.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(handler_runs.load(Ordering::SeqCst), 1);
		assert_eq!(
			report
				.committed()
				.iter()
				.map(|commit| commit.handler_id().as_str())
				.collect::<Vec<_>>(),
			["first", "second", "resource"]
		);
	}

	#[tokio::test]
	async fn route_and_middleware_share_one_request_task_scope() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/shared",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let runs = Arc::new(AtomicUsize::new(0));
		let task_runs = Arc::clone(&runs);
		let task = Task::new(Duration::ZERO, move |_ctx, _input: SharedInput| {
			let task_runs = Arc::clone(&task_runs);
			async move {
				task_runs.fetch_add(1, Ordering::SeqCst);
				tokio::time::sleep(Duration::from_millis(20)).await;
				Ok("shared".to_owned())
			}
		});
		let mut handlers = HandlerRegistry::default();
		handlers.insert(
			handler_id("middleware"),
			SharedTaskHandler {
				task: task.clone(),
				output: "middleware",
			},
		);
		handlers.insert(
			handler_id("resource"),
			SharedTaskHandler {
				task,
				output: "resource",
			},
		);

		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/shared"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(report.committed().len(), 2);
		assert_eq!(runs.load(Ordering::SeqCst), 1);
	}

	#[tokio::test]
	async fn terminal_boundary_suppresses_downstream_handlers_without_waiting() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers = HandlerRegistry::default();
		let handler_runs = Arc::new(AtomicUsize::new(0));
		handlers.insert(handler_id("middleware"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects.set_status_with_text(StatusCode::NOT_FOUND, "not found");
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::data(serde_json::json!("resource")))
				}
			});
		}

		let report = tokio::time::timeout(
			Duration::from_millis(500),
			engine.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers),
		)
		.await
		.unwrap()
		.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(report.committed().len(), 1);
		assert_eq!(report.suppressed().len(), 1);
		assert_eq!(report.terminal_handler_id().unwrap().as_str(), "middleware");
		assert_eq!(report.effects().status(), Some(StatusCode::NOT_FOUND));
		assert_eq!(report.suppressed()[0].handler_id().as_str(), "resource");
		assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn request_drop_cancels_running_handler_exec_ctx() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/pending",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let (token_tx, token_rx) = oneshot::channel();
		let mut handlers = HandlerRegistry::default();
		handlers.insert(
			handler_id("resource"),
			PendingTokenHandler {
				token_tx: Arc::new(Mutex::new(Some(token_tx))),
				started: None,
			},
		);
		let handle = tokio::spawn(async move {
			engine
				.execute(RequestInput::new(Method::GET, "/api/pending"), &handlers)
				.await
		});
		let token = tokio::time::timeout(Duration::from_secs(1), token_rx)
			.await
			.unwrap()
			.unwrap();

		handle.abort();
		let _ = handle.await;
		tokio::time::timeout(Duration::from_secs(1), token.cancelled())
			.await
			.unwrap();
		assert!(token.is_cancelled());
	}

	#[tokio::test]
	async fn handler_input_receives_decoded_query_values() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/search",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |input: HandlerInput| async move {
			Ok(HandlerOutput::data(serde_json::json!({
				"tag": input.query_params()["tag"],
				"q": input.query_params()["q"][0],
			})))
		});

		let report = engine
			.execute(
				RequestInput::new(Method::GET, "/api/search")
					.with_query("q=hello%20world&tag=a&tag=b"),
				&handlers,
			)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report.committed()[0].output().data_value().unwrap(),
			&serde_json::json!({"q": "hello world", "tag": ["a", "b"]})
		);
	}

	#[tokio::test]
	async fn handler_input_resolves_public_urls_from_committed_capabilities() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/asset-url",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |input: HandlerInput| async move {
			Ok(HandlerOutput::data(serde_json::json!({
				"url": input.public_url(" /app.css ").unwrap(),
			})))
		});

		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/asset-url"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report.committed()[0].output().data_value().unwrap(),
			&serde_json::json!({"url": "/static/app.css"})
		);
	}

	#[tokio::test]
	async fn resource_handler_receives_decoded_get_input_before_invocation() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Record {
			key: "SearchInput".to_owned(),
			name: "SearchInput".to_owned(),
			fields: vec![
				FieldDef::new("q", TypeRefContract::String, false),
				FieldDef::new("page", TypeRefContract::Integer, false),
			],
		});
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/search",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(
				named_type("SearchInput", "SearchInput"),
				TypeRefContract::Unknown,
			),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |input: HandlerInput| async move {
			Ok(HandlerOutput::data(input.decoded_input().value().clone()))
		});

		let report = engine
			.execute(
				RequestInput::new(Method::GET, "/api/search").with_query("q=ada&page=2"),
				&handlers,
			)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report.committed()[0].output().data_value().unwrap(),
			&serde_json::json!({"page": 2, "q": "ada"})
		);
	}

	#[tokio::test]
	async fn resource_handler_receives_decoded_form_data_input() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/form",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(TypeRefContract::FormData, TypeRefContract::Unknown),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |input: HandlerInput| async move {
			let form = input.decoded_input().parsed_form_data().unwrap();
			Ok(HandlerOutput::data(serde_json::json!({
				"name": form.text("name"),
				"tags": form.texts("tag").collect::<Vec<_>>(),
			})))
		});
		let mut headers = HeaderMap::new();
		headers.insert(
			http::header::CONTENT_TYPE,
			http::HeaderValue::from_static("application/x-www-form-urlencoded"),
		);

		let report = engine
			.execute(
				RequestInput::new(Method::POST, "/api/form")
					.with_headers(headers)
					.with_body(Bytes::from_static(b"name=ada&tag=a&tag=b")),
				&handlers,
			)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report.committed()[0].output().data_value().unwrap(),
			&serde_json::json!({"name": "ada", "tags": ["a", "b"]})
		);
	}

	#[tokio::test]
	async fn invalid_resource_input_fails_before_handler_invocation() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Record {
			key: "SearchInput".to_owned(),
			name: "SearchInput".to_owned(),
			fields: vec![FieldDef::new("page", TypeRefContract::Integer, false)],
		});
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/search",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(
				named_type("SearchInput", "SearchInput"),
				TypeRefContract::Unknown,
			),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			panic!("handler should not run after input decode failure")
		});

		let error = engine
			.execute(
				RequestInput::new(Method::GET, "/api/search").with_query("page=nope"),
				&handlers,
			)
			.await
			.unwrap_err();

		assert!(matches!(error, ExecutionError::InputFailed { .. }));
	}

	#[tokio::test]
	async fn view_handler_receives_decoded_query_input_without_internal_json_key() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Record {
			key: "ViewInput".to_owned(),
			name: "ViewInput".to_owned(),
			fields: vec![FieldDef::new("q", TypeRefContract::String, false)],
		});
		declarations.add_view(ViewDeclaration::new(
			"/search",
			"search.tsx",
			serde_json::json!({}),
			RouteTypeContract::new(
				named_type("ViewInput", "ViewInput"),
				TypeRefContract::Unknown,
			),
			handler_id("view"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("view"), |input: HandlerInput| async move {
			Ok(HandlerOutput::data(input.decoded_input().value().clone()))
		});

		let report = engine
			.execute(
				RequestInput::new(Method::GET, "/search")
					.with_query(format!("q=ada&{VORMA_JSON_QUERY_KEY}=build-id")),
				&handlers,
			)
			.await
			.unwrap();

		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		assert_eq!(
			report.committed()[0].output().data_value().unwrap(),
			&serde_json::json!({"q": "ada"})
		);
	}

	#[tokio::test]
	async fn failed_view_handler_returns_server_error_report_with_prior_commits() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"root": true})))
		});
		handlers.insert(handler_id("story"), |_| async {
			Err(HandlerExecutionError::with_client_message(
				"database unavailable",
				"client visible",
			))
		});

		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/123"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		assert_eq!(report.committed().len(), 1);
		assert_eq!(report.committed()[0].handler_id().as_str(), "root");
		assert_eq!(report.server_error().unwrap().pattern(), "/stories/:id");
		assert_eq!(report.server_error().unwrap().view_index(), 1);
		assert_eq!(
			report.server_error().unwrap().client_message(),
			"client visible"
		);
	}

	#[tokio::test]
	async fn later_middleware_redirect_waits_for_prior_sibling_success_commit() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("first")));
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("second")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let first_started = Arc::new(Notify::new());
		let first_can_finish = Arc::new(Notify::new());
		let mut handlers = HandlerRegistry::default();
		{
			let first_started = Arc::clone(&first_started);
			let first_can_finish = Arc::clone(&first_can_finish);
			handlers.insert(handler_id("first"), move |_| {
				let first_started = Arc::clone(&first_started);
				let first_can_finish = Arc::clone(&first_can_finish);
				async move {
					first_started.notify_one();
					first_can_finish.notified().await;
					Ok(HandlerOutput::data(serde_json::json!("first")))
				}
			});
		}
		handlers.insert(handler_id("second"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects
				.redirect(StatusCode::SEE_OTHER, "/login")
				.expect("relative redirect location is valid");
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("resource"), move |_| async move {
			Ok(HandlerOutput::data(serde_json::json!("resource")))
		});

		let execution = tokio::spawn(async move {
			engine
				.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
				.await
		});
		tokio::time::timeout(Duration::from_secs(1), first_started.notified())
			.await
			.unwrap();
		tokio::task::yield_now().await;
		tokio::task::yield_now().await;
		assert!(!execution.is_finished());

		first_can_finish.notify_one();
		let report = tokio::time::timeout(Duration::from_secs(1), execution)
			.await
			.unwrap()
			.unwrap()
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report
				.committed()
				.iter()
				.map(|commit| commit.handler_id().as_str())
				.collect::<Vec<_>>(),
			["first", "second"]
		);
		assert_eq!(report.terminal_handler_id().unwrap().as_str(), "second");
	}

	#[tokio::test]
	async fn middleware_error_short_circuits_without_waiting_for_later_pending_sibling() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("failing")));
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("pending")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("failing"), move |_| async move {
			Err(HandlerExecutionError::new("boom"))
		});
		handlers.insert(handler_id("pending"), move |_| {
			std::future::pending::<Result<HandlerOutput, HandlerExecutionError>>()
		});
		handlers.insert(handler_id("resource"), move |_| async move {
			Ok(HandlerOutput::data(serde_json::json!("resource")))
		});

		let report = tokio::time::timeout(
			Duration::from_millis(500),
			engine.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers),
		)
		.await
		.expect("middleware error must complete the request without waiting for later siblings")
		.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert!(report.committed().is_empty());
		assert_eq!(report.terminal_handler_id().unwrap().as_str(), "failing");
		assert_eq!(
			report.effects().status(),
			Some(StatusCode::INTERNAL_SERVER_ERROR)
		);
		assert!(
			report
				.suppressed()
				.iter()
				.any(|suppressed| suppressed.handler_id().as_str() == "resource")
		);
	}

	#[tokio::test]
	async fn prior_middleware_redirect_completes_without_waiting_for_pending_sibling() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("first")));
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("pending")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("first"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects
				.redirect(StatusCode::SEE_OTHER, "/login")
				.expect("relative redirect location is valid");
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("pending"), move |_| {
			std::future::pending::<Result<HandlerOutput, HandlerExecutionError>>()
		});
		handlers.insert(handler_id("resource"), move |_| async move {
			Ok(HandlerOutput::data(serde_json::json!("resource")))
		});

		let report = tokio::time::timeout(
			Duration::from_millis(500),
			engine.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers),
		)
		.await
		.expect("prior terminal middleware must not wait for an unfinishable later sibling")
		.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(report.committed().len(), 1);
		assert_eq!(report.committed()[0].handler_id().as_str(), "first");
		assert_eq!(report.terminal_handler_id().unwrap().as_str(), "first");
		assert!(
			report
				.suppressed()
				.iter()
				.any(|suppressed| suppressed.handler_id().as_str() == "resource")
		);
	}

	#[tokio::test]
	async fn deepest_view_success_status_wins_over_parent_status() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story"),
		));
		let engine = test_engine(declarations);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("root"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects.set_status(StatusCode::CREATED);
			Ok(HandlerOutput::data(serde_json::json!({"root": true})).with_effects(effects))
		});
		handlers.insert(handler_id("story"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects.set_status(StatusCode::PARTIAL_CONTENT);
			Ok(HandlerOutput::data(serde_json::json!({"story": true})).with_effects(effects))
		});

		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/123"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		assert_eq!(
			report
				.committed()
				.iter()
				.map(|commit| commit.handler_id().as_str())
				.collect::<Vec<_>>(),
			["root", "story"]
		);
		assert_eq!(report.effects().status(), Some(StatusCode::PARTIAL_CONTENT));
	}

	#[tokio::test]
	async fn child_view_redirect_waits_for_parent_success_commit() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story"),
		));
		let engine = test_engine(declarations);
		let root_started = Arc::new(Notify::new());
		let root_can_finish = Arc::new(Notify::new());
		let mut handlers = HandlerRegistry::default();
		{
			let root_started = Arc::clone(&root_started);
			let root_can_finish = Arc::clone(&root_can_finish);
			handlers.insert(handler_id("root"), move |_| {
				let root_started = Arc::clone(&root_started);
				let root_can_finish = Arc::clone(&root_can_finish);
				async move {
					root_started.notify_one();
					root_can_finish.notified().await;
					Ok(HandlerOutput::data(serde_json::json!({"root": true})))
				}
			});
		}
		handlers.insert(handler_id("story"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects
				.redirect(StatusCode::SEE_OTHER, "/login")
				.expect("relative redirect location is valid");
			Ok(HandlerOutput::data(serde_json::json!(null)).with_effects(effects))
		});

		let execution = tokio::spawn(async move {
			engine
				.execute(RequestInput::new(Method::GET, "/stories/123"), &handlers)
				.await
		});
		tokio::time::timeout(Duration::from_secs(1), root_started.notified())
			.await
			.unwrap();
		tokio::task::yield_now().await;
		tokio::task::yield_now().await;
		assert!(!execution.is_finished());

		root_can_finish.notify_one();
		let report = tokio::time::timeout(Duration::from_secs(1), execution)
			.await
			.unwrap()
			.unwrap()
			.unwrap();

		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		assert_eq!(
			report
				.committed()
				.iter()
				.map(|commit| commit.handler_id().as_str())
				.collect::<Vec<_>>(),
			["root", "story"]
		);
		assert_eq!(
			report.terminal_handler_id().map(HandlerId::as_str),
			Some("story")
		);
	}

	#[tokio::test]
	async fn middleware_head_elements_merge_before_handler_head_elements() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let engine = test_engine(declarations);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("middleware"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects.add_meta_head_element(crate::response_finalizer::HeadElement {
				tag: "meta".to_owned(),
				attributes_known_safe: BTreeMap::from([(
					"name".to_owned(),
					"middleware".to_owned(),
				)]),
				..crate::response_finalizer::HeadElement::default()
			});
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("resource"), move |_| async move {
			let mut effects = ResponseEffects::default();
			effects.add_meta_head_element(crate::response_finalizer::HeadElement {
				tag: "meta".to_owned(),
				attributes_known_safe: BTreeMap::from([("name".to_owned(), "handler".to_owned())]),
				..crate::response_finalizer::HeadElement::default()
			});
			Ok(HandlerOutput::data(serde_json::json!("resource")).with_effects(effects))
		});

		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		assert_eq!(
			report
				.effects()
				.meta_head_elements()
				.iter()
				.map(|element| element.attributes_known_safe["name"].as_str())
				.collect::<Vec<_>>(),
			["middleware", "handler"]
		);
	}

	#[tokio::test]
	async fn view_ancestor_terminal_effect_suppresses_child_view_commit() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story"),
		));
		let engine = test_engine(declarations);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			let mut effects = crate::response_finalizer::ResponseEffects::default();
			effects
				.redirect(StatusCode::SEE_OTHER, "/login")
				.expect("relative redirect location is valid");
			Ok(HandlerOutput::data(serde_json::json!(null)).with_effects(effects))
		});
		handlers.insert(handler_id("story"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"story": true})))
		});

		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/123"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		assert_eq!(report.committed().len(), 1);
		assert_eq!(report.committed()[0].handler_id().as_str(), "root");
		assert_eq!(
			report.terminal_handler_id().map(HandlerId::as_str),
			Some("root")
		);
		assert_eq!(
			report
				.suppressed()
				.iter()
				.map(|suppressed| suppressed.handler_id().as_str())
				.collect::<Vec<_>>(),
			["story"]
		);
	}
}
