//! Wire-contract types and constants shared with the browser runtime.

use std::collections::BTreeMap;

use serde::Serialize;
use serde_json::Value;

/// Response header carrying the expected client build identifier.
pub const CLIENT_BUILD_ID_HEADER: &str = "X-Vorma-Client-Build-Id";
/// Response header marking a build-skew JSON response.
pub const BUILD_SKEW_HEADER: &str = "X-Vorma-Build-Skew";
/// Response header carrying a browser-handled redirect target.
pub const CLIENT_REDIRECT_HEADER: &str = "X-Client-Redirect";
/// Request header indicating that the browser runtime accepts client redirects.
pub const CLIENT_ACCEPTS_REDIRECT_HEADER: &str = "X-Accepts-Client-Redirect";
/// Query key carrying the current route client build identifier.
pub const VORMA_JSON_QUERY_KEY: &str = "vorma-json";

/// Prepared HTML element used in document head payloads.
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
	#[serde(skip)]
	pub self_closing: bool,
}

/// Fixed browser route payload.
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
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub outermost_server_err: String,
	/// Index of the outermost server error.
	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub outermost_server_err_idx: Option<usize>,
}

/// Server-rendered bootstrap payload.
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
