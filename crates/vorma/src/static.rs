use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, OnceLock};

use crate::manifest::{MANIFEST_STATIC_OUT_DEV, MANIFEST_STATIC_OUT_PROD, Manifest};

pub const VORMA_OUT_DIR: &str = ".vorma";
pub const STATIC_OUT_DIR: &str = "static";
pub const PUBLIC_OUT_DIR: &str = "public";
pub const PUBLIC_ASSET_CACHE_CONTROL: &str = "public, max-age=31536000, immutable";

pub fn static_out_dir(dist_dir: impl AsRef<Path>) -> PathBuf {
	dist_dir.as_ref().join(VORMA_OUT_DIR).join(STATIC_OUT_DIR)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ManifestMode {
	Dev,
	Prod,
}

impl ManifestMode {
	fn manifest_path(self) -> &'static str {
		match self {
			Self::Dev => MANIFEST_STATIC_OUT_DEV,
			Self::Prod => MANIFEST_STATIC_OUT_PROD,
		}
	}

	fn name(self) -> &'static str {
		match self {
			Self::Dev => "dev",
			Self::Prod => "prod",
		}
	}
}

#[derive(Clone, Debug)]
pub struct RuntimeAssets {
	inner: RuntimeAssetsInner,
}

type PublicAssetCache = Arc<Mutex<BTreeMap<String, Vec<u8>>>>;

#[derive(Clone, Debug)]
enum RuntimeAssetsInner {
	Live {
		root: PathBuf,
		mode: ManifestMode,
	},
	Cached {
		root: PathBuf,
		mode: ManifestMode,
		cache: Arc<OnceLock<Arc<RuntimeAssetSnapshot>>>,
		public_cache: PublicAssetCache,
	},
}

impl RuntimeAssets {
	pub fn live_fs(root: impl Into<PathBuf>, mode: ManifestMode) -> Self {
		Self {
			inner: RuntimeAssetsInner::Live {
				root: root.into(),
				mode,
			},
		}
	}

	pub fn cached_fs(root: impl Into<PathBuf>, mode: ManifestMode) -> Self {
		Self {
			inner: RuntimeAssetsInner::Cached {
				root: root.into(),
				mode,
				cache: Arc::new(OnceLock::new()),
				public_cache: Arc::new(Mutex::new(BTreeMap::new())),
			},
		}
	}

	pub fn snapshot(&self) -> Result<Arc<RuntimeAssetSnapshot>, String> {
		match &self.inner {
			RuntimeAssetsInner::Live { root, mode } => {
				load_snapshot(&FsAssetReader { root }, *mode).map(Arc::new)
			}
			RuntimeAssetsInner::Cached {
				root, mode, cache, ..
			} => cache.get().cloned().map(Ok).unwrap_or_else(|| {
				let loaded = Arc::new(load_snapshot(&FsAssetReader { root }, *mode)?);
				let _ = cache.set(loaded.clone());
				Ok(loaded)
			}),
		}
	}

	pub fn read_public_asset(
		&self,
		request_path: &str,
	) -> Result<Option<RuntimePublicAssetRead>, String> {
		let snapshot = self.snapshot()?;
		let Some(asset) = public_asset_for_snapshot(&snapshot, request_path) else {
			return Ok(None);
		};
		let source_path = format!("{PUBLIC_OUT_DIR}/{}", asset.fs_path());
		let bytes = match &self.inner {
			RuntimeAssetsInner::Live { root, .. } => {
				FsAssetReader { root }.read_required_public(&source_path)
			}
			RuntimeAssetsInner::Cached {
				root, public_cache, ..
			} => read_cached_public_asset(public_cache, &FsAssetReader { root }, &source_path),
		}?;
		Ok(Some(RuntimePublicAssetRead {
			bytes,
			cache_control: asset.cache_control(),
			content_type: public_asset_content_type(asset.fs_path()),
		}))
	}

	pub fn stat_public_asset(
		&self,
		request_path: &str,
	) -> Result<Option<RuntimePublicAssetMetadata>, String> {
		let snapshot = self.snapshot()?;
		let Some(asset) = public_asset_for_snapshot(&snapshot, request_path) else {
			return Ok(None);
		};
		let source_path = format!("{PUBLIC_OUT_DIR}/{}", asset.fs_path());
		let content_length = match &self.inner {
			RuntimeAssetsInner::Live { root, .. } => {
				FsAssetReader { root }.metadata_required_public(&source_path)
			}
			RuntimeAssetsInner::Cached {
				root, public_cache, ..
			} => stat_cached_public_asset(public_cache, &FsAssetReader { root }, &source_path),
		}?;
		Ok(Some(RuntimePublicAssetMetadata {
			content_length,
			cache_control: asset.cache_control(),
			content_type: public_asset_content_type(asset.fs_path()),
		}))
	}
}

fn read_cached_public_asset(
	cache: &PublicAssetCache,
	reader: &FsAssetReader<'_>,
	source_path: &str,
) -> Result<Vec<u8>, String> {
	if let Some(bytes) = cache
		.lock()
		.expect("runtime public asset cache mutex poisoned")
		.get(source_path)
		.cloned()
	{
		return Ok(bytes);
	}
	let bytes = reader.read_required_public(source_path)?;
	cache
		.lock()
		.expect("runtime public asset cache mutex poisoned")
		.insert(source_path.to_owned(), bytes.clone());
	Ok(bytes)
}

fn stat_cached_public_asset(
	cache: &PublicAssetCache,
	reader: &FsAssetReader<'_>,
	source_path: &str,
) -> Result<u64, String> {
	if let Some(bytes) = cache
		.lock()
		.expect("runtime public asset cache mutex poisoned")
		.get(source_path)
		.cloned()
	{
		return Ok(bytes.len() as u64);
	}
	reader.metadata_required_public(source_path)
}

fn public_asset_content_type(path: &str) -> String {
	let content_type = mime_guess::from_path(path).first_or_octet_stream();
	if content_type.type_() == mime::TEXT && content_type.get_param(mime::CHARSET).is_none() {
		return format!("{content_type}; charset=utf-8");
	}
	content_type.to_string()
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RuntimePublicAssetRead {
	pub bytes: Vec<u8>,
	pub cache_control: &'static str,
	pub content_type: String,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RuntimePublicAssetMetadata {
	pub content_length: u64,
	pub cache_control: &'static str,
	pub content_type: String,
}

#[derive(Clone, Debug, PartialEq)]
pub struct RuntimeAssetSnapshot {
	manifest: Arc<Manifest>,
	client_build_id: String,
	final_public_filepaths: BTreeSet<String>,
}

impl RuntimeAssetSnapshot {
	fn new(mut manifest: Manifest) -> Result<Self, String> {
		manifest.normalize_runtime_paths()?;
		let client_build_id = manifest
			.to_client_build_id()
			.map_err(|err| err.to_string())?;
		let final_public_filepaths = manifest.final_public_filepaths();
		Ok(Self {
			manifest: Arc::new(manifest),
			client_build_id,
			final_public_filepaths,
		})
	}

	pub fn manifest(&self) -> &Manifest {
		&self.manifest
	}

	pub(crate) fn manifest_handle(&self) -> Arc<Manifest> {
		self.manifest.clone()
	}

	pub fn client_build_id(&self) -> &str {
		&self.client_build_id
	}

	pub fn final_public_filepaths(&self) -> &BTreeSet<String> {
		&self.final_public_filepaths
	}
}

trait AssetReader {
	fn read_required(&self, path: &str) -> Result<Vec<u8>, String>;
}

struct FsAssetReader<'a> {
	root: &'a Path,
}

impl FsAssetReader<'_> {
	fn read_required_public(&self, path: &str) -> Result<Vec<u8>, String> {
		let canonical_full = self.canonical_required_public_path(path)?;
		std::fs::read(&canonical_full).map_err(|err| {
			format!(
				"read runtime asset {}: {err}",
				self.root.join(path).display()
			)
		})
	}

	fn metadata_required_public(&self, path: &str) -> Result<u64, String> {
		let canonical_full = self.canonical_required_public_path(path)?;
		let metadata = canonical_full.metadata().map_err(|err| {
			format!(
				"read runtime asset metadata {}: {err}",
				self.root.join(path).display()
			)
		})?;
		if !metadata.is_file() {
			return Err(format!(
				"runtime asset {} is not a file",
				self.root.join(path).display()
			));
		}
		Ok(metadata.len())
	}

	fn canonical_required_public_path(&self, path: &str) -> Result<PathBuf, String> {
		self.canonical_optional_public_path(path)?.ok_or_else(|| {
			format!(
				"manifest-listed runtime public asset is missing: {}",
				self.root.join(path).display()
			)
		})
	}

	fn canonical_optional_public_path(&self, path: &str) -> Result<Option<PathBuf>, String> {
		let full = self.root.join(path);
		let canonical_full = match full.canonicalize() {
			Ok(canonical_full) => canonical_full,
			Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(None),
			Err(err) => return Err(format!("read runtime asset {}: {err}", full.display())),
		};
		let public_root = self
			.root
			.join(PUBLIC_OUT_DIR)
			.canonicalize()
			.map_err(|err| {
				format!(
					"read runtime public asset root {}: {err}",
					self.root.join(PUBLIC_OUT_DIR).display()
				)
			})?;
		if !canonical_full.starts_with(&public_root) {
			return Err(format!(
				"runtime public asset {} resolves outside public root {}",
				full.display(),
				public_root.display()
			));
		}
		Ok(Some(canonical_full))
	}
}

impl AssetReader for FsAssetReader<'_> {
	fn read_required(&self, path: &str) -> Result<Vec<u8>, String> {
		let full = self.root.join(path);
		std::fs::read(&full).map_err(|err| format!("read runtime asset {}: {err}", full.display()))
	}
}

fn load_snapshot(
	reader: &impl AssetReader,
	mode: ManifestMode,
) -> Result<RuntimeAssetSnapshot, String> {
	let bytes = reader.read_required(mode.manifest_path())?;
	let manifest = serde_json::from_slice(&bytes)
		.map_err(|err| format!("error parsing {} manifest: {err}", mode.name()))?;
	RuntimeAssetSnapshot::new(manifest)
}

pub fn path_is_under_public_static_base(manifest: &Manifest, path: &str) -> bool {
	let base = manifest.public_static_base_path.as_str();
	base != "/" && path.starts_with(base)
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublicAsset {
	fs_path: String,
	cache_control: &'static str,
}

impl PublicAsset {
	pub fn fs_path(&self) -> &str {
		&self.fs_path
	}

	pub fn cache_control(&self) -> &'static str {
		self.cache_control
	}
}

#[cfg(test)]
fn public_asset(manifest: &Manifest, path: &str) -> Option<PublicAsset> {
	let mut manifest = manifest.clone();
	manifest.normalize_runtime_paths().ok()?;
	let final_public_filepaths = manifest.final_public_filepaths();
	public_asset_with_filepaths(&manifest, &final_public_filepaths, path)
}

fn public_asset_for_snapshot(snapshot: &RuntimeAssetSnapshot, path: &str) -> Option<PublicAsset> {
	public_asset_with_filepaths(snapshot.manifest(), snapshot.final_public_filepaths(), path)
}

fn public_asset_with_filepaths(
	manifest: &Manifest,
	final_public_filepaths: &BTreeSet<String>,
	path: &str,
) -> Option<PublicAsset> {
	if !final_public_filepaths.contains(path) {
		return None;
	}

	let base = manifest.public_static_base_path.as_str();
	let fs_path = if base == "/" {
		path.trim_start_matches('/')
	} else {
		path.strip_prefix(base)?
	};
	let fs_path = safe_public_asset_fs_path(fs_path)?;
	Some(PublicAsset {
		fs_path,
		cache_control: PUBLIC_ASSET_CACHE_CONTROL,
	})
}

fn safe_public_asset_fs_path(path: &str) -> Option<String> {
	if path.is_empty() {
		return None;
	}
	if path.split('/').any(|segment| {
		segment.is_empty() || segment == "." || segment == ".." || segment.contains('\\')
	}) {
		return None;
	}
	Some(path.to_owned())
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::time::{SystemTime, UNIX_EPOCH};

	use crate::manifest::ClientModule;

	use super::*;

	fn temp_root(name: &str) -> PathBuf {
		let nanos = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!(
			"vorma-runtime-assets-{name}-{}-{nanos}",
			std::process::id()
		));
		fs::create_dir_all(root.join(PUBLIC_OUT_DIR)).unwrap();
		root
	}

	fn manifest(version: &str) -> Manifest {
		Manifest {
			vorma_version: version.to_owned(),
			public_static_base_path: "/static/".to_owned(),
			api_mount_root: "/api/".to_owned(),
			ui_variant: "react".to_owned(),
			root_document_shell_hash: "shell".to_owned(),
			public_filepaths: vec!["/static/app.css".to_owned()],
			public_filemap: [("app.css".to_owned(), format!("/static/{version}.app.css"))]
				.into_iter()
				.collect(),
			client_entry: ClientModule {
				url: "/static/entry.js".to_owned(),
				dep_urls: Vec::new(),
				css_bundle_urls: Vec::new(),
			},
			..Manifest::default()
		}
	}

	fn write_manifest(root: &Path, mode: ManifestMode, manifest: &Manifest) {
		fs::write(
			root.join(mode.manifest_path()),
			serde_json::to_vec(manifest).unwrap(),
		)
		.unwrap();
	}

	fn read_asset_bytes(assets: &RuntimeAssets) -> Vec<u8> {
		assets
			.read_public_asset("/static/app.css")
			.unwrap()
			.unwrap()
			.bytes
	}

	#[test]
	fn static_out_dir_matches_vorma_layout() {
		assert_eq!(
			static_out_dir("dist"),
			PathBuf::from("dist").join(".vorma").join("static")
		);
	}

	#[test]
	fn live_fs_reloads_manifest_each_snapshot() {
		let root = temp_root("live-manifest");
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		assert_eq!(assets.snapshot().unwrap().manifest().vorma_version, "one");

		write_manifest(&root, ManifestMode::Dev, &manifest("two"));

		assert_eq!(assets.snapshot().unwrap().manifest().vorma_version, "two");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cached_fs_uses_one_manifest_snapshot() {
		let root = temp_root("cached-manifest");
		write_manifest(&root, ManifestMode::Prod, &manifest("one"));
		let assets = RuntimeAssets::cached_fs(&root, ManifestMode::Prod);

		assert_eq!(assets.snapshot().unwrap().manifest().vorma_version, "one");

		write_manifest(&root, ManifestMode::Prod, &manifest("two"));

		assert_eq!(assets.snapshot().unwrap().manifest().vorma_version, "one");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn live_fs_reloads_public_asset_bytes() {
		let root = temp_root("live-public");
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		fs::write(root.join("public/app.css"), b"one").unwrap();
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		assert_eq!(read_asset_bytes(&assets), b"one");

		fs::write(root.join("public/app.css"), b"two").unwrap();

		assert_eq!(read_asset_bytes(&assets), b"two");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn manifest_listed_missing_public_asset_is_runtime_error() {
		let root = temp_root("missing-manifest-asset");
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		let read_error = assets.read_public_asset("/static/app.css").unwrap_err();
		let stat_error = assets.stat_public_asset("/static/app.css").unwrap_err();

		assert!(read_error.contains("manifest-listed runtime public asset is missing"));
		assert!(stat_error.contains("manifest-listed runtime public asset is missing"));
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cached_fs_reuses_public_asset_bytes() {
		let root = temp_root("cached-public");
		write_manifest(&root, ManifestMode::Prod, &manifest("one"));
		fs::write(root.join("public/app.css"), b"one").unwrap();
		let assets = RuntimeAssets::cached_fs(&root, ManifestMode::Prod);

		assert_eq!(read_asset_bytes(&assets), b"one");

		fs::write(root.join("public/app.css"), b"two").unwrap();

		assert_eq!(read_asset_bytes(&assets), b"one");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cached_fs_public_asset_metadata_does_not_fill_body_cache() {
		let root = temp_root("cached-public-head");
		write_manifest(&root, ManifestMode::Prod, &manifest("one"));
		fs::write(root.join("public/app.css"), b"one").unwrap();
		let assets = RuntimeAssets::cached_fs(&root, ManifestMode::Prod);

		let metadata = assets
			.stat_public_asset("/static/app.css")
			.unwrap()
			.unwrap();

		assert_eq!(
			metadata,
			RuntimePublicAssetMetadata {
				content_length: 3,
				cache_control: PUBLIC_ASSET_CACHE_CONTROL,
				content_type: "text/css; charset=utf-8".to_owned(),
			}
		);
		let RuntimeAssetsInner::Cached { public_cache, .. } = &assets.inner else {
			panic!("expected cached runtime assets");
		};
		assert!(public_cache.lock().unwrap().is_empty());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn public_asset_content_type_matches_file_server_text_shape() {
		assert_eq!(
			public_asset_content_type("app.css"),
			"text/css; charset=utf-8"
		);
		assert_eq!(public_asset_content_type("favicon.ico"), "image/x-icon");
	}

	#[test]
	fn public_asset_checks_manifest_paths_when_base_is_root() {
		let mut manifest = manifest("root");
		manifest.public_static_base_path = "/".to_owned();
		manifest.public_filepaths = vec!["/favicon.ico".to_owned()];

		assert_eq!(
			public_asset(&manifest, "/favicon.ico").unwrap().fs_path(),
			"favicon.ico"
		);
		assert!(public_asset(&manifest, "/missing.ico").is_none());
	}

	#[test]
	fn public_asset_checks_manifest_paths_when_base_is_not_root() {
		let manifest = manifest("base");

		assert_eq!(
			public_asset(&manifest, "/static/app.css")
				.unwrap()
				.fs_path(),
			"app.css"
		);
		assert!(public_asset(&manifest, "/static/unmanifested.css").is_none());
		assert!(path_is_under_public_static_base(
			&manifest,
			"/static/missing.css"
		));
		assert!(public_asset(&manifest, "/app.css").is_none());
	}

	#[test]
	fn public_asset_rejects_unmanifested_ambiguous_paths() {
		let mut manifest = manifest("bad");
		manifest.public_static_base_path = "/".to_owned();
		manifest.public_filepaths = vec!["/../secret.txt".to_owned()];

		assert!(public_asset(&manifest, "/../secret.txt").is_none());
	}

	#[test]
	fn public_asset_rejects_backslash_paths() {
		let mut manifest = manifest("bad");
		manifest.public_filepaths = vec![r"/static/dir\..\secret.txt".to_owned()];

		assert!(public_asset(&manifest, r"/static/dir\..\secret.txt").is_none());
	}

	#[cfg(unix)]
	#[test]
	fn live_fs_rejects_symlink_escape_public_asset() {
		let root = temp_root("symlink-escape");
		let external = root.with_extension("external-secret");
		fs::write(&external, b"secret").unwrap();
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		std::os::unix::fs::symlink(&external, root.join("public/app.css")).unwrap();
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		let error = assets.read_public_asset("/static/app.css").unwrap_err();

		assert!(error.contains("resolves outside public root"));
		fs::remove_dir_all(root).unwrap();
		fs::remove_file(external).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn live_fs_stat_rejects_symlink_escape_public_asset() {
		let root = temp_root("stat-symlink-escape");
		let external = root.with_extension("external-secret");
		fs::write(&external, b"secret").unwrap();
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		std::os::unix::fs::symlink(&external, root.join("public/app.css")).unwrap();
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		let error = assets.stat_public_asset("/static/app.css").unwrap_err();

		assert!(error.contains("resolves outside public root"));
		fs::remove_dir_all(root).unwrap();
		fs::remove_file(external).unwrap();
	}

	#[test]
	fn live_fs_rejects_unmanifested_asset_under_non_root_base() {
		let root = temp_root("unmanifested-non-root-base");
		write_manifest(&root, ManifestMode::Dev, &manifest("one"));
		fs::write(root.join("public/unmanifested.css"), b"extra").unwrap();
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		assert!(
			assets
				.read_public_asset("/static/unmanifested.css")
				.unwrap()
				.is_none()
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn live_fs_rejects_unmanifested_asset_when_base_is_root() {
		let root = temp_root("unmanifested-root-base");
		let mut manifest = manifest("root");
		manifest.public_static_base_path = "/".to_owned();
		manifest.public_filepaths = vec!["/app.css".to_owned()];
		write_manifest(&root, ManifestMode::Dev, &manifest);
		fs::write(root.join("public/unmanifested.css"), b"extra").unwrap();
		let assets = RuntimeAssets::live_fs(&root, ManifestMode::Dev);

		assert!(
			assets
				.read_public_asset("/unmanifested.css")
				.unwrap()
				.is_none()
		);
		fs::remove_dir_all(root).unwrap();
	}
}
