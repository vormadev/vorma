use std::fs::{self, File};
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

const DEFAULT_LOG_DIR: &str = "logs.local";
const MAKE_PROGRAM: &str = "make";
const MAKE_NO_PRINT_DIRECTORY_FLAG: &str = "--no-print-directory";
const RUST_GATE_TARGET: &str = "rust-gate";
const TS_GATE_TARGET: &str = "ts-gate";
const E2E_TARGET: &str = "e2e";
const GATE_STEPS: &[&str] = &[RUST_GATE_TARGET, TS_GATE_TARGET, E2E_TARGET];

#[derive(Debug, Eq, PartialEq)]
struct StepFailure {
	name: &'static str,
	log_path: PathBuf,
	reason: String,
}

pub(crate) fn run() -> Result<i32, String> {
	let log_dir = Path::new(DEFAULT_LOG_DIR);
	fs::create_dir_all(log_dir)
		.map_err(|error| format!("create gate log directory {}: {error}", log_dir.display()))?;

	let step_count = GATE_STEPS.len();
	let mut failures = Vec::new();

	for (index, step_name) in GATE_STEPS.iter().copied().enumerate() {
		let step_number = index + 1;
		let log_path = step_log_path(log_dir, index, step_name);
		println!("gate [{step_number}/{step_count}] {step_name}: running");
		match run_step(step_name, &log_path) {
			Ok(()) => {
				println!(
					"gate [{step_number}/{step_count}] {step_name}: passed ({})",
					log_path.display()
				);
			}
			Err(reason) => {
				println!(
					"gate [{step_number}/{step_count}] {step_name}: failed ({})",
					log_path.display()
				);
				failures.push(StepFailure {
					name: step_name,
					log_path,
					reason,
				});
			}
		}
	}

	if failures.is_empty() {
		println!("gate: all steps passed");
		return Ok(0);
	}

	let failed_names = failures
		.iter()
		.map(|failure| failure.name)
		.collect::<Vec<_>>()
		.join(", ");
	println!("gate: failed steps: {failed_names}");
	for failure in failures {
		println!(
			"gate: {}: {} ({})",
			failure.name,
			failure.reason,
			failure.log_path.display()
		);
	}
	Ok(1)
}

fn step_log_path(log_dir: &Path, index: usize, step_name: &str) -> PathBuf {
	log_dir.join(format!("gate-{:02}-{step_name}.txt", index + 1))
}

fn run_step(step_name: &str, log_path: &Path) -> Result<(), String> {
	let mut log_file = File::create(log_path)
		.map_err(|error| format!("create log file {}: {error}", log_path.display()))?;
	write_step_header(&mut log_file, step_name)?;
	let stdout = log_file
		.try_clone()
		.map_err(|error| format!("clone log file {}: {error}", log_path.display()))?;
	let stderr = log_file;

	let status = Command::new(MAKE_PROGRAM)
		.arg(MAKE_NO_PRINT_DIRECTORY_FLAG)
		.arg(step_name)
		.stdout(Stdio::from(stdout))
		.stderr(Stdio::from(stderr))
		.status()
		.map_err(|error| format!("spawn {step_name}: {error}"))?;
	if status.success() {
		Ok(())
	} else {
		Err(status.to_string())
	}
}

fn write_step_header(log_file: &mut File, step_name: &str) -> Result<(), String> {
	writeln!(
		log_file,
		"$ {MAKE_PROGRAM} {MAKE_NO_PRINT_DIRECTORY_FLAG} {step_name}"
	)
	.map_err(format_log_write_error)?;
	writeln!(log_file).map_err(format_log_write_error)?;
	log_file.flush().map_err(format_log_write_error)
}

fn format_log_write_error(error: io::Error) -> String {
	format!("write gate log: {error}")
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn gate_steps_match_make_gate_order() {
		assert_eq!(GATE_STEPS, &[RUST_GATE_TARGET, TS_GATE_TARGET, E2E_TARGET]);
	}

	#[test]
	fn log_paths_are_ordered() {
		assert_eq!(
			step_log_path(Path::new(DEFAULT_LOG_DIR), 0, RUST_GATE_TARGET),
			PathBuf::from(format!("{DEFAULT_LOG_DIR}/gate-01-{RUST_GATE_TARGET}.txt"))
		);
	}
}
