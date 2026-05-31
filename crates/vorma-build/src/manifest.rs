use std::collections::BTreeMap;
use std::path::Path;

use path_slash::PathExt;
use vorma::__private::constants::PROD_TMP_VITE_MANIFEST_FILENAME;
use vorma::__private::manifest::{ClientCoreAssets, ClientModule, Manifest};
use walkdir::WalkDir;

use crate::config::{VormaCfg, relative_path};
use crate::constants::DEV_LOOPBACK_HOST;
use crate::generation::{LiveMetadata, StaticMetadata};
use crate::ts_modules::TsRoute;
use crate::utils::write_json_to_file;
use crate::viteutil::ViteManifest;

const CLIENT_CORE_WASM_SOURCE_FILENAME: &str = "vorma_client_wasm_bg.wasm";
const VORMA_PACKAGE_VERSION: &str = env!("CARGO_PKG_VERSION");

#[derive(Clone, Debug, Default, PartialEq)]
pub(crate) struct ManifestInput {
	pub(crate) is_dev: bool,
	pub(crate) vite_server_port: i32,
	pub(crate) dev_mux_port: i32,
	pub(crate) dev_refresh_token: String,
	pub(crate) ts_modules: BTreeMap<String, TsRoute>,
	pub(crate) pub_fm: BTreeMap<String, String>,
	pub(crate) critical_css: String,
	pub(crate) search_schemas: BTreeMap<String, serde_json::Value>,
	pub(crate) root_document_hash_source: String,
}

impl ManifestInput {
	pub(crate) fn from_generation_metadata(
		is_dev: bool,
		vite_server_port: i32,
		dev_mux_port: i32,
		dev_refresh_token: String,
		live: &LiveMetadata,
		static_metadata: &StaticMetadata,
	) -> Self {
		Self {
			is_dev,
			vite_server_port,
			dev_mux_port,
			dev_refresh_token,
			ts_modules: live.route_modules.clone(),
			pub_fm: static_metadata.public_filemap.clone(),
			critical_css: static_metadata.critical_css.clone(),
			search_schemas: live.search_schemas.clone(),
			root_document_hash_source: live.root_document_hash_source.clone(),
		}
	}
}

pub(crate) fn write_manifest(
	cfg: &VormaCfg<'_>,
	input: &ManifestInput,
) -> Result<Manifest, String> {
	let manifest = prepare_manifest(cfg, input)?;
	write_json_to_file(&manifest, cfg.manifest_json_out(input.is_dev))
		.map_err(|err| format!("error writing vorma manifest json: {err}"))?;
	if !input.is_dev {
		cfg.remove_prod_tmp_vite_manifest()
			.map_err(|err| format!("error removing temporary Vite manifest: {err}"))?;
	}
	Ok(manifest)
}

pub(crate) fn prepare_manifest(
	cfg: &VormaCfg<'_>,
	input: &ManifestInput,
) -> Result<Manifest, String> {
	if input.is_dev && input.vite_server_port <= 0 {
		return Err("dev manifest requires Vite server port".to_owned());
	}

	let mut ts_routes = BTreeMap::new();

	let (ts_entry_cm, client_core_assets) = if input.is_dev {
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

		for (pattern, r) in &input.ts_modules {
			let ip_rel = relative_path(
				cfg.js_package_manager_dir(),
				cfg.root_path_string(&r.import_path),
			)
			.ok_or_else(|| {
				format!(
					"error getting relative path for TS route (pattern: {}, import path: {})",
					pattern, r.import_path,
				)
			})?;
			ts_routes.insert(
				pattern.clone(),
				ClientModule {
					url: to_url(&ip_rel.to_string_lossy()),
					dep_urls: Vec::new(),
					css_bundle_urls: Vec::new(),
				},
			);
		}

		(ts_entry_cm, None)
	} else {
		let vite_manifest = ViteManifest::read(cfg.prod_tmp_vite_manifest_out())
			.map_err(|err| format!("error reading Vite manifest: {err}"))?;

		let client_core_assets = cfg
			.to_client_core_assets(&vite_manifest)
			.map_err(|err| format!("error processing Vorma client core assets: {err}"))?;

		let ts_entry_src = cfg.ts_entry();
		let ts_entry_cm = cfg
			.to_client_module(&vite_manifest, &ts_entry_src)
			.map_err(|err| format!("error processing TS entry module for manifest: {err}"))?;

		for (pattern, r) in &input.ts_modules {
			ts_routes.insert(
				pattern.clone(),
				cfg.to_client_module(&vite_manifest, &r.import_path)
					.map_err(|err| {
						format!(
							"error processing TS route module for manifest (pattern: {}, import path: {}): {}",
							pattern, r.import_path, err,
						)
					})?,
			);
		}

		(ts_entry_cm, Some(client_core_assets))
	};

	let public_filepaths = collect_public_filepaths(cfg)?;
	let vorma_version = VORMA_PACKAGE_VERSION.to_owned();

	let mut manifest = Manifest {
		vorma_version,
		public_static_base_path: cfg.public_static_base_path(),
		api_mount_root: cfg.api_mount_root(),
		ui_variant: cfg.ui_variant(),
		root_document_shell_hash: root_document_shell_hash(&input.root_document_hash_source),
		public_filepaths,
		public_filemap: input.pub_fm.clone(),
		critical_css: input.critical_css.clone(),
		search_schemas: input.search_schemas.clone(),
		client_entry: ts_entry_cm,
		client_core_assets,
		client_routes: ts_routes,
		..Manifest::default()
	};

	if input.is_dev {
		manifest.dev_vite_server_port = input.vite_server_port;
		manifest.dev_mux_port = input.dev_mux_port;
		manifest.dev_refresh_token = input.dev_refresh_token.clone();
	}

	validate_manifest_public_file_outputs(input.is_dev, &manifest)?;

	Ok(manifest)
}

impl VormaCfg<'_> {
	pub(crate) fn to_client_module(
		&self,
		manifest: &ViteManifest,
		import_path: &str,
	) -> Result<ClientModule, String> {
		let base = self.public_static_base_path();

		let import_path = relative_path(
			self.js_package_manager_dir(),
			self.root_path_string(import_path),
		)
		.ok_or_else(|| "error getting relative import path".to_owned())?
		.to_slash_lossy()
		.into_owned();

		let Some(own_chunk) = manifest.get(&import_path) else {
			return Err(format!(
				"error finding module in Vite manifest: {import_path}"
			));
		};
		let own_file = vite_public_url(&base, &own_chunk.file, "module file")?;

		let deps_res = manifest.find_all_deps(&import_path)?;
		let mod_urls = deps_res
			.modules
			.into_iter()
			.map(|m| vite_public_url(&base, &m, "dependency module file"))
			.collect::<Result<Vec<_>, _>>()?;
		let css_bundle_urls = deps_res
			.css_bundles
			.into_iter()
			.map(|m| vite_public_url(&base, &m, "CSS bundle file"))
			.collect::<Result<Vec<_>, _>>()?;

		Ok(ClientModule {
			url: own_file,
			dep_urls: mod_urls,
			css_bundle_urls,
		})
	}

	pub(crate) fn to_client_core_assets(
		&self,
		manifest: &ViteManifest,
	) -> Result<ClientCoreAssets, String> {
		let base = self.public_static_base_path();
		let mut wasm_file = String::new();
		for (key, chunk) in manifest.iter() {
			let src = if chunk.src.is_empty() {
				key.as_str()
			} else {
				chunk.src.as_str()
			};
			if slash_basename(src) != CLIENT_CORE_WASM_SOURCE_FILENAME {
				continue;
			}
			wasm_file =
				validate_vite_output_path(&chunk.file, "Vorma client WASM file")?.to_owned();
			break;
		}
		if wasm_file.is_empty() {
			return Err("Vite manifest does not contain Vorma client WASM asset".to_owned());
		}

		for chunk in manifest.values() {
			if !chunk.assets.iter().any(|asset| asset == &wasm_file) {
				continue;
			}
			return Ok(ClientCoreAssets {
				module_url: vite_public_url(&base, &chunk.file, "Vorma client module file")?,
				wasm_url: vite_public_url(&base, &wasm_file, "Vorma client WASM file")?,
			});
		}
		Err("Vite manifest does not contain Vorma client WASM wrapper module".to_owned())
	}
}

fn vite_public_url(base: &str, path: &str, label: &str) -> Result<String, String> {
	Ok(format!("{base}{}", validate_vite_output_path(path, label)?))
}

fn validate_vite_output_path<'a>(path: &'a str, label: &str) -> Result<&'a str, String> {
	if path.is_empty() {
		return Err(format!("Vite manifest {label} cannot be empty"));
	}
	if path.starts_with('/') {
		return Err(format!(
			"Vite manifest {label} must be a relative output path: {path:?}"
		));
	}
	if path.contains('\\') {
		return Err(format!(
			"Vite manifest {label} must use slash separators: {path:?}"
		));
	}
	if path
		.split('/')
		.any(|segment| segment.is_empty() || segment == "." || segment == "..")
	{
		return Err(format!(
			"Vite manifest {label} contains an invalid path segment: {path:?}"
		));
	}
	Ok(path)
}

fn collect_public_filepaths(cfg: &VormaCfg<'_>) -> Result<Vec<String>, String> {
	let pub_out = cfg.pub_out();
	let pub_out = Path::new(&pub_out);
	let mut public_filepaths = Vec::new();

	if !pub_out.exists() {
		return Ok(public_filepaths);
	}

	for entry in WalkDir::new(pub_out) {
		let entry = entry.map_err(|err| format!("error collecting public filepaths: {err}"))?;
		if entry.file_type().is_dir() {
			continue;
		}
		let rel = entry.path().strip_prefix(pub_out).map_err(|err| {
			format!(
				"error getting relative public filepath for {}: {err}",
				entry.path().display()
			)
		})?;
		if rel == Path::new(PROD_TMP_VITE_MANIFEST_FILENAME) {
			continue;
		}
		public_filepaths.push(format!(
			"{}{}",
			cfg.public_static_base_path(),
			rel.to_slash_lossy()
		));
	}

	public_filepaths.sort();
	Ok(public_filepaths)
}

fn validate_manifest_public_file_outputs(is_dev: bool, manifest: &Manifest) -> Result<(), String> {
	let public_filepaths = manifest
		.public_filepaths
		.iter()
		.map(String::as_str)
		.collect::<std::collections::BTreeSet<_>>();

	for url in manifest.public_filemap.values() {
		if !public_filepaths.contains(url.as_str()) {
			return Err(format!(
				"manifest public filemap URL is not present in public output: {url}"
			));
		}
	}

	if is_dev {
		return Ok(());
	}
	if manifest.client_core_assets.is_none() {
		return Err("prod manifest missing Vorma client core assets".to_owned());
	}

	for url in manifest_client_asset_urls(manifest) {
		if !public_filepaths.contains(url.as_str()) {
			return Err(format!(
				"manifest client asset URL is not present in public output: {url}"
			));
		}
	}
	Ok(())
}

fn manifest_client_asset_urls(manifest: &Manifest) -> Vec<&String> {
	let mut urls = Vec::new();
	push_client_module_urls(&mut urls, &manifest.client_entry);
	if let Some(assets) = &manifest.client_core_assets {
		urls.push(&assets.module_url);
		urls.push(&assets.wasm_url);
	}
	for module in manifest.client_routes.values() {
		push_client_module_urls(&mut urls, module);
	}
	urls
}

fn push_client_module_urls<'a>(urls: &mut Vec<&'a String>, module: &'a ClientModule) {
	urls.push(&module.url);
	urls.extend(&module.dep_urls);
	urls.extend(&module.css_bundle_urls);
}

fn root_document_shell_hash(root_document_hash_source: &str) -> String {
	blake3::hash(root_document_hash_source.trim().as_bytes())
		.to_hex()
		.to_string()
}

fn slash_basename(path: &str) -> &str {
	path.rsplit('/').next().unwrap_or(path)
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
	use crate::viteutil::ViteManifestChunk;
	use vorma::Document;

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
		let input = ManifestInput {
			is_dev: true,
			vite_server_port: 5173,
			dev_mux_port: 3000,
			dev_refresh_token: "refresh".to_owned(),
			root_document_hash_source: "\n<html></html>\n".to_owned(),
			ts_modules: BTreeMap::from([(
				"/item/:id".to_owned(),
				TsRoute {
					pattern: "/item/:id".to_owned(),
					import_path: "src/item.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			pub_fm: BTreeMap::from([("z.js".to_owned(), "/static/z.js".to_owned())]),
			critical_css: "body{}".to_owned(),
			search_schemas: BTreeMap::new(),
		};

		let manifest = write_manifest(&cfg, &input).unwrap();

		assert_eq!(
			manifest.public_filepaths,
			["/static/nested/a.css", "/static/z.js"]
		);
		assert_eq!(
			manifest.client_entry.url,
			"http://127.0.0.1:5173/src/entry.tsx"
		);
		assert_eq!(
			manifest.client_routes["/item/:id"].url,
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
	fn prepare_manifest_rejects_dev_manifest_without_vite_port() {
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
		let input = ManifestInput {
			is_dev: true,
			vite_server_port: 0,
			dev_mux_port: 3000,
			dev_refresh_token: "refresh".to_owned(),
			root_document_hash_source: "<html></html>".to_owned(),
			..ManifestInput::default()
		};

		assert_eq!(
			prepare_manifest(&cfg, &input).unwrap_err(),
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
		let input = ManifestInput {
			is_dev: true,
			vite_server_port: 3000,
			dev_mux_port: 3001,
			dev_refresh_token: "refresh".to_owned(),
			root_document_hash_source: "<html></html>".to_owned(),
			..ManifestInput::default()
		};

		assert_eq!(
			prepare_manifest(&cfg, &input).unwrap().vorma_version,
			VORMA_PACKAGE_VERSION
		);
	}

	#[test]
	fn prepare_manifest_prod_reads_vite_manifest_without_removing_tmp() {
		let dir = temp_dir("prepare-manifest-prod-no-cleanup");
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
		fs::write(
			cfg.prod_tmp_vite_manifest_out(),
			serde_json::to_vec(&vite_manifest).unwrap(),
		)
		.unwrap();
		let input = ManifestInput {
			is_dev: false,
			root_document_hash_source: "<html></html>".to_owned(),
			..ManifestInput::default()
		};

		let manifest = prepare_manifest(&cfg, &input).unwrap();

		assert_eq!(manifest.client_entry.url, "/static/assets/entry.js");
		assert!(Path::new(&cfg.prod_tmp_vite_manifest_out()).exists());
		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_prod_reads_vite_manifest_removes_tmp_and_writes_client_assets() {
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
		fs::write(
			cfg.prod_tmp_vite_manifest_out(),
			serde_json::to_vec(&vite_manifest).unwrap(),
		)
		.unwrap();

		let input = ManifestInput {
			is_dev: false,
			root_document_hash_source: "<html></html>".to_owned(),
			ts_modules: BTreeMap::from([(
				"/item/:id".to_owned(),
				TsRoute {
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
			..ManifestInput::default()
		};

		let manifest = write_manifest(&cfg, &input).unwrap();

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
			manifest.client_routes["/item/:id"].url,
			"/static/assets/item.js"
		);
		assert_eq!(
			manifest.client_core_assets.unwrap().module_url,
			"/static/assets/wasm-wrapper.js"
		);
		assert!(!Path::new(&cfg.prod_tmp_vite_manifest_out()).exists());
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
		let input = ManifestInput {
			is_dev: true,
			vite_server_port: 5173,
			root_document_hash_source: "<html></html>".to_owned(),
			pub_fm: BTreeMap::from([("missing.svg".to_owned(), "/static/missing.svg".to_owned())]),
			..ManifestInput::default()
		};

		let error = write_manifest(&cfg, &input).unwrap_err();

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
		fs::write(
			cfg.prod_tmp_vite_manifest_out(),
			serde_json::to_vec(&vite_manifest).unwrap(),
		)
		.unwrap();
		let input = ManifestInput {
			is_dev: false,
			root_document_hash_source: "<html></html>".to_owned(),
			..ManifestInput::default()
		};

		let error = write_manifest(&cfg, &input).unwrap_err();

		assert_eq!(
			error,
			"manifest client asset URL is not present in public output: /static/assets/missing-entry.js"
		);
		fs::remove_dir_all(dir).unwrap();
	}

	#[test]
	fn write_manifest_prod_keeps_tmp_vite_manifest_when_final_manifest_write_fails() {
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
		fs::write(
			cfg.prod_tmp_vite_manifest_out(),
			serde_json::to_vec(&vite_manifest).unwrap(),
		)
		.unwrap();
		fs::create_dir_all(cfg.manifest_json_out(false)).unwrap();
		let input = ManifestInput {
			is_dev: false,
			root_document_hash_source: "<html></html>".to_owned(),
			..ManifestInput::default()
		};

		let error = write_manifest(&cfg, &input).unwrap_err();

		assert!(error.contains("error writing vorma manifest json"));
		assert!(Path::new(&cfg.prod_tmp_vite_manifest_out()).exists());
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

	fn temp_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-build-{name}-{nonce}"))
	}
}
