use vorma::{FrontendConfig, UiVariant};

use crate::constants::DEV_LOOPBACK_HOST;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct FrontendToolchain {
	ui_variant: UiVariant,
	js_package_manager_cmd_base: Vec<String>,
}

impl FrontendToolchain {
	pub(crate) fn from_config(config: &FrontendConfig) -> Result<Self, FrontendToolchainError> {
		Ok(Self {
			ui_variant: config.ui_variant,
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
		ui_variant_vite_dedupe_list(self.ui_variant)
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
	JsPackageManagerCommand(String),
}

impl std::fmt::Display for FrontendToolchainError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::JsPackageManagerCommand(err) => {
				write!(f, "error with JavaScript package-manager command: {err}")
			}
		}
	}
}

fn ui_variant_vite_dedupe_list(ui_variant: UiVariant) -> Vec<String> {
	match ui_variant {
		UiVariant::React => vec!["react".to_owned(), "react-dom".to_owned()],
		UiVariant::Preact => vec![
			"preact".to_owned(),
			"preact/hooks".to_owned(),
			"@preact/signals".to_owned(),
			"preact/jsx-runtime".to_owned(),
			"preact/compat".to_owned(),
			"preact/test-utils".to_owned(),
		],
		UiVariant::Solid => vec!["solid-js".to_owned(), "solid-js/web".to_owned()],
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
