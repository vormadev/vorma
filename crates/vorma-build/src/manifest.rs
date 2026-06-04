use std::collections::BTreeMap;
mod public_output;
mod vite_projection;

use std::path::Path;

use path_slash::PathExt;
use vorma::__private::manifest::{ClientCoreAssets, ClientModule, Manifest};

use crate::build_layout::RetainedViteManifest;
use crate::config::{VormaCfg, relative_path};
use crate::constants::DEV_LOOPBACK_HOST;
use crate::generation::{LiveMetadata, StaticMetadata};
use crate::manifest::public_output::{
	collect_public_filepaths, validate_dev_manifest_public_file_outputs,
	validate_prod_manifest_public_file_outputs,
};
use crate::ts_modules::TsViewModule;
use crate::utils::write_json_to_file;
use crate::viteutil::ViteManifest;

const VORMA_PACKAGE_VERSION: &str = env!("CARGO_PKG_VERSION");

#[derive(Clone, Debug, Default, PartialEq)]
pub(crate) struct ManifestGenerationData {
	pub(crate) ts_modules: BTreeMap<String, TsViewModule>,
	pub(crate) pub_fm: BTreeMap<String, String>,
	pub(crate) critical_css: String,
	pub(crate) search_schemas: BTreeMap<String, serde_json::Value>,
	pub(crate) root_document_hash_source: String,
}

impl ManifestGenerationData {
	pub(crate) fn from_generation_metadata(
		live: &LiveMetadata,
		static_metadata: &StaticMetadata,
	) -> Self {
		Self {
			ts_modules: live.view_modules.clone(),
			pub_fm: static_metadata.public_filemap.clone(),
			critical_css: static_metadata.critical_css.clone(),
			search_schemas: live.search_schemas.clone(),
			root_document_hash_source: live.root_document_hash_source.clone(),
		}
	}
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct DevManifestInput {
	pub(crate) vite_server_port: i32,
	pub(crate) dev_mux_port: i32,
	pub(crate) dev_refresh_token: String,
	pub(crate) generation: ManifestGenerationData,
}

impl DevManifestInput {
	pub(crate) fn from_generation_metadata(
		vite_server_port: i32,
		dev_mux_port: i32,
		dev_refresh_token: String,
		live: &LiveMetadata,
		static_metadata: &StaticMetadata,
	) -> Self {
		Self {
			vite_server_port,
			dev_mux_port,
			dev_refresh_token,
			generation: ManifestGenerationData::from_generation_metadata(live, static_metadata),
		}
	}
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct ProdManifestInput {
	pub(crate) retained_vite_manifest: RetainedViteManifest,
	pub(crate) generation: ManifestGenerationData,
}

impl ProdManifestInput {
	pub(crate) fn from_generation_metadata(
		retained_vite_manifest: RetainedViteManifest,
		live: &LiveMetadata,
		static_metadata: &StaticMetadata,
	) -> Self {
		Self {
			retained_vite_manifest,
			generation: ManifestGenerationData::from_generation_metadata(live, static_metadata),
		}
	}
}

pub(crate) fn write_dev_manifest(
	cfg: &VormaCfg<'_>,
	input: &DevManifestInput,
) -> Result<Manifest, String> {
	let manifest = prepare_dev_manifest(cfg, input)?;
	write_json_to_file(&manifest, cfg.manifest_json_out(true))
		.map_err(|err| format!("error writing vorma manifest json: {err}"))?;
	Ok(manifest)
}

pub(crate) fn write_prod_manifest(
	cfg: &VormaCfg<'_>,
	input: &ProdManifestInput,
) -> Result<Manifest, String> {
	let manifest = prepare_prod_manifest(cfg, input)?;
	write_json_to_file(&manifest, cfg.manifest_json_out(false))
		.map_err(|err| format!("error writing vorma manifest json: {err}"))?;
	Ok(manifest)
}

pub(crate) fn prepare_dev_manifest(
	cfg: &VormaCfg<'_>,
	input: &DevManifestInput,
) -> Result<Manifest, String> {
	if input.vite_server_port <= 0 {
		return Err("dev manifest requires Vite server port".to_owned());
	}

	let mut ts_views = BTreeMap::new();
	let to_url = |p: &str| -> String {
		format!(
			"http://{}:{}/{}",
			DEV_LOOPBACK_HOST,
			input.vite_server_port,
			Path::new(p).to_slash_lossy()
		)
	};

	let ts_entry_rel = relative_path(cfg.js_package_manager_dir(), cfg.ts_entry())
		.ok_or_else(|| "error getting relative path for TS entry".to_owned())?;
	let ts_entry_cm = ClientModule {
		url: to_url(&ts_entry_rel.to_string_lossy()),
		dep_urls: Vec::new(),
		css_bundle_urls: Vec::new(),
	};

	for (pattern, r) in &input.generation.ts_modules {
		let ip_rel = relative_path(
			cfg.js_package_manager_dir(),
			cfg.root_path_string(&r.import_path),
		)
		.ok_or_else(|| {
			format!(
				"error getting relative path for TS view (pattern: {}, import path: {})",
				pattern, r.import_path,
			)
		})?;
		ts_views.insert(
			pattern.clone(),
			ClientModule {
				url: to_url(&ip_rel.to_string_lossy()),
				dep_urls: Vec::new(),
				css_bundle_urls: Vec::new(),
			},
		);
	}

	let mut manifest =
		prepare_manifest_from_client_modules(cfg, &input.generation, ts_entry_cm, None, ts_views)?;
	manifest.dev_vite_server_port = input.vite_server_port;
	manifest.dev_mux_port = input.dev_mux_port;
	manifest.dev_refresh_token = input.dev_refresh_token.clone();

	validate_dev_manifest_public_file_outputs(&manifest)?;

	Ok(manifest)
}

pub(crate) fn prepare_prod_manifest(
	cfg: &VormaCfg<'_>,
	input: &ProdManifestInput,
) -> Result<Manifest, String> {
	let vite_manifest = ViteManifest::read(input.retained_vite_manifest.path())
		.map_err(|err| format!("error reading Vite manifest: {err}"))?;

	let client_core_assets = cfg
		.to_client_core_assets(&vite_manifest)
		.map_err(|err| format!("error processing Vorma client core assets: {err}"))?;

	let ts_entry_src = cfg.ts_entry();
	let ts_entry_cm = cfg
		.to_client_module(&vite_manifest, &ts_entry_src)
		.map_err(|err| format!("error processing TS entry module for manifest: {err}"))?;

	let mut ts_views = BTreeMap::new();
	for (pattern, r) in &input.generation.ts_modules {
		ts_views.insert(
			pattern.clone(),
			cfg.to_client_module(&vite_manifest, &r.import_path)
				.map_err(|err| {
					format!(
						"error processing TS view module for manifest (pattern: {}, import path: {}): {}",
						pattern, r.import_path, err,
					)
				})?,
		);
	}

	let manifest = prepare_manifest_from_client_modules(
		cfg,
		&input.generation,
		ts_entry_cm,
		Some(client_core_assets),
		ts_views,
	)?;

	validate_prod_manifest_public_file_outputs(&manifest)?;

	Ok(manifest)
}

fn prepare_manifest_from_client_modules(
	cfg: &VormaCfg<'_>,
	generation: &ManifestGenerationData,
	client_entry: ClientModule,
	client_core_assets: Option<ClientCoreAssets>,
	client_views: BTreeMap<String, ClientModule>,
) -> Result<Manifest, String> {
	let public_filepaths = collect_public_filepaths(cfg)?;
	let vorma_version = VORMA_PACKAGE_VERSION.to_owned();

	Ok(Manifest {
		vorma_version,
		public_static_base_path: cfg.public_static_base_path(),
		api_mount_root: cfg.api_mount_root(),
		ui_variant: cfg.ui_variant(),
		root_document_shell_hash: root_document_shell_hash(&generation.root_document_hash_source),
		public_filepaths,
		public_filemap: generation.pub_fm.clone(),
		critical_css: generation.critical_css.clone(),
		search_schemas: generation.search_schemas.clone(),
		client_entry,
		client_core_assets,
		client_views,
		..Manifest::default()
	})
}

fn root_document_shell_hash(root_document_hash_source: &str) -> String {
	blake3::hash(root_document_hash_source.trim().as_bytes())
		.to_hex()
		.to_string()
}

#[cfg(test)]
mod tests {
	use super::*;
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::time::{SystemTime, UNIX_EPOCH};
	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig};

	use crate::config::to_cfg;
	use crate::manifest::vite_projection::CLIENT_CORE_WASM_SOURCE_FILENAME;
	use crate::viteutil::ViteManifestChunk;
	use vorma::Document;

	fn dev_manifest_input(vite_server_port: i32) -> DevManifestInput {
		DevManifestInput {
			vite_server_port,
			dev_mux_port: 3000,
			dev_refresh_token: "refresh".to_owned(),
			generation: ManifestGenerationData {
				root_document_hash_source: "<html></html>".to_owned(),
				..ManifestGenerationData::default()
			},
		}
	}

	fn prod_manifest_input(retained_vite_manifest: RetainedViteManifest) -> ProdManifestInput {
		ProdManifestInput {
			retained_vite_manifest,
			generation: ManifestGenerationData {
				root_document_hash_source: "<html></html>".to_owned(),
				..ManifestGenerationData::default()
			},
		}
	}

	#[test]
	fn to_client_module_translates_vite_manifest_deps_to_public_urls() {
		let manifest = ViteManifest::from(BTreeMap::from([
			(
				"src/entry.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					imports: vec!["src/chunk.ts".to_owned(), "src/shared.ts".to_owned()],
					css: vec!["assets/entry.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"src/chunk.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/chunk.js".to_owned(),
					imports: vec!["src/shared.ts".to_owned()],
					css: vec!["assets/chunk.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"src/shared.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/shared.js".to_owned(),
					css: vec!["assets/shared.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
		]));
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();

		let got = cfg.to_client_module(&manifest, "src/entry.tsx").unwrap();

		assert_eq!(got.url, "/static/assets/entry.js");
		assert_eq!(
			got.dep_urls,
			[
				"/static/assets/entry.js",
				"/static/assets/chunk.js",
				"/static/assets/shared.js",
			]
		);
		assert_eq!(
			got.css_bundle_urls,
			[
				"/static/assets/entry.css",
				"/static/assets/chunk.css",
				"/static/assets/shared.css",
			]
		);
	}

	#[test]
	fn to_client_module_uses_path_relative_to_js_package_manager_dir() {
		let manifest = ViteManifest::from(BTreeMap::from([(
			"../app/src/entry.tsx".to_owned(),
			ViteManifestChunk {
				file: "assets/entry.js".to_owned(),
				..ViteManifestChunk::default()
			},
		)]));
		let config = config("frontend");
		let cfg = to_cfg(&config).unwrap();

		let got = cfg
			.to_client_module(&manifest, "app/src/entry.tsx")
			.unwrap();

		assert_eq!(got.url, "/static/assets/entry.js");
	}

	#[test]
	fn to_client_module_errors_when_manifest_entry_is_missing() {
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();
		let error = cfg
			.to_client_module(&ViteManifest::default(), "src/missing.tsx")
			.unwrap_err();

		assert_eq!(
			error,
			"error finding module in Vite manifest: src/missing.tsx"
		);
	}

	#[test]
	fn to_client_core_assets_finds_wrapper_and_wasm_from_vite_manifest() {
		let module_out = "vorma_out_vite_vorma_client_wasm.js";
		let wasm_src = format!("../../pkg/npm/.dist/vorma/core/{CLIENT_CORE_WASM_SOURCE_FILENAME}");
		let wasm_out = format!("vorma_out_vite_{CLIENT_CORE_WASM_SOURCE_FILENAME}");
		let manifest = ViteManifest::from(BTreeMap::from([
			(
				"../../pkg/npm/.dist/vorma/core/vorma_client_wasm-BiuOZDrD.js".to_owned(),
				ViteManifestChunk {
					file: module_out.to_owned(),
					src: "../../pkg/npm/.dist/vorma/core/vorma_client_wasm-BiuOZDrD.js".to_owned(),
					is_dynamic_entry: true,
					assets: vec![wasm_out.clone()],
					..ViteManifestChunk::default()
				},
			),
			(
				wasm_src.clone(),
				ViteManifestChunk {
					file: wasm_out.clone(),
					src: wasm_src,
					..ViteManifestChunk::default()
				},
			),
			(
				"node_modules/mdream/wasm/mdream_edge_bg.wasm".to_owned(),
				ViteManifestChunk {
					file: "vorma_out_vite_mdream_edge_bg.wasm".to_owned(),
					src: "node_modules/mdream/wasm/mdream_edge_bg.wasm".to_owned(),
					..ViteManifestChunk::default()
				},
			),
		]));
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();

		let got = cfg.to_client_core_assets(&manifest).unwrap();

		assert_eq!(got.module_url, format!("/static/{module_out}"));
		assert_eq!(got.wasm_url, format!("/static/{wasm_out}"));
	}

	#[test]
	fn to_client_core_assets_rejects_missing_vorma_wasm_asset() {
		let manifest = ViteManifest::from(BTreeMap::from([(
			"node_modules/mdream/wasm/mdream_edge_bg.wasm".to_owned(),
			ViteManifestChunk {
				file: "vorma_out_vite_mdream_edge_bg.wasm".to_owned(),
				src: "node_modules/mdream/wasm/mdream_edge_bg.wasm".to_owned(),
				..ViteManifestChunk::default()
			},
		)]));
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();

		assert_eq!(
			cfg.to_client_core_assets(&manifest).unwrap_err(),
			"Vite manifest does not contain Vorma client WASM asset",
		);
	}

	#[test]
	fn to_client_module_rejects_invalid_vite_output_paths() {
		let manifest = ViteManifest::from(BTreeMap::from([(
			"src/entry.tsx".to_owned(),
			ViteManifestChunk {
				file: "../entry.js".to_owned(),
				..ViteManifestChunk::default()
			},
		)]));
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();

		let error = cfg
			.to_client_module(&manifest, "src/entry.tsx")
			.unwrap_err();

		assert_eq!(
			error,
			"Vite manifest module file contains an invalid path segment: \"../entry.js\""
		);
	}

	#[test]
	fn write_manifest_dev_writes_manifest_json_and_collects_public_filepaths() {
		let dir = temp_dir("write-manifest-dev");
		fs::create_dir_all(dir.join(".vorma/static/public/nested")).unwrap();
		fs::write(dir.join(".vorma/static/public/z.js"), "z").unwrap();
		fs::write(dir.join(".vorma/static/public/nested/a.css"), "a").unwrap();

		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let input = DevManifestInput {
			vite_server_port: 5173,
			dev_mux_port: 3000,
			dev_refresh_token: "refresh".to_owned(),
			generation: ManifestGenerationData {
				root_document_hash_source: "\n<html></html>\n".to_owned(),
				ts_modules: BTreeMap::from([(
					"/item/:id".to_owned(),
					TsViewModule {
						pattern: "/item/:id".to_owned(),
						import_path: "src/item.tsx".to_owned(),
						deps: Vec::new(),
					},
				)]),
				pub_fm: BTreeMap::from([("z.js".to_owned(), "/static/z.js".to_owned())]),
				critical_css: "body{}".to_owned(),
				search_schemas: BTreeMap::new(),
			},
		};

		let manifest = write_dev_manifest(&cfg, &input).unwrap();

		assert_eq!(
			manifest.public_filepaths,
			["/static/nested/a.css", "/static/z.js"]
		);
		assert_eq!(
			manifest.client_entry.url,
			"http://127.0.0.1:5173/src/entry.tsx"
		);
		assert_eq!(
			manifest.client_views["/item/:id"].url,
			"http://127.0.0.1:5173/src/item.tsx"
		);
		assert_eq!(manifest.dev_vite_server_port, 5173);
		assert_eq!(manifest.dev_mux_port, 3000);
		assert_eq!(manifest.dev_refresh_token, "refresh");
		assert_eq!(
			manifest.root_document_shell_hash,
			root_document_shell_hash("<html></html>")
		);
		assert!(Path::new(&cfg.manifest_json_out(true)).exists());

		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn prepare_dev_manifest_rejects_missing_vite_port() {
		let dir = temp_dir("dev-manifest-without-vite-port");
		fs::create_dir_all(dir.join(".vorma/static/public")).unwrap();
		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let input = dev_manifest_input(0);

		assert_eq!(
			prepare_dev_manifest(&cfg, &input).unwrap_err(),
			"dev manifest requires Vite server port"
		);
		assert!(!Path::new(&cfg.manifest_json_out(true)).exists());

		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_document_hash_changes_with_default_head() {
		let mut first = Document::new();
		first.head().title("First");
		let mut second = Document::new();
		second.head().title("Second");

		assert_ne!(
			root_document_shell_hash(&crate::document_hash::hash_source(&first).unwrap()),
			root_document_shell_hash(&crate::document_hash::hash_source(&second).unwrap())
		);
	}

	#[test]
	fn write_manifest_uses_crate_package_version() {
		let config = config(".");
		let cfg = to_cfg(&config).unwrap();
		let input = dev_manifest_input(3000);

		assert_eq!(
			prepare_dev_manifest(&cfg, &input).unwrap().vorma_version,
			VORMA_PACKAGE_VERSION
		);
	}

	#[test]
	fn prepare_prod_manifest_reads_retained_vite_manifest() {
		let dir = temp_dir("prepare-manifest-prod-retained-vite-manifest");
		let public_dir = dir.join(".vorma/static/public");
		fs::create_dir_all(public_dir.join("assets")).unwrap();
		fs::write(public_dir.join("assets/entry.js"), "entry").unwrap();
		write_vorma_client_core_outputs(&public_dir);
		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let mut vite_manifest = BTreeMap::from([(
			"src/entry.tsx".to_owned(),
			ViteManifestChunk {
				file: "assets/entry.js".to_owned(),
				..ViteManifestChunk::default()
			},
		)]);
		insert_vorma_client_core_chunks(&mut vite_manifest);
		let vite_manifest = ViteManifest::from(vite_manifest);
		let prod_vite_manifest = write_vite_manifest(&cfg, &vite_manifest);
		let input = prod_manifest_input(prod_vite_manifest);

		let manifest = prepare_prod_manifest(&cfg, &input).unwrap();

		assert_eq!(manifest.client_entry.url, "/static/assets/entry.js");
		assert!(cfg.build_layout().prod_vite_manifest_out().exists());
		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_prod_reads_retained_vite_manifest_and_writes_client_assets() {
		let dir = temp_dir("write-manifest-prod");
		let public_dir = dir.join(".vorma/static/public");
		fs::create_dir_all(public_dir.join("assets")).unwrap();
		fs::write(public_dir.join("assets/entry.js"), "entry").unwrap();
		fs::write(public_dir.join("assets/chunk.js"), "chunk").unwrap();
		fs::write(public_dir.join("assets/item.js"), "item").unwrap();
		fs::write(public_dir.join("assets/entry.css"), "css").unwrap();
		fs::write(public_dir.join("assets/wasm-wrapper.js"), "wrapper").unwrap();
		fs::write(public_dir.join("assets/vorma_client_wasm_bg.wasm"), "wasm").unwrap();

		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let vite_manifest = ViteManifest::from(BTreeMap::from([
			(
				"src/entry.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					imports: vec!["src/chunk.ts".to_owned()],
					css: vec!["assets/entry.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"src/chunk.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/chunk.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
			(
				"src/item.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/item.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
			(
				"src/wasm-wrapper.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/wasm-wrapper.js".to_owned(),
					assets: vec!["assets/vorma_client_wasm_bg.wasm".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"pkg/vorma_client_wasm_bg.wasm".to_owned(),
				ViteManifestChunk {
					src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
					file: "assets/vorma_client_wasm_bg.wasm".to_owned(),
					..ViteManifestChunk::default()
				},
			),
		]));
		let prod_vite_manifest = write_vite_manifest(&cfg, &vite_manifest);
		let tmp_vite_manifest_out = cfg.prod_tmp_vite_manifest_out();
		fs::create_dir_all(Path::new(&tmp_vite_manifest_out).parent().unwrap()).unwrap();
		fs::write(&tmp_vite_manifest_out, "{}").unwrap();

		let input = ProdManifestInput {
			retained_vite_manifest: prod_vite_manifest,
			generation: ManifestGenerationData {
				root_document_hash_source: "<html></html>".to_owned(),
				ts_modules: BTreeMap::from([(
					"/item/:id".to_owned(),
					TsViewModule {
						pattern: "/item/:id".to_owned(),
						import_path: "src/item.tsx".to_owned(),
						deps: Vec::new(),
					},
				)]),
				pub_fm: BTreeMap::from([(
					"assets/entry.js".to_owned(),
					"/static/assets/entry.js".to_owned(),
				)]),
				critical_css: "body{}".to_owned(),
				search_schemas: BTreeMap::new(),
			},
		};

		let manifest = write_prod_manifest(&cfg, &input).unwrap();

		assert_eq!(manifest.client_entry.url, "/static/assets/entry.js");
		assert_eq!(
			manifest.client_entry.dep_urls,
			["/static/assets/entry.js", "/static/assets/chunk.js"]
		);
		assert_eq!(
			manifest.client_entry.css_bundle_urls,
			["/static/assets/entry.css"]
		);
		assert_eq!(
			manifest.client_views["/item/:id"].url,
			"/static/assets/item.js"
		);
		assert_eq!(
			manifest.client_core_assets.unwrap().module_url,
			"/static/assets/wasm-wrapper.js"
		);
		assert!(cfg.build_layout().prod_vite_manifest_out().exists());
		assert!(Path::new(&tmp_vite_manifest_out).exists());
		assert!(
			!manifest
				.public_filepaths
				.iter()
				.any(|path| path.ends_with("vorma_internal_tmp_vite_manifest.json"))
		);
		assert!(Path::new(&cfg.manifest_json_out(false)).exists());

		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_errors_when_public_filemap_url_is_missing_from_public_output() {
		let dir = temp_dir("write-manifest-missing-public-filemap");
		fs::create_dir_all(dir.join(".vorma/static/public")).unwrap();
		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let input = DevManifestInput {
			vite_server_port: 5173,
			dev_mux_port: 3000,
			dev_refresh_token: "refresh".to_owned(),
			generation: ManifestGenerationData {
				root_document_hash_source: "<html></html>".to_owned(),
				pub_fm: BTreeMap::from([(
					"missing.svg".to_owned(),
					"/static/missing.svg".to_owned(),
				)]),
				..ManifestGenerationData::default()
			},
		};

		let error = write_dev_manifest(&cfg, &input).unwrap_err();

		assert_eq!(
			error,
			"manifest public filemap URL is not present in public output: /static/missing.svg"
		);
		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_prod_errors_when_client_asset_is_missing_from_public_output() {
		let dir = temp_dir("write-manifest-missing-client-asset");
		let public_dir = dir.join(".vorma/static/public");
		write_vorma_client_core_outputs(&public_dir);
		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let mut vite_manifest = BTreeMap::from([(
			"src/entry.tsx".to_owned(),
			ViteManifestChunk {
				file: "assets/missing-entry.js".to_owned(),
				..ViteManifestChunk::default()
			},
		)]);
		insert_vorma_client_core_chunks(&mut vite_manifest);
		let vite_manifest = ViteManifest::from(vite_manifest);
		let prod_vite_manifest = write_vite_manifest(&cfg, &vite_manifest);
		let input = prod_manifest_input(prod_vite_manifest);

		let error = write_prod_manifest(&cfg, &input).unwrap_err();

		assert_eq!(
			error,
			"manifest client asset URL is not present in public output: /static/assets/missing-entry.js"
		);
		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_prod_keeps_retained_vite_manifest_when_final_manifest_write_fails() {
		let dir = temp_dir("write-manifest-prod-write-fails");
		let public_dir = dir.join(".vorma/static/public");
		fs::create_dir_all(public_dir.join("assets")).unwrap();
		fs::write(public_dir.join("assets/entry.js"), "entry").unwrap();
		write_vorma_client_core_outputs(&public_dir);
		let config = Config {
			root_dir: dir.clone(),
			dist_dir: dir.to_string_lossy().into_owned(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let cfg = to_cfg(&config).unwrap();
		let mut vite_manifest = BTreeMap::from([(
			"src/entry.tsx".to_owned(),
			ViteManifestChunk {
				file: "assets/entry.js".to_owned(),
				..ViteManifestChunk::default()
			},
		)]);
		insert_vorma_client_core_chunks(&mut vite_manifest);
		let vite_manifest = ViteManifest::from(vite_manifest);
		let prod_vite_manifest = write_vite_manifest(&cfg, &vite_manifest);
		fs::create_dir_all(cfg.manifest_json_out(false)).unwrap();
		let input = prod_manifest_input(prod_vite_manifest);

		let error = write_prod_manifest(&cfg, &input).unwrap_err();

		assert!(error.contains("error writing vorma manifest json"));
		assert!(cfg.build_layout().prod_vite_manifest_out().exists());
		fs::remove_dir_all(dir).unwrap();
	}

	fn config(js_package_manager_dir: &str) -> Config {
		Config {
			root_dir: crate::test_support::root_dir(),
			server_config: server_config(),
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: js_package_manager_dir.to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		}
	}

	fn server_config() -> ServerConfig {
		ServerConfig {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-server".to_owned(),
		}
	}

	fn insert_vorma_client_core_chunks(manifest: &mut BTreeMap<String, ViteManifestChunk>) {
		manifest.insert(
			"src/wasm-wrapper.ts".to_owned(),
			ViteManifestChunk {
				file: "assets/wasm-wrapper.js".to_owned(),
				assets: vec!["assets/vorma_client_wasm_bg.wasm".to_owned()],
				..ViteManifestChunk::default()
			},
		);
		manifest.insert(
			"pkg/vorma_client_wasm_bg.wasm".to_owned(),
			ViteManifestChunk {
				src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
				file: "assets/vorma_client_wasm_bg.wasm".to_owned(),
				..ViteManifestChunk::default()
			},
		);
	}

	fn write_vorma_client_core_outputs(public_dir: &Path) {
		fs::create_dir_all(public_dir.join("assets")).unwrap();
		fs::write(public_dir.join("assets/wasm-wrapper.js"), "wrapper").unwrap();
		fs::write(public_dir.join("assets/vorma_client_wasm_bg.wasm"), "wasm").unwrap();
	}

	fn write_vite_manifest(cfg: &VormaCfg<'_>, manifest: &ViteManifest) -> RetainedViteManifest {
		let out = cfg.build_layout().prod_vite_manifest_out();
		fs::create_dir_all(out.parent().unwrap()).unwrap();
		fs::write(&out, serde_json::to_vec(manifest).unwrap()).unwrap();
		RetainedViteManifest::new(out)
	}

	fn temp_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-build-{name}-{nonce}"))
	}
}
