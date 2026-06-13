//! Public static output compiler and publisher.

use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::io;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{LazyLock, Mutex};
use std::time::SystemTime;

use data_encoding::BASE32_NOPAD;
use lightningcss::bundler::{Bundler, FileProvider, ResolveResult, SourceProvider};
use lightningcss::stylesheet::{MinifyOptions, ParserOptions, PrinterOptions, StyleSheet};
use lightningcss::values::url::Url as CssUrl;
use lightningcss::visit_types;
use lightningcss::visitor::{Visit, VisitTypes, Visitor};
use path_slash::PathExt;
use url::Url;
use vorma::build_interface::assets::{PUBLIC_OUT_DIR, static_out_dir as runtime_static_out_dir};
use vorma_contract::constants::PUBLIC_STATIC_OUT_NAME_PREFIX;
use walkdir::WalkDir;

use crate::build_output::{create_dir_all_no_symlinks, resolve_workspace_path};
use crate::build_plan::BuildProjectionPlan;
use crate::vite_plugin_contract::VITE_PLUGIN_PUBLIC_URL_PREFIX;

static TEMP_OUTPUT_COUNTER: AtomicU64 = AtomicU64::new(0);

/// Prepared public static outputs before filesystem publication.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedPublicStaticOutputs {
	files: Vec<PreparedPublicStaticFile>,
	public_filepaths: Vec<String>,
	public_filemap: BTreeMap<String, String>,
}

impl PreparedPublicStaticOutputs {
	/// Files prepared from the configured public static source directory.
	pub fn files(&self) -> &[PreparedPublicStaticFile] {
		&self.files
	}

	/// Manifest public file paths produced by these static outputs.
	pub fn public_filepaths(&self) -> &[String] {
		&self.public_filepaths
	}

	/// Logical source path to manifest public URL map.
	pub fn public_filemap(&self) -> &BTreeMap<String, String> {
		&self.public_filemap
	}
}

/// One prepared public static file.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedPublicStaticFile {
	source_path: String,
	source_fs_path: PathBuf,
	output_name: String,
	public_path: String,
}

impl PreparedPublicStaticFile {
	/// Logical source path below the public static source directory.
	#[cfg(test)]
	pub fn source_path(&self) -> &str {
		&self.source_path
	}

	/// Source filesystem path.
	pub fn source_fs_path(&self) -> &Path {
		&self.source_fs_path
	}

	/// Hashed output filename below the runtime public output directory.
	pub fn output_name(&self) -> &str {
		&self.output_name
	}

	/// Manifest public URL for this output.
	#[cfg(test)]
	pub fn public_path(&self) -> &str {
		&self.public_path
	}
}

/// Report returned after publishing prepared public static outputs.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PublicStaticPublishReport {
	public_output_dir: PathBuf,
	written_files: Vec<PathBuf>,
	retained_stale_files: Vec<PathBuf>,
}

/// Bundled critical CSS output.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct CriticalCssBundle {
	css: String,
	imports: Vec<String>,
}

impl CriticalCssBundle {
	/// Minified critical CSS.
	pub fn css(&self) -> &str {
		&self.css
	}

	/// Canonical CSS source files read by the bundler.
	pub fn imports(&self) -> &[String] {
		&self.imports
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ParsedCssUrl {
	path: String,
	query: Option<String>,
	fragment: Option<String>,
}

impl PublicStaticPublishReport {
	pub(crate) fn unchanged(plan: &BuildProjectionPlan) -> Result<Self, PublicStaticOutputError> {
		Ok(Self {
			public_output_dir: public_static_output_dir(plan)?,
			written_files: Vec::new(),
			retained_stale_files: Vec::new(),
		})
	}

	/// Runtime public output directory.
	#[cfg(test)]
	pub fn public_output_dir(&self) -> &Path {
		&self.public_output_dir
	}

	/// Files written during this publish operation.
	#[cfg(test)]
	pub fn written_files(&self) -> &[PathBuf] {
		&self.written_files
	}

	/// Stale framework-owned public static outputs retained during publication.
	#[cfg(test)]
	pub fn retained_stale_files(&self) -> &[PathBuf] {
		&self.retained_stale_files
	}
}

/// Prepare public static outputs from the configured source directory.
pub fn prepare_public_static_outputs(
	plan: &BuildProjectionPlan,
) -> Result<PreparedPublicStaticOutputs, PublicStaticOutputError> {
	let root_dir = canonical_root_dir(plan.workspace().root_dir())?;
	let source_dir =
		canonical_source_dir(&root_dir, plan.static_inputs().public_static_source_dir())?;
	let output_dir = public_static_output_dir_from_root(&root_dir, plan.workspace().dist_dir());
	if output_dir.starts_with(&source_dir) {
		return Err(PublicStaticOutputError::OutputDirInsideSourceDir {
			source_dir: source_dir.display().to_string(),
			output_dir: output_dir.display().to_string(),
		});
	}

	let mut files = Vec::new();
	for entry in WalkDir::new(&source_dir) {
		let entry = entry.map_err(|source| PublicStaticOutputError::WalkSource {
			path: source_dir.display().to_string(),
			message: source.to_string(),
		})?;
		let file_type = entry.file_type();
		if file_type.is_symlink() {
			return Err(PublicStaticOutputError::SymlinkSource {
				path: entry.path().display().to_string(),
			});
		}
		if file_type.is_dir() {
			continue;
		}
		if !file_type.is_file() {
			return Err(PublicStaticOutputError::NonFileSource {
				path: entry.path().display().to_string(),
			});
		}
		if entry.file_name() == ".DS_Store" {
			continue;
		}
		let source_path = entry
			.path()
			.strip_prefix(&source_dir)
			.map_err(|source| PublicStaticOutputError::SourceRelativePath {
				path: entry.path().display().to_string(),
				message: source.to_string(),
			})?
			.to_slash_lossy()
			.into_owned();
		if source_path.is_empty() {
			return Err(PublicStaticOutputError::EmptySourcePath {
				path: entry.path().display().to_string(),
			});
		}
		let output_name = hashed_output_name(&source_path, entry.path())?;
		let public_path = public_path(plan.vite_inputs().public_static_base(), &output_name);
		files.push(PreparedPublicStaticFile {
			source_path,
			source_fs_path: entry.path().to_path_buf(),
			output_name,
			public_path,
		});
	}
	files.sort_by(|left, right| left.source_path.cmp(&right.source_path));

	let public_filepaths = files
		.iter()
		.map(|file| file.public_path.clone())
		.collect::<Vec<_>>();
	let public_filemap = files
		.iter()
		.map(|file| (file.source_path.clone(), file.public_path.clone()))
		.collect::<BTreeMap<_, _>>();
	Ok(PreparedPublicStaticOutputs {
		files,
		public_filepaths,
		public_filemap,
	})
}

/// Publish prepared public static outputs into the committed runtime public directory.
pub fn publish_public_static_outputs(
	plan: &BuildProjectionPlan,
	prepared: &PreparedPublicStaticOutputs,
) -> Result<PublicStaticPublishReport, PublicStaticOutputError> {
	let root_dir = canonical_root_dir(plan.workspace().root_dir())?;
	let public_output_dir =
		public_static_output_dir_from_root(&root_dir, plan.workspace().dist_dir());
	create_dir_all_no_symlinks(&public_output_dir).map_err(|source| {
		PublicStaticOutputError::CreateOutputDir {
			path: public_output_dir.display().to_string(),
			message: source.to_string(),
		}
	})?;

	let current_outputs = prepared
		.files()
		.iter()
		.map(|file| public_output_dir.join(file.output_name()))
		.collect::<BTreeSet<_>>();
	let mut written_files = Vec::new();
	for file in prepared.files() {
		let output_path = public_output_dir.join(file.output_name());
		if output_needs_write(&output_path)? {
			copy_file_atomically(file.source_fs_path(), &output_path)?;
			written_files.push(output_path);
		}
	}

	let retained_stale_files =
		collect_stale_framework_static_outputs(&public_output_dir, &current_outputs)?;
	Ok(PublicStaticPublishReport {
		public_output_dir,
		written_files,
		retained_stale_files,
	})
}

/// Bundle critical CSS and rewrite framework public URLs through the prepared public filemap.
pub fn bundle_critical_css(
	plan: &BuildProjectionPlan,
	public_filemap: &BTreeMap<String, String>,
) -> Result<CriticalCssBundle, PublicStaticOutputError> {
	let critical_css_file = plan.static_inputs().critical_css_file().trim();
	if critical_css_file.is_empty() {
		return Ok(CriticalCssBundle::default());
	}
	let root_dir = canonical_root_dir(plan.workspace().root_dir())?;
	let entry_path = resolve_workspace_path(&root_dir, critical_css_file);
	let provider = RootedCssFileProvider::new(&root_dir)?;
	let entry_path = provider.canonical_source_path(&entry_path)?;
	let mut bundler = Bundler::new(&provider, None, ParserOptions::default());
	let mut stylesheet =
		bundler
			.bundle(&entry_path)
			.map_err(|source| PublicStaticOutputError::CriticalCss {
				message: source.to_string(),
			})?;
	let imports = stylesheet
		.sources
		.iter()
		.map(|source| Path::new(source).components().collect::<PathBuf>())
		.map(|source| source.to_string_lossy().into_owned())
		.collect();
	rewrite_critical_css_urls(&mut stylesheet, public_filemap)?;
	stylesheet
		.minify(MinifyOptions::default())
		.map_err(|source| PublicStaticOutputError::CriticalCss {
			message: source.to_string(),
		})?;
	let css = stylesheet
		.to_css(PrinterOptions {
			minify: true,
			..PrinterOptions::default()
		})
		.map_err(|source| PublicStaticOutputError::CriticalCss {
			message: source.to_string(),
		})?
		.code
		.trim()
		.to_owned();
	Ok(CriticalCssBundle { css, imports })
}

/// Runtime public output directory for this build plan.
pub fn public_static_output_dir(
	plan: &BuildProjectionPlan,
) -> Result<PathBuf, PublicStaticOutputError> {
	let root_dir = canonical_root_dir(plan.workspace().root_dir())?;
	Ok(public_static_output_dir_from_root(
		&root_dir,
		plan.workspace().dist_dir(),
	))
}

/// Public static output preparation and publication error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PublicStaticOutputError {
	/// Workspace root directory was empty.
	EmptyRootDir,
	/// Workspace root directory could not be canonicalized.
	RootDir {
		/// Rejected root path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Public static source directory could not be canonicalized.
	SourceDir {
		/// Rejected source path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Public static source directory escaped the workspace root.
	SourceDirEscapesRoot {
		/// Canonical workspace root.
		root_dir: String,
		/// Canonical public static source directory.
		source_dir: String,
	},
	/// Public static source directory was the workspace root.
	SourceDirIsRoot {
		/// Canonical workspace root.
		root_dir: String,
	},
	/// Runtime public output directory was inside the public static source directory.
	OutputDirInsideSourceDir {
		/// Canonical public static source directory.
		source_dir: String,
		/// Runtime public output directory.
		output_dir: String,
	},
	/// Walking the public static source directory failed.
	WalkSource {
		/// Source root.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// A public static source entry was a symlink.
	SymlinkSource {
		/// Rejected path.
		path: String,
	},
	/// A public static source entry was not a regular file.
	NonFileSource {
		/// Rejected path.
		path: String,
	},
	/// A source file relative path could not be produced.
	SourceRelativePath {
		/// Source path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// A source file relative path was empty.
	EmptySourcePath {
		/// Source path.
		path: String,
	},
	/// A source file could not be read for hashing or copying.
	ReadSource {
		/// Source path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Runtime public output directory could not be created.
	CreateOutputDir {
		/// Output directory.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// A temporary output file could not be written.
	WriteTempFile {
		/// Temporary output path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// A temporary output file could not be renamed into place.
	RenameTempFile {
		/// Temporary output path.
		temp_path: String,
		/// Final output path.
		final_path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Removing a conflicting current output path failed.
	RemoveConflictingOutput {
		/// Conflicting output path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Critical CSS bundling failed.
	CriticalCss {
		/// Bundler or URL rewrite error message.
		message: String,
	},
	/// A critical CSS URL used an unsupported relative path.
	InvalidCriticalCssUrl {
		/// Rejected CSS URL.
		raw: String,
	},
	/// A critical CSS public URL reference was absent from the public filemap.
	UnresolvedCriticalCssPublicUrl {
		/// Rejected CSS URL path.
		path: String,
	},
}

impl std::fmt::Display for PublicStaticOutputError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::RootDir { path, message } => write!(f, "canonicalize root_dir {path}: {message}"),
			Self::SourceDir { path, message } => {
				write!(f, "canonicalize public static source dir {path}: {message}")
			}
			Self::SourceDirEscapesRoot {
				root_dir,
				source_dir,
			} => write!(
				f,
				"public static source dir {source_dir} must be inside root_dir {root_dir}"
			),
			Self::SourceDirIsRoot { root_dir } => {
				write!(
					f,
					"public static source dir must not be root_dir {root_dir}"
				)
			}
			Self::OutputDirInsideSourceDir {
				source_dir,
				output_dir,
			} => write!(
				f,
				"public static output dir {output_dir} must not be inside source dir {source_dir}"
			),
			Self::WalkSource { path, message } => {
				write!(f, "walk public static source dir {path}: {message}")
			}
			Self::SymlinkSource { path } => {
				write!(f, "public static source cannot be a symlink: {path}")
			}
			Self::NonFileSource { path } => write!(f, "public static source is not a file: {path}"),
			Self::SourceRelativePath { path, message } => {
				write!(f, "resolve public static source path {path}: {message}")
			}
			Self::EmptySourcePath { path } => {
				write!(f, "public static source path is empty: {path}")
			}
			Self::ReadSource { path, message } => {
				write!(f, "read public static source {path}: {message}")
			}
			Self::CreateOutputDir { path, message } => {
				write!(f, "create public static output dir {path}: {message}")
			}
			Self::WriteTempFile { path, message } => {
				write!(f, "write temporary static output {path}: {message}")
			}
			Self::RenameTempFile {
				temp_path,
				final_path,
				message,
			} => write!(
				f,
				"rename temporary static output {temp_path} to {final_path}: {message}"
			),
			Self::RemoveConflictingOutput { path, message } => {
				write!(
					f,
					"remove conflicting public static output {path}: {message}"
				)
			}
			Self::CriticalCss { message } => write!(f, "bundle critical CSS: {message}"),
			Self::InvalidCriticalCssUrl { raw: _ } => write!(
				f,
				"CSS URL paths must be absolute, external, or start with {VITE_PLUGIN_PUBLIC_URL_PREFIX:?}",
			),
			Self::UnresolvedCriticalCssPublicUrl { path } => {
				write!(f, "unresolved static public asset {path:?}")
			}
		}
	}
}

impl std::error::Error for PublicStaticOutputError {}

fn canonical_root_dir(root_dir: &str) -> Result<PathBuf, PublicStaticOutputError> {
	let root_dir = PathBuf::from(root_dir);
	if root_dir.as_os_str().is_empty() {
		return Err(PublicStaticOutputError::EmptyRootDir);
	}
	std::fs::canonicalize(&root_dir).map_err(|source| PublicStaticOutputError::RootDir {
		path: root_dir.display().to_string(),
		message: source.to_string(),
	})
}

fn canonical_source_dir(
	root_dir: &Path,
	source_dir: &str,
) -> Result<PathBuf, PublicStaticOutputError> {
	let declared_source_dir = resolve_workspace_path(root_dir, source_dir);
	let source_dir = std::fs::canonicalize(&declared_source_dir).map_err(|source| {
		PublicStaticOutputError::SourceDir {
			path: declared_source_dir.display().to_string(),
			message: source.to_string(),
		}
	})?;
	if !source_dir.starts_with(root_dir) {
		return Err(PublicStaticOutputError::SourceDirEscapesRoot {
			root_dir: root_dir.display().to_string(),
			source_dir: source_dir.display().to_string(),
		});
	}
	if source_dir == root_dir {
		return Err(PublicStaticOutputError::SourceDirIsRoot {
			root_dir: root_dir.display().to_string(),
		});
	}
	Ok(source_dir)
}

fn public_static_output_dir_from_root(root_dir: &Path, dist_dir: &str) -> PathBuf {
	let dist_dir = resolve_workspace_path(root_dir, dist_dir);
	runtime_static_out_dir(dist_dir).join(PUBLIC_OUT_DIR)
}

fn hashed_output_name(
	source_path: &str,
	source_fs_path: &Path,
) -> Result<String, PublicStaticOutputError> {
	let content_hash = hash_file(source_fs_path)?;
	let mut hasher = blake3::Hasher::new();
	hasher.update(source_path.as_bytes());
	hasher.update(&[0]);
	hasher.update(content_hash.as_bytes());
	let mut encoded = BASE32_NOPAD.encode(hasher.finalize().as_bytes());
	encoded.make_ascii_lowercase();
	let ext = path_ext(source_path);
	let stem = if ext.is_empty() {
		source_path
	} else {
		source_path.strip_suffix(ext).unwrap_or(source_path)
	};
	Ok(format!(
		"{PUBLIC_STATIC_OUT_NAME_PREFIX}{}_{}{ext}",
		stem.replace('/', "_"),
		&encoded[..12]
	))
}

#[derive(Clone, Copy)]
struct CachedFileHash {
	mod_time: SystemTime,
	size: u64,
	hash: [u8; 32],
}

/// Content hashes keyed by source path so dev rebuilds re-read only files
/// whose `(mod_time, size)` stat changed since they were last hashed.
static FILE_HASH_CACHE: LazyLock<Mutex<HashMap<PathBuf, CachedFileHash>>> =
	LazyLock::new(|| Mutex::new(HashMap::new()));

/// Backstop against unbounded growth across long dev sessions that churn
/// many distinct paths; one entry costs ~the path length plus 48 bytes.
const FILE_HASH_CACHE_MAX_ENTRIES: usize = 65_536;

fn hash_file(path: &Path) -> Result<blake3::Hash, PublicStaticOutputError> {
	let metadata =
		std::fs::metadata(path).map_err(|source| PublicStaticOutputError::ReadSource {
			path: path.display().to_string(),
			message: source.to_string(),
		})?;
	let size = metadata.len();
	let mod_time = metadata.modified().ok();
	if let Some(mod_time) = mod_time
		&& let Some(cached) = FILE_HASH_CACHE
			.lock()
			.expect("file hash cache lock poisoned")
			.get(path)
		&& cached.mod_time == mod_time
		&& cached.size == size
	{
		return Ok(blake3::Hash::from(cached.hash));
	}
	let mut file =
		std::fs::File::open(path).map_err(|source| PublicStaticOutputError::ReadSource {
			path: path.display().to_string(),
			message: source.to_string(),
		})?;
	let mut hasher = blake3::Hasher::new();
	hasher
		.update_reader(&mut file)
		.map_err(|source| PublicStaticOutputError::ReadSource {
			path: path.display().to_string(),
			message: source.to_string(),
		})?;
	let hash = hasher.finalize();
	if let Some(mod_time) = mod_time {
		let mut cache = FILE_HASH_CACHE
			.lock()
			.expect("file hash cache lock poisoned");
		if cache.len() >= FILE_HASH_CACHE_MAX_ENTRIES {
			cache.clear();
		}
		cache.insert(
			path.to_path_buf(),
			CachedFileHash {
				mod_time,
				size,
				hash: *hash.as_bytes(),
			},
		);
	}
	Ok(hash)
}

fn path_ext(path: &str) -> &str {
	let last_segment = path.rsplit('/').next().unwrap_or(path);
	let Some(dot_idx) = last_segment.rfind('.') else {
		return "";
	};
	&path[path.len() - last_segment.len() + dot_idx..]
}

fn public_path(public_static_base: &str, output_name: &str) -> String {
	if public_static_base == "/" {
		return format!("/{output_name}");
	}
	format!("{public_static_base}{output_name}")
}

fn copy_file_atomically(
	source_path: &Path,
	output_path: &Path,
) -> Result<(), PublicStaticOutputError> {
	let temp_path = temp_output_path(output_path);
	let mut source =
		std::fs::File::open(source_path).map_err(|source| PublicStaticOutputError::ReadSource {
			path: source_path.display().to_string(),
			message: source.to_string(),
		})?;
	let write_result = (|| -> io::Result<()> {
		let mut temp = std::fs::OpenOptions::new()
			.write(true)
			.create_new(true)
			.open(&temp_path)?;
		io::copy(&mut source, &mut temp)?;
		temp.sync_all()?;
		Ok(())
	})();
	if let Err(source) = write_result {
		let _ = std::fs::remove_file(&temp_path);
		return Err(PublicStaticOutputError::WriteTempFile {
			path: temp_path.display().to_string(),
			message: source.to_string(),
		});
	}
	if output_path.exists() {
		if output_path.is_dir() {
			std::fs::remove_dir_all(output_path).map_err(|source| {
				PublicStaticOutputError::RemoveConflictingOutput {
					path: output_path.display().to_string(),
					message: source.to_string(),
				}
			})?;
		} else {
			std::fs::remove_file(output_path).map_err(|source| {
				PublicStaticOutputError::RemoveConflictingOutput {
					path: output_path.display().to_string(),
					message: source.to_string(),
				}
			})?;
		}
	}
	if let Err(source) = std::fs::rename(&temp_path, output_path) {
		let _ = std::fs::remove_file(&temp_path);
		return Err(PublicStaticOutputError::RenameTempFile {
			temp_path: temp_path.display().to_string(),
			final_path: output_path.display().to_string(),
			message: source.to_string(),
		});
	}
	Ok(())
}

fn output_needs_write(output_path: &Path) -> Result<bool, PublicStaticOutputError> {
	match std::fs::symlink_metadata(output_path) {
		Ok(metadata) if metadata.file_type().is_symlink() => {
			std::fs::remove_file(output_path).map_err(|source| {
				PublicStaticOutputError::RemoveConflictingOutput {
					path: output_path.display().to_string(),
					message: source.to_string(),
				}
			})?;
			Ok(true)
		}
		Ok(metadata) if metadata.is_file() => Ok(false),
		Ok(metadata) if metadata.is_dir() => {
			std::fs::remove_dir_all(output_path).map_err(|source| {
				PublicStaticOutputError::RemoveConflictingOutput {
					path: output_path.display().to_string(),
					message: source.to_string(),
				}
			})?;
			Ok(true)
		}
		Ok(_) => {
			std::fs::remove_file(output_path).map_err(|source| {
				PublicStaticOutputError::RemoveConflictingOutput {
					path: output_path.display().to_string(),
					message: source.to_string(),
				}
			})?;
			Ok(true)
		}
		Err(source) if source.kind() == io::ErrorKind::NotFound => Ok(true),
		Err(source) => Err(PublicStaticOutputError::RemoveConflictingOutput {
			path: output_path.display().to_string(),
			message: source.to_string(),
		}),
	}
}

fn collect_stale_framework_static_outputs(
	public_output_dir: &Path,
	current_outputs: &BTreeSet<PathBuf>,
) -> Result<Vec<PathBuf>, PublicStaticOutputError> {
	let mut retained = Vec::new();
	for entry in WalkDir::new(public_output_dir).min_depth(1) {
		let entry = entry.map_err(|source| PublicStaticOutputError::WalkSource {
			path: public_output_dir.display().to_string(),
			message: source.to_string(),
		})?;
		if entry.file_type().is_dir() {
			continue;
		}
		if !entry
			.file_name()
			.to_string_lossy()
			.starts_with(PUBLIC_STATIC_OUT_NAME_PREFIX)
		{
			continue;
		}
		let path = entry.path().to_path_buf();
		if current_outputs.contains(&path) {
			continue;
		}
		retained.push(path);
	}
	Ok(retained)
}

fn temp_output_path(path: &Path) -> PathBuf {
	let file_name = path
		.file_name()
		.and_then(|value| value.to_str())
		.unwrap_or("static-output");
	let nanos = std::time::SystemTime::now()
		.duration_since(std::time::UNIX_EPOCH)
		.expect("system time should be after UNIX epoch")
		.as_nanos();
	let sequence = TEMP_OUTPUT_COUNTER.fetch_add(1, Ordering::Relaxed);
	path.with_file_name(format!("{file_name}.tmp.{nanos}.{sequence}"))
}

struct RootedCssFileProvider {
	inner: FileProvider,
	canonical_root_dir: PathBuf,
}

impl RootedCssFileProvider {
	fn new(root_dir: &Path) -> Result<Self, PublicStaticOutputError> {
		Ok(Self {
			inner: FileProvider::new(),
			canonical_root_dir: root_dir.to_path_buf(),
		})
	}

	fn canonical_source_path(&self, path: &Path) -> Result<PathBuf, PublicStaticOutputError> {
		let canonical =
			std::fs::canonicalize(path).map_err(|source| PublicStaticOutputError::CriticalCss {
				message: format!("resolve critical CSS source {}: {source}", path.display()),
			})?;
		if !canonical.starts_with(&self.canonical_root_dir) {
			return Err(PublicStaticOutputError::CriticalCss {
				message: format!(
					"critical CSS source must stay inside root_dir: {}",
					canonical.display()
				),
			});
		}
		Ok(canonical)
	}
}

impl SourceProvider for RootedCssFileProvider {
	type Error = io::Error;

	fn read<'a>(&'a self, file: &Path) -> Result<&'a str, Self::Error> {
		let canonical = self
			.canonical_source_path(file)
			.map_err(|source| io::Error::new(io::ErrorKind::InvalidInput, source.to_string()))?;
		self.inner.read(&canonical)
	}

	fn resolve(
		&self,
		specifier: &str,
		originating_file: &Path,
	) -> Result<ResolveResult, Self::Error> {
		match self.inner.resolve(specifier, originating_file)? {
			ResolveResult::External(url) => Ok(ResolveResult::External(url)),
			ResolveResult::File(path) => {
				let path = self.canonical_source_path(&path).map_err(|source| {
					io::Error::new(io::ErrorKind::InvalidInput, source.to_string())
				})?;
				Ok(ResolveResult::File(path))
			}
		}
	}
}

fn rewrite_critical_css_urls(
	stylesheet: &mut StyleSheet<'_, '_>,
	public_filemap: &BTreeMap<String, String>,
) -> Result<(), PublicStaticOutputError> {
	let mut visitor = CriticalCssPublicUrlRewriteVisitor { public_filemap };
	stylesheet.visit(&mut visitor)?;
	Ok(())
}

struct CriticalCssPublicUrlRewriteVisitor<'a> {
	public_filemap: &'a BTreeMap<String, String>,
}

impl<'i> Visitor<'i> for CriticalCssPublicUrlRewriteVisitor<'_> {
	type Error = PublicStaticOutputError;

	fn visit_types(&self) -> VisitTypes {
		visit_types!(URLS)
	}

	fn visit_url(&mut self, url: &mut CssUrl<'i>) -> Result<(), Self::Error> {
		let raw = url.url.trim().to_owned();
		let Some(parsed) = parse_rewritable_critical_css_url(&raw) else {
			return Ok(());
		};
		if let Some(resolved) = resolve_critical_css_url(&raw, &parsed, self.public_filemap)? {
			url.url = resolved.into();
		}
		Ok(())
	}
}

fn parse_rewritable_critical_css_url(raw: &str) -> Option<ParsedCssUrl> {
	if raw.starts_with("//") || Url::parse(raw).is_ok() {
		return None;
	}
	let (without_fragment, fragment) = match raw.split_once('#') {
		Some((before, after)) => (before, Some(after.to_owned())),
		None => (raw, None),
	};
	let (path, query) = match without_fragment.split_once('?') {
		Some((before, after)) => (before.to_owned(), Some(after.to_owned())),
		None => (without_fragment.to_owned(), None),
	};
	if path.is_empty() && fragment.is_some() && raw.trim_start().starts_with('#') {
		return None;
	}
	Some(ParsedCssUrl {
		path,
		query,
		fragment,
	})
}

fn resolve_critical_css_url(
	raw: &str,
	parsed: &ParsedCssUrl,
	public_filemap: &BTreeMap<String, String>,
) -> Result<Option<String>, PublicStaticOutputError> {
	if parsed.path.starts_with('/') {
		return Ok(Some(raw.to_owned()));
	}
	if !parsed.path.starts_with(VITE_PLUGIN_PUBLIC_URL_PREFIX) {
		return Err(PublicStaticOutputError::InvalidCriticalCssUrl {
			raw: raw.to_owned(),
		});
	}
	let lookup = parsed
		.path
		.strip_prefix(VITE_PLUGIN_PUBLIC_URL_PREFIX)
		.expect("critical CSS public URL path should have framework prefix");
	let Some(public_url) = public_filemap.get(lookup) else {
		return Err(PublicStaticOutputError::UnresolvedCriticalCssPublicUrl {
			path: parsed.path.clone(),
		});
	};
	let mut resolved = public_url.clone();
	if let Some(query) = &parsed.query {
		resolved.push('?');
		resolved.push_str(query);
	}
	if let Some(fragment) = &parsed.fragment {
		resolved.push('#');
		resolved.push_str(fragment);
	}
	Ok(Some(resolved))
}

#[cfg(test)]
mod tests {
	use std::fs;

	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};

	use super::*;
	use crate::projection_compiler::{
		ClientModuleArtifacts, CompletedBuildArtifacts, ProjectionBundle,
	};
	use crate::test_support::route_type_contract;
	use crate::test_support::unique_temp_root;

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_ROOT_DOCUMENT_HASH_SOURCE: &str = "document-hash-source";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn temp_root() -> PathBuf {
		unique_temp_root("vorma-build-static")
	}

	fn plan(root_dir: &Path) -> BuildProjectionPlan {
		let mut declarations = FrameworkDeclarations::new(
			FrameworkConfig::new(TEST_PUBLIC_STATIC_BASE).with_build_inputs(BuildInputConfig::new(
				ServerBuildTarget::new(TEST_CARGO_PACKAGE, TEST_CARGO_BIN),
				root_dir.display().to_string(),
				TEST_DIST_DIR,
				FrontendBuildInputs::new(
					"react",
					"pnpm",
					".",
					"vite.config.ts",
					"src/entry.tsx",
					TEST_PUBLIC_STATIC_SOURCE_DIR,
					"src/critical.css",
				),
				"src/vorma.gen.ts",
				DevWatchConfig::default(),
			)),
		);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		let bundle = ProjectionBundle::compile(&FrameworkGraph::compile(declarations).unwrap());
		BuildProjectionPlan::compile(&bundle).unwrap()
	}

	#[test]
	fn public_static_outputs_hash_publish_and_project_capability_facts() {
		let root_dir = temp_root();
		let public_dir = root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR);
		fs::create_dir_all(public_dir.join("img")).unwrap();
		fs::write(public_dir.join("app.css"), "body{}").unwrap();
		fs::write(public_dir.join("img/logo.svg"), "<svg></svg>").unwrap();
		fs::write(public_dir.join(".DS_Store"), "ignored").unwrap();
		let plan = plan(&root_dir);

		let prepared = prepare_public_static_outputs(&plan).unwrap();

		assert_eq!(prepared.files().len(), 2);
		assert_eq!(prepared.files()[0].source_path(), "app.css");
		assert!(
			prepared.files()[0]
				.output_name()
				.starts_with(PUBLIC_STATIC_OUT_NAME_PREFIX)
		);
		assert!(prepared.files()[0].output_name().ends_with(".css"));
		assert_eq!(
			prepared.public_filemap()["app.css"],
			prepared.files()[0].public_path()
		);
		assert_eq!(prepared.public_filepaths().len(), 2);

		let output_dir = public_static_output_dir(&plan).unwrap();
		fs::create_dir_all(&output_dir).unwrap();
		let stale_path = output_dir.join("vorma_out_stale_aaaaaaaaaaaa.css");
		let vite_path = output_dir.join("root.js");
		let blocked_output_path = output_dir.join(prepared.files()[0].output_name());
		fs::write(&stale_path, "stale").unwrap();
		fs::write(&vite_path, "export {};").unwrap();
		fs::create_dir_all(&blocked_output_path).unwrap();

		let report = publish_public_static_outputs(&plan, &prepared).unwrap();

		assert_eq!(report.written_files().len(), 2);
		assert_eq!(
			report.retained_stale_files(),
			std::slice::from_ref(&stale_path)
		);
		assert!(stale_path.exists());
		assert!(vite_path.exists());
		assert!(blocked_output_path.is_file());
		for file in prepared.files() {
			assert!(report.public_output_dir().join(file.output_name()).exists());
		}

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[tokio::test]
	async fn completed_build_artifacts_accept_prepared_public_static_outputs() {
		let root_dir = temp_root();
		let public_dir = root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR);
		fs::create_dir_all(&public_dir).unwrap();
		fs::write(public_dir.join("app.css"), "body{}").unwrap();
		let plan = plan(&root_dir);
		let prepared = prepare_public_static_outputs(&plan).unwrap();
		let artifacts = CompletedBuildArtifacts::new(
			"critical-css",
			prepared.public_filepaths().to_vec(),
			prepared.public_filemap().clone(),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		)
		.into_generation_artifacts(&vorma::DocumentBuilder::default())
		.await
		.unwrap();

		assert_eq!(
			artifacts.public_filemap()["app.css"],
			prepared.public_filemap()["app.css"]
		);
		assert_ne!(
			artifacts.root_document_hash_source(),
			TEST_ROOT_DOCUMENT_HASH_SOURCE
		);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn critical_css_bundle_rewrites_public_urls_and_tracks_imports() {
		let root_dir = temp_root();
		let public_dir = root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR);
		let src_dir = root_dir.join("src");
		fs::create_dir_all(&public_dir).unwrap();
		fs::create_dir_all(&src_dir).unwrap();
		fs::write(public_dir.join("logo.svg"), "<svg></svg>").unwrap();
		fs::write(
			src_dir.join("critical.css"),
			r#"@import "./base.css"; .hero { background: url("@public/logo.svg?v=1#mark"); }"#,
		)
		.unwrap();
		fs::write(src_dir.join("base.css"), "body { margin: 0; }").unwrap();
		let plan = plan(&root_dir);
		let prepared = prepare_public_static_outputs(&plan).unwrap();

		let bundle = bundle_critical_css(&plan, prepared.public_filemap()).unwrap();

		assert!(bundle.css().contains("body{margin:0}"));
		assert!(bundle.css().contains("?v=1#mark"));
		assert!(
			bundle
				.css()
				.contains(prepared.public_filemap()["logo.svg"].as_str())
		);
		assert!(
			bundle.imports().contains(
				&fs::canonicalize(src_dir.join("critical.css"))
					.unwrap()
					.display()
					.to_string()
			)
		);
		assert!(
			bundle.imports().contains(
				&fs::canonicalize(src_dir.join("base.css"))
					.unwrap()
					.display()
					.to_string()
			)
		);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn critical_css_bundle_rejects_imports_outside_root_dir() {
		let base = temp_root();
		let root_dir = base.join("app");
		let src_dir = root_dir.join("src");
		fs::create_dir_all(&src_dir).unwrap();
		fs::write(base.join("outside.css"), "body { margin: 0; }").unwrap();
		fs::write(
			src_dir.join("critical.css"),
			r#"@import "../../outside.css";"#,
		)
		.unwrap();
		let plan = plan(&root_dir);

		let error = bundle_critical_css(&plan, &BTreeMap::new()).unwrap_err();

		assert!(matches!(error, PublicStaticOutputError::CriticalCss { .. }));
		fs::remove_dir_all(base).unwrap();
	}

	#[test]
	fn critical_css_bundle_rejects_relative_non_public_urls() {
		let root_dir = temp_root();
		let public_dir = root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR);
		let src_dir = root_dir.join("src");
		fs::create_dir_all(&public_dir).unwrap();
		fs::create_dir_all(&src_dir).unwrap();
		fs::write(
			src_dir.join("critical.css"),
			r#".hero { background: url("./logo.svg"); }"#,
		)
		.unwrap();
		let plan = plan(&root_dir);
		let prepared = prepare_public_static_outputs(&plan).unwrap();

		let error = bundle_critical_css(&plan, prepared.public_filemap()).unwrap_err();

		assert!(matches!(
			error,
			PublicStaticOutputError::InvalidCriticalCssUrl { .. }
		));
		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn hashed_output_names_strip_one_suffix_and_keep_dotfile_and_trailing_dot_exts() {
		let source_dir = crate::test_support::unique_temp_root("vorma-output-name");
		std::fs::create_dir_all(&source_dir).unwrap();
		for (file_name, content) in [("archive.tar.tar", "a"), (".env", "b"), ("name.", "c")] {
			std::fs::write(source_dir.join(file_name), content).unwrap();
		}

		let double_suffix =
			hashed_output_name("archive.tar.tar", &source_dir.join("archive.tar.tar")).unwrap();
		let dotfile = hashed_output_name(".env", &source_dir.join(".env")).unwrap();
		let trailing_dot = hashed_output_name("name.", &source_dir.join("name.")).unwrap();

		assert!(double_suffix.starts_with(&format!("{PUBLIC_STATIC_OUT_NAME_PREFIX}archive.tar_")));
		assert!(double_suffix.ends_with(".tar"));
		assert!(dotfile.starts_with(&format!("{PUBLIC_STATIC_OUT_NAME_PREFIX}_")));
		assert!(dotfile.ends_with(".env"));
		assert!(trailing_dot.starts_with(&format!("{PUBLIC_STATIC_OUT_NAME_PREFIX}name_")));
		assert!(trailing_dot.ends_with('.'));
		std::fs::remove_dir_all(source_dir).unwrap();
	}

	#[test]
	fn public_static_outputs_reject_output_directory_inside_source_directory() {
		let root_dir = temp_root();
		fs::create_dir_all(root_dir.join(TEST_PUBLIC_STATIC_SOURCE_DIR)).unwrap();
		let mut declarations = FrameworkDeclarations::new(
			FrameworkConfig::new(TEST_PUBLIC_STATIC_BASE).with_build_inputs(BuildInputConfig::new(
				ServerBuildTarget::new(TEST_CARGO_PACKAGE, TEST_CARGO_BIN),
				root_dir.display().to_string(),
				TEST_PUBLIC_STATIC_SOURCE_DIR,
				FrontendBuildInputs::new(
					"react",
					"pnpm",
					".",
					"vite.config.ts",
					"src/entry.tsx",
					TEST_PUBLIC_STATIC_SOURCE_DIR,
					"src/critical.css",
				),
				"src/vorma.gen.ts",
				DevWatchConfig::default(),
			)),
		);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		let bundle = ProjectionBundle::compile(&FrameworkGraph::compile(declarations).unwrap());
		let plan = BuildProjectionPlan::compile(&bundle).unwrap();

		let error = prepare_public_static_outputs(&plan).unwrap_err();

		assert!(matches!(
			error,
			PublicStaticOutputError::OutputDirInsideSourceDir { .. }
		));
		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn hash_file_caches_content_hashes_by_path_mod_time_and_size() {
		let root_dir = unique_temp_root("vorma-static-hash-cache");
		fs::create_dir_all(&root_dir).unwrap();
		let file_path = root_dir.join("asset.txt");

		fs::write(&file_path, "one").unwrap();
		let first = hash_file(&file_path).unwrap();
		let stat = fs::metadata(&file_path).unwrap().modified().unwrap();

		// Same stat key (path, mod_time, size) serves the cached hash without
		// re-reading the changed bytes.
		fs::write(&file_path, "owt").unwrap();
		let file = fs::File::options().write(true).open(&file_path).unwrap();
		file.set_modified(stat).unwrap();
		drop(file);
		assert_eq!(hash_file(&file_path).unwrap(), first);

		// A changed mod_time invalidates the entry and re-hashes the content.
		let file = fs::File::options().write(true).open(&file_path).unwrap();
		file.set_modified(stat + std::time::Duration::from_secs(1))
			.unwrap();
		drop(file);
		assert_ne!(hash_file(&file_path).unwrap(), first);

		// A changed size invalidates the entry even with an unchanged mod_time.
		fs::write(&file_path, "three").unwrap();
		let file = fs::File::options().write(true).open(&file_path).unwrap();
		file.set_modified(stat).unwrap();
		drop(file);
		assert_ne!(hash_file(&file_path).unwrap(), first);

		fs::remove_dir_all(root_dir).unwrap();
	}
}
