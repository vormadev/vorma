use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use path_clean::PathClean;
use path_slash::PathBufExt;
use vorma::__private::constants::PUBLIC_STATIC_OUT_NAME_PREFIX;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct PublicStaticInputs {
	source_dir: PathBuf,
	base_path: String,
}

impl PublicStaticInputs {
	pub(crate) fn new(
		root_dir: &Path,
		source_dir: &str,
		base_path: String,
	) -> Result<Self, String> {
		let source_dir = required_root_path(
			root_dir,
			source_dir,
			"frontend_config.public_static_src_dir",
		)?;
		if !source_dir.starts_with(root_dir) {
			return Err("frontend_config.public_static_src_dir must be inside root_dir".to_owned());
		}
		if source_dir == root_dir {
			return Err("frontend_config.public_static_src_dir must not be root_dir".to_owned());
		}
		if let Ok(metadata) = fs::symlink_metadata(&source_dir)
			&& metadata.file_type().is_symlink()
		{
			return Err("frontend_config.public_static_src_dir must not be a symlink".to_owned());
		}
		Ok(Self {
			source_dir,
			base_path,
		})
	}

	pub(crate) fn source_catch_pattern(&self) -> String {
		to_catch_dir_pattern(&self.source_dir)
	}

	pub(crate) fn collect_physical_files(&self) -> Result<crate::staticproc::Files, String> {
		crate::staticproc::collect_physical(&self.source_dir, PUBLIC_STATIC_OUT_NAME_PREFIX)
			.map_err(|err| err.to_string())
	}

	pub(crate) fn to_public_filemap(
		&self,
		files: &crate::staticproc::Files,
	) -> BTreeMap<String, String> {
		files
			.iter()
			.map(|(rel, file)| (rel.clone(), format!("{}{}", self.base_path, file.out_name)))
			.collect()
	}
}

fn required_root_path(root_dir: &Path, path: &str, label: &str) -> Result<PathBuf, String> {
	let trimmed = path.trim();
	if trimmed.is_empty() {
		return Err(format!("{label} cannot be empty"));
	}
	let path = PathBuf::from_slash(trimmed).clean();
	if path == Path::new(".") {
		return Err(format!("{label} cannot be ."));
	}
	Ok(root_path_from(root_dir, path))
}

fn root_path_from(root_dir: &Path, path: impl AsRef<Path>) -> PathBuf {
	let path = path.as_ref();
	if path.is_absolute() {
		return path.clean();
	}
	root_dir.join(path).clean()
}

fn to_catch_dir_pattern(path: impl AsRef<Path>) -> String {
	let mut path = path.as_ref().to_path_buf();
	path.push("**/*");
	path.clean().to_string_lossy().into_owned()
}
