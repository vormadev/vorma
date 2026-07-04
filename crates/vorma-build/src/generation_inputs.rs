//! Shared generation input preparation.
//!
//! [`prepare_build_inputs`] is the one function both
//! [`crate::dev_build`] and [`crate::production_build`] call to do the part
//! of assembling a generation that is identical between dev and
//! production: compile graph projections, derive the build plan, prepare
//! public static outputs, render generated TypeScript, bundle critical CSS,
//! and project the Vite plugin config. What differs between dev and
//! production — resolving client module URLs against a running Vite dev
//! server versus a finished production build — happens after this, in
//! [`crate::dev_generation`] and [`crate::production_generation`]
//! respectively.

use vorma::build_interface::AppBuildContract;
use vorma_contract::framework_graph::FrameworkGraph;

use crate::build_plan::{BuildPlanError, BuildProjectionPlan};
use crate::projection_compiler::ProjectionBundle;
use crate::static_outputs::{
	CriticalCssBundle, PreparedPublicStaticOutputs, PublicStaticOutputError, bundle_critical_css,
	prepare_public_static_outputs,
};
use crate::typescript_contracts::{
	GeneratedTypeScriptContracts, TypeScriptContractError, render_typescript_contracts,
};
use crate::vite_plugin_contract::{VitePluginConfig, VitePluginConfigError};

/// Prepared build inputs before Vite or dev runtime URLs are resolved: the
/// shared assembly step's output, returned by [`prepare_build_inputs`] and
/// [`prepare_build_inputs_from_graph`]. See the module docs above for where
/// this fits between graph compilation and generation-specific finishing.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedBuildInputs {
	pub(crate) bundle: ProjectionBundle,
	pub(crate) plan: BuildProjectionPlan,
	pub(crate) typescript_contracts: GeneratedTypeScriptContracts,
	pub(crate) public_static_outputs: PreparedPublicStaticOutputs,
	pub(crate) critical_css: CriticalCssBundle,
	pub(crate) vite_plugin_config: VitePluginConfig,
}

impl PreparedBuildInputs {
	/// Compiled graph projection bundle.
	pub fn bundle(&self) -> &ProjectionBundle {
		&self.bundle
	}

	/// Build projection plan.
	pub fn plan(&self) -> &BuildProjectionPlan {
		&self.plan
	}

	/// Generated TypeScript contracts needed before Vite imports app modules.
	pub fn typescript_contracts(&self) -> &GeneratedTypeScriptContracts {
		&self.typescript_contracts
	}

	/// Prepared public static outputs.
	pub fn public_static_outputs(&self) -> &PreparedPublicStaticOutputs {
		&self.public_static_outputs
	}

	/// Bundled critical CSS.
	pub fn critical_css(&self) -> &CriticalCssBundle {
		&self.critical_css
	}

	/// Vite plugin config response for this generation candidate.
	pub fn vite_plugin_config(&self) -> &VitePluginConfig {
		&self.vite_plugin_config
	}
}

/// Prepare build inputs that must exist before Vite or dev runtime URL
/// projection runs. See the module docs above for exactly what this
/// assembles.
pub fn prepare_build_inputs(
	app: &AppBuildContract,
) -> Result<PreparedBuildInputs, BuildInputError> {
	prepare_build_inputs_from_graph(app.graph())
}

/// Prepare build inputs from an already-normalized framework graph: the
/// [`prepare_build_inputs`] logic for callers (the dev loop's live-state
/// path) that already have a [`FrameworkGraph`] without an
/// [`AppBuildContract`] wrapping it.
pub fn prepare_build_inputs_from_graph(
	graph: &FrameworkGraph,
) -> Result<PreparedBuildInputs, BuildInputError> {
	let bundle = ProjectionBundle::compile(graph);
	let plan = BuildProjectionPlan::compile(&bundle)
		.map_err(|source| BuildInputError::BuildPlan { source })?;
	let public_static = prepare_public_static_outputs(&plan)
		.map_err(|source| BuildInputError::PublicStatic { source })?;
	let typescript_contracts = render_typescript_contracts(&bundle, public_static.public_filemap())
		.map_err(|source| BuildInputError::TypeScriptContracts { source })?;
	let critical_css = bundle_critical_css(&plan, public_static.public_filemap())
		.map_err(|source| BuildInputError::PublicStatic { source })?;
	let vite_plugin_config = VitePluginConfig::from_build_plan(&plan)
		.map_err(|source| BuildInputError::VitePluginConfig { source })?;

	Ok(PreparedBuildInputs {
		bundle,
		plan,
		typescript_contracts,
		public_static_outputs: public_static,
		critical_css,
		vite_plugin_config,
	})
}

/// Error from [`prepare_build_inputs`] or [`prepare_build_inputs_from_graph`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BuildInputError {
	/// Build/dev projection plan failed.
	BuildPlan {
		/// Source build plan error.
		source: BuildPlanError,
	},
	/// Generated TypeScript rendering failed.
	TypeScriptContracts {
		/// Source TypeScript rendering error.
		source: TypeScriptContractError,
	},
	/// Public static or critical CSS processing failed.
	PublicStatic {
		/// Source static output error.
		source: PublicStaticOutputError,
	},
	/// Vite plugin config projection failed.
	VitePluginConfig {
		/// Source Vite plugin config error.
		source: VitePluginConfigError,
	},
}

impl std::fmt::Display for BuildInputError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::BuildPlan { source } => write!(f, "{source}"),
			Self::TypeScriptContracts { source } => write!(f, "{source}"),
			Self::PublicStatic { source } => write!(f, "{source}"),
			Self::VitePluginConfig { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for BuildInputError {}
