use std::fmt;

use crate::RunMode;
use crate::config::to_cfg;
use crate::generation::GenerationCandidate;
use crate::generation_workspace::prepare_generation_output_layout;
use crate::live_refresh::{prepare_dev_live_generation, prepare_prod_live_generation};
use crate::session::BuildSession;
use crate::static_build::{
	StaticBuildError, StaticBuildInput, StaticPublishOutcome, prepare_static_build,
	publish_static_outputs,
};
use paranoid::local_lock::ProcessLock;

pub(crate) fn prepare_generation_candidate(
	session: &mut BuildSession,
) -> Result<GenerationCandidate, PipelineError> {
	let bootstrap_config = session
		.bootstrap_config_view()
		.map_err(|source| PipelineError::BootstrapConfig { source })?
		.source()
		.clone();
	let bootstrap_layout = to_cfg(&bootstrap_config).map_err(|source| PipelineError::Config {
		phase: "bootstrap layout",
		source,
	})?;
	prepare_generation_output_layout(&bootstrap_layout).map_err(|source| {
		PipelineError::GenerationOutputLayout {
			phase: "bootstrap generation",
			source,
		}
	})?;
	if session.mode() == RunMode::Dev {
		ensure_dev_lock(&bootstrap_layout, session)?;
	}

	let build_cancel = session.runtime().build_cancel();
	let live_generation = match session.mode() {
		RunMode::Build => {
			prepare_prod_live_generation(&bootstrap_layout, session.build_entry(), &build_cancel)
				.map_err(|source| PipelineError::LiveGeneration {
					mode: RunMode::Build,
					source,
				})?
		}
		RunMode::Dev => {
			prepare_dev_live_generation(&bootstrap_layout, session.build_entry(), &build_cancel)
				.map_err(|source| PipelineError::LiveGeneration {
					mode: RunMode::Dev,
					source,
				})?
		}
	};

	let current_layout =
		to_cfg(&live_generation.config).map_err(|source| PipelineError::Config {
			phase: "live generation layout",
			source,
		})?;
	prepare_generation_output_layout(&current_layout).map_err(|source| {
		PipelineError::GenerationOutputLayout {
			phase: "live generation",
			source,
		}
	})?;

	let previous_static = session
		.committed()
		.map(|generation| generation.static_metadata());
	let static_input = StaticBuildInput {
		config: &live_generation.config,
		previous_static,
		build_cancel: build_cancel.clone(),
		includes_client_revalidate: false,
	};
	let prepared_static_outputs = prepare_static_build(&static_input)
		.map_err(|source| PipelineError::StaticBuild { source })?;
	let published_static_outputs = match publish_static_outputs(
		&live_generation.config,
		&live_generation.live,
		prepared_static_outputs,
		&build_cancel,
	)
	.map_err(|source| PipelineError::StaticBuild { source })?
	{
		StaticPublishOutcome::Published(published) => published,
		StaticPublishOutcome::CancelledBeforePublish => {
			return Err(PipelineError::StaticPublishCancelled {
				phase: "before publishing static build outputs",
			});
		}
		StaticPublishOutcome::CancelledAfterPublish(_) => {
			return Err(PipelineError::StaticPublishCancelled {
				phase: "after publishing static build outputs",
			});
		}
	};

	Ok(GenerationCandidate {
		config: live_generation.config,
		live: live_generation.live,
		static_metadata: published_static_outputs.metadata,
		manifest: None,
		artifacts: live_generation.artifacts,
		static_effects: published_static_outputs.effects,
	})
}

fn ensure_dev_lock(
	cfg: &crate::config::VormaCfg<'_>,
	session: &mut BuildSession,
) -> Result<(), PipelineError> {
	if session.runtime().has_dev_lock() {
		return Ok(());
	}
	let mut dev_lock = ProcessLock::new(cfg.dev_lock_out());
	dev_lock
		.acquire()
		.map_err(|source| PipelineError::DevLock {
			source: source.to_string(),
		})?;
	session.runtime_mut().hold_dev_lock(dev_lock);
	Ok(())
}

#[derive(Debug)]
pub(crate) enum PipelineError {
	BootstrapConfig {
		source: String,
	},
	Config {
		phase: &'static str,
		source: crate::config::ConfigError,
	},
	GenerationOutputLayout {
		phase: &'static str,
		source: String,
	},
	DevLock {
		source: String,
	},
	LiveGeneration {
		mode: RunMode,
		source: String,
	},
	StaticBuild {
		source: StaticBuildError,
	},
	StaticPublishCancelled {
		phase: &'static str,
	},
}

impl fmt::Display for PipelineError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::BootstrapConfig { source } => f.write_str(source),
			Self::Config { phase, source } => {
				write!(f, "error converting config for {phase}: {source}")
			}
			Self::GenerationOutputLayout { phase, source } => {
				write!(f, "error preparing output layout for {phase}: {source}")
			}
			Self::DevLock { source } => write!(f, "error acquiring dev lock: {source}"),
			Self::LiveGeneration { mode, source } => {
				write!(f, "error preparing {mode:?} live generation: {source}")
			}
			Self::StaticBuild { source } => write!(f, "{source}"),
			Self::StaticPublishCancelled { phase } => write!(f, "build cancelled {phase}"),
		}
	}
}

impl std::error::Error for PipelineError {}
