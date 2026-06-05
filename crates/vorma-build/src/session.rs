use vorma::__private::Config;

use crate::RunMode;
use crate::cargo_target::CargoBinTarget;
use crate::config::ConfigView;
use vorma::__private::manifest::Manifest;

use crate::generation::{CommittedGeneration, GenerationCandidate};
use crate::runtime::DevRuntime;

#[derive(Debug)]
pub(crate) struct BuildSession {
	initial_config: Config,
	bootstrap_config_override: Option<Config>,
	build_entry: CargoBinTarget,
	mode: RunMode,
	committed: Option<CommittedGeneration>,
	runtime: DevRuntime,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum BootstrapConfigSource {
	RetryOverride,
	CommittedGeneration,
	InitialConfig,
}

impl BuildSession {
	pub(crate) fn new(initial_config: Config, build_entry: CargoBinTarget, mode: RunMode) -> Self {
		Self {
			initial_config,
			bootstrap_config_override: None,
			build_entry,
			mode,
			committed: None,
			runtime: DevRuntime::new(),
		}
	}

	pub(crate) fn bootstrap_config_view(&self) -> Result<ConfigView<'_>, String> {
		let config = match self.bootstrap_config_source() {
			BootstrapConfigSource::RetryOverride => self
				.bootstrap_config_override
				.as_ref()
				.expect("retry override source requires retry override config"),
			BootstrapConfigSource::CommittedGeneration => self
				.committed
				.as_ref()
				.expect("committed generation source requires committed generation")
				.config(),
			BootstrapConfigSource::InitialConfig => &self.initial_config,
		};
		ConfigView::new(config).map_err(|err| err.to_string())
	}

	pub(crate) fn bootstrap_config_source(&self) -> BootstrapConfigSource {
		if self.bootstrap_config_override.is_some() {
			return BootstrapConfigSource::RetryOverride;
		}
		if self.committed.is_some() {
			return BootstrapConfigSource::CommittedGeneration;
		}
		BootstrapConfigSource::InitialConfig
	}

	pub(crate) fn bootstrap_config(&self) -> Result<Config, String> {
		Ok(self.bootstrap_config_view()?.source().clone())
	}

	pub(crate) fn set_bootstrap_config_override(&mut self, config: Config) {
		self.bootstrap_config_override = Some(config);
	}

	pub(crate) fn build_entry(&self) -> &CargoBinTarget {
		&self.build_entry
	}

	pub(crate) fn mode(&self) -> RunMode {
		self.mode
	}

	pub(crate) fn committed(&self) -> Option<&CommittedGeneration> {
		self.committed.as_ref()
	}

	pub(crate) fn committed_mut(&mut self) -> Option<&mut CommittedGeneration> {
		self.committed.as_mut()
	}

	pub(crate) fn runtime(&self) -> &DevRuntime {
		&self.runtime
	}

	pub(crate) fn runtime_mut(&mut self) -> &mut DevRuntime {
		&mut self.runtime
	}

	pub(crate) fn commit_generation(
		&mut self,
		candidate: GenerationCandidate,
	) -> &CommittedGeneration {
		self.bootstrap_config_override = None;
		self.committed = Some(candidate.commit());
		self.committed
			.as_ref()
			.expect("committed generation was just stored")
	}

	pub(crate) fn set_committed_manifest(&mut self, manifest: Manifest) -> Result<(), String> {
		let committed = self
			.committed
			.as_mut()
			.ok_or_else(|| "committed generation not available".to_owned())?;
		committed.set_manifest(manifest);
		Ok(())
	}
}

#[cfg(test)]
mod tests {
	use std::path::PathBuf;

	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use crate::generation::{
		BuildArtifactMode, BuildArtifacts, GenerationCandidate, LiveMetadata, StaticEffects,
		StaticMetadata,
	};

	use super::*;

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
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/client/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..Config::default()
		}
	}

	#[test]
	fn session_bootstraps_from_initial_config_until_generation_commits() {
		let root = std::env::current_dir().unwrap();
		let mut initial = config(root.clone());
		initial.dist_dir = "dist-a".to_owned();
		let mut next = config(root.clone());
		next.dist_dir = "dist-b".to_owned();
		let mut retry = config(root);
		retry.dist_dir = "dist-c".to_owned();
		let mut session = BuildSession::new(
			initial,
			CargoBinTarget {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-build".to_owned(),
			},
			RunMode::Dev,
		);

		assert_eq!(
			session.bootstrap_config_source(),
			BootstrapConfigSource::InitialConfig
		);
		assert!(
			session
				.bootstrap_config_view()
				.unwrap()
				.dist_dir()
				.ends_with("dist-a")
		);

		session.set_bootstrap_config_override(retry);

		assert_eq!(
			session.bootstrap_config_source(),
			BootstrapConfigSource::RetryOverride
		);
		assert!(
			session
				.bootstrap_config_view()
				.unwrap()
				.dist_dir()
				.ends_with("dist-c")
		);

		session.commit_generation(GenerationCandidate {
			config: next,
			live: LiveMetadata::default(),
			static_metadata: StaticMetadata::default(),
			manifest: None,
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/build-entry"),
				mode: BuildArtifactMode::Dev {
					app_server_executable: PathBuf::from("/tmp/app-server"),
				},
			},
			static_effects: StaticEffects::default(),
		});

		assert_eq!(
			session.bootstrap_config_source(),
			BootstrapConfigSource::CommittedGeneration
		);
		assert!(
			session
				.bootstrap_config_view()
				.unwrap()
				.dist_dir()
				.ends_with("dist-b")
		);
	}
}
