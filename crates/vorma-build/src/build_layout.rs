use std::fs;
use std::path::{Path, PathBuf};

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
	fs::metadata(&tmp)?;
	if let Some(parent) = out.parent() {
		fs::create_dir_all(parent)?;
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
