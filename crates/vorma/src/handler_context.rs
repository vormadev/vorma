//! Stateful runtime handler context adapter.

use std::collections::BTreeMap;
use std::future::Future;
use std::sync::Arc;

use bytes::Bytes;
use cookie::Cookie;
use http::StatusCode;
use http::header::{HeaderName, HeaderValue};
use serde_json::Value;
use vorma_tasks::ExecCtx;

use crate::asset_capabilities::PublicUrlError;
use crate::contracts::DocumentElementContract;
use crate::document_renderer::DocumentRenderError;
use crate::execution_engine::{
	HandlerExecutionError, HandlerFuture, HandlerInput, HandlerOutput, RuntimeHandler,
};
use crate::head::{
	description_element, icon_element, meta_charset_element, meta_name_content_element,
	meta_property_content_element, preload_element, prepare_head_element,
	prepare_static_head_element, title_element,
};
use crate::input_decoder::DecodedRouteInput;
use crate::response_finalizer::{ResponseEffects, ResponseEffectsError, accepts_client_redirect};

/// Build a runtime handler from a stateful context handler.
pub fn contextual_runtime_handler<S, F, Fut>(
	state: Arc<S>,
	handler: F,
) -> ContextualRuntimeHandler<S, F>
where
	S: Send + Sync + 'static,
	F: Fn(HandlerContext<S>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<HandlerOutput, HandlerExecutionError>> + Send + 'static,
{
	ContextualRuntimeHandler { state, handler }
}

/// Runtime handler adapter that injects shared application state.
pub struct ContextualRuntimeHandler<S, F> {
	state: Arc<S>,
	handler: F,
}

impl<S, F, Fut> RuntimeHandler for ContextualRuntimeHandler<S, F>
where
	S: Send + Sync + 'static,
	F: Fn(HandlerContext<S>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<HandlerOutput, HandlerExecutionError>> + Send + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let context = HandlerContext::new(Arc::clone(&self.state), input, exec_ctx);
		Box::pin((self.handler)(context))
	}
}

/// Context facts and mutable effects for one stateful handler invocation.
pub struct HandlerContext<S> {
	state: Arc<S>,
	input: HandlerInput,
	exec_ctx: ExecCtx<crate::Error>,
	effects: ResponseEffects,
}

impl<S> HandlerContext<S> {
	/// Create a handler context from committed runtime facts.
	pub fn new(state: Arc<S>, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> Self {
		Self {
			state,
			input,
			exec_ctx,
			effects: ResponseEffects::default(),
		}
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		&self.state
	}

	/// Original runtime handler input.
	pub fn handler_input(&self) -> &HandlerInput {
		&self.input
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<crate::Error> {
		&self.exec_ctx
	}

	/// Original request facts.
	pub fn request(&self) -> &crate::execution_engine::RequestInput {
		self.input.request()
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
	pub fn response(&mut self) -> ResponseHandle<'_> {
		ResponseHandle {
			effects: &mut self.effects,
			request_headers: self.input.request().headers(),
		}
	}

	/// Mutable document-head effect handle.
	pub fn head(&mut self) -> HeadHandle<'_> {
		HeadHandle {
			effects: &mut self.effects,
		}
	}

	/// Consume the context into an empty handler output.
	pub fn empty(self) -> HandlerOutput {
		HandlerOutput::empty().with_effects(self.effects)
	}

	/// Consume the context into a data handler output.
	pub fn data(self, data: Value) -> HandlerOutput {
		HandlerOutput::data(data).with_effects(self.effects)
	}

	/// Consume the context into a body handler output.
	pub fn body(self, body: Bytes) -> HandlerOutput {
		HandlerOutput::body(body).with_effects(self.effects)
	}
}

/// Mutable response-effect handle for stateful handlers.
pub struct ResponseHandle<'a> {
	effects: &'a mut ResponseEffects,
	request_headers: &'a http::HeaderMap,
}

impl ResponseHandle<'_> {
	/// Set the response status.
	pub fn set_status(&mut self, status: StatusCode) -> &mut Self {
		self.effects.set_status(status);
		self
	}

	/// Set an error response status with client-visible text.
	pub fn set_client_error(&mut self, status: StatusCode, text: impl Into<String>) -> &mut Self {
		self.effects.set_status_with_text(status, text);
		self
	}

	/// Set a response header, replacing prior values with the same name.
	pub fn set_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.effects.set_header(key, value);
		self
	}

	/// Append a response header value.
	pub fn append_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.effects.add_header(key, value);
		self
	}

	/// Set a response cookie.
	pub fn set_cookie(&mut self, cookie: Cookie<'static>) -> &mut Self {
		self.effects.set_cookie(cookie);
		self
	}

	/// Redirect using the default server redirect status.
	pub fn redirect(
		&mut self,
		location: impl Into<String>,
	) -> Result<&mut Self, ResponseEffectsError> {
		self.redirect_with_status(location, None)
	}

	/// Redirect using an explicit 3xx status when supplied.
	pub fn redirect_with_status(
		&mut self,
		location: impl Into<String>,
		status: Option<StatusCode>,
	) -> Result<&mut Self, ResponseEffectsError> {
		self.effects.redirect_with_client_preference(
			accepts_client_redirect(self.request_headers),
			location,
			status,
		)?;
		Ok(self)
	}
}

/// Mutable head-effect handle for stateful handlers.
pub struct HeadHandle<'a> {
	effects: &'a mut ResponseEffects,
}

impl HeadHandle<'_> {
	/// Set the document title.
	pub fn title(&mut self, title: impl Into<String>) -> &mut Self {
		self.apply_static(title_element(title))
	}

	/// Set the meta description.
	pub fn description(&mut self, description: impl Into<String>) -> &mut Self {
		self.apply_static(description_element(description))
	}

	/// Add a favicon link.
	pub fn icon(&mut self, href: impl Into<String>) -> &mut Self {
		self.apply_static(icon_element(href))
	}

	/// Add a preload link.
	pub fn preload(&mut self, href: impl Into<String>, r#as: impl Into<String>) -> &mut Self {
		self.apply_static(preload_element(href, r#as))
	}

	/// Add a `<meta name="..." content="...">` element.
	pub fn meta_name_content(
		&mut self,
		name: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.apply_static(meta_name_content_element(name, content))
	}

	/// Add a `<meta property="..." content="...">` element.
	pub fn meta_property_content(
		&mut self,
		property: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.apply_static(meta_property_content_element(property, content))
	}

	/// Add a charset meta element.
	pub fn meta_charset(&mut self, charset: impl Into<String>) -> &mut Self {
		self.apply_static(meta_charset_element(charset))
	}

	/// Add a low-level document element contract as a head effect.
	pub fn add(
		&mut self,
		element: DocumentElementContract,
	) -> Result<&mut Self, HandlerContextError> {
		let prepared = prepare_head_element(element)
			.map_err(|source| HandlerContextError::HeadElement { source })?;
		self.effects.apply_head_element(prepared);
		Ok(self)
	}

	fn apply_static(&mut self, element: DocumentElementContract) -> &mut Self {
		self.effects
			.apply_head_element(prepare_static_head_element(element));
		self
	}
}

/// Stateful handler context error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HandlerContextError {
	/// Head element rendering failed.
	HeadElement {
		/// Source rendering error.
		source: DocumentRenderError,
	},
}

impl std::fmt::Display for HandlerContextError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::HeadElement { source } => write!(f, "invalid handler head element: {source}"),
		}
	}
}

impl std::error::Error for HandlerContextError {
	fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
		match self {
			Self::HeadElement { source } => Some(source),
		}
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;
	use http::header::CONTENT_TYPE;

	use super::*;
	use crate::asset_capabilities::AssetCapabilities;
	use crate::execution_engine::{
		ExecutionEngine, HandlerRegistry, RequestExecutionReport, RequestInput,
	};
	use crate::execution_plan::ExecutionPlan;
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration,
	};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	#[tokio::test]
	async fn contextual_handler_commits_state_data_public_url_and_effects() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/items/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let engine = ExecutionEngine::new(
			plan,
			AssetCapabilities::new(
				"/static/",
				vec!["/static/app.hash.css".to_owned()],
				BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			)
			.unwrap(),
		);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(
			handler_id("resource"),
			contextual_runtime_handler(
				Arc::new("state-value".to_owned()),
				|mut context| async move {
					assert_eq!(context.state(), "state-value");
					context
						.response()
						.set_status(StatusCode::CREATED)
						.set_header(CONTENT_TYPE, HeaderValue::from_static("application/json"));
					context
						.head()
						.title("Title <escaped>")
						.description("Description <escaped>");
					let data = serde_json::json!({
						"id": context.param("id").unwrap(),
						"url": context.public_url(" /app.css ").unwrap(),
					});
					Ok(context.data(data))
				},
			),
		);

		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/items/123"), &handlers)
			.await
			.unwrap();

		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};
		let output = report.committed()[0].output();
		assert_eq!(
			output.data_value().unwrap(),
			&serde_json::json!({"id": "123", "url": "/static/app.hash.css"})
		);
		assert_eq!(output.effects().status(), Some(StatusCode::CREATED));
		assert_eq!(
			output.effects().title().unwrap().dangerous_inner_html,
			"Title &lt;escaped&gt;"
		);
		assert_eq!(
			output.effects().meta_head_elements()[0].attributes_known_safe["content"],
			"Description &lt;escaped&gt;"
		);
	}

	#[tokio::test]
	async fn head_handle_rejects_invalid_low_level_elements() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/bad-head",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let engine = ExecutionEngine::new(
			plan,
			AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap(),
		);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(
			handler_id("resource"),
			contextual_runtime_handler(Arc::new(()), |mut context| async move {
				let error = match context.head().add(DocumentElementContract::new("bad tag")) {
					Ok(_) => panic!("invalid head element should fail"),
					Err(error) => error,
				};
				assert!(matches!(error, HandlerContextError::HeadElement { .. }));
				Ok(context.empty())
			}),
		);

		engine
			.execute(RequestInput::new(Method::GET, "/api/bad-head"), &handlers)
			.await
			.unwrap();
	}
}
