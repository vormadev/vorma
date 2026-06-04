use std::path::{Path, PathBuf};

use path_clean::PathClean;
use path_slash::PathBufExt;
use vorma::DevWatchConfig;

use crate::config::{relative_path, sys_norm};

const BASE_WATCH_PATTERNS: &[&str] = &[
	"!.git",
	"!node_modules",
	"!target",
	"!**/target",
	"!.vorma",
	"!**/.vorma",
];

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct WatchConfig {
	watch_patterns: Vec<String>,
	server_watch_patterns: Vec<String>,
	client_revalidate_on_change_patterns: Vec<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum WatchConfigError {
	Watch(String),
	ServerWatch(String),
	ClientRevalidateOnChange(String),
}

impl WatchConfig {
	pub(crate) fn new(
		root_dir: &Path,
		vorma_internal_root: &Path,
		source: &DevWatchConfig,
	) -> Result<Self, WatchConfigError> {
		let mut watch_patterns =
			watch_patterns_or_defaults(root_dir, &source.watch_patterns, &["."], "watch pattern")
				.map_err(WatchConfigError::Watch)?;
		for pattern in BASE_WATCH_PATTERNS {
			watch_patterns.push(
				watch_pattern(root_dir, pattern, "base watch pattern")
					.map_err(WatchConfigError::Watch)?,
			);
		}
		watch_patterns.push(format!(
			"!./{}",
			watch_relative_path(root_dir, vorma_internal_root)
		));

		let server_watch_patterns = watch_patterns_or_defaults(
			root_dir,
			&source.on_change_recompile_server,
			&["**/*.rs"],
			"server watch pattern",
		)
		.map_err(WatchConfigError::ServerWatch)?;

		let client_revalidate_on_change_patterns = watch_patterns_or_defaults(
			root_dir,
			&source.on_change_client_revalidate,
			&[],
			"client revalidate watch pattern",
		)
		.map_err(WatchConfigError::ClientRevalidateOnChange)?;

		Ok(Self {
			watch_patterns,
			server_watch_patterns,
			client_revalidate_on_change_patterns,
		})
	}

	pub(crate) fn watch_patterns(&self) -> Vec<String> {
		self.watch_patterns.clone()
	}

	pub(crate) fn server_watch_patterns(&self) -> Vec<String> {
		self.server_watch_patterns.clone()
	}

	pub(crate) fn client_revalidate_on_change_patterns(&self) -> Vec<String> {
		self.client_revalidate_on_change_patterns.clone()
	}
}

fn watch_patterns_or_defaults(
	root_dir: &Path,
	configured: &[String],
	defaults: &[&str],
	label: &str,
) -> Result<Vec<String>, String> {
	if configured.is_empty() {
		return defaults
			.iter()
			.map(|pattern| watch_pattern(root_dir, pattern, label))
			.collect();
	}
	configured
		.iter()
		.map(|pattern| watch_pattern(root_dir, pattern, label))
		.collect()
}

fn watch_pattern(root_dir: &Path, pattern: &str, label: &str) -> Result<String, String> {
	let pattern = pattern.trim();
	if pattern.is_empty() {
		return Err(format!("{label} cannot be empty"));
	}
	if let Some(included) = pattern.strip_prefix('!') {
		return Ok(format!("!{}", watch_pattern(root_dir, included, label)?));
	}
	if pattern.starts_with("**/") {
		let root = watch_relative_path(root_dir, root_dir);
		if root.is_empty() || root == "." {
			return Ok(pattern.to_owned());
		}
		return Ok(format!("{root}/{pattern}"));
	}
	let rooted = root_path_from(root_dir, PathBuf::from_slash(pattern));
	let Some(relative) = relative_path(root_dir, rooted) else {
		return Err(format!("error calculating relative {label}"));
	};
	let relative = sys_norm(relative);
	if relative.is_empty() {
		return Ok(".".to_owned());
	}
	Ok(relative)
}

fn watch_relative_path(root_dir: &Path, path: impl AsRef<Path>) -> String {
	relative_path(root_dir, path)
		.map(|path| {
			if path.as_os_str().is_empty() {
				".".to_owned()
			} else {
				sys_norm(&path)
			}
		})
		.unwrap_or_else(|| sys_norm(root_dir))
}

fn root_path_from(root_dir: &Path, path: impl AsRef<Path>) -> PathBuf {
	let path = path.as_ref();
	if path.is_absolute() {
		return path.clean();
	}
	root_dir.join(path).clean()
}
