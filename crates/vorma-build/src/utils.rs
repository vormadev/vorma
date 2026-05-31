use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::Serialize;

pub(crate) fn serialize_pretty_json<T: Serialize>(data: &T) -> Result<Vec<u8>, String> {
	let mut out = Vec::new();
	{
		let formatter = serde_json::ser::PrettyFormatter::with_indent(b"\t");
		let mut serializer = serde_json::Serializer::with_formatter(&mut out, formatter);
		data.serialize(&mut serializer)
			.map_err(|err| format!("error serializing data: {err}"))?;
	}
	Ok(out)
}

pub(crate) fn write_str_to_file(s: &str, out_path: impl AsRef<Path>) -> io::Result<()> {
	write_bytes_to_file(s.as_bytes(), out_path)
}

pub(crate) fn write_json_to_file<T: Serialize>(
	data: T,
	out_path: impl AsRef<Path>,
) -> Result<(), String> {
	let mut out = serialize_pretty_json(&data)?;
	out.push(b'\n');

	write_bytes_to_file(&out, out_path).map_err(|err| format!("error writing data to file: {err}"))
}

pub(crate) fn create_dir_all_no_symlinks(path: impl AsRef<Path>) -> io::Result<()> {
	let path = path.as_ref();

	match fs::symlink_metadata(path) {
		Ok(metadata) if metadata.file_type().is_symlink() => {
			return Err(io::Error::new(
				io::ErrorKind::InvalidInput,
				format!("output directory cannot be a symlink: {}", path.display()),
			));
		}
		Ok(metadata) if metadata.is_dir() => return Ok(()),
		Ok(_) => {
			return Err(io::Error::new(
				io::ErrorKind::InvalidInput,
				format!("output path is not a directory: {}", path.display()),
			));
		}
		Err(err) if err.kind() == io::ErrorKind::NotFound => {}
		Err(err) => return Err(err),
	}

	if let Some(parent) = path.parent()
		&& !parent.as_os_str().is_empty()
		&& parent != path
	{
		create_dir_all_no_symlinks(parent)?;
	}

	fs::create_dir(path)?;
	let metadata = fs::symlink_metadata(path)?;
	if metadata.file_type().is_symlink() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!("output directory cannot be a symlink: {}", path.display()),
		));
	}
	if !metadata.is_dir() {
		return Err(io::Error::new(
			io::ErrorKind::InvalidInput,
			format!("output path is not a directory: {}", path.display()),
		));
	}
	Ok(())
}

fn write_bytes_to_file(bytes: &[u8], out_path: impl AsRef<Path>) -> io::Result<()> {
	let out_path = out_path.as_ref();
	if let Some(parent) = out_path.parent() {
		create_dir_all_no_symlinks(parent)?;
	}

	let temp_path = temp_output_path(out_path)?;
	let write_result = (|| -> io::Result<()> {
		let mut file = fs::OpenOptions::new()
			.write(true)
			.create_new(true)
			.open(&temp_path)?;
		file.write_all(bytes)?;
		file.sync_all()?;
		Ok(())
	})();
	if let Err(err) = write_result {
		let _ = fs::remove_file(&temp_path);
		return Err(err);
	}

	match fs::symlink_metadata(out_path) {
		Ok(metadata) if metadata.is_dir() => {
			let _ = fs::remove_file(&temp_path);
			return Err(io::Error::new(
				io::ErrorKind::InvalidInput,
				format!("output path is a directory: {}", out_path.display()),
			));
		}
		Ok(_) => {}
		Err(err) if err.kind() == io::ErrorKind::NotFound => {}
		Err(err) => {
			let _ = fs::remove_file(&temp_path);
			return Err(err);
		}
	}

	fs::rename(&temp_path, out_path).inspect_err(|_| {
		let _ = fs::remove_file(&temp_path);
	})
}

fn temp_output_path(out_path: &Path) -> io::Result<PathBuf> {
	let parent = out_path.parent().unwrap_or_else(|| Path::new("."));
	let file_name = out_path
		.file_name()
		.ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "output path has no file name"))?
		.to_string_lossy();
	let nonce = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.map_err(io::Error::other)?
		.as_nanos();

	for i in 0..100 {
		let path = parent.join(format!(
			".{file_name}.vorma-tmp-{}-{nonce}-{i}",
			std::process::id()
		));
		if !path.exists() {
			return Ok(path);
		}
	}

	Err(io::Error::new(
		io::ErrorKind::AlreadyExists,
		format!(
			"could not allocate temp output path for {}",
			out_path.display()
		),
	))
}

pub(crate) fn random_id(id_len: u8) -> Result<String, String> {
	const DEFAULT_CHARSET: &[u8] =
		b"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";

	if id_len == 0 {
		return Ok(String::new());
	}

	let charset_len = DEFAULT_CHARSET.len();
	let effective_total_values = (256 / charset_len) * charset_len;
	let mut out = String::with_capacity(id_len as usize);
	let mut random_byte_holder = [0; 1];
	for _ in 0..id_len {
		loop {
			getrandom::fill(&mut random_byte_holder)
				.map_err(|err| format!("failed to read random bytes: {err}"))?;
			let random_val = usize::from(random_byte_holder[0]);
			if random_val < effective_total_values {
				out.push(DEFAULT_CHARSET[random_val % charset_len] as char);
				break;
			}
		}
	}
	Ok(out)
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn random_id_generates_mixed_alphanumeric_ids() {
		let id = random_id(32).unwrap();

		assert_eq!(id.len(), 32);
		assert!(id.chars().all(|c| c.is_ascii_alphanumeric()));
	}

	#[cfg(unix)]
	#[test]
	fn create_dir_all_no_symlinks_rejects_symlink_component() {
		let root = temp_dir("rejects-symlink-component");
		let outside = temp_dir("rejects-symlink-component-outside");
		fs::create_dir_all(&outside).unwrap();
		std::os::unix::fs::symlink(&outside, root.join("link")).unwrap();

		let err = create_dir_all_no_symlinks(root.join("link/nested")).unwrap_err();

		assert_eq!(err.kind(), io::ErrorKind::InvalidInput);
		assert!(!outside.join("nested").exists());

		fs::remove_dir_all(root).unwrap();
		fs::remove_dir_all(outside).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn write_str_to_file_replaces_symlink_without_touching_target() {
		let root = temp_dir("replaces-symlink");
		let external = root.join("external.txt");
		let link = root.join("link.txt");
		fs::write(&external, "external").unwrap();
		std::os::unix::fs::symlink(&external, &link).unwrap();

		write_str_to_file("generated", &link).unwrap();

		assert_eq!(fs::read_to_string(&external).unwrap(), "external");
		assert_eq!(fs::read_to_string(&link).unwrap(), "generated");
		assert!(
			!fs::symlink_metadata(&link)
				.unwrap()
				.file_type()
				.is_symlink()
		);

		fs::remove_dir_all(root).unwrap();
	}

	fn temp_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-utils-{name}-{nonce}"));
		fs::create_dir_all(&root).unwrap();
		root
	}
}
