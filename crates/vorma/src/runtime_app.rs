//! Committed runtime application request boundary.
//!
//! First serving layer over the contract: pairs a `RuntimeSnapshot` with the
//! `HandlerRegistry` and turns one decoded request into one response via the
//! execution engine. HTTP transport, body limits, and asset serving live a
//! layer up in `runtime_service`.

use bytes::Bytes;
use futures_util::future;
use http::header::{ALLOW, CACHE_CONTROL, CONTENT_LENGTH, CONTENT_TYPE, HeaderValue};
use http::{Method, Request, Response, StatusCode};
use percent_encoding::percent_decode_str;
use url::form_urlencoded;

use crate::asset_body_provider::{PublicAssetBodyError, PublicAssetBodyProvider};
use crate::asset_capabilities::PublicAsset;
use crate::contracts::DocumentContract;
use crate::execution_engine::{
	ExecutionError, HandlerRegistry, RequestExecutionReport, RequestInput, RequestTarget,
};
use crate::resource_response::{ResourceResponseError, finalize_resource_report};
use crate::response_finalizer::{
	FinalizerError, INTERNAL_SERVER_ERROR_STATUS_TEXT, JSON_CONTENT_TYPE, ResponseEffects,
	TEXT_CONTENT_TYPE, VORMA_JSON_QUERY_KEY, finalize_response, finalize_view_build_skew_response,
	suppress_response_body_preserving_content_length,
};
use crate::runtime_document::{RuntimeDocumentInput, RuntimeDocumentProvider};
use crate::runtime_snapshot::RuntimeSnapshot;
use crate::view_response::{
	ViewHtmlResponseInput, ViewResponseError, finalize_view_report_html_response,
	finalize_view_report_json_response,
};

pub(crate) const BAD_REQUEST_BODY: &[u8] = b"Bad Request\n";
pub(crate) const INTERNAL_SERVER_ERROR_BODY: &[u8] = b"Internal Server Error\n";
pub(crate) const METHOD_NOT_ALLOWED_BODY: &[u8] = b"Method Not Allowed\n";

/// Runtime request input plus trusted HTML rendering input.
#[derive(Clone, Debug)]
pub struct RuntimeAppRequest<'a> {
	request: RequestInput,
	view_html: ViewHtmlResponseInput<'a>,
}

impl<'a> RuntimeAppRequest<'a> {
	/// Create a runtime app request with empty HTML body markup.
	pub fn new(request: RequestInput) -> Self {
		Self {
			request,
			view_html: ViewHtmlResponseInput::new(""),
		}
	}

	/// Replace trusted HTML rendering input.
	pub fn with_view_html_input(mut self, view_html: ViewHtmlResponseInput<'a>) -> Self {
		self.view_html = view_html;
		self
	}

	/// Create a runtime app request from an HTTP request with a percent-decoded path.
	pub fn try_from_http_request(request: Request<Bytes>) -> Result<Self, RuntimeAppRequestError> {
		let (parts, body) = request.into_parts();
		let method = parts.method;
		let decoded_path = percent_decode_str(parts.uri.path())
			.decode_utf8()
			.map_err(|_| RuntimeAppRequestError::InvalidRequestPathUtf8 {
				method: method.clone(),
				path: parts.uri.path().to_owned(),
			})?
			.into_owned();
		let request = RequestInput::from_http_uri(method, parts.uri, decoded_path)
			.with_headers(parts.headers)
			.with_extensions(parts.extensions)
			.with_body(body);
		Ok(Self::new(request))
	}

	/// Request execution input.
	pub fn request(&self) -> &RequestInput {
		&self.request
	}

	/// Trusted HTML rendering input.
	pub fn view_html(&self) -> ViewHtmlResponseInput<'a> {
		self.view_html.clone()
	}
}

/// Runtime app request conversion error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeAppRequestError {
	/// Request path was not valid UTF-8 after percent decoding.
	InvalidRequestPathUtf8 {
		/// Rejected request method.
		method: Method,
		/// Rejected raw path.
		path: String,
	},
}

/// Committed runtime application.
pub struct CommittedRuntimeApp {
	snapshot: RuntimeSnapshot,
	handlers: HandlerRegistry,
}

impl CommittedRuntimeApp {
	/// Create a committed runtime application from one snapshot and matching handlers.
	pub fn new(snapshot: RuntimeSnapshot, handlers: HandlerRegistry) -> Self {
		Self { snapshot, handlers }
	}

	/// Committed runtime snapshot.
	pub fn snapshot(&self) -> &RuntimeSnapshot {
		&self.snapshot
	}

	/// Runtime handlers bound to this app.
	pub fn handlers(&self) -> &HandlerRegistry {
		&self.handlers
	}

	/// Handle one request through the committed runtime application.
	pub async fn handle_request<P>(
		&self,
		request: RuntimeAppRequest<'_>,
		asset_provider: &P,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
	{
		let json_view_build_id = json_view_client_build_id(request.request().query());
		let target = self
			.snapshot
			.engine()
			.classify(request.request().method(), request.request().path());
		if let Some(response) =
			self.view_build_skew_response(&request, &target, &json_view_build_id)?
		{
			return Ok(response);
		}

		let execution_report = match self.execute_request(request.request().clone()).await {
			Ok(report) => report,
			Err(error) => {
				let wire = matches!(target, RequestTarget::Resource(_));
				return self.execution_error_response(error, request.request().method(), wire);
			}
		};
		self.finalize_execution_report(
			execution_report,
			&request,
			json_view_build_id.is_some(),
			self.snapshot.graph().document(),
			asset_provider,
		)
		.await
	}

	/// Handle one request with a dynamic document provider for matched view responses.
	pub async fn handle_request_with_document_provider<P, D>(
		&self,
		request: RuntimeAppRequest<'_>,
		asset_provider: &P,
		document_provider: &D,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
		D: RuntimeDocumentProvider + ?Sized,
	{
		let json_view_build_id = json_view_client_build_id(request.request().query());
		let target = self
			.snapshot
			.engine()
			.classify(request.request().method(), request.request().path());
		if let Some(response) =
			self.view_build_skew_response(&request, &target, &json_view_build_id)?
		{
			return Ok(response);
		}
		if !matches!(target, RequestTarget::View(_)) {
			return self.handle_request(request, asset_provider).await;
		}

		let document_input = RuntimeDocumentInput::new(
			request.request().clone(),
			self.snapshot.manifest().clone(),
			self.snapshot.asset_capabilities().clone(),
		);
		let document_future = document_provider.document_for_request(document_input);
		let execution_future = self
			.snapshot
			.execute(request.request().clone(), &self.handlers);
		let (document, execution_report) = match future::try_join(
			async {
				document_future
					.await
					.map_err(|_| ConcurrentViewPreparationError::Document)
			},
			async {
				execution_future
					.await
					.map_err(ConcurrentViewPreparationError::Execution)
			},
		)
		.await
		{
			Ok(result) => result,
			Err(ConcurrentViewPreparationError::Document) => {
				return finalize_internal_server_error_response(
					self.snapshot.client_build_id(),
					request.request().method() == Method::HEAD,
					false,
				);
			}
			Err(ConcurrentViewPreparationError::Execution(error)) => {
				/*
				This is the document/view path: framework faults here render
				as plain responses, not resource envelopes.
				*/
				return self.execution_error_response(error, request.request().method(), false);
			}
		};
		self.finalize_execution_report(
			execution_report,
			&request,
			json_view_build_id.is_some(),
			&document,
			asset_provider,
		)
		.await
	}

	/// Handle one HTTP request through the committed runtime application.
	pub async fn handle_http_request<P>(
		&self,
		request: Request<Bytes>,
		view_html: ViewHtmlResponseInput<'_>,
		asset_provider: &P,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
	{
		let request = match RuntimeAppRequest::try_from_http_request(request) {
			Ok(request) => request.with_view_html_input(view_html),
			Err(RuntimeAppRequestError::InvalidRequestPathUtf8 { method, .. }) => {
				return suppress_head_response_if_needed(bad_request_response(), &method);
			}
		};
		self.handle_request(request, asset_provider).await
	}

	/// Handle one HTTP request with a dynamic document provider for matched view responses.
	pub async fn handle_http_request_with_document_provider<P, D>(
		&self,
		request: Request<Bytes>,
		view_html: ViewHtmlResponseInput<'_>,
		asset_provider: &P,
		document_provider: &D,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
		D: RuntimeDocumentProvider + ?Sized,
	{
		let request = match RuntimeAppRequest::try_from_http_request(request) {
			Ok(request) => request.with_view_html_input(view_html),
			Err(RuntimeAppRequestError::InvalidRequestPathUtf8 { method, .. }) => {
				return suppress_head_response_if_needed(bad_request_response(), &method);
			}
		};
		self.handle_request_with_document_provider(request, asset_provider, document_provider)
			.await
	}

	fn view_build_skew_response(
		&self,
		request: &RuntimeAppRequest<'_>,
		target: &RequestTarget,
		json_view_build_id: &Option<String>,
	) -> Result<Option<Response<Bytes>>, RuntimeAppError> {
		if json_view_build_id.is_some()
			&& matches!(target, RequestTarget::View(_))
			&& let Some(response) = finalize_view_build_skew_response(
				json_view_build_id.as_deref(),
				self.snapshot.client_build_id(),
			)
			.map_err(|source| RuntimeAppError::Finalizer { source })?
		{
			return suppress_head_response_if_needed(response, request.request().method())
				.map(Some);
		}
		Ok(None)
	}

	async fn execute_request(
		&self,
		request: RequestInput,
	) -> Result<RequestExecutionReport, ExecutionError> {
		self.snapshot.execute(request, &self.handlers).await
	}

	fn execution_error_response(
		&self,
		error: ExecutionError,
		method: &Method,
		wire: bool,
	) -> Result<Response<Bytes>, RuntimeAppError> {
		match error {
			ExecutionError::InputFailed { .. } => finalize_bad_request_response(
				self.snapshot.client_build_id(),
				*method == Method::HEAD,
				wire,
			),
			ExecutionError::MissingHandler { .. } => finalize_internal_server_error_response(
				self.snapshot.client_build_id(),
				*method == Method::HEAD,
				wire,
			),
		}
	}

	async fn finalize_execution_report<P>(
		&self,
		execution_report: RequestExecutionReport,
		request: &RuntimeAppRequest<'_>,
		json_view_requested: bool,
		document: &DocumentContract,
		asset_provider: &P,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
	{
		match execution_report {
			RequestExecutionReport::PublicAsset(asset) => {
				self.finalize_public_asset_response(asset, request.request(), asset_provider)
					.await
			}
			RequestExecutionReport::Resource(report) => {
				finalize_resource_report(&report, self.snapshot.client_build_id())
					.map_err(|source| RuntimeAppError::Resource { source })
			}
			RequestExecutionReport::MethodNotAllowed(methods) => {
				finalize_method_not_allowed_response(
					&methods,
					self.snapshot.client_build_id(),
					request.request().method(),
				)
			}
			RequestExecutionReport::View(report) => {
				if json_view_requested {
					finalize_view_report_json_response(
						self.snapshot.manifest(),
						document,
						&report,
						self.snapshot.client_build_id(),
					)
					.map_err(|source| RuntimeAppError::View { source })
				} else {
					let view_html = request.view_html();
					finalize_view_report_html_response(
						self.snapshot.manifest(),
						document,
						&report,
						&view_html,
						self.snapshot.client_build_id(),
					)
					.map_err(|source| RuntimeAppError::View { source })
				}
			}
			RequestExecutionReport::NotFound => finalize_framework_status_response(
				StatusCode::NOT_FOUND,
				self.snapshot.client_build_id(),
			),
		}
	}

	async fn finalize_public_asset_response<P>(
		&self,
		asset: PublicAsset,
		request: &RequestInput,
		asset_provider: &P,
	) -> Result<Response<Bytes>, RuntimeAppError>
	where
		P: PublicAssetBodyProvider,
	{
		let loaded = asset_provider
			.body_for_asset(asset.clone())
			.await
			.map_err(|source| RuntimeAppError::PublicAsset { source })?;
		let full_length = loaded.body().len();
		let body = if request.method() == Method::HEAD {
			Bytes::new()
		} else {
			loaded.body().clone()
		};
		let mut response = Response::new(body);
		let cache_control = HeaderValue::from_str(asset.cache_control()).map_err(|_| {
			RuntimeAppError::InvalidPublicAssetCacheControl {
				value: asset.cache_control().to_owned(),
			}
		})?;
		let content_length = full_length.to_string();
		let content_length = HeaderValue::from_str(&content_length).map_err(|_| {
			RuntimeAppError::InvalidContentLengthHeader {
				value: content_length.clone(),
			}
		})?;
		response.headers_mut().insert(CACHE_CONTROL, cache_control);
		response
			.headers_mut()
			.insert(CONTENT_LENGTH, content_length);
		if let Some(content_type) = loaded.content_type() {
			response
				.headers_mut()
				.insert(CONTENT_TYPE, content_type.clone());
		}
		Ok(response)
	}
}

/// Runtime app request handling error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeAppError {
	/// Runtime execution failed.
	Execution {
		/// Source execution error.
		source: ExecutionError,
	},
	/// Public asset body loading failed.
	PublicAsset {
		/// Source asset body loading error.
		source: PublicAssetBodyError,
	},
	/// Resource response finalization failed.
	Resource {
		/// Source resource response error.
		source: ResourceResponseError,
	},
	/// View response finalization failed.
	View {
		/// Source view response error.
		source: ViewResponseError,
	},
	/// Generic response finalization failed.
	Finalizer {
		/// Source finalizer error.
		source: FinalizerError,
	},
	/// Method list could not be encoded into an `Allow` header.
	InvalidAllowHeader {
		/// Rejected header value.
		value: String,
	},
	/// Public asset cache-control policy could not be encoded into a header.
	InvalidPublicAssetCacheControl {
		/// Rejected header value.
		value: String,
	},
	/// Public asset content length could not be encoded into a header.
	InvalidContentLengthHeader {
		/// Rejected header value.
		value: String,
	},
}

enum ConcurrentViewPreparationError {
	Document,
	Execution(ExecutionError),
}

fn json_view_client_build_id(query: Option<&str>) -> Option<String> {
	for (key, value) in form_urlencoded::parse(query.unwrap_or_default().as_bytes()) {
		if key == VORMA_JSON_QUERY_KEY && !value.is_empty() {
			return Some(value.into_owned());
		}
	}
	None
}

fn finalize_method_not_allowed_response(
	methods: &[Method],
	client_build_id: &str,
	method: &Method,
) -> Result<Response<Bytes>, RuntimeAppError> {
	let mut effects = ResponseEffects::default();
	effects.set_status(StatusCode::METHOD_NOT_ALLOWED);
	effects.set_header(ALLOW, allow_header_value(methods)?);
	let mut response = finalize_response(
		Bytes::from_static(METHOD_NOT_ALLOWED_BODY),
		&effects,
		client_build_id,
	)
	.map_err(|source| RuntimeAppError::Finalizer { source })?;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(TEXT_CONTENT_TYPE));
	suppress_head_response_if_needed(response, method)
}

fn allow_header_value(methods: &[Method]) -> Result<HeaderValue, RuntimeAppError> {
	let allow = methods
		.iter()
		.map(Method::as_str)
		.collect::<Vec<_>>()
		.join(", ");
	HeaderValue::from_str(&allow).map_err(|_| RuntimeAppError::InvalidAllowHeader {
		value: allow.clone(),
	})
}

fn finalize_framework_status_response(
	status: StatusCode,
	client_build_id: &str,
) -> Result<Response<Bytes>, RuntimeAppError> {
	let mut effects = ResponseEffects::default();
	effects.set_status(status);
	finalize_response(Bytes::new(), &effects, client_build_id)
		.map_err(|source| RuntimeAppError::Finalizer { source })
}

fn bad_request_response() -> Response<Bytes> {
	plain_text_response(StatusCode::BAD_REQUEST, BAD_REQUEST_BODY)
}

fn plain_text_response(status: StatusCode, body: &'static [u8]) -> Response<Bytes> {
	let mut response = Response::new(Bytes::from_static(body));
	*response.status_mut() = status;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(TEXT_CONTENT_TYPE));
	response
}

fn finalize_bad_request_response(
	client_build_id: &str,
	suppress_body: bool,
	wire: bool,
) -> Result<Response<Bytes>, RuntimeAppError> {
	finalize_framework_fault_response(
		client_build_id,
		suppress_body,
		wire,
		StatusCode::BAD_REQUEST,
		BAD_REQUEST_BODY,
		"Bad Request",
	)
}

fn finalize_internal_server_error_response(
	client_build_id: &str,
	suppress_body: bool,
	wire: bool,
) -> Result<Response<Bytes>, RuntimeAppError> {
	finalize_framework_fault_response(
		client_build_id,
		suppress_body,
		wire,
		StatusCode::INTERNAL_SERVER_ERROR,
		INTERNAL_SERVER_ERROR_BODY,
		INTERNAL_SERVER_ERROR_STATUS_TEXT,
	)
}

/*
Framework faults are generic by design (nothing app-authored exists to
show). Wire requests (resources) get the JSON error envelope the TS client
parses; document requests get the plain text body.
*/
fn finalize_framework_fault_response(
	client_build_id: &str,
	suppress_body: bool,
	wire: bool,
	status: StatusCode,
	text_body: &'static [u8],
	generic_text: &str,
) -> Result<Response<Bytes>, RuntimeAppError> {
	let mut effects = ResponseEffects::default();
	effects.set_status_with_text(status, generic_text);
	let (body, content_type) = if wire {
		let envelope = serde_json::json!({ "error": generic_text });
		(
			Bytes::from(
				serde_json::to_vec(&envelope)
					.expect("framework fault envelope serialization cannot fail"),
			),
			JSON_CONTENT_TYPE,
		)
	} else {
		(Bytes::from_static(text_body), TEXT_CONTENT_TYPE)
	};
	let response = finalize_response(body, &effects, client_build_id)
		.map_err(|source| RuntimeAppError::Finalizer { source })?;
	let mut response = response;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(content_type));
	if suppress_body {
		return suppress_head_response_if_needed(response, &Method::HEAD);
	}
	Ok(response)
}

fn suppress_head_response_if_needed(
	response: Response<Bytes>,
	method: &Method,
) -> Result<Response<Bytes>, RuntimeAppError> {
	if *method != Method::HEAD {
		return Ok(response);
	}
	suppress_response_body_preserving_content_length(response)
		.map_err(|source| RuntimeAppError::Finalizer { source })
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::sync::atomic::{AtomicUsize, Ordering};
	use std::sync::{Arc, Mutex};
	use std::time::Duration;

	use http::header::{CACHE_CONTROL, CONTENT_LENGTH, CONTENT_TYPE, LOCATION};
	use http::{HeaderMap, HeaderName};
	use tokio::sync::{Notify, oneshot};
	use tokio::time::timeout;

	use super::*;
	use crate::asset_body_provider::PublicAssetBody;
	use crate::constants::VORMA_DATA_JSON_SCRIPT_EL_ID;
	use crate::contracts::{
		DocumentContract, DocumentElementContract, FieldDef, RouteTypeContract, TypeDef,
		TypeRefContract,
	};
	use crate::execution_engine::{HandlerExecutionError, HandlerInput, HandlerOutput};
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, MiddlewareDeclaration,
		ResourceDeclaration, StaticAssetDeclaration, ViewDeclaration,
	};
	use crate::response_finalizer::{
		BUILD_SKEW_HEADER, BUILD_SKEW_RESPONSE_BODY, CLIENT_ACCEPTS_REDIRECT_HEADER,
		CLIENT_BUILD_ID_HEADER, CLIENT_REDIRECT_HEADER, JSON_CONTENT_TYPE, ResponseEffects,
		VORMA_JSON_QUERY_KEY, accepts_client_redirect,
	};
	use crate::runtime_manifest::{RuntimeManifest, RuntimeViewModule};
	use crate::runtime_snapshot::RuntimeSnapshotInput;
	use crate::test_support::route_type_contract;

	#[derive(Clone)]
	struct RequestMarker(&'static str);

	const TEST_CRITICAL_CSS: &str = "body{color:black}";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn runtime_app(calls: Arc<AtomicUsize>) -> CommittedRuntimeApp {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		declarations.add_static_asset(StaticAssetDeclaration::new(
			"app.css",
			"/static/app.hash.css",
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			TEST_CRITICAL_CSS,
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec![
				"/static/app.hash.css".to_owned(),
				"/static/root.js".to_owned(),
			],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
		});
		handlers.insert(handler_id("view"), move |_| {
			let calls = Arc::clone(&calls);
			async move {
				calls.fetch_add(1, Ordering::SeqCst);
				let mut effects = ResponseEffects::default();
				effects.set_status(StatusCode::OK);
				Ok(HandlerOutput::data(serde_json::json!({"view": true})).with_effects(effects))
			}
		});
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn runtime_app_with_typed_resource(calls: Arc<AtomicUsize>) -> CommittedRuntimeApp {
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
				TypeRefContract::Named {
					key: "SearchInput".to_owned(),
					name: "SearchInput".to_owned(),
				},
				TypeRefContract::Unknown,
			),
			handler_id("typed.resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("typed.resource"), move |input: HandlerInput| {
			let calls = Arc::clone(&calls);
			async move {
				calls.fetch_add(1, Ordering::SeqCst);
				Ok(HandlerOutput::body(Bytes::from(
					input.decoded_input().value().to_string(),
				)))
			}
		});
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn runtime_app_with_form_data_resource(calls: Arc<AtomicUsize>) -> CommittedRuntimeApp {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/form",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(TypeRefContract::FormData, TypeRefContract::Unknown),
			handler_id("form.resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("form.resource"), move |input: HandlerInput| {
			let calls = Arc::clone(&calls);
			async move {
				calls.fetch_add(1, Ordering::SeqCst);
				let form = input.decoded_input().parsed_form_data().unwrap();
				Ok(HandlerOutput::data(serde_json::json!({
					"name": form.text("name"),
					"tags": form.texts("tag").collect::<Vec<_>>(),
				})))
			}
		});
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn runtime_app_with_single_resource_handler<H>(handler: H) -> CommittedRuntimeApp
	where
		H: crate::execution_engine::RuntimeHandler + 'static,
	{
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/fail",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), handler);
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn runtime_app_with_single_view_handler<H>(handler: H) -> CommittedRuntimeApp
	where
		H: crate::execution_engine::RuntimeHandler + 'static,
	{
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
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
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("view"), handler);
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn asset_provider() -> impl PublicAssetBodyProvider {
		|asset: PublicAsset| async move {
			match asset.fs_path() {
				"app.hash.css" => Ok(PublicAssetBody::new(Bytes::from_static(b"body{}"))
					.with_content_type(HeaderValue::from_static("text/css"))),
				"root.js" => Ok(PublicAssetBody::new(Bytes::from_static(b"export {};"))
					.with_content_type(HeaderValue::from_static("application/javascript"))),
				_ => panic!("unexpected public asset {}", asset.fs_path()),
			}
		}
	}

	#[tokio::test]
	async fn runtime_app_finalizes_resource_responses() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.body(), &Bytes::from_static(b"pong"));
	}

	#[tokio::test]
	async fn runtime_app_finalizes_view_json_and_html_responses() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app(Arc::clone(&calls));

		let json_response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=_")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();
		let html_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/"))
					.with_view_html_input(ViewHtmlResponseInput::new("<main id=\"app\"></main>\n")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let json_body: serde_json::Value = serde_json::from_slice(json_response.body()).unwrap();
		let html_body = String::from_utf8(html_response.body().to_vec()).unwrap();

		assert_eq!(json_response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert!(json_body.get("client_build_id").is_none());
		assert_eq!(json_body["views_data"], serde_json::json!([{"view": true}]));
		assert!(html_body.contains("<main id=\"app\"></main>"));
		assert!(html_body.contains("\"client_build_id\":\"build-id\""));
		assert_eq!(calls.load(Ordering::SeqCst), 2);
	}

	#[tokio::test]
	async fn runtime_app_empty_vorma_json_query_preserves_html_response_contract() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();
		let body = String::from_utf8(response.body().to_vec()).unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert!(body.contains(&format!(r#"id="{VORMA_DATA_JSON_SCRIPT_EL_ID}""#)));
		assert!(body.contains("\"client_build_id\":\"build-id\""));
	}

	#[tokio::test]
	async fn runtime_app_dynamic_document_provider_feeds_view_json_and_html_responses() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app(Arc::clone(&calls));
		let document_calls = Arc::new(AtomicUsize::new(0));
		let provider = {
			let document_calls = Arc::clone(&document_calls);
			move |_| {
				let document_calls = Arc::clone(&document_calls);
				async move {
					document_calls.fetch_add(1, Ordering::SeqCst);
					Ok(DocumentContract::new(
						Vec::new(),
						Vec::new(),
						vec![DocumentElementContract::new("title").with_text_content("Dynamic")],
						Vec::new(),
						Vec::new(),
					))
				}
			}
		};

		let json_response = app
			.handle_request_with_document_provider(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=_")),
				),
				&asset_provider(),
				&provider,
			)
			.await
			.unwrap();
		let html_response = app
			.handle_request_with_document_provider(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/"))
					.with_view_html_input(ViewHtmlResponseInput::new("<main id=\"app\"></main>\n")),
				&asset_provider(),
				&provider,
			)
			.await
			.unwrap();
		let json_body: serde_json::Value = serde_json::from_slice(json_response.body()).unwrap();
		let html_body = String::from_utf8(html_response.body().to_vec()).unwrap();

		assert_eq!(json_body["title"]["dangerous_inner_html"], "Dynamic");
		assert!(html_body.contains("<title>Dynamic</title>"));
		assert_eq!(calls.load(Ordering::SeqCst), 2);
		assert_eq!(document_calls.load(Ordering::SeqCst), 2);
	}

	#[tokio::test]
	async fn runtime_app_build_skew_bypasses_dynamic_document_provider() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app(Arc::clone(&calls));
		let document_calls = Arc::new(AtomicUsize::new(0));
		let provider = {
			let document_calls = Arc::clone(&document_calls);
			move |_| {
				let document_calls = Arc::clone(&document_calls);
				async move {
					document_calls.fetch_add(1, Ordering::SeqCst);
					Ok(DocumentContract::default())
				}
			}
		};

		let response = app
			.handle_request_with_document_provider(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=stale")),
				),
				&asset_provider(),
				&provider,
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[BUILD_SKEW_HEADER], "1");
		assert_eq!(calls.load(Ordering::SeqCst), 0);
		assert_eq!(document_calls.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_dynamic_document_provider_runs_concurrently_with_view_handler() {
		let document_started = Arc::new(Notify::new());
		let handler_started = Arc::new(Notify::new());
		let app = runtime_app_with_single_view_handler({
			let document_started = Arc::clone(&document_started);
			let handler_started = Arc::clone(&handler_started);
			move |_| {
				let document_started = Arc::clone(&document_started);
				let handler_started = Arc::clone(&handler_started);
				async move {
					handler_started.notify_one();
					document_started.notified().await;
					Ok(HandlerOutput::data(serde_json::json!({"ready": true})))
				}
			}
		});
		let provider = {
			let document_started = Arc::clone(&document_started);
			let handler_started = Arc::clone(&handler_started);
			move |_| {
				let document_started = Arc::clone(&document_started);
				let handler_started = Arc::clone(&handler_started);
				async move {
					document_started.notify_one();
					handler_started.notified().await;
					Ok(DocumentContract::default())
				}
			}
		};

		let response = timeout(
			Duration::from_secs(1),
			app.handle_request_with_document_provider(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/")),
				&asset_provider(),
				&provider,
			),
		)
		.await
		.expect("document provider and view handler should run concurrently")
		.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
	}

	#[tokio::test]
	async fn runtime_app_dynamic_document_provider_error_cancels_running_view_handler() {
		struct DropSignal(Option<oneshot::Sender<()>>);

		impl Drop for DropSignal {
			fn drop(&mut self) {
				if let Some(sender) = self.0.take() {
					let _ = sender.send(());
				}
			}
		}

		let handler_started = Arc::new(Notify::new());
		let (dropped_sender, dropped_receiver) = oneshot::channel();
		let dropped_sender = Arc::new(Mutex::new(Some(dropped_sender)));
		let app = runtime_app_with_single_view_handler({
			let handler_started = Arc::clone(&handler_started);
			let dropped_sender = Arc::clone(&dropped_sender);
			move |_| {
				let handler_started = Arc::clone(&handler_started);
				let dropped_sender = Arc::clone(&dropped_sender);
				async move {
					handler_started.notify_one();
					let _drop_signal = DropSignal(dropped_sender.lock().unwrap().take());
					std::future::pending::<Result<HandlerOutput, HandlerExecutionError>>().await
				}
			}
		});
		let provider = {
			let handler_started = Arc::clone(&handler_started);
			move |_| {
				let handler_started = Arc::clone(&handler_started);
				async move {
					handler_started.notified().await;
					Err(crate::runtime_document::RuntimeDocumentError::new(
						"document failed",
					))
				}
			}
		};

		let response = timeout(
			Duration::from_secs(1),
			app.handle_request_with_document_provider(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/")),
				&asset_provider(),
				&provider,
			),
		)
		.await
		.expect("document failure should not wait for the running view handler")
		.unwrap();

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		timeout(Duration::from_secs(1), dropped_receiver)
			.await
			.expect("document failure should cancel the running view handler")
			.unwrap();
	}

	#[tokio::test]
	async fn runtime_app_build_skew_bypasses_view_execution() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app(Arc::clone(&calls));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=stale")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[BUILD_SKEW_HEADER], "1");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(calls.load(Ordering::SeqCst), 0);

		let head_response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::HEAD, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=stale")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(head_response.status(), StatusCode::OK);
		assert_eq!(head_response.headers()[BUILD_SKEW_HEADER], "1");
		assert_eq!(
			head_response.headers()[CONTENT_LENGTH],
			BUILD_SKEW_RESPONSE_BODY.len().to_string().as_str()
		);
		assert!(head_response.body().is_empty());
	}

	#[tokio::test]
	async fn runtime_app_finalizes_public_asset_get_and_head_responses() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));

		let get_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/static/app.hash.css")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let head_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::HEAD, "/static/app.hash.css")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(get_response.status(), StatusCode::OK);
		assert_eq!(
			get_response.headers()[CACHE_CONTROL],
			"public, max-age=31536000, immutable"
		);
		assert_eq!(get_response.headers()[CONTENT_LENGTH], "6");
		assert_eq!(get_response.headers()[CONTENT_TYPE], "text/css");
		assert_eq!(get_response.body(), &Bytes::from_static(b"body{}"));
		assert_eq!(head_response.headers()[CONTENT_LENGTH], "6");
		assert!(head_response.body().is_empty());
	}

	#[tokio::test]
	async fn runtime_app_serves_view_module_urls_as_public_capabilities() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/static/root.js")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CONTENT_TYPE], "application/javascript");
		assert_eq!(response.body(), &Bytes::from_static(b"export {};"));
	}

	#[tokio::test]
	async fn runtime_app_http_request_adapter_percent_decodes_paths() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));
		let request = Request::builder()
			.method(Method::GET)
			.uri("/static/%61pp.hash.css")
			.body(Bytes::new())
			.unwrap();

		let response = app
			.handle_http_request(request, ViewHtmlResponseInput::new(""), &asset_provider())
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), &Bytes::from_static(b"body{}"));
	}

	#[tokio::test]
	async fn runtime_app_http_request_adapter_rejects_invalid_decoded_path_utf8() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));
		let request = Request::builder()
			.method(Method::GET)
			.uri("/static/%FF.css")
			.body(Bytes::new())
			.unwrap();

		let response = app
			.handle_http_request(request, ViewHtmlResponseInput::new(""), &asset_provider())
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[CONTENT_TYPE], TEXT_CONTENT_TYPE);
		assert_eq!(response.body(), &Bytes::from_static(BAD_REQUEST_BODY));
	}

	#[tokio::test]
	async fn runtime_app_http_request_adapter_preserves_request_extensions() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
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
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("view"), |input: HandlerInput| async move {
			let marker = input
				.request()
				.extension::<RequestMarker>()
				.map(|marker| marker.0)
				.unwrap_or("");
			Ok(HandlerOutput::data(serde_json::json!({"marker": marker})))
		});
		let app = CommittedRuntimeApp::new(snapshot, handlers);
		let request = Request::builder()
			.method(Method::GET)
			.uri(format!("/?{VORMA_JSON_QUERY_KEY}=_"))
			.extension(RequestMarker("from-extension"))
			.body(Bytes::new())
			.unwrap();

		let response = app
			.handle_http_request(request, ViewHtmlResponseInput::new(""), &asset_provider())
			.await
			.unwrap();
		let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(
			payload["views_data"],
			serde_json::json!([{"marker": "from-extension"}])
		);
	}

	#[tokio::test]
	async fn runtime_app_finalizes_method_not_allowed_and_not_found() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));

		let method_without_route_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::POST, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let api_not_found_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/missing")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let not_found_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/missing")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let static_mount_not_found_response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/static/missing.css")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("get.resource"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/other",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("post.resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let path_method_app: CommittedRuntimeApp =
			CommittedRuntimeApp::new(snapshot, HandlerRegistry::default());
		let path_specific_method_response = path_method_app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::POST, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();
		let head_method_response = path_method_app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::HEAD, "/api/other")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(
			method_without_route_response.status(),
			StatusCode::METHOD_NOT_ALLOWED
		);
		assert_eq!(method_without_route_response.headers()[ALLOW], "GET, HEAD");
		assert_eq!(
			method_without_route_response.headers()[CONTENT_TYPE],
			TEXT_CONTENT_TYPE
		);
		assert_eq!(
			method_without_route_response.body(),
			&Bytes::from_static(METHOD_NOT_ALLOWED_BODY)
		);
		assert_eq!(
			method_without_route_response.headers()[CLIENT_BUILD_ID_HEADER],
			"build-id"
		);
		assert_eq!(api_not_found_response.status(), StatusCode::NOT_FOUND);
		assert_eq!(
			api_not_found_response.headers()[CLIENT_BUILD_ID_HEADER],
			"build-id"
		);
		assert!(api_not_found_response.body().is_empty());
		assert_eq!(not_found_response.status(), StatusCode::NOT_FOUND);
		assert!(not_found_response.body().is_empty());
		assert_eq!(
			not_found_response.headers()[CLIENT_BUILD_ID_HEADER],
			"build-id"
		);
		assert_eq!(
			static_mount_not_found_response.status(),
			StatusCode::NOT_FOUND
		);
		assert!(static_mount_not_found_response.body().is_empty());
		assert_eq!(
			static_mount_not_found_response.headers()[CLIENT_BUILD_ID_HEADER],
			"build-id"
		);
		assert_eq!(
			path_specific_method_response.status(),
			StatusCode::METHOD_NOT_ALLOWED
		);
		assert_eq!(path_specific_method_response.headers()[ALLOW], "GET, HEAD");
		assert_eq!(
			path_specific_method_response.headers()[CLIENT_BUILD_ID_HEADER],
			"build-id"
		);
		assert_eq!(
			path_specific_method_response.body(),
			&Bytes::from_static(METHOD_NOT_ALLOWED_BODY)
		);
		assert_eq!(
			head_method_response.status(),
			StatusCode::METHOD_NOT_ALLOWED
		);
		assert_eq!(head_method_response.headers()[ALLOW], "POST");
		assert_eq!(
			head_method_response.headers()[CONTENT_LENGTH],
			HeaderValue::from_str(&METHOD_NOT_ALLOWED_BODY.len().to_string()).unwrap()
		);
		assert!(head_method_response.body().is_empty());
	}

	#[tokio::test]
	async fn runtime_app_direct_http_head_bad_path_suppresses_bad_request_body() {
		let app = runtime_app(Arc::new(AtomicUsize::new(0)));
		let request = Request::builder()
			.method(Method::HEAD)
			.uri("/%FF")
			.body(Bytes::new())
			.unwrap();

		let response = app
			.handle_http_request(request, ViewHtmlResponseInput::new(""), &asset_provider())
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(
			response.headers()[CONTENT_LENGTH],
			HeaderValue::from_str(&BAD_REQUEST_BODY.len().to_string()).unwrap()
		);
		assert!(response.body().is_empty());
	}

	#[tokio::test]
	async fn runtime_app_bad_input_returns_bad_request_without_handler_invocation() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app_with_typed_resource(Arc::clone(&calls));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/api/search").with_query("page=nope"),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"Bad Request"}"#)
		);
		assert_eq!(calls.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_head_bad_input_suppresses_bad_request_body() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app_with_typed_resource(Arc::clone(&calls));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::HEAD, "/api/search").with_query("page=nope"),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(
			response.headers()[CONTENT_LENGTH],
			br#"{"error":"Bad Request"}"#.len().to_string().as_str()
		);
		assert!(response.body().is_empty());
		assert_eq!(calls.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_decodes_form_data_resource_input() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app_with_form_data_resource(Arc::clone(&calls));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::POST, "/api/form")
						.with_headers(HeaderMap::from_iter([(
							CONTENT_TYPE,
							HeaderValue::from_static("application/x-www-form-urlencoded"),
						)]))
						.with_body(Bytes::from_static(b"name=ada&tag=a&tag=b")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"name":"ada","tags":["a","b"]}"#)
		);
		assert_eq!(calls.load(Ordering::SeqCst), 1);
	}

	#[tokio::test]
	async fn runtime_app_rejects_form_data_resource_without_form_content_type() {
		let calls = Arc::new(AtomicUsize::new(0));
		let app = runtime_app_with_form_data_resource(Arc::clone(&calls));

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::POST, "/api/form")
						.with_body(Bytes::from_static(b"name=ada")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"Bad Request"}"#)
		);
		assert_eq!(calls.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_resource_handler_failure_returns_internal_server_error_response() {
		let app = runtime_app_with_single_resource_handler(|_| async {
			Err(HandlerExecutionError::new("database unavailable"))
		});

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/fail")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"Internal Server Error"}"#)
		);
	}

	#[tokio::test]
	async fn runtime_app_resource_handler_terminal_error_preserves_handler_owned_effects() {
		let validation_header = http::header::HeaderName::from_static("x-validation");
		let app = runtime_app_with_single_resource_handler({
			let validation_header = validation_header.clone();
			move |_| {
				let validation_header = validation_header.clone();
				async move {
					let mut effects = ResponseEffects::default();
					effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
					effects.set_header(validation_header, HeaderValue::from_static("1"));
					Err(HandlerExecutionError::new("validation failed").with_effects(effects))
				}
			}
		});

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/fail")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[&validation_header], "1");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"bad input"}"#)
		);
	}

	#[tokio::test]
	async fn runtime_app_resource_handler_client_redirect_uses_contract_header() {
		let app = runtime_app_with_single_resource_handler(|input: HandlerInput| async move {
			let mut effects = ResponseEffects::default();
			effects
				.redirect_with_client_preference(
					accepts_client_redirect(input.request().headers()),
					"/target",
					None,
				)
				.unwrap();
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		let accepts_client_redirect_header =
			http::header::HeaderName::from_bytes(CLIENT_ACCEPTS_REDIRECT_HEADER.as_bytes())
				.unwrap();

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/fail").with_headers(
					HeaderMap::from_iter([(
						accepts_client_redirect_header,
						HeaderValue::from_static("1"),
					)]),
				)),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CLIENT_REDIRECT_HEADER], "/target");
		assert!(!response.headers().contains_key(LOCATION));
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert!(response.body().is_empty());
	}

	#[tokio::test]
	async fn runtime_app_resource_handler_failure_drops_nonterminal_effects() {
		let success_header = http::header::HeaderName::from_static("x-success");
		let app = runtime_app_with_single_resource_handler({
			let success_header = success_header.clone();
			move |_| {
				let success_header = success_header.clone();
				async move {
					let mut effects = ResponseEffects::default();
					effects.set_status(StatusCode::CREATED);
					effects.set_header(success_header, HeaderValue::from_static("1"));
					Err(HandlerExecutionError::new("database unavailable").with_effects(effects))
				}
			}
		});

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/fail")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert!(!response.headers().contains_key(&success_header));
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"Internal Server Error"}"#)
		);
	}

	#[tokio::test]
	async fn runtime_app_view_handler_failure_returns_server_error_payload() {
		let app = runtime_app_with_single_view_handler(|_| async {
			Err(HandlerExecutionError::with_client_message(
				"database unavailable",
				"client visible",
			))
		});

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=_")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();
		let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(payload["outermost_server_err"], "client visible");
		assert_eq!(payload["outermost_server_err_idx"], 0);
		assert_eq!(
			payload["import_urls"],
			serde_json::json!(["/static/root.js"])
		);
	}

	#[tokio::test]
	async fn runtime_app_view_handler_terminal_error_preserves_handler_owned_effects() {
		let validation_header = http::header::HeaderName::from_static("x-validation");
		let app = runtime_app_with_single_view_handler({
			let validation_header = validation_header.clone();
			move |_| {
				let validation_header = validation_header.clone();
				async move {
					let mut effects = ResponseEffects::default();
					effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
					effects.set_header(validation_header, HeaderValue::from_static("1"));
					Err(HandlerExecutionError::new("validation failed").with_effects(effects))
				}
			}
		});

		let response = app
			.handle_request(
				RuntimeAppRequest::new(
					RequestInput::new(Method::GET, "/")
						.with_query(format!("{VORMA_JSON_QUERY_KEY}=_")),
				),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[&validation_header], "1");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		/*
		View-path terminal errors stay plain text: the envelope is a
		resource-protocol shape.
		*/
		assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
	}

	/// App with the given middleware ids (scope "/") ahead of one GET /api/ping
	/// resource, mirroring the retired black-box middleware suites.
	fn runtime_app_with_middleware_chain(
		middleware_ids: &[&str],
		handlers: HandlerRegistry,
	) -> CommittedRuntimeApp {
		let mut declarations = FrameworkDeclarations::default();
		for middleware_id in middleware_ids {
			declarations.add_middleware(MiddlewareDeclaration::new(handler_id(middleware_id)));
		}
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn effects_output(configure: impl FnOnce(&mut ResponseEffects)) -> HandlerOutput {
		let mut effects = ResponseEffects::default();
		configure(&mut effects);
		HandlerOutput::empty().with_effects(effects)
	}

	#[tokio::test]
	async fn runtime_app_merges_middleware_effects_in_scope_order_with_handler_last() {
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("first"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_header(
					HeaderName::from_static("x-shared"),
					HeaderValue::from_static("first"),
				);
				effects.set_header(
					HeaderName::from_static("x-first"),
					HeaderValue::from_static("1"),
				);
			}))
		});
		handlers.insert(handler_id("second"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_header(
					HeaderName::from_static("x-shared"),
					HeaderValue::from_static("second"),
				);
				effects.set_header(
					HeaderName::from_static("x-second"),
					HeaderValue::from_static("1"),
				);
			}))
		});
		handlers.insert(handler_id("resource"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_header(
				HeaderName::from_static("x-shared"),
				HeaderValue::from_static("resource"),
			);
			Ok(HandlerOutput::body(Bytes::from_static(b"pong")).with_effects(effects))
		});
		let app = runtime_app_with_middleware_chain(&["first", "second"], handlers);

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()["x-first"], "1");
		assert_eq!(response.headers()["x-second"], "1");
		assert_eq!(response.headers()["x-shared"], "resource");
		assert_eq!(response.body(), &Bytes::from_static(b"pong"));
	}

	#[tokio::test]
	async fn runtime_app_terminal_middleware_suppresses_handler_and_later_middleware_effects() {
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("first"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_status_with_text(StatusCode::UNAUTHORIZED, "unauthorized");
				effects.set_header(
					HeaderName::from_static("x-first"),
					HeaderValue::from_static("1"),
				);
			}))
		});
		handlers.insert(handler_id("second"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_header(
					HeaderName::from_static("x-second"),
					HeaderValue::from_static("1"),
				);
			}))
		});
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
				}
			});
		}
		let app = runtime_app_with_middleware_chain(&["first", "second"], handlers);

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
		assert_eq!(response.headers()["x-first"], "1");
		assert!(!response.headers().contains_key("x-second"));
		assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_middleware_success_status_does_not_short_circuit_handler() {
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("first"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_status(StatusCode::CREATED);
			}))
		});
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
				}
			});
		}
		let app = runtime_app_with_middleware_chain(&["first"], handlers);

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::CREATED);
		assert_eq!(response.body(), &Bytes::from_static(b"pong"));
		assert_eq!(handler_runs.load(Ordering::SeqCst), 1);
	}

	#[tokio::test]
	async fn runtime_app_middleware_redirect_short_circuits_handler_with_location() {
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("first"), |_| async {
			Ok(effects_output(|effects| {
				effects
					.redirect(StatusCode::SEE_OTHER, "/login")
					.expect("relative redirect location is valid");
			}))
		});
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
				}
			});
		}
		let app = runtime_app_with_middleware_chain(&["first"], handlers);

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::SEE_OTHER);
		assert_eq!(response.headers()[LOCATION], "/login");
		assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	}

	#[tokio::test]
	async fn runtime_app_middleware_failure_preserves_prior_committed_middleware_effects() {
		let handler_runs = Arc::new(AtomicUsize::new(0));
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("first"), |_| async {
			Ok(effects_output(|effects| {
				effects.set_header(
					HeaderName::from_static("x-first"),
					HeaderValue::from_static("1"),
				);
			}))
		});
		handlers.insert(handler_id("second"), |_| async {
			Err(HandlerExecutionError::new("boom"))
		});
		{
			let handler_runs = Arc::clone(&handler_runs);
			handlers.insert(handler_id("resource"), move |_| {
				let handler_runs = Arc::clone(&handler_runs);
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
				}
			});
		}
		let app = runtime_app_with_middleware_chain(&["first", "second"], handlers);

		let response = app
			.handle_request(
				RuntimeAppRequest::new(RequestInput::new(Method::GET, "/api/ping")),
				&asset_provider(),
			)
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert_eq!(response.headers()["x-first"], "1");
		assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	}
}
