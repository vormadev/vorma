use std::fs;
use std::path::Path;
use std::process::Command;

const CARGO_TOML_PATH: &str = "Cargo.toml";
const GIT_PROGRAM: &str = "git";
const PNPM_PROGRAM: &str = "pnpm";
const PRE_RELEASE_NPM_TAG: &str = "pre";
const VERSION_PREFIX: &str = "v";
const WORKSPACE_PACKAGE_HEADER: &str = "[workspace.package]";

pub(crate) fn run() -> Result<i32, String> {
	let version = read_workspace_package_version(Path::new(CARGO_TOML_PATH))?;
	let tag = format!("{VERSION_PREFIX}{version}");

	run_command(
		PNPM_PROGRAM,
		&[
			"version",
			version.as_str(),
			"--recursive",
			"--no-git-checks",
			"--no-git-tag-version",
			"--allow-same-version",
		],
	)?;
	run_command(GIT_PROGRAM, &["add", "."])?;
	run_command(GIT_PROGRAM, &["commit", "-m", tag.as_str(), "--no-verify"])?;
	run_command(GIT_PROGRAM, &["tag", tag.as_str()])?;

	let mut publish_args = vec!["publish", "--access", "public", "--recursive"];
	if is_pre_release_version(&version) {
		publish_args.extend(["--tag", PRE_RELEASE_NPM_TAG]);
	}
	run_command(PNPM_PROGRAM, &publish_args)?;

	Ok(0)
}

fn read_workspace_package_version(cargo_toml_path: &Path) -> Result<String, String> {
	let cargo_toml = fs::read_to_string(cargo_toml_path)
		.map_err(|error| format!("read {}: {error}", cargo_toml_path.display()))?;
	parse_workspace_package_version(&cargo_toml)
}

fn parse_workspace_package_version(cargo_toml: &str) -> Result<String, String> {
	let mut in_workspace_package = false;

	for line in cargo_toml.lines() {
		let line = line
			.split_once('#')
			.map_or(line, |(line_before_comment, _)| line_before_comment)
			.trim();
		if line.is_empty() {
			continue;
		}
		if line.starts_with('[') && line.ends_with(']') {
			in_workspace_package = line == WORKSPACE_PACKAGE_HEADER;
			continue;
		}
		if !in_workspace_package {
			continue;
		}
		let Some((key, value)) = line.split_once('=') else {
			continue;
		};
		if key.trim() != "version" {
			continue;
		}
		return parse_quoted_version_value(value.trim());
	}

	Err(format!(
		"missing workspace package version in {CARGO_TOML_PATH}"
	))
}

fn parse_quoted_version_value(value: &str) -> Result<String, String> {
	let Some(value) = value
		.strip_prefix('"')
		.and_then(|value| value.strip_suffix('"'))
	else {
		return Err(format!(
			"workspace package version must be a quoted string: {value}"
		));
	};
	if value.is_empty() {
		return Err("workspace package version must not be empty".to_owned());
	}
	Ok(value.to_owned())
}

fn is_pre_release_version(version: &str) -> bool {
	version.contains('-')
}

fn run_command(program: &str, args: &[&str]) -> Result<(), String> {
	let status = Command::new(program)
		.args(args)
		.status()
		.map_err(|error| format!("spawn {}: {error}", format_command(program, args)))?;
	if status.success() {
		Ok(())
	} else {
		Err(format!(
			"{} exited with {status}",
			format_command(program, args)
		))
	}
}

fn format_command(program: &str, args: &[&str]) -> String {
	let mut command = program.to_owned();
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
	fn parses_workspace_package_version() {
		let cargo_toml = r#"
[package]
version = "wrong"

[workspace.package]
edition = "2024"
version = "0.86.0-pre.2"

[workspace.dependencies]
vorma = { version = "wrong" }
"#;

		assert_eq!(
			parse_workspace_package_version(cargo_toml).unwrap(),
			"0.86.0-pre.2"
		);
	}

	#[test]
	fn detects_pre_release_versions() {
		assert!(is_pre_release_version("1.2.3-pre.1"));
		assert!(!is_pre_release_version("1.2.3"));
	}
}
