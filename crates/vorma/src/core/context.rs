use cookie::Cookie;
use http::{HeaderName, HeaderValue, StatusCode};
use std::sync::{Arc, Mutex, MutexGuard};
use vorma_matcher::Params;
use vorma_tasks::ExecCtx;

use crate::head;
use crate::mux::{None, RequestCtx};
use crate::request::HttpRequest;
use crate::response::{CLIENT_ACCEPTS_REDIRECT_HEADER, Proxy};

/// Route parameters exposed to route-generic task middleware.
#[derive(Clone, Copy, Debug)]
pub struct RouteParams<'a> {
	inner: &'a Params,
}

impl<'a> RouteParams<'a> {
	/// Return the value for a named parameter.
	pub fn get(&self, key: &str) -> Option<&'a str> {
		self.inner.get(key).map(String::as_str)
	}

	/// Iterate over parameter name/value pairs.
	pub fn iter(&self) -> impl Iterator<Item = (&'a str, &'a str)> {
		self.inner
			.iter()
			.map(|(key, value)| (key.as_str(), value.as_str()))
	}

	/// Number of captured parameters.
	pub fn len(&self) -> usize {
		self.inner.len()
	}

	/// Whether no parameters were captured.
	pub fn is_empty(&self) -> bool {
		self.inner.is_empty()
	}
}

/// Context passed to API-route handlers.
pub struct ApiCtx<S, E, I, P = ()> {
	inner: RequestCtx<S, E, I>,
	params: P,
}

impl<S, E, I, P> ApiCtx<S, E, I, P> {
	pub(crate) fn new(inner: RequestCtx<S, E, I>, params: P) -> Self {
		Self { inner, params }
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Parsed API-route input.
	pub fn input(&self) -> &I {
		self.inner.input()
	}

	/// Typed route parameters for macro-generated parameter structs.
	pub fn params(&self) -> &P {
		&self.params
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &[String] {
		self.inner.splat_values()
	}

	/// Current HTTP request.
	pub fn request(&self) -> HttpRequest<'_> {
		HttpRequest::new(self.inner.request())
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<E> {
		self.inner.exec_ctx()
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner.public_url(src_path)
	}

	/// Mutable response-effect handle for status, headers, cookies, and redirects.
	pub fn response(&self) -> ResponseHandle<'_> {
		response_handle_for(&self.inner)
	}
}

/// Context passed to view loader handlers.
pub struct ViewCtx<S, E, I, P = ()> {
	inner: RequestCtx<S, E, I>,
	params: P,
}

impl<S, E, I, P> ViewCtx<S, E, I, P> {
	pub(crate) fn new(inner: RequestCtx<S, E, I>, params: P) -> Self {
		Self { inner, params }
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Parsed view input.
	pub fn input(&self) -> &I {
		self.inner.input()
	}

	/// Typed route parameters for macro-generated parameter structs.
	pub fn params(&self) -> &P {
		&self.params
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &[String] {
		self.inner.splat_values()
	}

	/// Current HTTP request.
	pub fn request(&self) -> HttpRequest<'_> {
		HttpRequest::new(self.inner.request())
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<E> {
		self.inner.exec_ctx()
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner.public_url(src_path)
	}

	/// Mutable response-effect handle for status, headers, cookies, and redirects.
	pub fn response(&self) -> ResponseHandle<'_> {
		response_handle_for(&self.inner)
	}

	/// Mutable head-effect handle for this view.
	pub fn head(&self) -> HeadHandle {
		head_handle_for(&self.inner)
	}
}

/// Mutable response-effect handle.
pub struct ResponseHandle<'a> {
	proxy: Arc<Mutex<Proxy>>,
	request_headers: &'a http::HeaderMap,
}

impl ResponseHandle<'_> {
	/// Set the handler response status.
	pub fn set_status(&mut self, status: StatusCode) -> &mut Self {
		self.proxy()
			.set_status(status, std::option::Option::<String>::None);
		self
	}

	/// Set an error status and client-visible plain-text body.
	pub fn set_error_status(&mut self, status: StatusCode, text: impl Into<String>) -> &mut Self {
		assert!(
			status.as_u16() >= 400,
			"set_error_status requires an error status, got {status}"
		);
		self.proxy().set_status(status, Some(text.into()));
		self
	}

	/// Set a response header, replacing prior values with the same name.
	pub fn set_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.proxy().set_header(key, value);
		self
	}

	/// Append a response header value.
	pub fn append_header(&mut self, key: HeaderName, value: HeaderValue) -> &mut Self {
		self.proxy().add_header(key, value);
		self
	}

	/// Set a response cookie.
	pub fn set_cookie(&mut self, cookie: Cookie<'static>) -> &mut Self {
		self.proxy().set_cookie(cookie);
		self
	}

	/// Redirect using the default server redirect status.
	pub fn redirect(&mut self, location: impl AsRef<str>) -> Result<&mut Self, String> {
		self.redirect_with_status(location, Option::None)
	}

	/// Redirect using an explicit 3xx status when valid.
	pub fn redirect_with_status(
		&mut self,
		location: impl AsRef<str>,
		status: impl Into<Option<StatusCode>>,
	) -> Result<&mut Self, String> {
		let accepts_client_redirect = accepts_client_redirect(self.request_headers);
		self.proxy()
			.redirect(accepts_client_redirect, location.as_ref(), status.into())
			.map_err(|err| err.to_string())?;
		Ok(self)
	}

	fn proxy(&self) -> MutexGuard<'_, Proxy> {
		self.proxy.lock().expect("response proxy lock poisoned")
	}
}

pub(super) fn accepts_client_redirect(headers: &http::HeaderMap) -> bool {
	let Some(value) = headers.get(CLIENT_ACCEPTS_REDIRECT_HEADER) else {
		return false;
	};
	let Ok(value) = value.to_str() else {
		return false;
	};
	matches!(value, "1" | "t" | "T" | "TRUE" | "true" | "True")
}

fn response_handle_for<S, E, I>(inner: &RequestCtx<S, E, I>) -> ResponseHandle<'_> {
	let request_headers = inner.request().headers();
	ResponseHandle {
		proxy: inner.response_proxy(),
		request_headers,
	}
}

fn head_handle_for<S, E, I>(inner: &RequestCtx<S, E, I>) -> HeadHandle {
	HeadHandle {
		proxy: inner.response_proxy(),
	}
}

/// Mutable head-effect handle.
pub struct HeadHandle {
	proxy: Arc<Mutex<Proxy>>,
}

impl HeadHandle {
	/// Set the document title.
	pub fn title(&mut self, title: impl Into<String>) -> &mut Self {
		self.proxy().head_builder().title(title);
		self
	}

	/// Set the meta description.
	pub fn description(&mut self, description: impl Into<String>) -> &mut Self {
		self.proxy().head_builder().description(description);
		self
	}

	/// Add a favicon link.
	pub fn icon(&mut self, href: impl Into<String>) -> &mut Self {
		self.proxy().head_builder().icon(href);
		self
	}

	/// Add a preload link.
	pub fn preload(&mut self, href: impl Into<String>, r#as: impl Into<String>) -> &mut Self {
		self.proxy().head_builder().preload(href, r#as);
		self
	}

	/// Add a `<meta name="..." content="...">` element.
	pub fn meta_name_content(
		&mut self,
		name: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.proxy().head_builder().meta_name_content(name, content);
		self
	}

	/// Add a `<meta property="..." content="...">` element.
	pub fn meta_property_content(
		&mut self,
		property: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.proxy()
			.head_builder()
			.meta_property_content(property, content);
		self
	}

	/// Add a charset meta element.
	pub fn meta_charset(&mut self, charset: impl Into<String>) -> &mut Self {
		self.proxy().head_builder().meta_charset(charset);
		self
	}

	/// Add a low-level head element definition.
	pub fn add(
		&mut self,
		defs: impl IntoIterator<Item = head::HtmlElementDef>,
	) -> Result<&mut Self, String> {
		self.proxy().head_builder().add(defs)?;
		Ok(self)
	}

	/// Append another head builder's elements.
	pub fn append(&mut self, other: &head::HeadBuilder) -> &mut Self {
		self.proxy().head_builder().append(other);
		self
	}

	fn proxy(&self) -> MutexGuard<'_, Proxy> {
		self.proxy.lock().expect("response proxy lock poisoned")
	}
}

/// Context passed to task middleware.
pub struct TaskMiddlewareCtx<S, E = Box<dyn std::error::Error + Send + Sync>> {
	inner: RequestCtx<S, E, None>,
}

impl<S, E> TaskMiddlewareCtx<S, E> {
	pub(crate) fn new(inner: RequestCtx<S, E, None>) -> Self {
		Self { inner }
	}

	/// Pattern matched by the view or API-route being handled.
	pub fn matched_pattern(&self) -> &str {
		self.inner.matched_pattern()
	}

	/// Route parameters for the matched view or API-route.
	pub fn params(&self) -> RouteParams<'_> {
		RouteParams {
			inner: self.inner.params(),
		}
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &[String] {
		self.inner.splat_values()
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<E> {
		self.inner.exec_ctx()
	}

	/// Current HTTP request.
	pub fn request(&self) -> HttpRequest<'_> {
		HttpRequest::new(self.inner.request())
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner.public_url(src_path)
	}

	/// Mutable response-effect handle for status, headers, cookies, and redirects.
	pub fn response(&self) -> ResponseHandle<'_> {
		response_handle_for(&self.inner)
	}

	/// Mutable head-effect handle.
	pub fn head(&self) -> HeadHandle {
		head_handle_for(&self.inner)
	}
}
