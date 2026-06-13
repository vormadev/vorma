//! Runtime manifest contract consumed by committed server snapshots.

use std::collections::BTreeMap;

use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use data_encoding::BASE32_NOPAD;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};

use crate::contracts::DocumentElementContract;
use crate::document_renderer::{DocumentRenderError, trusted_element_parts};

/// DOM id for the critical CSS style element.
pub const CRITICAL_CSS_STYLE_ELEMENT_ID: &str = "vorma-critical-css";
const HEAD_TAG_STYLE: &str = "style";
const HEAD_ATTR_ID: &str = "id";

/// Runtime manifest consumed by the server and browser payload builder.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct RuntimeManifest {
	#[serde(skip)]
	client_build_id: String,
	vorma_version: String,
	#[serde(default, skip_serializing_if = "is_zero")]
	dev_vite_server_port: i32,
	#[serde(default, skip_serializing_if = "is_zero")]
	dev_mux_port: i32,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	dev_refresh_token: String,
	public_static_base_path: String,
	ui_variant: String,
	root_document_shell_hash: String,
	public_filepaths: Vec<String>,
	public_filemap: BTreeMap<String, String>,
	critical_css: String,
	search_schemas: BTreeMap<String, Value>,
	client_entry: ClientModule,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	client_core_assets: Option<ClientCoreAssets>,
	client_views: BTreeMap<String, ClientModule>,
	#[serde(skip)]
	view_modules: Vec<RuntimeViewModule>,
}

impl RuntimeManifest {
	/// Create a runtime manifest.
	pub fn new(
		client_build_id: impl Into<String>,
		public_static_base: impl Into<String>,
		critical_css: impl Into<String>,
		search_schemas: BTreeMap<String, Value>,
		public_filepaths: Vec<String>,
		public_filemap: BTreeMap<String, String>,
		view_modules: Vec<RuntimeViewModule>,
	) -> Self {
		let mut manifest = Self {
			client_build_id: client_build_id.into(),
			vorma_version: env!("CARGO_PKG_VERSION").to_owned(),
			dev_vite_server_port: 0,
			dev_mux_port: 0,
			dev_refresh_token: String::new(),
			public_static_base_path: public_static_base.into(),
			ui_variant: String::new(),
			root_document_shell_hash: String::new(),
			public_filepaths,
			public_filemap,
			critical_css: critical_css.into(),
			search_schemas,
			client_entry: ClientModule::default(),
			client_core_assets: None,
			client_views: view_modules
				.iter()
				.map(|module| {
					(
						module.pattern().to_owned(),
						ClientModule::new(
							module.import_url(),
							module.dep_urls().to_vec(),
							module.css_bundle_urls().to_vec(),
						),
					)
				})
				.collect(),
			view_modules,
		};
		manifest.populate_client_build_id();
		manifest
	}

	/// Attach client entry module artifacts.
	pub fn with_client_entry(mut self, client_entry: ClientModule) -> Self {
		self.client_entry = client_entry;
		self.refresh_client_build_id();
		self
	}

	/// Attach client matcher/core assets.
	pub fn with_client_core_assets(mut self, client_core_assets: Option<ClientCoreAssets>) -> Self {
		self.client_core_assets = client_core_assets;
		self.refresh_client_build_id();
		self
	}

	/// Attach dev runtime manifest metadata.
	pub fn with_dev_metadata(
		mut self,
		dev_vite_server_port: i32,
		dev_mux_port: i32,
		dev_refresh_token: impl Into<String>,
	) -> Self {
		self.dev_vite_server_port = dev_vite_server_port;
		self.dev_mux_port = dev_mux_port;
		self.dev_refresh_token = dev_refresh_token.into();
		self.refresh_client_build_id();
		self
	}

	/// Attach UI variant metadata.
	pub fn with_ui_variant(mut self, ui_variant: impl Into<String>) -> Self {
		self.ui_variant = ui_variant.into();
		self.refresh_client_build_id();
		self
	}

	/// Attach root document shell hash metadata.
	pub fn with_root_document_shell_hash(
		mut self,
		root_document_shell_hash: impl Into<String>,
	) -> Self {
		self.root_document_shell_hash = root_document_shell_hash.into();
		self.refresh_client_build_id();
		self
	}

	/// Attach framework package version metadata.
	pub fn with_vorma_version(mut self, vorma_version: impl Into<String>) -> Self {
		self.vorma_version = vorma_version.into();
		self.refresh_client_build_id();
		self
	}

	fn populate_client_build_id(&mut self) {
		if self.client_build_id.trim().is_empty() {
			self.refresh_client_build_id();
		}
	}

	fn refresh_client_build_id(&mut self) {
		self.client_build_id = self
			.to_client_build_id()
			.expect("runtime manifest should serialize for build id derivation");
	}

	/// Decode a runtime manifest from JSON bytes.
	pub fn from_json_slice(bytes: &[u8]) -> Result<Self, serde_json::Error> {
		let mut manifest: Self = serde_json::from_slice(bytes)?;
		manifest.view_modules = manifest
			.client_views
			.iter()
			.map(|(pattern, module)| {
				RuntimeViewModule::new(
					pattern,
					module.url(),
					module.dep_urls().to_vec(),
					module.css_bundle_urls().to_vec(),
				)
			})
			.collect();
		manifest.client_build_id = manifest.to_client_build_id()?;
		Ok(manifest)
	}

	/// Encode this runtime manifest to stable tab-indented pretty JSON bytes.
	pub fn to_json_vec(&self) -> Result<Vec<u8>, serde_json::Error> {
		crate::contracts::to_tab_indented_json_string(self).map(String::into_bytes)
	}

	/// Derive the fixed client build identifier from the serialized manifest contract.
	pub fn to_client_build_id(&self) -> Result<String, serde_json::Error> {
		let data = serde_json::to_vec(self)?;
		let hash = blake3::hash(&data);
		let mut encoded = BASE32_NOPAD.encode(hash.as_bytes());
		encoded.make_ascii_lowercase();
		encoded.truncate(24);
		Ok(encoded)
	}

	/// Client build identifier.
	pub fn client_build_id(&self) -> &str {
		&self.client_build_id
	}

	/// Vorma package version that produced this manifest.
	pub fn vorma_version(&self) -> &str {
		&self.vorma_version
	}

	/// Dev Vite server port.
	pub fn dev_vite_server_port(&self) -> i32 {
		self.dev_vite_server_port
	}

	/// Dev mux port.
	pub fn dev_mux_port(&self) -> i32 {
		self.dev_mux_port
	}

	/// Dev browser refresh token.
	pub fn dev_refresh_token(&self) -> &str {
		&self.dev_refresh_token
	}

	/// Normalized public static base.
	pub fn public_static_base(&self) -> &str {
		&self.public_static_base_path
	}

	/// UI adapter variant.
	pub fn ui_variant(&self) -> &str {
		&self.ui_variant
	}

	/// Root document shell hash.
	pub fn root_document_shell_hash(&self) -> &str {
		&self.root_document_shell_hash
	}

	/// Critical CSS content inserted into server-rendered HTML.
	pub fn critical_css(&self) -> &str {
		&self.critical_css
	}

	/// Critical CSS style element contract inserted into server-rendered HTML.
	pub fn critical_css_element(&self) -> DocumentElementContract {
		DocumentElementContract::new(HEAD_TAG_STYLE)
			.with_attributes(BTreeMap::from([(
				HEAD_ATTR_ID.to_owned(),
				CRITICAL_CSS_STYLE_ELEMENT_ID.to_owned(),
			)]))
			.with_dangerous_inner_html(self.critical_css())
	}

	/// SHA-256 hash of the rendered critical CSS style contents for CSP.
	pub fn critical_css_content_sha256(&self) -> Result<String, DocumentRenderError> {
		let parts = trusted_element_parts(&self.critical_css_element())?;
		let hash = Sha256::digest(parts.dangerous_inner_html.as_bytes());
		Ok(BASE64_STANDARD.encode(hash))
	}

	/// View search schemas keyed by route pattern.
	pub fn search_schemas(&self) -> &BTreeMap<String, Value> {
		&self.search_schemas
	}

	/// Manifest-listed public file paths.
	pub fn public_filepaths(&self) -> &[String] {
		&self.public_filepaths
	}

	/// Public source-to-URL map.
	pub fn public_filemap(&self) -> &BTreeMap<String, String> {
		&self.public_filemap
	}

	/// Runtime view modules.
	pub fn view_modules(&self) -> &[RuntimeViewModule] {
		&self.view_modules
	}

	/// Client entry module.
	pub fn client_entry(&self) -> &ClientModule {
		&self.client_entry
	}

	/// Client matcher/core assets.
	pub fn client_core_assets(&self) -> Option<&ClientCoreAssets> {
		self.client_core_assets.as_ref()
	}

	/// Runtime view module for a route pattern.
	pub fn view_module(&self, pattern: &str) -> Option<&RuntimeViewModule> {
		self.view_modules
			.iter()
			.find(|module| module.pattern() == pattern)
	}

	/// Resolve a public static source path through this manifest's public file map.
	pub fn public_url(&self, src_path: &str) -> Option<&str> {
		self.public_filemap
			.get(public_source_key(src_path)?)
			.map(String::as_str)
	}
}

/// Browser client module manifest entry.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ClientModule {
	url: String,
	dep_urls: Vec<String>,
	css_bundle_urls: Vec<String>,
}

impl ClientModule {
	/// Create a browser client module manifest entry.
	pub fn new(
		url: impl Into<String>,
		dep_urls: Vec<String>,
		css_bundle_urls: Vec<String>,
	) -> Self {
		Self {
			url: url.into(),
			dep_urls,
			css_bundle_urls,
		}
	}

	/// Browser import URL.
	pub fn url(&self) -> &str {
		&self.url
	}

	/// Module dependency URLs.
	pub fn dep_urls(&self) -> &[String] {
		&self.dep_urls
	}

	/// CSS bundle URLs.
	pub fn css_bundle_urls(&self) -> &[String] {
		&self.css_bundle_urls
	}
}

/// Client core assets emitted by the frontend build.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ClientCoreAssets {
	module_url: String,
	wasm_url: String,
}

impl ClientCoreAssets {
	/// Create client core asset metadata.
	pub fn new(module_url: impl Into<String>, wasm_url: impl Into<String>) -> Self {
		Self {
			module_url: module_url.into(),
			wasm_url: wasm_url.into(),
		}
	}

	/// Client core module URL.
	pub fn module_url(&self) -> &str {
		&self.module_url
	}

	/// Client core WASM URL.
	pub fn wasm_url(&self) -> &str {
		&self.wasm_url
	}
}

/// Runtime view module manifest entry.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct RuntimeViewModule {
	pattern: String,
	import_url: String,
	dep_urls: Vec<String>,
	css_bundle_urls: Vec<String>,
}

impl RuntimeViewModule {
	/// Create a runtime view module manifest entry.
	pub fn new(
		pattern: impl Into<String>,
		import_url: impl Into<String>,
		dep_urls: Vec<String>,
		css_bundle_urls: Vec<String>,
	) -> Self {
		Self {
			pattern: pattern.into(),
			import_url: import_url.into(),
			dep_urls,
			css_bundle_urls,
		}
	}

	/// View route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Browser import URL.
	pub fn import_url(&self) -> &str {
		&self.import_url
	}

	/// Module dependency URLs.
	pub fn dep_urls(&self) -> &[String] {
		&self.dep_urls
	}

	/// CSS bundle URLs.
	pub fn css_bundle_urls(&self) -> &[String] {
		&self.css_bundle_urls
	}
}

fn public_source_key(src_path: &str) -> Option<&str> {
	let clean = src_path.trim().trim_start_matches('/');
	if clean.is_empty() {
		return None;
	}
	Some(clean)
}

fn is_zero(value: &i32) -> bool {
	*value == 0
}

#[cfg(test)]
mod tests {
	use super::*;

	const TEST_CLIENT_BUILD_ID: &str = "build-id";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";

	#[test]
	fn manifest_indexes_view_modules_by_pattern() {
		let manifest = RuntimeManifest::new(
			TEST_CLIENT_BUILD_ID,
			TEST_PUBLIC_STATIC_BASE,
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/assets/root.js",
				vec!["/assets/shared.js".to_owned()],
				vec!["/assets/root.css".to_owned()],
			)],
		);

		let module = manifest.view_module("/").unwrap();

		assert_eq!(manifest.critical_css(), "");
		assert_eq!(module.import_url(), "/assets/root.js");
		assert_eq!(module.dep_urls(), ["/assets/shared.js"]);
		assert_eq!(module.css_bundle_urls(), ["/assets/root.css"]);
	}

	#[test]
	fn manifest_hashes_rendered_critical_css_content() {
		let manifest = RuntimeManifest::new(
			TEST_CLIENT_BUILD_ID,
			TEST_PUBLIC_STATIC_BASE,
			r#"body::before{content:"</style><script>x</script>"}"#,
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);
		let rendered = trusted_element_parts(&manifest.critical_css_element()).unwrap();
		let expected =
			BASE64_STANDARD.encode(Sha256::digest(rendered.dangerous_inner_html.as_bytes()));

		assert_eq!(manifest.critical_css_content_sha256().unwrap(), expected);
		assert!(rendered.dangerous_inner_html.contains("\\3C /style"));
	}

	#[test]
	fn manifest_serializes_roundtrippable_runtime_contract() {
		let manifest = RuntimeManifest::new(
			"",
			TEST_PUBLIC_STATIC_BASE,
			"body{}",
			BTreeMap::from([("/".to_owned(), serde_json::json!({"type": "object"}))]),
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				vec!["/static/shared.js".to_owned()],
				vec!["/static/root.css".to_owned()],
			)],
		);

		let bytes = manifest.to_json_vec().unwrap();
		let value: serde_json::Value = serde_json::from_slice(&bytes).unwrap();
		let decoded = RuntimeManifest::from_json_slice(&bytes).unwrap();

		assert_eq!(
			value["public_static_base_path"],
			serde_json::Value::String(TEST_PUBLIC_STATIC_BASE.to_owned())
		);
		assert!(value.get("public_static_base").is_none());
		assert!(value.get("client_build_id").is_none());
		assert_eq!(
			value["client_views"]["/"]["url"],
			serde_json::Value::String("/static/root.js".to_owned())
		);
		assert_eq!(
			decoded.client_build_id(),
			decoded.to_client_build_id().unwrap()
		);
		assert_eq!(
			decoded.view_module("/").unwrap().import_url(),
			"/static/root.js"
		);
		assert_eq!(
			decoded.public_url("/app.css").unwrap(),
			"/static/app.hash.css"
		);
	}
}
