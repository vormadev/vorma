use std::collections::{BTreeMap, BTreeSet};

use data_encoding::BASE32_NOPAD;
use serde::{Deserialize, Serialize};

use crate::config::validate_public_static_base_against_api_mount;
use crate::constants::CRITICAL_CSS_EL_ID;
use crate::htmlutil::{Element, compute_content_sha256};

#[doc(hidden)]
pub const MANIFEST_STATIC_OUT_DEV: &str = "vorma.manifest.dev.json";
#[doc(hidden)]
pub const MANIFEST_STATIC_OUT_PROD: &str = "vorma.manifest.prod.json";

#[doc(hidden)]
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ClientModule {
	pub url: String,
	pub dep_urls: Vec<String>,
	pub css_bundle_urls: Vec<String>,
}

#[doc(hidden)]
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ClientCoreAssets {
	pub module_url: String,
	pub wasm_url: String,
}

#[doc(hidden)]
#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub struct Manifest {
	pub vorma_version: String,

	#[serde(default, skip_serializing_if = "is_zero")]
	pub dev_vite_server_port: i32,
	#[serde(default, skip_serializing_if = "is_zero")]
	pub dev_mux_port: i32,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub dev_refresh_token: String,

	pub public_static_base_path: String,
	pub api_mount_root: String,
	pub ui_variant: String,
	pub root_document_shell_hash: String,

	pub public_filepaths: Vec<String>,
	pub public_filemap: BTreeMap<String, String>,
	pub critical_css: String,
	pub search_schemas: BTreeMap<String, serde_json::Value>,

	pub client_entry: ClientModule,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub client_core_assets: Option<ClientCoreAssets>,
	pub client_views: BTreeMap<String, ClientModule>,
}

impl Manifest {
	#[doc(hidden)]
	pub fn to_client_build_id(&self) -> Result<String, serde_json::Error> {
		let data = serde_json::to_vec(self)?;
		let hash = blake3::hash(&data);
		let mut encoded = BASE32_NOPAD.encode(hash.as_bytes());
		encoded.make_ascii_lowercase();
		encoded.truncate(24);
		Ok(encoded)
	}

	#[doc(hidden)]
	pub(crate) fn critical_css_el(&self) -> Element {
		Element {
			tag: "style".to_owned(),
			attributes: BTreeMap::from([("id".to_owned(), CRITICAL_CSS_EL_ID.to_owned())]),
			dangerous_inner_html: self.critical_css.clone(),
			..Element::default()
		}
	}

	#[doc(hidden)]
	pub fn critical_css_content_sha256(&self) -> String {
		compute_content_sha256(&self.critical_css_el())
	}

	#[doc(hidden)]
	pub fn public_url(&self, src_path: &str) -> Option<&str> {
		self.public_filemap
			.get(public_src_key(src_path)?)
			.map(String::as_str)
	}

	#[doc(hidden)]
	pub fn final_public_filepaths(&self) -> BTreeSet<String> {
		self.public_filepaths.iter().cloned().collect()
	}

	#[doc(hidden)]
	pub(crate) fn normalize_runtime_paths(&mut self) -> Result<(), String> {
		let (public_static_base_path, api_mount_root) =
			validate_public_static_base_against_api_mount(
				&self.public_static_base_path,
				&self.api_mount_root,
			)?;
		self.public_static_base_path = public_static_base_path;
		self.api_mount_root = api_mount_root;
		Ok(())
	}
}

pub(crate) fn public_src_key(src_path: &str) -> Option<&str> {
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
	use crate::htmlutil::render_element;

	#[test]
	fn manifest_json_keys_are_snake_case() {
		let manifest = Manifest {
			vorma_version: "0.1.0".to_owned(),
			public_static_base_path: "/public/".to_owned(),
			api_mount_root: "/api/".to_owned(),
			ui_variant: "react".to_owned(),
			root_document_shell_hash: "shell-hash".to_owned(),
			public_filepaths: vec!["/public/app.css".to_owned()],
			public_filemap: BTreeMap::from([(
				"app.css".to_owned(),
				"/public/app_hash.css".to_owned(),
			)]),
			critical_css: "body{}".to_owned(),
			search_schemas: BTreeMap::new(),
			client_entry: ClientModule {
				url: "/public/entry.js".to_owned(),
				dep_urls: Vec::new(),
				css_bundle_urls: Vec::new(),
			},
			client_views: BTreeMap::new(),
			..Manifest::default()
		};

		let json = serde_json::to_value(&manifest).unwrap();

		assert!(json.get("vorma_version").is_some());
		assert!(json.get("public_static_base_path").is_some());
		assert!(json.get("api_mount_root").is_some());
		assert!(json.get("root_document_shell_hash").is_some());
		assert!(json.get("root_html_template_hash").is_none());
		assert!(json.get("client_entry").unwrap().get("url").is_some());
		assert!(json.get("client_entry").unwrap().get("dep_urls").is_some());
		assert!(
			json.get("client_entry")
				.unwrap()
				.get("css_bundle_urls")
				.is_some()
		);
	}

	#[test]
	fn client_build_id_is_stable_base32_blake3_of_json_manifest() {
		let manifest = Manifest {
			vorma_version: "0.1.0".to_owned(),
			root_document_shell_hash: "shell-hash".to_owned(),
			client_entry: ClientModule {
				url: "/entry.js".to_owned(),
				..ClientModule::default()
			},
			..Manifest::default()
		};

		let first = manifest.to_client_build_id().unwrap();
		let second = manifest.to_client_build_id().unwrap();

		assert_eq!(first, second);
		assert_eq!(first.len(), 24);
		assert!(
			first
				.chars()
				.all(|ch| ch.is_ascii_lowercase() || ch.is_ascii_digit())
		);
	}

	#[test]
	fn public_url_trims_leading_slash_and_whitespace() {
		let manifest = Manifest {
			public_filemap: BTreeMap::from([(
				"favicon.ico".to_owned(),
				"/public/favicon_hash.ico".to_owned(),
			)]),
			..Manifest::default()
		};

		assert_eq!(
			manifest.public_url(" /favicon.ico "),
			Some("/public/favicon_hash.ico")
		);
		assert_eq!(manifest.public_url(" "), None);
	}

	#[test]
	fn manifest_runtime_paths_reject_overlapping_public_static_base() {
		let mut manifest = Manifest {
			public_static_base_path: "/api/assets/".to_owned(),
			api_mount_root: "/api/".to_owned(),
			..Manifest::default()
		};

		assert_eq!(
			manifest.normalize_runtime_paths().unwrap_err(),
			"public_static_base \"/api/assets/\" must not be under api_base \"/api/\""
		);
	}

	#[test]
	fn critical_css_el_escapes_raw_text_end_tag() {
		let manifest = Manifest {
			critical_css: "body{}/* </StYlE><script>alert(1)</script> */".to_owned(),
			..Manifest::default()
		};

		let html = render_element(&manifest.critical_css_el()).unwrap();

		assert!(!html.to_ascii_lowercase().contains("</style><script>"));
		assert!(html.contains(r"\3C /StYlE><script>alert(1)</script>"));
		assert!(html.ends_with("</style>"));
	}
}
