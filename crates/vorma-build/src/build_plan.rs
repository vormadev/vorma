//! Build/dev projection plan derived from the canonical graph bundle.

use std::collections::BTreeSet;

use vorma_contract::framework_graph::BuildInputConfig;

use crate::projection_compiler::{ProjectionBundle, StaticAssetContract};

/// Complete build/dev plan derived from one projection bundle.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildProjectionPlan {
	workspace: BuildWorkspacePlan,
	server_target: ServerTargetPlan,
	vite_inputs: ViteInputPlan,
	static_inputs: StaticInputPlan,
	generated_typescript: GeneratedTypeScriptPlan,
	dev_watch: BuildDevWatchPlan,
}

impl BuildProjectionPlan {
	/// Compile a complete build/dev plan from graph projections.
	pub fn compile(bundle: &ProjectionBundle) -> Result<Self, BuildPlanError> {
		let inputs = bundle
			.build_inputs()
			.ok_or(BuildPlanError::MissingBuildInputs)?;
		let server_target = ServerTargetPlan {
			cargo_package: inputs.server_target().cargo_package().to_owned(),
			cargo_bin: inputs.server_target().cargo_bin().to_owned(),
		};
		let workspace = BuildWorkspacePlan {
			root_dir: inputs.root_dir().to_owned(),
			dist_dir: inputs.dist_dir().to_owned(),
		};
		let frontend_inputs = inputs.frontend_inputs();
		let ui_variant = validate_ui_variant(frontend_inputs.ui_variant())?;
		let vite_inputs = ViteInputPlan {
			ui_variant: ui_variant.to_owned(),
			js_package_manager_base_cmd: frontend_inputs.js_package_manager_base_cmd().to_owned(),
			js_package_manager_dir: frontend_inputs.js_package_manager_dir().to_owned(),
			vite_config_file: frontend_inputs.vite_config_file().to_owned(),
			entry_file: frontend_inputs.entry_file().to_owned(),
			public_static_base: bundle.public_static_base().to_owned(),
			view_client_files: bundle
				.view_modules()
				.iter()
				.map(|view| view.client_file().to_owned())
				.collect::<BTreeSet<_>>()
				.into_iter()
				.collect(),
		};
		let static_inputs = StaticInputPlan {
			public_static_source_dir: frontend_inputs.public_static_source_dir().to_owned(),
			critical_css_file: frontend_inputs.critical_css_file().to_owned(),
			declared_assets: bundle.static_assets().to_vec(),
		};
		let generated_typescript = GeneratedTypeScriptPlan {
			output_file: inputs.generated_typescript_file().to_owned(),
		};
		let dev_watch = BuildDevWatchPlan::derive(inputs, bundle);
		Ok(Self {
			workspace,
			server_target,
			vite_inputs,
			static_inputs,
			generated_typescript,
			dev_watch,
		})
	}

	/// Build workspace path plan.
	pub fn workspace(&self) -> &BuildWorkspacePlan {
		&self.workspace
	}

	/// Cargo server target plan.
	pub fn server_target(&self) -> &ServerTargetPlan {
		&self.server_target
	}

	/// Vite input plan.
	pub fn vite_inputs(&self) -> &ViteInputPlan {
		&self.vite_inputs
	}

	/// Static and critical-CSS input plan.
	pub fn static_inputs(&self) -> &StaticInputPlan {
		&self.static_inputs
	}

	/// Generated TypeScript output plan.
	pub fn generated_typescript(&self) -> &GeneratedTypeScriptPlan {
		&self.generated_typescript
	}

	/// Typed dev watch plan.
	pub fn dev_watch(&self) -> &BuildDevWatchPlan {
		&self.dev_watch
	}
}

/// Build workspace projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildWorkspacePlan {
	root_dir: String,
	dist_dir: String,
}

impl BuildWorkspacePlan {
	/// Application root directory.
	pub fn root_dir(&self) -> &str {
		&self.root_dir
	}

	/// Build output directory.
	pub fn dist_dir(&self) -> &str {
		&self.dist_dir
	}
}

/// Cargo server target projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServerTargetPlan {
	cargo_package: String,
	cargo_bin: String,
}

impl ServerTargetPlan {
	/// Cargo package containing the server binary.
	pub fn cargo_package(&self) -> &str {
		&self.cargo_package
	}

	/// Cargo binary name for the server.
	pub fn cargo_bin(&self) -> &str {
		&self.cargo_bin
	}
}

/// Vite input projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViteInputPlan {
	ui_variant: String,
	js_package_manager_base_cmd: String,
	js_package_manager_dir: String,
	vite_config_file: String,
	entry_file: String,
	public_static_base: String,
	view_client_files: Vec<String>,
}

impl ViteInputPlan {
	/// Stable UI adapter variant label.
	#[cfg(test)]
	pub fn ui_variant(&self) -> &str {
		&self.ui_variant
	}

	/// JavaScript package-manager command prefix.
	pub fn js_package_manager_base_cmd(&self) -> &str {
		&self.js_package_manager_base_cmd
	}

	/// JavaScript package-manager working directory.
	pub fn js_package_manager_dir(&self) -> &str {
		&self.js_package_manager_dir
	}

	/// Vite config file.
	pub fn vite_config_file(&self) -> &str {
		&self.vite_config_file
	}

	/// Browser entry module file.
	pub fn entry_file(&self) -> &str {
		&self.entry_file
	}

	/// Public static base emitted into Vite/framework build config.
	pub fn public_static_base(&self) -> &str {
		&self.public_static_base
	}

	/// Route client modules that must become Vite inputs.
	pub fn view_client_files(&self) -> &[String] {
		&self.view_client_files
	}

	/// Frontend dependencies Vite should de-duplicate for this UI adapter.
	pub fn dedupe_list(&self) -> &'static [&'static str] {
		match self.ui_variant.as_str() {
			"react" => &["react", "react-dom"],
			"preact" => &[
				"preact",
				"preact/hooks",
				"@preact/signals",
				"preact/jsx-runtime",
				"preact/compat",
				"preact/test-utils",
			],
			"solid" => &["solid-js", "solid-js/web"],
			_ => &[],
		}
	}
}

/// Static input projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticInputPlan {
	public_static_source_dir: String,
	critical_css_file: String,
	declared_assets: Vec<StaticAssetContract>,
}

impl StaticInputPlan {
	/// User public static source directory.
	pub fn public_static_source_dir(&self) -> &str {
		&self.public_static_source_dir
	}

	/// Critical CSS source file.
	pub fn critical_css_file(&self) -> &str {
		&self.critical_css_file
	}

	/// Declared static asset source/public-output contracts.
	#[cfg(test)]
	pub fn declared_assets(&self) -> &[StaticAssetContract] {
		&self.declared_assets
	}
}

/// Generated TypeScript output projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GeneratedTypeScriptPlan {
	output_file: String,
}

impl GeneratedTypeScriptPlan {
	/// Generated TypeScript output file.
	pub fn output_file(&self) -> &str {
		&self.output_file
	}
}

/// Typed dev watch plan.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildDevWatchPlan {
	entries: Vec<BuildDevWatchEntry>,
}

impl BuildDevWatchPlan {
	/// Watch entries with their framework-level generation intent.
	pub fn entries(&self) -> &[BuildDevWatchEntry] {
		&self.entries
	}

	/*
	Critical-CSS imports are only known after bundling, so the compiled plan
	is extended per generation rather than at derive time (Go parity:
	css_files_to_watch was rebuilt every build).
	*/
	pub fn extended_with_critical_css_imports(&self, imports: &[String]) -> Self {
		let mut entries = self.entries.iter().cloned().collect::<BTreeSet<_>>();
		for import in imports {
			insert_watch_entry(&mut entries, import, BuildDevWatchIntent::CriticalCssInput);
		}
		Self {
			entries: entries.into_iter().collect(),
		}
	}

	#[cfg(test)]
	pub(crate) fn from_entries_for_test(entries: Vec<(String, BuildDevWatchIntent)>) -> Self {
		Self {
			entries: entries
				.into_iter()
				.map(|(source_path, intent)| BuildDevWatchEntry {
					source_path,
					intent,
				})
				.collect(),
		}
	}

	fn derive(inputs: &BuildInputConfig, bundle: &ProjectionBundle) -> Self {
		let frontend_inputs = inputs.frontend_inputs();
		let mut entries = BTreeSet::<BuildDevWatchEntry>::new();
		for source_path in inputs.dev_watch().watch_patterns() {
			insert_watch_entry(&mut entries, source_path, BuildDevWatchIntent::GeneralWatch);
		}
		for source_path in inputs.dev_watch().server_recompile_patterns() {
			insert_watch_entry(
				&mut entries,
				source_path,
				BuildDevWatchIntent::ServerRecompile,
			);
		}
		for source_path in inputs.dev_watch().client_revalidate_patterns() {
			insert_watch_entry(
				&mut entries,
				source_path,
				BuildDevWatchIntent::ClientRevalidate,
			);
		}
		insert_watch_entry(
			&mut entries,
			frontend_inputs.vite_config_file(),
			BuildDevWatchIntent::FrontendBuildInput,
		);
		insert_watch_entry(
			&mut entries,
			frontend_inputs.entry_file(),
			BuildDevWatchIntent::FrontendBuildInput,
		);
		for view in bundle.view_modules() {
			insert_watch_entry(
				&mut entries,
				view.client_file(),
				BuildDevWatchIntent::FrontendBuildInput,
			);
		}
		insert_watch_entry(
			&mut entries,
			frontend_inputs.public_static_source_dir(),
			BuildDevWatchIntent::PublicStaticInput,
		);
		for asset in bundle.static_assets() {
			insert_watch_entry(
				&mut entries,
				asset.source_path(),
				BuildDevWatchIntent::PublicStaticInput,
			);
		}
		insert_watch_entry(
			&mut entries,
			frontend_inputs.critical_css_file(),
			BuildDevWatchIntent::CriticalCssInput,
		);
		Self {
			entries: entries.into_iter().collect(),
		}
	}
}

/// One dev watch path/glob plus its generation intent.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct BuildDevWatchEntry {
	source_path: String,
	intent: BuildDevWatchIntent,
}

impl BuildDevWatchEntry {
	/// Watched source path or glob.
	pub fn source_path(&self) -> &str {
		&self.source_path
	}

	/// Framework-level generation intent for this watch entry.
	pub fn intent(&self) -> BuildDevWatchIntent {
		self.intent
	}
}

/// Framework-level reason a watched source can produce a new generation candidate.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub enum BuildDevWatchIntent {
	/// User-configured general watch path/glob.
	GeneralWatch,
	/// Rebuild the Rust application server.
	ServerRecompile,
	/// Revalidate active browser route data.
	ClientRevalidate,
	/// Rebuild browser-facing Vite inputs.
	FrontendBuildInput,
	/// Rebuild public static outputs.
	PublicStaticInput,
	/// Recompute critical CSS.
	CriticalCssInput,
}

/// Build plan projection error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BuildPlanError {
	/// The graph did not contain build/dev inputs.
	MissingBuildInputs,
	/// The graph contained an unknown UI adapter variant.
	InvalidUiVariant {
		/// Rejected UI adapter variant.
		value: String,
	},
}

impl std::fmt::Display for BuildPlanError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::MissingBuildInputs => f.write_str("framework graph is missing build inputs"),
			Self::InvalidUiVariant { value } => write!(f, "invalid UI variant {value:?}"),
		}
	}
}

impl std::error::Error for BuildPlanError {}

fn validate_ui_variant(value: &str) -> Result<&str, BuildPlanError> {
	match value {
		"react" | "preact" | "solid" => Ok(value),
		_ => Err(BuildPlanError::InvalidUiVariant {
			value: value.to_owned(),
		}),
	}
}

fn insert_watch_entry(
	entries: &mut BTreeSet<BuildDevWatchEntry>,
	source_path: &str,
	intent: BuildDevWatchIntent,
) {
	let source_path = source_path.trim();
	if source_path.is_empty() {
		return;
	}
	entries.insert(BuildDevWatchEntry {
		source_path: source_path.to_owned(),
		intent,
	});
}

#[cfg(test)]
mod tests {
	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget,
		StaticAssetDeclaration, ViewDeclaration,
	};

	use super::*;
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn projection_bundle_with_build_inputs() -> ProjectionBundle {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			".",
			"dist",
			FrontendBuildInputs::new(
				"react",
				"pnpm exec",
				"web",
				"vite.config.ts",
				"src/client/entry.tsx",
				"public",
				"src/client/critical.css",
			),
			"src/client/vorma.gen.ts",
			DevWatchConfig::new(
				vec!["src/**/*".to_owned()],
				vec!["src/**/*.rs".to_owned()],
				vec!["content/**/*.md".to_owned()],
			),
		));
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"src/client/root.view.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/about",
			"src/client/about.view.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("about"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		declarations.add_static_asset(StaticAssetDeclaration::new(
			"public/app.css",
			"/static/app.css",
		));
		ProjectionBundle::compile(&FrameworkGraph::compile(declarations).unwrap())
	}

	#[test]
	fn build_projection_plan_derives_vite_static_ts_server_and_watch_inputs() {
		let bundle = projection_bundle_with_build_inputs();

		let plan = BuildProjectionPlan::compile(&bundle).unwrap();

		assert_eq!(plan.workspace().root_dir(), ".");
		assert_eq!(plan.workspace().dist_dir(), "dist");
		assert_eq!(plan.server_target().cargo_package(), "example-app");
		assert_eq!(plan.server_target().cargo_bin(), "example-server");
		assert_eq!(
			plan.vite_inputs().js_package_manager_base_cmd(),
			"pnpm exec"
		);
		assert_eq!(plan.vite_inputs().ui_variant(), "react");
		assert_eq!(plan.vite_inputs().dedupe_list(), ["react", "react-dom"]);
		assert_eq!(plan.vite_inputs().js_package_manager_dir(), "web");
		assert_eq!(plan.vite_inputs().vite_config_file(), "vite.config.ts");
		assert_eq!(plan.vite_inputs().entry_file(), "src/client/entry.tsx");
		assert_eq!(plan.vite_inputs().public_static_base(), "/static/");
		assert_eq!(
			plan.vite_inputs().view_client_files(),
			["src/client/about.view.tsx", "src/client/root.view.tsx"]
		);
		assert_eq!(plan.static_inputs().public_static_source_dir(), "public");
		assert_eq!(
			plan.static_inputs().critical_css_file(),
			"src/client/critical.css"
		);
		assert_eq!(
			plan.static_inputs().declared_assets()[0].source_path(),
			"public/app.css"
		);
		assert_eq!(
			plan.generated_typescript().output_file(),
			"src/client/vorma.gen.ts"
		);
		assert!(plan.dev_watch().entries().contains(&BuildDevWatchEntry {
			source_path: "src/**/*.rs".to_owned(),
			intent: BuildDevWatchIntent::ServerRecompile,
		}));
		assert!(plan.dev_watch().entries().contains(&BuildDevWatchEntry {
			source_path: "content/**/*.md".to_owned(),
			intent: BuildDevWatchIntent::ClientRevalidate,
		}));
		assert!(plan.dev_watch().entries().contains(&BuildDevWatchEntry {
			source_path: "src/client/root.view.tsx".to_owned(),
			intent: BuildDevWatchIntent::FrontendBuildInput,
		}));
		assert!(plan.dev_watch().entries().contains(&BuildDevWatchEntry {
			source_path: "public".to_owned(),
			intent: BuildDevWatchIntent::PublicStaticInput,
		}));
		assert!(plan.dev_watch().entries().contains(&BuildDevWatchEntry {
			source_path: "src/client/critical.css".to_owned(),
			intent: BuildDevWatchIntent::CriticalCssInput,
		}));
	}

	#[test]
	fn build_projection_plan_requires_graph_owned_build_inputs() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		let bundle = ProjectionBundle::compile(&FrameworkGraph::compile(declarations).unwrap());

		let error = BuildProjectionPlan::compile(&bundle).unwrap_err();

		assert_eq!(error, BuildPlanError::MissingBuildInputs);
	}

	#[test]
	fn build_projection_plan_rejects_unknown_ui_variant() {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			".",
			"dist",
			FrontendBuildInputs::new(
				"jquery",
				"pnpm",
				".",
				"vite.config.ts",
				"src/client/entry.tsx",
				"public",
				"src/client/critical.css",
			),
			"src/client/vorma.gen.ts",
			DevWatchConfig::default(),
		));
		let bundle = ProjectionBundle::compile(
			&FrameworkGraph::compile(FrameworkDeclarations::new(config)).unwrap(),
		);

		assert_eq!(
			BuildProjectionPlan::compile(&bundle).unwrap_err(),
			BuildPlanError::InvalidUiVariant {
				value: "jquery".to_owned()
			}
		);
	}

	#[test]
	fn extended_watch_plan_adds_critical_css_import_entries() {
		let plan = BuildDevWatchPlan::from_entries_for_test(vec![(
			"styles/critical.css".to_owned(),
			BuildDevWatchIntent::CriticalCssInput,
		)]);
		let extended = plan.extended_with_critical_css_imports(&[
			"/workspace/app/styles/base.css".to_owned(),
			"styles/critical.css".to_owned(),
		]);

		assert_eq!(extended.entries().len(), 2);
		assert!(extended.entries().iter().any(|entry| {
			entry.source_path() == "/workspace/app/styles/base.css"
				&& entry.intent() == BuildDevWatchIntent::CriticalCssInput
		}));
		/*
		The original plan is untouched; extension builds a fresh plan per
		generation.
		*/
		assert_eq!(plan.entries().len(), 1);
	}
}
