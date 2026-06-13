//! Manifest-listed public asset capabilities.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt;
use std::sync::Arc;

use vorma_matcher::ensure_leading_and_trailing_slash;

/// Public asset capability table.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AssetCapabilities {
	inner: Arc<AssetCapabilitiesInner>,
}

#[derive(Debug, Eq, PartialEq)]
struct AssetCapabilitiesInner {
	public_static_base: String,
	final_public_filepaths: BTreeSet<String>,
	public_filemap: BTreeMap<String, String>,
}

impl AssetCapabilities {
	/// Create a capability table from manifest public paths and logical source mapping.
	pub fn new(
		public_static_base: impl Into<String>,
		public_filepaths: impl IntoIterator<Item = String>,
		public_filemap: BTreeMap<String, String>,
	) -> Result<Self, AssetCapabilityError> {
		let public_static_base = normalize_public_static_base(public_static_base.into())?;
		let mut final_public_filepaths = BTreeSet::new();
		for path in public_filepaths {
			validate_manifest_public_path(&public_static_base, &path)?;
			if !final_public_filepaths.insert(path.clone()) {
				return Err(AssetCapabilityError::DuplicateManifestPublicPath { path });
			}
		}
		for (source_path, public_path) in &public_filemap {
			if safe_public_asset_fs_path(source_path).is_none()
				|| !final_public_filepaths.contains(public_path)
			{
				return Err(AssetCapabilityError::InvalidPublicFileMapOutput {
					source_path: source_path.clone(),
					public_path: public_path.clone(),
				});
			}
			validate_manifest_public_path(&public_static_base, public_path)?;
		}
		Ok(Self {
			inner: Arc::new(AssetCapabilitiesInner {
				public_static_base,
				final_public_filepaths,
				public_filemap,
			}),
		})
	}

	/// Resolve a logical public source path to a generated public URL.
	pub fn public_url(&self, src_path: &str) -> Result<&str, PublicUrlError> {
		let clean = public_source_path_key(src_path).ok_or(PublicUrlError::EmptySourcePath)?;
		self.inner
			.public_filemap
			.get(clean)
			.map(String::as_str)
			.ok_or_else(|| PublicUrlError::MissingSourcePath {
				src_path: src_path.to_owned(),
			})
	}

	/// Resolve a request path to a manifest-listed public asset.
	pub fn asset_for_request_path(&self, request_path: &str) -> Option<PublicAsset> {
		if !self.inner.final_public_filepaths.contains(request_path) {
			return None;
		}
		let fs_path = if self.inner.public_static_base == "/" {
			request_path.trim_start_matches('/')
		} else {
			request_path.strip_prefix(&self.inner.public_static_base)?
		};
		let fs_path = safe_public_asset_fs_path(fs_path)?;
		Some(PublicAsset {
			fs_path,
			cache_control: "public, max-age=31536000, immutable",
		})
	}
}

/// Public URL resolution error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PublicUrlError {
	/// Source path is empty after public-source normalization.
	EmptySourcePath,
	/// Source path is absent from the committed public file map.
	MissingSourcePath {
		/// Source path requested by application code.
		src_path: String,
	},
}

impl fmt::Display for PublicUrlError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::EmptySourcePath => f.write_str("public URL source path is empty"),
			Self::MissingSourcePath { src_path } => {
				write!(f, "file {src_path} not found in manifest public filemap")
			}
		}
	}
}

impl std::error::Error for PublicUrlError {}

/// Resolved public asset capability.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublicAsset {
	fs_path: String,
	cache_control: &'static str,
}

impl PublicAsset {
	/// Filesystem path below the committed public output directory.
	pub fn fs_path(&self) -> &str {
		&self.fs_path
	}

	/// Cache-control policy.
	pub fn cache_control(&self) -> &'static str {
		self.cache_control
	}
}

/// Asset capability table error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AssetCapabilityError {
	/// Public static base is invalid.
	InvalidPublicStaticBase {
		/// Rejected public static base value.
		value: String,
	},
	/// Manifest listed an invalid public path.
	InvalidManifestPublicPath {
		/// Rejected manifest public path.
		path: String,
	},
	/// Manifest listed the same public path more than once.
	DuplicateManifestPublicPath {
		/// Duplicated manifest public path.
		path: String,
	},
	/// Public file map pointed at a non-capability output.
	InvalidPublicFileMapOutput {
		/// Static source path.
		source_path: String,
		/// Rejected public output path.
		public_path: String,
	},
}

impl std::fmt::Display for AssetCapabilityError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidPublicStaticBase { value } => {
				write!(f, "invalid public static base {value:?}")
			}
			Self::InvalidManifestPublicPath { path } => {
				write!(f, "invalid manifest public path {path:?}")
			}
			Self::DuplicateManifestPublicPath { path } => {
				write!(f, "duplicate manifest public path {path:?}")
			}
			Self::InvalidPublicFileMapOutput {
				source_path,
				public_path,
			} => write!(
				f,
				"public file map source {source_path:?} points at non-capability output {public_path:?}"
			),
		}
	}
}

impl std::error::Error for AssetCapabilityError {}

fn normalize_public_static_base(value: String) -> Result<String, AssetCapabilityError> {
	let trimmed = value.trim();
	if trimmed.is_empty() {
		return Err(AssetCapabilityError::InvalidPublicStaticBase { value });
	}
	Ok(ensure_leading_and_trailing_slash(trimmed))
}

fn validate_manifest_public_path(base: &str, path: &str) -> Result<(), AssetCapabilityError> {
	if !path.starts_with('/') {
		return Err(AssetCapabilityError::InvalidManifestPublicPath {
			path: path.to_owned(),
		});
	}
	if base != "/" && !path.starts_with(base) {
		return Err(AssetCapabilityError::InvalidManifestPublicPath {
			path: path.to_owned(),
		});
	}
	if safe_public_asset_fs_path(path.trim_start_matches('/')).is_none() {
		return Err(AssetCapabilityError::InvalidManifestPublicPath {
			path: path.to_owned(),
		});
	}
	Ok(())
}

/// Normalize a user-facing public static source path for public file-map lookups.
pub fn public_source_path_key(src_path: &str) -> Option<&str> {
	let src_path = src_path.trim().trim_start_matches('/');
	if src_path.is_empty() {
		return None;
	}
	Some(src_path)
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
	use super::*;

	#[test]
	fn capabilities_resolve_only_manifest_listed_public_paths() {
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
		)
		.unwrap();

		assert_eq!(
			capabilities
				.asset_for_request_path("/static/app.hash.css")
				.unwrap()
				.fs_path(),
			"app.hash.css"
		);
		assert!(
			capabilities
				.asset_for_request_path("/static/missing.css")
				.is_none()
		);
		assert_eq!(
			capabilities.public_url("app.css").unwrap(),
			"/static/app.hash.css"
		);
	}

	#[test]
	fn capabilities_resolve_public_urls_from_normalized_source_paths() {
		let capabilities = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
		)
		.unwrap();

		assert_eq!(
			capabilities.public_url(" //app.css ").unwrap(),
			"/static/app.hash.css"
		);
		assert_eq!(
			capabilities.public_url(" ").unwrap_err(),
			PublicUrlError::EmptySourcePath
		);
		assert!(matches!(
			capabilities.public_url("missing.css").unwrap_err(),
			PublicUrlError::MissingSourcePath { .. }
		));
	}

	#[test]
	fn capabilities_reject_ambiguous_manifest_paths() {
		let error = AssetCapabilities::new("/", vec!["/../secret.txt".to_owned()], BTreeMap::new())
			.unwrap_err();

		assert!(matches!(
			error,
			AssetCapabilityError::InvalidManifestPublicPath { .. }
		));
	}

	#[test]
	fn capabilities_reject_duplicate_manifest_public_paths() {
		let error = AssetCapabilities::new(
			"/static/",
			vec![
				"/static/app.hash.css".to_owned(),
				"/static/app.hash.css".to_owned(),
			],
			BTreeMap::new(),
		)
		.unwrap_err();

		assert!(matches!(
			error,
			AssetCapabilityError::DuplicateManifestPublicPath { .. }
		));
	}

	#[test]
	fn capabilities_reject_filemap_outputs_that_are_not_manifest_listed() {
		let error = AssetCapabilities::new(
			"/static/",
			vec!["/static/app.hash.css".to_owned()],
			BTreeMap::from([("other.css".to_owned(), "/static/other.hash.css".to_owned())]),
		)
		.unwrap_err();

		assert!(matches!(
			error,
			AssetCapabilityError::InvalidPublicFileMapOutput { .. }
		));
	}
}
