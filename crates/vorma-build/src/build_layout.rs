use std::fs;
use std::io;
use std::path::{Path, PathBuf};

use crate::utils::create_dir_all_no_symlinks;

use vorma::__private::manifest::{MANIFEST_STATIC_OUT_DEV, MANIFEST_STATIC_OUT_PROD};

const PROD_TMP_VITE_MANIFEST_DIR: &str = "tmp";
const PROD_VITE_MANIFEST_FILENAME: &str = "vite_manifest.json";

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct BuildLayout {
	dist_dir: PathBuf,
}

impl BuildLayout {
	pub(crate) fn new(dist_dir: PathBuf) -> Self {
		Self { dist_dir }
	}

	pub(crate) fn dist_dir(&self) -> PathBuf {
		self.dist_dir.clone()
	}

	pub(crate) fn vorma_internal_root(&self) -> PathBuf {
		self.dist_dir.join(".vorma")
	}

	pub(crate) fn static_root(&self) -> PathBuf {
		self.vorma_internal_root().join("static")
	}

	pub(crate) fn public_static_out(&self) -> PathBuf {
		self.static_root().join("public")
	}

	pub(crate) fn dev_cargo_target_dir(&self) -> PathBuf {
		self.vorma_internal_root().join("cargo").join("dev")
	}

	pub(crate) fn dev_lock_out(&self) -> PathBuf {
		self.vorma_internal_root().join("dev.lock")
	}

	pub(crate) fn gitignore_out(&self) -> PathBuf {
		self.vorma_internal_root().join(".gitignore")
	}

	pub(crate) fn manifest_json_out(&self, is_dev: bool) -> PathBuf {
		let out = if is_dev {
			MANIFEST_STATIC_OUT_DEV
		} else {
			MANIFEST_STATIC_OUT_PROD
		};
		self.static_root().join(out)
	}

	pub(crate) fn prod_tmp_vite_manifest_dir(&self) -> PathBuf {
		self.public_static_out().join(PROD_TMP_VITE_MANIFEST_DIR)
	}

	pub(crate) fn prod_tmp_vite_manifest_out(&self) -> PathBuf {
		self.prod_tmp_vite_manifest_dir()
			.join(vorma::__private::constants::PROD_TMP_VITE_MANIFEST_FILENAME)
	}

	pub(crate) fn prod_vite_manifest_out(&self) -> PathBuf {
		self.static_root().join(PROD_VITE_MANIFEST_FILENAME)
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct RetainedViteManifest {
	path: PathBuf,
}

impl RetainedViteManifest {
	pub(crate) fn new(path: impl Into<PathBuf>) -> Self {
		Self { path: path.into() }
	}

	pub(crate) fn path(&self) -> &Path {
		&self.path
	}
}

pub(crate) fn promote_prod_tmp_vite_manifest(
	layout: &BuildLayout,
) -> Result<RetainedViteManifest, std::io::Error> {
	let tmp = layout.prod_tmp_vite_manifest_out();
	let out = layout.prod_vite_manifest_out();
	let tmp_metadata = fs::symlink_metadata(&tmp)?;
	if tmp_metadata.file_type().is_symlink() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!(
				"temporary Vite manifest cannot be a symlink: {}",
				tmp.display()
			),
		));
	}
	if !tmp_metadata.is_file() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!("temporary Vite manifest must be a file: {}", tmp.display()),
		));
	}
	if let Some(parent) = out.parent() {
		create_dir_all_no_symlinks(parent)?;
	}
	match fs::remove_file(&out) {
		Ok(()) => {}
		Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
		Err(err) => return Err(err),
	}
	fs::rename(&tmp, &out)?;
	match fs::remove_dir(layout.prod_tmp_vite_manifest_dir()) {
		Ok(()) => Ok(RetainedViteManifest::new(out)),
		Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
			Ok(RetainedViteManifest::new(out))
		}
		Err(err) => Err(err),
	}
}

#[cfg(test)]
mod tests {
	use std::time::{SystemTime, UNIX_EPOCH};

	use super::*;

	fn temp_dist_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!(
			"vorma-build-layout-{name}-{}-{nonce}",
			std::process::id()
		));
		fs::create_dir_all(&root).unwrap();
		root
	}

	#[test]
	fn promote_prod_tmp_vite_manifest_retains_manifest_and_removes_tmp_dir() {
		let dist_dir = temp_dist_dir("promote-manifest");
		let layout = BuildLayout::new(dist_dir.clone());
		fs::create_dir_all(layout.prod_tmp_vite_manifest_dir()).unwrap();
		fs::write(layout.prod_tmp_vite_manifest_out(), "{}").unwrap();

		let retained = promote_prod_tmp_vite_manifest(&layout).unwrap();

		assert_eq!(retained.path(), layout.prod_vite_manifest_out());
		assert_eq!(
			fs::read_to_string(layout.prod_vite_manifest_out()).unwrap(),
			"{}"
		);
		assert!(!layout.prod_tmp_vite_manifest_dir().exists());
		fs::remove_dir_all(dist_dir).unwrap();
	}

	#[test]
	fn promote_prod_tmp_vite_manifest_rejects_non_file_tmp_manifest() {
		let dist_dir = temp_dist_dir("reject-non-file-manifest");
		let layout = BuildLayout::new(dist_dir.clone());
		fs::create_dir_all(layout.prod_tmp_vite_manifest_out()).unwrap();

		let err = promote_prod_tmp_vite_manifest(&layout).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(err.to_string().contains("must be a file"));
		fs::remove_dir_all(dist_dir).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn promote_prod_tmp_vite_manifest_rejects_symlink_tmp_manifest() {
		let dist_dir = temp_dist_dir("reject-symlink-manifest");
		let layout = BuildLayout::new(dist_dir.clone());
		let external = dist_dir.with_extension("external-vite-manifest");
		fs::create_dir_all(layout.prod_tmp_vite_manifest_dir()).unwrap();
		fs::write(&external, "{}").unwrap();
		std::os::unix::fs::symlink(&external, layout.prod_tmp_vite_manifest_out()).unwrap();

		let err = promote_prod_tmp_vite_manifest(&layout).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(err.to_string().contains("cannot be a symlink"));
		assert_eq!(fs::read_to_string(&external).unwrap(), "{}");
		assert!(!layout.prod_vite_manifest_out().exists());
		fs::remove_dir_all(dist_dir).unwrap();
		fs::remove_file(external).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn promote_prod_tmp_vite_manifest_rejects_symlink_retained_parent() {
		let dist_dir = temp_dist_dir("reject-symlink-retained-parent");
		let layout = BuildLayout::new(dist_dir.clone());
		let external = dist_dir.with_extension("external-static-root");
		fs::create_dir_all(external.join("public/tmp")).unwrap();
		fs::write(
			external
				.join("public/tmp")
				.join(vorma::__private::constants::PROD_TMP_VITE_MANIFEST_FILENAME),
			"{}",
		)
		.unwrap();
		fs::create_dir_all(layout.vorma_internal_root()).unwrap();
		std::os::unix::fs::symlink(&external, layout.static_root()).unwrap();

		let err = promote_prod_tmp_vite_manifest(&layout).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(
			err.to_string()
				.contains("output directory cannot be a symlink")
		);
		assert!(!external.join("vite_manifest.json").exists());
		fs::remove_file(layout.static_root()).unwrap();
		fs::remove_dir_all(dist_dir).unwrap();
		fs::remove_dir_all(external).unwrap();
	}
}
