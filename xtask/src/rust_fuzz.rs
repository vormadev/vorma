use std::env;
use std::fs;
use std::path::Path;
use std::process::Command;

const ARTIFACTS_ROOT: &str = "fuzz/artifacts";
const CARGO_PROGRAM: &str = "cargo";
const CLIENT_WASM_PROTOCOL_TARGET: &str = "client_wasm_protocol";
const CORPUS_ROOT: &str = "fuzz/corpus";
const DEFAULT_FUZZ_RUNS: &str = "4096";
const FUZZ_DIR: &str = "fuzz";
const FUZZ_GATE_CORPUS_ROOT: &str = "target/fuzz-gate-corpus";
const FUZZ_RUNS_ENV: &str = "FUZZ_RUNS";
const MATCHER_PATTERNS_TARGET: &str = "matcher_patterns";
const RELEASE_DIR: &str = "release";
const RUSTC_PROGRAM: &str = "rustc";
const TARGET_DIR: &str = "target";

const FUZZ_TARGETS: &[&str] = &[MATCHER_PATTERNS_TARGET, CLIENT_WASM_PROTOCOL_TARGET];

pub(crate) fn run() -> Result<i32, String> {
	let fuzz_runs = env::var(FUZZ_RUNS_ENV).unwrap_or_else(|_| DEFAULT_FUZZ_RUNS.to_owned());
	let host_triple = read_rust_host_triple()?;

	reset_gate_corpus()?;
	for target_name in FUZZ_TARGETS {
		build_fuzz_target(target_name)?;
		run_fuzz_target(target_name, &host_triple, &fuzz_runs)?;
	}

	Ok(0)
}

fn reset_gate_corpus() -> Result<(), String> {
	let gate_corpus_root = Path::new(FUZZ_GATE_CORPUS_ROOT);
	match fs::remove_dir_all(gate_corpus_root) {
		Ok(()) => {}
		Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
		Err(error) => {
			return Err(format!(
				"remove fuzz gate corpus {}: {error}",
				gate_corpus_root.display()
			));
		}
	}
	fs::create_dir_all(gate_corpus_root).map_err(|error| {
		format!(
			"create fuzz gate corpus {}: {error}",
			gate_corpus_root.display()
		)
	})?;
	copy_dir(Path::new(CORPUS_ROOT), gate_corpus_root)
}

fn build_fuzz_target(target_name: &str) -> Result<(), String> {
	run_command_in_dir(
		Path::new(FUZZ_DIR),
		CARGO_PROGRAM,
		&["+nightly", "fuzz", "build", target_name],
	)
}

fn run_fuzz_target(target_name: &str, host_triple: &str, fuzz_runs: &str) -> Result<(), String> {
	let binary_path = Path::new(FUZZ_DIR)
		.join(TARGET_DIR)
		.join(host_triple)
		.join(RELEASE_DIR)
		.join(target_name);
	let artifact_prefix = Path::new(ARTIFACTS_ROOT).join(target_name);
	let copied_corpus = Path::new(FUZZ_GATE_CORPUS_ROOT).join(target_name);

	fs::create_dir_all(&artifact_prefix).map_err(|error| {
		format!(
			"create fuzz artifact directory {}: {error}",
			artifact_prefix.display()
		)
	})?;

	let artifact_prefix_arg = format!("-artifact_prefix={}/", artifact_prefix.display());
	let runs_arg = format!("-runs={fuzz_runs}");
	let copied_corpus = copied_corpus
		.to_str()
		.ok_or_else(|| format!("non-UTF-8 fuzz corpus path {}", copied_corpus.display()))?;

	run_command(
		binary_path
			.to_str()
			.ok_or_else(|| format!("non-UTF-8 fuzz binary path {}", binary_path.display()))?,
		&[
			artifact_prefix_arg.as_str(),
			runs_arg.as_str(),
			copied_corpus,
		],
	)
}

fn read_rust_host_triple() -> Result<String, String> {
	let output = Command::new(RUSTC_PROGRAM)
		.arg("-vV")
		.output()
		.map_err(|error| format!("spawn {RUSTC_PROGRAM} -vV: {error}"))?;
	if !output.status.success() {
		return Err(format!("{RUSTC_PROGRAM} -vV exited with {}", output.status));
	}
	let stdout = String::from_utf8(output.stdout)
		.map_err(|error| format!("{RUSTC_PROGRAM} -vV emitted non-UTF-8 stdout: {error}"))?;
	parse_rust_host_triple(&stdout)
}

fn parse_rust_host_triple(rustc_verbose_version: &str) -> Result<String, String> {
	for line in rustc_verbose_version.lines() {
		let Some(host_triple) = line.strip_prefix("host: ") else {
			continue;
		};
		if host_triple.is_empty() {
			return Err("rustc host triple was empty".to_owned());
		}
		return Ok(host_triple.to_owned());
	}

	Err("rustc -vV did not report a host triple".to_owned())
}

fn copy_dir(src: &Path, dest: &Path) -> Result<(), String> {
	fs::create_dir_all(dest)
		.map_err(|error| format!("create directory {}: {error}", dest.display()))?;
	for entry in
		fs::read_dir(src).map_err(|error| format!("read directory {}: {error}", src.display()))?
	{
		let entry =
			entry.map_err(|error| format!("read directory entry {}: {error}", src.display()))?;
		let file_type = entry
			.file_type()
			.map_err(|error| format!("read file type {}: {error}", entry.path().display()))?;
		let dest_path = dest.join(entry.file_name());
		if file_type.is_dir() {
			copy_dir(&entry.path(), &dest_path)?;
		} else if file_type.is_file() {
			fs::copy(entry.path(), &dest_path).map_err(|error| {
				format!(
					"copy {} to {}: {error}",
					entry.path().display(),
					dest_path.display()
				)
			})?;
		} else {
			return Err(format!(
				"unsupported fuzz corpus entry {}",
				entry.path().display()
			));
		}
	}
	Ok(())
}

fn run_command(program: &str, args: &[&str]) -> Result<(), String> {
	run_command_with_dir(None, program, args)
}

fn run_command_in_dir(cwd: &Path, program: &str, args: &[&str]) -> Result<(), String> {
	run_command_with_dir(Some(cwd), program, args)
}

fn run_command_with_dir(cwd: Option<&Path>, program: &str, args: &[&str]) -> Result<(), String> {
	let mut command = Command::new(program);
	command.args(args);
	if let Some(cwd) = cwd {
		command.current_dir(cwd);
	}
	let status = command
		.status()
		.map_err(|error| format!("spawn {}: {error}", format_command(cwd, program, args)))?;
	if status.success() {
		Ok(())
	} else {
		Err(format!(
			"{} exited with {status}",
			format_command(cwd, program, args)
		))
	}
}

fn format_command(cwd: Option<&Path>, program: &str, args: &[&str]) -> String {
	let mut command = String::new();
	if let Some(cwd) = cwd {
		command.push_str("cd ");
		command.push_str(&cwd.display().to_string());
		command.push_str(" && ");
	}
	command.push_str(program);
	for arg in args {
		command.push(' ');
		command.push_str(arg);
	}
	command
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn parses_host_triple() {
		let output = "rustc 1.90.0\nhost: aarch64-apple-darwin\nrelease: 1.90.0\n";

		assert_eq!(
			parse_rust_host_triple(output).unwrap(),
			"aarch64-apple-darwin"
		);
	}

	#[test]
	fn rejects_missing_host_triple() {
		assert!(parse_rust_host_triple("rustc 1.90.0").is_err());
	}
}
