use std::fs;
use std::io::{self, ErrorKind, Read, Seek, SeekFrom};
use std::net::TcpStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::mpsc;
use std::thread;
use std::time::{Duration, Instant};

use serde::Deserialize;

const DEV_SERVER_READY_PREFIX: &str = "App server ready: ";
const PROD_SERVER_READY_POLL_INTERVAL: Duration = Duration::from_millis(25);
const PROD_SERVER_READY_STABILITY_INTERVAL: Duration = Duration::from_millis(10);
const PROD_SERVER_READY_TIMEOUT: Duration = Duration::from_secs(5);
const DEV_SERVER_READY_POLL_INTERVAL: Duration = Duration::from_millis(25);
const DEV_SERVER_READY_TIMEOUT: Duration = Duration::from_secs(90);
const DEV_SERVER_SHUTDOWN_TIMEOUT: Duration = Duration::from_secs(10);
const DEV_SERVER_SHUTDOWN_POLL_INTERVAL: Duration = Duration::from_millis(25);
const DEV_CHANGE_POLL_INTERVAL: Duration = Duration::from_millis(25);
const DEV_SERVER_REBUILD_TIMEOUT: Duration = Duration::from_secs(180);
const DEV_CLIENT_MODULE_CHANGE_TIMEOUT: Duration = Duration::from_secs(60);
const LOCALHOST_URL_PREFIX: &str = "http://localhost:";
const LOOPBACK_URL_PREFIX: &str = "http://127.0.0.1:";
const CHROME_LOCAL_PERMISSIONS: &str = "local-network-access,local-network,loopback-network";
const BOMBADIL_ARTIFACTS_DIR: &str = ".bombadil";
const BOMBADIL_LOGS_DIR: &str = ".bombadil/logs";
const BOMBADIL_SERVER_BIN_DIR: &str = ".bombadil/server-bin";
const BOMBADIL_DEV_CARGO_DIR: &str = ".bombadil/cargo/dev";
const BOMBADIL_SERVER_CARGO_DIR: &str = ".bombadil/cargo/prod";
const BOMBADIL_VITE_CACHE_DIR: &str = ".bombadil/vite-cache";
const COMMAND_LOG_TAIL_BYTES: u64 = 16_000;
const FRAMEWORK_DIST_DIR_PREFIX: &str = ".dist";
const DEV_MANIFEST_FILENAME: &str = "vorma.manifest.dev.json";
const DEV_MARKER_PATH: &str = "src/dev_marker.rs";
const DEV_MARKER_A: &str = "server-marker-a";
const DEV_MARKER_B: &str = "server-marker-b";
const DEV_CRITICAL_CSS_PATH: &str = "shared/styles/main.critical.css";
const DEV_CRITICAL_CSS_MARKER: &str = "dev-change-critical-css-probe";
const DEV_VIEW_MODULE_PATH: &str = "components/routes/root.ts";
const DEV_VIEW_MODULE_MARKER: &str = "client-view-module-probe";
const DEV_HMR_CHECK_SCRIPT_PATH: &str = "dev_hmr_check.mjs";
const DEV_HMR_REACT_PROBE_PATH: &str = "runtime/react_hmr_probe.tsx";
const DEV_HMR_PREACT_PROBE_PATH: &str = "runtime/preact_hmr_probe.tsx";
const DEV_HMR_SOLID_PROBE_PATH: &str = "runtime/solid_hmr_probe.tsx";
const DEV_HMR_MARKER_A: &str = "client-hmr-marker-a";
const DEV_HMR_MARKER_B: &str = "client-hmr-marker-b";
const VORMA_SPEC_PATH: &str = "./specs/vorma.property.ts";
const BUILD_SKEW_SPEC_PATH: &str = "./specs/build_skew.property.ts";
const LATENCY_SPEC_PATH: &str = "./specs/latency.property.ts";
const BOMBADIL_VARIANT_ENV_KEY: &str = "VORMA_BOMBADIL_VARIANT";
const BOMBADIL_DEPLOYMENT_ENV_KEY: &str = "VORMA_BOMBADIL_DEPLOYMENT";
const BOMBADIL_MODE_ENV_KEY: &str = "VORMA_BOMBADIL_MODE";
const VARIANT_REACT: &str = "react";
const VARIANT_PREACT: &str = "preact";
const VARIANT_SOLID: &str = "solid";
const BOMBADIL_MODE_DEV: &str = "dev";
const BOMBADIL_MODE_PROD: &str = "prod";
const DEPLOYMENT_A_SUFFIX: &str = "a";
const DEPLOYMENT_B_SUFFIX: &str = "b";
const PACKAGE_MANAGER_NONINTERACTIVE_ENV_KEY: &str = "CI";
const PACKAGE_MANAGER_NONINTERACTIVE_ENV_VALUE: &str = "true";

#[derive(Clone)]
struct VariantConfig {
	name: &'static str,
	port: u16,
}

#[derive(Clone)]
struct RunConfig {
	intensity: u64,
	variants: Vec<VariantConfig>,
}

#[derive(Clone)]
struct VariantRunner {
	config: RunConfig,
	variant: VariantConfig,
}

#[derive(Deserialize)]
struct DevManifest {
	dev_vite_server_port: u16,
	client_entry: DevClientModule,
	client_views: std::collections::BTreeMap<String, DevClientModule>,
}

#[derive(Deserialize)]
struct DevClientModule {
	url: String,
}

fn main() {
	let config = RunConfig {
		intensity: 1,
		variants: vec![
			VariantConfig {
				name: VARIANT_REACT,
				port: 18080,
			},
			VariantConfig {
				name: VARIANT_PREACT,
				port: 18081,
			},
			VariantConfig {
				name: VARIANT_SOLID,
				port: 18082,
			},
		],
	};

	if let Err(error) = config.run_command(std::env::args().skip(1).collect()) {
		eprintln!("{error}");
		std::process::exit(1);
	}
}

impl RunConfig {
	fn run_command(mut self, args: Vec<String>) -> Result<(), String> {
		let Some(command) = args.first().map(String::as_str) else {
			return Err(
				"usage: bombadil <build|test-prod|test-dev|test-dev-changes|serve-dev|inspect>"
					.to_owned(),
			);
		};

		match command {
			"build" => self.build(),
			"test-prod" => {
				let (intensity, variant_name) = parse_test_flags(&args[1..])?;
				self.intensity = intensity;
				self.test_prod(variant_name.as_deref())
			}
			"serve-dev" => {
				if args.len() != 2 {
					return Err("usage: bombadil serve-dev <react|preact|solid>".to_owned());
				}
				self.serve_dev(&args[1])
			}
			"test-dev" => {
				let (intensity, variant_name) = parse_test_flags(&args[1..])?;
				self.intensity = intensity;
				self.test_dev(variant_name.as_deref())
			}
			"test-dev-changes" => {
				let (intensity, variant_name) = parse_test_flags(&args[1..])?;
				self.intensity = intensity;
				self.test_dev_changes(variant_name.as_deref())
			}
			"inspect" => {
				let inspect_path = args
					.get(1)
					.map(String::as_str)
					.unwrap_or(BOMBADIL_ARTIFACTS_DIR);
				let mut cmd = Command::new("pnpm");
				set_noninteractive_package_manager_env(&mut cmd);
				cmd.current_dir(framework_root()).args([
					"exec",
					"bombadil",
					"inspect",
					inspect_path,
				]);
				cmd.status()
					.map_err(|error| error.to_string())
					.and_then(status_result)
			}
			_ => Err(format!("unknown command {command:?}")),
		}
	}

	fn build(&self) -> Result<(), String> {
		self.for_each_variant(|runner| runner.build())
	}

	fn test_prod(&self, variant_name: Option<&str>) -> Result<(), String> {
		if variant_name.is_some() {
			return self.for_each_selected_variant(variant_name, |runner| runner.test_prod());
		}

		self.for_each_variant_serial(|runner| runner.build())?;
		self.for_each_variant_serial(|runner| runner.test())
	}

	fn test_dev(&self, variant_name: Option<&str>) -> Result<(), String> {
		self.for_each_selected_variant(variant_name, |runner| runner.test_dev())
	}

	fn test_dev_changes(&self, variant_name: Option<&str>) -> Result<(), String> {
		let Some(variant_name) = variant_name else {
			return Err("test-dev-changes requires -variant".to_owned());
		};
		self.for_each_selected_variant(Some(variant_name), |runner| runner.test_dev_changes())
	}

	fn serve_dev(&self, name: &str) -> Result<(), String> {
		let variant = self
			.find_variant(name)
			.ok_or_else(|| format!("unknown variant {name:?}"))?;
		let runner = VariantRunner {
			config: self.clone(),
			variant,
		};
		runner.clean_dev_artifacts()?;
		let (build_binary, cleanup) = runner.build_dev_binary()?;
		let mut server = Command::new(build_binary);
		server.current_dir(framework_root());
		server.arg("dev");
		runner.set_process_group(&mut server);
		set_noninteractive_package_manager_env(&mut server);
		server.env(BOMBADIL_VARIANT_ENV_KEY, runner.variant.name);
		server.env(BOMBADIL_DEPLOYMENT_ENV_KEY, "A");
		server.env(BOMBADIL_MODE_ENV_KEY, BOMBADIL_MODE_DEV);
		let mut child = server.spawn().map_err(|error| error.to_string())?;
		let status = child.wait().map_err(|error| error.to_string());
		cleanup();
		status.and_then(status_result)
	}

	fn for_each_selected_variant<F>(
		&self,
		variant_name: Option<&str>,
		callback: F,
	) -> Result<(), String>
	where
		F: Fn(VariantRunner) -> Result<(), String> + Copy + Send + Sync + 'static,
	{
		if let Some(variant_name) = variant_name {
			let variant = self
				.find_variant(variant_name)
				.ok_or_else(|| format!("unknown variant {variant_name:?}"))?;
			return callback(VariantRunner {
				config: self.clone(),
				variant,
			});
		}
		self.for_each_variant(callback)
	}

	fn find_variant(&self, name: &str) -> Option<VariantConfig> {
		self.variants
			.iter()
			.find(|variant| variant.name == name)
			.cloned()
	}

	fn for_each_variant<F>(&self, callback: F) -> Result<(), String>
	where
		F: Fn(VariantRunner) -> Result<(), String> + Copy + Send + Sync + 'static,
	{
		let (tx, rx) = mpsc::channel();
		for variant in self.variants.clone() {
			let tx = tx.clone();
			let runner = VariantRunner {
				config: self.clone(),
				variant,
			};
			thread::spawn(move || {
				let _ = tx.send(callback(runner));
			});
		}
		drop(tx);

		let mut errors = Vec::new();
		for result in rx {
			if let Err(error) = result {
				errors.push(error);
			}
		}
		if errors.is_empty() {
			return Ok(());
		}
		Err(errors.join("\n"))
	}

	fn for_each_variant_serial<F>(&self, callback: F) -> Result<(), String>
	where
		F: Fn(VariantRunner) -> Result<(), String>,
	{
		for variant in self.variants.clone() {
			callback(VariantRunner {
				config: self.clone(),
				variant,
			})?;
		}
		Ok(())
	}
}

impl VariantRunner {
	fn test_prod(&self) -> Result<(), String> {
		self.build()?;
		self.test()
	}

	fn test_dev(&self) -> Result<(), String> {
		self.log("starting dev fixture server");
		let (log_path, mut child, cleanup) = self.start_logged_dev_server("dev")?;

		let base_url = match self.wait_for_dev_server_ready(&log_path, &mut child) {
			Ok(base_url) => base_url.replace(LOCALHOST_URL_PREFIX, LOOPBACK_URL_PREFIX),
			Err(error) => {
				print_file_to_stderr(&log_path);
				self.stop_dev_server(&mut child);
				cleanup();
				return Err(error);
			}
		};

		self.log(&format!("dev fixture server ready at {base_url}"));
		let result =
			self.run_bombadil_suite(&base_url, &format!("dev-{}", self.variant.name), "inline");
		self.stop_dev_server(&mut child);
		cleanup();
		result.and_then(|_| self.clean_dev_artifacts())
	}

	fn test_dev_changes(&self) -> Result<(), String> {
		self.log("starting dev fixture server for change coverage");
		let (log_path, mut child, cleanup) = self.start_logged_dev_server("dev-changes")?;
		let result = self.test_dev_changes_inner(&log_path, &mut child);
		self.stop_dev_server(&mut child);
		cleanup();
		result.and_then(|_| self.clean_dev_artifacts())
	}

	fn start_logged_dev_server(
		&self,
		log_label: &str,
	) -> Result<(PathBuf, Child, impl FnOnce()), String> {
		self.clean_dev_artifacts()?;
		let log_path = self.log_path(log_label)?;
		let log_file = fs::File::create(&log_path).map_err(|error| error.to_string())?;

		let (build_binary, cleanup) = self.build_dev_binary()?;
		let mut server = Command::new(build_binary);
		server.current_dir(framework_root());
		server.arg("dev");
		self.set_process_group(&mut server);
		set_noninteractive_package_manager_env(&mut server);
		server.env(BOMBADIL_VARIANT_ENV_KEY, self.variant.name);
		server.env(BOMBADIL_DEPLOYMENT_ENV_KEY, "A");
		server.env(BOMBADIL_MODE_ENV_KEY, BOMBADIL_MODE_DEV);
		server.stdout(Stdio::from(
			log_file.try_clone().map_err(|error| error.to_string())?,
		));
		server.stderr(Stdio::from(log_file));
		let child = match server.spawn() {
			Ok(child) => child,
			Err(error) => {
				cleanup();
				return Err(error.to_string());
			}
		};
		Ok((log_path, child, cleanup))
	}

	fn test_dev_changes_inner(&self, log_path: &Path, child: &mut Child) -> Result<(), String> {
		let base_url = self
			.wait_for_dev_server_ready(log_path, child)?
			.replace(LOCALHOST_URL_PREFIX, LOOPBACK_URL_PREFIX);

		self.log(&format!("dev fixture server ready at {base_url}"));

		let marker_url = format!("{base_url}/api/server-marker");
		self.wait_for_http_body_contains(
			child,
			&marker_url,
			DEV_MARKER_A,
			"initial server marker",
			DEV_SERVER_REBUILD_TIMEOUT,
		)?;

		let _server_guard = replace_file_text(
			&framework_root().join(DEV_MARKER_PATH),
			DEV_MARKER_A,
			DEV_MARKER_B,
		)?;
		self.wait_for_http_body_contains(
			child,
			&marker_url,
			DEV_MARKER_B,
			"server Rust rebuild",
			DEV_SERVER_REBUILD_TIMEOUT,
		)?;

		self.run_dev_hmr_check(child, &base_url)?;

		let root_view_url = self
			.read_dev_manifest()?
			.client_views
			.get("/")
			.ok_or_else(|| "dev manifest missing root client view".to_owned())?
			.url
			.clone();
		let root_view_url = self.dev_url(&base_url, &root_view_url);
		let _view_guard = append_file_text(
			&framework_root().join(DEV_VIEW_MODULE_PATH),
			&format!("\nexport const dev_change_probe = \"{DEV_VIEW_MODULE_MARKER}\";\n"),
		)?;
		self.wait_for_http_body_contains(
			child,
			&root_view_url,
			DEV_VIEW_MODULE_MARKER,
			"client view module refresh",
			DEV_CLIENT_MODULE_CHANGE_TIMEOUT,
		)
	}

	fn run_dev_hmr_check(&self, child: &mut Child, base_url: &str) -> Result<(), String> {
		if let Some(status) = child.try_wait().map_err(|error| error.to_string())? {
			return Err(format!(
				"{} dev server exited before browser HMR check: {status}",
				self.variant.name,
			));
		}

		let hmr_probe_path = match self.variant.name {
			VARIANT_REACT => DEV_HMR_REACT_PROBE_PATH,
			VARIANT_PREACT => DEV_HMR_PREACT_PROBE_PATH,
			VARIANT_SOLID => DEV_HMR_SOLID_PROBE_PATH,
			other => return Err(format!("unknown variant {other:?}")),
		};
		let mut cmd = Command::new("node");
		cmd.args([
			DEV_HMR_CHECK_SCRIPT_PATH,
			base_url,
			self.variant.name,
			hmr_probe_path,
			DEV_HMR_MARKER_A,
			DEV_HMR_MARKER_B,
			DEV_CRITICAL_CSS_PATH,
			DEV_CRITICAL_CSS_MARKER,
		]);
		let log_path = bombadil_logs_dir().join(format!("dev-hmr-{}.log", self.variant.name));
		self.run_command_to_log(&mut cmd, &log_path)?;

		if let Some(status) = child.try_wait().map_err(|error| error.to_string())? {
			return Err(format!(
				"{} dev server exited during browser HMR check: {status}",
				self.variant.name,
			));
		}
		self.log("browser critical CSS and HMR checks observed");
		Ok(())
	}

	fn build_dev_binary(&self) -> Result<(PathBuf, impl FnOnce()), String> {
		let target_dir = bombadil_dev_cargo_dir(self.variant.name);
		fs::create_dir_all(&target_dir).map_err(|error| {
			format!(
				"create dev Cargo target dir {}: {error}",
				target_dir.display()
			)
		})?;
		let mut build_cmd = Command::new("cargo");
		build_cmd.args([
			"build",
			"-p",
			"vorma-framework-tests",
			"--bin",
			"framework-build",
			"--target-dir",
		]);
		build_cmd.arg(&target_dir);
		let log_path = bombadil_logs_dir().join(format!("build-dev-{}.log", self.variant.name));
		self.run_command_to_log(&mut build_cmd, &log_path)?;
		let build_binary = target_dir
			.join("debug")
			.join(format!("framework-build{}", std::env::consts::EXE_SUFFIX));
		Ok((build_binary, || {}))
	}

	fn build(&self) -> Result<(), String> {
		self.log("building deployment A");
		self.build_client_deployment("A")?;

		self.log("building deployment B");
		self.build_client_deployment("B")?;

		let bin_dir = bombadil_server_bin_dir(self.variant.name);
		fs::create_dir_all(&bin_dir).map_err(|error| error.to_string())?;

		self.log("building server binary");
		let target_dir = bombadil_server_cargo_dir(self.variant.name);
		let mut cmd = Command::new("cargo");
		cmd.args([
			"build",
			"-p",
			"vorma-framework-tests",
			"--release",
			"--bin",
			"framework-serve",
			"--target-dir",
		]);
		cmd.arg(&target_dir);
		cmd.env(BOMBADIL_VARIANT_ENV_KEY, self.variant.name);
		cmd.env(BOMBADIL_MODE_ENV_KEY, BOMBADIL_MODE_PROD);
		let log_path =
			bombadil_logs_dir().join(format!("build-prod-{}-server.log", self.variant.name));
		self.run_command_to_log(&mut cmd, &log_path)?;

		let built = target_dir
			.join("release")
			.join(format!("framework-serve{}", std::env::consts::EXE_SUFFIX));
		fs::copy(
			&built,
			bin_dir.join(format!("main{}", std::env::consts::EXE_SUFFIX)),
		)
		.map_err(|error| format!("copy server binary from {}: {error}", built.display()))?;
		Ok(())
	}

	fn build_client_deployment(&self, deployment: &str) -> Result<(), String> {
		let mut cmd = Command::new("cargo");
		cmd.args([
			"run",
			"-p",
			"vorma-framework-tests",
			"--bin",
			"framework-build",
		]);
		cmd.env(BOMBADIL_VARIANT_ENV_KEY, self.variant.name);
		cmd.env(BOMBADIL_DEPLOYMENT_ENV_KEY, deployment);
		cmd.env(BOMBADIL_MODE_ENV_KEY, BOMBADIL_MODE_PROD);
		set_noninteractive_package_manager_env(&mut cmd);
		let log_path = bombadil_logs_dir().join(format!(
			"build-prod-{}-{}.log",
			self.variant.name,
			deployment.to_ascii_lowercase(),
		));
		self.run_command_to_log(&mut cmd, &log_path)
	}

	fn test(&self) -> Result<(), String> {
		self.log("starting fixture server");
		let log_path = self.log_path("prod")?;
		ensure_port_available(self.variant.port)?;
		let log_file = fs::File::create(&log_path).map_err(|error| error.to_string())?;

		let mut server = Command::new(
			bombadil_server_bin_dir(self.variant.name)
				.join(format!("main{}", std::env::consts::EXE_SUFFIX)),
		);
		server.current_dir(framework_root());
		self.set_process_group(&mut server);
		server.env("PORT", self.variant.port.to_string());
		server.env(BOMBADIL_VARIANT_ENV_KEY, self.variant.name);
		server.env(BOMBADIL_MODE_ENV_KEY, BOMBADIL_MODE_PROD);
		server.stdout(Stdio::from(
			log_file.try_clone().map_err(|error| error.to_string())?,
		));
		server.stderr(Stdio::from(log_file));
		let mut child = server.spawn().map_err(|error| error.to_string())?;

		if let Err(error) = self.wait_until_ready(&mut child) {
			print_file_to_stderr(&log_path);
			self.stop_server(&mut child);
			return Err(error);
		}

		self.log("fixture server ready");
		let base_url = format!("http://127.0.0.1:{}", self.variant.port);
		let result = self
			.run_bombadil_suite(&base_url, self.variant.name, "files,inline")
			.and_then(|_| {
				self.run_bombadil_test(
					&base_url,
					"",
					25,
					&format!("{}-build-skew", self.variant.name),
					"files,inline",
					BUILD_SKEW_SPEC_PATH,
				)
			});
		self.stop_server(&mut child);
		result.and_then(|_| self.clean_successful_prod_artifacts())
	}

	fn log_path(&self, mode: &str) -> Result<PathBuf, String> {
		let logs_dir = bombadil_logs_dir();
		fs::create_dir_all(&logs_dir).map_err(|error| error.to_string())?;
		Ok(logs_dir.join(format!("{mode}-{}.log", self.variant.name)))
	}

	fn wait_until_ready(&self, server: &mut Child) -> Result<(), String> {
		let url = format!("http://127.0.0.1:{}/", self.variant.port);
		let deadline = Instant::now() + PROD_SERVER_READY_TIMEOUT;
		while Instant::now() < deadline {
			if let Some(status) = server.try_wait().map_err(|error| error.to_string())? {
				return Err(format!(
					"{} server exited before ready with {status}",
					self.variant.name,
				));
			}
			if http_get_ok_enough(&url) {
				thread::sleep(PROD_SERVER_READY_STABILITY_INTERVAL);
				if let Some(status) = server.try_wait().map_err(|error| error.to_string())? {
					return Err(format!(
						"{} server exited during ready probe with {status}",
						self.variant.name,
					));
				}
				return Ok(());
			}
			thread::sleep(PROD_SERVER_READY_POLL_INTERVAL);
		}
		Err(format!("{} server did not become ready", self.variant.name))
	}

	fn run_bombadil_suite(
		&self,
		base_url: &str,
		output_prefix: &str,
		instrument_javascript: &str,
	) -> Result<(), String> {
		self.run_bombadil_test(
			base_url,
			"",
			30,
			output_prefix,
			instrument_javascript,
			VORMA_SPEC_PATH,
		)?;
		self.run_bombadil_test(
			base_url,
			"/nested/alpha/details",
			15,
			&format!("{output_prefix}-nested"),
			instrument_javascript,
			VORMA_SPEC_PATH,
		)?;
		self.run_bombadil_test(
			base_url,
			"/counter?n=0",
			10,
			&format!("{output_prefix}-counter"),
			instrument_javascript,
			VORMA_SPEC_PATH,
		)?;
		self.run_bombadil_test(
			base_url,
			"",
			10,
			&format!("{output_prefix}-latency"),
			instrument_javascript,
			LATENCY_SPEC_PATH,
		)
	}

	fn run_bombadil_test(
		&self,
		base_url: &str,
		path: &str,
		seconds: u64,
		output_path: &str,
		instrument_javascript: &str,
		spec_path: &str,
	) -> Result<(), String> {
		let total_seconds = seconds
			.checked_mul(self.config.intensity)
			.ok_or_else(|| "Bombadil time limit overflowed u64 seconds".to_owned())?;
		let time_limit = format!("{total_seconds}s");
		let artifact_path = bombadil_artifact_path(output_path);
		self.log(&format!("testing {base_url}{path} for {time_limit}"));
		self.log(&format!(
			"writing Bombadil artifacts to {}",
			artifact_path.display(),
		));
		remove_dir_all_if_exists(&artifact_path, "stale Bombadil run artifacts")?;
		let logs_dir = bombadil_logs_dir();
		fs::create_dir_all(&logs_dir).map_err(|error| error.to_string())?;
		let test_log_path = logs_dir.join(format!("test-{output_path}.log"));

		let mut cmd = Command::new("pnpm");
		set_noninteractive_package_manager_env(&mut cmd);
		cmd.args([
			"exec",
			"bombadil",
			"test",
			&format!("{base_url}{path}"),
			spec_path,
			"--headless",
			"--exit-on-violation",
			"--instrument-javascript",
			instrument_javascript,
			"--chrome-grant-permissions",
			CHROME_LOCAL_PERMISSIONS,
			"--time-limit",
			&time_limit,
			"--output-path",
		]);
		cmd.arg(&artifact_path);
		let result = self.run_command_to_log(&mut cmd, &test_log_path);
		if result.is_ok() {
			remove_dir_all_if_exists(&artifact_path, "successful Bombadil run artifacts")?;
		}
		result.map_err(|error| {
			format!(
				"inspect artifacts with `cargo run -p vorma-framework-tests --bin framework-bombadil -- inspect {}`; read log at {}: {error}",
				artifact_path.display(),
				test_log_path.display(),
			)
		})
	}

	fn run_command_to_log(&self, cmd: &mut Command, log_path: &Path) -> Result<(), String> {
		if let Some(parent) = log_path.parent() {
			fs::create_dir_all(parent).map_err(|error| error.to_string())?;
		}
		let log_file = fs::File::create(log_path).map_err(|error| error.to_string())?;
		self.log(&format!("writing command log to {}", log_path.display()));
		cmd.stdout(Stdio::from(
			log_file.try_clone().map_err(|error| error.to_string())?,
		));
		cmd.stderr(Stdio::from(log_file));
		self.run_command(cmd).map_err(|error| {
			let log_tail =
				read_log_tail(log_path, COMMAND_LOG_TAIL_BYTES).unwrap_or_else(|tail_error| {
					format!("<failed to read command log tail: {tail_error}>")
				});
			format!(
				"read log at {}: {error}\n\n--- command log tail ---\n{log_tail}",
				log_path.display(),
			)
		})
	}

	fn run_command(&self, cmd: &mut Command) -> Result<(), String> {
		cmd.current_dir(framework_root());
		cmd.status()
			.map_err(|error| error.to_string())
			.and_then(status_result)
			.map_err(|error| format!("{}: {error}", self.variant.name))
	}

	fn wait_for_dev_server_ready(
		&self,
		log_path: &Path,
		server: &mut Child,
	) -> Result<String, String> {
		let deadline = Instant::now() + DEV_SERVER_READY_TIMEOUT;
		while Instant::now() < deadline {
			if let Some(status) = server.try_wait().map_err(|error| error.to_string())? {
				return Err(format!(
					"{} dev server exited before ready: {status}",
					self.variant.name,
				));
			}

			let (ready_url, ok) = self.read_dev_server_ready_url(log_path)?;
			if ok && self.dev_client_modules_serving(&ready_url) {
				return Ok(ready_url);
			}
			thread::sleep(DEV_SERVER_READY_POLL_INTERVAL);
		}
		Err(format!(
			"{} dev server did not become ready",
			self.variant.name
		))
	}

	fn read_dev_server_ready_url(&self, log_path: &Path) -> Result<(String, bool), String> {
		let data = fs::read_to_string(log_path).map_err(|error| error.to_string())?;
		for line in data.lines() {
			let Some((_, after)) = line.split_once(DEV_SERVER_READY_PREFIX) else {
				continue;
			};
			return Ok((after.trim().to_owned(), true));
		}
		Ok((String::new(), false))
	}

	fn dev_client_modules_serving(&self, app_base_url: &str) -> bool {
		let Ok(manifest) = self.read_dev_manifest() else {
			return false;
		};
		if manifest.dev_vite_server_port == 0 {
			return false;
		}

		let mut urls = vec![
			format!(
				"http://127.0.0.1:{}/@vite/client",
				manifest.dev_vite_server_port,
			),
			self.dev_url(app_base_url, &manifest.client_entry.url),
		];
		if let Some(root_view) = manifest.client_views.get("/") {
			urls.push(self.dev_url(app_base_url, &root_view.url));
		}

		urls.iter().all(|url| {
			if url.is_empty() {
				return false;
			}
			http_get_status(url) == Some(200)
		})
	}

	fn read_dev_manifest(&self) -> Result<DevManifest, String> {
		let manifest_path = self.dev_manifest_path();
		let data = fs::read(&manifest_path)
			.map_err(|error| format!("read dev manifest {}: {error}", manifest_path.display()))?;
		serde_json::from_slice::<DevManifest>(&data)
			.map_err(|error| format!("parse dev manifest {}: {error}", manifest_path.display()))
	}

	fn dev_manifest_path(&self) -> PathBuf {
		framework_dev_dist_dir(self.variant.name)
			.join(".vorma")
			.join("static")
			.join(DEV_MANIFEST_FILENAME)
	}

	fn clean_dev_artifacts(&self) -> Result<(), String> {
		for path in [
			framework_dev_dist_dir(self.variant.name),
			bombadil_vite_cache_dir(BOMBADIL_MODE_DEV, self.variant.name),
		] {
			remove_dir_all_if_exists(&path, "dev generated artifacts")?;
		}
		Ok(())
	}

	fn clean_successful_prod_artifacts(&self) -> Result<(), String> {
		for path in [
			framework_prod_dist_dir(self.variant.name, DEPLOYMENT_A_SUFFIX),
			framework_prod_dist_dir(self.variant.name, DEPLOYMENT_B_SUFFIX),
			bombadil_server_bin_dir(self.variant.name),
			bombadil_server_cargo_dir(self.variant.name),
			bombadil_vite_cache_dir(BOMBADIL_MODE_PROD, self.variant.name),
		] {
			remove_dir_all_if_exists(&path, "successful production generated artifacts")?;
		}
		Ok(())
	}

	fn dev_url(&self, app_base_url: &str, raw_url: &str) -> String {
		if raw_url.starts_with('/') {
			return format!("{}{}", app_base_url.trim_end_matches('/'), raw_url);
		}
		raw_url.to_owned()
	}

	fn wait_for_http_body_contains(
		&self,
		child: &mut Child,
		raw_url: &str,
		needle: &str,
		label: &str,
		timeout: Duration,
	) -> Result<(), String> {
		let deadline = Instant::now() + timeout;
		let mut last_error = String::new();
		while Instant::now() < deadline {
			if let Some(status) = child.try_wait().map_err(|error| error.to_string())? {
				return Err(format!(
					"{} dev server exited during {label}: {status}",
					self.variant.name,
				));
			}

			match http_get(raw_url) {
				Ok(response) if response.status == 200 && response.body.contains(needle) => {
					self.log(&format!("{label} observed"));
					return Ok(());
				}
				Ok(response) => {
					last_error = format!(
						"status {}, body did not contain {needle:?}",
						response.status
					);
				}
				Err(error) => {
					last_error = error;
				}
			}
			thread::sleep(DEV_CHANGE_POLL_INTERVAL);
		}
		Err(format!("{label} did not settle: {last_error}"))
	}

	fn log(&self, message: &str) {
		eprintln!("[{}] {message}", self.variant.name);
	}

	fn stop_server(&self, server: &mut Child) {
		if server.try_wait().ok().flatten().is_some() {
			return;
		}
		self.signal_process_group(server, libc::SIGKILL);
		let _ = server.wait();
	}

	fn stop_dev_server(&self, server: &mut Child) {
		if server.try_wait().ok().flatten().is_some() {
			return;
		}
		self.signal_process_group(server, libc::SIGINT);
		let deadline = Instant::now() + DEV_SERVER_SHUTDOWN_TIMEOUT;
		while Instant::now() < deadline {
			if server.try_wait().ok().flatten().is_some() {
				return;
			}
			thread::sleep(DEV_SERVER_SHUTDOWN_POLL_INTERVAL);
		}
		self.signal_process_group(server, libc::SIGKILL);
		let _ = server.wait();
	}

	fn set_process_group(&self, cmd: &mut Command) {
		set_process_group(cmd);
	}

	fn signal_process_group(&self, server: &mut Child, signal: libc::c_int) {
		signal_process_group(server, signal);
	}
}

fn framework_root() -> PathBuf {
	PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn bombadil_artifact_path(path: impl AsRef<Path>) -> PathBuf {
	framework_root().join(BOMBADIL_ARTIFACTS_DIR).join(path)
}

fn bombadil_logs_dir() -> PathBuf {
	framework_root().join(BOMBADIL_LOGS_DIR)
}

fn bombadil_server_bin_dir(variant_name: &str) -> PathBuf {
	framework_root()
		.join(BOMBADIL_SERVER_BIN_DIR)
		.join(variant_name)
}

fn bombadil_dev_cargo_dir(variant_name: &str) -> PathBuf {
	framework_root()
		.join(BOMBADIL_DEV_CARGO_DIR)
		.join(variant_name)
}

fn bombadil_server_cargo_dir(variant_name: &str) -> PathBuf {
	framework_root()
		.join(BOMBADIL_SERVER_CARGO_DIR)
		.join(variant_name)
}

fn bombadil_vite_cache_dir(mode: &str, variant_name: &str) -> PathBuf {
	framework_root()
		.join(BOMBADIL_VITE_CACHE_DIR)
		.join(format!("{mode}-{variant_name}"))
}

fn framework_prod_dist_dir(variant_name: &str, deployment_suffix: &str) -> PathBuf {
	framework_root().join(format!(
		"{FRAMEWORK_DIST_DIR_PREFIX}.{variant_name}.{deployment_suffix}"
	))
}

fn framework_dev_dist_dir(variant_name: &str) -> PathBuf {
	framework_root().join(format!(
		"{FRAMEWORK_DIST_DIR_PREFIX}.{variant_name}.{BOMBADIL_MODE_DEV}.{DEPLOYMENT_A_SUFFIX}"
	))
}

fn remove_dir_all_if_exists(path: &Path, label: &str) -> Result<(), String> {
	fs::remove_dir_all(path)
		.or_else(|error| {
			if error.kind() == io::ErrorKind::NotFound {
				return Ok(());
			}
			Err(error)
		})
		.map_err(|error| format!("error removing {label} at {}: {error}", path.display()))
}

fn parse_test_flags(args: &[String]) -> Result<(u64, Option<String>), String> {
	let mut intensity = 1;
	let mut variant = None;
	let mut idx = 0;
	while idx < args.len() {
		match args[idx].as_str() {
			"-intensity" => {
				idx += 1;
				let Some(raw) = args.get(idx) else {
					return Err("-intensity requires a value".to_owned());
				};
				intensity = raw.parse::<u64>().map_err(|error| error.to_string())?;
				if intensity == 0 {
					return Err("-intensity must be at least 1".to_owned());
				}
			}
			"-variant" => {
				idx += 1;
				let Some(raw) = args.get(idx) else {
					return Err("-variant requires a value".to_owned());
				};
				variant = Some(raw.clone());
			}
			arg => return Err(format!("unknown flag {arg:?}")),
		}
		idx += 1;
	}
	Ok((intensity, variant))
}

fn status_result(status: std::process::ExitStatus) -> Result<(), String> {
	if status.success() {
		return Ok(());
	}
	Err(format!("process exited with status {status}"))
}

fn set_noninteractive_package_manager_env(cmd: &mut Command) {
	cmd.env(
		PACKAGE_MANAGER_NONINTERACTIVE_ENV_KEY,
		PACKAGE_MANAGER_NONINTERACTIVE_ENV_VALUE,
	);
}

struct HttpResponse {
	status: u16,
	body: String,
}

struct FileRestoreGuard {
	path: PathBuf,
	original: String,
}

impl Drop for FileRestoreGuard {
	fn drop(&mut self) {
		let _ = fs::write(&self.path, &self.original);
	}
}

fn replace_file_text(path: &Path, from: &str, to: &str) -> Result<FileRestoreGuard, String> {
	let original =
		fs::read_to_string(path).map_err(|error| format!("read {}: {error}", path.display()))?;
	if !original.contains(from) {
		return Err(format!("{} did not contain {from:?}", path.display()));
	}
	let next = original.replacen(from, to, 1);
	fs::write(path, next).map_err(|error| format!("write {}: {error}", path.display()))?;
	Ok(FileRestoreGuard {
		path: path.to_path_buf(),
		original,
	})
}

fn append_file_text(path: &Path, suffix: &str) -> Result<FileRestoreGuard, String> {
	let original =
		fs::read_to_string(path).map_err(|error| format!("read {}: {error}", path.display()))?;
	let mut next = original.clone();
	next.push_str(suffix);
	fs::write(path, next).map_err(|error| format!("write {}: {error}", path.display()))?;
	Ok(FileRestoreGuard {
		path: path.to_path_buf(),
		original,
	})
}

fn ensure_port_available(port: u16) -> Result<(), String> {
	match TcpStream::connect(("127.0.0.1", port)) {
		Ok(_) => Err(format!(
			"cannot start fixture server on 127.0.0.1:{port}: port is already in use",
		)),
		Err(error) if error.kind() == ErrorKind::ConnectionRefused => Ok(()),
		Err(_) => Ok(()),
	}
}

fn http_get_ok_enough(url: &str) -> bool {
	matches!(http_get_status(url), Some(200..=499))
}

fn http_get_status(url: &str) -> Option<u16> {
	http_get(url).ok().map(|response| response.status)
}

fn http_get(url: &str) -> Result<HttpResponse, String> {
	let response = ureq::get(url)
		.call()
		.map_err(|error| format!("GET {url}: {error}"))?;
	let status = response.status().as_u16();
	let body = response
		.into_body()
		.read_to_string()
		.map_err(|error| format!("read GET {url} response body: {error}"))?;
	Ok(HttpResponse { status, body })
}

fn print_file_to_stderr(path: &Path) {
	let Ok(mut file) = fs::File::open(path) else {
		return;
	};
	let _ = file.seek(SeekFrom::Start(0));
	let _ = io::copy(&mut file, &mut io::stderr());
}

fn read_log_tail(path: &Path, max_bytes: u64) -> Result<String, String> {
	let mut file =
		fs::File::open(path).map_err(|error| format!("open {}: {error}", path.display()))?;
	let len = file
		.metadata()
		.map_err(|error| format!("stat {}: {error}", path.display()))?
		.len();
	file.seek(SeekFrom::Start(len.saturating_sub(max_bytes)))
		.map_err(|error| format!("seek {}: {error}", path.display()))?;
	let mut data = String::new();
	file.read_to_string(&mut data)
		.map_err(|error| format!("read {}: {error}", path.display()))?;
	Ok(data)
}

#[cfg(unix)]
fn set_process_group(cmd: &mut Command) {
	use std::os::unix::process::CommandExt;

	cmd.process_group(0);
}

#[cfg(not(unix))]
fn set_process_group(_: &mut Command) {}

#[cfg(unix)]
fn signal_process_group(server: &mut Child, signal: libc::c_int) {
	let pid = server.id() as libc::pid_t;
	unsafe {
		if libc::kill(-pid, signal) == 0 {
			return;
		}
	}
	let _ = server.kill();
}

#[cfg(not(unix))]
fn signal_process_group(server: &mut Child, _: libc::c_int) {
	let _ = server.kill();
}

#[cfg(test)]
mod tests {
	use std::ffi::OsStr;

	use super::*;

	#[test]
	fn harness_artifacts_are_rooted_in_framework_fixture() {
		let artifact = bombadil_artifact_path("react");
		assert!(artifact.is_absolute());
		assert!(artifact.starts_with(framework_root()));
		assert!(artifact.ends_with(Path::new(".bombadil").join("react")));
	}

	#[test]
	fn harness_server_artifacts_do_not_use_vorma_output_namespace() {
		for path in [
			bombadil_server_bin_dir("react"),
			bombadil_dev_cargo_dir("react"),
			bombadil_server_cargo_dir("react"),
			bombadil_vite_cache_dir(BOMBADIL_MODE_DEV, "react"),
		] {
			assert!(path.is_absolute());
			assert!(path.starts_with(framework_root().join(".bombadil")));
			assert!(
				!path
					.components()
					.any(|component| component.as_os_str() == OsStr::new(".vorma")),
				"harness-owned server artifact path must not use Vorma's .vorma namespace: {}",
				path.display(),
			);
		}
	}

	#[test]
	fn framework_dist_artifacts_are_rooted_in_framework_fixture() {
		for path in [
			framework_prod_dist_dir("react", DEPLOYMENT_A_SUFFIX),
			framework_prod_dist_dir("react", DEPLOYMENT_B_SUFFIX),
			framework_dev_dist_dir("react"),
		] {
			assert!(path.is_absolute());
			assert!(path.starts_with(framework_root()));
			assert_eq!(path.parent(), Some(framework_root().as_path()));
		}
		assert!(framework_prod_dist_dir("react", DEPLOYMENT_A_SUFFIX).ends_with(".dist.react.a"));
		assert!(framework_prod_dist_dir("react", DEPLOYMENT_B_SUFFIX).ends_with(".dist.react.b"));
		assert!(framework_dev_dist_dir("react").ends_with(".dist.react.dev.a"));
	}

	#[test]
	fn test_flags_reject_zero_intensity() {
		assert!(parse_test_flags(&["-intensity".to_owned(), "0".to_owned()]).is_err());
	}

	#[test]
	fn dev_change_probe_intervals_stay_below_local_latency_budget() {
		assert!(PROD_SERVER_READY_POLL_INTERVAL <= Duration::from_millis(25));
		assert!(PROD_SERVER_READY_STABILITY_INTERVAL <= Duration::from_millis(10));
		assert!(DEV_SERVER_READY_POLL_INTERVAL <= Duration::from_millis(25));
		assert!(DEV_SERVER_SHUTDOWN_POLL_INTERVAL <= Duration::from_millis(25));
		assert!(DEV_CHANGE_POLL_INTERVAL <= Duration::from_millis(25));
	}

	#[test]
	fn dev_change_timeouts_are_contract_specific() {
		assert!(DEV_CLIENT_MODULE_CHANGE_TIMEOUT < DEV_SERVER_REBUILD_TIMEOUT);
		assert!(DEV_SERVER_READY_TIMEOUT < DEV_SERVER_REBUILD_TIMEOUT);
		assert!(DEV_SERVER_SHUTDOWN_TIMEOUT < DEV_CLIENT_MODULE_CHANGE_TIMEOUT);
	}
}
