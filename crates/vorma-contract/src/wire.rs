//! Wire-contract types and constants shared with the browser runtime.
//!
//! Framework-integration surface, and **frozen**: every type and constant
//! here is part of the JSON shape and HTTP header vocabulary the server
//! and an already-downloaded browser runtime speak to each other. An
//! application author never constructs [`ViewPayload`](crate::wire::ViewPayload)
//! or [`SsrPayload`](crate::wire::SsrPayload) by hand — `vorma` builds them
//! from a matched route and its handler output. Changing a field name, a
//! header value, or the shape a `#[derive(TsGen)]`'d type here renders as
//! is a framework release decision: a client built against the old shape
//! is still running in someone's browser tab when a new server ships, and
//! it has to keep working (or hit rejects with a version-skew response,
//! never receive silently-misinterpreted data). See [`crate::live_state`]
//! for the sibling protocol on the build side.

use std::collections::BTreeMap;

use serde::Serialize;
use serde_json::Value;

/// Response header carrying the expected client build identifier.
///
/// Sent on every response so the browser runtime can detect that its own
/// downloaded build no longer matches what the server is running (a
/// deploy happened underneath it) and react — typically by falling back
/// to a full navigation instead of a client-side route transition.
pub const CLIENT_BUILD_ID_HEADER: &str = "X-Vorma-Client-Build-Id";
/// Response header marking a build-skew JSON response.
///
/// Set on a JSON response whose [`CLIENT_BUILD_ID_HEADER`] no longer
/// matches the requesting client's build, telling the browser runtime the
/// body is a version-skew notice rather than the route data it asked for.
pub const BUILD_SKEW_HEADER: &str = "X-Vorma-Build-Skew";
/// Response header carrying a browser-handled redirect target.
///
/// Set instead of a normal HTTP redirect when the browser runtime — not
/// the browser's own navigation — is expected to perform the redirect, so
/// a client-side data fetch can follow it without a full page load.
pub const CLIENT_REDIRECT_HEADER: &str = "X-Client-Redirect";
/// Response header marking a typed raw resource body.
///
/// Present on a resource response whose body is a raw, non-JSON payload
/// (see `vorma::ResourceBody`), so the browser runtime knows not to parse
/// it as the usual route JSON envelope.
pub const RESOURCE_BODY_HEADER: &str = "x-vorma-resource-body";
/// Request header indicating that the browser runtime accepts client redirects.
///
/// Sent by the browser runtime on requests it originates, so the server
/// knows it may answer with [`CLIENT_REDIRECT_HEADER`] instead of a plain
/// HTTP redirect; requests without it (a direct browser navigation, a
/// non-Vorma client) never receive that header.
pub const CLIENT_ACCEPTS_REDIRECT_HEADER: &str = "X-Accepts-Client-Redirect";
/// Query key carrying the current route client build identifier.
///
/// Appended by the browser runtime to its own data-fetch requests so the
/// server can compare it against the build actually running and answer
/// with a [`BUILD_SKEW_HEADER`] response when they disagree.
pub const VORMA_JSON_QUERY_KEY: &str = "vorma-json";

/// Prepared HTML element used in document head payloads.
///
/// The wire shape of one document head element (a `<title>`, a `<meta>`,
/// a `<link>`) after server-side validation and escaping: every string
/// field here is already safe to insert into the DOM as-is, which is why
/// this type — unlike [`crate::contracts::DocumentElementContract`], its
/// unvalidated build-time counterpart — carries no separate "trusted vs.
/// escaped" attribute split; validation already happened before a
/// `HeadElement` was constructed.
#[derive(Clone, Debug, Default, Eq, Ord, PartialEq, PartialOrd, Serialize, vorma_macros::TsGen)]
pub struct HeadElement {
	/// Element tag name.
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub tag: String,
	/// Element attributes that are already escaped/trusted.
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	pub attributes_known_safe: BTreeMap<String, String>,
	/// Boolean element attributes.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub boolean_attributes: Vec<String>,
	/// Unsafe inner HTML that has already been produced by framework-owned rendering.
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub dangerous_inner_html: String,
	/// Whether the server HTML renderer should render this as self-closing.
	///
	/// Server-only: never sent to the browser (`#[serde(skip)]`, and
	/// correspondingly absent from the generated TypeScript shape). The
	/// browser's own HTML parser already knows which elements self-close;
	/// this field exists purely to drive the server's own HTML string
	/// renderer.
	#[serde(skip)]
	pub self_closing: bool,
}

/// Fixed browser route payload.
///
/// Everything the browser runtime needs to commit one route change:
/// which patterns matched (outermost to innermost), captured params and
/// splat values, the prepared head elements for that chain, module
/// import/dependency/CSS URLs to load, and each matched view's server
/// data in the same outermost-to-innermost order as `matched_patterns`.
/// [`Self::views_data`] is untyped JSON here deliberately — the generated
/// TypeScript client, not this Rust type, is where each view's data gets
/// its real static type back.
#[derive(Clone, Debug, Default, PartialEq, Serialize, vorma_macros::TsGen)]
pub struct ViewPayload {
	/// Matched route patterns from outermost to innermost.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub matched_patterns: Vec<String>,
	/// Captured route params.
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	pub params: BTreeMap<String, String>,
	/// Captured splat values.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub splat_values: Vec<String>,
	/// Search schemas for matched views.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub search_schemas: Vec<Value>,
	/// Prepared title head element.
	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub title: Option<HeadElement>,
	/// Prepared meta head elements.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub meta_head_els: Vec<HeadElement>,
	/// Prepared non-meta head elements.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub rest_head_els: Vec<HeadElement>,
	/// Browser import URLs for matched view modules.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub import_urls: Vec<String>,
	/// Module dependency URLs for preloading.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub deps: Vec<String>,
	/// CSS bundle URLs.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub css_bundles: Vec<String>,
	/// Server view data in matched-view order.
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub views_data: Vec<Value>,
	/// Outermost server error visible to the browser.
	///
	/// Empty when no matched view errored. When present, it is the
	/// message from the shallowest (outermost) view in the chain that
	/// failed — deeper views may also have failed, but the browser only
	/// surfaces the one closest to the layout root.
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub outermost_server_err: String,
	/// Index into [`Self::matched_patterns`]/[`Self::views_data`] of the
	/// view [`Self::outermost_server_err`] came from. `None` exactly when
	/// `outermost_server_err` is empty.
	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub outermost_server_err_idx: Option<usize>,
}

/// Server-rendered bootstrap payload.
///
/// The inline JSON a fresh (non-client-navigated) page load embeds for
/// the browser runtime to hydrate from, so the first route commit does
/// not have to re-fetch data the server already rendered with. Carries
/// [`ViewPayload`]'s fields (via `#[serde(flatten)]`) plus the build
/// identity and dev-mode facts the browser needs before it can safely
/// treat that data as current.
#[derive(Clone, Debug, Default, PartialEq, Serialize)]
pub struct SsrPayload {
	/// Current client build identifier.
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub client_build_id: String,
	/// Whether the payload was produced in dev mode.
	#[serde(default, skip_serializing_if = "is_false")]
	pub is_dev: bool,
	/// Optional deployment identifier used by platform skew protection.
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub deployment_id: String,
	/// Fixed browser route payload fields.
	#[serde(flatten)]
	pub view_payload: ViewPayload,
}

fn is_false(value: &bool) -> bool {
	!*value
}
