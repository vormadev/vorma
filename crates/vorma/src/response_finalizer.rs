//! Typed route response finalization.

use crate::contracts::DocumentContract;
use crate::document_renderer::{DocumentRenderError, DocumentRenderInput, render_document};
use crate::head::{HEAD_TAG_META, HEAD_TAG_TITLE};
use bytes::Bytes;
use cookie::Cookie;
use http::header::{
	CACHE_CONTROL, CONTENT_LENGTH, CONTENT_TYPE, HeaderName, HeaderValue, LOCATION, SET_COOKIE,
	TRANSFER_ENCODING,
};
use http::{HeaderMap, Response, StatusCode, Uri};

pub use vorma_contract::wire::{
	BUILD_SKEW_HEADER, CLIENT_ACCEPTS_REDIRECT_HEADER, CLIENT_BUILD_ID_HEADER,
	CLIENT_REDIRECT_HEADER, HeadElement, SsrPayload, VORMA_JSON_QUERY_KEY, ViewPayload,
};

const CLIENT_BUILD_ID_HEADER_NAME: HeaderName = HeaderName::from_static("x-vorma-client-build-id");
const BUILD_SKEW_HEADER_NAME: HeaderName = HeaderName::from_static("x-vorma-build-skew");
const CLIENT_REDIRECT_HEADER_NAME: HeaderName = HeaderName::from_static("x-client-redirect");
const CLIENT_ACCEPTS_REDIRECT_HEADER_NAME: HeaderName =
	HeaderName::from_static("x-accepts-client-redirect");
/// Default JSON response content type.
pub const JSON_CONTENT_TYPE: &str = "application/json; charset=utf-8";
/// Default plain text terminal response content type.
pub const TEXT_CONTENT_TYPE: &str = "text/plain; charset=utf-8";
/// Fixed JSON body used for stale view build-skew responses.
pub const BUILD_SKEW_RESPONSE_BODY: &[u8] = br#"{"ok":true}"#;
/// Default internal server error status text.
pub const INTERNAL_SERVER_ERROR_STATUS_TEXT: &str = "Internal Server Error";
const HTML_CONTENT_TYPE: &str = "text/html; charset=utf-8";
const PRIVATE_NO_STORE_CACHE_CONTROL: &str = "no-store";
const VIEW_RESPONSE_CACHE_CONTROL: &str = "private, max-age=0, must-revalidate, no-cache";

/// Typed response effects collected during route execution.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct ResponseEffects {
	status: Option<StatusCode>,
	status_text: Option<String>,
	/*
	Engine-owned: only the framework's error finalization sets this (via
	set_status_with_text). Handler statuses are NEVER control flow — a 2xx
	override or even a hypothetical 4xx via internal paths does not abort
	body production unless the engine says so.
	*/
	terminal_error: bool,
	location: Option<String>,
	client_redirect_location: Option<String>,
	header_ops: Vec<HeaderOp>,
	cookies: Vec<ResponseCookie>,
	title: Option<HeadElement>,
	meta_head_elements: Vec<HeadElement>,
	rest_head_elements: Vec<HeadElement>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct HeaderOp {
	kind: HeaderOpKind,
	key: HeaderName,
	value: HeaderValue,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum HeaderOpKind {
	Set,
	Add,
}

impl ResponseEffects {
	/// Set HTTP status.
	pub fn set_status(&mut self, status: StatusCode) {
		self.status = Some(status);
		self.status_text = None;
		self.terminal_error = false;
		if !redirect_status(status) {
			self.location = None;
		}
		if status.as_u16() >= 400 {
			self.client_redirect_location = None;
		}
	}

	/// Set HTTP status and terminal error response text.
	pub fn set_status_with_text(&mut self, status: StatusCode, status_text: impl Into<String>) {
		self.status = Some(status);
		self.status_text = Some(status_text.into());
		self.terminal_error = true;
		if !redirect_status(status) {
			self.location = None;
		}
		if status.as_u16() >= 400 {
			self.client_redirect_location = None;
		}
	}

	/// Set redirect location and status.
	pub fn redirect(
		&mut self,
		status: StatusCode,
		location: impl Into<String>,
	) -> Result<(), ResponseEffectsError> {
		if !redirect_status(status) {
			return Err(ResponseEffectsError::InvalidRedirectStatus { status });
		}
		let location = location.into();
		if !valid_redirect_location(&location) {
			return Err(ResponseEffectsError::InvalidRedirectLocation { location });
		}
		if self.terminal_error {
			return Ok(());
		}
		self.status = Some(status);
		self.status_text = None;
		self.terminal_error = false;
		self.location = Some(location);
		self.client_redirect_location = None;
		Ok(())
	}

	/// Set a client-side redirect location and return without a native HTTP redirect.
	pub fn client_redirect(
		&mut self,
		location: impl Into<String>,
	) -> Result<(), ResponseEffectsError> {
		let location = location.into();
		if !valid_redirect_location(&location) {
			return Err(ResponseEffectsError::InvalidRedirectLocation { location });
		}
		if self.terminal_error {
			return Ok(());
		}
		if self.status.is_none() {
			self.status = Some(StatusCode::OK);
		}
		self.status_text = None;
		self.terminal_error = false;
		self.location = None;
		self.client_redirect_location = Some(location);
		Ok(())
	}

	/// Redirect using the browser client redirect contract when the request accepts it.
	pub fn redirect_with_client_preference(
		&mut self,
		accepts_client_redirect: bool,
		location: impl Into<String>,
		status: Option<StatusCode>,
	) -> Result<bool, ResponseEffectsError> {
		let status = status.unwrap_or(StatusCode::SEE_OTHER);
		if !redirect_status(status) {
			return Err(ResponseEffectsError::InvalidRedirectStatus { status });
		}
		if accepts_client_redirect {
			self.client_redirect(location)?;
			return Ok(true);
		}
		self.redirect(status, location)?;
		Ok(false)
	}

	/// Add or replace a header.
	pub fn set_header(&mut self, key: HeaderName, value: HeaderValue) {
		self.header_ops.push(HeaderOp {
			kind: HeaderOpKind::Set,
			key,
			value,
		});
	}

	/// Add another header value.
	pub fn add_header(&mut self, key: HeaderName, value: HeaderValue) {
		self.header_ops.push(HeaderOp {
			kind: HeaderOpKind::Add,
			key,
			value,
		});
	}

	/// Set a cookie response effect.
	pub fn set_cookie(&mut self, cookie: Cookie<'static>) {
		let cookie = ResponseCookie::from_cookie(cookie);
		self.merge_cookie(cookie);
	}

	fn merge_cookie(&mut self, cookie: ResponseCookie) {
		if let Some(index) = self
			.cookies
			.iter()
			.position(|existing| existing.name() == cookie.name())
		{
			self.cookies.remove(index);
		}
		self.cookies.push(cookie);
	}

	/// Set the document title element.
	pub fn set_title(&mut self, title: HeadElement) {
		self.title = Some(title);
	}

	/// Add a prepared meta head element.
	pub fn add_meta_head_element(&mut self, element: HeadElement) {
		self.meta_head_elements.push(element);
	}

	/// Add a prepared non-meta head element.
	pub fn add_rest_head_element(&mut self, element: HeadElement) {
		self.rest_head_elements.push(element);
	}

	/// Route one prepared head element into its effect slot by tag.
	pub(crate) fn apply_head_element(&mut self, prepared: HeadElement) {
		if prepared.tag == HEAD_TAG_META {
			self.add_meta_head_element(prepared);
		} else if prepared.tag == HEAD_TAG_TITLE {
			self.set_title(prepared);
		} else {
			self.add_rest_head_element(prepared);
		}
	}

	/// Whether these effects terminate normal response body projection.
	pub fn is_terminal(&self) -> bool {
		self.terminal_error
			|| self
				.status
				.is_some_and(|status| redirect_status(status) && self.location.is_some())
			|| self.client_redirect_location.is_some()
	}

	/// Whether the terminal state is an engine-finalized error (vs a redirect).
	pub fn is_terminal_error(&self) -> bool {
		self.terminal_error
	}

	/// Merge later handler effects into this accumulated effect set.
	pub fn merge_from(&mut self, other: &Self) {
		if other.status.is_some() {
			self.status = other.status;
			self.status_text.clone_from(&other.status_text);
			self.terminal_error = other.terminal_error;
			if self.status.is_some_and(|status| !redirect_status(status)) {
				self.location = None;
			}
			if self.status.is_some_and(|status| status.as_u16() >= 400) {
				self.client_redirect_location = None;
			}
		}
		if other.location.is_some() {
			self.location.clone_from(&other.location);
			self.client_redirect_location = None;
		}
		if other.client_redirect_location.is_some() {
			self.client_redirect_location
				.clone_from(&other.client_redirect_location);
			self.location = None;
		}
		self.header_ops.extend(other.header_ops.iter().cloned());
		for cookie in &other.cookies {
			self.merge_cookie(cookie.clone());
		}
		if other.title.is_some() {
			self.title.clone_from(&other.title);
		}
		self.meta_head_elements
			.extend(other.meta_head_elements.iter().cloned());
		self.rest_head_elements
			.extend(other.rest_head_elements.iter().cloned());
	}

	/// Current status effect.
	pub fn status(&self) -> Option<StatusCode> {
		self.status
	}

	/// Current status text effect.
	pub fn status_text(&self) -> Option<&str> {
		self.status_text.as_deref()
	}

	/// Current redirect location effect.
	pub fn location(&self) -> Option<&str> {
		self.location.as_deref()
	}

	/// Current client-side redirect location effect.
	pub fn client_redirect_location(&self) -> Option<&str> {
		self.client_redirect_location.as_deref()
	}

	/// Current title element effect.
	pub fn title(&self) -> Option<&HeadElement> {
		self.title.as_ref()
	}

	/// Current meta head element effects.
	pub fn meta_head_elements(&self) -> &[HeadElement] {
		&self.meta_head_elements
	}

	/// Current non-meta head element effects.
	pub fn rest_head_elements(&self) -> &[HeadElement] {
		&self.rest_head_elements
	}

	/// Current cookie effects after identity dedupe.
	pub fn cookies(&self) -> &[ResponseCookie] {
		&self.cookies
	}
}

/// Comparable committed cookie response effect.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResponseCookie {
	name: String,
	value: String,
	domain: Option<String>,
	path: Option<String>,
	header_value: String,
}

impl ResponseCookie {
	fn from_cookie(cookie: Cookie<'static>) -> Self {
		Self {
			name: cookie.name().to_owned(),
			value: cookie.value().to_owned(),
			domain: cookie.domain().map(ToOwned::to_owned),
			path: cookie.path().map(ToOwned::to_owned),
			header_value: cookie.to_string(),
		}
	}

	/// Cookie name.
	pub fn name(&self) -> &str {
		&self.name
	}

	/// Cookie value.
	pub fn value(&self) -> &str {
		&self.value
	}

	/// Cookie domain identity, when set.
	pub fn domain(&self) -> Option<&str> {
		self.domain.as_deref()
	}

	/// Cookie path identity, when set.
	pub fn path(&self) -> Option<&str> {
		self.path.as_deref()
	}

	fn header_value(&self) -> &str {
		&self.header_value
	}
}

/// Response effect construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ResponseEffectsError {
	/// Redirect status was not a 3xx status code.
	InvalidRedirectStatus {
		/// Rejected status code.
		status: StatusCode,
	},
	/// Redirect location was not a safe server redirect target.
	InvalidRedirectLocation {
		/// Rejected location.
		location: String,
	},
}

/// Finalize an HTTP response from a body and typed effects.
pub fn finalize_response(
	body: Bytes,
	effects: &ResponseEffects,
	client_build_id: &str,
) -> Result<Response<Bytes>, FinalizerError> {
	let mut response = Response::new(body);
	if let Some(status) = effects.status {
		*response.status_mut() = status;
	}
	for op in &effects.header_ops {
		match op.kind {
			HeaderOpKind::Set => {
				response.headers_mut().remove(&op.key);
				response.headers_mut().append(&op.key, op.value.clone());
			}
			HeaderOpKind::Add => {
				response.headers_mut().append(&op.key, op.value.clone());
			}
		}
	}
	for cookie in &effects.cookies {
		let value =
			HeaderValue::from_str(cookie.header_value()).map_err(|_| FinalizerError::Cookie {
				cookie_name: cookie.name().to_owned(),
			})?;
		response.headers_mut().append(SET_COOKIE, value);
	}
	if let Some(location) = &effects.location
		&& effects.status.is_some_and(redirect_status)
	{
		insert_header(response.headers_mut(), LOCATION, location)?;
	}
	if let Some(location) = &effects.client_redirect_location {
		insert_header(
			response.headers_mut(),
			CLIENT_REDIRECT_HEADER_NAME,
			location,
		)?;
	}
	insert_header(
		response.headers_mut(),
		CLIENT_BUILD_ID_HEADER_NAME,
		client_build_id,
	)?;
	Ok(response)
}

/// Whether a request accepts the browser client redirect response contract.
pub fn accepts_client_redirect(headers: &HeaderMap) -> bool {
	let Some(value) = headers.get(CLIENT_ACCEPTS_REDIRECT_HEADER_NAME) else {
		return false;
	};
	let Ok(value) = value.to_str() else {
		return false;
	};
	matches!(value, "1" | "t" | "T" | "TRUE" | "true" | "True")
}

/// Finalize a terminal route response without projecting a normal route body.
pub fn finalize_terminal_response(
	effects: &ResponseEffects,
	client_build_id: &str,
) -> Result<Response<Bytes>, FinalizerError> {
	let (body, text) = terminal_response_body(effects);
	let mut response = finalize_response(body, effects, client_build_id)?;
	if text {
		response
			.headers_mut()
			.insert(CONTENT_TYPE, HeaderValue::from_static(TEXT_CONTENT_TYPE));
	}
	Ok(response)
}

/// Suppress a HEAD response body while preserving the would-have-been body length.
pub fn suppress_response_body_preserving_content_length(
	response: Response<Bytes>,
) -> Result<Response<Bytes>, FinalizerError> {
	let (mut parts, body) = response.into_parts();
	if !parts.headers.contains_key(CONTENT_LENGTH) && !parts.headers.contains_key(TRANSFER_ENCODING)
	{
		insert_header(&mut parts.headers, CONTENT_LENGTH, &body.len().to_string())?;
	}
	Ok(Response::from_parts(parts, Bytes::new()))
}

/// Finalize a JSON view payload response.
pub fn finalize_view_json_response(
	payload: &ViewPayload,
	effects: &ResponseEffects,
	client_build_id: &str,
) -> Result<Response<Bytes>, FinalizerError> {
	let body = serde_json::to_vec(payload).map_err(|error| FinalizerError::Json {
		message: error.to_string(),
	})?;
	let mut response = finalize_response(Bytes::from(body), effects, client_build_id)?;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(JSON_CONTENT_TYPE));
	insert_default_view_cache_control(response.headers_mut());
	Ok(response)
}

/// Finalize a server-rendered HTML view response.
pub fn finalize_view_html_response(
	document: &DocumentContract,
	input: DocumentRenderInput<'_>,
	effects: &ResponseEffects,
	client_build_id: &str,
) -> Result<Response<Bytes>, FinalizerError> {
	let html =
		render_document(document, input).map_err(|source| FinalizerError::Document { source })?;
	let mut response = finalize_response(Bytes::from(html), effects, client_build_id)?;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(HTML_CONTENT_TYPE));
	insert_default_view_cache_control(response.headers_mut());
	Ok(response)
}

/// Render the JSON body embedded in an SSR document.
pub fn ssr_payload_json(payload: &SsrPayload) -> Result<String, FinalizerError> {
	let json = serde_json::to_string(payload).map_err(|error| FinalizerError::Json {
		message: error.to_string(),
	})?;
	Ok(escape_ssr_payload_json(&json))
}

fn escape_ssr_payload_json(json: &str) -> String {
	let mut escaped = String::with_capacity(json.len());
	for ch in json.chars() {
		match ch {
			'&' => escaped.push_str("\\u0026"),
			'<' => escaped.push_str("\\u003c"),
			'>' => escaped.push_str("\\u003e"),
			'\u{2028}' => escaped.push_str("\\u2028"),
			'\u{2029}' => escaped.push_str("\\u2029"),
			_ => escaped.push(ch),
		}
	}
	escaped
}

/// Return a build-skew JSON response when a submitted route build id is stale.
pub fn finalize_view_build_skew_response(
	submitted_client_build_id: Option<&str>,
	expected_client_build_id: &str,
) -> Result<Option<Response<Bytes>>, FinalizerError> {
	if submitted_client_build_id.is_none()
		|| submitted_client_build_id == Some(expected_client_build_id)
		|| submitted_client_build_id == Some("_")
	{
		return Ok(None);
	}
	let mut response = finalize_response(
		Bytes::from_static(BUILD_SKEW_RESPONSE_BODY),
		&ResponseEffects::default(),
		expected_client_build_id,
	)?;
	response
		.headers_mut()
		.insert(CONTENT_TYPE, HeaderValue::from_static(JSON_CONTENT_TYPE));
	insert_header(response.headers_mut(), BUILD_SKEW_HEADER_NAME, "1")?;
	response.headers_mut().insert(
		CACHE_CONTROL,
		HeaderValue::from_static(PRIVATE_NO_STORE_CACHE_CONTROL),
	);
	Ok(Some(response))
}

/// Response finalization error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FinalizerError {
	/// Header value was invalid.
	InvalidHeaderValue {
		/// Rejected header value.
		value: String,
	},
	/// JSON serialization failed.
	Json {
		/// JSON serialization error message.
		message: String,
	},
	/// Document rendering failed.
	Document {
		/// Source document rendering error.
		source: DocumentRenderError,
	},
	/// Cookie serialization produced an invalid header value.
	Cookie {
		/// Rejected cookie name.
		cookie_name: String,
	},
}

fn insert_header(
	headers: &mut HeaderMap,
	key: HeaderName,
	value: &str,
) -> Result<(), FinalizerError> {
	let value = HeaderValue::from_str(value).map_err(|_| FinalizerError::InvalidHeaderValue {
		value: value.to_owned(),
	})?;
	headers.insert(key, value);
	Ok(())
}

fn insert_default_view_cache_control(headers: &mut HeaderMap) {
	if !headers.contains_key(CACHE_CONTROL) {
		headers.insert(
			CACHE_CONTROL,
			HeaderValue::from_static(VIEW_RESPONSE_CACHE_CONTROL),
		);
	}
}

fn terminal_response_body(effects: &ResponseEffects) -> (Bytes, bool) {
	let Some(status) = effects.status else {
		return (Bytes::new(), false);
	};
	if status.as_u16() < 400 {
		return (Bytes::new(), false);
	}
	let status_text = effects.status_text().unwrap_or_default();
	(Bytes::from(format!("{status_text}\n")), true)
}

fn redirect_status(status: StatusCode) -> bool {
	let status = status.as_u16();
	(300..400).contains(&status)
}

fn valid_redirect_location(location: &str) -> bool {
	if location.is_empty() || location.starts_with("//") || location.contains('\\') {
		return false;
	}
	if let Ok(url) = url::Url::parse(location) {
		return matches!(url.scheme(), "http" | "https") && url.host_str().is_some();
	}
	let Ok(uri) = location.parse::<Uri>() else {
		return false;
	};
	uri.scheme_str().is_none() && uri.authority().is_none()
}

#[cfg(test)]
mod tests {
	use serde_json::Value;

	use super::*;
	use crate::contracts::{DocumentAttributeContract, DocumentElementContract};

	#[test]
	fn finalizer_applies_status_headers_cookies_redirect_and_build_id() {
		let mut effects = ResponseEffects::default();
		effects.redirect(StatusCode::SEE_OTHER, "/target").unwrap();
		effects.set_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("ok"),
		);
		effects.add_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("also"),
		);
		effects.set_cookie(Cookie::new("session", "abc"));

		let response = finalize_response(Bytes::new(), &effects, "build-id").unwrap();

		assert_eq!(response.status(), StatusCode::SEE_OTHER);
		assert_eq!(response.headers()["location"], "/target");
		assert_eq!(
			response
				.headers()
				.get_all(HeaderName::from_static("x-test"))
				.iter()
				.map(|value| value.to_str().unwrap())
				.collect::<Vec<_>>(),
			["ok", "also"]
		);
		assert_eq!(response.headers()["set-cookie"], "session=abc");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
	}

	#[test]
	fn merged_set_header_replaces_prior_values_before_later_adds_append() {
		let mut middleware = ResponseEffects::default();
		middleware.add_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("middleware"),
		);
		let mut handler = ResponseEffects::default();
		handler.set_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("handler"),
		);
		handler.add_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("extra"),
		);
		middleware.merge_from(&handler);

		let response = finalize_response(Bytes::new(), &middleware, "build-id").unwrap();

		assert_eq!(
			response
				.headers()
				.get_all(HeaderName::from_static("x-test"))
				.iter()
				.map(|value| value.to_str().unwrap())
				.collect::<Vec<_>>(),
			["handler", "extra"]
		);
	}

	#[test]
	fn finalizer_merges_cookies_by_name_with_later_values() {
		let mut first = ResponseEffects::default();
		first.set_cookie(Cookie::build(("session", "old")).path("/").build());
		first.set_cookie(Cookie::build(("theme", "dark")).build());
		let mut second = ResponseEffects::default();
		second.set_cookie(Cookie::build(("session", "new")).path("/").build());
		second.set_cookie(Cookie::build(("session", "nested")).path("/nested").build());

		first.merge_from(&second);

		assert_eq!(first.cookies().len(), 2);
		assert_eq!(first.cookies()[0].name(), "theme");
		assert_eq!(first.cookies()[1].name(), "session");
		assert_eq!(first.cookies()[1].value(), "nested");
		assert_eq!(first.cookies()[1].path(), Some("/nested"));
	}

	#[test]
	fn finalizer_protects_framework_client_build_id_header() {
		let mut effects = ResponseEffects::default();
		effects.set_header(
			CLIENT_BUILD_ID_HEADER_NAME,
			HeaderValue::from_static("handler-build-id"),
		);
		effects.add_header(
			CLIENT_BUILD_ID_HEADER_NAME,
			HeaderValue::from_static("extra-build-id"),
		);

		let response = finalize_response(Bytes::new(), &effects, "framework-build-id").unwrap();

		assert_eq!(
			response.headers()[CLIENT_BUILD_ID_HEADER],
			"framework-build-id"
		);
		assert_eq!(
			response
				.headers()
				.get_all(CLIENT_BUILD_ID_HEADER_NAME)
				.iter()
				.count(),
			1
		);
	}

	#[test]
	fn finalizer_short_circuits_terminal_errors_and_redirects() {
		let mut error_effects = ResponseEffects::default();
		error_effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
		error_effects.set_header(
			HeaderName::from_static("x-terminal"),
			HeaderValue::from_static("1"),
		);
		let mut redirect_effects = ResponseEffects::default();
		redirect_effects
			.redirect(StatusCode::FOUND, "/login")
			.unwrap();

		let error_response = finalize_terminal_response(&error_effects, "build-id").unwrap();
		let redirect_response = finalize_terminal_response(&redirect_effects, "build-id").unwrap();

		assert_eq!(error_response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(error_response.headers()["x-terminal"], "1");
		assert_eq!(error_response.headers()[CONTENT_TYPE], TEXT_CONTENT_TYPE);
		assert_eq!(error_response.body(), &Bytes::from_static(b"bad input\n"));
		assert_eq!(redirect_response.status(), StatusCode::FOUND);
		assert_eq!(redirect_response.headers()[LOCATION], "/login");
		assert!(redirect_response.body().is_empty());
		assert!(!redirect_response.headers().contains_key(CONTENT_TYPE));
	}

	#[test]
	fn finalizer_short_circuits_client_redirects_with_contract_header() {
		let mut effects = ResponseEffects::default();
		let used_client_redirect = effects
			.redirect_with_client_preference(true, "/target", None)
			.unwrap();

		let response = finalize_terminal_response(&effects, "build-id").unwrap();

		assert!(used_client_redirect);
		assert_eq!(effects.status(), Some(StatusCode::OK));
		assert_eq!(effects.client_redirect_location(), Some("/target"));
		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CLIENT_REDIRECT_HEADER], "/target");
		assert!(!response.headers().contains_key(LOCATION));
		assert!(response.body().is_empty());
	}

	#[test]
	fn finalizer_redirect_with_client_preference_preserves_error_priority() {
		let mut effects = ResponseEffects::default();
		effects.set_status_with_text(StatusCode::FORBIDDEN, "denied");

		let used_client_redirect = effects
			.redirect_with_client_preference(true, "/login", None)
			.unwrap();
		let response = finalize_terminal_response(&effects, "build-id").unwrap();

		assert!(used_client_redirect);
		assert_eq!(effects.status(), Some(StatusCode::FORBIDDEN));
		assert_eq!(effects.client_redirect_location(), None);
		assert_eq!(response.status(), StatusCode::FORBIDDEN);
		assert!(!response.headers().contains_key(CLIENT_REDIRECT_HEADER));
		assert_eq!(response.body(), &Bytes::from_static(b"denied\n"));
	}

	#[test]
	fn finalizer_rejects_invalid_redirect_effects() {
		let mut effects = ResponseEffects::default();

		let invalid_status = effects.redirect(StatusCode::OK, "/target").unwrap_err();
		let invalid_location = effects
			.redirect(StatusCode::FOUND, "javascript:alert(1)")
			.unwrap_err();
		effects
			.redirect(StatusCode::FOUND, "https://example.com/target")
			.unwrap();

		assert!(matches!(
			invalid_status,
			ResponseEffectsError::InvalidRedirectStatus { .. }
		));
		assert!(matches!(
			invalid_location,
			ResponseEffectsError::InvalidRedirectLocation { .. }
		));
		assert_eq!(effects.location(), Some("https://example.com/target"));
	}

	#[test]
	fn finalizer_non_redirect_status_clears_server_redirect_location() {
		let mut effects = ResponseEffects::default();
		effects.redirect(StatusCode::FOUND, "/target").unwrap();
		effects.set_status(StatusCode::CREATED);

		let response = finalize_response(Bytes::new(), &effects, "build-id").unwrap();

		assert!(!effects.is_terminal());
		assert_eq!(effects.location(), None);
		assert_eq!(response.status(), StatusCode::CREATED);
		assert!(!response.headers().contains_key(LOCATION));
	}

	#[test]
	fn finalizer_parses_client_redirect_acceptance_header() {
		let mut headers = HeaderMap::new();
		assert!(!accepts_client_redirect(&headers));

		headers.insert(
			CLIENT_ACCEPTS_REDIRECT_HEADER_NAME,
			HeaderValue::from_static("true"),
		);

		assert!(accepts_client_redirect(&headers));
	}

	#[test]
	fn finalizer_builds_view_json_payload_with_client_build_header() {
		let payload = ViewPayload {
			matched_patterns: vec!["/".to_owned()],
			title: Some(HeadElement {
				tag: "title".to_owned(),
				dangerous_inner_html: "Home".to_owned(),
				..HeadElement::default()
			}),
			views_data: vec![serde_json::json!({"loaded": true})],
			..ViewPayload::default()
		};

		let response =
			finalize_view_json_response(&payload, &ResponseEffects::default(), "build-id").unwrap();
		let body: Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(
			response.headers()[CACHE_CONTROL],
			VIEW_RESPONSE_CACHE_CONTROL
		);
		assert!(body.get("client_build_id").is_none());
		assert_eq!(body["matched_patterns"], serde_json::json!(["/"]));
		assert_eq!(body["title"]["dangerous_inner_html"], "Home");
		assert_eq!(body["views_data"], serde_json::json!([{"loaded": true}]));
	}

	#[test]
	fn finalizer_preserves_explicit_view_cache_control() {
		let mut effects = ResponseEffects::default();
		effects.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));

		let response =
			finalize_view_json_response(&ViewPayload::default(), &effects, "build-id").unwrap();

		assert_eq!(response.headers()[CACHE_CONTROL], "no-store");
		assert_eq!(response.headers().get_all(CACHE_CONTROL).iter().count(), 1);
	}

	#[test]
	fn finalizer_builds_html_response_with_document_contract() {
		let document = DocumentContract::new(
			vec![DocumentAttributeContract::new("lang", "en", false, false)],
			Vec::new(),
			vec![DocumentElementContract::new("title").with_text_content("Home")],
			Vec::new(),
			Vec::new(),
		);

		let response = finalize_view_html_response(
			&document,
			DocumentRenderInput::new("", "<div id=\"app\"></div>\n"),
			&ResponseEffects::default(),
			"build-id",
		)
		.unwrap();
		let html = String::from_utf8(response.body().to_vec()).unwrap();

		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.headers()[CONTENT_TYPE], HTML_CONTENT_TYPE);
		assert_eq!(
			response.headers()[CACHE_CONTROL],
			VIEW_RESPONSE_CACHE_CONTROL
		);
		assert!(html.contains("<html lang=\"en\">"));
		assert!(html.contains("<title>Home</title>"));
		assert!(html.contains("<div id=\"app\"></div>\n</body>"));
	}

	#[test]
	fn finalizer_escapes_ssr_payload_json_for_html_script_embedding() {
		let payload = SsrPayload {
			client_build_id: "build-id".to_owned(),
			view_payload: ViewPayload {
				views_data: vec![serde_json::json!({"html": "</script>&>\u{2028}\u{2029}"})],
				..ViewPayload::default()
			},
			..SsrPayload::default()
		};

		let json = ssr_payload_json(&payload).unwrap();

		assert!(json.contains("\\u003c/script\\u003e\\u0026"));
		assert!(json.contains("\\u003e"));
		assert!(json.contains("\\u2028"));
		assert!(json.contains("\\u2029"));
		assert!(!json.contains("</script>"));
	}

	#[test]
	fn finalizer_build_skew_response_preserves_contract_headers() {
		let response = finalize_view_build_skew_response(Some("old-build"), "new-build")
			.unwrap()
			.unwrap();
		let bypass = finalize_view_build_skew_response(Some("_"), "new-build").unwrap();

		assert!(bypass.is_none());
		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "new-build");
		assert_eq!(response.headers()[BUILD_SKEW_HEADER], "1");
		assert_eq!(
			response.headers()[CACHE_CONTROL],
			PRIVATE_NO_STORE_CACHE_CONTROL
		);
		assert_eq!(
			response.body(),
			&Bytes::from_static(BUILD_SKEW_RESPONSE_BODY)
		);
	}
}
