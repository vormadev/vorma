//! Exclusive per-workspace output-layout lock for build-entry processes.
/*
Every build-entry flow that mutates a workspace's generated output layout
(generated TypeScript, manifest, static outputs) must hold this lock for the
full duration of its writes: production builds for the build's lifetime, dev
servers for the process lifetime. The lock file lives inside the Vorma-owned
output directory under the dist dir, so any two processes pointed at the same
dist contend on the same file regardless of mode.

The dev server's live-state child process must never acquire this lock: the
parent dev server already holds it for the same layout, so acquiring from the
child would deadlock against its own parent. The live-state mode early-exits
before any generation work, which preserves that property by construction.
*/

use std::path::{Path, PathBuf};

use paranoid::local_lock::ProcessLock;
use vorma::build_interface::assets::VORMA_OUT_DIR;

use crate::build_output::{create_dir_all_no_symlinks, resolve_workspace_path};

/// Lock file name below the Vorma-owned output directory.
pub const OUTPUT_LAYOUT_LOCK_FILE_NAME: &str = "build.lock";

/// Held exclusive lock over one workspace output layout.
pub struct OutputLayoutLock {
	lock: ProcessLock,
	lock_path: PathBuf,
}

impl OutputLayoutLock {
	/// Acquire the output-layout lock for the given workspace root and dist dir.
	pub fn acquire_for_workspace_output_layout(
		root_dir: &str,
		dist_dir: &str,
	) -> Result<Self, OutputLayoutLockError> {
		let lock_path = output_layout_lock_path(root_dir, dist_dir)?;
		let lock = acquire_process_lock(&lock_path)?;
		Ok(Self { lock, lock_path })
	}

	/// Acquired lock file path.
	#[cfg(test)]
	pub fn lock_path(&self) -> &Path {
		&self.lock_path
	}

	/// Move the lock to a new output layout, acquiring the replacement before
	/// releasing the previous lock so no unlocked window exists. Returns the
	/// previous Vorma output directory when the layout moved.
	/*
	The previous layout must NOT be deleted here: the currently committed
	generation keeps serving from it until the new generation activates, and
	a failed activation rolls back onto it. Callers delete the orphan only
	after the moved-layout generation commits.
	*/
	pub fn reacquire_if_output_layout_moved(
		&mut self,
		root_dir: &str,
		dist_dir: &str,
	) -> Result<Option<PathBuf>, OutputLayoutLockError> {
		let next_lock_path = output_layout_lock_path(root_dir, dist_dir)?;
		if next_lock_path == self.lock_path {
			return Ok(None);
		}
		let next_lock = acquire_process_lock(&next_lock_path)?;
		let mut previous_lock = std::mem::replace(&mut self.lock, next_lock);
		let previous_lock_path = std::mem::replace(&mut self.lock_path, next_lock_path);
		previous_lock
			.release()
			.map_err(|source| OutputLayoutLockError::Release {
				path: previous_lock_path.display().to_string(),
				message: source.to_string(),
			})?;
		Ok(previous_lock_path.parent().map(Path::to_path_buf))
	}

	/// Release the lock, surfacing release failures.
	pub fn release(mut self) -> Result<(), OutputLayoutLockError> {
		self.lock
			.release()
			.map_err(|source| OutputLayoutLockError::Release {
				path: self.lock_path.display().to_string(),
				message: source.to_string(),
			})
	}
}

fn output_layout_lock_path(
	root_dir: &str,
	dist_dir: &str,
) -> Result<PathBuf, OutputLayoutLockError> {
	if root_dir.trim().is_empty() {
		return Err(OutputLayoutLockError::EmptyRootDir);
	}
	let dist_dir = resolve_workspace_path(Path::new(root_dir), dist_dir);
	Ok(dist_dir
		.join(VORMA_OUT_DIR)
		.join(OUTPUT_LAYOUT_LOCK_FILE_NAME))
}

fn acquire_process_lock(lock_path: &Path) -> Result<ProcessLock, OutputLayoutLockError> {
	let lock_dir = lock_path
		.parent()
		.expect("output layout lock path always has a parent directory");
	create_dir_all_no_symlinks(lock_dir).map_err(|source| {
		OutputLayoutLockError::CreateLockDirectory {
			path: lock_dir.display().to_string(),
			message: source.to_string(),
		}
	})?;
	let mut lock = ProcessLock::new(lock_path.to_path_buf());
	lock.acquire()
		.map_err(|source| OutputLayoutLockError::Acquire {
			path: lock_path.display().to_string(),
			message: source.to_string(),
		})?;
	Ok(lock)
}

/// Output-layout lock error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum OutputLayoutLockError {
	/// Workspace root dir was empty.
	EmptyRootDir,
	/// Lock directory creation failed.
	CreateLockDirectory {
		/// Lock directory path.
		path: String,
		/// Error message.
		message: String,
	},
	/// Lock acquisition failed; usually another build entry holds the layout.
	Acquire {
		/// Lock file path.
		path: String,
		/// Error message.
		message: String,
	},
	/// Lock release failed.
	Release {
		/// Lock file path.
		path: String,
		/// Error message.
		message: String,
	},
}

impl std::fmt::Display for OutputLayoutLockError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("workspace root dir cannot be empty"),
			Self::CreateLockDirectory { path, message } => {
				write!(f, "create output layout lock directory {path}: {message}")
			}
			Self::Acquire { path, message } => {
				write!(f, "acquire output layout lock {path}: {message}")
			}
			Self::Release { path, message } => {
				write!(f, "release output layout lock {path}: {message}")
			}
		}
	}
}

impl std::error::Error for OutputLayoutLockError {}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::test_support::unique_temp_root;

	const TEST_DIST_DIR: &str = "dist";

	fn temp_workspace_root() -> (PathBuf, String) {
		let root = unique_temp_root("vorma-output-lock");
		std::fs::create_dir_all(&root).unwrap();
		let root_string = root.to_string_lossy().into_owned();
		(root, root_string)
	}

	#[test]
	fn acquire_blocks_second_acquirer_until_release() {
		let (root, root_string) = temp_workspace_root();

		let held =
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR)
				.unwrap();
		assert!(
			held.lock_path().ends_with(
				Path::new(TEST_DIST_DIR)
					.join(VORMA_OUT_DIR)
					.join(OUTPUT_LAYOUT_LOCK_FILE_NAME)
			)
		);
		assert!(matches!(
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR),
			Err(OutputLayoutLockError::Acquire { .. })
		));

		held.release().unwrap();
		let reacquired =
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR)
				.unwrap();
		reacquired.release().unwrap();
		std::fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn acquire_rejects_empty_root_dir() {
		assert_eq!(
			OutputLayoutLock::acquire_for_workspace_output_layout("   ", TEST_DIST_DIR)
				.map(|_| ())
				.unwrap_err(),
			OutputLayoutLockError::EmptyRootDir
		);
	}

	#[test]
	fn reacquire_is_noop_for_unmoved_layout() {
		let (root, root_string) = temp_workspace_root();

		let mut held =
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR)
				.unwrap();
		let original_lock_path = held.lock_path().to_path_buf();
		held.reacquire_if_output_layout_moved(&root_string, TEST_DIST_DIR)
			.unwrap();
		assert_eq!(held.lock_path(), original_lock_path);
		assert!(matches!(
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR),
			Err(OutputLayoutLockError::Acquire { .. })
		));

		held.release().unwrap();
		std::fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn reacquire_swaps_to_moved_layout_and_frees_previous() {
		let (root, root_string) = temp_workspace_root();
		const NEXT_DIST_DIR: &str = "dist-next";

		let mut held =
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR)
				.unwrap();
		held.reacquire_if_output_layout_moved(&root_string, NEXT_DIST_DIR)
			.unwrap();
		assert!(
			held.lock_path().ends_with(
				Path::new(NEXT_DIST_DIR)
					.join(VORMA_OUT_DIR)
					.join(OUTPUT_LAYOUT_LOCK_FILE_NAME)
			)
		);

		let previous_layout =
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, TEST_DIST_DIR)
				.unwrap();
		assert!(matches!(
			OutputLayoutLock::acquire_for_workspace_output_layout(&root_string, NEXT_DIST_DIR),
			Err(OutputLayoutLockError::Acquire { .. })
		));

		previous_layout.release().unwrap();
		held.release().unwrap();
		std::fs::remove_dir_all(root).unwrap();
	}
}
