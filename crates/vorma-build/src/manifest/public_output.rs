use std::path::Path;

use path_slash::PathExt;
use vorma::__private::manifest::{ClientModule, Manifest};
use walkdir::WalkDir;

use crate::config::VormaCfg;

pub(super) fn collect_public_filepaths(cfg: &VormaCfg<'_>) -> Result<Vec<String>, String> {
	let pub_out = cfg.pub_out();
	let pub_out = Path::new(&pub_out);
	let prod_tmp_vite_manifest_rel = Path::new(&cfg.prod_tmp_vite_manifest_out())
		.strip_prefix(pub_out)
		.ok()
		.map(Path::to_path_buf);
	let mut public_filepaths = Vec::new();

	if !pub_out.exists() {
		return Ok(public_filepaths);
	}

	for entry in WalkDir::new(pub_out) {
		let entry = entry.map_err(|err| format!("error collecting public filepaths: {err}"))?;
		if entry.file_type().is_dir() {
			continue;
		}
		if entry.file_type().is_symlink() {
			return Err(format!(
				"public output file cannot be a symlink: {}",
				entry.path().display()
			));
		}
		let rel = entry.path().strip_prefix(pub_out).map_err(|err| {
			format!(
				"error getting relative public filepath for {}: {err}",
				entry.path().display()
			)
		})?;
		if prod_tmp_vite_manifest_rel.as_deref() == Some(rel) {
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

pub(super) fn validate_dev_manifest_public_file_outputs(manifest: &Manifest) -> Result<(), String> {
	validate_manifest_public_filemap_urls(manifest)
}

pub(super) fn validate_prod_manifest_public_file_outputs(
	manifest: &Manifest,
) -> Result<(), String> {
	validate_manifest_public_filemap_urls(manifest)?;
	if manifest.client_core_assets.is_none() {
		return Err("prod manifest missing Vorma client core assets".to_owned());
	}

	let public_filepaths = manifest
		.public_filepaths
		.iter()
		.map(String::as_str)
		.collect::<std::collections::BTreeSet<_>>();

	for url in manifest_client_asset_urls(manifest) {
		if !public_filepaths.contains(url.as_str()) {
			return Err(format!(
				"manifest client asset URL is not present in public output: {url}"
			));
		}
	}
	Ok(())
}

fn validate_manifest_public_filemap_urls(manifest: &Manifest) -> Result<(), String> {
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

	Ok(())
}

fn manifest_client_asset_urls(manifest: &Manifest) -> Vec<&String> {
	let mut urls = Vec::new();
	push_client_module_urls(&mut urls, &manifest.client_entry);
	if let Some(assets) = &manifest.client_core_assets {
		urls.push(&assets.module_url);
		urls.push(&assets.wasm_url);
	}
	for module in manifest.client_views.values() {
		push_client_module_urls(&mut urls, module);
	}
	urls
}

fn push_client_module_urls<'a>(urls: &mut Vec<&'a String>, module: &'a ClientModule) {
	urls.push(&module.url);
	urls.extend(&module.dep_urls);
	urls.extend(&module.css_bundle_urls);
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::path::PathBuf;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::config::to_cfg;

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!(
			"vorma-public-output-{name}-{}-{nonce}",
			std::process::id()
		));
		fs::create_dir_all(&root).unwrap();
		root
	}

	fn config(root_dir: PathBuf) -> Config {
		Config {
			root_dir,
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/client/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..Config::default()
		}
	}

	#[cfg(unix)]
	#[test]
	fn collect_public_filepaths_rejects_symlink_outputs() {
		let root = temp_root("reject-symlink");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		let pub_out = PathBuf::from(cfg.pub_out());
		let external = root.with_extension("external-public-file");
		fs::create_dir_all(&pub_out).unwrap();
		fs::write(&external, "secret").unwrap();
		std::os::unix::fs::symlink(&external, pub_out.join("linked.txt")).unwrap();

		let err = collect_public_filepaths(&cfg).unwrap_err();

		assert!(err.contains("public output file cannot be a symlink"));
		assert!(err.contains("linked.txt"));
		fs::remove_dir_all(root).unwrap();
		fs::remove_file(external).unwrap();
	}
}
