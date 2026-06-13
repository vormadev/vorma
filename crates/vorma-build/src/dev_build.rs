//! Complete development build orchestration.

use std::collections::{BTreeMap, BTreeSet};
use std::net::TcpListener;
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant};

use vorma::build_interface::AppBuildContract;
use vorma::build_interface::assets::ManifestMode;
use vorma_contract::constants::DEV_HEALTH_PATH;

use crate::app_server_process::{
	AppServerDevInput, AppServerProcessError, PrebuiltAppServer, start_app_server_dev,
	start_app_server_dev_with_prebuilt,
};
use crate::build_output::{
	BuildOutputWriteError, TypeScriptOutputWriteReport, generated_typescript_output_path,
	generation_candidate_output_paths, write_file_atomically, write_typescript_contracts,
};
use crate::build_plan::BuildProjectionPlan;
use crate::dev_generation::{
	DevGenerationError, DevOutputPublishReport, DevRuntimeInputs, PreparedDevGeneration,
	prepare_dev_generation_from_build_inputs, prepare_dev_generation_from_committed_static_outputs,
	prepare_dev_generation_from_static_outputs, publish_and_activate_prepared_dev_generation,
	publish_cached_static_candidate_outputs, publish_prepared_candidate_outputs,
};
use crate::dev_mux::{DevMuxError, DevMuxServer, start_loopback_dev_mux_server};
#[cfg(test)]
use crate::dev_refresh::DevRefreshClientSubscription;
use crate::dev_refresh::{ChangeType, RefreshPayload, refresh_payloads_from_effects};
use crate::dev_vite::{DevViteError, ViteDevServerInput, start_vite_dev_server};
use crate::generation_epoch::{CommittedGeneration, EpochSupervisor, GenerationCandidate};
use crate::generation_inputs::{
	BuildInputError, PreparedBuildInputs, prepare_build_inputs, prepare_build_inputs_from_graph,
};
use crate::live_state::{LiveBuildState, LiveBuildStateError};
use crate::output_lock::{OutputLayoutLock, OutputLayoutLockError};
use crate::process_ready::{
	ProcessReadyError, wait_for_loopback_http_ready, wait_for_loopback_http_ready_until_cancelled,
};
use crate::process_runner::{
	BuildProcessCancel, BuildProcessRunner, StartedBuildProcess, StdBuildProcessRunner,
};
use crate::projection_compiler::ProjectionBundle;
use crate::static_outputs::{
	PreparedPublicStaticOutputs, bundle_critical_css, public_static_output_dir,
};
use crate::tokens::{TokenGenerationError, generate_dev_refresh_token, generate_vite_plugin_token};
use crate::typescript_contracts::{GeneratedTypeScriptContracts, render_typescript_contracts};
use crate::vite_plugin_contract::VITE_PLUGIN_LOOPBACK_HOST;
use crate::vite_plugin_contract::VitePluginConfig;
use crate::vite_plugin_control::{
	LoopbackVitePluginControlClient, VitePluginControlClient, VitePluginControlError,
};
use crate::vite_plugin_rpc::{
	LoopbackVitePluginRpcServer, VitePluginRpcServer, VitePluginRpcServerHandle,
	VitePluginRpcState, VitePluginServerError,
};

/// Dev child-process display name for the app server.
const DEV_APP_SERVER_PROCESS_NAME: &str = "app server";
/// Dev child-process display name for the Vite server.
const DEV_VITE_PROCESS_NAME: &str = "Vite server";
const VITE_CONTROL_PORT_WAIT_TIMEOUT: Duration = Duration::from_secs(10);
const VITE_CONTROL_PORT_WAIT_INTERVAL: Duration = Duration::from_millis(10);
const DEV_PROCESS_READY_TIMEOUT: Duration = Duration::from_secs(90);
const VITE_READY_PATH: &str = "/@vite/client";

/// Scalar inputs for starting one complete dev generation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevGenerationStartConfig {
	dev_mux_port: u16,
	app_server_port: u16,
	vite_server_port: u16,
	vite_plugin_token: String,
	dev_refresh_token: String,
}

impl DevGenerationStartConfig {
	/// Create dev generation startup inputs.
	pub fn new(
		dev_mux_port: u16,
		app_server_port: u16,
		vite_server_port: u16,
		vite_plugin_token: impl Into<String>,
		dev_refresh_token: impl Into<String>,
	) -> Self {
		Self {
			dev_mux_port,
			app_server_port,
			vite_server_port,
			vite_plugin_token: vite_plugin_token.into(),
			dev_refresh_token: dev_refresh_token.into(),
		}
	}
}

/// Complete dev generation start report.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevBuildReport {
	prebuild_typescript: TypeScriptOutputWriteReport,
	output_publish: DevOutputPublishReport,
	vite_server_port: u16,
	vite_plugin_control_port: u16,
	app_server_port: u16,
	dev_mux_port: u16,
}

impl DevBuildReport {
	/// Generated TypeScript written before Vite started.
	#[cfg(test)]
	pub fn prebuild_typescript(&self) -> &TypeScriptOutputWriteReport {
		&self.prebuild_typescript
	}

	/// Final static/runtime output publication report.
	#[cfg(test)]
	pub fn output_publish(&self) -> &DevOutputPublishReport {
		&self.output_publish
	}

	/// Vite dev server port reported by the Vite plugin.
	#[cfg(test)]
	pub fn vite_server_port(&self) -> u16 {
		self.vite_server_port
	}

	/// Vite plugin control server port reported by the Vite plugin.
	#[cfg(test)]
	pub fn vite_plugin_control_port(&self) -> u16 {
		self.vite_plugin_control_port
	}

	/// App server port.
	#[cfg(test)]
	pub fn app_server_port(&self) -> u16 {
		self.app_server_port
	}

	/// Dev mux server port recorded in the runtime manifest.
	pub fn dev_mux_port(&self) -> u16 {
		self.dev_mux_port
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum DevStaticGenerationKind {
	CriticalCss,
	PublicStaticAndCriticalCss,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct DevStaticGenerationUpdate {
	kind: DevStaticGenerationKind,
	client_revalidate_required: bool,
}

impl DevStaticGenerationUpdate {
	pub(crate) const fn new(
		kind: DevStaticGenerationKind,
		client_revalidate_required: bool,
	) -> Self {
		Self {
			kind,
			client_revalidate_required,
		}
	}

	pub(crate) const fn kind(self) -> DevStaticGenerationKind {
		self.kind
	}

	pub(crate) const fn client_revalidate_required(self) -> bool {
		self.client_revalidate_required
	}
}

/// Dev generation update report.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevGenerationUpdateReport {
	output_publish: DevOutputPublishReport,
	refresh_payloads: Vec<RefreshPayload>,
	retired_app_server_termination_error: Option<String>,
}

impl DevGenerationUpdateReport {
	/// Final static/runtime output publication report.
	#[cfg(test)]
	pub fn output_publish(&self) -> &DevOutputPublishReport {
		&self.output_publish
	}

	/// Refresh payloads broadcast to connected dev browser clients.
	pub fn refresh_payloads(&self) -> &[RefreshPayload] {
		&self.refresh_payloads
	}

	/// Process termination error for the retired app server after the new generation
	/// became active.
	pub fn retired_app_server_termination_error(&self) -> Option<&str> {
		self.retired_app_server_termination_error.as_deref()
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct PublishedDevGenerationUpdateCandidate {
	candidate: GenerationCandidate,
	prebuild_typescript: TypeScriptOutputWriteReport,
	output_publish: DevOutputPublishReport,
	vite_plugin_config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
	critical_css_imports: Vec<String>,
	orphaned_output_layout: Option<PathBuf>,
	disk_rollback: DevDiskRollback,
}

struct StartedCandidateAppServer {
	port: u16,
	process: Box<dyn StartedBuildProcess + Send>,
}

struct PreparedDevGenerationUpdateOutputs {
	prebuild_typescript: TypeScriptOutputWriteReport,
	prepared: PreparedDevGeneration,
	vite_plugin_config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
	critical_css_imports: Vec<String>,
	disk_rollback: DevDiskRollback,
}

struct PreparedCachedStaticDevGenerationUpdateOutputs {
	prebuild_typescript: TypeScriptOutputWriteReport,
	typescript_contracts: GeneratedTypeScriptContracts,
	vite_plugin_config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
	disk_rollback: DevDiskRollback,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
struct DevDiskRollback {
	files: BTreeMap<PathBuf, DevDiskRollbackFile>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum DevDiskRollbackFile {
	Existing(Vec<u8>),
	Missing,
}

impl DevDiskRollback {
	fn capture_path(&mut self, path: PathBuf) -> Result<(), DevDiskRollbackError> {
		if self.files.contains_key(&path) {
			return Ok(());
		}
		let state = match std::fs::read(&path) {
			Ok(bytes) => DevDiskRollbackFile::Existing(bytes),
			Err(source) if source.kind() == std::io::ErrorKind::NotFound => {
				DevDiskRollbackFile::Missing
			}
			Err(source) => {
				return Err(DevDiskRollbackError::Capture {
					path: path.display().to_string(),
					message: source.to_string(),
				});
			}
		};
		self.files.insert(path, state);
		Ok(())
	}

	fn capture_generated_typescript(
		&mut self,
		plan: &BuildProjectionPlan,
	) -> Result<(), DevBuildError> {
		let path = generated_typescript_output_path(plan)
			.map_err(|source| DevBuildError::TypeScriptOutput { source })?;
		self.capture_path(path)
			.map_err(|source| DevBuildError::DiskSnapshot { source })
	}

	fn capture_candidate_outputs(
		&mut self,
		candidate: &GenerationCandidate,
		prepared_public_static_outputs: &PreparedPublicStaticOutputs,
	) -> Result<(), DevBuildError> {
		self.capture_generation_outputs(candidate)?;
		let public_output_dir =
			public_static_output_dir(candidate.build_plan()).map_err(|source| {
				DevBuildError::DevGeneration {
					source: DevGenerationError::PublicStatic { source },
				}
			})?;
		for file in prepared_public_static_outputs.files() {
			self.capture_path(public_output_dir.join(file.output_name()))
				.map_err(|source| DevBuildError::DiskSnapshot { source })?;
		}
		Ok(())
	}

	fn capture_generation_outputs(
		&mut self,
		candidate: &GenerationCandidate,
	) -> Result<(), DevBuildError> {
		let output_paths = generation_candidate_output_paths(candidate, ManifestMode::Dev)
			.map_err(|source| DevBuildError::TypeScriptOutput { source })?;
		self.capture_path(output_paths.generated_typescript_path().to_path_buf())
			.map_err(|source| DevBuildError::DiskSnapshot {
				source: source.clone(),
			})?;
		self.capture_path(output_paths.manifest_path().to_path_buf())
			.map_err(|source| DevBuildError::DiskSnapshot { source })?;
		Ok(())
	}

	fn restore(self) -> Result<(), DevDiskRollbackError> {
		for (path, file) in self.files {
			match file {
				DevDiskRollbackFile::Existing(bytes) => {
					write_file_atomically(&path, &bytes).map_err(|source| {
						DevDiskRollbackError::Restore {
							path: path.display().to_string(),
							message: source.to_string(),
						}
					})?;
				}
				DevDiskRollbackFile::Missing => match std::fs::symlink_metadata(&path) {
					Ok(metadata) if metadata.is_dir() => {
						std::fs::remove_dir_all(&path).map_err(|source| {
							DevDiskRollbackError::Restore {
								path: path.display().to_string(),
								message: source.to_string(),
							}
						})?;
					}
					Ok(_) => {
						std::fs::remove_file(&path).map_err(|source| {
							DevDiskRollbackError::Restore {
								path: path.display().to_string(),
								message: source.to_string(),
							}
						})?;
					}
					Err(source) if source.kind() == std::io::ErrorKind::NotFound => {}
					Err(source) => {
						return Err(DevDiskRollbackError::Restore {
							path: path.display().to_string(),
							message: source.to_string(),
						});
					}
				},
			}
		}
		Ok(())
	}
}

/// Dev disk rollback error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevDiskRollbackError {
	/// Existing file state could not be captured.
	Capture {
		/// File path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Captured file state could not be restored.
	Restore {
		/// File path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
}

impl std::fmt::Display for DevDiskRollbackError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Capture { path, message } => {
				write!(f, "capture dev output rollback file {path}: {message}")
			}
			Self::Restore { path, message } => {
				write!(f, "restore dev output rollback file {path}: {message}")
			}
		}
	}
}

impl std::error::Error for DevDiskRollbackError {}

/// Started dev generation and owned long-lived process handles.
pub struct StartedDevGeneration {
	supervisor: EpochSupervisor,
	committed_generation: CommittedGeneration,
	critical_css_import_paths: Vec<String>,
	child_exit_notifier: Option<std::sync::Arc<dyn Fn(&'static str) + Send + Sync>>,
	report: DevBuildReport,
	vite_plugin_rpc_state: VitePluginRpcState,
	vite_plugin_token: String,
	vite_plugin_control_client: Box<dyn VitePluginControlClient + Send>,
	dev_mux_server: DevMuxServer,
	app_server_process: Box<dyn StartedBuildProcess + Send>,
	vite_process: Box<dyn StartedBuildProcess + Send>,
	/*
	Held for ownership only: dropping the handle shuts the RPC server down with
	the generation.
	*/
	_vite_plugin_rpc_server: VitePluginRpcServerHandle,
	/*
	Declared last so the layout stays locked until every child process above
	has been terminated by the preceding fields' drops.
	*/
	output_layout_lock: OutputLayoutLock,
}

impl StartedDevGeneration {
	/// Canonical critical-CSS source files read by the committed generation's bundle.
	pub fn critical_css_import_paths(&self) -> &[String] {
		&self.critical_css_import_paths
	}

	/*
	Registered on the current children immediately and on every replacement
	app server at activation, so a dead child is reported for the whole dev
	session, not just the first generation (Go parity: supervisor
	on_unexpected_exit).
	*/
	/// Report unexpected child-process exits to `notifier` for this session.
	pub fn watch_child_exits(
		&mut self,
		notifier: std::sync::Arc<dyn Fn(&'static str) + Send + Sync>,
	) {
		self.app_server_process.watch_unexpected_exit(Box::new({
			let notifier = std::sync::Arc::clone(&notifier);
			move || notifier(DEV_APP_SERVER_PROCESS_NAME)
		}));
		self.vite_process.watch_unexpected_exit(Box::new({
			let notifier = std::sync::Arc::clone(&notifier);
			move || notifier(DEV_VITE_PROCESS_NAME)
		}));
		self.child_exit_notifier = Some(notifier);
	}

	/// Committed generation activated for the dev server.
	pub fn committed_generation(&self) -> &CommittedGeneration {
		&self.committed_generation
	}

	/// Dev start report.
	pub fn report(&self) -> &DevBuildReport {
		&self.report
	}

	/// Vite child process id when available.
	#[cfg(test)]
	pub fn vite_process_id(&self) -> Option<u32> {
		self.vite_process.process_id()
	}

	/// Vite plugin RPC server port.
	#[cfg(test)]
	pub fn vite_plugin_rpc_server_port(&self) -> u16 {
		self._vite_plugin_rpc_server.port()
	}

	/// Subscribe to refresh payloads emitted by this dev generation.
	#[cfg(test)]
	pub fn subscribe_refresh_payloads(&self) -> DevRefreshClientSubscription {
		self.dev_mux_server.add_test_client()
	}

	/// Notify connected dev browsers that a rebuild has started.
	pub fn broadcast_rebuilding(&self) {
		self.dev_mux_server.broadcast(RefreshPayload::new(
			ChangeType::ShowRebuildingOverlay,
			"",
			"",
		));
	}

	/// Notify connected dev browsers that a rebuild failed.
	pub fn broadcast_build_error(&self, message: impl Into<String>) {
		self.dev_mux_server
			.broadcast(RefreshPayload::new(ChangeType::ShowBuildError, "", message));
	}

	/// Notify connected dev browsers to revalidate active route data.
	pub fn broadcast_client_revalidation(&self) {
		self.dev_mux_server
			.broadcast(RefreshPayload::new(ChangeType::ClientRevalidate, "", ""));
	}

	/// Commit the next dev generation from live build-state facts while reusing the live
	/// Vite process and RPC server.
	#[cfg(test)]
	pub fn activate_next_live_state_generation(
		&mut self,
		live_state: LiveBuildState,
		prebuilt_app_server: PrebuiltAppServer,
		process_runner: &mut impl BuildProcessRunner,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		let cancel = BuildProcessCancel::new();
		self.activate_next_live_state_generation_until_cancelled(
			live_state,
			prebuilt_app_server,
			process_runner,
			&cancel,
		)
	}

	/// Commit the next live-state generation unless cancellation interrupts activation.
	pub fn activate_next_live_state_generation_until_cancelled(
		&mut self,
		live_state: LiveBuildState,
		prebuilt_app_server: PrebuiltAppServer,
		process_runner: &mut impl BuildProcessRunner,
		cancel: &BuildProcessCancel,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		let published = build_and_publish_live_dev_generation_update_candidate(
			&self.supervisor,
			&mut self.output_layout_lock,
			live_state,
			DevRuntimeInputs::new(
				i32::from(self.report.vite_server_port),
				i32::from(self.report.dev_mux_port),
				self.committed_generation
					.manifest()
					.dev_refresh_token()
					.to_owned(),
			),
		)?;
		self.activate_published_update_until_cancelled(
			published,
			Some(prebuilt_app_server),
			process_runner,
			cancel,
		)
	}

	pub(crate) fn activate_next_live_state_generation_reusing_committed_static_outputs_until_cancelled(
		&mut self,
		live_state: LiveBuildState,
		prebuilt_app_server: PrebuiltAppServer,
		process_runner: &mut impl BuildProcessRunner,
		cancel: &BuildProcessCancel,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		let current_critical_css_imports = self.critical_css_import_paths.clone();
		let published = build_and_publish_cached_static_live_dev_generation_update_candidate(
			&self.supervisor,
			&mut self.output_layout_lock,
			&self.committed_generation,
			live_state,
			DevRuntimeInputs::new(
				i32::from(self.report.vite_server_port),
				i32::from(self.report.dev_mux_port),
				self.committed_generation
					.manifest()
					.dev_refresh_token()
					.to_owned(),
			),
			current_critical_css_imports,
		)?;
		self.activate_published_update_until_cancelled(
			published,
			Some(prebuilt_app_server),
			process_runner,
			cancel,
		)
	}

	pub(crate) fn activate_next_static_generation(
		&mut self,
		update: DevStaticGenerationUpdate,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		let runtime = DevRuntimeInputs::new(
			i32::from(self.report.vite_server_port),
			i32::from(self.report.dev_mux_port),
			self.committed_generation
				.manifest()
				.dev_refresh_token()
				.to_owned(),
		);
		let published = match update.kind() {
			DevStaticGenerationKind::CriticalCss => {
				build_and_publish_critical_css_dev_generation_update_candidate(
					&self.supervisor,
					&self.committed_generation,
					runtime,
				)?
			}
			DevStaticGenerationKind::PublicStaticAndCriticalCss => {
				build_and_publish_public_static_dev_generation_update_candidate(
					&self.supervisor,
					&self.committed_generation,
					runtime,
				)?
			}
		};
		self.activate_static_published_update(published, update.client_revalidate_required())
	}

	fn activate_published_update_until_cancelled(
		&mut self,
		published: PublishedDevGenerationUpdateCandidate,
		prebuilt_app_server: Option<PrebuiltAppServer>,
		process_runner: &mut impl BuildProcessRunner,
		cancel: &BuildProcessCancel,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		let started_app_server = match self.start_candidate_app_server_until_cancelled(
			published.candidate.build_plan(),
			prebuilt_app_server.as_ref(),
			process_runner,
			cancel,
		) {
			Ok(started_app_server) => started_app_server,
			Err(error) => {
				return Err(rollback_dev_activation_error(
					error,
					published.disk_rollback,
				));
			}
		};
		let previous_vite_plugin_contract = self.vite_plugin_rpc_state.generation_contract();
		self.vite_plugin_rpc_state.update_generation_contract(
			published.vite_plugin_config.clone(),
			published.public_filemap.clone(),
		);
		let vite_config_changed =
			previous_vite_plugin_contract.config() != &published.vite_plugin_config;
		/*
		Restarting Vite is reserved for changes the plugin's config()
		actually consumes; changed assets only invalidate the modules that
		referenced them. The changed-key set is symmetric, so the same
		vector re-syncs the plugin if a later failure restores the previous
		contract.
		*/
		let changed_asset_source_paths = changed_public_source_paths(
			previous_vite_plugin_contract.public_filemap(),
			&published.public_filemap,
		);
		if let Err(error) =
			self.notify_vite_plugin(vite_config_changed, &changed_asset_source_paths)
		{
			self.vite_plugin_rpc_state
				.restore_generation_contract(previous_vite_plugin_contract);
			discard_uncommitted_app_server_process(started_app_server.process);
			return Err(rollback_dev_activation_error(
				error,
				published.disk_rollback,
			));
		}
		if let Err(source) = self
			.dev_mux_server
			.set_active_app_server_port(started_app_server.port)
		{
			self.vite_plugin_rpc_state
				.restore_generation_contract(previous_vite_plugin_contract);
			let _ = self.notify_vite_plugin(vite_config_changed, &changed_asset_source_paths);
			discard_uncommitted_app_server_process(started_app_server.process);
			return Err(rollback_dev_activation_error(
				DevBuildError::DevMux { source },
				published.disk_rollback,
			));
		}

		let critical_css_import_paths = published.critical_css_imports.clone();
		let orphaned_output_layout = published.orphaned_output_layout.clone();
		let (committed_generation, mut update_report) =
			commit_published_dev_generation_update(&mut self.supervisor, published);
		self.committed_generation = committed_generation;
		self.critical_css_import_paths = critical_css_import_paths;
		/*
		Only after the moved-layout generation commits is the previous
		output directory truly orphaned; until then the prior generation
		serves from it (and failed activations roll back onto it).
		*/
		if let Some(orphaned) = orphaned_output_layout {
			let _ = std::fs::remove_dir_all(orphaned);
		}
		let mut retired_app_server_process =
			std::mem::replace(&mut self.app_server_process, started_app_server.process);
		if let Some(notifier) = &self.child_exit_notifier {
			self.app_server_process.watch_unexpected_exit(Box::new({
				let notifier = std::sync::Arc::clone(notifier);
				move || notifier(DEV_APP_SERVER_PROCESS_NAME)
			}));
		}
		if let Err(source) = retired_app_server_process.terminate() {
			update_report.retired_app_server_termination_error = Some(source.to_string());
		}
		self.report.app_server_port = started_app_server.port;
		self.dev_mux_server
			.broadcast_all(update_report.refresh_payloads().iter().cloned());
		Ok(update_report)
	}

	fn activate_static_published_update(
		&mut self,
		published: PublishedDevGenerationUpdateCandidate,
		force_client_revalidation: bool,
	) -> Result<DevGenerationUpdateReport, DevBuildError> {
		/*
		Static fast-path updates cannot change the plugin config (it is
		projected from the unchanged graph), so Vite is never restarted
		here; changed assets only invalidate their referencing modules.
		*/
		let changed_asset_source_paths = changed_public_source_paths(
			self.committed_generation.artifacts().public_filemap(),
			&published.public_filemap,
		);
		let previous_vite_plugin_contract = self.vite_plugin_rpc_state.generation_contract();
		if !changed_asset_source_paths.is_empty() {
			self.vite_plugin_rpc_state.update_generation_contract(
				published.vite_plugin_config.clone(),
				published.public_filemap.clone(),
			);
			if let Err(error) = self.notify_vite_plugin_assets_changed(&changed_asset_source_paths)
			{
				self.vite_plugin_rpc_state
					.restore_generation_contract(previous_vite_plugin_contract);
				return Err(rollback_dev_activation_error(
					error,
					published.disk_rollback,
				));
			}
		}

		let critical_css_import_paths = published.critical_css_imports.clone();
		let orphaned_output_layout = published.orphaned_output_layout.clone();
		let (committed_generation, mut update_report) =
			commit_published_dev_generation_update(&mut self.supervisor, published);
		self.committed_generation = committed_generation;
		self.critical_css_import_paths = critical_css_import_paths;
		/*
		Only after the moved-layout generation commits is the previous
		output directory truly orphaned; until then the prior generation
		serves from it (and failed activations roll back onto it).
		*/
		if let Some(orphaned) = orphaned_output_layout {
			let _ = std::fs::remove_dir_all(orphaned);
		}
		remove_static_fast_path_overlay_cleanup(&mut update_report);
		if force_client_revalidation {
			force_client_revalidation_refresh(&mut update_report);
		}
		self.dev_mux_server
			.broadcast_all(update_report.refresh_payloads().iter().cloned());
		Ok(update_report)
	}

	fn notify_vite_plugin(
		&mut self,
		vite_config_changed: bool,
		changed_asset_source_paths: &[String],
	) -> Result<(), DevBuildError> {
		if vite_config_changed {
			return self.notify_vite_plugin_config_changed();
		}
		if !changed_asset_source_paths.is_empty() {
			return self.notify_vite_plugin_assets_changed(changed_asset_source_paths);
		}
		Ok(())
	}

	fn notify_vite_plugin_config_changed(&mut self) -> Result<(), DevBuildError> {
		let control_port = wait_for_vite_plugin_control_port(&self.vite_plugin_rpc_state)?;
		self.vite_plugin_control_client
			.notify_config_changed(control_port, &self.vite_plugin_token)
			.map_err(|source| DevBuildError::VitePluginControl { source })
	}

	fn notify_vite_plugin_assets_changed(
		&mut self,
		changed_source_paths: &[String],
	) -> Result<(), DevBuildError> {
		let control_port = wait_for_vite_plugin_control_port(&self.vite_plugin_rpc_state)?;
		self.vite_plugin_control_client
			.notify_assets_changed(control_port, &self.vite_plugin_token, changed_source_paths)
			.map_err(|source| DevBuildError::VitePluginControl { source })
	}

	fn start_candidate_app_server_until_cancelled(
		&self,
		plan: &BuildProjectionPlan,
		prebuilt_app_server: Option<&PrebuiltAppServer>,
		process_runner: &mut impl BuildProcessRunner,
		cancel: &BuildProcessCancel,
	) -> Result<StartedCandidateAppServer, DevBuildError> {
		let port = allocate_loopback_port()?;
		let input = AppServerDevInput::new(port);
		let mut process = match prebuilt_app_server {
			Some(prebuilt_app_server) => start_app_server_dev_with_prebuilt(
				plan,
				&input,
				prebuilt_app_server,
				process_runner,
			),
			None => start_app_server_dev(plan, &input, process_runner),
		}
		.map_err(|source| DevBuildError::AppServer { source })?;
		let ready_result = wait_for_loopback_http_ready_until_cancelled(
			process.as_mut(),
			port,
			DEV_HEALTH_PATH,
			DEV_PROCESS_READY_TIMEOUT,
			cancel,
		);
		if let Err(source) = ready_result {
			let _ = process.terminate();
			return Err(DevBuildError::ProcessReady { source });
		}
		Ok(StartedCandidateAppServer { port, process })
	}
}

impl Drop for StartedDevGeneration {
	fn drop(&mut self) {
		let _ = self.app_server_process.terminate();
		let _ = self.vite_process.terminate();
	}
}

/// Start a complete dev generation using Vorma's standard loopback Vite plugin server.
pub async fn start_and_activate_dev_generation_on_loopback(
	app: AppBuildContract,
) -> Result<StartedDevGeneration, DevBuildError> {
	let dev_mux_port = allocate_loopback_port()?;
	let vite_server_port = allocate_loopback_port()?;
	let app_server_port = allocate_loopback_port()?;
	let vite_plugin_token =
		generate_vite_plugin_token().map_err(|source| DevBuildError::TokenGeneration { source })?;
	let dev_refresh_token =
		generate_dev_refresh_token().map_err(|source| DevBuildError::TokenGeneration { source })?;
	let mut vite_plugin_server = LoopbackVitePluginRpcServer;
	let mut process_runner = StdBuildProcessRunner;
	let vite_plugin_control_client = Box::new(LoopbackVitePluginControlClient);
	start_and_activate_dev_generation(
		EpochSupervisor::default(),
		app,
		DevGenerationStartConfig::new(
			dev_mux_port,
			app_server_port,
			vite_server_port,
			vite_plugin_token,
			dev_refresh_token,
		),
		&mut vite_plugin_server,
		&mut process_runner,
		vite_plugin_control_client,
	)
	.await
}

/// Start a complete dev generation and keep long-lived dev processes owned by the result.
pub async fn start_and_activate_dev_generation(
	mut supervisor: EpochSupervisor,
	app: AppBuildContract,
	start_config: DevGenerationStartConfig,
	vite_plugin_server: &mut impl VitePluginRpcServer,
	process_runner: &mut impl BuildProcessRunner,
	vite_plugin_control_client: Box<dyn VitePluginControlClient + Send>,
) -> Result<StartedDevGeneration, DevBuildError> {
	let vite_plugin_token = start_config.vite_plugin_token;
	if vite_plugin_token.is_empty() {
		return Err(DevBuildError::EmptyVitePluginToken);
	}
	let dev_refresh_token = start_config.dev_refresh_token;
	if dev_refresh_token.is_empty() {
		return Err(DevBuildError::EmptyDevRefreshToken);
	}
	if start_config.dev_mux_port == 0 {
		return Err(DevBuildError::InvalidDevMuxPort);
	}
	if start_config.vite_server_port == 0 {
		return Err(DevBuildError::InvalidViteServerPort);
	}
	if start_config.app_server_port == 0 {
		return Err(DevBuildError::InvalidAppServerPort);
	}
	let dev_mux_server = start_loopback_dev_mux_server(
		start_config.dev_mux_port,
		start_config.app_server_port,
		&dev_refresh_token,
	)
	.map_err(|source| DevBuildError::DevMux { source })?;

	let build_inputs =
		prepare_build_inputs(&app).map_err(|source| DevBuildError::BuildInputs { source })?;
	let output_layout_lock = OutputLayoutLock::acquire_for_workspace_output_layout(
		build_inputs.plan().workspace().root_dir(),
		build_inputs.plan().workspace().dist_dir(),
	)
	.map_err(|source| DevBuildError::OutputLayoutLock { source })?;
	let prebuild_typescript =
		write_typescript_contracts(build_inputs.plan(), build_inputs.typescript_contracts())
			.map_err(|source| DevBuildError::TypeScriptOutput { source })?;
	let rpc_state =
		VitePluginRpcState::from_prepared_build_inputs(&vite_plugin_token, &build_inputs);
	let vite_plugin_rpc_server = vite_plugin_server
		.start_vite_plugin_rpc_server(rpc_state.clone())
		.map_err(|source| DevBuildError::VitePluginServer { source })?;
	let mut vite_process = start_vite_dev_server(
		build_inputs.plan(),
		&ViteDevServerInput::new(
			start_config.vite_server_port,
			vite_plugin_rpc_server.port(),
			&vite_plugin_token,
		),
		process_runner,
	)
	.map_err(|source| DevBuildError::Vite { source })?;
	let vite_plugin_control_port = wait_for_vite_plugin_control_port(&rpc_state)?;
	wait_for_loopback_http_ready(
		vite_process.as_mut(),
		start_config.vite_server_port,
		VITE_READY_PATH,
		DEV_PROCESS_READY_TIMEOUT,
	)
	.map_err(|source| DevBuildError::ProcessReady { source })?;
	let critical_css_import_paths = build_inputs.critical_css().imports().to_vec();
	let prepared = prepare_dev_generation_from_build_inputs(
		build_inputs,
		DevRuntimeInputs::new(
			i32::from(start_config.vite_server_port),
			i32::from(start_config.dev_mux_port),
			dev_refresh_token,
		),
	)
	.map_err(|source| DevBuildError::DevGeneration { source })?;
	let (committed_generation, output_publish) =
		publish_and_activate_prepared_dev_generation(&mut supervisor, app, &prepared)
			.await
			.map_err(|source| DevBuildError::DevGeneration { source })?;
	let committed_generation = committed_generation.clone();
	let mut app_server_process = start_app_server_dev(
		committed_generation.build_plan(),
		&AppServerDevInput::new(start_config.app_server_port),
		process_runner,
	)
	.map_err(|source| DevBuildError::AppServer { source })?;
	wait_for_loopback_http_ready(
		app_server_process.as_mut(),
		start_config.app_server_port,
		DEV_HEALTH_PATH,
		DEV_PROCESS_READY_TIMEOUT,
	)
	.map_err(|source| DevBuildError::ProcessReady { source })?;
	Ok(StartedDevGeneration {
		supervisor,
		committed_generation,
		critical_css_import_paths,
		child_exit_notifier: None,
		output_layout_lock,
		report: DevBuildReport {
			prebuild_typescript,
			output_publish,
			vite_server_port: start_config.vite_server_port,
			vite_plugin_control_port,
			app_server_port: start_config.app_server_port,
			dev_mux_port: start_config.dev_mux_port,
		},
		vite_plugin_rpc_state: rpc_state,
		vite_plugin_token,
		vite_plugin_control_client,
		dev_mux_server,
		app_server_process,
		vite_process,
		_vite_plugin_rpc_server: vite_plugin_rpc_server,
	})
}

fn prepare_dev_generation_update_outputs(
	build_inputs: PreparedBuildInputs,
	runtime: DevRuntimeInputs,
) -> Result<PreparedDevGenerationUpdateOutputs, DevBuildError> {
	let mut disk_rollback = DevDiskRollback::default();
	disk_rollback.capture_generated_typescript(build_inputs.plan())?;
	let prebuild_typescript =
		write_typescript_contracts(build_inputs.plan(), build_inputs.typescript_contracts())
			.map_err(|source| DevBuildError::TypeScriptOutput { source })?;
	let vite_plugin_config = build_inputs.vite_plugin_config().clone();
	let public_filemap = build_inputs
		.public_static_outputs()
		.public_filemap()
		.clone();
	let critical_css_imports = build_inputs.critical_css().imports().to_vec();
	let prepared = prepare_dev_generation_from_build_inputs(build_inputs, runtime)
		.map_err(|source| DevBuildError::DevGeneration { source })?;
	Ok(PreparedDevGenerationUpdateOutputs {
		prebuild_typescript,
		prepared,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		disk_rollback,
	})
}

fn publish_prepared_dev_generation_update_candidate(
	candidate: GenerationCandidate,
	prepared_outputs: PreparedDevGenerationUpdateOutputs,
	orphaned_output_layout: Option<PathBuf>,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	let PreparedDevGenerationUpdateOutputs {
		prebuild_typescript,
		prepared,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		mut disk_rollback,
	} = prepared_outputs;
	disk_rollback.capture_candidate_outputs(&candidate, prepared.public_static_outputs())?;
	let output_publish = publish_prepared_candidate_outputs(&candidate, &prepared)
		.map_err(|source| DevBuildError::DevGeneration { source })?;
	Ok(PublishedDevGenerationUpdateCandidate {
		candidate,
		prebuild_typescript,
		output_publish,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		orphaned_output_layout,
		disk_rollback,
	})
}

fn prepare_cached_static_dev_generation_update_outputs(
	bundle: &ProjectionBundle,
	plan: &BuildProjectionPlan,
	public_filemap: &BTreeMap<String, String>,
) -> Result<PreparedCachedStaticDevGenerationUpdateOutputs, DevBuildError> {
	let mut disk_rollback = DevDiskRollback::default();
	disk_rollback.capture_generated_typescript(plan)?;
	let typescript_contracts =
		render_typescript_contracts(bundle, public_filemap).map_err(|source| {
			DevBuildError::BuildInputs {
				source: BuildInputError::TypeScriptContracts { source },
			}
		})?;
	let prebuild_typescript = write_typescript_contracts(plan, &typescript_contracts)
		.map_err(|source| DevBuildError::TypeScriptOutput { source })?;
	let vite_plugin_config =
		VitePluginConfig::from_build_plan(plan).map_err(|source| DevBuildError::BuildInputs {
			source: BuildInputError::VitePluginConfig { source },
		})?;
	Ok(PreparedCachedStaticDevGenerationUpdateOutputs {
		prebuild_typescript,
		typescript_contracts,
		vite_plugin_config,
		public_filemap: public_filemap.clone(),
		disk_rollback,
	})
}

fn publish_cached_static_dev_generation_update_candidate(
	candidate: GenerationCandidate,
	prebuild_typescript: TypeScriptOutputWriteReport,
	vite_plugin_config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
	critical_css_imports: Vec<String>,
	orphaned_output_layout: Option<PathBuf>,
	mut disk_rollback: DevDiskRollback,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	disk_rollback.capture_generation_outputs(&candidate)?;
	let output_publish = publish_cached_static_candidate_outputs(&candidate)
		.map_err(|source| DevBuildError::DevGeneration { source })?;
	Ok(PublishedDevGenerationUpdateCandidate {
		candidate,
		prebuild_typescript,
		output_publish,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		orphaned_output_layout,
		disk_rollback,
	})
}

fn build_and_publish_live_dev_generation_update_candidate(
	supervisor: &EpochSupervisor,
	output_layout_lock: &mut OutputLayoutLock,
	live_state: LiveBuildState,
	runtime: DevRuntimeInputs,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	let graph = live_state
		.framework_graph()
		.map_err(|source| DevBuildError::LiveState { source })?;
	let build_inputs = prepare_build_inputs_from_graph(&graph)
		.map_err(|source| DevBuildError::BuildInputs { source })?;
	let orphaned_output_layout = output_layout_lock
		.reacquire_if_output_layout_moved(
			build_inputs.plan().workspace().root_dir(),
			build_inputs.plan().workspace().dist_dir(),
		)
		.map_err(|source| DevBuildError::OutputLayoutLock { source })?;
	let prepared_outputs = prepare_dev_generation_update_outputs(build_inputs, runtime)?;
	let candidate = supervisor
		.build_next_live_graph_candidate_with_precomputed_projections(
			graph,
			prepared_outputs.prepared.projection_bundle().clone(),
			prepared_outputs.prepared.build_plan().clone(),
			prepared_outputs.prepared.typescript_contracts().clone(),
			prepared_outputs.prepared.artifacts().clone(),
			live_state.root_document_hash_source(),
		)
		.map_err(|source| DevBuildError::DevGeneration {
			source: DevGenerationError::Generation { source },
		})?;
	publish_prepared_dev_generation_update_candidate(
		candidate,
		prepared_outputs,
		orphaned_output_layout,
	)
}

fn build_and_publish_public_static_dev_generation_update_candidate(
	supervisor: &EpochSupervisor,
	committed: &CommittedGeneration,
	runtime: DevRuntimeInputs,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	let build_inputs = prepare_build_inputs_from_graph(committed.graph())
		.map_err(|source| DevBuildError::BuildInputs { source })?;
	let prepared_outputs = prepare_dev_generation_update_outputs(build_inputs, runtime)?;
	let candidate = supervisor
		.build_next_live_graph_candidate_with_precomputed_projections(
			committed.graph().clone(),
			prepared_outputs.prepared.projection_bundle().clone(),
			prepared_outputs.prepared.build_plan().clone(),
			prepared_outputs.prepared.typescript_contracts().clone(),
			prepared_outputs.prepared.artifacts().clone(),
			committed.artifacts().root_document_hash_source(),
		)
		.map_err(|source| DevBuildError::DevGeneration {
			source: DevGenerationError::Generation { source },
		})?;
	publish_prepared_dev_generation_update_candidate(candidate, prepared_outputs, None)
}

fn build_and_publish_critical_css_dev_generation_update_candidate(
	supervisor: &EpochSupervisor,
	committed: &CommittedGeneration,
	runtime: DevRuntimeInputs,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	let bundle = committed.projections().clone();
	let plan = committed.build_plan().clone();
	let critical_css =
		bundle_critical_css(&plan, committed.artifacts().public_filemap()).map_err(|source| {
			DevBuildError::BuildInputs {
				source: BuildInputError::PublicStatic { source },
			}
		})?;
	let prepared_outputs = prepare_cached_static_dev_generation_update_outputs(
		&bundle,
		&plan,
		committed.artifacts().public_filemap(),
	)?;
	let PreparedCachedStaticDevGenerationUpdateOutputs {
		prebuild_typescript,
		typescript_contracts,
		vite_plugin_config,
		public_filemap,
		disk_rollback,
	} = prepared_outputs;
	let critical_css_imports = critical_css.imports().to_vec();
	let completed_artifacts = prepare_dev_generation_from_static_outputs(
		&bundle,
		&plan,
		critical_css.css(),
		committed.artifacts().public_filepaths().to_vec(),
		public_filemap.clone(),
		committed.artifacts(),
		runtime,
	)
	.map_err(|source| DevBuildError::DevGeneration { source })?;
	let candidate = supervisor
		.build_next_live_graph_candidate_with_precomputed_projections(
			committed.graph().clone(),
			bundle,
			plan,
			typescript_contracts,
			completed_artifacts,
			committed.artifacts().root_document_hash_source(),
		)
		.map_err(|source| DevBuildError::DevGeneration {
			source: DevGenerationError::Generation { source },
		})?;
	publish_cached_static_dev_generation_update_candidate(
		candidate,
		prebuild_typescript,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		None,
		disk_rollback,
	)
}

/*
Reused static outputs mean the committed critical CSS (and therefore its
import set) is unchanged; the caller passes the current set through.
*/
fn build_and_publish_cached_static_live_dev_generation_update_candidate(
	supervisor: &EpochSupervisor,
	output_layout_lock: &mut OutputLayoutLock,
	committed: &CommittedGeneration,
	live_state: LiveBuildState,
	runtime: DevRuntimeInputs,
	critical_css_imports: Vec<String>,
) -> Result<PublishedDevGenerationUpdateCandidate, DevBuildError> {
	let graph = live_state
		.framework_graph()
		.map_err(|source| DevBuildError::LiveState { source })?;
	let bundle = ProjectionBundle::compile(&graph);
	let plan =
		BuildProjectionPlan::compile(&bundle).map_err(|source| DevBuildError::BuildInputs {
			source: BuildInputError::BuildPlan { source },
		})?;
	let orphaned_output_layout = output_layout_lock
		.reacquire_if_output_layout_moved(plan.workspace().root_dir(), plan.workspace().dist_dir())
		.map_err(|source| DevBuildError::OutputLayoutLock { source })?;
	if !can_reuse_committed_static_outputs(committed.build_plan(), &plan) {
		return build_and_publish_live_dev_generation_update_candidate(
			supervisor,
			output_layout_lock,
			live_state,
			runtime,
		);
	}

	let prepared_outputs = prepare_cached_static_dev_generation_update_outputs(
		&bundle,
		&plan,
		committed.artifacts().public_filemap(),
	)?;
	let PreparedCachedStaticDevGenerationUpdateOutputs {
		prebuild_typescript,
		typescript_contracts,
		vite_plugin_config,
		public_filemap,
		disk_rollback,
	} = prepared_outputs;
	let completed_artifacts = prepare_dev_generation_from_committed_static_outputs(
		&bundle,
		&plan,
		committed.artifacts(),
		runtime,
	)
	.map_err(|source| DevBuildError::DevGeneration { source })?;
	let candidate = supervisor
		.build_next_live_graph_candidate_with_precomputed_projections(
			graph,
			bundle,
			plan,
			typescript_contracts,
			completed_artifacts,
			live_state.root_document_hash_source(),
		)
		.map_err(|source| DevBuildError::DevGeneration {
			source: DevGenerationError::Generation { source },
		})?;
	publish_cached_static_dev_generation_update_candidate(
		candidate,
		prebuild_typescript,
		vite_plugin_config,
		public_filemap,
		critical_css_imports,
		orphaned_output_layout,
		disk_rollback,
	)
}

fn can_reuse_committed_static_outputs(
	previous: &BuildProjectionPlan,
	next: &BuildProjectionPlan,
) -> bool {
	previous.workspace() == next.workspace()
		&& previous.static_inputs() == next.static_inputs()
		&& previous.vite_inputs().public_static_base() == next.vite_inputs().public_static_base()
}

fn commit_published_dev_generation_update(
	supervisor: &mut EpochSupervisor,
	published: PublishedDevGenerationUpdateCandidate,
) -> (CommittedGeneration, DevGenerationUpdateReport) {
	let committed_generation = supervisor.activate(published.candidate).clone();
	let refresh_payloads = refresh_payloads_from_effects(
		&committed_generation.browser_refresh_effects(),
		committed_generation.artifacts().critical_css(),
	);
	let report = DevGenerationUpdateReport {
		output_publish: published.output_publish,
		refresh_payloads,
		retired_app_server_termination_error: None,
	};
	(committed_generation, report)
}

fn remove_static_fast_path_overlay_cleanup(report: &mut DevGenerationUpdateReport) {
	report
		.refresh_payloads
		.retain(|payload| payload.change_type() != ChangeType::HideRebuildingOverlay);
}

fn force_client_revalidation_refresh(report: &mut DevGenerationUpdateReport) {
	if !report.refresh_payloads.iter().any(|payload| {
		matches!(
			payload.change_type(),
			ChangeType::ClientRevalidate | ChangeType::HardReload
		)
	}) {
		report
			.refresh_payloads
			.push(RefreshPayload::new(ChangeType::ClientRevalidate, "", ""));
	}
}

fn discard_uncommitted_app_server_process(mut process: Box<dyn StartedBuildProcess + Send>) {
	let _ = process.terminate();
}

fn rollback_dev_activation_error(
	error: DevBuildError,
	disk_rollback: DevDiskRollback,
) -> DevBuildError {
	match disk_rollback.restore() {
		Ok(()) => error,
		Err(rollback_error) => DevBuildError::DiskRollback {
			activation_error: error.to_string(),
			rollback_error: rollback_error.to_string(),
		},
	}
}

/// Complete dev build start error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevBuildError {
	/// Secure token generation failed.
	TokenGeneration {
		/// Source token generation error.
		source: TokenGenerationError,
	},
	/// Vite plugin token was empty.
	EmptyVitePluginToken,
	/// Dev refresh token was empty.
	EmptyDevRefreshToken,
	/// Dev mux port cannot be zero.
	InvalidDevMuxPort,
	/// Vite server port cannot be zero.
	InvalidViteServerPort,
	/// Output-layout lock acquisition or handoff failed.
	OutputLayoutLock {
		/// Source output-layout lock error.
		source: OutputLayoutLockError,
	},
	/// App server port cannot be zero.
	InvalidAppServerPort,
	/// Dev loopback port allocation failed.
	PortAllocation {
		/// Source network error message.
		message: String,
	},
	/// Shared build input preparation failed.
	BuildInputs {
		/// Source build input error.
		source: BuildInputError,
	},
	/// Generated TypeScript prebuild output failed.
	TypeScriptOutput {
		/// Source output write error.
		source: BuildOutputWriteError,
	},
	/// Capturing pre-update disk state failed.
	DiskSnapshot {
		/// Source disk snapshot error.
		source: DevDiskRollbackError,
	},
	/// Restoring pre-update disk state after an activation failure failed.
	DiskRollback {
		/// Activation error that required rollback.
		activation_error: String,
		/// Rollback failure.
		rollback_error: String,
	},
	/// Vite plugin RPC server could not start.
	VitePluginServer {
		/// Source Vite plugin server error.
		source: VitePluginServerError,
	},
	/// Vite plugin control notification failed.
	VitePluginControl {
		/// Source Vite plugin control error.
		source: VitePluginControlError,
	},
	/// Dev mux server failed.
	DevMux {
		/// Source dev mux error.
		source: DevMuxError,
	},
	/// Vite dev server failed to start.
	Vite {
		/// Source Vite dev server error.
		source: DevViteError,
	},
	/// App server failed to start.
	AppServer {
		/// Source app server process error.
		source: AppServerProcessError,
	},
	/// Long-lived dev process did not become ready.
	ProcessReady {
		/// Source readiness error.
		source: ProcessReadyError,
	},
	/// Live-state protocol failed.
	LiveState {
		/// Source live-state error.
		source: LiveBuildStateError,
	},
	/// Vite plugin did not report the dev server port in time.
	ViteControlPortTimeout,
	/// Dev generation preparation, publication, or activation failed.
	DevGeneration {
		/// Source dev generation error.
		source: DevGenerationError,
	},
}

impl std::fmt::Display for DevBuildError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::TokenGeneration { source } => write!(f, "{source}"),
			Self::EmptyVitePluginToken => f.write_str("Vite plugin token cannot be empty"),
			Self::EmptyDevRefreshToken => f.write_str("dev refresh token cannot be empty"),
			Self::InvalidDevMuxPort => f.write_str("dev mux port cannot be zero"),
			Self::InvalidViteServerPort => f.write_str("Vite server port cannot be zero"),
			Self::InvalidAppServerPort => f.write_str("app server port cannot be zero"),
			Self::OutputLayoutLock { source } => write!(f, "{source}"),
			Self::PortAllocation { message } => {
				write!(f, "allocate dev loopback port: {message}")
			}
			Self::BuildInputs { source } => write!(f, "{source}"),
			Self::TypeScriptOutput { source } => write!(f, "{source}"),
			Self::DiskSnapshot { source } => write!(f, "{source}"),
			Self::DiskRollback {
				activation_error,
				rollback_error,
			} => write!(
				f,
				"dev generation activation failed ({activation_error}) and disk rollback failed ({rollback_error})"
			),
			Self::VitePluginServer { source } => write!(f, "{source}"),
			Self::VitePluginControl { source } => write!(f, "{source}"),
			Self::DevMux { source } => write!(f, "{source}"),
			Self::Vite { source } => write!(f, "{source}"),
			Self::AppServer { source } => write!(f, "{source}"),
			Self::ProcessReady { source } => write!(f, "{source}"),
			Self::LiveState { source } => write!(f, "{source}"),
			Self::ViteControlPortTimeout => {
				f.write_str("Vite plugin did not report a dev server port")
			}
			Self::DevGeneration { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for DevBuildError {}

fn wait_for_vite_plugin_control_port(state: &VitePluginRpcState) -> Result<u16, DevBuildError> {
	let deadline = Instant::now() + VITE_CONTROL_PORT_WAIT_TIMEOUT;
	loop {
		if let Some(port) = state.control_port() {
			return Ok(port);
		}
		if Instant::now() >= deadline {
			return Err(DevBuildError::ViteControlPortTimeout);
		}
		thread::sleep(VITE_CONTROL_PORT_WAIT_INTERVAL);
	}
}

fn allocate_loopback_port() -> Result<u16, DevBuildError> {
	let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).map_err(|source| {
		DevBuildError::PortAllocation {
			message: source.to_string(),
		}
	})?;
	Ok(listener
		.local_addr()
		.map_err(|source| DevBuildError::PortAllocation {
			message: source.to_string(),
		})?
		.port())
}

fn changed_public_source_paths(
	previous: &BTreeMap<String, String>,
	next: &BTreeMap<String, String>,
) -> Vec<String> {
	let mut changed = BTreeSet::new();
	for (source_path, public_path) in previous {
		if next.get(source_path) != Some(public_path) {
			changed.insert(source_path.clone());
		}
	}
	for source_path in next.keys() {
		if !previous.contains_key(source_path) {
			changed.insert(source_path.clone());
		}
	}
	changed.into_iter().collect()
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::io::{Read, Write};
	use std::net::TcpListener;
	use std::path::PathBuf;
	use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
	use std::sync::{Arc, Mutex};
	use std::thread::{self, JoinHandle};

	use super::*;
	use crate::app_server_process::APP_SERVER_PORT_ENV_KEY;
	use crate::dev_refresh::ChangeType;
	use crate::process_runner::{BuildProcessCommand, BuildProcessError};
	use crate::test_support::unique_temp_root_with_dirs;
	use crate::vite_plugin_contract::{
		VITE_PLUGIN_LOOPBACK_HOST, VITE_PLUGIN_SERVER_PORT_ENV_KEY,
		VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, VITE_PLUGIN_TOKEN_HEADER,
	};

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_PREBUILT_APP_SERVER_EXECUTABLE: &str = "/workspace/app/target/debug/example-server";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_STATIC_SOURCE_DIR: &str = "public";
	const TEST_ENTRY_FILE: &str = "src/entry.tsx";
	const TEST_CRITICAL_CSS_FILE: &str = "src/critical.css";
	const TEST_VIEW_PATTERN: &str = "/";
	const TEST_VIEW_CLIENT_FILE: &str = "src/root.tsx";
	const TEST_VITE_PLUGIN_TOKEN: &str = "vite-plugin-token";
	const TEST_DEV_REFRESH_TOKEN: &str = "dev-refresh-token";

	fn temp_root() -> PathBuf {
		unique_temp_root_with_dirs(
			"vorma-build-dev-build",
			&[TEST_PUBLIC_STATIC_SOURCE_DIR, "src"],
		)
	}

	fn prebuilt_app_server() -> PrebuiltAppServer {
		PrebuiltAppServer::new(
			TEST_CARGO_PACKAGE,
			TEST_CARGO_BIN,
			TEST_PREBUILT_APP_SERVER_EXECUTABLE,
		)
	}

	#[tokio::test]
	async fn view_module_list_change_restarts_vite_instead_of_invalidating_assets() {
		let root_dir = temp_root();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner::default();
		let control_client = FakeControlClient::default();
		let control_calls = control_client.calls.clone();
		let assets_calls = control_client.assets_calls.clone();
		let dev_mux_port = allocate_loopback_port().unwrap();
		let (vite_server_port, vite_ready_thread) = spawn_ready_server();
		let app_server_port = allocate_loopback_port().unwrap();
		let mut started = start_and_activate_dev_generation(
			EpochSupervisor::default(),
			app,
			DevGenerationStartConfig::new(
				dev_mux_port,
				app_server_port,
				vite_server_port,
				TEST_VITE_PLUGIN_TOKEN,
				TEST_DEV_REFRESH_TOKEN,
			),
			&mut server,
			&mut runner,
			Box::new(control_client),
		)
		.await
		.unwrap();
		vite_ready_thread.join().unwrap();
		let state = server.state.clone().unwrap();
		state.record_control_port(5174);

		/*
		Same public assets, one additional view: the plugin's config payload
		changes, so the full restart channel must fire and the targeted
		asset-invalidation channel must stay quiet.
		*/
		let live_state = vorma::build_interface::live_build_state_from_app_build_contract(
			app_build_contract_with_views(
				root_dir.clone(),
				vec![
					(TEST_VIEW_PATTERN, TEST_VIEW_CLIENT_FILE),
					("/about", "src/views/about.tsx"),
				],
			),
		)
		.await
		.unwrap();
		started
			.activate_next_live_state_generation(live_state, prebuilt_app_server(), &mut runner)
			.unwrap();

		assert_eq!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.as_slice(),
			&[(5174, TEST_VITE_PLUGIN_TOKEN.to_owned())]
		);
		assert!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);

		drop(started);
		fs::remove_dir_all(root_dir).unwrap();
	}

	fn app_build_contract(root_dir: PathBuf) -> AppBuildContract {
		app_build_contract_with_views(root_dir, vec![(TEST_VIEW_PATTERN, TEST_VIEW_CLIENT_FILE)])
	}

	fn app_build_contract_with_views(
		root_dir: PathBuf,
		view_declarations: Vec<(&'static str, &'static str)>,
	) -> AppBuildContract {
		let mut views = vorma::Views::new();
		for (pattern, client_file) in view_declarations {
			views.push(vorma::View::from_static(
				pattern,
				client_file,
				vorma::build_interface::route_input::type_resolver::<()>,
				vorma::build_interface::route_input::type_resolver::<()>,
				vorma::build_interface::route_input::search_schema_resolver::<()>,
				root_view_handler,
			));
		}
		vorma::build_interface::app_build_contract(vorma::AppConfig {
			root_dir,
			server_target: vorma::ServerTarget {
				cargo_package: TEST_CARGO_PACKAGE.to_owned(),
				cargo_bin: TEST_CARGO_BIN.to_owned(),
			},
			dist_dir: TEST_DIST_DIR.to_owned(),
			public_static_base: TEST_PUBLIC_STATIC_BASE.to_owned(),
			frontend_config: vorma::FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: TEST_ENTRY_FILE.to_owned(),
				public_static_src_dir: TEST_PUBLIC_STATIC_SOURCE_DIR.to_owned(),
				critical_css_file: TEST_CRITICAL_CSS_FILE.to_owned(),
			},
			ts_gen_config: vorma::TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				..vorma::TsGenConfig::default()
			},
			views,
			..vorma::AppConfig::<()>::default()
		})
		.unwrap()
	}

	fn root_view_handler(
		ctx: vorma::build_interface::ErasedRequestCtx<()>,
	) -> vorma::build_interface::ErasedRouteFuture {
		vorma::build_interface::run_static_view::<(), (), (), ()>(ctx, root_view_inner)
	}

	fn root_view_inner(
		_ctx: vorma::ViewCtx<(), ()>,
	) -> vorma::build_interface::RouteFuture<(), vorma::ViewExit> {
		Box::pin(async { Ok(()) })
	}

	fn spawn_ready_server() -> (u16, JoinHandle<()>) {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		(port, spawn_ready_server_with_listener(listener))
	}

	fn spawn_ready_server_on_port(port: u16) -> JoinHandle<()> {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, port)).unwrap();
		spawn_ready_server_with_listener(listener)
	}

	fn spawn_ready_server_with_listener(listener: TcpListener) -> JoinHandle<()> {
		thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut request = [0_u8; 512];
			let _ = stream.read(&mut request);
			stream
				.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				.unwrap();
		})
	}

	#[derive(Default)]
	struct FakeVitePluginServer {
		state: Option<VitePluginRpcState>,
	}

	impl VitePluginRpcServer for FakeVitePluginServer {
		fn start_vite_plugin_rpc_server(
			&mut self,
			state: VitePluginRpcState,
		) -> Result<VitePluginRpcServerHandle, VitePluginServerError> {
			state.record_control_port(5174);
			self.state = Some(state);
			Ok(VitePluginRpcServerHandle::new(4173))
		}
	}

	#[derive(Default)]
	struct FakeProcessRunner {
		commands: Vec<BuildProcessCommand>,
		terminations: Arc<AtomicU64>,
		exit_watch_registrations: Arc<AtomicU64>,
	}

	impl BuildProcessRunner for FakeProcessRunner {
		fn run_inheriting_stdio(
			&mut self,
			_command: &BuildProcessCommand,
		) -> Result<(), BuildProcessError> {
			unreachable!("dev build should start Vite without waiting for it to exit")
		}

		fn start_inheriting_stdio(
			&mut self,
			command: &BuildProcessCommand,
		) -> Result<Box<dyn StartedBuildProcess + Send>, BuildProcessError> {
			self.commands.push(command.clone());
			if let Some(port) = command
				.env()
				.get(APP_SERVER_PORT_ENV_KEY)
				.and_then(|port| port.parse::<u16>().ok())
			{
				let _ = spawn_ready_server_on_port(port);
			}
			Ok(Box::new(FakeStartedProcess {
				terminations: self.terminations.clone(),
				exit_watch_registrations: self.exit_watch_registrations.clone(),
			}))
		}
	}

	struct FakeStartedProcess {
		terminations: Arc<AtomicU64>,
		exit_watch_registrations: Arc<AtomicU64>,
	}

	impl StartedBuildProcess for FakeStartedProcess {
		fn process_id(&self) -> Option<u32> {
			Some(99)
		}

		fn watch_unexpected_exit(&mut self, _on_exit: Box<dyn FnOnce() + Send>) {
			self.exit_watch_registrations.fetch_add(1, Ordering::SeqCst);
		}

		fn terminate(&mut self) -> Result<(), BuildProcessError> {
			self.terminations.fetch_add(1, Ordering::SeqCst);
			Ok(())
		}
	}

	#[derive(Clone, Default)]
	struct FakeControlClient {
		calls: Arc<Mutex<Vec<(u16, String)>>>,
		assets_calls: Arc<Mutex<Vec<Vec<String>>>>,
		fail_notifications: Arc<AtomicBool>,
	}

	impl VitePluginControlClient for FakeControlClient {
		fn notify_config_changed(
			&mut self,
			control_port: u16,
			token: &str,
		) -> Result<(), VitePluginControlError> {
			self.calls
				.lock()
				.expect("fake control client lock poisoned")
				.push((control_port, token.to_owned()));
			if self.fail_notifications.load(Ordering::SeqCst) {
				return Err(VitePluginControlError::UnexpectedStatus {
					status_line: "HTTP/1.1 503 Test Failure".to_owned(),
				});
			}
			Ok(())
		}

		fn notify_assets_changed(
			&mut self,
			_control_port: u16,
			_token: &str,
			changed_source_paths: &[String],
		) -> Result<(), VitePluginControlError> {
			if self.fail_notifications.load(Ordering::SeqCst) {
				return Err(VitePluginControlError::UnexpectedStatus {
					status_line: "HTTP/1.1 503 Test Failure".to_owned(),
				});
			}
			self.assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.push(changed_source_paths.to_vec());
			Ok(())
		}
	}

	#[tokio::test]
	async fn critical_css_update_uses_static_fast_path_without_app_server_restart_or_overlay() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg/>",
		)
		.unwrap();
		fs::write(
			root_dir.join(TEST_CRITICAL_CSS_FILE),
			"@import \"./critical-partial.css\";\nbody{}",
		)
		.unwrap();
		fs::write(
			root_dir
				.join(TEST_CRITICAL_CSS_FILE)
				.with_file_name("critical-partial.css"),
			".partial{color:red}",
		)
		.unwrap();
		let app = app_build_contract(root_dir.clone());
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner::default();
		let control_client = FakeControlClient::default();
		let control_calls = control_client.calls.clone();
		let assets_calls = control_client.assets_calls.clone();
		let dev_mux_port = allocate_loopback_port().unwrap();
		let (vite_server_port, vite_ready_thread) = spawn_ready_server();
		let app_server_port = allocate_loopback_port().unwrap();

		let mut started = start_and_activate_dev_generation(
			EpochSupervisor::default(),
			app,
			DevGenerationStartConfig::new(
				dev_mux_port,
				app_server_port,
				vite_server_port,
				TEST_VITE_PLUGIN_TOKEN,
				TEST_DEV_REFRESH_TOKEN,
			),
			&mut server,
			&mut runner,
			Box::new(control_client),
		)
		.await
		.unwrap();
		vite_ready_thread.join().unwrap();
		let expected_partial_import = fs::canonicalize(
			root_dir
				.join(TEST_CRITICAL_CSS_FILE)
				.with_file_name("critical-partial.css"),
		)
		.unwrap()
		.display()
		.to_string();
		assert!(
			started
				.critical_css_import_paths()
				.contains(&expected_partial_import)
		);
		let mut refresh_client = started.subscribe_refresh_payloads();
		fs::write(
			root_dir.join(TEST_CRITICAL_CSS_FILE),
			"@import \"./critical-partial.css\";\nbody{}\n.dev-change-critical-css-probe{color:rgb(1,2,3)}",
		)
		.unwrap();

		let update_report = started
			.activate_next_static_generation(DevStaticGenerationUpdate::new(
				DevStaticGenerationKind::CriticalCss,
				false,
			))
			.unwrap();

		assert_eq!(started.report().app_server_port(), app_server_port);
		assert_eq!(runner.commands.len(), 2);
		assert_eq!(started.committed_generation().id(), 2);
		assert!(
			started
				.committed_generation()
				.manifest()
				.critical_css()
				.contains("dev-change-critical-css-probe")
		);
		assert!(
			update_report
				.output_publish()
				.public_static()
				.written_files()
				.is_empty()
		);
		assert_eq!(update_report.refresh_payloads().len(), 1);
		assert_eq!(
			update_report.refresh_payloads()[0].change_type(),
			ChangeType::UpdateCriticalCss
		);
		assert!(
			update_report.refresh_payloads()[0]
				.critical_css()
				.contains("dev-change-critical-css-probe")
		);
		let refresh_payload = refresh_client.try_recv().unwrap();
		assert_eq!(refresh_payload.change_type(), ChangeType::UpdateCriticalCss);
		assert!(
			refresh_payload
				.critical_css()
				.contains("dev-change-critical-css-probe")
		);
		assert!(matches!(
			refresh_client.try_recv(),
			Err(tokio::sync::mpsc::error::TryRecvError::Empty)
		));
		assert!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);

		/*
		The re-bundled generation still tracks the partial, so the dev loop's
		per-generation watch-plan swap keeps classifying its edits.
		*/
		assert!(
			started
				.critical_css_import_paths()
				.contains(&expected_partial_import)
		);

		drop(started);
		assert_eq!(runner.terminations.load(Ordering::SeqCst), 2);
		assert!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		fs::remove_dir_all(root_dir).unwrap();
	}

	#[tokio::test]
	async fn public_static_update_uses_static_fast_path_without_app_server_restart() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg/>",
		)
		.unwrap();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner::default();
		let control_client = FakeControlClient::default();
		let control_calls = control_client.calls.clone();
		let assets_calls = control_client.assets_calls.clone();
		let dev_mux_port = allocate_loopback_port().unwrap();
		let (vite_server_port, vite_ready_thread) = spawn_ready_server();
		let app_server_port = allocate_loopback_port().unwrap();

		let mut started = start_and_activate_dev_generation(
			EpochSupervisor::default(),
			app,
			DevGenerationStartConfig::new(
				dev_mux_port,
				app_server_port,
				vite_server_port,
				TEST_VITE_PLUGIN_TOKEN,
				TEST_DEV_REFRESH_TOKEN,
			),
			&mut server,
			&mut runner,
			Box::new(control_client),
		)
		.await
		.unwrap();
		vite_ready_thread.join().unwrap();
		let state = server.state.as_ref().unwrap().clone();
		let mut headers = http::HeaderMap::new();
		headers.insert(
			VITE_PLUGIN_TOKEN_HEADER,
			TEST_VITE_PLUGIN_TOKEN.parse().unwrap(),
		);
		let initial_hash_response =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);
		let mut refresh_client = started.subscribe_refresh_payloads();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg>updated-static-fast-path</svg>",
		)
		.unwrap();

		let update_report = started
			.activate_next_static_generation(DevStaticGenerationUpdate::new(
				DevStaticGenerationKind::PublicStaticAndCriticalCss,
				false,
			))
			.unwrap();
		let updated_hash_response =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);

		assert_ne!(initial_hash_response.body(), updated_hash_response.body());
		assert_eq!(started.report().app_server_port(), app_server_port);
		assert_eq!(runner.commands.len(), 2);
		assert_eq!(started.committed_generation().id(), 2);
		assert!(
			update_report
				.output_publish()
				.public_static()
				.written_files()
				.iter()
				.any(|path| path
					.file_name()
					.and_then(|name| name.to_str())
					.is_some_and(|name| name.contains("logo")))
		);
		assert_eq!(update_report.refresh_payloads().len(), 1);
		assert_eq!(
			update_report.refresh_payloads()[0].change_type(),
			ChangeType::HardReload
		);
		assert_eq!(
			refresh_client.try_recv().unwrap().change_type(),
			ChangeType::HardReload
		);
		assert!(matches!(
			refresh_client.try_recv(),
			Err(tokio::sync::mpsc::error::TryRecvError::Empty)
		));
		assert!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		assert_eq!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.as_slice(),
			&[vec!["logo.svg".to_owned()]]
		);

		drop(started);
		assert_eq!(runner.terminations.load(Ordering::SeqCst), 2);
		fs::remove_dir_all(root_dir).unwrap();
	}

	#[tokio::test]
	async fn dev_server_recompile_update_reuses_committed_static_outputs() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg/>",
		)
		.unwrap();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner::default();
		let control_client = FakeControlClient::default();
		let control_calls = control_client.calls.clone();
		let assets_calls = control_client.assets_calls.clone();
		let dev_mux_port = allocate_loopback_port().unwrap();
		let (vite_server_port, vite_ready_thread) = spawn_ready_server();
		let app_server_port = allocate_loopback_port().unwrap();

		let mut started = start_and_activate_dev_generation(
			EpochSupervisor::default(),
			app,
			DevGenerationStartConfig::new(
				dev_mux_port,
				app_server_port,
				vite_server_port,
				TEST_VITE_PLUGIN_TOKEN,
				TEST_DEV_REFRESH_TOKEN,
			),
			&mut server,
			&mut runner,
			Box::new(control_client),
		)
		.await
		.unwrap();
		vite_ready_thread.join().unwrap();
		let public_filemap_before_update = started
			.committed_generation()
			.artifacts()
			.public_filemap()
			.clone();

		let live_state = vorma::build_interface::live_build_state_from_app_build_contract(
			app_build_contract(root_dir.clone()),
		)
		.await
		.unwrap();
		started.watch_child_exits(Arc::new(|_| {}));
		/*
		One registration per current child (app server + Vite); every
		replacement app server must re-register so exits stay observed
		across generations.
		*/
		assert_eq!(runner.exit_watch_registrations.load(Ordering::SeqCst), 2);
		let cancel = BuildProcessCancel::new();
		let update_report = started
			.activate_next_live_state_generation_reusing_committed_static_outputs_until_cancelled(
				live_state,
				prebuilt_app_server(),
				&mut runner,
				&cancel,
			)
			.unwrap();

		assert_eq!(started.committed_generation().id(), 2);
		assert_eq!(runner.exit_watch_registrations.load(Ordering::SeqCst), 3);
		assert_eq!(
			started.committed_generation().artifacts().public_filemap(),
			&public_filemap_before_update
		);
		assert!(
			update_report
				.output_publish()
				.public_static()
				.written_files()
				.is_empty()
		);
		assert!(
			update_report
				.output_publish()
				.public_static()
				.retained_stale_files()
				.is_empty()
		);
		assert_eq!(
			update_report.refresh_payloads()[0].change_type(),
			ChangeType::HideRebuildingOverlay
		);
		assert!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		assert!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		assert_eq!(runner.commands.len(), 3);

		drop(started);
		assert_eq!(runner.terminations.load(Ordering::SeqCst), 3);
		fs::remove_dir_all(root_dir).unwrap();
	}

	#[tokio::test]
	async fn dev_build_starts_vite_and_commits_reported_generation() {
		let root_dir = temp_root();
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg/>",
		)
		.unwrap();
		fs::write(root_dir.join(TEST_CRITICAL_CSS_FILE), "body{}").unwrap();
		let app = app_build_contract(root_dir.clone());
		let mut server = FakeVitePluginServer::default();
		let mut runner = FakeProcessRunner::default();
		let control_client = FakeControlClient::default();
		let control_calls = control_client.calls.clone();
		let assets_calls = control_client.assets_calls.clone();
		let fail_notifications = control_client.fail_notifications.clone();
		let dev_mux_port = allocate_loopback_port().unwrap();
		let (vite_server_port, vite_ready_thread) = spawn_ready_server();
		let app_server_port = allocate_loopback_port().unwrap();

		let mut started = start_and_activate_dev_generation(
			EpochSupervisor::default(),
			app,
			DevGenerationStartConfig::new(
				dev_mux_port,
				app_server_port,
				vite_server_port,
				TEST_VITE_PLUGIN_TOKEN,
				TEST_DEV_REFRESH_TOKEN,
			),
			&mut server,
			&mut runner,
			Box::new(control_client),
		)
		.await
		.unwrap();
		vite_ready_thread.join().unwrap();

		let state = server.state.as_ref().unwrap().clone();
		let mut headers = http::HeaderMap::new();
		headers.insert(
			VITE_PLUGIN_TOKEN_HEADER,
			TEST_VITE_PLUGIN_TOKEN.parse().unwrap(),
		);
		let initial_hash_response =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);
		let vite_command = runner.commands[0].clone();
		let app_server_command = runner.commands[1].clone();
		let committed_id = started.committed_generation().id();
		let initial_effects = started.committed_generation().browser_refresh_effects();
		let mut refresh_client = started.subscribe_refresh_payloads();
		started.broadcast_rebuilding();
		started.broadcast_build_error("build failed");
		started.broadcast_client_revalidation();

		assert_eq!(started.vite_process_id(), Some(99));
		assert_eq!(started.vite_plugin_rpc_server_port(), 4173);
		assert_eq!(started.report().vite_server_port(), vite_server_port);
		assert_eq!(started.report().vite_plugin_control_port(), 5174);
		assert_eq!(started.report().app_server_port(), app_server_port);
		assert_eq!(started.report().dev_mux_port(), dev_mux_port);
		assert!(started.report().prebuild_typescript().path().exists());
		assert!(
			started
				.report()
				.output_publish()
				.generation_outputs()
				.manifest_path()
				.exists()
		);
		assert_eq!(
			started
				.committed_generation()
				.manifest()
				.client_entry()
				.url(),
			format!("http://127.0.0.1:{vite_server_port}/src/entry.tsx")
		);
		assert_eq!(
			started
				.committed_generation()
				.manifest()
				.dev_refresh_token(),
			TEST_DEV_REFRESH_TOKEN
		);
		assert_eq!(initial_effects.generation_id(), committed_id);
		assert_eq!(
			started
				.committed_generation()
				.manifest()
				.dev_refresh_token(),
			TEST_DEV_REFRESH_TOKEN
		);
		assert!(initial_effects.public_assets_changed());
		assert!(initial_effects.critical_css_changed());
		assert!(initial_effects.client_revalidate_required());
		assert_eq!(
			refresh_client.try_recv().unwrap().change_type(),
			ChangeType::ShowRebuildingOverlay
		);
		let build_error = refresh_client.try_recv().unwrap();
		assert_eq!(build_error.change_type(), ChangeType::ShowBuildError);
		assert_eq!(build_error.build_error(), "build failed");
		assert_eq!(
			refresh_client.try_recv().unwrap().change_type(),
			ChangeType::ClientRevalidate
		);
		assert_eq!(
			vite_command
				.env()
				.get(VITE_PLUGIN_SERVER_PORT_ENV_KEY)
				.unwrap(),
			"4173"
		);
		assert_eq!(
			vite_command
				.env()
				.get(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY)
				.unwrap(),
			TEST_VITE_PLUGIN_TOKEN
		);
		assert_eq!(app_server_command.program(), "cargo");
		assert_eq!(
			app_server_command
				.env()
				.get(APP_SERVER_PORT_ENV_KEY)
				.unwrap(),
			&app_server_port.to_string()
		);
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg>updated</svg>",
		)
		.unwrap();

		let live_state = vorma::build_interface::live_build_state_from_app_build_contract(
			app_build_contract(root_dir.clone()),
		)
		.await
		.unwrap();
		let update_report = started
			.activate_next_live_state_generation(live_state, prebuilt_app_server(), &mut runner)
			.unwrap();
		let updated_hash_response =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);
		let restarted_app_server_port = runner.commands[2]
			.env()
			.get(APP_SERVER_PORT_ENV_KEY)
			.unwrap()
			.parse::<u16>()
			.unwrap();

		assert_ne!(initial_hash_response.body(), updated_hash_response.body());
		assert_ne!(restarted_app_server_port, app_server_port);
		assert_eq!(
			started.report().app_server_port(),
			restarted_app_server_port
		);
		assert_eq!(started.committed_generation().id(), 2);
		assert_eq!(started.supervisor.committed().unwrap().id(), 2);
		let updated_effects = started.committed_generation().browser_refresh_effects();
		assert_eq!(updated_effects.generation_id(), 2);
		assert_eq!(
			started
				.committed_generation()
				.manifest()
				.dev_refresh_token(),
			TEST_DEV_REFRESH_TOKEN
		);
		assert!(updated_effects.public_assets_changed());
		assert!(!updated_effects.critical_css_changed());
		assert!(!updated_effects.client_revalidate_required());
		assert_eq!(
			update_report.refresh_payloads()[0].change_type(),
			ChangeType::HardReload
		);
		assert_eq!(
			refresh_client.try_recv().unwrap().change_type(),
			ChangeType::HardReload
		);
		assert!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		assert_eq!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.as_slice(),
			&[vec!["logo.svg".to_owned()]]
		);
		assert_eq!(runner.commands.len(), 3);

		state.record_control_port(6174);
		let committed_id_before_failed_update = started.committed_generation().id();
		let app_server_port_before_failed_update = started.report().app_server_port();
		let hash_before_failed_update = updated_hash_response.body().to_vec();
		let generated_typescript_path = update_report
			.output_publish()
			.generation_outputs()
			.generated_typescript_path()
			.to_path_buf();
		let manifest_path = update_report
			.output_publish()
			.generation_outputs()
			.manifest_path()
			.to_path_buf();
		let public_output_dir = update_report
			.output_publish()
			.public_static()
			.public_output_dir()
			.to_path_buf();
		let generated_typescript_before_failed_update =
			fs::read(&generated_typescript_path).unwrap();
		let manifest_before_failed_update = fs::read(&manifest_path).unwrap();
		fail_notifications.store(true, Ordering::SeqCst);
		fs::write(
			root_dir
				.join(TEST_PUBLIC_STATIC_SOURCE_DIR)
				.join("logo.svg"),
			"<svg>failed-update</svg>",
		)
		.unwrap();
		let failed_live_state = vorma::build_interface::live_build_state_from_app_build_contract(
			app_build_contract(root_dir.clone()),
		)
		.await
		.unwrap();
		let failed_update = started
			.activate_next_live_state_generation(
				failed_live_state,
				prebuilt_app_server(),
				&mut runner,
			)
			.unwrap_err();
		let hash_after_failed_update =
			state.handle_rpc(&headers, br#"{"method":"hash","src_path":"logo.svg"}"#);

		assert!(matches!(
			failed_update,
			DevBuildError::VitePluginControl { .. }
		));
		assert_eq!(
			started.committed_generation().id(),
			committed_id_before_failed_update
		);
		assert_eq!(
			started.supervisor.committed().unwrap().id(),
			committed_id_before_failed_update
		);
		assert_eq!(
			started.report().app_server_port(),
			app_server_port_before_failed_update
		);
		assert_eq!(hash_after_failed_update.body(), hash_before_failed_update);
		assert_eq!(
			fs::read(&generated_typescript_path).unwrap(),
			generated_typescript_before_failed_update
		);
		assert_eq!(
			fs::read(&manifest_path).unwrap(),
			manifest_before_failed_update
		);
		for entry in fs::read_dir(public_output_dir).unwrap() {
			let path = entry.unwrap().path();
			if path.is_file() {
				assert!(!fs::read_to_string(path).unwrap().contains("failed-update"));
			}
		}
		assert!(matches!(
			refresh_client.try_recv(),
			Err(tokio::sync::mpsc::error::TryRecvError::Empty)
		));
		assert!(
			control_calls
				.lock()
				.expect("fake control client lock poisoned")
				.is_empty()
		);
		/*
		Only the successful generation-2 update invalidated assets; the
		failed update's notification errored before being recorded.
		*/
		assert_eq!(
			assets_calls
				.lock()
				.expect("fake control client lock poisoned")
				.as_slice(),
			&[vec!["logo.svg".to_owned()]]
		);
		assert_eq!(runner.commands.len(), 4);

		drop(started);
		assert_eq!(runner.terminations.load(Ordering::SeqCst), 4);
		fs::remove_dir_all(root_dir).unwrap();
	}
}
