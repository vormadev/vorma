//! Runtime filesystem loading for committed manifests and public asset capabilities.
//!
//! Serve-side filesystem boundary: locates the committed output directory,
//! loads the runtime manifest, and projects public asset capabilities for
//! the host. Nothing here mutates build outputs; the build crate is the only
//! writer.

use std::path::{Path, PathBuf};

use crate::asset_body_provider::{PublicAssetDirectory, PublicAssetDirectoryError};
use crate::config::Config;
use crate::runtime_manifest::RuntimeManifest;

/// Vorma-owned output directory below the configured dist directory.
pub const VORMA_OUT_DIR: &str = ".vorma";
/// Vorma-owned static output directory below [`VORMA_OUT_DIR`].
pub const STATIC_OUT_DIR: &str = "static";
/// Public asset body directory below the static output directory.
pub const PUBLIC_OUT_DIR: &str = "public";
/// Development runtime manifest filename.
pub const MANIFEST_STATIC_OUT_DEV: &str = "vorma.manifest.dev.json";
/// Production runtime manifest filename.
pub const MANIFEST_STATIC_OUT_PROD: &str = "vorma.manifest.prod.json";

/// Runtime manifest mode.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ManifestMode {
	/// Development manifest.
	Dev,
	/// Production manifest.
	Prod,
}

impl ManifestMode {
	/// Runtime manifest filename for this mode.
	pub const fn manifest_filename(self) -> &'static str {
		match self {
			Self::Dev => MANIFEST_STATIC_OUT_DEV,
			Self::Prod => MANIFEST_STATIC_OUT_PROD,
		}
	}
}

/// Committed runtime assets loaded from the configured filesystem.
#[derive(Clone, Debug)]
pub struct RuntimeAssetBundle {
	static_out_dir: PathBuf,
	manifest_path: PathBuf,
	manifest: RuntimeManifest,
	public_asset_directory: PublicAssetDirectory,
}

/// Environment key Vercel sets on every deployed runtime.
const VERCEL_SYSTEM_ENV_KEY: &str = "VERCEL";
/// Environment key carrying the Vercel deployment environment name.
const VERCEL_ENVIRONMENT_ENV_KEY: &str = "VERCEL_ENV";

impl RuntimeAssetBundle {
	/// Load a committed runtime asset bundle from public config and manifest mode.
	pub fn load(config: &Config, mode: ManifestMode) -> Result<Self, RuntimeAssetLoadError> {
		let static_out_dir = runtime_static_out_dir(config)?;
		match Self::load_from_static_out_dir(static_out_dir, mode) {
			Err(RuntimeAssetLoadError::ReadManifest { path, message }) if vercel_runtime() => {
				let current_root_dir = std::env::current_dir().map_err(|source| {
					RuntimeAssetLoadError::VercelManifestFallback {
						configured_path: path.clone(),
						configured_message: message.clone(),
						fallback_path: "<current dir>".to_owned(),
						fallback_message: source.to_string(),
					}
				})?;
				Self::load_from_vercel_fallback_root(
					&config.dist_dir,
					mode,
					path,
					message,
					current_root_dir,
				)
			}
			result => result,
		}
	}

	/*
	Vercel's serverless filesystem roots the deployed bundle at the runtime
	working directory rather than the configured build-time root, so a missing
	manifest at the configured path retries against the current dir before
	failing with both attempted locations.
	*/
	fn load_from_vercel_fallback_root(
		dist_dir: &str,
		mode: ManifestMode,
		configured_path: String,
		configured_message: String,
		current_root_dir: PathBuf,
	) -> Result<Self, RuntimeAssetLoadError> {
		let fallback_static_out_dir =
			static_out_dir(declared_dist_dir(&current_root_dir, dist_dir));
		Self::load_from_static_out_dir(fallback_static_out_dir, mode).map_err(|fallback| {
			match fallback {
				RuntimeAssetLoadError::ReadManifest { path, message } => {
					RuntimeAssetLoadError::VercelManifestFallback {
						configured_path,
						configured_message,
						fallback_path: path,
						fallback_message: message,
					}
				}
				other => other,
			}
		})
	}

	fn load_from_static_out_dir(
		static_out_dir: PathBuf,
		mode: ManifestMode,
	) -> Result<Self, RuntimeAssetLoadError> {
		let manifest_path = static_out_dir.join(mode.manifest_filename());
		let manifest_bytes = std::fs::read(&manifest_path).map_err(|source| {
			RuntimeAssetLoadError::ReadManifest {
				path: manifest_path.display().to_string(),
				message: source.to_string(),
			}
		})?;
		let manifest = RuntimeManifest::from_json_slice(&manifest_bytes).map_err(|source| {
			RuntimeAssetLoadError::DecodeManifest {
				path: manifest_path.display().to_string(),
				message: source.to_string(),
			}
		})?;
		let public_asset_directory = PublicAssetDirectory::new(static_out_dir.join(PUBLIC_OUT_DIR))
			.map_err(|source| RuntimeAssetLoadError::PublicAssetDirectory { source })?;
		Ok(Self {
			static_out_dir,
			manifest_path,
			manifest,
			public_asset_directory,
		})
	}

	/// Vorma static output root containing the manifest and public output directory.
	pub fn static_out_dir(&self) -> &Path {
		&self.static_out_dir
	}

	/// Runtime manifest path loaded for this bundle.
	pub fn manifest_path(&self) -> &Path {
		&self.manifest_path
	}

	/// Loaded runtime manifest.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}

	/// Filesystem-backed public asset body provider.
	pub fn public_asset_directory(&self) -> &PublicAssetDirectory {
		&self.public_asset_directory
	}

	/// Consume the bundle into the manifest and public asset body provider.
	pub fn into_manifest_and_public_asset_directory(
		self,
	) -> (RuntimeManifest, PublicAssetDirectory) {
		(self.manifest, self.public_asset_directory)
	}
}

/// Compute Vorma's static output directory below a dist directory.
pub fn static_out_dir(dist_dir: impl AsRef<Path>) -> PathBuf {
	dist_dir.as_ref().join(VORMA_OUT_DIR).join(STATIC_OUT_DIR)
}

/// Runtime asset loading error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeAssetLoadError {
	/// Runtime root directory was empty.
	EmptyRootDir,
	/// Runtime root directory was not absolute.
	RelativeRootDir {
		/// Rejected root directory.
		path: String,
	},
	/// Runtime root directory could not be canonicalized.
	RootDir {
		/// Rejected root directory.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Dist directory could not be canonicalized.
	DistDir {
		/// Rejected dist directory.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Dist directory escaped the configured root directory.
	DistDirEscapesRoot {
		/// Canonical root directory.
		root_dir: String,
		/// Canonical dist directory.
		dist_dir: String,
	},
	/// Runtime manifest file could not be read.
	ReadManifest {
		/// Manifest path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Runtime manifest missing at the configured path and the Vercel current-dir fallback.
	VercelManifestFallback {
		/// Configured manifest path.
		configured_path: String,
		/// Configured-path read error message.
		configured_message: String,
		/// Vercel current-dir fallback manifest path.
		fallback_path: String,
		/// Fallback-path read error message.
		fallback_message: String,
	},
	/// Runtime manifest JSON was invalid.
	DecodeManifest {
		/// Manifest path.
		path: String,
		/// JSON error message.
		message: String,
	},
	/// Public asset directory could not be committed.
	PublicAssetDirectory {
		/// Source public asset directory error.
		source: PublicAssetDirectoryError,
	},
}

impl std::fmt::Display for RuntimeAssetLoadError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::RelativeRootDir { path } => write!(f, "root_dir must be absolute: {path}"),
			Self::RootDir { path, message } => {
				write!(f, "canonicalize root_dir {path}: {message}")
			}
			Self::DistDir { path, message } => {
				write!(f, "canonicalize dist_dir {path}: {message}")
			}
			Self::DistDirEscapesRoot { root_dir, dist_dir } => {
				write!(f, "dist_dir {dist_dir} must be inside root_dir {root_dir}")
			}
			Self::ReadManifest { path, message } => {
				write!(f, "read runtime manifest {path}: {message}")
			}
			Self::VercelManifestFallback {
				configured_path,
				configured_message,
				fallback_path,
				fallback_message,
			} => {
				write!(
					f,
					"read runtime manifest {configured_path}: {configured_message}; \
					 Vercel current-dir fallback {fallback_path}: {fallback_message}"
				)
			}
			Self::DecodeManifest { path, message } => {
				write!(f, "decode runtime manifest {path}: {message}")
			}
			Self::PublicAssetDirectory { source } => {
				write!(f, "commit public asset directory: {source}")
			}
		}
	}
}

impl std::error::Error for RuntimeAssetLoadError {
	fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
		match self {
			Self::PublicAssetDirectory { source } => Some(source),
			Self::EmptyRootDir
			| Self::RelativeRootDir { .. }
			| Self::RootDir { .. }
			| Self::DistDir { .. }
			| Self::DistDirEscapesRoot { .. }
			| Self::ReadManifest { .. }
			| Self::VercelManifestFallback { .. }
			| Self::DecodeManifest { .. } => None,
		}
	}
}

fn runtime_static_out_dir(config: &Config) -> Result<PathBuf, RuntimeAssetLoadError> {
	if config.root_dir.as_os_str().is_empty() {
		return Err(RuntimeAssetLoadError::EmptyRootDir);
	}
	if !config.root_dir.is_absolute() {
		return Err(RuntimeAssetLoadError::RelativeRootDir {
			path: config.root_dir.display().to_string(),
		});
	}
	let root_dir = std::fs::canonicalize(&config.root_dir).map_err(|source| {
		RuntimeAssetLoadError::RootDir {
			path: config.root_dir.display().to_string(),
			message: source.to_string(),
		}
	})?;
	let declared_dist_dir = declared_dist_dir(&root_dir, &config.dist_dir);
	let dist_dir = std::fs::canonicalize(&declared_dist_dir).map_err(|source| {
		RuntimeAssetLoadError::DistDir {
			path: declared_dist_dir.display().to_string(),
			message: source.to_string(),
		}
	})?;
	if !dist_dir.starts_with(&root_dir) {
		return Err(RuntimeAssetLoadError::DistDirEscapesRoot {
			root_dir: root_dir.display().to_string(),
			dist_dir: dist_dir.display().to_string(),
		});
	}
	Ok(static_out_dir(dist_dir))
}

fn vercel_runtime() -> bool {
	[VERCEL_SYSTEM_ENV_KEY, VERCEL_ENVIRONMENT_ENV_KEY]
		.iter()
		.any(|key| std::env::var_os(key).is_some_and(|value| !value.is_empty()))
}

fn declared_dist_dir(root_dir: &Path, dist_dir: &str) -> PathBuf {
	let dist_dir = PathBuf::from(dist_dir);
	if dist_dir.is_absolute() {
		return dist_dir;
	}
	root_dir.join(dist_dir)
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;

	use bytes::Bytes;
	use http::HeaderValue;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyProvider;
	use crate::config::{DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig, UiVariant};
	use crate::test_support::unique_temp_root;
	use crate::tsgen::TsDrafter;

	const TEST_CLIENT_BUILD_ID: &str = "build-id";

	#[test]
	fn vercel_fallback_reports_configured_and_current_manifest_paths_when_missing() {
		let fallback_root = unique_temp_root("vorma-vercel-fallback");
		fs::create_dir_all(&fallback_root).unwrap();

		let error = RuntimeAssetBundle::load_from_vercel_fallback_root(
			"dist",
			ManifestMode::Prod,
			"/configured/dist/.vorma/static/vorma.manifest.prod.json".to_owned(),
			"missing".to_owned(),
			fallback_root.clone(),
		)
		.unwrap_err();

		let RuntimeAssetLoadError::VercelManifestFallback {
			configured_path,
			fallback_path,
			..
		} = &error
		else {
			panic!("expected Vercel fallback error, got {error}");
		};
		assert_eq!(
			configured_path,
			"/configured/dist/.vorma/static/vorma.manifest.prod.json"
		);
		assert!(fallback_path.contains("vorma-vercel-fallback"));
		assert!(fallback_path.ends_with(MANIFEST_STATIC_OUT_PROD));
		assert!(error.to_string().contains("Vercel current-dir fallback"));
		fs::remove_dir_all(fallback_root).unwrap();
	}
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_SOURCE_PATH: &str = "app.css";
	const TEST_PUBLIC_OUTPUT_PATH: &str = "/static/app.hash.css";

	fn test_config(root_dir: PathBuf) -> Config {
		Config {
			root_dir,
			server_target: ServerTarget::default(),
			dist_dir: "dist".to_owned(),
			public_static_base: TEST_PUBLIC_STATIC_BASE.to_owned(),
			frontend_config: FrontendConfig {
				ui_variant: UiVariant::React,
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "src/critical.css".to_owned(),
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				extra_types: Vec::new(),
				extra_ts: TsDrafter::new(),
			},
			dev_watch_config: DevWatchConfig::default(),
		}
	}

	fn temp_root() -> PathBuf {
		unique_temp_root("vorma-runtime-assets")
	}

	fn test_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			TEST_CLIENT_BUILD_ID,
			TEST_PUBLIC_STATIC_BASE,
			"",
			BTreeMap::new(),
			vec![TEST_PUBLIC_OUTPUT_PATH.to_owned()],
			BTreeMap::from([(
				TEST_PUBLIC_SOURCE_PATH.to_owned(),
				TEST_PUBLIC_OUTPUT_PATH.to_owned(),
			)]),
			Vec::new(),
		)
	}

	#[tokio::test]
	async fn runtime_asset_bundle_loads_manifest_and_public_asset_directory_from_config() {
		let root_dir = temp_root();
		let static_out = static_out_dir(root_dir.join("dist"));
		let public_dir = static_out.join(PUBLIC_OUT_DIR);
		fs::create_dir_all(&public_dir).unwrap();
		fs::write(
			static_out.join(MANIFEST_STATIC_OUT_PROD),
			test_manifest().to_json_vec().unwrap(),
		)
		.unwrap();
		fs::write(public_dir.join("app.hash.css"), b"body{}").unwrap();

		let bundle =
			RuntimeAssetBundle::load(&test_config(root_dir.clone()), ManifestMode::Prod).unwrap();
		let (_, asset_directory) = bundle.into_manifest_and_public_asset_directory();
		let asset = crate::asset_capabilities::AssetCapabilities::new(
			TEST_PUBLIC_STATIC_BASE,
			vec![TEST_PUBLIC_OUTPUT_PATH.to_owned()],
			BTreeMap::new(),
		)
		.unwrap()
		.asset_for_request_path(TEST_PUBLIC_OUTPUT_PATH)
		.unwrap();
		let body = asset_directory.body_for_asset(asset).await.unwrap();

		assert_eq!(body.body(), &Bytes::from_static(b"body{}"));
		assert_eq!(
			body.content_type(),
			Some(&HeaderValue::from_static("text/css"))
		);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn runtime_asset_bundle_rejects_relative_root_and_escaped_dist_dir() {
		let mut relative = test_config(PathBuf::from("relative"));
		assert!(matches!(
			RuntimeAssetBundle::load(&relative, ManifestMode::Prod),
			Err(RuntimeAssetLoadError::RelativeRootDir { .. })
		));

		let root_dir = temp_root();
		let outside = temp_root();
		fs::create_dir_all(root_dir.join("dist")).unwrap();
		fs::create_dir_all(&outside).unwrap();
		relative = test_config(root_dir.clone());
		relative.dist_dir = outside.display().to_string();
		assert!(matches!(
			RuntimeAssetBundle::load(&relative, ManifestMode::Prod),
			Err(RuntimeAssetLoadError::DistDirEscapesRoot { .. })
		));

		fs::remove_dir_all(root_dir).unwrap();
		fs::remove_dir_all(outside).unwrap();
	}
}
