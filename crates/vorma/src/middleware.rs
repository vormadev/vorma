//! Optional Tower middleware helpers for Vorma app servers.

use std::error::Error as StdError;
use std::fmt;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};
use std::time::Duration;

use bytes::Bytes;
use http::header::HeaderName;
use http::header::{CACHE_CONTROL, CONTENT_LENGTH, ETAG, IF_NONE_MATCH, SET_COOKIE};
use http::{Extensions, HeaderMap, HeaderValue, Method, Request, Response, StatusCode, Uri};
use http_body_util::{BodyExt, Either, Full, LengthLimitError, Limited};
use tower_http::catch_panic::{CatchPanicLayer, DefaultResponseForPanic};
use tower_http::compression::CompressionLayer;
use tower_http::limit::RequestBodyLimitLayer;
use tower_http::request_id::{MakeRequestUuid, PropagateRequestId, SetRequestId};
use tower_http::timeout::{RequestBodyTimeoutLayer, ResponseBodyTimeoutLayer, TimeoutLayer};
use tower_layer::Layer;
use tower_service::Service;

type BoxError = Box<dyn StdError + Send + Sync>;

/////////////////////////////////////////////////////////////////////
/////// Header Hygiene
/////////////////////////////////////////////////////////////////////

/// Tower layer that sets and propagates `x-request-id`.
#[derive(Clone, Debug, Default)]
pub struct RequestIdLayer;

/// Sets and propagates `x-request-id` using Tower HTTP's UUID request IDs.
pub fn request_id() -> RequestIdLayer {
	RequestIdLayer
}

impl<S> Layer<S> for RequestIdLayer {
	type Service = SetRequestId<PropagateRequestId<S>, MakeRequestUuid>;

	fn layer(&self, inner: S) -> Self::Service {
		SetRequestId::x_request_id(PropagateRequestId::x_request_id(inner), MakeRequestUuid)
	}
}

/// Tower layer that marks sensitive request and response headers.
#[derive(Clone, Debug, Default)]
pub struct SensitiveHeadersLayer;

/// Marks sensitive request and response headers so logs/debug output can redact them.
pub fn sensitive_headers() -> SensitiveHeadersLayer {
	SensitiveHeadersLayer
}

impl<S> Layer<S> for SensitiveHeadersLayer {
	type Service = SensitiveHeadersService<S>;

	fn layer(&self, inner: S) -> Self::Service {
		SensitiveHeadersService { inner }
	}
}

/// Service produced by [`SensitiveHeadersLayer`].
#[derive(Clone, Debug)]
pub struct SensitiveHeadersService<S> {
	inner: S,
}

impl<S, ReqBody, ResBody> Service<Request<ReqBody>> for SensitiveHeadersService<S>
where
	S: Service<Request<ReqBody>, Response = Response<ResBody>>,
	S::Future: Send + 'static,
	S::Error: Send + 'static,
	ResBody: Send + 'static,
{
	type Response = Response<ResBody>;
	type Error = S::Error;
	type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

	fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
		self.inner.poll_ready(cx)
	}

	fn call(&mut self, mut request: Request<ReqBody>) -> Self::Future {
		mark_sensitive_headers(request.headers_mut());
		let response = self.inner.call(request);

		Box::pin(async move {
			let mut response = response.await?;
			mark_sensitive_headers(response.headers_mut());
			Ok(response)
		})
	}
}

fn mark_sensitive_headers(headers: &mut http::HeaderMap) {
	for name in SENSITIVE_HEADERS {
		if let http::header::Entry::Occupied(mut entry) = headers.entry(name.clone()) {
			for value in entry.iter_mut() {
				value.set_sensitive(true);
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// Resilience
/////////////////////////////////////////////////////////////////////

/// Converts panics into a 500 response at the Tower layer.
pub fn panic_recovery() -> CatchPanicLayer<DefaultResponseForPanic> {
	CatchPanicLayer::new()
}

/////////////////////////////////////////////////////////////////////
/////// Request Limits
/////////////////////////////////////////////////////////////////////

/// Rejects request bodies larger than `bytes`.
pub fn request_body_limit(bytes: usize) -> RequestBodyLimitLayer {
	RequestBodyLimitLayer::new(bytes)
}

/// Times out the whole handler service after `seconds`.
pub fn handler_timeout(seconds: u64) -> TimeoutLayer {
	TimeoutLayer::with_status_code(StatusCode::REQUEST_TIMEOUT, Duration::from_secs(seconds))
}

/// Times out request body reads after `seconds`.
pub fn request_body_timeout(seconds: u64) -> RequestBodyTimeoutLayer {
	RequestBodyTimeoutLayer::new(Duration::from_secs(seconds))
}

/// Times out response body writes after `seconds`.
pub fn response_body_timeout(seconds: u64) -> ResponseBodyTimeoutLayer {
	ResponseBodyTimeoutLayer::new(Duration::from_secs(seconds))
}

/////////////////////////////////////////////////////////////////////
/////// Compression
/////////////////////////////////////////////////////////////////////

/// Applies standard HTTP response compression.
pub fn compression() -> CompressionLayer {
	CompressionLayer::new()
}

/////////////////////////////////////////////////////////////////////
/////// ETags
/////////////////////////////////////////////////////////////////////

const DEFAULT_ETAG_MAX_BODY_SIZE: u64 = 8 * 1024 * 1024;
const CLIENT_BUILD_ID_HEADER: &str = "x-vorma-client-build-id";
const NOT_MODIFIED_PAYLOAD_HEADERS: &[&str] = &[
	"content-type",
	"content-length",
	"content-encoding",
	"content-language",
	"content-md5",
	"content-range",
	"content-disposition",
	"last-modified",
	"digest",
];

type EtagSkipPredicate = Arc<dyn for<'a> Fn(&EtagRequest<'a>) -> bool + Send + Sync>;

/// Tower layer that adds ETag headers and handles `If-None-Match`.
#[derive(Clone)]
pub struct EtagLayer {
	max_body_size: u64,
	strong: bool,
	skip: Option<EtagSkipPredicate>,
}

/// Adds conditional ETag support for successful GET/HEAD responses.
pub fn etag() -> EtagLayer {
	EtagLayer {
		max_body_size: DEFAULT_ETAG_MAX_BODY_SIZE,
		strong: false,
		skip: None,
	}
}

impl EtagLayer {
	/// Emit strong ETags instead of weak ETags.
	pub fn strong(mut self) -> Self {
		self.strong = true;
		self
	}

	/// Maximum response body size that will be buffered for ETag generation.
	pub fn max_body_size(mut self, bytes: usize) -> Self {
		self.max_body_size = bytes as u64;
		self
	}

	/// Skip ETag generation for requests matching `predicate`.
	pub fn skip<F>(mut self, predicate: F) -> Self
	where
		F: for<'a> Fn(&EtagRequest<'a>) -> bool + Send + Sync + 'static,
	{
		self.skip = Some(Arc::new(predicate));
		self
	}
}

impl fmt::Debug for EtagLayer {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_struct("EtagLayer")
			.field("max_body_size", &self.max_body_size)
			.field("strong", &self.strong)
			.field("skip", &self.skip.is_some())
			.finish()
	}
}

impl<S> Layer<S> for EtagLayer {
	type Service = EtagService<S>;

	fn layer(&self, inner: S) -> Self::Service {
		EtagService {
			inner,
			max_body_size: self.max_body_size,
			strong: self.strong,
			skip: self.skip.clone(),
		}
	}
}

/// Request metadata passed to an ETag skip predicate.
pub struct EtagRequest<'a> {
	method: &'a Method,
	uri: &'a Uri,
	headers: &'a HeaderMap,
	extensions: &'a Extensions,
}

impl<'a> EtagRequest<'a> {
	/// Request method.
	pub fn method(&self) -> &'a Method {
		self.method
	}

	/// Request URI.
	pub fn uri(&self) -> &'a Uri {
		self.uri
	}

	/// Request headers.
	pub fn headers(&self) -> &'a HeaderMap {
		self.headers
	}

	/// Request extensions.
	pub fn extensions(&self) -> &'a Extensions {
		self.extensions
	}
}

/// Service produced by [`EtagLayer`].
#[derive(Clone)]
pub struct EtagService<S> {
	inner: S,
	max_body_size: u64,
	strong: bool,
	skip: Option<EtagSkipPredicate>,
}

impl<S: fmt::Debug> fmt::Debug for EtagService<S> {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_struct("EtagService")
			.field("inner", &self.inner)
			.field("max_body_size", &self.max_body_size)
			.field("strong", &self.strong)
			.field("skip", &self.skip.is_some())
			.finish()
	}
}

impl<S, ReqBody, ResBody> Service<Request<ReqBody>> for EtagService<S>
where
	S: Service<Request<ReqBody>, Response = Response<ResBody>>,
	S::Future: Send + 'static,
	S::Error: Send + 'static,
	ReqBody: Send + 'static,
	ResBody: http_body::Body<Data = Bytes> + Send + 'static,
	ResBody::Error: Into<BoxError> + Send + Sync + 'static,
{
	type Response = Response<Either<ResBody, Full<Bytes>>>;
	type Error = S::Error;
	type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

	fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
		self.inner.poll_ready(cx)
	}

	fn call(&mut self, request: Request<ReqBody>) -> Self::Future {
		let method = request.method().clone();
		let request_etags = request.headers().get(IF_NONE_MATCH).cloned();
		let max_body_size = self.max_body_size;
		let strong = self.strong;
		let should_skip = etag_request_should_skip(self.skip.as_ref(), &request);
		let response = self.inner.call(request);

		Box::pin(async move {
			let response = response.await?;

			if should_skip
				|| !etag_request_method_is_eligible(&method)
				|| !etag_response_is_eligible(&response, max_body_size)
			{
				return Ok(response.map(Either::Left));
			}

			let (mut parts, body) = response.into_parts();
			let bytes = match Limited::new(body, max_body_size as usize).collect().await {
				Ok(collected) => collected.to_bytes(),
				Err(error) if error.is::<LengthLimitError>() => {
					let mut response = Response::new(Either::Right(Full::new(Bytes::from_static(
						b"response body exceeded ETag limit",
					))));
					*response.status_mut() = StatusCode::INTERNAL_SERVER_ERROR;
					return Ok(response);
				}
				Err(_error) => {
					let mut response = Response::new(Either::Right(Full::new(Bytes::from_static(
						b"response body error",
					))));
					*response.status_mut() = StatusCode::INTERNAL_SERVER_ERROR;
					return Ok(response);
				}
			};
			if bytes.is_empty() || bytes.len() as u64 > max_body_size {
				return Ok(Response::from_parts(parts, Either::Right(Full::new(bytes))));
			}

			let tag = generate_etag(&bytes, &parts.headers, strong);
			if etag_matches(request_etags.as_ref(), &tag) {
				parts.status = StatusCode::NOT_MODIFIED;
				remove_not_modified_payload_headers(&mut parts.headers);
				parts.headers.insert(ETAG, tag);
				return Ok(Response::from_parts(
					parts,
					Either::Right(Full::new(Bytes::new())),
				));
			}

			parts.headers.insert(ETAG, tag);
			parts.headers.insert(
				CONTENT_LENGTH,
				HeaderValue::from_str(&bytes.len().to_string()).expect("body length is a header"),
			);
			Ok(Response::from_parts(parts, Either::Right(Full::new(bytes))))
		})
	}
}

fn etag_request_should_skip<ReqBody>(
	skip: Option<&EtagSkipPredicate>,
	request: &Request<ReqBody>,
) -> bool {
	let Some(skip) = skip else {
		return false;
	};
	let request = EtagRequest {
		method: request.method(),
		uri: request.uri(),
		headers: request.headers(),
		extensions: request.extensions(),
	};
	skip(&request)
}

fn etag_request_method_is_eligible(method: &Method) -> bool {
	method == Method::GET || method == Method::HEAD
}

fn etag_response_is_eligible<B>(response: &Response<B>, max_body_size: u64) -> bool
where
	B: http_body::Body,
{
	if response.status() != StatusCode::OK {
		return false;
	}
	if response.headers().contains_key(SET_COOKIE) {
		return false;
	}
	if cache_control_has_no_store(response.headers()) {
		return false;
	}
	response
		.body()
		.size_hint()
		.exact()
		.is_some_and(|size| size > 0 && size <= max_body_size)
}

fn cache_control_has_no_store(headers: &HeaderMap) -> bool {
	headers
		.get_all(CACHE_CONTROL)
		.iter()
		.filter_map(|value| value.to_str().ok())
		.flat_map(|value| value.split(','))
		.any(|directive| {
			let name = directive
				.split_once('=')
				.map_or(directive, |(name, _value)| name)
				.trim();
			name.eq_ignore_ascii_case("no-store")
		})
}

fn generate_etag(bytes: &Bytes, headers: &HeaderMap, strong: bool) -> HeaderValue {
	let mut hasher = blake3::Hasher::new();
	hasher.update(bytes);
	if let Some(build_id) = headers.get(CLIENT_BUILD_ID_HEADER) {
		hasher.update(build_id.as_bytes());
	}
	let digest = hasher.finalize().to_hex();
	let tag = if strong {
		format!(r#""{digest}""#)
	} else {
		format!(r#"W/"{digest}""#)
	};
	HeaderValue::from_str(&tag).expect("BLAKE3 hex digest is a valid header")
}

fn etag_matches(request_etags: Option<&HeaderValue>, response_etag: &HeaderValue) -> bool {
	let Some(request_etags) = request_etags.and_then(|value| value.to_str().ok()) else {
		return false;
	};
	let Some(response_etag) = response_etag.to_str().ok().and_then(normalized_etag_token) else {
		return false;
	};

	request_etags.split(',').any(|candidate| {
		let candidate = candidate.trim();
		candidate == "*"
			|| normalized_etag_token(candidate).is_some_and(|candidate| candidate == response_etag)
	})
}

fn normalized_etag_token(value: &str) -> Option<&str> {
	let value = value.trim();
	let value = value.strip_prefix("W/").unwrap_or(value).trim();
	value.strip_prefix('"')?.strip_suffix('"')
}

fn remove_not_modified_payload_headers(headers: &mut HeaderMap) {
	for &name in NOT_MODIFIED_PAYLOAD_HEADERS {
		headers.remove(name);
	}
}

/////////////////////////////////////////////////////////////////////
/////// Secure Headers
/////////////////////////////////////////////////////////////////////

const PERMISSIONS_POLICY: &str = "accelerometer=(), autoplay=(), camera=(), cross-origin-isolated=(), display-capture=(), encrypted-media=(), fullscreen=(), geolocation=(), gyroscope=(), keyboard-map=(), magnetometer=(), microphone=(), midi=(), payment=(), picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), sync-xhr=(self), usb=(), web-share=(), xr-spatial-tracking=(), clipboard-read=(), clipboard-write=(), gamepad=(), hid=(), idle-detection=(), interest-cohort=(), serial=(), unload=()";

const SECURITY_HEADERS: &[(&str, &str)] = &[
	("cross-origin-embedder-policy", "require-corp"),
	("cross-origin-opener-policy", "same-origin"),
	("cross-origin-resource-policy", "same-origin"),
	("permissions-policy", PERMISSIONS_POLICY),
	("referrer-policy", "no-referrer"),
	(
		"strict-transport-security",
		"max-age=31536000; includeSubDomains",
	),
	("x-content-type-options", "nosniff"),
	("x-frame-options", "deny"),
	("x-permitted-cross-domain-policies", "none"),
];

const STRIPPED_HEADERS: &[&str] = &["server", "x-powered-by"];
const SENSITIVE_HEADERS: &[HeaderName] = &[
	http::header::AUTHORIZATION,
	http::header::COOKIE,
	http::header::PROXY_AUTHORIZATION,
	http::header::SET_COOKIE,
];

/// Tower layer that adds baseline secure headers and strips fingerprinting headers.
#[derive(Clone, Debug, Default)]
pub struct SecureHeadersLayer;

/// Adds a conservative baseline set of secure response headers.
pub fn secure_headers() -> SecureHeadersLayer {
	SecureHeadersLayer
}

impl<S> Layer<S> for SecureHeadersLayer {
	type Service = SecureHeadersService<S>;

	fn layer(&self, inner: S) -> Self::Service {
		SecureHeadersService { inner }
	}
}

/// Service produced by [`SecureHeadersLayer`].
#[derive(Clone, Debug)]
pub struct SecureHeadersService<S> {
	inner: S,
}

impl<S, Req, B> Service<Req> for SecureHeadersService<S>
where
	S: Service<Req, Response = Response<B>>,
	S::Future: Send + 'static,
	S::Error: Send + 'static,
	B: Send + 'static,
{
	type Response = Response<B>;
	type Error = S::Error;
	type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

	fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
		self.inner.poll_ready(cx)
	}

	fn call(&mut self, request: Req) -> Self::Future {
		let response = self.inner.call(request);

		Box::pin(async move {
			let mut response = response.await?;
			apply_secure_headers(&mut response);
			Ok(response)
		})
	}
}

fn apply_secure_headers<B>(response: &mut Response<B>) {
	let headers = response.headers_mut();

	for &(name, value) in SECURITY_HEADERS {
		headers
			.entry(static_header_name(name))
			.or_insert(static_header_value(value));
	}

	for &name in STRIPPED_HEADERS {
		headers.remove(static_header_name(name));
	}
}

fn static_header_name(name: &'static str) -> HeaderName {
	HeaderName::from_static(name)
}

fn static_header_value(value: &'static str) -> HeaderValue {
	HeaderValue::from_static(value)
}

/////////////////////////////////////////////////////////////////////
/////// Tests
/////////////////////////////////////////////////////////////////////

#[cfg(test)]
mod tests {
	use super::*;
	use http::Method;
	use http::StatusCode;
	use http::header::{AUTHORIZATION, CONTENT_LENGTH, CONTENT_TYPE, SET_COOKIE};
	use http_body::{Frame, SizeHint};
	use std::pin::Pin;
	use std::task::{Context, Poll};
	use tower_layer::Layer;
	use tower_service::Service;

	#[derive(Clone)]
	struct TestService;

	impl Service<()> for TestService {
		type Response = Response<()>;
		type Error = std::convert::Infallible;
		type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

		fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
			Poll::Ready(Ok(()))
		}

		fn call(&mut self, _request: ()) -> Self::Future {
			let mut response = Response::new(());
			response.headers_mut().insert(
				HeaderName::from_static("server"),
				HeaderValue::from_static("example"),
			);
			response.headers_mut().insert(
				HeaderName::from_static("x-powered-by"),
				HeaderValue::from_static("example"),
			);
			std::future::ready(Ok(response))
		}
	}

	#[tokio::test]
	async fn secure_headers_sets_policy_and_strips_fingerprinting_headers() {
		let mut service = secure_headers().layer(TestService);
		let response = service.call(()).await.unwrap();

		for &(name, expected) in SECURITY_HEADERS {
			assert_eq!(
				response.headers().get(static_header_name(name)).unwrap(),
				expected
			);
		}
		for &name in STRIPPED_HEADERS {
			assert!(!response.headers().contains_key(static_header_name(name)));
		}
	}

	#[tokio::test]
	async fn secure_headers_does_not_override_existing_policy_headers() {
		#[derive(Clone)]
		struct OverrideService;

		impl Service<()> for OverrideService {
			type Response = Response<()>;
			type Error = std::convert::Infallible;
			type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

			fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
				Poll::Ready(Ok(()))
			}

			fn call(&mut self, _request: ()) -> Self::Future {
				let mut response = Response::new(());
				response.headers_mut().insert(
					HeaderName::from_static("referrer-policy"),
					HeaderValue::from_static("same-origin"),
				);
				std::future::ready(Ok(response))
			}
		}

		let mut service = secure_headers().layer(OverrideService);
		let response = service.call(()).await.unwrap();

		assert_eq!(
			response
				.headers()
				.get(HeaderName::from_static("referrer-policy"))
				.unwrap(),
			"same-origin"
		);
	}

	#[tokio::test]
	async fn secure_headers_preserves_status_and_body() {
		#[derive(Clone)]
		struct BodyService;

		impl Service<()> for BodyService {
			type Response = Response<&'static str>;
			type Error = std::convert::Infallible;
			type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

			fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
				Poll::Ready(Ok(()))
			}

			fn call(&mut self, _request: ()) -> Self::Future {
				std::future::ready(Ok(Response::builder()
					.status(StatusCode::CREATED)
					.body("hello")
					.unwrap()))
			}
		}

		let mut service = secure_headers().layer(BodyService);
		let response = service.call(()).await.unwrap();

		assert_eq!(response.status(), StatusCode::CREATED);
		assert_eq!(response.body(), &"hello");
	}

	#[tokio::test]
	async fn request_id_sets_and_propagates_x_request_id() {
		#[derive(Clone)]
		struct IdService;

		impl Service<Request<()>> for IdService {
			type Response = Response<()>;
			type Error = std::convert::Infallible;
			type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

			fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
				Poll::Ready(Ok(()))
			}

			fn call(&mut self, _request: Request<()>) -> Self::Future {
				std::future::ready(Ok(Response::new(())))
			}
		}

		let mut service = request_id().layer(IdService);
		let response = service.call(Request::new(())).await.unwrap();

		assert!(response.headers().contains_key("x-request-id"));
	}

	#[tokio::test]
	async fn request_id_preserves_incoming_x_request_id() {
		#[derive(Clone)]
		struct IdService;

		impl Service<Request<()>> for IdService {
			type Response = Response<()>;
			type Error = std::convert::Infallible;
			type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

			fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
				Poll::Ready(Ok(()))
			}

			fn call(&mut self, _request: Request<()>) -> Self::Future {
				std::future::ready(Ok(Response::new(())))
			}
		}

		let request = Request::builder()
			.header("x-request-id", "client-id")
			.body(())
			.unwrap();

		let mut service = request_id().layer(IdService);
		let response = service.call(request).await.unwrap();

		assert_eq!(response.headers().get("x-request-id").unwrap(), "client-id");
	}

	#[tokio::test]
	async fn sensitive_headers_marks_request_and_response_values() {
		#[derive(Clone)]
		struct SensitiveService;

		impl Service<Request<()>> for SensitiveService {
			type Response = Response<()>;
			type Error = std::convert::Infallible;
			type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

			fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
				Poll::Ready(Ok(()))
			}

			fn call(&mut self, request: Request<()>) -> Self::Future {
				let request_was_sensitive = request
					.headers()
					.get(AUTHORIZATION)
					.is_some_and(HeaderValue::is_sensitive);
				let mut response = Response::new(());
				response.headers_mut().insert(
					HeaderName::from_static("x-request-was-sensitive"),
					HeaderValue::from_static(if request_was_sensitive {
						"true"
					} else {
						"false"
					}),
				);
				response
					.headers_mut()
					.insert(SET_COOKIE, HeaderValue::from_static("sid=secret"));
				std::future::ready(Ok(response))
			}
		}

		let request = Request::builder()
			.header(AUTHORIZATION, "Bearer secret")
			.body(())
			.unwrap();

		let mut service = sensitive_headers().layer(SensitiveService);
		let response = service.call(request).await.unwrap();

		assert_eq!(
			response
				.headers()
				.get(HeaderName::from_static("x-request-was-sensitive"))
				.unwrap(),
			"true"
		);
		assert!(response.headers().get(SET_COOKIE).unwrap().is_sensitive());
	}

	#[derive(Clone)]
	struct EtagBodyService {
		status: StatusCode,
		body: &'static [u8],
		headers: Vec<(HeaderName, HeaderValue)>,
	}

	impl EtagBodyService {
		fn ok(body: &'static [u8]) -> Self {
			Self {
				status: StatusCode::OK,
				body,
				headers: Vec::new(),
			}
		}

		fn with_status(mut self, status: StatusCode) -> Self {
			self.status = status;
			self
		}

		fn with_header(mut self, name: HeaderName, value: HeaderValue) -> Self {
			self.headers.push((name, value));
			self
		}
	}

	impl Service<Request<()>> for EtagBodyService {
		type Response = Response<Full<Bytes>>;
		type Error = std::convert::Infallible;
		type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

		fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
			Poll::Ready(Ok(()))
		}

		fn call(&mut self, _request: Request<()>) -> Self::Future {
			let mut response = Response::builder()
				.status(self.status)
				.body(Full::new(Bytes::from_static(self.body)))
				.unwrap();
			for (name, value) in &self.headers {
				response.headers_mut().insert(name.clone(), value.clone());
			}
			std::future::ready(Ok(response))
		}
	}

	#[derive(Clone)]
	struct DishonestBodyService;

	struct DishonestBody {
		sent: bool,
	}

	impl http_body::Body for DishonestBody {
		type Data = Bytes;
		type Error = std::convert::Infallible;

		fn poll_frame(
			mut self: Pin<&mut Self>,
			_cx: &mut Context<'_>,
		) -> Poll<Option<Result<Frame<Self::Data>, Self::Error>>> {
			if self.sent {
				return Poll::Ready(None);
			}
			self.sent = true;
			Poll::Ready(Some(Ok(Frame::data(Bytes::from_static(
				b"larger than one byte",
			)))))
		}

		fn size_hint(&self) -> SizeHint {
			SizeHint::with_exact(1)
		}
	}

	impl Service<Request<()>> for DishonestBodyService {
		type Response = Response<DishonestBody>;
		type Error = std::convert::Infallible;
		type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

		fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
			Poll::Ready(Ok(()))
		}

		fn call(&mut self, _request: Request<()>) -> Self::Future {
			std::future::ready(Ok(Response::new(DishonestBody { sent: false })))
		}
	}

	async fn collect_response_body<B>(response: Response<B>) -> Bytes
	where
		B: http_body::Body<Data = Bytes>,
		B::Error: std::fmt::Debug,
	{
		response.into_body().collect().await.unwrap().to_bytes()
	}

	#[tokio::test]
	async fn etag_sets_weak_etag_for_get_ok_body() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().layer(EtagBodyService::ok(b"hello"));

		let response = service.call(request).await.unwrap();

		let tag = response.headers().get(ETAG).unwrap().to_str().unwrap();
		assert!(tag.starts_with("W/\""));
		assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "5");
		assert_eq!(
			collect_response_body(response).await,
			Bytes::from_static(b"hello")
		);
	}

	#[tokio::test]
	async fn etag_can_generate_strong_etag() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().strong().layer(EtagBodyService::ok(b"hello"));

		let response = service.call(request).await.unwrap();

		let tag = response.headers().get(ETAG).unwrap().to_str().unwrap();
		assert!(tag.starts_with('"'));
		assert!(tag.ends_with('"'));
		assert!(!tag.starts_with("W/"));
	}

	#[tokio::test]
	async fn etag_enforces_limit_while_collecting_body() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().max_body_size(1).layer(DishonestBodyService);

		let response = service.call(request).await.unwrap();

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert_eq!(
			collect_response_body(response).await,
			Bytes::from_static(b"response body exceeded ETag limit")
		);
	}

	#[tokio::test]
	async fn etag_returns_not_modified_for_matching_if_none_match() {
		let first_request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().layer(EtagBodyService::ok(b"hello"));
		let first_response = service.call(first_request).await.unwrap();
		let tag = first_response.headers().get(ETAG).unwrap().clone();

		let second_request = Request::builder()
			.method(Method::GET)
			.header(IF_NONE_MATCH, tag)
			.body(())
			.unwrap();
		let mut service = etag().layer(
			EtagBodyService::ok(b"hello")
				.with_header(CONTENT_TYPE, HeaderValue::from_static("text/plain")),
		);

		let response = service.call(second_request).await.unwrap();

		assert_eq!(response.status(), StatusCode::NOT_MODIFIED);
		assert!(response.headers().contains_key(ETAG));
		assert!(!response.headers().contains_key(CONTENT_TYPE));
		assert_eq!(collect_response_body(response).await, Bytes::new());
	}

	#[tokio::test]
	async fn etag_accepts_weak_and_strong_request_matches() {
		let tag = HeaderValue::from_static(r#"W/"abc123""#);
		assert!(etag_matches(
			Some(&HeaderValue::from_static(r#""abc123""#)),
			&tag
		));
		assert!(etag_matches(
			Some(&HeaderValue::from_static(r#"W/"other", "abc123""#)),
			&tag
		));
		assert!(etag_matches(Some(&HeaderValue::from_static("*")), &tag));
	}

	#[tokio::test]
	async fn etag_skip_predicate_can_bypass_etag() {
		let request = Request::builder()
			.method(Method::GET)
			.uri("/skip-me")
			.header("x-skip-etag", "true")
			.body(())
			.unwrap();
		let mut service = etag()
			.skip(|request| {
				request.uri().path() == "/skip-me" && request.headers().contains_key("x-skip-etag")
			})
			.layer(EtagBodyService::ok(b"hello"));

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
		assert_eq!(
			collect_response_body(response).await,
			Bytes::from_static(b"hello")
		);
	}

	#[tokio::test]
	async fn etag_skips_non_get_and_head_requests() {
		let request = Request::builder().method(Method::POST).body(()).unwrap();
		let mut service = etag().layer(EtagBodyService::ok(b"hello"));

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
		assert_eq!(
			collect_response_body(response).await,
			Bytes::from_static(b"hello")
		);
	}

	#[tokio::test]
	async fn etag_skips_non_ok_response() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag()
			.layer(EtagBodyService::ok(b"hello").with_status(StatusCode::INTERNAL_SERVER_ERROR));

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
	}

	#[tokio::test]
	async fn etag_skips_no_store_response() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().layer(
			EtagBodyService::ok(b"hello")
				.with_header(CACHE_CONTROL, HeaderValue::from_static("public, no-store")),
		);

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
	}

	#[tokio::test]
	async fn etag_skips_set_cookie_response() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().layer(
			EtagBodyService::ok(b"hello")
				.with_header(SET_COOKIE, HeaderValue::from_static("sid=1")),
		);

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
	}

	#[tokio::test]
	async fn etag_skips_body_over_limit() {
		let request = Request::builder().method(Method::GET).body(()).unwrap();
		let mut service = etag().max_body_size(3).layer(EtagBodyService::ok(b"hello"));

		let response = service.call(request).await.unwrap();

		assert!(!response.headers().contains_key(ETAG));
		assert_eq!(
			collect_response_body(response).await,
			Bytes::from_static(b"hello")
		);
	}
}
