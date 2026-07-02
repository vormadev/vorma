//! In-memory test harness for Vorma applications.
//!
//! [`TestApp`](crate::testing::TestApp) boots a complete application from its
//! public [`AppConfig`](crate::AppConfig)
//! declaration without requiring build artifacts on disk: the runtime
//! manifest and asset capabilities are synthesized from the compiled
//! framework graph, and public assets are served from in-memory bytes.
//! This makes request-level behavior — routing, input decoding, middleware
//! composition, response effects, view payloads — assertable in ordinary
//! unit and integration tests.
//!
//! ```
//! # use serde::{Deserialize, Serialize};
//! # vorma::app!(mod doc_app for ());
//! # #[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
//! # struct StoryInput {}
//! # #[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
//! # struct StoryOutput {
//! #     id: String,
//! # }
//! # const STORY_VIEW: doc_app::View = doc_app::view! {
//! #     client_file: "src/client/views/story.view.tsx";
//! #     pattern: "/stories/:id";
//! #     input: StoryInput;
//! #     output: StoryOutput;
//! #
//! #     handler: |ctx| {
//! #         Ok(StoryOutput {
//! #             id: ctx.param("id").to_owned(),
//! #         })
//! #     };
//! # };
//! # fn app_config() -> vorma::AppConfig<()> {
//! #     vorma::AppConfig {
//! #         views: doc_app::views![STORY_VIEW],
//! #         ..vorma::AppConfig::default()
//! #     }
//! # }
//! # #[tokio::main(flavor = "current_thread")]
//! # async fn main() -> vorma::Result<()> {
//! let app = vorma::testing::TestApp::from_config(app_config())?;
//! let response = app.get("/stories/42").await;
//! assert_eq!(response.status(), vorma::HttpStatusCode::OK);
//! # Ok(())
//! # }
//! ```
//!
//! The harness serves the same committed request pipeline as production
//! (`RuntimeHost`); only the manifest/asset loading differs. Responses are
//! therefore suitable as captured wire-contract fixtures.

use std::collections::{BTreeMap, HashMap};
use std::sync::Arc;

use bytes::Bytes;
use cookie::Cookie;
use http::header::{CONTENT_TYPE, COOKIE, SET_COOKIE};
use http::{HeaderValue, Method, Request, Response};

use crate::asset_body_provider::{
	PublicAssetBody, PublicAssetBodyError, PublicAssetBodyFuture, PublicAssetBodyProvider,
};
use crate::asset_capabilities::PublicAsset;
use crate::config::normalize_public_static_base;
use crate::error::Error;
use crate::framework_graph::FrameworkGraph;
use crate::public_app::{App, AppConfig};
use crate::response_finalizer::VORMA_JSON_QUERY_KEY;
use crate::runtime_host::CommittedRuntimeHost;
use crate::runtime_manifest::{RuntimeManifest, RuntimeViewModule};

/// Client build identifier committed by synthesized [`TestApp`] manifests.
pub const TEST_CLIENT_BUILD_ID: &str = "vorma-test-build";

/// In-memory application booted from public declarations for request-level tests.
pub struct TestApp {
	host: CommittedRuntimeHost<InMemoryPublicAssets>,
}

impl TestApp {
	/// Boot an in-memory app from a complete public configuration.
	pub fn from_config<S>(app_config: AppConfig<S>) -> crate::Result<Self>
	where
		S: Send + Sync + 'static,
	{
		Self::builder(app_config).build()
	}

	/// Start building an in-memory app, e.g. to attach public assets.
	pub fn builder<S>(app_config: AppConfig<S>) -> TestAppBuilder<S>
	where
		S: Send + Sync + 'static,
	{
		TestAppBuilder {
			app_config,
			public_assets: Vec::new(),
		}
	}

	/// Handle one already-collected request through the committed runtime pipeline.
	pub async fn handle_request(&self, request: Request<Bytes>) -> Response<Bytes> {
		self.host.service().handle_request(request).await
	}

	/// Issue a GET request.
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI.
	pub async fn get(&self, path_and_query: &str) -> Response<Bytes> {
		let request = Request::builder()
			.method(Method::GET)
			.uri(path_and_query)
			.body(Bytes::new())
			.expect("test request URI should be valid");
		self.handle_request(request).await
	}

	/// Issue a GET request for a view's JSON payload (as the browser router does).
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI.
	pub async fn get_view_payload(&self, path_and_query: &str) -> Response<Bytes> {
		let request = Request::builder()
			.method(Method::GET)
			.uri(view_payload_uri(path_and_query))
			.body(Bytes::new())
			.expect("test request URI should be valid");
		self.handle_request(request).await
	}

	/// Issue a request with a JSON body.
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI or `body` fails
	/// JSON serialization.
	pub async fn request_json<B>(
		&self,
		method: Method,
		path_and_query: &str,
		body: &B,
	) -> Response<Bytes>
	where
		B: serde::Serialize,
	{
		let body = serde_json::to_vec(body).expect("test request body should serialize");
		let request = Request::builder()
			.method(method)
			.uri(path_and_query)
			.header(CONTENT_TYPE, HeaderValue::from_static("application/json"))
			.body(Bytes::from(body))
			.expect("test request URI should be valid");
		self.handle_request(request).await
	}

	/// Start building a request with arbitrary method, headers, and body.
	pub fn request(&self, method: Method, path_and_query: &str) -> TestRequest<'_> {
		TestRequest {
			app: self,
			method,
			path_and_query: path_and_query.to_owned(),
			headers: Vec::new(),
			body: Bytes::new(),
		}
	}

	/// Start an explicit cookie-continuation session against this app.
	///
	/// `TestApp` itself stays stateless: no request carries cookies unless it
	/// goes through a [`TestSession`], and no two sessions share a jar. Use
	/// this for auth-flow tests that log in once and then need later
	/// requests to carry the cookies the login response set — the session
	/// reads every response's `Set-Cookie` headers back into its own jar and
	/// applies the accumulated cookies to each later request it sends,
	/// exactly as a browser would.
	pub fn session(&self) -> TestSession<'_> {
		TestSession {
			app: self,
			cookies: HashMap::new(),
		}
	}

	/// Client build identifier committed by the synthesized manifest.
	pub fn client_build_id(&self) -> &str {
		self.host.client_build_id()
	}

	/// Resolve a public source path through the committed asset capabilities.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.host
			.snapshot()
			.asset_capabilities()
			.public_url(src_path)
			.map(ToOwned::to_owned)
			.map_err(|source| Error::new(source.to_string()))
	}
}

/// In-flight test request: headers and body before dispatch.
pub struct TestRequest<'a> {
	app: &'a TestApp,
	method: Method,
	path_and_query: String,
	headers: Vec<(String, String)>,
	body: Bytes,
}

impl TestRequest<'_> {
	/// Add a request header.
	pub fn header(mut self, name: impl Into<String>, value: impl Into<String>) -> Self {
		self.headers.push((name.into(), value.into()));
		self
	}

	/// Add a `Cookie` header from a name/value pair.
	pub fn cookie(self, name: impl AsRef<str>, value: impl AsRef<str>) -> Self {
		let cookie = format!("{}={}", name.as_ref(), value.as_ref());
		self.header("cookie", cookie)
	}

	/// Set a raw request body with its content type.
	pub fn body(mut self, content_type: impl Into<String>, body: impl Into<Vec<u8>>) -> Self {
		self.headers
			.push(("content-type".to_owned(), content_type.into()));
		self.body = Bytes::from(body.into());
		self
	}

	/// Dispatch through the committed runtime pipeline.
	///
	/// # Panics
	///
	/// Panics when the URI or any header is invalid for an HTTP request.
	pub async fn send(self) -> Response<Bytes> {
		let mut builder = Request::builder()
			.method(self.method)
			.uri(self.path_and_query);
		for (name, value) in self.headers {
			builder = builder.header(name, value);
		}
		let request = builder
			.body(self.body)
			.expect("test request parts should be valid");
		self.app.handle_request(request).await
	}
}

/// Typed access to the `Set-Cookie` headers on a [`TestApp`] response.
///
/// Reading a cookie a handler set back into a later request otherwise means
/// hand-parsing the raw `Set-Cookie` header string. This reads every
/// `Set-Cookie` header on the response through the same [`HttpCookie`](crate::HttpCookie)
/// type application code already uses to build them, attributes included
/// (`Max-Age`, `Path`, `HttpOnly`, and so on) instead of just a name/value
/// pair. Prefer [`TestApp::session`] for auth-flow tests that need to carry
/// cookies forward automatically; reach for this directly only when a test
/// needs to assert on a cookie's attributes rather than just continue the
/// session.
pub trait TestResponseCookies {
	/// Every cookie this response set, parsed from its `Set-Cookie` headers.
	///
	/// # Panics
	///
	/// Panics if a `Set-Cookie` header is not valid UTF-8 or fails to parse
	/// as a cookie — both are a broken test fixture, not a case tests should
	/// need to handle.
	fn set_cookie_headers(&self) -> Vec<Cookie<'static>>;
}

impl TestResponseCookies for Response<Bytes> {
	fn set_cookie_headers(&self) -> Vec<Cookie<'static>> {
		self.headers()
			.get_all(SET_COOKIE)
			.iter()
			.map(|value| {
				let raw = value
					.to_str()
					.expect("test Set-Cookie header should be ascii");
				Cookie::parse(raw.to_owned())
					.expect("test Set-Cookie header should parse")
					.into_owned()
			})
			.collect()
	}
}

/// Explicit cookie-continuation session against one [`TestApp`].
///
/// A session accumulates cookies from every response it receives and sends
/// them back on every later request it makes, the same way a browser tab
/// would across a login flow — [`TestApp`] itself never does this on its
/// own, so a plain `app.get(...)` call never carries cookies and two
/// sessions never share a jar. Overwrite-by-name and cookie clearing
/// (`Max-Age <= 0` or an `Expires` in the past, exactly as
/// [`HttpCookie::make_removal`](crate::HttpCookie::make_removal) produces)
/// are both honored, matching real cookie-jar semantics rather than just
/// copying the latest value seen for each name.
///
/// ```
/// # vorma::app!(mod doc_app for ());
/// # const SESSION_VIEW: doc_app::View = doc_app::view! {
/// #     client_file: "src/client/views/session.view.tsx";
/// #     pattern: "/";
/// #     input: ();
/// #     output: ();
/// #     handler: |ctx| {
/// #         ctx.response().set_cookie(vorma::HttpCookie::new("session", "abc"));
/// #         Ok(())
/// #     };
/// # };
/// # fn app_config() -> vorma::AppConfig<()> {
/// #     vorma::AppConfig {
/// #         views: doc_app::views![SESSION_VIEW],
/// #         ..vorma::AppConfig::default()
/// #     }
/// # }
/// # #[tokio::main(flavor = "current_thread")]
/// # async fn main() -> vorma::Result<()> {
/// let app = vorma::testing::TestApp::from_config(app_config())?;
/// let mut session = app.session();
/// session.get("/").await; // absorbs the `session` cookie the view set
/// let next = session.get("/").await; // carries it back automatically
/// assert_eq!(next.status(), vorma::HttpStatusCode::OK);
/// # Ok(())
/// # }
/// ```
pub struct TestSession<'a> {
	app: &'a TestApp,
	cookies: HashMap<String, String>,
}

impl<'a> TestSession<'a> {
	/// Issue a GET request, applying and then updating this session's jar.
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI.
	pub async fn get(&mut self, path_and_query: &str) -> Response<Bytes> {
		self.request(Method::GET, path_and_query).send().await
	}

	/// Issue a GET request for a view's JSON payload, applying and then
	/// updating this session's jar.
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI.
	pub async fn get_view_payload(&mut self, path_and_query: &str) -> Response<Bytes> {
		self.request(Method::GET, &view_payload_uri(path_and_query))
			.send()
			.await
	}

	/// Issue a request with a JSON body, applying and then updating this
	/// session's jar.
	///
	/// # Panics
	///
	/// Panics when `path_and_query` is not a valid request URI or `body` fails
	/// JSON serialization.
	pub async fn request_json<B>(
		&mut self,
		method: Method,
		path_and_query: &str,
		body: &B,
	) -> Response<Bytes>
	where
		B: serde::Serialize,
	{
		let body = serde_json::to_vec(body).expect("test request body should serialize");
		self.request(method, path_and_query)
			.body("application/json", body)
			.send()
			.await
	}

	/// Start building a request that applies and then updates this session's
	/// jar, with arbitrary method, headers, and body.
	pub fn request(&mut self, method: Method, path_and_query: &str) -> TestSessionRequest<'a, '_> {
		TestSessionRequest {
			session: self,
			method,
			path_and_query: path_and_query.to_owned(),
			headers: Vec::new(),
			extra_cookies: Vec::new(),
			body: None,
		}
	}

	// Every request-carried `Cookie` header must be exactly one merged
	// value: the `cookie` request header is a single `name=value; ...` line
	// by the HTTP spec, and `http::HeaderMap::get` (what real app code reads
	// through, mirroring a browser) only ever sees the FIRST header line for
	// a repeated name — so appending a second `Cookie:` line the way
	// `TestRequest::cookie` does for a single explicit pair would silently
	// make every jar entry but the first invisible to the app under test.
	// `extra` cookies (added via `TestSessionRequest::cookie`) take
	// precedence over the jar's own entries with the same name, matching an
	// explicit per-request override winning over ambient session state.
	fn merged_cookie_header(&self, extra: &[(String, String)]) -> Option<String> {
		let mut merged = self.cookies.clone();
		for (name, value) in extra {
			merged.insert(name.clone(), value.clone());
		}
		if merged.is_empty() {
			return None;
		}
		let mut pairs = merged.into_iter().collect::<Vec<_>>();
		pairs.sort_unstable_by(|(a, _), (b, _)| a.cmp(b));
		Some(
			pairs
				.into_iter()
				.map(|(name, value)| format!("{name}={value}"))
				.collect::<Vec<_>>()
				.join("; "),
		)
	}

	fn absorb_response_cookies(&mut self, response: &Response<Bytes>) {
		for cookie in response.set_cookie_headers() {
			let is_cleared = cookie
				.max_age()
				.is_some_and(|max_age| max_age <= cookie::time::Duration::ZERO)
				|| cookie
					.expires_datetime()
					.is_some_and(|expires| expires <= cookie::time::OffsetDateTime::now_utc());
			if is_cleared {
				self.cookies.remove(cookie.name());
			} else {
				self.cookies
					.insert(cookie.name().to_owned(), cookie.value().to_owned());
			}
		}
	}
}

/// In-flight session request: applies the session's jar (merged with any
/// cookies added directly on this request) on send, then folds the
/// response's `Set-Cookie` headers back into the jar.
///
/// Mirrors [`TestRequest`]'s builder verbs — the same request-building
/// verbs a stateless [`TestApp::request`] call exposes — so switching a call
/// site between a bare app request and a session request never means
/// learning a different builder.
pub struct TestSessionRequest<'a, 's> {
	session: &'s mut TestSession<'a>,
	method: Method,
	path_and_query: String,
	headers: Vec<(String, String)>,
	extra_cookies: Vec<(String, String)>,
	body: Option<(String, Vec<u8>)>,
}

impl<'a> TestSessionRequest<'a, '_> {
	/// Add a request header.
	pub fn header(mut self, name: impl Into<String>, value: impl Into<String>) -> Self {
		self.headers.push((name.into(), value.into()));
		self
	}

	/// Add a `Cookie` header from a name/value pair, taking precedence over
	/// a same-named cookie the session's jar already carries.
	pub fn cookie(mut self, name: impl AsRef<str>, value: impl AsRef<str>) -> Self {
		self.extra_cookies
			.push((name.as_ref().to_owned(), value.as_ref().to_owned()));
		self
	}

	/// Set a raw request body with its content type.
	pub fn body(mut self, content_type: impl Into<String>, body: impl Into<Vec<u8>>) -> Self {
		self.body = Some((content_type.into(), body.into()));
		self
	}

	/// Dispatch through the committed runtime pipeline, then fold the
	/// response's `Set-Cookie` headers into the session's jar.
	///
	/// # Panics
	///
	/// Panics when the URI or any header is invalid for an HTTP request.
	pub async fn send(self) -> Response<Bytes> {
		let mut request = self.session.app.request(self.method, &self.path_and_query);
		if let Some(cookie_header) = self.session.merged_cookie_header(&self.extra_cookies) {
			request = request.header(COOKIE.as_str(), cookie_header);
		}
		for (name, value) in self.headers {
			request = request.header(name, value);
		}
		if let Some((content_type, body)) = self.body {
			request = request.body(content_type, body);
		}
		let response = request.send().await;
		self.session.absorb_response_cookies(&response);
		response
	}
}

/// Builder for [`TestApp`] instances with optional in-memory public assets.
pub struct TestAppBuilder<S> {
	app_config: AppConfig<S>,
	public_assets: Vec<(String, Bytes)>,
}

impl<S> TestAppBuilder<S>
where
	S: Send + Sync + 'static,
{
	/// Attach an in-memory public asset, addressed by its public source path
	/// (the same path application code passes to `public_url`).
	pub fn with_public_asset(
		mut self,
		src_path: impl Into<String>,
		body: impl Into<Bytes>,
	) -> Self {
		self.public_assets.push((src_path.into(), body.into()));
		self
	}

	/// Boot the in-memory app.
	pub fn build(self) -> crate::Result<TestApp> {
		let public_static_base = normalize_public_static_base(&self.app_config.public_static_base);
		let app = App::from_app_config(self.app_config)?;
		let (_config, assembly) = app.into_config_and_assembly();
		let compiled = assembly
			.compile()
			.map_err(|source| Error::new(format!("{source:?}")))?;
		let manifest = synthetic_manifest(
			compiled.facade().graph(),
			&public_static_base,
			&self.public_assets,
		);
		let assets = InMemoryPublicAssets::new(self.public_assets);
		let host = compiled
			.into_runtime_host(manifest, assets)
			.map_err(|source| Error::new(format!("{source:?}")))?;
		Ok(TestApp { host })
	}
}

/// Synthesize the runtime manifest a real build would commit for this graph.
fn synthetic_manifest(
	graph: &FrameworkGraph,
	public_static_base: &str,
	public_assets: &[(String, Bytes)],
) -> RuntimeManifest {
	let search_schemas = graph
		.views()
		.iter()
		.filter(|view| !view.search_schema().is_null())
		.map(|view| (view.pattern().to_owned(), view.search_schema().clone()))
		.collect::<BTreeMap<_, _>>();
	// Each synthetic view module carries one dep and one CSS bundle so view
	// payloads exercise the full wire contract, as real builds do.
	let view_modules = graph
		.views()
		.iter()
		.enumerate()
		.map(|(index, view)| {
			RuntimeViewModule::new(
				view.pattern(),
				format!("{public_static_base}vorma-test-views/view-{index}.js"),
				vec![format!(
					"{public_static_base}vorma-test-views/view-{index}-dep.js"
				)],
				vec![format!(
					"{public_static_base}vorma-test-views/view-{index}.css"
				)],
			)
		})
		.collect::<Vec<_>>();
	let public_filemap = public_assets
		.iter()
		.map(|(src_path, _)| (src_path.clone(), format!("{public_static_base}{src_path}")))
		.collect::<BTreeMap<_, _>>();
	// View module, dep, and CSS URLs must be listed as committed public outputs.
	let public_filepaths = public_filemap
		.values()
		.cloned()
		.chain(view_modules.iter().flat_map(|module| {
			std::iter::once(module.import_url().to_owned())
				.chain(module.dep_urls().iter().cloned())
				.chain(module.css_bundle_urls().iter().cloned())
		}))
		.collect::<Vec<_>>();
	RuntimeManifest::new(
		TEST_CLIENT_BUILD_ID,
		public_static_base,
		"",
		search_schemas,
		public_filepaths,
		public_filemap,
		view_modules,
	)
}

/// Public asset provider backed by in-memory bytes declared on the builder.
struct InMemoryPublicAssets {
	bodies: Arc<HashMap<String, Bytes>>,
}

impl InMemoryPublicAssets {
	fn new(public_assets: Vec<(String, Bytes)>) -> Self {
		Self {
			bodies: Arc::new(public_assets.into_iter().collect()),
		}
	}
}

impl PublicAssetBodyProvider for InMemoryPublicAssets {
	fn body_for_asset(&self, asset: PublicAsset) -> PublicAssetBodyFuture {
		let result = match self.bodies.get(asset.fs_path()) {
			Some(body) => {
				let body = PublicAssetBody::new(body.clone());
				Ok(match guessed_content_type(asset.fs_path()) {
					Some(content_type) => body.with_content_type(content_type),
					None => body,
				})
			}
			None => Err(PublicAssetBodyError::new(format!(
				"no in-memory test asset registered for `{}`",
				asset.fs_path()
			))),
		};
		Box::pin(async move { result })
	}
}

fn guessed_content_type(fs_path: &str) -> Option<HeaderValue> {
	let mime = mime_guess::from_path(fs_path).first()?;
	HeaderValue::from_str(mime.essence_str()).ok()
}

/// Append the JSON-view-payload query marker the browser router sends.
fn view_payload_uri(path_and_query: &str) -> String {
	let separator = if path_and_query.contains('?') {
		'&'
	} else {
		'?'
	};
	format!("{path_and_query}{separator}{VORMA_JSON_QUERY_KEY}=_")
}
