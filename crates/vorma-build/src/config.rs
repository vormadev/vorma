use std::fmt;
use std::ops::Deref;
use std::path::{Path, PathBuf};

use path_clean::PathClean;
use path_slash::{PathBufExt, PathExt};
use vorma::__private::{Config, validate_public_static_base_against_api_mount};
use vorma::TsExtraType;

use crate::build_layout::BuildLayout;
use crate::cargo_target::{CargoBinTarget, validate_cargo_bin_target};
use crate::frontend_toolchain::FrontendToolchain;
use crate::public_static_inputs::PublicStaticInputs;
use crate::watch_config::{WatchConfig, WatchConfigError};

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct ConfigError {
	kind: ConfigErrorKind,
	message: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum ConfigErrorKind {
	RootDir,
	DistDir,
	ServerConfig,
	FrontendToolchain,
	PublicStaticAndApiPaths,
	WatchPatterns,
	ServerWatchPatterns,
	ClientRevalidateOnChangePatterns,
	JavaScriptPackageManagerDir,
	PublicStaticSourceDir,
	TypeScriptOutputFile,
	FrontendEntryFile,
	CriticalCssEntry,
	ViteConfigFile,
}

impl ConfigError {
	fn raw(kind: ConfigErrorKind, message: impl Into<String>) -> Self {
		Self {
			kind,
			message: message.into(),
		}
	}

	fn with_context(kind: ConfigErrorKind, context: &str, err: impl fmt::Display) -> Self {
		Self::raw(kind, format!("error with {context}: {err}"))
	}
}

impl fmt::Display for ConfigError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.write_str(&self.message)
	}
}

impl std::error::Error for ConfigError {}

impl Deref for ConfigError {
	type Target = str;

	fn deref(&self) -> &Self::Target {
		&self.message
	}
}

impl PartialEq<&str> for ConfigError {
	fn eq(&self, other: &&str) -> bool {
		self.message == *other
	}
}

#[derive(Debug)]
pub(crate) struct ConfigView<'a> {
	source: &'a Config,
	root_dir: PathBuf,
	build_layout: BuildLayout,
	frontend_toolchain: FrontendToolchain,
	js_package_manager_dir: PathBuf,
	app_server: CargoBinTarget,
	watch_config: WatchConfig,
	public_static_inputs: PublicStaticInputs,
	public_static_base_path: String,
	api_mount_root: String,
}

impl<'a> ConfigView<'a> {
	pub(crate) fn new(source: &'a Config) -> Result<Self, ConfigError> {
		let root_dir = validate_root_dir(&source.root_dir)
			.map_err(|err| ConfigError::with_context(ConfigErrorKind::RootDir, "root dir", err))?;
		let build_layout = BuildLayout::new(
			generated_output_path_from_root(&root_dir, &source.dist_dir, "dist_dir").map_err(
				|err| ConfigError::with_context(ConfigErrorKind::DistDir, "dist dir", err),
			)?,
		);
		let app_server = validate_cargo_bin_target(
			&CargoBinTarget {
				cargo_package: source.server_config.cargo_package.clone(),
				cargo_bin: source.server_config.cargo_bin.clone(),
			},
			"server config",
		)
		.map_err(|err| {
			ConfigError::with_context(ConfigErrorKind::ServerConfig, "server config", err)
		})?;
		let frontend_toolchain = FrontendToolchain::from_config(&source.frontend_config)
			.map_err(|err| ConfigError::raw(ConfigErrorKind::FrontendToolchain, err.to_string()))?;
		let js_package_manager_dir = root_contained_directory_path(
			&root_dir,
			&source.frontend_config.js_package_manager_dir,
			"frontend_config.js_package_manager_dir",
		)
		.map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::JavaScriptPackageManagerDir,
				"JavaScript package-manager dir",
				err,
			)
		})?;
		let (public_static_base_path, api_mount_root) =
			validate_public_static_base_against_api_mount(
				&source.path_config.public_static_base,
				&source.path_config.api_base,
			)
			.map_err(|err| {
				ConfigError::with_context(
					ConfigErrorKind::PublicStaticAndApiPaths,
					"public static/API paths",
					err,
				)
			})?;
		let watch_config = WatchConfig::new(
			&root_dir,
			&build_layout.vorma_internal_root(),
			&source.dev_watch_config,
		)
		.map_err(|err| match err {
			WatchConfigError::Watch(err) => {
				ConfigError::with_context(ConfigErrorKind::WatchPatterns, "watch patterns", err)
			}
			WatchConfigError::ServerWatch(err) => ConfigError::with_context(
				ConfigErrorKind::ServerWatchPatterns,
				"server watch patterns",
				err,
			),
			WatchConfigError::ClientRevalidateOnChange(err) => ConfigError::with_context(
				ConfigErrorKind::ClientRevalidateOnChangePatterns,
				"client revalidate on change patterns",
				err,
			),
		})?;
		let public_static_inputs = PublicStaticInputs::new(
			&root_dir,
			&source.frontend_config.public_static_src_dir,
			public_static_base_path.clone(),
		)
		.map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::PublicStaticSourceDir,
				"public static source dir",
				err,
			)
		})?;

		let view = Self {
			source,
			root_dir,
			build_layout,
			frontend_toolchain,
			js_package_manager_dir,
			app_server,
			watch_config,
			public_static_inputs,
			public_static_base_path,
			api_mount_root,
		};
		view.ts_gen_out_file().map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::TypeScriptOutputFile,
				"TypeScript output file",
				err,
			)
		})?;
		view.ts_entry().map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::FrontendEntryFile,
				"frontend entry file",
				err,
			)
		})?;
		view.critical_css_entry().map_err(|err| {
			ConfigError::with_context(ConfigErrorKind::CriticalCssEntry, "critical CSS entry", err)
		})?;
		let watch_patterns = view.watch_patterns();
		crate::globset::compile(&watch_patterns).map_err(|err| {
			ConfigError::with_context(ConfigErrorKind::WatchPatterns, "watch patterns", err)
		})?;
		let server_watch_patterns = view.server_watch_patterns();
		crate::globset::compile(&server_watch_patterns).map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::ServerWatchPatterns,
				"server watch patterns",
				err,
			)
		})?;
		let client_revalidate_patterns = view.client_revalidate_on_change_patterns();
		crate::globset::compile(&client_revalidate_patterns).map_err(|err| {
			ConfigError::with_context(
				ConfigErrorKind::ClientRevalidateOnChangePatterns,
				"client revalidate on change patterns",
				err,
			)
		})?;
		view.vite_config_file().map_err(|err| {
			ConfigError::with_context(ConfigErrorKind::ViteConfigFile, "Vite config file", err)
		})?;
		Ok(view)
	}

	pub(crate) fn source(&self) -> &Config {
		self.source
	}

	pub(crate) fn root_dir(&self) -> &Path {
		&self.root_dir
	}

	pub(crate) fn root_path(&self, path: impl AsRef<Path>) -> PathBuf {
		root_path_from(&self.root_dir, path)
	}

	pub(crate) fn root_path_string(&self, path: &str) -> String {
		self.root_path(PathBuf::from_slash(path.trim()))
			.to_string_lossy()
			.into_owned()
	}

	pub(crate) fn build_layout(&self) -> &BuildLayout {
		&self.build_layout
	}

	pub(crate) fn dist_dir(&self) -> PathBuf {
		self.build_layout.dist_dir()
	}

	pub(crate) fn ts_gen_out_file(&self) -> Result<PathBuf, String> {
		self.required_generated_output_path(
			&self.source.ts_gen_config.out_file,
			"ts_gen_config.out_file",
		)
	}

	pub(crate) fn critical_css_entry(&self) -> Result<Option<PathBuf>, String> {
		let entry = self.source.frontend_config.critical_css_file.trim();
		if entry.is_empty() || entry == "." {
			return Ok(None);
		}
		self.required_root_contained_file_path(
			&self.source.frontend_config.critical_css_file,
			"frontend_config.critical_css_file",
		)
		.map(Some)
	}

	pub(crate) fn public_static_base_path(&self) -> &str {
		&self.public_static_base_path
	}

	pub(crate) fn api_mount_root(&self) -> &str {
		&self.api_mount_root
	}

	pub(crate) fn ui_variant(&self) -> &'static str {
		self.frontend_toolchain.ui_variant()
	}

	pub(crate) fn app_server_cargo_target(&self) -> &CargoBinTarget {
		&self.app_server
	}

	pub(crate) fn js_package_manager_cmd_base(&self) -> &[String] {
		self.frontend_toolchain.js_package_manager_cmd_base()
	}

	pub(crate) fn js_package_manager_dir(&self) -> PathBuf {
		self.js_package_manager_dir.clone()
	}

	pub(crate) fn vite_config_file(&self) -> Result<String, String> {
		if self
			.source
			.frontend_config
			.vite_config_file
			.trim()
			.is_empty()
		{
			return Ok(String::new());
		}
		let vite_config_file = self.required_root_contained_file_path(
			&self.source.frontend_config.vite_config_file,
			"frontend_config.vite_config_file",
		)?;
		relative_path(self.js_package_manager_dir(), vite_config_file)
			.map(|path| path.to_string_lossy().into_owned())
			.ok_or_else(|| "error calculating relative path".to_owned())
	}

	pub(crate) fn ts_entry(&self) -> Result<PathBuf, String> {
		self.required_root_contained_file_path(
			&self.source.frontend_config.entry_file,
			"frontend_config.entry_file",
		)
	}

	pub(crate) fn vorma_out(&self) -> PathBuf {
		self.build_layout.vorma_internal_root()
	}

	pub(crate) fn pub_out(&self) -> PathBuf {
		self.build_layout.public_static_out()
	}

	pub(crate) fn dev_cargo_target_dir(&self) -> PathBuf {
		self.build_layout.dev_cargo_target_dir()
	}

	pub(crate) fn dev_lock_out(&self) -> PathBuf {
		self.build_layout.dev_lock_out()
	}

	pub(crate) fn gitignore_out(&self) -> PathBuf {
		self.build_layout.gitignore_out()
	}

	pub(crate) fn manifest_json_out(&self, is_dev: bool) -> PathBuf {
		self.build_layout.manifest_json_out(is_dev)
	}

	pub(crate) fn prod_tmp_vite_manifest_out(&self) -> PathBuf {
		self.build_layout.prod_tmp_vite_manifest_out()
	}

	pub(crate) fn watch_patterns(&self) -> Vec<String> {
		self.watch_config.watch_patterns()
	}

	pub(crate) fn server_watch_patterns(&self) -> Vec<String> {
		self.watch_config.server_watch_patterns()
	}

	pub(crate) fn client_revalidate_on_change_patterns(&self) -> Vec<String> {
		self.watch_config.client_revalidate_on_change_patterns()
	}

	pub(crate) fn vite_dedupe_list(&self) -> Vec<String> {
		self.frontend_toolchain.vite_dedupe_list()
	}

	pub(crate) fn vite_server_args(&self, vite_port: u16) -> Result<Vec<String>, String> {
		Ok(self
			.frontend_toolchain
			.vite_server_args(vite_port, self.vite_config_file()?))
	}

	pub(crate) fn cargo_build_args(
		&self,
		build_entry: &CargoBinTarget,
	) -> Result<Vec<String>, String> {
		let build_entry = validate_cargo_bin_target(build_entry, "build entry")?;
		let app_server = self.app_server_cargo_target().clone();
		if build_entry == app_server {
			return Err(
				"build entry Cargo target must be different from app server Cargo target"
					.to_owned(),
			);
		}

		let mut args = vec![
			"build".to_owned(),
			"--message-format=json-render-diagnostics".to_owned(),
			"--target-dir".to_owned(),
			self.dev_cargo_target_dir().to_string_lossy().into_owned(),
			"-p".to_owned(),
			build_entry.cargo_package.clone(),
		];
		if build_entry.cargo_package != app_server.cargo_package {
			args.extend(["-p".to_owned(), app_server.cargo_package.clone()]);
		}
		args.extend(["--bin".to_owned(), build_entry.cargo_bin.clone()]);
		if build_entry.cargo_bin != app_server.cargo_bin {
			args.extend(["--bin".to_owned(), app_server.cargo_bin]);
		}
		Ok(args)
	}

	pub(crate) fn cargo_build_entry_args(
		&self,
		build_entry: &CargoBinTarget,
	) -> Result<Vec<String>, String> {
		let build_entry = validate_cargo_bin_target(build_entry, "build entry")?;
		Ok(vec![
			"build".to_owned(),
			"--message-format=json-render-diagnostics".to_owned(),
			"--target-dir".to_owned(),
			self.dev_cargo_target_dir().to_string_lossy().into_owned(),
			"-p".to_owned(),
			build_entry.cargo_package,
			"--bin".to_owned(),
			build_entry.cargo_bin,
		])
	}

	fn required_generated_output_path(&self, path: &str, label: &str) -> Result<PathBuf, String> {
		let path = self.required_config_path(path, label)?;
		let path = self.root_path(path);
		validate_path_inside_root(&path, &self.root_dir, label)?;
		Ok(path)
	}

	fn required_root_path(&self, path: &str, label: &str) -> Result<PathBuf, String> {
		Ok(self.root_path(self.required_config_path(path, label)?))
	}

	fn required_root_contained_file_path(
		&self,
		path: &str,
		label: &str,
	) -> Result<PathBuf, String> {
		let path = self.required_root_path(path, label)?;
		validate_path_inside_root(&path, &self.root_dir, label)?;
		Ok(path)
	}

	fn required_config_path(&self, path: &str, label: &str) -> Result<PathBuf, String> {
		let trimmed = path.trim();
		if trimmed.is_empty() {
			return Err(format!("{label} cannot be empty"));
		}
		let path = PathBuf::from_slash(trimmed).clean();
		if path == Path::new(".") {
			return Err(format!("{label} cannot be ."));
		}
		Ok(path)
	}
}

#[derive(Debug)]
pub(crate) struct VormaCfg<'a> {
	view: ConfigView<'a>,
}

pub(crate) fn to_cfg(config: &Config) -> Result<VormaCfg<'_>, ConfigError> {
	ConfigView::new(config).map(|view| VormaCfg { view })
}

impl VormaCfg<'_> {
	pub(crate) fn source(&self) -> &Config {
		self.view.source()
	}

	pub(crate) fn extra_ts(&self) -> String {
		self.source().ts_gen_config.extra_ts.to_string()
	}

	pub(crate) fn extra_types(&self) -> &[TsExtraType] {
		&self.source().ts_gen_config.extra_types
	}

	pub(crate) fn root_dir(&self) -> String {
		path_string(self.view.root_dir())
	}

	pub(crate) fn root_path_string(&self, path: &str) -> String {
		self.view.root_path_string(path)
	}

	pub(crate) fn build_layout(&self) -> &BuildLayout {
		self.view.build_layout()
	}

	pub(crate) fn dist_dir(&self) -> String {
		path_string(self.view.dist_dir())
	}

	pub(crate) fn ts_gen_out_file(&self) -> String {
		checked_path(self.view.ts_gen_out_file())
	}

	pub(crate) fn watch_patterns(&self) -> Vec<String> {
		self.view.watch_patterns()
	}

	pub(crate) fn server_watch_patterns(&self) -> Vec<String> {
		self.view.server_watch_patterns()
	}

	pub(crate) fn client_revalidate_on_change_patterns(&self) -> Vec<String> {
		self.view.client_revalidate_on_change_patterns()
	}

	pub(crate) fn critical_css_entry(&self) -> Option<String> {
		checked(self.view.critical_css_entry()).map(path_string)
	}

	pub(crate) fn public_static_base_path(&self) -> String {
		self.view.public_static_base_path().to_owned()
	}

	pub(crate) fn api_mount_root(&self) -> String {
		self.view.api_mount_root().to_owned()
	}

	pub(crate) fn ui_variant(&self) -> String {
		self.view.ui_variant().to_owned()
	}

	pub(crate) fn ts_entry(&self) -> String {
		checked_path(self.view.ts_entry())
	}

	pub(crate) fn js_package_manager_cmd_base(&self) -> Vec<String> {
		self.view.js_package_manager_cmd_base().to_vec()
	}

	pub(crate) fn js_package_manager_dir(&self) -> String {
		path_string(self.view.js_package_manager_dir())
	}

	pub(crate) fn vite_config_file(&self) -> String {
		checked(self.view.vite_config_file())
	}

	pub(crate) fn pub_src_pattern(&self) -> String {
		self.view.public_static_inputs.source_catch_pattern()
	}

	pub(crate) fn collect_physical_pub_files(&self) -> Result<crate::staticproc::Files, String> {
		self.view.public_static_inputs.collect_physical_files()
	}

	pub(crate) fn to_pub_fm(
		&self,
		pub_files: &crate::staticproc::Files,
	) -> std::collections::BTreeMap<String, String> {
		self.view.public_static_inputs.to_public_filemap(pub_files)
	}

	pub(crate) fn vorma_out(&self) -> String {
		path_string(self.view.vorma_out())
	}

	pub(crate) fn pub_out(&self) -> String {
		path_string(self.view.pub_out())
	}

	pub(crate) fn dev_lock_out(&self) -> String {
		path_string(self.view.dev_lock_out())
	}

	pub(crate) fn gitignore_out(&self) -> String {
		path_string(self.view.gitignore_out())
	}

	pub(crate) fn vorma_out_abs_slash_pattern(&self) -> String {
		let mut path = PathBuf::from(self.vorma_out());
		path.push("**/*");
		abs_slash(path)
	}

	pub(crate) fn gen_out_file_abs_slash(&self) -> String {
		abs_slash(self.ts_gen_out_file())
	}

	pub(crate) fn ts_entry_abs_slash(&self) -> String {
		abs_slash(self.ts_entry())
	}

	pub(crate) fn vite_ignored_patterns(&self) -> Vec<String> {
		vec![
			"**/*.rs".to_owned(),
			self.vorma_out_abs_slash_pattern(),
			self.gen_out_file_abs_slash(),
		]
	}

	pub(crate) fn vite_dedupe_list(&self) -> Vec<String> {
		self.view.vite_dedupe_list()
	}

	pub(crate) fn manifest_json_out(&self, is_dev: bool) -> String {
		path_string(self.view.manifest_json_out(is_dev))
	}

	pub(crate) fn prod_tmp_vite_manifest_out(&self) -> String {
		path_string(self.view.prod_tmp_vite_manifest_out())
	}

	pub(crate) fn vite_server_args(&self, vite_port: u16) -> Vec<String> {
		checked(self.view.vite_server_args(vite_port))
	}

	pub(crate) fn cargo_build_args(
		&self,
		build_entry: &CargoBinTarget,
	) -> Result<Vec<String>, String> {
		self.view.cargo_build_args(build_entry)
	}

	pub(crate) fn cargo_build_entry_args(
		&self,
		build_entry: &CargoBinTarget,
	) -> Result<Vec<String>, String> {
		self.view.cargo_build_entry_args(build_entry)
	}

	pub(crate) fn app_server_cargo_target(&self) -> CargoBinTarget {
		self.view.app_server_cargo_target().clone()
	}
}

fn validate_root_dir(path: &Path) -> Result<PathBuf, String> {
	if path.as_os_str().is_empty() {
		return Err("root_dir cannot be empty".to_owned());
	}
	let root_dir = path.clean();
	if !root_dir.is_absolute() {
		return Err("root_dir must be absolute".to_owned());
	}
	if !root_dir.exists() {
		return Err(format!("root dir does not exist: {}", root_dir.display()));
	}
	if !root_dir.is_dir() {
		return Err(format!(
			"root dir is not a directory: {}",
			root_dir.display()
		));
	}
	Ok(root_dir)
}

fn root_path_from(root_dir: &Path, path: impl AsRef<Path>) -> PathBuf {
	let path = path.as_ref();
	if path.is_absolute() {
		return path.clean();
	}
	root_dir.join(path).clean()
}

fn generated_output_path_from_root(
	root_dir: &Path,
	path: &str,
	label: &str,
) -> Result<PathBuf, String> {
	let path = root_path_from(root_dir, PathBuf::from_slash(path.trim()));
	validate_path_inside_root(&path, root_dir, label)?;
	Ok(path)
}

fn root_contained_directory_path(
	root_dir: &Path,
	path: &str,
	label: &str,
) -> Result<PathBuf, String> {
	let trimmed = path.trim();
	if trimmed.is_empty() {
		return Err(format!("{label} cannot be empty"));
	}
	let path = root_path_from(root_dir, PathBuf::from_slash(trimmed));
	validate_path_inside_root(&path, root_dir, label)?;
	Ok(path)
}

fn validate_path_inside_root(path: &Path, root_dir: &Path, label: &str) -> Result<(), String> {
	if !path.starts_with(root_dir) {
		return Err(format!("{label} must be inside root_dir"));
	}
	Ok(())
}

pub(crate) fn relative_path(from: impl AsRef<Path>, to: impl AsRef<Path>) -> Option<PathBuf> {
	pathdiff::diff_paths(to, from)
}

pub(crate) fn sys_norm(path: impl AsRef<Path>) -> String {
	path.as_ref().clean().to_string_lossy().into_owned()
}

fn to_slash(path: impl AsRef<Path>) -> String {
	path.as_ref().to_slash_lossy().into_owned()
}

fn path_string(path: impl AsRef<Path>) -> String {
	path.as_ref().to_string_lossy().into_owned()
}

fn checked<T>(value: Result<T, String>) -> T {
	value.expect("validated Vorma config getter failed")
}

fn checked_path(value: Result<PathBuf, String>) -> String {
	path_string(checked(value))
}

fn abs_slash(path: impl AsRef<Path>) -> String {
	let path = path.as_ref();
	let abs = if path.is_absolute() {
		path.to_path_buf()
	} else {
		std::env::current_dir()
			.expect("current dir should be available")
			.join(path)
	};
	to_slash(abs.clean())
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::{DevWatchConfig, FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-build-{name}-{nonce}"));
		fs::create_dir_all(&root).unwrap();
		root
	}

	fn config(root: PathBuf) -> Config {
		Config {
			root_dir: root,
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: String::new(),
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: String::new(),
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/client/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			dev_watch_config: DevWatchConfig::default(),
		}
	}

	fn dev_cargo_target_dir(root_dir: &Path) -> String {
		root_dir
			.join("dist")
			.join(".vorma")
			.join("cargo")
			.join("dev")
			.to_string_lossy()
			.into_owned()
	}

	fn cfg(config: &Config) -> VormaCfg<'_> {
		to_cfg(config).unwrap()
	}

	#[test]
	fn config_view_derives_framework_owned_output_paths_from_build_layout() {
		let root = temp_root("paths");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(view.dist_dir(), root.join("dist"));
		assert_eq!(view.vorma_out(), root.join("dist/.vorma"));
		assert_eq!(
			view.build_layout.static_root(),
			root.join("dist/.vorma/static")
		);
		assert_eq!(view.pub_out(), root.join("dist/.vorma/static/public"));
		assert_eq!(
			view.manifest_json_out(true),
			root.join("dist/.vorma/static")
				.join(vorma::__private::manifest::MANIFEST_STATIC_OUT_DEV)
		);
		assert_eq!(
			view.manifest_json_out(false),
			root.join("dist/.vorma/static")
				.join(vorma::__private::manifest::MANIFEST_STATIC_OUT_PROD)
		);
		assert_eq!(
			view.build_layout().prod_tmp_vite_manifest_dir(),
			root.join("dist/.vorma/static/public/tmp")
		);
		assert_eq!(
			view.prod_tmp_vite_manifest_out(),
			root.join("dist/.vorma/static/public/tmp/vorma_internal_tmp_vite_manifest.json")
		);
		assert_eq!(
			view.build_layout().prod_vite_manifest_out(),
			root.join("dist/.vorma/static/vite_manifest.json")
		);
		assert_eq!(view.dev_lock_out(), root.join("dist/.vorma/dev.lock"));
		assert_eq!(view.gitignore_out(), root.join("dist/.vorma/.gitignore"));
		assert_eq!(
			view.dev_cargo_target_dir(),
			root.join("dist/.vorma/cargo/dev")
		);
		assert_eq!(
			view.ts_gen_out_file().unwrap(),
			root.join("src/client/vorma.gen.ts")
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_normalizes_public_static_and_api_bases() {
		let root = temp_root("bases");
		let mut config = config(root.clone());
		config.path_config = PathConfig {
			public_static_base: "static".to_owned(),
			api_base: "api".to_owned(),
		};
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(view.public_static_base_path(), "/static/");
		assert_eq!(view.api_mount_root(), "/api/");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_defaults_api_base_to_api_mount_root() {
		let root = temp_root("api-default");
		let mut config = config(root.clone());
		config.path_config = PathConfig::default();
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(view.api_mount_root(), "/api/");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_root_api_mount() {
		let root = temp_root("api-root");
		let mut config = config(root.clone());
		config.path_config.api_base = "/".to_owned();

		let err = ConfigView::new(&config).unwrap_err();

		assert_eq!(
			err,
			"error with public static/API paths: api_base must be a non-root path prefix such as /api/"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_public_static_base_under_api_mount_root() {
		let root = temp_root("static-api-overlap");
		let mut config = config(root.clone());
		config.path_config = PathConfig {
			public_static_base: "/api/assets/".to_owned(),
			api_base: "/api/".to_owned(),
		};

		let err = ConfigView::new(&config).unwrap_err();

		assert_eq!(
			err,
			"error with public static/API paths: public_static_base \"/api/assets/\" must not be under api_base \"/api/\""
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_defaults_server_watch_patterns_to_rust_files() {
		let root = temp_root("server-watch-default");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(view.server_watch_patterns(), vec!["**/*.rs"]);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_makes_vite_config_relative_to_js_package_manager_dir() {
		let root = temp_root("vite-config-relative");
		fs::create_dir_all(root.join("frontend")).unwrap();
		let mut config = config(root.clone());
		config.frontend_config.js_package_manager_dir = "frontend".to_owned();
		config.frontend_config.vite_config_file = "frontend/vite.config.ts".to_owned();
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(view.vite_config_file().unwrap(), "vite.config.ts");
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_relative_empty_or_missing_root_dir() {
		let root = temp_root("root-validation");
		let relative_config = Config {
			root_dir: PathBuf::from("."),
			..config(root.clone())
		};
		let empty_config = Config {
			root_dir: PathBuf::new(),
			..config(root.clone())
		};
		let missing_config = Config {
			root_dir: root.join("missing"),
			..config(root.clone())
		};

		assert_eq!(
			ConfigView::new(&relative_config).unwrap_err(),
			"error with root dir: root_dir must be absolute"
		);
		assert_eq!(
			ConfigView::new(&empty_config).unwrap_err(),
			"error with root dir: root_dir cannot be empty"
		);
		assert!(
			ConfigView::new(&missing_config)
				.unwrap_err()
				.starts_with("error with root dir: root dir does not exist:")
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_generated_output_paths_that_escape_root_dir() {
		let root = temp_root("generated-output-escape");
		let dist_config = Config {
			dist_dir: "../dist".to_owned(),
			..config(root.clone())
		};
		let ts_config = Config {
			ts_gen_config: TsGenConfig {
				out_file: "../vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..config(root.clone())
		};

		assert_eq!(
			ConfigView::new(&dist_config).unwrap_err(),
			"error with dist dir: dist_dir must be inside root_dir"
		);
		assert_eq!(
			ConfigView::new(&ts_config).unwrap_err(),
			"error with TypeScript output file: ts_gen_config.out_file must be inside root_dir"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_required_frontend_paths_that_collapse_to_root_dir() {
		let root = temp_root("required-frontend-paths");
		let ts_config = Config {
			ts_gen_config: TsGenConfig {
				out_file: String::new(),
				..TsGenConfig::default()
			},
			..config(root.clone())
		};
		let entry_config = Config {
			frontend_config: FrontendConfig {
				entry_file: ".".to_owned(),
				..config(root.clone()).frontend_config
			},
			..config(root.clone())
		};
		let public_config = Config {
			frontend_config: FrontendConfig {
				public_static_src_dir: ".".to_owned(),
				..config(root.clone()).frontend_config
			},
			..config(root.clone())
		};

		assert_eq!(
			ConfigView::new(&ts_config).unwrap_err(),
			"error with TypeScript output file: ts_gen_config.out_file cannot be empty"
		);
		assert_eq!(
			ConfigView::new(&entry_config).unwrap_err(),
			"error with frontend entry file: frontend_config.entry_file cannot be ."
		);
		assert_eq!(
			ConfigView::new(&public_config).unwrap_err(),
			"error with public static source dir: frontend_config.public_static_src_dir cannot be ."
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_public_static_source_dir_outside_root_dir() {
		let root = temp_root("public-static-source-outside-root");
		let mut config = config(root.clone());
		config.frontend_config.public_static_src_dir = "../public".to_owned();

		let err = ConfigView::new(&config).unwrap_err();

		assert_eq!(
			err,
			"error with public static source dir: frontend_config.public_static_src_dir must be inside root_dir"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[cfg(unix)]
	#[test]
	fn config_view_rejects_public_static_source_dir_symlink() {
		let root = temp_root("public-static-source-symlink");
		let outside = temp_root("public-static-source-symlink-outside");
		std::os::unix::fs::symlink(&outside, root.join("public")).unwrap();
		let config = config(root.clone());

		let err = ConfigView::new(&config).unwrap_err();

		assert_eq!(
			err,
			"error with public static source dir: frontend_config.public_static_src_dir must not be a symlink"
		);
		fs::remove_dir_all(root).unwrap();
		fs::remove_dir_all(outside).unwrap();
	}

	#[test]
	fn config_view_rejects_frontend_paths_outside_root_dir() {
		let root = temp_root("frontend-paths-outside-root");

		let mut package_manager_dir_config = config(root.clone());
		package_manager_dir_config
			.frontend_config
			.js_package_manager_dir = "../frontend".to_owned();
		assert_eq!(
			ConfigView::new(&package_manager_dir_config).unwrap_err(),
			"error with JavaScript package-manager dir: frontend_config.js_package_manager_dir must be inside root_dir"
		);

		let mut entry_config = config(root.clone());
		entry_config.frontend_config.entry_file = "../entry.tsx".to_owned();
		assert_eq!(
			ConfigView::new(&entry_config).unwrap_err(),
			"error with frontend entry file: frontend_config.entry_file must be inside root_dir"
		);

		let mut critical_css_config = config(root.clone());
		critical_css_config.frontend_config.critical_css_file = "../critical.css".to_owned();
		assert_eq!(
			ConfigView::new(&critical_css_config).unwrap_err(),
			"error with critical CSS entry: frontend_config.critical_css_file must be inside root_dir"
		);

		let mut vite_config = config(root.clone());
		vite_config.frontend_config.vite_config_file = "../vite.config.ts".to_owned();
		assert_eq!(
			ConfigView::new(&vite_config).unwrap_err(),
			"error with Vite config file: frontend_config.vite_config_file must be inside root_dir"
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_rejects_blank_js_command_and_watch_entries() {
		let root = temp_root("blank-config-values");
		let blank_command = Config {
			frontend_config: FrontendConfig {
				js_package_manager_base_cmd: " \t ".to_owned(),
				..config(root.clone()).frontend_config
			},
			..config(root.clone())
		};
		let blank_include = Config {
			dev_watch_config: DevWatchConfig {
				watch_patterns: vec![" ".to_owned()],
				..DevWatchConfig::default()
			},
			..config(root.clone())
		};
		let blank_exclude = Config {
			dev_watch_config: DevWatchConfig {
				on_change_client_revalidate: vec!["!".to_owned()],
				..DevWatchConfig::default()
			},
			..config(root.clone())
		};

		assert_eq!(
			ConfigView::new(&blank_command).unwrap_err(),
			"error with JavaScript package-manager command: frontend_config.js_package_manager_base_cmd cannot be empty"
		);
		assert_eq!(
			ConfigView::new(&blank_include).unwrap_err(),
			"error with watch patterns: watch pattern cannot be empty"
		);
		assert_eq!(
			ConfigView::new(&blank_exclude).unwrap_err(),
			"error with client revalidate on change patterns: client revalidate watch pattern cannot be empty"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn watch_patterns_may_explicitly_escape_root_dir() {
		let parent = temp_root("watch-parent");
		let root = parent.join("app");
		let outside = parent.join("shared");
		fs::create_dir_all(&root).unwrap();
		fs::create_dir_all(&outside).unwrap();
		let mut config = config(root);
		config.dev_watch_config.watch_patterns = vec![
			"../shared".to_owned(),
			format!("{}/src/**/*.tsx", outside.to_string_lossy()),
		];
		let view = ConfigView::new(&config).unwrap();

		let patterns = view.watch_patterns();

		assert!(patterns.iter().any(|pattern| pattern == "../shared"));
		assert!(
			patterns
				.iter()
				.any(|pattern| pattern.ends_with("shared/src/**/*.tsx"))
		);
		fs::remove_dir_all(parent).unwrap();
	}

	#[test]
	fn config_view_excludes_framework_and_rust_output_trees_from_watch_patterns() {
		let root = temp_root("watch-excludes");
		let mut config = config(root.clone());
		config.dist_dir = "nested/dist".to_owned();
		config.dev_watch_config.watch_patterns = vec![".".to_owned()];
		let cfg = cfg(&config);
		let watch_set = crate::globset::compile(&cfg.watch_patterns()).unwrap();

		assert!(!watch_set.is_match("nested/dist/.vorma/main"));
		assert!(!watch_set.is_match("target/debug/build/app"));
		assert!(!watch_set.is_match("crates/example/target/debug/app"));
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_builds_public_file_map_with_static_base_prefix() {
		let root = temp_root("public-file-map");
		fs::create_dir_all(root.join("public")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		let config = config(root.clone());
		let cfg = cfg(&config);
		let files = cfg.collect_physical_pub_files().unwrap();
		let map = cfg.to_pub_fm(&files);
		let out = map.get("app.css").unwrap();

		assert!(out.starts_with("/static/vorma_out_app_"));
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn config_view_includes_prehashed_directory_as_ordinary_public_input() {
		let root = temp_root("ordinary-prehashed-dir");
		fs::create_dir_all(root.join("public/__prehashed/vendor")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		fs::write(root.join("public/__prehashed/vendor/app.abc123.js"), "app").unwrap();
		let config = config(root.clone());
		let cfg = cfg(&config);
		let files = cfg.collect_physical_pub_files().unwrap();
		let map = cfg.to_pub_fm(&files);

		assert!(map.contains_key("app.css"));
		let prehashed_dir_url = map
			.get("__prehashed/vendor/app.abc123.js")
			.expect("__prehashed source dir should be ordinary public input");
		assert!(prehashed_dir_url.starts_with("/static/vorma_out___prehashed_vendor_app.abc123_"));
		assert!(prehashed_dir_url.ends_with(".js"));
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn sys_norm_cleans_relative_segments() {
		let slash = to_slash(PathBuf::from("./target/../src").clean());

		assert!(!slash.contains("/./"));
		assert!(!slash.contains("/target/../"));
		assert_eq!(slash, "src");
	}

	#[test]
	fn config_view_requires_server_config_for_cargo_builds() {
		let root = temp_root("server-config-required");
		let mut config = config(root.clone());
		config.server_config = ServerConfig::default();

		assert_eq!(
			ConfigView::new(&config).unwrap_err(),
			"error with server config: server config cargo_package cannot be empty"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cargo_build_args_build_distinct_build_and_app_targets_together() {
		let root = temp_root("cargo-args");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();
		let build_entry = CargoBinTarget {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-build".to_owned(),
		};

		assert_eq!(
			view.cargo_build_args(&build_entry).unwrap(),
			[
				"build",
				"--message-format=json-render-diagnostics",
				"--target-dir",
				&dev_cargo_target_dir(&root),
				"-p",
				"example-app",
				"--bin",
				"example-build",
				"--bin",
				"example-server",
			]
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cargo_build_args_reject_same_build_entry_and_app_server_target() {
		let root = temp_root("same-build-entry-app-server");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();

		assert_eq!(
			view.cargo_build_args(view.app_server_cargo_target())
				.unwrap_err(),
			"build entry Cargo target must be different from app server Cargo target"
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cargo_build_args_allows_same_bin_name_across_packages() {
		let root = temp_root("same-bin-different-package");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();
		let build_entry = CargoBinTarget {
			cargo_package: "example-build".to_owned(),
			cargo_bin: "example-server".to_owned(),
		};

		assert_eq!(
			view.cargo_build_args(&build_entry).unwrap(),
			[
				"build",
				"--message-format=json-render-diagnostics",
				"--target-dir",
				&dev_cargo_target_dir(&root),
				"-p",
				"example-build",
				"-p",
				"example-app",
				"--bin",
				"example-server",
			]
		);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cargo_build_entry_args_builds_only_build_entry_target() {
		let root = temp_root("build-entry-only");
		let config = config(root.clone());
		let view = ConfigView::new(&config).unwrap();
		let build_entry = CargoBinTarget {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-build".to_owned(),
		};

		assert_eq!(
			view.cargo_build_entry_args(&build_entry).unwrap(),
			[
				"build",
				"--message-format=json-render-diagnostics",
				"--target-dir",
				&dev_cargo_target_dir(&root),
				"-p",
				"example-app",
				"--bin",
				"example-build",
			]
		);
		fs::remove_dir_all(root).unwrap();
	}
}
