//! Generation filesystem output writer.

use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use vorma::build_interface::assets::{
	ManifestMode, VORMA_OUT_DIR, static_out_dir as runtime_static_out_dir,
};

use crate::build_plan::BuildProjectionPlan;
use crate::generation_epoch::GenerationCandidate;
use crate::typescript_contracts::GeneratedTypeScriptContracts;

const TEMP_FILE_INFIX: &str = ".tmp.";

/// Self-ignore file name written inside the Vorma output directory.
pub(crate) const VORMA_OUTPUT_GITIGNORE_FILE_NAME: &str = ".gitignore";
/// Self-ignore contents covering every generated file below the Vorma output directory.
pub(crate) const VORMA_OUTPUT_GITIGNORE_CONTENT: &str = "*\n";

/// Report returned after writing candidate generation outputs.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildOutputWriteReport {
	manifest_path: PathBuf,
	generated_typescript_path: PathBuf,
	manifest_bytes: usize,
	generated_typescript_bytes: usize,
}

impl BuildOutputWriteReport {
	/// Runtime manifest path written by this operation.
	#[cfg(test)]
	pub fn manifest_path(&self) -> &Path {
		&self.manifest_path
	}

	/// Generated TypeScript contract path written by this operation.
	#[cfg(test)]
	pub fn generated_typescript_path(&self) -> &Path {
		&self.generated_typescript_path
	}

	/// Manifest byte count.
	#[cfg(test)]
	pub fn manifest_bytes(&self) -> usize {
		self.manifest_bytes
	}

	/// Generated TypeScript byte count.
	#[cfg(test)]
	pub fn generated_typescript_bytes(&self) -> usize {
		self.generated_typescript_bytes
	}
}

/// Report returned after writing generated TypeScript contracts.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TypeScriptOutputWriteReport {
	path: PathBuf,
	bytes: usize,
}

impl TypeScriptOutputWriteReport {
	/// Generated TypeScript contract output path.
	#[cfg(test)]
	pub fn path(&self) -> &Path {
		&self.path
	}

	/// Generated TypeScript byte count.
	#[cfg(test)]
	pub fn bytes(&self) -> usize {
		self.bytes
	}
}

/// Write generated TypeScript contracts before the frontend build imports them.
pub fn write_typescript_contracts(
	plan: &BuildProjectionPlan,
	contracts: &GeneratedTypeScriptContracts,
) -> Result<TypeScriptOutputWriteReport, BuildOutputWriteError> {
	let path = generated_typescript_output_path(plan)?;
	let bytes = write_typescript_contracts_to_path(&path, contracts)?;
	Ok(TypeScriptOutputWriteReport { path, bytes })
}

/// Write candidate manifest and generated TypeScript outputs.
pub fn write_generation_candidate_outputs(
	candidate: &GenerationCandidate,
	mode: ManifestMode,
) -> Result<BuildOutputWriteReport, BuildOutputWriteError> {
	let output_paths = generation_candidate_output_paths(candidate, mode)?;
	let manifest_bytes = candidate.manifest().to_json_vec().map_err(|source| {
		BuildOutputWriteError::EncodeManifest {
			message: source.to_string(),
		}
	})?;
	let typescript_bytes = candidate
		.typescript_contracts()
		.source()
		.as_bytes()
		.to_vec();
	write_file_atomically(&output_paths.generated_typescript_path, &typescript_bytes)?;
	write_file_atomically(&output_paths.manifest_path, &manifest_bytes)?;
	write_file_atomically(
		&output_paths.vorma_output_gitignore_path,
		VORMA_OUTPUT_GITIGNORE_CONTENT.as_bytes(),
	)?;
	Ok(BuildOutputWriteReport {
		manifest_path: output_paths.manifest_path,
		generated_typescript_path: output_paths.generated_typescript_path,
		manifest_bytes: manifest_bytes.len(),
		generated_typescript_bytes: typescript_bytes.len(),
	})
}

/// Generation output paths.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GenerationOutputPaths {
	manifest_path: PathBuf,
	generated_typescript_path: PathBuf,
	vorma_output_gitignore_path: PathBuf,
}

impl GenerationOutputPaths {
	/// Runtime manifest output path.
	pub fn manifest_path(&self) -> &Path {
		&self.manifest_path
	}

	/// Generated TypeScript output path.
	pub fn generated_typescript_path(&self) -> &Path {
		&self.generated_typescript_path
	}
}

/// Compute generation candidate output paths without writing files.
pub fn generation_candidate_output_paths(
	candidate: &GenerationCandidate,
	mode: ManifestMode,
) -> Result<GenerationOutputPaths, BuildOutputWriteError> {
	let workspace = candidate.build_plan().workspace();
	let root_dir = PathBuf::from(workspace.root_dir());
	if root_dir.as_os_str().is_empty() {
		return Err(BuildOutputWriteError::EmptyRootDir);
	}
	let dist_dir = resolve_workspace_path(&root_dir, workspace.dist_dir());
	let manifest_path = runtime_static_out_dir(&dist_dir).join(mode.manifest_filename());
	let generated_typescript_path = generated_typescript_output_path(candidate.build_plan())?;
	let vorma_output_gitignore_path = dist_dir
		.join(VORMA_OUT_DIR)
		.join(VORMA_OUTPUT_GITIGNORE_FILE_NAME);
	Ok(GenerationOutputPaths {
		manifest_path,
		generated_typescript_path,
		vorma_output_gitignore_path,
	})
}

/// Build output write error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BuildOutputWriteError {
	/// Build root directory was empty.
	EmptyRootDir,
	/// Runtime manifest could not be encoded.
	EncodeManifest {
		/// JSON encoding error message.
		message: String,
	},
	/// Output file parent directory could not be created.
	CreateParent {
		/// Parent directory path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Temporary output file could not be written.
	WriteTempFile {
		/// Temporary output path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Existing output file could not be read before deciding whether to rewrite it.
	ReadExistingFile {
		/// Existing output path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Temporary output file could not be renamed into place.
	RenameTempFile {
		/// Temporary output path.
		temp_path: String,
		/// Final output path.
		final_path: String,
		/// Filesystem error message.
		message: String,
	},
}

impl std::fmt::Display for BuildOutputWriteError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::EncodeManifest { message } => write!(f, "encode runtime manifest: {message}"),
			Self::CreateParent { path, message } => {
				write!(f, "create output parent {path}: {message}")
			}
			Self::WriteTempFile { path, message } => {
				write!(f, "write temporary output {path}: {message}")
			}
			Self::ReadExistingFile { path, message } => {
				write!(f, "read existing output {path}: {message}")
			}
			Self::RenameTempFile {
				temp_path,
				final_path,
				message,
			} => write!(
				f,
				"rename temporary output {temp_path} to {final_path}: {message}"
			),
		}
	}
}

impl std::error::Error for BuildOutputWriteError {}

pub(crate) fn resolve_workspace_path(root_dir: &Path, path: &str) -> PathBuf {
	let path = PathBuf::from(path);
	if path.is_absolute() {
		return path;
	}
	root_dir.join(path)
}

pub(crate) fn generated_typescript_output_path(
	plan: &BuildProjectionPlan,
) -> Result<PathBuf, BuildOutputWriteError> {
	let root_dir = PathBuf::from(plan.workspace().root_dir());
	if root_dir.as_os_str().is_empty() {
		return Err(BuildOutputWriteError::EmptyRootDir);
	}
	Ok(resolve_workspace_path(
		&root_dir,
		plan.generated_typescript().output_file(),
	))
}

fn write_typescript_contracts_to_path(
	path: &Path,
	contracts: &GeneratedTypeScriptContracts,
) -> Result<usize, BuildOutputWriteError> {
	let bytes = contracts.source().as_bytes();
	write_file_atomically(path, bytes)?;
	Ok(bytes.len())
}

pub(crate) fn write_file_atomically(
	path: &Path,
	bytes: &[u8],
) -> Result<(), BuildOutputWriteError> {
	match std::fs::read(path) {
		Ok(existing) if existing == bytes => return Ok(()),
		Ok(_) => {}
		Err(source) if source.kind() == std::io::ErrorKind::NotFound => {}
		Err(source) => {
			return Err(BuildOutputWriteError::ReadExistingFile {
				path: path.display().to_string(),
				message: source.to_string(),
			});
		}
	}
	let parent = path
		.parent()
		.ok_or_else(|| BuildOutputWriteError::CreateParent {
			path: path.display().to_string(),
			message: "output path has no parent directory".to_owned(),
		})?;
	create_dir_all_no_symlinks(parent).map_err(|source| BuildOutputWriteError::CreateParent {
		path: parent.display().to_string(),
		message: source.to_string(),
	})?;
	let temp_path = temp_output_path(path);
	if let Err(source) = std::fs::write(&temp_path, bytes) {
		return Err(BuildOutputWriteError::WriteTempFile {
			path: temp_path.display().to_string(),
			message: source.to_string(),
		});
	}
	if let Err(source) = std::fs::rename(&temp_path, path) {
		let _ = std::fs::remove_file(&temp_path);
		return Err(BuildOutputWriteError::RenameTempFile {
			temp_path: temp_path.display().to_string(),
			final_path: path.display().to_string(),
			message: source.to_string(),
		});
	}
	Ok(())
}

/*
Output directories must never be reached through a symlinked component, so a
generated write cannot be redirected outside the workspace output layout by a
planted link. Verified for every component this call creates or reuses.
*/
pub(crate) fn create_dir_all_no_symlinks(path: &Path) -> std::io::Result<()> {
	match std::fs::symlink_metadata(path) {
		Ok(metadata) if metadata.file_type().is_symlink() => {
			return Err(std::io::Error::new(
				std::io::ErrorKind::InvalidInput,
				format!("output directory cannot be a symlink: {}", path.display()),
			));
		}
		Ok(metadata) if metadata.is_dir() => return Ok(()),
		Ok(_) => {
			return Err(std::io::Error::new(
				std::io::ErrorKind::InvalidInput,
				format!("output path is not a directory: {}", path.display()),
			));
		}
		Err(source) if source.kind() == std::io::ErrorKind::NotFound => {}
		Err(source) => return Err(source),
	}

	if let Some(parent) = path.parent()
		&& !parent.as_os_str().is_empty()
		&& parent != path
	{
		create_dir_all_no_symlinks(parent)?;
	}

	std::fs::create_dir(path)?;
	let metadata = std::fs::symlink_metadata(path)?;
	if metadata.file_type().is_symlink() {
		return Err(std::io::Error::new(
			std::io::ErrorKind::InvalidInput,
			format!("output directory cannot be a symlink: {}", path.display()),
		));
	}
	if !metadata.is_dir() {
		return Err(std::io::Error::new(
			std::io::ErrorKind::InvalidInput,
			format!("output path is not a directory: {}", path.display()),
		));
	}
	Ok(())
}

fn temp_output_path(path: &Path) -> PathBuf {
	let file_name = path
		.file_name()
		.and_then(|value| value.to_str())
		.unwrap_or("output");
	let nanos = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.expect("system time should be after UNIX epoch")
		.as_nanos();
	path.with_file_name(format!("{file_name}{TEMP_FILE_INFIX}{nanos}"))
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;

	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};
	use vorma_contract::runtime_manifest::RuntimeManifest;

	use super::*;
	use crate::generation_epoch::{EpochSupervisor, GenerationCandidate, GenerationError};
	use crate::projection_compiler::{ClientModuleArtifacts, GenerationArtifacts};
	use crate::test_support::route_type_contract;
	use crate::test_support::unique_temp_root;

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_GENERATED_TYPESCRIPT_FILE: &str = "src/vorma.gen.ts";
	const TEST_ROOT_DOCUMENT_HASH_SOURCE: &str = "document-hash-source";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn temp_root() -> PathBuf {
		unique_temp_root("vorma-build-output")
	}

	fn graph(root_dir: &Path) -> FrameworkGraph {
		let mut declarations = FrameworkDeclarations::new(
			FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
				ServerBuildTarget::new(TEST_CARGO_PACKAGE, TEST_CARGO_BIN),
				root_dir.display().to_string(),
				TEST_DIST_DIR,
				FrontendBuildInputs::new(
					"react",
					"pnpm",
					".",
					"vite.config.ts",
					"src/entry.tsx",
					"public",
					"src/critical.css",
				),
				TEST_GENERATED_TYPESCRIPT_FILE,
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
		FrameworkGraph::compile(declarations).unwrap()
	}

	fn artifacts() -> GenerationArtifacts {
		GenerationArtifacts::new(
			"body{}",
			TEST_ROOT_DOCUMENT_HASH_SOURCE,
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			BTreeMap::from([(
				"/".to_owned(),
				ClientModuleArtifacts::new("/static/root.js", Vec::new(), Vec::new()),
			)]),
		)
	}

	fn generation_candidate(root_dir: &Path) -> Result<GenerationCandidate, GenerationError> {
		EpochSupervisor::default().build_next_candidate(graph(root_dir), artifacts())
	}

	#[test]
	fn build_output_writer_materializes_manifest_and_generated_typescript_atomically() {
		let root_dir = temp_root();
		fs::create_dir_all(root_dir.join(TEST_DIST_DIR)).unwrap();
		let candidate = generation_candidate(&root_dir).unwrap();

		let report = write_generation_candidate_outputs(&candidate, ManifestMode::Prod).unwrap();

		let manifest_bytes = fs::read(report.manifest_path()).unwrap();
		let manifest = RuntimeManifest::from_json_slice(&manifest_bytes).unwrap();
		let typescript = fs::read_to_string(report.generated_typescript_path()).unwrap();

		assert_eq!(
			manifest.client_build_id(),
			candidate.manifest().client_build_id()
		);
		assert!(typescript.contains("vormaClientSeed"));
		assert_eq!(report.manifest_bytes(), manifest_bytes.len());
		assert_eq!(report.generated_typescript_bytes(), typescript.len());
		let gitignore = fs::read_to_string(
			root_dir
				.join(TEST_DIST_DIR)
				.join(VORMA_OUT_DIR)
				.join(VORMA_OUTPUT_GITIGNORE_FILE_NAME),
		)
		.unwrap();
		assert_eq!(gitignore, VORMA_OUTPUT_GITIGNORE_CONTENT);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn create_dir_all_no_symlinks_creates_nested_directories_and_accepts_reuse() {
		let root = temp_root();
		let nested = root.join("a/b/c");

		create_dir_all_no_symlinks(&nested).unwrap();
		create_dir_all_no_symlinks(&nested).unwrap();

		assert!(nested.is_dir());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn create_dir_all_no_symlinks_rejects_file_at_path() {
		let root = temp_root();
		fs::create_dir_all(&root).unwrap();
		let file_path = root.join("occupied");
		fs::write(&file_path, b"x").unwrap();

		assert!(create_dir_all_no_symlinks(&file_path).is_err());
		fs::remove_dir_all(root).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn create_dir_all_no_symlinks_rejects_symlink_component() {
		let root = temp_root();
		let outside = root.join("outside");
		fs::create_dir_all(&outside).unwrap();
		let linked = root.join("linked");
		std::os::unix::fs::symlink(&outside, &linked).unwrap();

		assert!(create_dir_all_no_symlinks(&linked).is_err());
		assert!(create_dir_all_no_symlinks(&linked.join("child")).is_err());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn typescript_contract_writer_materializes_prebuild_contracts() {
		let root_dir = temp_root();
		let candidate = generation_candidate(&root_dir).unwrap();

		let report =
			write_typescript_contracts(candidate.build_plan(), candidate.typescript_contracts())
				.unwrap();
		let typescript = fs::read_to_string(report.path()).unwrap();

		assert!(typescript.contains("vormaClientSeed"));
		assert_eq!(report.bytes(), typescript.len());
		assert_eq!(report.path(), root_dir.join(TEST_GENERATED_TYPESCRIPT_FILE));

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn atomic_writer_does_not_replace_byte_identical_existing_file() {
		let root_dir = temp_root();
		let path = root_dir.join("out.txt");
		let linked_path = root_dir.join("linked-out.txt");
		fs::create_dir_all(&root_dir).unwrap();
		fs::write(&path, b"same").unwrap();
		fs::hard_link(&path, &linked_path).unwrap();

		write_file_atomically(&path, b"same").unwrap();

		assert!(same_file::is_same_file(&path, &linked_path).unwrap());
		fs::remove_dir_all(root_dir).unwrap();
	}
}
