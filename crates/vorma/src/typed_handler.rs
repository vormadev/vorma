//! Typed public-style handler adapter for the fresh runtime.

use std::collections::BTreeMap;
use std::future::Future;
use std::sync::{Arc, Mutex};

use cookie::Cookie;
use http::StatusCode;
use http::header::{HeaderName, HeaderValue};
use serde::Serialize;
use serde::de::DeserializeOwned;
use vorma_tasks::ExecCtx;

use crate::asset_capabilities::PublicUrlError;
use crate::contracts::DocumentElementContract;
use crate::document_renderer::DocumentRenderError;
use crate::execution_engine::{
	HandlerExecutionError, HandlerFuture, HandlerInput, HandlerOutput, RuntimeHandler,
};
use crate::form_data::{DECODED_FORM_DATA_MISSING_MESSAGE, FormData};
use crate::head::{
	description_element, icon_element, meta_charset_element, meta_name_content_element,
	meta_property_content_element, preload_element, prepare_head_element, title_element,
};
use crate::input_decoder::DecodedRouteInput;
use crate::resource_body::{ResourceOutput, resource_output_with_effects};
use crate::response_finalizer::{ResponseEffects, accepts_client_redirect, lock_effects};

/// Build a runtime handler from a typed public-style context handler.
pub fn typed_runtime_handler<S, I, O, F, Fut>(
	state: Arc<S>,
	handler: F,
) -> TypedRuntimeHandler<S, I, O, F>
where
	S: Send + Sync + 'static,
	I: DeserializeOwned + Send + Sync + 'static,
	O: Serialize + Send + Sync + 'static,
	F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	TypedRuntimeHandler {
		state,
		handler,
		_marker: std::marker::PhantomData,
	}
}

/// Build a runtime handler from a typed public-style `FormData` context handler.
pub fn form_data_runtime_handler<S, O, F, Fut>(
	state: Arc<S>,
	handler: F,
) -> FormDataRuntimeHandler<S, O, F>
where
	S: Send + Sync + 'static,
	O: ResourceOutput,
	F: Fn(TypedHandlerContext<S, FormData>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	FormDataRuntimeHandler {
		state,
		handler,
		_marker: std::marker::PhantomData,
	}
}

/// Build a runtime handler from a typed public-style resource context handler.
pub fn typed_resource_runtime_handler<S, I, O, F, Fut>(
	state: Arc<S>,
	handler: F,
) -> TypedResourceRuntimeHandler<S, I, O, F>
where
	S: Send + Sync + 'static,
	I: DeserializeOwned + Send + Sync + 'static,
	O: ResourceOutput,
	F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	TypedResourceRuntimeHandler {
		state,
		handler,
		_marker: std::marker::PhantomData,
	}
}

/// Runtime handler adapter that injects typed input and shared application state.
pub struct TypedRuntimeHandler<S, I, O, F> {
	state: Arc<S>,
	handler: F,
	_marker: std::marker::PhantomData<fn(I, O)>,
}

/// Runtime handler adapter for resource outputs.
pub struct TypedResourceRuntimeHandler<S, I, O, F> {
	state: Arc<S>,
	handler: F,
	_marker: std::marker::PhantomData<fn(I, O)>,
}

impl<S, I, O, F, Fut> RuntimeHandler for TypedRuntimeHandler<S, I, O, F>
where
	S: Send + Sync + 'static,
	I: DeserializeOwned + Send + Sync + 'static,
	O: Serialize + Send + Sync + 'static,
	F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let context = match TypedHandlerContext::try_new(Arc::clone(&self.state), input, exec_ctx) {
			Ok(context) => context,
			Err(error) => return Box::pin(async { Err(error) }),
		};
		let effects = Arc::clone(&context.effects);
		let future = (self.handler)(context);
		Box::pin(async move {
			let output = match future.await {
				Ok(output) => output,
				Err(error) => {
					return Err(handler_error_with_effects(
						HandlerExecutionError::from_application_error(&error),
						&effects,
					));
				}
			};
			handler_output_with_effects(output, &effects)
		})
	}
}

impl<S, I, O, F, Fut> RuntimeHandler for TypedResourceRuntimeHandler<S, I, O, F>
where
	S: Send + Sync + 'static,
	I: DeserializeOwned + Send + Sync + 'static,
	O: ResourceOutput,
	F: Fn(TypedHandlerContext<S, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let context = match TypedHandlerContext::try_new(Arc::clone(&self.state), input, exec_ctx) {
			Ok(context) => context,
			Err(error) => return Box::pin(async { Err(error) }),
		};
		let effects = Arc::clone(&context.effects);
		let future = (self.handler)(context);
		Box::pin(async move {
			let output = match future.await {
				Ok(output) => output,
				Err(error) => {
					return Err(handler_error_with_effects(
						HandlerExecutionError::from_application_error(&error),
						&effects,
					));
				}
			};
			resource_output_with_effects(output, &effects)
		})
	}
}

/// Runtime handler adapter for `FormData` resource inputs.
pub struct FormDataRuntimeHandler<S, O, F> {
	state: Arc<S>,
	handler: F,
	_marker: std::marker::PhantomData<fn(O)>,
}

impl<S, O, F, Fut> RuntimeHandler for FormDataRuntimeHandler<S, O, F>
where
	S: Send + Sync + 'static,
	O: ResourceOutput,
	F: Fn(TypedHandlerContext<S, FormData>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::Error>> + Send + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let form_data = match input.decoded_input().parsed_form_data().cloned() {
			Some(form_data) => form_data,
			None => {
				return Box::pin(async {
					Err(HandlerExecutionError::new(
						DECODED_FORM_DATA_MISSING_MESSAGE,
					))
				});
			}
		};
		let context = TypedHandlerContext::from_typed_input(
			Arc::clone(&self.state),
			input,
			form_data,
			exec_ctx,
		);
		let effects = Arc::clone(&context.effects);
		let future = (self.handler)(context);
		Box::pin(async move {
			let output = match future.await {
				Ok(output) => output,
				Err(error) => {
					return Err(handler_error_with_effects(
						HandlerExecutionError::from_application_error(&error),
						&effects,
					));
				}
			};
			resource_output_with_effects(output, &effects)
		})
	}
}

/// Typed handler context projected from committed runtime facts.
pub struct TypedHandlerContext<S, I> {
	state: Arc<S>,
	input: HandlerInput,
	typed_input: I,
	exec_ctx: ExecCtx<crate::Error>,
	effects: Arc<Mutex<ResponseEffects>>,
}

impl<S, I> TypedHandlerContext<S, I>
where
	I: DeserializeOwned,
{
	pub(crate) fn try_new(
		state: Arc<S>,
		input: HandlerInput,
		exec_ctx: ExecCtx<crate::Error>,
	) -> Result<Self, HandlerExecutionError> {
		let typed_input = input_value(input.decoded_input()).map_err(|error| {
			HandlerExecutionError::new(format!("deserialize route input: {error}"))
		})?;
		Ok(Self::from_typed_input(state, input, typed_input, exec_ctx))
	}
}

impl<S, I> TypedHandlerContext<S, I> {
	pub(crate) fn from_typed_input(
		state: Arc<S>,
		input: HandlerInput,
		typed_input: I,
		exec_ctx: ExecCtx<crate::Error>,
	) -> Self {
		Self {
			state,
			input,
			typed_input,
			exec_ctx,
			effects: Arc::new(Mutex::new(ResponseEffects::default())),
		}
	}
}

impl<S, I> TypedHandlerContext<S, I> {
	/// Shared application state.
	pub fn state(&self) -> &S {
		&self.state
	}

	/// Typed route input.
	pub fn input(&self) -> &I {
		&self.typed_input
	}

	/// Original runtime handler input.
	pub fn handler_input(&self) -> &HandlerInput {
		&self.input
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<crate::Error> {
		&self.exec_ctx
	}

	/// Shared mutable effects committed by this handler context.
	pub(crate) fn response_effects(&self) -> Arc<Mutex<ResponseEffects>> {
		Arc::clone(&self.effects)
	}

	/// Original request facts.
	pub fn request(&self) -> crate::HttpRequest<'_> {
		crate::HttpRequest::new(self.input.request())
	}

	/// Matched route pattern.
	pub fn pattern(&self) -> &str {
		self.input.pattern()
	}

	/// Captured string parameter.
	pub fn param(&self, key: &str) -> Option<&str> {
		self.input.params().get(key)
	}

	/// Captured splat values.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		self.input.splat_values()
	}

	/// Decoded query values keyed by query parameter name.
	pub fn query_params(&self) -> &BTreeMap<String, Vec<String>> {
		self.input.query_params()
	}

	/// Decoded route input.
	pub fn decoded_input(&self) -> &DecodedRouteInput {
		self.input.decoded_input()
	}

	/// Resolve a logical public source path to a committed public URL.
	pub fn public_url(&self, src_path: &str) -> Result<String, PublicUrlError> {
		self.input.public_url(src_path).map(ToOwned::to_owned)
	}

	/// Mutable response-effect handle.
	pub fn response(&self) -> TypedResponseHandle<'_> {
		TypedResponseHandle {
			effects: Arc::clone(&self.effects),
			_lifetime: std::marker::PhantomData,
		}
	}

	/// Mutable resource response-effect handle (success-status capable).
	pub fn resource_response(&self) -> TypedResourceResponseHandle<'_> {
		TypedResourceResponseHandle::new(self.response())
	}

	pub(crate) fn apply_redirect(
		&self,
		location: &str,
		status: Option<StatusCode>,
	) -> Result<(), String> {
		apply_handler_redirect(
			&self.effects,
			self.input.request().headers(),
			location,
			status,
		)
	}

	/// Mutable document-head effect handle.
	pub fn head(&self) -> TypedHeadHandle {
		TypedHeadHandle {
			effects: Arc::clone(&self.effects),
		}
	}
}

/// Mutable response-effect handle for typed handlers.
pub struct TypedResponseHandle<'a> {
	effects: Arc<Mutex<ResponseEffects>>,
	_lifetime: std::marker::PhantomData<&'a ()>,
}

impl TypedResponseHandle<'_> {
	/// Set a response header, replacing prior values with the same name.
	pub fn set_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		lock_effects(&self.effects).set_header(key, value);
		self
	}

	/// Append a response header value.
	pub fn append_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		lock_effects(&self.effects).add_header(key, value);
		self
	}

	/// Set a response cookie.
	pub fn set_cookie(&mut self, cookie: Cookie<'static>) -> &mut Self {
		lock_effects(&self.effects).set_cookie(cookie);
		self
	}
}

/// Mutable response-effect handle for resource handlers.
/*
Resources are real HTTP endpoints, so they may override the SUCCESS status
(201 Created and friends). Error statuses ride `HttpExit` and redirects ride
`ctx.redirect(...)` — each outcome has exactly one door, so this asserts
below the redirect range rather than merely below the error range.
*/
pub struct TypedResourceResponseHandle<'a> {
	inner: TypedResponseHandle<'a>,
}

impl<'a> TypedResourceResponseHandle<'a> {
	pub(crate) fn new(inner: TypedResponseHandle<'a>) -> Self {
		Self { inner }
	}

	/// Set the success response status (2xx/1xx only).
	pub fn set_status(&mut self, status: StatusCode) -> &mut Self {
		assert!(
			status.as_u16() < 300,
			"set_status takes success statuses only (redirects use ctx.redirect, \
			errors ride HttpExit); got {status}"
		);
		lock_effects(&self.inner.effects).set_status(status);
		self
	}

	/// Set a response header, replacing prior values with the same name.
	pub fn set_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.inner.set_header(key, value);
		self
	}

	/// Append a response header value.
	pub fn append_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.inner.append_header(key, value);
		self
	}

	/// Set a response cookie.
	pub fn set_cookie(&mut self, cookie: Cookie<'static>) -> &mut Self {
		self.inner.set_cookie(cookie);
		self
	}
}

/// Apply a redirect to handler response effects, honoring client preference.
pub(crate) fn apply_handler_redirect(
	effects: &Arc<Mutex<ResponseEffects>>,
	request_headers: &http::HeaderMap,
	location: &str,
	status: Option<StatusCode>,
) -> Result<(), String> {
	lock_effects(effects)
		.redirect_with_client_preference(accepts_client_redirect(request_headers), location, status)
		.map(|_| ())
		.map_err(|source| match source {
			crate::response_finalizer::ResponseEffectsError::InvalidRedirectLocation {
				location,
			} => format!("invalid URL: {}", location),
			crate::response_finalizer::ResponseEffectsError::InvalidRedirectStatus { status } => {
				format!("redirect status must be 3xx, got {status}")
			}
		})
}

/// Mutable head-effect handle for typed handlers.
pub struct TypedHeadHandle {
	effects: Arc<Mutex<ResponseEffects>>,
}

impl TypedHeadHandle {
	/// Set the document title.
	pub fn title(&mut self, title: impl Into<String>) -> &mut Self {
		self.add_static_contract(title_element(title))
	}

	/// Set the meta description.
	pub fn description(&mut self, description: impl Into<String>) -> &mut Self {
		self.add_static_contract(description_element(description))
	}

	/// Add a favicon link.
	pub fn icon(&mut self, href: impl Into<String>) -> &mut Self {
		self.add_static_contract(icon_element(href))
	}

	/// Add a preload link.
	pub fn preload(&mut self, href: impl Into<String>, r#as: impl Into<String>) -> &mut Self {
		self.add_static_contract(preload_element(href, r#as))
	}

	/// Add a `<meta name="..." content="...">` element.
	pub fn meta_name_content(
		&mut self,
		name: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.add_static_contract(meta_name_content_element(name, content))
	}

	/// Add a `<meta property="..." content="...">` element.
	pub fn meta_property_content(
		&mut self,
		property: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.add_static_contract(meta_property_content_element(property, content))
	}

	/// Add a charset meta element.
	pub fn meta_charset(&mut self, charset: impl Into<String>) -> &mut Self {
		self.add_static_contract(meta_charset_element(charset))
	}

	/// Add a low-level document element contract as a head effect.
	pub fn add(
		&mut self,
		defs: impl IntoIterator<Item = crate::HtmlElementDef>,
	) -> Result<&mut Self, String> {
		let element = crate::head::element_contract_from_defs(defs).map_err(|message| {
			TypedHandlerContextError::HeadElementDefinition { message }.to_string()
		})?;
		self.add_contract(element)
			.map_err(|source| source.to_string())
	}

	/// Append another head builder's elements.
	pub fn append(&mut self, other: &crate::HeadBuilder) -> &mut Self {
		for element in other.elements() {
			self.add_static_contract(element.clone());
		}
		self
	}

	fn add_contract(
		&mut self,
		element: DocumentElementContract,
	) -> Result<&mut Self, TypedHandlerContextError> {
		let prepared = prepare_head_element(element)
			.map_err(|source| TypedHandlerContextError::HeadElement { source })?;
		lock_effects(&self.effects).apply_head_element(prepared);
		Ok(self)
	}

	fn add_static_contract(&mut self, element: DocumentElementContract) -> &mut Self {
		self.add_contract(element)
			.expect("static typed head element contract is valid");
		self
	}
}

/// Typed handler context error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TypedHandlerContextError {
	/// Head element rendering failed.
	HeadElement {
		/// Source rendering error.
		source: DocumentRenderError,
	},
	/// Low-level head definition could not become an element contract.
	HeadElementDefinition {
		/// Definition error message.
		message: String,
	},
}

impl std::fmt::Display for TypedHandlerContextError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::HeadElement { source } => {
				write!(f, "invalid typed handler head element: {source}")
			}
			Self::HeadElementDefinition { message } => {
				write!(f, "invalid typed handler head element: {message}")
			}
		}
	}
}

impl std::error::Error for TypedHandlerContextError {
	fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
		match self {
			Self::HeadElement { source } => Some(source),
			Self::HeadElementDefinition { .. } => None,
		}
	}
}

fn input_value<I>(decoded_input: &DecodedRouteInput) -> Result<I, serde_json::Error>
where
	I: DeserializeOwned,
{
	serde_json::from_value(decoded_input.value().clone())
}

pub(crate) fn handler_output_with_effects<O>(
	output: O,
	effects: &Arc<Mutex<ResponseEffects>>,
) -> Result<HandlerOutput, HandlerExecutionError>
where
	O: Serialize,
{
	let data = serde_json::to_value(output)
		.map_err(|error| HandlerExecutionError::new(format!("serialize route output: {error}")))?;
	let effects = lock_effects(effects).clone();
	Ok(HandlerOutput::data(data).with_effects(effects))
}

fn handler_error_with_effects(
	error: HandlerExecutionError,
	effects: &Arc<Mutex<ResponseEffects>>,
) -> HandlerExecutionError {
	let effects = lock_effects(effects).clone();
	error.with_effects(effects)
}

#[cfg(test)]
mod tests {
	use std::sync::Arc;

	use bytes::Bytes;
	use http::header::CONTENT_TYPE;
	use http::header::{HeaderName, HeaderValue};
	use http::{Method, StatusCode};
	use serde::{Deserialize, Serialize};

	use super::*;
	use crate::contracts::{FieldDef, RouteTypeContract, TypeDef, TypeRefContract};
	use crate::execution_engine::{HandlerRegistry, RequestExecutionReport, RequestInput};
	use crate::execution_plan::ExecutionPlan;
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration, ViewDeclaration,
	};
	use crate::runtime_manifest::{RuntimeManifest, RuntimeViewModule};

	const TEST_HEADER: &str = "x-typed-handler";
	const TEST_TYPE_KEY: &str = "TypedInput";
	const TEST_TYPE_NAME: &str = "TypedInput";

	#[derive(Clone, Debug, Deserialize)]
	struct TypedInput {
		name: String,
	}

	#[derive(Clone, Debug, Serialize)]
	struct TypedOutput {
		message: String,
	}

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn type_ref() -> TypeRefContract {
		TypeRefContract::Named {
			key: TEST_TYPE_KEY.to_owned(),
			name: TEST_TYPE_NAME.to_owned(),
		}
	}

	fn type_def() -> TypeDef {
		TypeDef::Record {
			key: TEST_TYPE_KEY.to_owned(),
			name: TEST_TYPE_NAME.to_owned(),
			fields: vec![FieldDef::new("name", TypeRefContract::String, false)],
		}
	}

	fn route_type_contract() -> RouteTypeContract {
		RouteTypeContract::new(type_ref(), TypeRefContract::Unknown)
	}

	fn graph() -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(type_def());
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/hello",
			None,
			Some(serde_json::json!({"type": "object"})),
			route_type_contract(),
			handler_id("hello"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({"type": "object"}),
			route_type_contract(),
			handler_id("root"),
		));
		FrameworkGraph::compile(declarations).unwrap()
	}

	fn runtime_snapshot() -> crate::runtime_snapshot::RuntimeSnapshot {
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({"type": "object"}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		);
		crate::runtime_snapshot::RuntimeSnapshot::compile(
			crate::runtime_snapshot::RuntimeSnapshotInput::new(graph(), manifest),
		)
		.unwrap()
	}

	#[tokio::test]
	async fn typed_runtime_handler_deserializes_input_serializes_output_and_commits_effects() {
		let state = Arc::new("hello ".to_owned());
		let handler = typed_runtime_handler(
			Arc::clone(&state),
			|ctx: TypedHandlerContext<String, TypedInput>| async move {
				ctx.resource_response()
					.set_status(StatusCode::CREATED)
					.set_header(
						HeaderName::from_static(TEST_HEADER),
						HeaderValue::from_static("1"),
					);
				Ok::<_, crate::Error>(TypedOutput {
					message: format!("{}{}", ctx.state(), ctx.input().name),
				})
			},
		);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("hello"), handler);
		let plan = ExecutionPlan::compile(&graph()).unwrap();
		let assets = crate::asset_capabilities::AssetCapabilities::new(
			"/static/",
			Vec::new(),
			BTreeMap::new(),
		)
		.unwrap();
		let engine = crate::execution_engine::ExecutionEngine::new(plan, assets);
		let report = engine
			.execute(
				RequestInput::new(Method::POST, "/api/hello")
					.with_body(Bytes::from_static(br#"{"name":"Ada"}"#)),
				&handlers,
			)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		assert_eq!(
			report.committed()[0].output().data_value().unwrap()["message"],
			"hello Ada"
		);
		assert_eq!(report.effects().status(), Some(StatusCode::CREATED));
	}

	#[tokio::test]
	async fn typed_runtime_handler_commits_view_head_effects() {
		let handler = typed_runtime_handler(
			Arc::new(()),
			|ctx: TypedHandlerContext<(), TypedInput>| async move {
				ctx.head().title(format!("Hello {}", ctx.input().name));
				Ok::<_, crate::Error>(TypedOutput {
					message: ctx.input().name.clone(),
				})
			},
		);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("root"), handler);
		let report = runtime_snapshot()
			.execute(
				RequestInput::new(Method::GET, "/").with_query("name=Ada"),
				&handlers,
			)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		assert_eq!(
			report.committed()[0].output().data_value().unwrap()["message"],
			"Ada"
		);
		assert_eq!(
			report.effects().title().unwrap().dangerous_inner_html,
			"Hello Ada"
		);
	}

	#[tokio::test]
	async fn typed_runtime_handler_allows_overlapping_response_and_head_handles() {
		let handler = typed_runtime_handler(
			Arc::new(()),
			|ctx: TypedHandlerContext<(), TypedInput>| async move {
				let mut response = ctx.resource_response();
				let mut head = ctx.head();
				response.set_status(StatusCode::CREATED).set_header(
					HeaderName::from_static(TEST_HEADER),
					HeaderValue::from_static("1"),
				);
				head.title(format!("Hello {}", ctx.input().name));
				Ok::<_, crate::Error>(TypedOutput {
					message: ctx.input().name.clone(),
				})
			},
		);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("root"), handler);
		let report = tokio::time::timeout(
			std::time::Duration::from_secs(1),
			runtime_snapshot().execute(
				RequestInput::new(Method::GET, "/").with_query("name=Ada"),
				&handlers,
			),
		)
		.await
		.unwrap()
		.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		assert_eq!(report.effects().status(), Some(StatusCode::CREATED));
		assert_eq!(
			report.effects().title().unwrap().dangerous_inner_html,
			"Hello Ada"
		);
	}

	#[tokio::test]
	async fn typed_runtime_handler_errors_never_leak_to_the_client() {
		let handler = typed_runtime_handler(
			Arc::new(()),
			|_ctx: TypedHandlerContext<(), TypedInput>| async move {
				Err::<TypedOutput, _>(crate::Error::new("server-side reason stays in logs"))
			},
		);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("root"), handler);
		let report = runtime_snapshot()
			.execute(
				RequestInput::new(Method::GET, "/").with_query("name=Ada"),
				&handlers,
			)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		/*
		Facade-typed handlers are framework-internal: they have NO client
		channel, so the wire always carries the generic message regardless
		of the server-side record.
		*/
		assert_eq!(
			report.server_error().unwrap().client_message(),
			"An unexpected error occurred."
		);
	}

	#[tokio::test]
	async fn form_data_runtime_handler_receives_decoded_form_data_input() {
		let handler = form_data_runtime_handler(
			Arc::new(()),
			|ctx: TypedHandlerContext<(), FormData>| async move {
				Ok::<_, crate::Error>(TypedOutput {
					message: ctx.input().text("name").unwrap_or("").to_owned(),
				})
			},
		);
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/form",
			None,
			None,
			RouteTypeContract::new(TypeRefContract::FormData, TypeRefContract::Unknown),
			handler_id("form"),
		));
		let plan = ExecutionPlan::compile(&FrameworkGraph::compile(declarations).unwrap()).unwrap();
		let assets = crate::asset_capabilities::AssetCapabilities::new(
			"/static/",
			Vec::new(),
			BTreeMap::new(),
		)
		.unwrap();
		let engine = crate::execution_engine::ExecutionEngine::new(plan, assets);
		let mut handlers = HandlerRegistry::default();
		handlers.insert(handler_id("form"), handler);
		let mut headers = http::HeaderMap::new();
		headers.insert(
			CONTENT_TYPE,
			HeaderValue::from_static("application/x-www-form-urlencoded"),
		);

		let report = engine
			.execute(
				RequestInput::new(Method::POST, "/api/form")
					.with_headers(headers)
					.with_body(Bytes::from_static(b"name=Ada")),
				&handlers,
			)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		assert_eq!(
			report.committed()[0].output().data_value().unwrap()["message"],
			"Ada"
		);
	}
}
