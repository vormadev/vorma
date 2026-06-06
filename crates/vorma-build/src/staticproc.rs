use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::hash::{Hash, Hasher};
use std::io::{self, Write};
use std::num::NonZeroUsize;
use std::path::{Path, PathBuf};
use std::sync::{LazyLock, Mutex};
use std::time::SystemTime;

use data_encoding::BASE32_NOPAD;
use lru::LruCache;
use path_slash::PathExt;
use walkdir::WalkDir;

use crate::utils::create_dir_all_no_symlinks;

#[derive(Clone, Debug, Eq)]
struct HashCacheKey {
	src_path: PathBuf,
	mod_time: SystemTime,
	size: u64,
}

impl PartialEq for HashCacheKey {
	fn eq(&self, other: &Self) -> bool {
		self.src_path == other.src_path
			&& self.mod_time == other.mod_time
			&& self.size == other.size
	}
}

impl Hash for HashCacheKey {
	fn hash<H: Hasher>(&self, state: &mut H) {
		self.src_path.hash(state);
		self.mod_time.hash(state);
		self.size.hash(state);
	}
}

static HASH_CACHE: LazyLock<Mutex<LruCache<HashCacheKey, [u8; 32]>>> = LazyLock::new(|| {
	Mutex::new(LruCache::new(
		NonZeroUsize::new(10_000).expect("staticproc hash cache capacity should be nonzero"),
	))
});

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct Files {
	files: BTreeMap<String, File>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct File {
	pub src_dir: PathBuf,
	pub src_path_rel: String,
	pub bytes: Option<Vec<u8>>,
	pub out_name_prefix: String,
	pub mod_time: Option<SystemTime>,
	pub out_name: String,
	pub size: u64,
}

impl Files {
	pub fn new() -> Self {
		Self::default()
	}

	#[cfg(test)]
	fn map(&self) -> BTreeMap<String, String> {
		self.files
			.iter()
			.map(|(rel, file)| (rel.clone(), file.out_name.clone()))
			.collect()
	}

	pub fn iter(&self) -> impl Iterator<Item = (&String, &File)> {
		self.files.iter()
	}

	fn insert(&mut self, file: File) {
		self.files.insert(file.src_path_rel.clone(), file);
	}
}

pub fn collect_physical(src_dir: impl AsRef<Path>, out_name_prefix: &str) -> io::Result<Files> {
	let src_dir = src_dir.as_ref();
	let mut files = Files::new();

	for entry in WalkDir::new(src_dir) {
		let entry = entry?;
		if entry.file_type().is_dir() {
			continue;
		}
		if entry.file_type().is_symlink() {
			return Err(io::Error::new(
				io::ErrorKind::InvalidInput,
				format!(
					"public static file cannot be a symlink: {}",
					entry.path().display()
				),
			));
		}
		if !entry.file_type().is_file() {
			continue;
		}
		if entry.file_name() == ".DS_Store" {
			continue;
		}

		let rel = entry
			.path()
			.strip_prefix(src_dir)
			.map_err(io::Error::other)?
			.to_slash_lossy()
			.into_owned();

		let mut file = File {
			src_dir: src_dir.to_path_buf(),
			src_path_rel: rel,
			bytes: None,
			out_name_prefix: out_name_prefix.to_owned(),
			mod_time: None,
			out_name: String::new(),
			size: 0,
		};

		file.hash()?;
		files.insert(file);
	}

	Ok(files)
}

pub fn reconcile(out_dir: impl AsRef<Path>, files: &Files) -> io::Result<()> {
	let out_dir = out_dir.as_ref();
	create_dir_all_no_symlinks(out_dir)?;
	let mut target_paths = BTreeSet::new();
	let mut to_copy = Vec::new();

	for (_, file) in files.iter() {
		let out_path = out_dir.join(&file.out_name);
		target_paths.insert(out_path.clone());
		if remove_symlink_parent(out_dir, &out_path)? {
			to_copy.push(file);
			continue;
		}
		match fs::symlink_metadata(&out_path) {
			Ok(metadata) if metadata.file_type().is_symlink() => {
				fs::remove_file(&out_path)?;
				to_copy.push(file);
			}
			Ok(metadata) if metadata.is_file() => {}
			Ok(metadata) if metadata.is_dir() => {
				fs::remove_dir_all(&out_path)?;
				to_copy.push(file);
			}
			Ok(_) => {
				fs::remove_file(&out_path)?;
				to_copy.push(file);
			}
			Err(err) if err.kind() == io::ErrorKind::NotFound => to_copy.push(file),
			Err(err) => return Err(err),
		}
	}

	for file in to_copy {
		let out_path = out_dir.join(&file.out_name);
		if let Some(parent) = out_path.parent() {
			create_dir_all_no_symlinks(parent)?;
		}
		if let Some(bytes) = &file.bytes {
			write_file(&out_path, bytes)?;
		} else {
			copy_file(file.src_dir.join(&file.src_path_rel), out_path)?;
		}
	}

	if out_dir.exists() {
		for entry in WalkDir::new(out_dir) {
			let entry = entry?;
			if entry.file_type().is_dir() {
				continue;
			}
			let path = entry.path();
			if !target_paths.contains(path) {
				fs::remove_file(path)?;
			}
		}
	}

	Ok(())
}

fn remove_symlink_parent(root: &Path, path: &Path) -> io::Result<bool> {
	let rel = path.strip_prefix(root).unwrap_or(path);
	let mut current = root.to_path_buf();
	let mut components = rel.components().peekable();
	while let Some(component) = components.next() {
		if components.peek().is_none() {
			return Ok(false);
		}
		current.push(component.as_os_str());
		match fs::symlink_metadata(&current) {
			Ok(metadata) if metadata.file_type().is_symlink() => {
				fs::remove_file(&current)?;
				return Ok(true);
			}
			Ok(metadata) if metadata.is_dir() => {}
			Ok(_) => return Ok(false),
			Err(err) if err.kind() == io::ErrorKind::NotFound => return Ok(false),
			Err(err) => return Err(err),
		}
	}
	Ok(false)
}

fn copy_file(src: impl AsRef<Path>, dest: impl AsRef<Path>) -> io::Result<()> {
	let src = src.as_ref();
	let dest = dest.as_ref();
	let src_meta = fs::symlink_metadata(src)?;
	if src_meta.file_type().is_symlink() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!("source file cannot be a symlink: {}", src.display()),
		));
	}
	if !src_meta.is_file() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!("source path is not a file: {}", src.display()),
		));
	}

	match fs::symlink_metadata(dest) {
		Ok(dest_meta) => {
			if dest_meta.file_type().is_symlink() {
				return Err(io::Error::new(
					io::ErrorKind::InvalidInput,
					format!("destination file cannot be a symlink: {}", dest.display()),
				));
			}
			if same_file::is_same_file(src, dest)? {
				return Err(io::Error::other(format!(
					"source and destination refer to the same file: {}",
					src.display()
				)));
			}
			return Err(io::Error::new(
				io::ErrorKind::AlreadyExists,
				format!("destination file already exists: {}", dest.display()),
			));
		}
		Err(err) if err.kind() == io::ErrorKind::NotFound => {}
		Err(err) => return Err(err),
	}

	if let Some(parent) = dest.parent() {
		create_dir_all_no_symlinks(parent)?;
	}

	let copy_result = (|| -> io::Result<()> {
		let mut source = fs::File::open(src)?;
		let mut target = fs::OpenOptions::new()
			.write(true)
			.create_new(true)
			.open(dest)?;
		io::copy(&mut source, &mut target)?;
		target.sync_all()?;
		Ok(())
	})();
	if let Err(err) = copy_result {
		let _ = fs::remove_file(dest);
		return Err(err);
	}

	Ok(())
}

fn write_file(path: impl AsRef<Path>, bytes: &[u8]) -> io::Result<()> {
	let mut file = fs::OpenOptions::new()
		.write(true)
		.create_new(true)
		.open(path)?;
	file.write_all(bytes)?;
	file.sync_all()
}

impl File {
	pub fn hash(&mut self) -> io::Result<()> {
		if self.src_path_rel.is_empty() {
			return Err(io::Error::new(
				io::ErrorKind::InvalidInput,
				"empty src_path_rel",
			));
		}

		let is_physical = self.bytes.is_none();
		let mut needs_hash = self.out_name.is_empty() || !is_physical;
		let mut src_path = PathBuf::new();

		if is_physical {
			src_path = self.src_dir.join(&self.src_path_rel);
			let info = fs::symlink_metadata(&src_path)?;
			if info.file_type().is_symlink() {
				return Err(io::Error::new(
					io::ErrorKind::InvalidInput,
					format!("source file cannot be a symlink: {}", src_path.display()),
				));
			}
			if !info.is_file() {
				return Err(io::Error::new(
					io::ErrorKind::InvalidInput,
					format!("source path is not a file: {}", src_path.display()),
				));
			}
			let mod_time = info.modified()?;
			if self.mod_time != Some(mod_time) {
				needs_hash = true;
				self.mod_time = Some(mod_time);
			}
			if self.size != info.len() {
				needs_hash = true;
				self.size = info.len();
			}
		}

		if !needs_hash {
			return Ok(());
		}

		let mut hasher = blake3::Hasher::new();
		hasher.update(self.src_path_rel.as_bytes());
		hasher.update(&[0]);

		if is_physical {
			let content_hash = hash_physical(HashCacheKey {
				src_path,
				mod_time: self
					.mod_time
					.expect("physical static file should have mod_time before hashing"),
				size: self.size,
			})?;
			hasher.update(&content_hash);
		} else if let Some(bytes) = &self.bytes {
			hasher.update(blake3::hash(bytes).as_bytes());
		}

		self.out_name = self.out_name_prefix.clone();
		let ext = path_ext(&self.src_path_rel);
		let stem = if ext.is_empty() {
			self.src_path_rel.as_str()
		} else {
			self.src_path_rel
				.strip_suffix(&ext)
				.unwrap_or(&self.src_path_rel)
		};
		let flat = stem.replace('/', "_");
		let mut trunc_hash = BASE32_NOPAD.encode(hasher.finalize().as_bytes());
		trunc_hash.make_ascii_lowercase();
		self.out_name
			.push_str(&format!("{flat}_{}{ext}", &trunc_hash[..12]));

		Ok(())
	}
}

fn hash_physical(cache_key: HashCacheKey) -> io::Result<[u8; 32]> {
	if let Some(hash) = HASH_CACHE
		.lock()
		.expect("staticproc hash cache should not be poisoned")
		.get(&cache_key)
		.copied()
	{
		return Ok(hash);
	}

	let mut file = fs::File::open(&cache_key.src_path)?;
	let mut hasher = blake3::Hasher::new();
	hasher.update_reader(&mut file)?;
	let hash: [u8; 32] = hasher.finalize().into();

	HASH_CACHE
		.lock()
		.expect("staticproc hash cache should not be poisoned")
		.put(cache_key, hash);

	Ok(hash)
}

fn path_ext(path: &str) -> &str {
	let last_segment = path.rsplit('/').next().unwrap_or(path);
	let Some(dot_idx) = last_segment.rfind('.') else {
		return "";
	};
	&path[path.len() - last_segment.len() + dot_idx..]
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn collect_physical_hashes_and_maps_files() {
		let fixture = test_dir("collect_physical_hashes_and_maps_files");
		let src_dir = fixture.join("public");
		fs::create_dir_all(src_dir.join("img")).unwrap();
		fs::write(src_dir.join("app.css"), "body{}").unwrap();
		fs::write(src_dir.join("img/logo.svg"), "<svg></svg>").unwrap();
		fs::write(src_dir.join(".DS_Store"), "ignored").unwrap();

		let files = collect_physical(&src_dir, "vorma_out_").unwrap();
		let filemap = files.map();

		assert_eq!(filemap.len(), 2);
		assert!(filemap["app.css"].starts_with("vorma_out_app_"));
		assert!(filemap["app.css"].ends_with(".css"));
		assert!(filemap["img/logo.svg"].starts_with("vorma_out_img_logo_"));
		assert!(filemap["img/logo.svg"].ends_with(".svg"));
	}

	#[test]
	fn prehashed_directory_is_ordinary_hashed_public_input() {
		let fixture = test_dir("prehashed_directory_is_ordinary_hashed_public_input");
		let src_dir = fixture.join("public");
		fs::create_dir_all(src_dir.join("__prehashed/vendor")).unwrap();
		fs::write(src_dir.join("__prehashed/already.abc123.js"), "ok").unwrap();
		fs::write(src_dir.join("__prehashed/vendor/chunk.def456.css"), "ok").unwrap();

		let files = collect_physical(&src_dir, "vorma_out_").unwrap();
		let filemap = files.map();

		assert!(
			filemap["__prehashed/already.abc123.js"]
				.starts_with("vorma_out___prehashed_already.abc123_")
		);
		assert!(filemap["__prehashed/already.abc123.js"].ends_with(".js"));
		assert!(
			filemap["__prehashed/vendor/chunk.def456.css"]
				.starts_with("vorma_out___prehashed_vendor_chunk.def456_")
		);
		assert!(filemap["__prehashed/vendor/chunk.def456.css"].ends_with(".css"));
	}

	#[test]
	fn reconcile_copies_current_files_and_deletes_stale_files() {
		let fixture = test_dir("reconcile_copies_current_files_and_deletes_stale_files");
		let src_dir = fixture.join("public");
		let out_dir = fixture.join("out");
		fs::create_dir_all(&src_dir).unwrap();
		fs::create_dir_all(&out_dir).unwrap();
		fs::write(src_dir.join("app.css"), "body{}").unwrap();
		fs::write(out_dir.join("stale.txt"), "stale").unwrap();

		let files = collect_physical(&src_dir, "vorma_out_").unwrap();
		reconcile(&out_dir, &files).unwrap();

		assert!(!out_dir.join("stale.txt").exists());
		let out_names = files.map().into_values().collect::<Vec<_>>();
		assert_eq!(out_names.len(), 1);
		assert!(out_dir.join(&out_names[0]).exists());
	}

	#[test]
	fn reconcile_copies_prehashed_directory_files_to_hashed_paths() {
		let fixture = test_dir("reconcile_copies_prehashed_directory_files_to_hashed_paths");
		let src_dir = fixture.join("public");
		let out_dir = fixture.join("out");
		fs::create_dir_all(src_dir.join("__prehashed/assets")).unwrap();
		fs::write(src_dir.join("__prehashed/assets/app.abc123.js"), "app").unwrap();

		let files = collect_physical(&src_dir, "vorma_out_").unwrap();
		reconcile(&out_dir, &files).unwrap();
		let out_name = files.map()["__prehashed/assets/app.abc123.js"].clone();

		assert_eq!(fs::read_to_string(out_dir.join(out_name)).unwrap(), "app");
		assert!(!out_dir.join("__prehashed").exists());
	}

	#[test]
	fn reconcile_preserves_stale_files_when_current_copy_fails() {
		let fixture = test_dir("reconcile_preserves_stale_files_when_current_copy_fails");
		let src_dir = fixture.join("public");
		let out_dir = fixture.join("out");
		fs::create_dir_all(&src_dir).unwrap();
		fs::create_dir_all(&out_dir).unwrap();
		fs::write(out_dir.join("stale.txt"), "stale").unwrap();

		let mut files = Files::new();
		files.insert(File {
			src_dir,
			src_path_rel: "missing.css".to_owned(),
			bytes: None,
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: "vorma_out_missing.css".to_owned(),
			size: 0,
		});

		let error = reconcile(&out_dir, &files).unwrap_err();

		assert_eq!(error.kind(), io::ErrorKind::NotFound);
		assert_eq!(
			fs::read_to_string(out_dir.join("stale.txt")).unwrap(),
			"stale"
		);
	}

	#[test]
	fn hash_physical_cache_hit_does_not_reread_file() {
		let fixture = test_dir("hash_physical_cache_hit_does_not_reread_file");
		let src = fixture.join("asset.txt");
		fs::write(&src, "asset").unwrap();
		let metadata = fs::metadata(&src).unwrap();
		let cache_key = HashCacheKey {
			src_path: src.clone(),
			mod_time: metadata.modified().unwrap(),
			size: metadata.len(),
		};

		let first = hash_physical(cache_key.clone()).unwrap();
		fs::remove_file(&src).unwrap();
		let second = hash_physical(cache_key).unwrap();

		assert_eq!(first, second);
	}

	#[test]
	fn copy_file_rejects_same_file_against_same_file() {
		let fixture = test_dir("copy_file_rejects_same_file_against_same_file");
		let path = fixture.join("asset.txt");
		fs::write(&path, "asset").unwrap();

		let err = copy_file(&path, &path).unwrap_err();

		assert!(err.to_string().contains("same file"));
	}

	#[test]
	fn copy_file_missing_source_preserves_existing_dest() {
		let fixture = test_dir("copy_file_missing_source_preserves_existing_dest");
		let src = fixture.join("missing.txt");
		let dest = fixture.join("asset.txt");
		fs::write(&dest, "old").unwrap();

		copy_file(&src, &dest).unwrap_err();

		assert_eq!(fs::read_to_string(dest).unwrap(), "old");
	}

	#[test]
	fn copy_file_existing_destination_preserves_existing_dest() {
		let fixture = test_dir("copy_file_existing_destination_preserves_existing_dest");
		let src = fixture.join("src.txt");
		let dest = fixture.join("asset.txt");
		fs::write(&src, "new").unwrap();
		fs::write(&dest, "old").unwrap();

		let err = copy_file(&src, &dest).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::AlreadyExists);
		assert_eq!(fs::read_to_string(dest).unwrap(), "old");
	}

	#[cfg(unix)]
	#[test]
	fn collect_physical_rejects_symlink_files() {
		let fixture = test_dir("collect_physical_rejects_symlink_files");
		let src_dir = fixture.join("public");
		let outside = fixture.join("secret.txt");
		fs::create_dir_all(&src_dir).unwrap();
		fs::write(&outside, "secret").unwrap();
		std::os::unix::fs::symlink(&outside, src_dir.join("linked.txt")).unwrap();

		let err = collect_physical(&src_dir, "vorma_out_").unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(err.to_string().contains("symlink"));
	}

	#[cfg(unix)]
	#[test]
	fn physical_file_hash_rejects_symlink_files() {
		let fixture = test_dir("physical_file_hash_rejects_symlink_files");
		let src_dir = fixture.join("public");
		let outside = fixture.join("secret.txt");
		fs::create_dir_all(&src_dir).unwrap();
		fs::write(&outside, "secret").unwrap();
		std::os::unix::fs::symlink(&outside, src_dir.join("linked.txt")).unwrap();
		let mut file = File {
			src_dir,
			src_path_rel: "linked.txt".to_owned(),
			bytes: None,
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: String::new(),
			size: 0,
		};

		let err = file.hash().unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(err.to_string().contains("symlink"));
	}

	#[cfg(unix)]
	#[test]
	fn copy_file_rejects_source_symlink() {
		let fixture = test_dir("copy_file_rejects_source_symlink");
		let source_target = fixture.join("source-target.txt");
		let source_link = fixture.join("source-link.txt");
		let dest = fixture.join("dest.txt");
		fs::write(&source_target, "secret").unwrap();
		std::os::unix::fs::symlink(&source_target, &source_link).unwrap();

		let err = copy_file(&source_link, &dest).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(!dest.exists());
	}

	#[cfg(unix)]
	#[test]
	fn copy_file_rejects_destination_symlink_without_touching_target() {
		let fixture = test_dir("copy_file_rejects_destination_symlink_without_touching_target");
		let src = fixture.join("src.txt");
		let external = fixture.join("external.txt");
		let dest = fixture.join("dest-link.txt");
		fs::write(&src, "new").unwrap();
		fs::write(&external, "old").unwrap();
		std::os::unix::fs::symlink(&external, &dest).unwrap();

		let err = copy_file(&src, &dest).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert_eq!(fs::read_to_string(external).unwrap(), "old");
	}

	#[cfg(unix)]
	#[test]
	fn reconcile_replaces_existing_output_symlink_without_touching_target() {
		let fixture =
			test_dir("reconcile_replaces_existing_output_symlink_without_touching_target");
		let src_dir = fixture.join("public");
		let out_dir = fixture.join("out");
		let external = fixture.join("external.txt");
		fs::create_dir_all(&src_dir).unwrap();
		fs::create_dir_all(&out_dir).unwrap();
		fs::write(src_dir.join("app.css"), "body{}").unwrap();
		fs::write(&external, "old").unwrap();

		let files = collect_physical(&src_dir, "vorma_out_").unwrap();
		let out_name = files.map()["app.css"].clone();
		std::os::unix::fs::symlink(&external, out_dir.join(&out_name)).unwrap();

		reconcile(&out_dir, &files).unwrap();

		assert_eq!(fs::read_to_string(&external).unwrap(), "old");
		assert_eq!(
			fs::read_to_string(out_dir.join(out_name)).unwrap(),
			"body{}"
		);
	}

	#[cfg(unix)]
	#[test]
	fn reconcile_replaces_existing_output_parent_symlink_without_touching_target() {
		let fixture =
			test_dir("reconcile_replaces_existing_output_parent_symlink_without_touching_target");
		let src_dir = fixture.join("public");
		let out_dir = fixture.join("out");
		let external = fixture.join("external");
		fs::create_dir_all(&out_dir).unwrap();
		fs::create_dir_all(external.join("nested")).unwrap();
		fs::write(external.join("nested/app.js"), "external").unwrap();
		let mut files = Files::new();
		files.insert(File {
			src_dir,
			src_path_rel: "unused".to_owned(),
			bytes: Some(b"app".to_vec()),
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: "nested/app.js".to_owned(),
			size: 0,
		});
		let out_name = "nested/app.js";
		let out_parent = Path::new(out_name).parent().unwrap().to_owned();
		std::os::unix::fs::symlink(&external, out_dir.join(&out_parent)).unwrap();

		reconcile(&out_dir, &files).unwrap();

		assert_eq!(
			fs::read_to_string(external.join("nested/app.js")).unwrap(),
			"external"
		);
		assert_eq!(fs::read_to_string(out_dir.join(out_name)).unwrap(), "app");
		assert!(
			!fs::symlink_metadata(out_dir.join(out_parent))
				.unwrap()
				.file_type()
				.is_symlink()
		);
	}

	#[test]
	fn output_name_strips_only_one_extension_suffix() {
		let mut file = File {
			src_dir: PathBuf::new(),
			src_path_rel: "archive.tar.tar".to_owned(),
			bytes: Some(b"content".to_vec()),
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: String::new(),
			size: 0,
		};

		file.hash().unwrap();

		assert!(file.out_name.starts_with("vorma_out_archive.tar_"));
		assert!(file.out_name.ends_with(".tar"));
	}

	#[test]
	fn output_name_uses_path_ext_for_dotfiles() {
		let mut file = File {
			src_dir: PathBuf::new(),
			src_path_rel: ".env".to_owned(),
			bytes: Some(b"content".to_vec()),
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: String::new(),
			size: 0,
		};

		file.hash().unwrap();

		assert!(file.out_name.starts_with("vorma_out__"));
		assert!(file.out_name.ends_with(".env"));
	}

	#[test]
	fn output_name_uses_path_ext_for_trailing_dot() {
		let mut file = File {
			src_dir: PathBuf::new(),
			src_path_rel: "name.".to_owned(),
			bytes: Some(b"content".to_vec()),
			out_name_prefix: "vorma_out_".to_owned(),
			mod_time: None,
			out_name: String::new(),
			size: 0,
		};

		file.hash().unwrap();

		assert!(file.out_name.starts_with("vorma_out_name_"));
		assert!(file.out_name.ends_with('.'));
	}

	fn test_dir(name: &str) -> PathBuf {
		let path =
			std::env::temp_dir().join(format!("vorma_staticproc_{name}_{}", std::process::id()));
		let _ = fs::remove_dir_all(&path);
		fs::create_dir_all(&path).unwrap();
		path
	}
}
