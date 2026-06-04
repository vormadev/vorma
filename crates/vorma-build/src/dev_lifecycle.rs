use std::fmt;
use std::fs;
use std::io;

use paranoid::local_lock::ProcessLock;
use vorma::__private::Config;
use vorma::__private::manifest::Manifest;

use crate::activation::{ActivationError, ActivationOutcome};
use crate::config::ConfigError;
use crate::config::to_cfg;
use crate::generation::{GenerationCandidate, StaticEffects, StaticMetadata};
use crate::manifest::{DevManifestInput, write_dev_manifest as write_dev_manifest_json};
use crate::pipeline::PipelineError;
use crate::session::BuildSession;
use crate::static_build::{
	StaticBuildError, StaticBuildInput, StaticPublishOutcome, prepare_static_build,
	publish_static_outputs,
};

pub(crate) struct DevLifecycle<'a> {
	session: &'a mut BuildSession,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct PublishedDevGeneration {
	pub(crate) effects: StaticEffects,
	pub(crate) static_metadata: StaticMetadata,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct LiveConfigRetryReset {
	pub(crate) dist_dir_changed: bool,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct LiveConfigRetryTransition {
	pub(crate) bootstrap_config: Config,
	pub(crate) next_config: Config,
}

impl<'a> DevLifecycle<'a> {
	pub(crate) fn new(session: &'a mut BuildSession) -> Self {
		Self { session }
	}

	pub(crate) fn prepare_refresh_with_app_server_stopped(
		&mut self,
	) -> Result<GenerationCandidate, DevLifecycleError> {
		let stop_app_server = self.session.runtime_mut().begin_stop_app_server();
		let prepared = self.prepare_refresh();
		self.session
			.runtime_mut()
			.finish_stop_app_server(stop_app_server, false);
		prepared
	}

	pub(crate) fn prepare_refresh(&mut self) -> Result<GenerationCandidate, DevLifecycleError> {
		loop {
			match self.prepare_refresh_attempt()? {
				ServerRefreshCandidateAttempt::Stable(candidate) => return Ok(*candidate),
				ServerRefreshCandidateAttempt::LiveConfigChanged(transition) => {
					let transition = *transition;
					self.reset_after_config_change(&transition)?;
					self.session
						.set_bootstrap_config_override(transition.next_config);
				}
			}
		}
	}

	pub(crate) fn reset_after_config_change(
		&mut self,
		transition: &LiveConfigRetryTransition,
	) -> Result<LiveConfigRetryReset, DevLifecycleError> {
		let bootstrap_cfg =
			to_cfg(&transition.bootstrap_config).map_err(|source| DevLifecycleError::Config {
				phase: "live-config retry bootstrap",
				source,
			})?;
		let next_cfg =
			to_cfg(&transition.next_config).map_err(|source| DevLifecycleError::Config {
				phase: "live-config retry replacement",
				source,
			})?;
		let dist_dir_changed = bootstrap_cfg.dist_dir() != next_cfg.dist_dir();
		if dist_dir_changed {
			let mut next_dev_lock = ProcessLock::new(next_cfg.dev_lock_out());
			next_dev_lock
				.acquire()
				.map_err(|source| DevLifecycleError::DevLock {
					action: "acquiring replacement dev lock",
					source: source.to_string(),
				})?;
			if let Some(mut old_dev_lock) =
				self.session.runtime_mut().replace_dev_lock(next_dev_lock)
			{
				old_dev_lock
					.release()
					.map_err(|source| DevLifecycleError::DevLock {
						action: "releasing previous dev lock",
						source: source.to_string(),
					})?;
			}
			match fs::remove_dir_all(bootstrap_cfg.vorma_out()) {
				Ok(()) => {}
				Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
				Err(source) => {
					return Err(DevLifecycleError::OldOutputCleanup { source });
				}
			}
		}
		self.session
			.runtime_mut()
			.reset_after_live_config_retry()
			.map_err(|source| DevLifecycleError::Runtime {
				phase: "live-config retry reset",
				source,
			})?;
		Ok(LiveConfigRetryReset { dist_dir_changed })
	}

	pub(crate) fn activate_generation(
		&mut self,
		candidate: GenerationCandidate,
	) -> Result<PublishedDevGeneration, DevLifecycleError> {
		let effects = candidate.static_effects.clone();
		self.session.commit_generation(candidate);
		let committed =
			self.session
				.committed()
				.cloned()
				.ok_or(DevLifecycleError::MissingCommitted {
					phase: "dev activation",
				})?;
		let activation = crate::activation::activate_generation(
			&committed,
			crate::RunMode::Dev,
			self.session.runtime_mut(),
		)
		.map_err(|source| DevLifecycleError::Activation { source })?;
		let manifest = match activation {
			ActivationOutcome::Published(manifest) => *manifest,
			ActivationOutcome::Cancelled => {
				return Err(DevLifecycleError::DevActivationCancelled);
			}
		};
		self.session
			.set_committed_manifest(manifest)
			.map_err(|source| DevLifecycleError::Session {
				phase: "committing dev manifest",
				source,
			})?;
		Ok(PublishedDevGeneration {
			effects,
			static_metadata: committed.static_metadata().clone(),
		})
	}

	pub(crate) fn publish_static_update(
		&mut self,
		includes_client_revalidate: bool,
	) -> Result<PublishedDevGeneration, DevLifecycleError> {
		let committed =
			self.session
				.committed()
				.cloned()
				.ok_or(DevLifecycleError::MissingCommitted {
					phase: "static build",
				})?;
		let build_cancel = self.session.runtime().build_cancel();
		let prepared = prepare_static_build(&StaticBuildInput {
			config: committed.config(),
			previous_static: Some(committed.static_metadata()),
			build_cancel: build_cancel.clone(),
			includes_client_revalidate,
		})
		.map_err(|source| DevLifecycleError::StaticBuild { source })?;
		let published_static = match publish_static_outputs(
			committed.config(),
			committed.live(),
			prepared,
			&build_cancel,
		)
		.map_err(|source| DevLifecycleError::StaticBuild { source })?
		{
			StaticPublishOutcome::Published(published) => published,
			StaticPublishOutcome::CancelledBeforePublish => {
				return Err(DevLifecycleError::StaticPublishCancelled {
					phase: "before publishing static build outputs",
				});
			}
			StaticPublishOutcome::CancelledAfterPublish(_) => {
				return Err(DevLifecycleError::StaticPublishCancelled {
					phase: "after publishing static build outputs",
				});
			}
		};
		let committed =
			self.session
				.committed_mut()
				.ok_or(DevLifecycleError::MissingCommitted {
					phase: "static metadata replacement",
				})?;
		committed.replace_static_metadata(published_static.metadata);
		let committed = committed.clone();
		self.session
			.runtime_mut()
			.restart_watcher(&committed)
			.map_err(|source| DevLifecycleError::Runtime {
				phase: "watcher restart after static build",
				source,
			})?;
		let manifest = write_dev_manifest(&committed, self.session.runtime())?;
		self.session
			.set_committed_manifest(manifest)
			.map_err(|source| DevLifecycleError::Session {
				phase: "committing static-update manifest",
				source,
			})?;
		Ok(PublishedDevGeneration {
			effects: published_static.effects,
			static_metadata: committed.static_metadata().clone(),
		})
	}

	fn prepare_refresh_attempt(
		&mut self,
	) -> Result<ServerRefreshCandidateAttempt, DevLifecycleError> {
		let bootstrap_config =
			self.session
				.bootstrap_config()
				.map_err(|source| DevLifecycleError::Session {
					phase: "reading bootstrap config",
					source,
				})?;
		let candidate = crate::pipeline::prepare_generation_candidate(self.session)
			.map_err(|source| DevLifecycleError::Pipeline { source })?;
		if candidate.config == bootstrap_config {
			return Ok(ServerRefreshCandidateAttempt::Stable(Box::new(candidate)));
		}
		Ok(ServerRefreshCandidateAttempt::LiveConfigChanged(Box::new(
			LiveConfigRetryTransition {
				bootstrap_config,
				next_config: candidate.config,
			},
		)))
	}
}

#[derive(Clone, Debug, PartialEq)]
enum ServerRefreshCandidateAttempt {
	Stable(Box<GenerationCandidate>),
	LiveConfigChanged(Box<LiveConfigRetryTransition>),
}

fn write_dev_manifest(
	committed: &crate::generation::CommittedGeneration,
	runtime: &crate::runtime::DevRuntime,
) -> Result<Manifest, DevLifecycleError> {
	let vite_server_port =
		runtime
			.vite_server_port()
			.ok_or(DevLifecycleError::RuntimeInvariant {
				message: "Vite server port is not available for dev manifest",
			})?;
	let cfg = to_cfg(committed.config()).map_err(|source| DevLifecycleError::Config {
		phase: "dev manifest",
		source,
	})?;
	write_dev_manifest_json(
		&cfg,
		&DevManifestInput::from_generation_metadata(
			i32::from(vite_server_port),
			runtime
				.dev_mux_port_i32()
				.map_err(|source| DevLifecycleError::Runtime {
					phase: "dev manifest mux port",
					source,
				})?,
			runtime
				.dev_refresh_token()
				.map_err(|source| DevLifecycleError::Runtime {
					phase: "dev manifest refresh token",
					source,
				})?,
			committed.live(),
			committed.static_metadata(),
		),
	)
	.map_err(|source| DevLifecycleError::ManifestWrite { source })
}

#[derive(Debug)]
pub(crate) enum DevLifecycleError {
	Session {
		phase: &'static str,
		source: String,
	},
	Config {
		phase: &'static str,
		source: ConfigError,
	},
	Pipeline {
		source: PipelineError,
	},
	Activation {
		source: ActivationError,
	},
	StaticBuild {
		source: StaticBuildError,
	},
	StaticPublishCancelled {
		phase: &'static str,
	},
	DevLock {
		action: &'static str,
		source: String,
	},
	OldOutputCleanup {
		source: io::Error,
	},
	Runtime {
		phase: &'static str,
		source: String,
	},
	RuntimeInvariant {
		message: &'static str,
	},
	ManifestWrite {
		source: String,
	},
	MissingCommitted {
		phase: &'static str,
	},
	DevActivationCancelled,
}

impl fmt::Display for DevLifecycleError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Session { phase, source } => write!(f, "session error during {phase}: {source}"),
			Self::Config { phase, source } => {
				write!(f, "error converting config for {phase}: {source}")
			}
			Self::Pipeline { source } => write!(f, "{source}"),
			Self::Activation { source } => write!(f, "{source}"),
			Self::StaticBuild { source } => write!(f, "{source}"),
			Self::StaticPublishCancelled { phase } => write!(f, "build cancelled {phase}"),
			Self::DevLock { action, source } => write!(f, "error {action}: {source}"),
			Self::OldOutputCleanup { source } => {
				write!(f, "failed to clean up old .vorma directory: {source}")
			}
			Self::Runtime { phase, source } => write!(f, "runtime error during {phase}: {source}"),
			Self::RuntimeInvariant { message } => f.write_str(message),
			Self::ManifestWrite { source } => write!(f, "error writing dev manifest: {source}"),
			Self::MissingCommitted { phase } => {
				write!(f, "committed generation not available during {phase}")
			}
			Self::DevActivationCancelled => f.write_str("dev activation cannot be cancelled"),
		}
	}
}

impl std::error::Error for DevLifecycleError {}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::time::{SystemTime, UNIX_EPOCH};

	use paranoid::local_lock::ProcessLock;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::RunMode;
	use crate::cargo_target::CargoBinTarget;
	use crate::generation::{
		BuildArtifactMode, BuildArtifacts, GenerationCandidate, LiveMetadata, StaticEffects,
		StaticMetadata,
	};

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-dev-lifecycle-{name}-{nonce}"));
		fs::create_dir_all(&root).unwrap();
		root
	}

	fn config(root: &Path, dist_dir: &str) -> Config {
		Config {
			root_dir: root.to_path_buf(),
			dist_dir: dist_dir.to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
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

	fn build_entry() -> CargoBinTarget {
		CargoBinTarget {
			cargo_package: "example-app".to_owned(),
			cargo_bin: "example-build".to_owned(),
		}
	}

	fn acquired_dev_lock(config: &Config) -> ProcessLock {
		let cfg = to_cfg(config).unwrap();
		let mut lock = ProcessLock::new(cfg.dev_lock_out());
		lock.acquire().unwrap();
		lock
	}

	fn committed_static_session(config: Config, static_metadata: StaticMetadata) -> BuildSession {
		let mut session = BuildSession::new(config.clone(), build_entry(), RunMode::Dev);
		session.commit_generation(GenerationCandidate {
			config,
			live: LiveMetadata {
				root_document_hash_source: "document-hash-source".to_owned(),
				..LiveMetadata::default()
			},
			static_metadata,
			manifest: None,
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/example-build"),
				mode: BuildArtifactMode::Dev {
					app_server_executable: PathBuf::from("/tmp/example-server"),
				},
			},
			static_effects: StaticEffects::default(),
		});
		session
	}

	#[test]
	fn live_config_retry_transition_keeps_dev_lock_when_dist_dir_is_unchanged() {
		let root = temp_root("same-dist-lock");
		let bootstrap = config(&root, "dist");
		let mut next = config(&root, "dist");
		next.frontend_config.entry_file = "src/client/next-entry.tsx".to_owned();
		let mut session = BuildSession::new(bootstrap.clone(), build_entry(), RunMode::Dev);
		session
			.runtime_mut()
			.hold_dev_lock(acquired_dev_lock(&bootstrap));

		let reset = DevLifecycle::new(&mut session)
			.reset_after_config_change(&LiveConfigRetryTransition {
				bootstrap_config: bootstrap,
				next_config: next,
			})
			.unwrap();

		assert_eq!(
			reset,
			LiveConfigRetryReset {
				dist_dir_changed: false,
			}
		);
		assert!(session.runtime().has_dev_lock());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn live_config_retry_transition_swaps_dev_lock_and_removes_old_vorma_on_dist_change() {
		let root = temp_root("dist-lock-swap");
		let bootstrap = config(&root, "dist-a");
		let next = config(&root, "dist-b");
		let bootstrap_cfg = to_cfg(&bootstrap).unwrap();
		let old_vorma_out = PathBuf::from(bootstrap_cfg.vorma_out());
		fs::create_dir_all(&old_vorma_out).unwrap();
		fs::write(old_vorma_out.join("stale.txt"), "stale").unwrap();
		let mut session = BuildSession::new(bootstrap.clone(), build_entry(), RunMode::Dev);
		session
			.runtime_mut()
			.hold_dev_lock(acquired_dev_lock(&bootstrap));

		let reset = DevLifecycle::new(&mut session)
			.reset_after_config_change(&LiveConfigRetryTransition {
				bootstrap_config: bootstrap,
				next_config: next,
			})
			.unwrap();

		assert_eq!(
			reset,
			LiveConfigRetryReset {
				dist_dir_changed: true,
			}
		);
		assert!(session.runtime().has_dev_lock());
		assert!(!old_vorma_out.exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn static_build_failure_does_not_mutate_committed_generation() {
		let root = temp_root("static-failure-no-commit");
		fs::create_dir_all(root.join("public")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		fs::create_dir_all(root.join("src/client/vorma.gen.ts")).unwrap();
		let config = config(&root, "dist");
		let old_static = StaticMetadata {
			public_filemap: BTreeMap::from([("old.css".to_owned(), "/static/old.css".to_owned())]),
			critical_css: "old css".to_owned(),
			..StaticMetadata::default()
		};
		let mut session = committed_static_session(config, old_static.clone());

		assert!(
			DevLifecycle::new(&mut session)
				.publish_static_update(true)
				.is_err()
		);

		assert_eq!(session.committed().unwrap().static_metadata(), &old_static);
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn cancelled_static_build_does_not_mutate_committed_generation() {
		let root = temp_root("static-cancel-no-commit");
		fs::create_dir_all(root.join("public")).unwrap();
		fs::write(root.join("public/app.css"), "body{}").unwrap();
		let config = config(&root, "dist");
		let old_static = StaticMetadata {
			public_filemap: BTreeMap::from([("old.css".to_owned(), "/static/old.css".to_owned())]),
			critical_css: "old css".to_owned(),
			..StaticMetadata::default()
		};
		let mut session = committed_static_session(config, old_static.clone());
		session
			.runtime()
			.build_cancel()
			.store(true, std::sync::atomic::Ordering::SeqCst);

		assert!(
			DevLifecycle::new(&mut session)
				.publish_static_update(true)
				.is_err()
		);

		assert_eq!(session.committed().unwrap().static_metadata(), &old_static);
		fs::remove_dir_all(root).unwrap();
	}
}
