//! Tower service boundary for committed runtime applications.
//!
//! Second serving layer: wraps a `CommittedRuntimeApp` with the transport
//! concerns — body collection and limits, asset-provider dispatch, dynamic
//! document providers — and exposes the whole thing as a Tower service for
//! `runtime_host` to mount.

use std::convert::Infallible;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};

use bytes::{Buf, Bytes};
use http::header::{CONTENT_LENGTH, CONTENT_TYPE, HeaderValue};
use http::{Method, Request, Response, StatusCode};
use http_body::Body;
use http_body_util::{BodyExt, Full, LengthLimitError, Limited};
use tower_service::Service;

use crate::asset_body_provider::PublicAssetBodyProvider;
use crate::response_finalizer::TEXT_CONTENT_TYPE;
use crate::runtime_app::{CommittedRuntimeApp, INTERNAL_SERVER_ERROR_BODY, RuntimeAppError};
use crate::runtime_document::{RuntimeDocumentProvider, StaticRuntimeDocumentProvider};
use crate::view_response::ViewHtmlResponseInput;

const PAYLOAD_TOO_LARGE_BODY: &[u8] = b"Payload Too Large\n";

/// Tower service for one committed runtime application snapshot.
pub struct CommittedRuntimeService<P> {
	app: Arc<CommittedRuntimeApp>,
	asset_provider: Arc<P>,
	document_provider: Arc<dyn RuntimeDocumentProvider>,
	view_html: ViewHtmlResponseInput<'static>,
	request_body_limit: usize,
}

impl<P> CommittedRuntimeService<P>
where
	P: PublicAssetBodyProvider + 'static,
{
	/// Create a service for one committed runtime app.
	pub fn new(app: CommittedRuntimeApp, asset_provider: P, request_body_limit: usize) -> Self {
		let document_provider =
			StaticRuntimeDocumentProvider::new(app.snapshot().graph().document().clone());
		Self {
			app: Arc::new(app),
			asset_provider: Arc::new(asset_provider),
			document_provider: Arc::new(document_provider),
			view_html: ViewHtmlResponseInput::new("").with_is_dev(crate::is_dev()),
			request_body_limit,
		}
	}

	/// Replace the trusted SSR HTML input used for HTML view responses.
	pub fn with_view_html(mut self, view_html: ViewHtmlResponseInput<'static>) -> Self {
		self.view_html = view_html;
		self
	}

	/// Replace the runtime document provider used for matched view responses.
	pub fn with_document_provider<D>(mut self, document_provider: D) -> Self
	where
		D: RuntimeDocumentProvider + 'static,
	{
		self.document_provider = Arc::new(document_provider);
		self
	}

	/// Committed runtime application served by this service.
	pub fn app(&self) -> &CommittedRuntimeApp {
		&self.app
	}

	/// Maximum request body bytes this service will collect.
	pub fn request_body_limit(&self) -> usize {
		self.request_body_limit
	}

	/// Handle an already-collected request.
	pub async fn handle_request(&self, request: Request<Bytes>) -> Response<Bytes> {
		let method = request.method().clone();
		let response = self
			.app
			.handle_http_request_with_document_provider(
				request,
				self.view_html.clone(),
				self.asset_provider.as_ref(),
				self.document_provider.as_ref(),
			)
			.await
			.unwrap_or_else(runtime_app_error_response);
		if method == Method::HEAD {
			return suppress_head_body_preserving_length(response);
		}
		response
	}
}

impl<P> Clone for CommittedRuntimeService<P> {
	fn clone(&self) -> Self {
		Self {
			app: Arc::clone(&self.app),
			asset_provider: Arc::clone(&self.asset_provider),
			document_provider: Arc::clone(&self.document_provider),
			view_html: self.view_html.clone(),
			request_body_limit: self.request_body_limit,
		}
	}
}

impl<P, B> Service<Request<B>> for CommittedRuntimeService<P>
where
	P: PublicAssetBodyProvider + 'static,
	B: Body + Send + 'static,
	B::Data: Buf + Send + 'static,
	B::Error:
		std::fmt::Display + Send + Sync + 'static + Into<Box<dyn std::error::Error + Send + Sync>>,
{
	type Response = Response<Full<Bytes>>;
	type Error = Infallible;
	type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

	fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
		Poll::Ready(Ok(()))
	}

	fn call(&mut self, request: Request<B>) -> Self::Future {
		let service = self.clone();
		Box::pin(async move {
			let head_request = request.method() == Method::HEAD;
			let response = match collect_limited_request(request, service.request_body_limit).await
			{
				Ok(request) => service.handle_request(request).await,
				Err(RequestBodyError::PayloadTooLarge) => {
					plain_text_response(StatusCode::PAYLOAD_TOO_LARGE, PAYLOAD_TOO_LARGE_BODY)
				}
				Err(RequestBodyError::Read { .. }) => internal_server_error_response(),
			};
			let response = if head_request {
				suppress_head_body_preserving_length(response)
			} else {
				response
			};
			Ok(response.map(Full::new))
		})
	}
}

async fn collect_limited_request<B>(
	request: Request<B>,
	limit: usize,
) -> Result<Request<Bytes>, RequestBodyError>
where
	B: Body + Send + 'static,
	B::Data: Buf + Send + 'static,
	B::Error:
		std::fmt::Display + Send + Sync + 'static + Into<Box<dyn std::error::Error + Send + Sync>>,
{
	let (parts, body) = request.into_parts();
	let bytes = match Limited::new(body, limit).collect().await {
		Ok(collected) => collected.to_bytes(),
		Err(error) if error.is::<LengthLimitError>() => {
			return Err(RequestBodyError::PayloadTooLarge);
		}
		Err(error) => {
			return Err(RequestBodyError::Read {
				message: error.to_string(),
			});
		}
	};
	Ok(Request::from_parts(parts, bytes))
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum RequestBodyError {
	PayloadTooLarge,
	Read { message: String },
}

fn runtime_app_error_response(_error: RuntimeAppError) -> Response<Bytes> {
	internal_server_error_response()
}

fn internal_server_error_response() -> Response<Bytes> {
	plain_text_response(
		StatusCode::INTERNAL_SERVER_ERROR,
		INTERNAL_SERVER_ERROR_BODY,
	)
}

fn plain_text_response(status: StatusCode, body: &'static [u8]) -> Response<Bytes> {
	let mut response = Response::new(Bytes::from_static(body));
	*response.status_mut() = status;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(TEXT_CONTENT_TYPE));
	response
}

fn suppress_head_body_preserving_length(response: Response<Bytes>) -> Response<Bytes> {
	let (mut parts, body) = response.into_parts();
	if !parts.headers.contains_key(CONTENT_LENGTH) {
		let value = HeaderValue::from_str(&body.len().to_string())
			.expect("usize string should be a valid content-length header value");
		parts.headers.insert(CONTENT_LENGTH, value);
	}
	Response::from_parts(parts, Bytes::new())
}

#[cfg(test)]
mod tests {
	use std::sync::Arc;
	use std::sync::atomic::{AtomicUsize, Ordering};

	use http::header::CONTENT_LENGTH;
	use http::{Method, Request, StatusCode};
	use http_body_util::Full;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyError;
	use crate::config::UiVariant;
	use crate::contracts::{DocumentContract, DocumentElementContract};
	use crate::execution_engine::{HandlerInput, HandlerOutput, HandlerRegistry};
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration, ViewDeclaration,
	};
	use crate::response_finalizer::CLIENT_BUILD_ID_HEADER;
	use crate::runtime_app::METHOD_NOT_ALLOWED_BODY;
	use crate::runtime_manifest::{ClientModule, RuntimeManifest, RuntimeViewModule};
	use crate::runtime_snapshot::{RuntimeSnapshot, RuntimeSnapshotInput};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn runtime_app(handler_calls: Arc<AtomicUsize>) -> CommittedRuntimeApp {
		runtime_app_with_manifest(handler_calls, runtime_manifest())
	}

	fn runtime_app_with_manifest(
		handler_calls: Arc<AtomicUsize>,
		manifest: RuntimeManifest,
	) -> CommittedRuntimeApp {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/echo",
			None,
			None,
			route_type_contract(),
			handler_id("echo"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("echo"), move |input: HandlerInput| {
			let handler_calls = Arc::clone(&handler_calls);
			async move {
				handler_calls.fetch_add(1, Ordering::SeqCst);
				Ok(HandlerOutput::body(Bytes::from(
					input.request().body().len().to_string(),
				)))
			}
		});
		handlers.insert(handler_id("view"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"view": true})))
		});
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn runtime_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			std::collections::BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			Default::default(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		)
	}

	fn dev_runtime_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			std::collections::BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			Vec::new(),
			Default::default(),
			vec![RuntimeViewModule::new(
				"/",
				"http://127.0.0.1:5173/root.tsx",
				Vec::new(),
				Vec::new(),
			)],
		)
		.with_client_entry(ClientModule::new(
			"http://127.0.0.1:5173/entry.tsx",
			Vec::new(),
			Vec::new(),
		))
		.with_dev_metadata(5173, 4173, "refresh-token")
		.with_ui_variant(UiVariant::React.as_str())
	}

	fn asset_provider() -> impl PublicAssetBodyProvider {
		|_| async { Err(PublicAssetBodyError::new("no asset")) }
	}

	#[tokio::test]
	async fn runtime_service_collects_body_before_delegating_to_runtime_app() {
		let handler_calls = Arc::new(AtomicUsize::new(0));
		let mut service = CommittedRuntimeService::new(
			runtime_app(Arc::clone(&handler_calls)),
			asset_provider(),
			1024,
		);
		let request = Request::builder()
			.method(Method::GET)
			.uri("/api/echo")
			.body(Full::new(Bytes::from_static(b"hello")))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(handler_calls.load(Ordering::SeqCst), 1);
		assert_eq!(body, Bytes::from_static(b"5"));
	}

	#[tokio::test]
	async fn runtime_service_rejects_oversized_body_before_handler_execution() {
		let handler_calls = Arc::new(AtomicUsize::new(0));
		let mut service = CommittedRuntimeService::new(
			runtime_app(Arc::clone(&handler_calls)),
			asset_provider(),
			4,
		);
		let request = Request::builder()
			.method(Method::GET)
			.uri("/api/echo")
			.body(Full::new(Bytes::from_static(b"hello")))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let status = response.status();
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(handler_calls.load(Ordering::SeqCst), 0);
		assert_eq!(status, StatusCode::PAYLOAD_TOO_LARGE);
		assert_eq!(body, Bytes::from_static(PAYLOAD_TOO_LARGE_BODY));
	}

	#[tokio::test]
	async fn runtime_service_head_limit_error_preserves_content_length_without_body() {
		let mut service = CommittedRuntimeService::new(
			runtime_app(Arc::new(AtomicUsize::new(0))),
			asset_provider(),
			4,
		);
		let request = Request::builder()
			.method(Method::HEAD)
			.uri("/api/echo")
			.body(Full::new(Bytes::from_static(b"hello")))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let status = response.status();
		let content_length = response.headers()[CONTENT_LENGTH].clone();
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(status, StatusCode::PAYLOAD_TOO_LARGE);
		assert_eq!(
			content_length,
			HeaderValue::from_str(&PAYLOAD_TOO_LARGE_BODY.len().to_string()).unwrap()
		);
		assert!(body.is_empty());
	}

	#[tokio::test]
	async fn runtime_service_head_method_not_allowed_preserves_content_length_without_body() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/api/submit",
			None,
			None,
			route_type_contract(),
			handler_id("submit"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			Default::default(),
			Vec::new(),
			Default::default(),
			Vec::new(),
		);
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let app: CommittedRuntimeApp =
			CommittedRuntimeApp::new(snapshot, HandlerRegistry::default());
		let mut service = CommittedRuntimeService::new(app, asset_provider(), 1024);
		let request = Request::builder()
			.method(Method::HEAD)
			.uri("/api/submit")
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let status = response.status();
		let content_length = response.headers()[CONTENT_LENGTH].clone();
		let has_client_build_id = response.headers().contains_key(CLIENT_BUILD_ID_HEADER);
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(status, StatusCode::METHOD_NOT_ALLOWED);
		assert_eq!(
			content_length,
			HeaderValue::from_str(&METHOD_NOT_ALLOWED_BODY.len().to_string()).unwrap()
		);
		assert!(has_client_build_id);
		assert!(body.is_empty());
	}

	#[tokio::test]
	async fn runtime_service_uses_dynamic_document_provider_for_view_responses() {
		let mut service = CommittedRuntimeService::new(
			runtime_app(Arc::new(AtomicUsize::new(0))),
			asset_provider(),
			1024,
		)
		.with_document_provider(|_| async {
			Ok(DocumentContract::new(
				Vec::new(),
				Vec::new(),
				vec![DocumentElementContract::new("title").with_text_content("Service")],
				Vec::new(),
				Vec::new(),
			))
		})
		.with_view_html(ViewHtmlResponseInput::new("<main id=\"app\"></main>\n"));
		let request = Request::builder()
			.method(Method::GET)
			.uri("/")
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let html = String::from_utf8(body.to_vec()).unwrap();

		assert!(html.contains("<title>Service</title>"));
		assert!(html.contains("<main id=\"app\"></main>"));
	}

	#[tokio::test]
	async fn runtime_service_defaults_view_html_to_dev_mode_when_process_is_dev() {
		let mut service = crate::envutil::with_dev_mode(|| {
			CommittedRuntimeService::new(
				runtime_app_with_manifest(Arc::new(AtomicUsize::new(0)), dev_runtime_manifest()),
				asset_provider(),
				1024,
			)
		});
		let request = Request::builder()
			.method(Method::GET)
			.uri("/")
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let html = String::from_utf8(body.to_vec()).unwrap();

		assert!(html.contains("\"is_dev\":true"));
		assert!(html.contains("/@react-refresh"));
		assert!(html.contains("/@vite/client"));
		assert!(html.contains("refresh-token"));
	}
}
