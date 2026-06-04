use vorma::FrontendConfig;

use crate::constants::DEV_LOOPBACK_HOST;

const UI_VARIANT_REACT: &str = "react";
const UI_VARIANT_PREACT: &str = "preact";
const UI_VARIANT_REMIX: &str = "remix";
const UI_VARIANT_SOLID: &str = "solid";

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct FrontendToolchain {
	ui_variant: UiVariant,
	js_package_manager_cmd_base: Vec<String>,
}

impl FrontendToolchain {
	pub(crate) fn from_config(config: &FrontendConfig) -> Result<Self, FrontendToolchainError> {
		Ok(Self {
			ui_variant: UiVariant::parse(&config.ui_variant)
				.map_err(FrontendToolchainError::UiVariant)?,
			js_package_manager_cmd_base: parse_required_command(
				&config.js_package_manager_base_cmd,
			)
			.map_err(FrontendToolchainError::JsPackageManagerCommand)?,
		})
	}

	pub(crate) fn ui_variant(&self) -> &'static str {
		self.ui_variant.as_str()
	}

	pub(crate) fn js_package_manager_cmd_base(&self) -> &[String] {
		&self.js_package_manager_cmd_base
	}

	pub(crate) fn vite_dedupe_list(&self) -> Vec<String> {
		self.ui_variant.vite_dedupe_list()
	}

	pub(crate) fn vite_server_args(&self, vite_port: u16, vite_config_file: String) -> Vec<String> {
		let mut args = self.js_package_manager_cmd_base().to_vec();
		args.extend([
			"vite".to_owned(),
			"--host".to_owned(),
			DEV_LOOPBACK_HOST.to_owned(),
			"--port".to_owned(),
			vite_port.to_string(),
			"--clearScreen".to_owned(),
			"false".to_owned(),
			"--strictPort".to_owned(),
			"true".to_owned(),
		]);
		if !vite_config_file.is_empty() {
			args.extend(["--config".to_owned(), vite_config_file]);
		}
		args
	}
}

#[derive(Debug)]
pub(crate) enum FrontendToolchainError {
	UiVariant(String),
	JsPackageManagerCommand(String),
}

impl std::fmt::Display for FrontendToolchainError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::UiVariant(err) => write!(f, "error with UI variant: {err}"),
			Self::JsPackageManagerCommand(err) => {
				write!(f, "error with JavaScript package-manager command: {err}")
			}
		}
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum UiVariant {
	React,
	Preact,
	Remix,
	Solid,
}

impl UiVariant {
	fn parse(raw: &str) -> Result<Self, String> {
		match raw.trim() {
			UI_VARIANT_REACT => Ok(Self::React),
			UI_VARIANT_PREACT => Ok(Self::Preact),
			UI_VARIANT_REMIX => Ok(Self::Remix),
			UI_VARIANT_SOLID => Ok(Self::Solid),
			ui => Err(format!("invalid UI variant: {ui}")),
		}
	}

	fn as_str(self) -> &'static str {
		match self {
			Self::React => UI_VARIANT_REACT,
			Self::Preact => UI_VARIANT_PREACT,
			Self::Remix => UI_VARIANT_REMIX,
			Self::Solid => UI_VARIANT_SOLID,
		}
	}

	fn vite_dedupe_list(self) -> Vec<String> {
		match self {
			Self::React => vec!["react".to_owned(), "react-dom".to_owned()],
			Self::Preact => vec![
				"preact".to_owned(),
				"preact/hooks".to_owned(),
				"@preact/signals".to_owned(),
				"preact/jsx-runtime".to_owned(),
				"preact/compat".to_owned(),
				"preact/test-utils".to_owned(),
			],
			Self::Remix => vec![
				"remix".to_owned(),
				"remix/ui".to_owned(),
				"@remix-run/ui".to_owned(),
			],
			Self::Solid => vec!["solid-js".to_owned(), "solid-js/web".to_owned()],
		}
	}
}

fn parse_required_command(raw: &str) -> Result<Vec<String>, String> {
	let args = raw
		.split_whitespace()
		.map(str::to_owned)
		.collect::<Vec<_>>();
	if args.is_empty() {
		return Err("frontend_config.js_package_manager_base_cmd cannot be empty".to_owned());
	}
	Ok(args)
}
