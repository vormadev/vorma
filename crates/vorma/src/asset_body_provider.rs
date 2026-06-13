//! Public asset body providers.

use std::collections::HashMap;
use std::future::Future;
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use bytes::Bytes;
use http::HeaderValue;
use tokio::fs;

use crate::asset_capabilities::PublicAsset;

/// Future returned by a public asset body provider.
pub type PublicAssetBodyFuture =
	Pin<Box<dyn Future<Output = Result<PublicAssetBody, PublicAssetBodyError>> + Send + 'static>>;

/// Loader for bytes behind an already-authorized public asset capability.
pub trait PublicAssetBodyProvider: Send + Sync {
	/// Load the body for a committed public asset capability.
	fn body_for_asset(&self, asset: PublicAsset) -> PublicAssetBodyFuture;
}

impl<F, Fut> PublicAssetBodyProvider for F
where
	F: Fn(PublicAsset) -> Fut + Send + Sync,
	Fut: Future<Output = Result<PublicAssetBody, PublicAssetBodyError>> + Send + 'static,
{
	fn body_for_asset(&self, asset: PublicAsset) -> PublicAssetBodyFuture {
		Box::pin(self(asset))
	}
}

/// Loaded public asset body.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublicAssetBody {
	body: Bytes,
	content_type: Option<HeaderValue>,
}

impl PublicAssetBody {
	/// Create a public asset body.
	pub fn new(body: Bytes) -> Self {
		Self {
			body,
			content_type: None,
		}
	}

	/// Attach a content type header.
	pub fn with_content_type(mut self, content_type: HeaderValue) -> Self {
		self.content_type = Some(content_type);
		self
	}

	/// Asset bytes.
	pub fn body(&self) -> &Bytes {
		&self.body
	}

	/// Optional content type header.
	pub fn content_type(&self) -> Option<&HeaderValue> {
		self.content_type.as_ref()
	}
}

/// Public asset loading error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublicAssetBodyError {
	message: String,
}

impl PublicAssetBodyError {
	/// Create a public asset body loading error.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}

	/// Error message.
	pub fn message(&self) -> &str {
		&self.message
	}
}

/// Filesystem-backed public asset body provider for committed public output directories.
#[derive(Clone, Debug)]
pub struct PublicAssetDirectory {
	root: PathBuf,
	/*
	Public outputs are content-hashed, so a given path's bytes are immutable
	for the lifetime of a deployment and cached bodies never need
	invalidation. Dev skips the cache so freshly published generations are
	always re-read from disk.
	*/
	body_cache: Option<Arc<Mutex<HashMap<PathBuf, PublicAssetBody>>>>,
}

impl PublicAssetDirectory {
	/// Create a public asset directory from a committed public output root.
	pub fn new(root: impl Into<PathBuf>) -> Result<Self, PublicAssetDirectoryError> {
		let root = root.into();
		let root = std::fs::canonicalize(&root).map_err(|error| {
			PublicAssetDirectoryError::RootCanonicalize {
				path: root.display().to_string(),
				message: error.to_string(),
			}
		})?;
		let body_cache = (!crate::envutil::is_dev()).then(|| Arc::new(Mutex::new(HashMap::new())));
		Ok(Self { root, body_cache })
	}

	/// Canonical public output root.
	pub fn root(&self) -> &Path {
		&self.root
	}
}

impl PublicAssetBodyProvider for PublicAssetDirectory {
	fn body_for_asset(&self, asset: PublicAsset) -> PublicAssetBodyFuture {
		let root = self.root.clone();
		let body_cache = self.body_cache.clone();
		Box::pin(async move {
			let cache_key = body_cache.as_ref().and_then(|_| {
				safe_relative_asset_path(asset.fs_path()).map(|relative| root.join(relative))
			});
			if let (Some(cache), Some(cache_key)) = (&body_cache, &cache_key) {
				let cached = cache
					.lock()
					.expect("public asset body cache lock poisoned")
					.get(cache_key)
					.cloned();
				if let Some(cached) = cached {
					return Ok(cached);
				}
			}
			let body = load_public_asset_body(&root, &asset)
				.await
				.map_err(|error| PublicAssetBodyError::new(error.to_string()))?;
			if let (Some(cache), Some(cache_key)) = (body_cache, cache_key) {
				cache
					.lock()
					.expect("public asset body cache lock poisoned")
					.insert(cache_key, body.clone());
			}
			Ok(body)
		})
	}
}

/// Public asset directory error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PublicAssetDirectoryError {
	/// Public output root could not be canonicalized.
	RootCanonicalize {
		/// Rejected root path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Asset filesystem path was not a safe relative path.
	UnsafeAssetPath {
		/// Rejected asset path.
		path: String,
	},
	/// Asset canonical path escaped the public output root.
	AssetEscapesRoot {
		/// Rejected asset path.
		path: String,
	},
	/// Asset file could not be read.
	Read {
		/// Asset path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Guessed content type could not be converted into a header value.
	ContentTypeHeader {
		/// Rejected content type.
		value: String,
	},
}

impl std::fmt::Display for PublicAssetDirectoryError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::RootCanonicalize { path, message } => {
				write!(f, "canonicalize public asset root {path}: {message}")
			}
			Self::UnsafeAssetPath { path } => write!(f, "unsafe public asset path {path}"),
			Self::AssetEscapesRoot { path } => {
				write!(f, "public asset path escapes committed root: {path}")
			}
			Self::Read { path, message } => write!(f, "read public asset {path}: {message}"),
			Self::ContentTypeHeader { value } => {
				write!(f, "invalid public asset content type header {value}")
			}
		}
	}
}

impl std::error::Error for PublicAssetDirectoryError {}

async fn load_public_asset_body(
	root: &Path,
	asset: &PublicAsset,
) -> Result<PublicAssetBody, PublicAssetDirectoryError> {
	let relative_path = safe_relative_asset_path(asset.fs_path()).ok_or_else(|| {
		PublicAssetDirectoryError::UnsafeAssetPath {
			path: asset.fs_path().to_owned(),
		}
	})?;
	let full_path = root.join(relative_path);
	let canonical_path =
		fs::canonicalize(&full_path)
			.await
			.map_err(|error| PublicAssetDirectoryError::Read {
				path: full_path.display().to_string(),
				message: error.to_string(),
			})?;
	if !canonical_path.starts_with(root) {
		return Err(PublicAssetDirectoryError::AssetEscapesRoot {
			path: canonical_path.display().to_string(),
		});
	}
	let bytes =
		fs::read(&canonical_path)
			.await
			.map_err(|error| PublicAssetDirectoryError::Read {
				path: canonical_path.display().to_string(),
				message: error.to_string(),
			})?;
	let content_type = mime_guess::from_path(asset.fs_path())
		.first_or_octet_stream()
		.to_string();
	let content_type = HeaderValue::from_str(&content_type).map_err(|_| {
		PublicAssetDirectoryError::ContentTypeHeader {
			value: content_type.clone(),
		}
	})?;
	Ok(PublicAssetBody::new(Bytes::from(bytes)).with_content_type(content_type))
}

fn safe_relative_asset_path(path: &str) -> Option<&Path> {
	let path = Path::new(path);
	if path.is_absolute()
		|| path
			.components()
			.any(|component| !matches!(component, std::path::Component::Normal(_)))
	{
		return None;
	}
	Some(path)
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs as std_fs;
	use std::time::{SystemTime, UNIX_EPOCH};

	use crate::asset_capabilities::AssetCapabilities;

	use super::*;

	fn temp_public_root(name: &str) -> PathBuf {
		let stamp = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-{name}-{stamp}"))
	}

	#[tokio::test]
	async fn public_asset_directory_caches_bodies_outside_dev() {
		let root = temp_public_root("asset-dir-cache");
		std_fs::create_dir_all(&root).unwrap();
		std_fs::write(root.join("app.css"), b"body{}").unwrap();
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.css".to_owned()],
			BTreeMap::new(),
		)
		.unwrap();
		let provider = PublicAssetDirectory::new(&root).unwrap();

		let first = provider
			.body_for_asset(
				capabilities
					.asset_for_request_path("/static/app.css")
					.unwrap(),
			)
			.await
			.unwrap();
		std_fs::write(root.join("app.css"), b"main{}").unwrap();
		let second = provider
			.body_for_asset(
				capabilities
					.asset_for_request_path("/static/app.css")
					.unwrap(),
			)
			.await
			.unwrap();

		assert_eq!(first.body(), &Bytes::from_static(b"body{}"));
		assert_eq!(second.body(), &Bytes::from_static(b"body{}"));
		std_fs::remove_dir_all(root).unwrap();
	}

	#[tokio::test]
	async fn public_asset_directory_rereads_bodies_in_dev_mode() {
		let root = temp_public_root("asset-dir-dev-reread");
		std_fs::create_dir_all(&root).unwrap();
		std_fs::write(root.join("app.css"), b"body{}").unwrap();
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.css".to_owned()],
			BTreeMap::new(),
		)
		.unwrap();
		let provider = crate::envutil::with_dev_mode(|| PublicAssetDirectory::new(&root)).unwrap();

		let first = provider
			.body_for_asset(
				capabilities
					.asset_for_request_path("/static/app.css")
					.unwrap(),
			)
			.await
			.unwrap();
		std_fs::write(root.join("app.css"), b"main{}").unwrap();
		let second = provider
			.body_for_asset(
				capabilities
					.asset_for_request_path("/static/app.css")
					.unwrap(),
			)
			.await
			.unwrap();

		assert_eq!(first.body(), &Bytes::from_static(b"body{}"));
		assert_eq!(second.body(), &Bytes::from_static(b"main{}"));
		std_fs::remove_dir_all(root).unwrap();
	}

	#[cfg(unix)]
	#[tokio::test]
	async fn public_asset_directory_rejects_symlink_escape() {
		let base = temp_public_root("asset-dir-symlink-escape");
		let root = base.join("public");
		std_fs::create_dir_all(&root).unwrap();
		std_fs::write(base.join("outside.css"), b"secret{}").unwrap();
		std::os::unix::fs::symlink(base.join("outside.css"), root.join("app.css")).unwrap();
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.css".to_owned()],
			BTreeMap::new(),
		)
		.unwrap();
		let provider = PublicAssetDirectory::new(&root).unwrap();

		let error = provider
			.body_for_asset(
				capabilities
					.asset_for_request_path("/static/app.css")
					.unwrap(),
			)
			.await
			.unwrap_err();

		assert!(error.message().contains("escapes committed root"));
		std_fs::remove_dir_all(base).unwrap();
	}

	#[tokio::test]
	async fn public_asset_directory_reads_only_authorized_asset_body() {
		let root = temp_public_root("asset-dir-read");
		std_fs::create_dir_all(&root).unwrap();
		std_fs::write(root.join("app.css"), b"body{}").unwrap();
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.css".to_owned()],
			BTreeMap::new(),
		)
		.unwrap();
		let asset = capabilities
			.asset_for_request_path("/static/app.css")
			.unwrap();
		let provider = PublicAssetDirectory::new(&root).unwrap();

		let body = provider.body_for_asset(asset).await.unwrap();

		assert_eq!(body.body(), &Bytes::from_static(b"body{}"));
		assert_eq!(body.content_type().unwrap(), "text/css");
		std_fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn public_asset_directory_rejects_missing_root() {
		let root = temp_public_root("asset-dir-missing");

		let error = PublicAssetDirectory::new(&root).unwrap_err();

		assert!(matches!(
			error,
			PublicAssetDirectoryError::RootCanonicalize { .. }
		));
	}

	#[tokio::test]
	async fn public_asset_directory_reports_missing_authorized_file() {
		let root = temp_public_root("asset-dir-missing-file");
		std_fs::create_dir_all(&root).unwrap();
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/missing.css".to_owned()],
			BTreeMap::new(),
		)
		.unwrap();
		let asset = capabilities
			.asset_for_request_path("/static/missing.css")
			.unwrap();
		let provider = PublicAssetDirectory::new(&root).unwrap();

		let error = provider.body_for_asset(asset).await.unwrap_err();

		assert!(error.message().contains("read public asset"));
		std_fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn safe_relative_asset_path_rejects_absolute_and_parent_paths() {
		assert!(safe_relative_asset_path("nested/app.css").is_some());
		assert!(safe_relative_asset_path("/app.css").is_none());
		assert!(safe_relative_asset_path("../app.css").is_none());
		assert!(safe_relative_asset_path("nested/../app.css").is_none());
	}
}
