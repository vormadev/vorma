use path_slash::PathExt;
use vorma::__private::manifest::{ClientCoreAssets, ClientModule};

use crate::config::{VormaCfg, relative_path};
use crate::viteutil::ViteManifest;

pub(crate) const CLIENT_CORE_WASM_SOURCE_FILENAME: &str = "vorma_client_wasm_bg.wasm";

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

fn slash_basename(path: &str) -> &str {
	path.rsplit('/').next().unwrap_or(path)
}
