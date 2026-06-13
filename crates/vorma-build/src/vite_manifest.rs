//! Vite production manifest projection.

use std::collections::{BTreeMap, BTreeSet};
use std::ops::Deref;
use std::path::{Path, PathBuf};

use path_slash::PathExt;
use serde::{Deserialize, Serialize};

use crate::build_output::resolve_workspace_path;
use crate::build_plan::BuildProjectionPlan;
use crate::projection_compiler::{ClientCoreArtifacts, ClientModuleArtifacts, ProjectionBundle};

/// Source filename emitted by wasm-bindgen for Vorma client matcher WASM.
pub const CLIENT_CORE_WASM_SOURCE_FILENAME: &str = "vorma_client_wasm_bg.wasm";

/// One entry from a Vite manifest.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ViteManifestChunk {
	/// Source module path recorded by Vite.
	#[serde(default)]
	pub src: String,
	/// Output file path.
	#[serde(default)]
	pub file: String,
	/// CSS bundle output paths directly associated with this chunk.
	#[serde(default)]
	pub css: Vec<String>,
	/// Asset output paths directly associated with this chunk.
	#[serde(default)]
	pub assets: Vec<String>,
	/// Whether this chunk is an entry chunk.
	#[serde(default)]
	pub is_entry: bool,
	/// Vite chunk name.
	#[serde(default)]
	pub name: String,
	/// Whether this chunk is a dynamic entry chunk.
	#[serde(default)]
	pub is_dynamic_entry: bool,
	/// Static imports referenced by this chunk.
	#[serde(default)]
	pub imports: Vec<String>,
	/// Dynamic imports referenced by this chunk.
	#[serde(default)]
	pub dynamic_imports: Vec<String>,
}

/// Parsed Vite manifest keyed by source import path.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(transparent)]
pub struct ViteManifest(BTreeMap<String, ViteManifestChunk>);

impl ViteManifest {
	/// Parse a Vite manifest from JSON bytes.
	pub fn from_json_slice(bytes: &[u8]) -> Result<Self, ViteManifestError> {
		serde_json::from_slice(bytes).map_err(|source| ViteManifestError::Decode {
			message: source.to_string(),
		})
	}

	/// Read and parse a Vite manifest file.
	pub fn read(path: impl AsRef<Path>) -> Result<Self, ViteManifestError> {
		let path = path.as_ref();
		let bytes = std::fs::read(path).map_err(|source| ViteManifestError::Read {
			path: path.display().to_string(),
			message: source.to_string(),
		})?;
		Self::from_json_slice(&bytes)
	}

	/// Find transitive static module and CSS dependencies for one import path.
	pub fn find_all_deps(&self, import_path: &str) -> Result<ViteDeps, ViteManifestError> {
		let mut seen = BTreeSet::new();
		let mut ordered = Vec::new();
		self.recurse(import_path, &mut seen, &mut ordered)?;

		let mut seen_modules = BTreeSet::new();
		let mut modules = Vec::with_capacity(ordered.len());
		let mut seen_css_bundles = BTreeSet::new();
		let mut css_bundles = Vec::with_capacity(ordered.len());
		for import_path in &ordered {
			let chunk = self
				.0
				.get(import_path)
				.expect("dependency walk only records existing chunks");
			if seen_modules.insert(chunk.file.clone()) {
				modules.push(chunk.file.clone());
			}
			for css in &chunk.css {
				if seen_css_bundles.insert(css.clone()) {
					css_bundles.push(css.clone());
				}
			}
		}
		Ok(ViteDeps {
			import_path: import_path.to_owned(),
			modules,
			css_bundles,
		})
	}

	fn recurse(
		&self,
		import_path: &str,
		seen: &mut BTreeSet<String>,
		ordered: &mut Vec<String>,
	) -> Result<(), ViteManifestError> {
		if !seen.insert(import_path.to_owned()) {
			return Ok(());
		}
		let Some(chunk) = self.0.get(import_path) else {
			return Err(ViteManifestError::MissingImport {
				import_path: import_path.to_owned(),
			});
		};
		ordered.push(import_path.to_owned());
		for import_path in &chunk.imports {
			self.recurse(import_path, seen, ordered)?;
		}
		Ok(())
	}
}

impl From<BTreeMap<String, ViteManifestChunk>> for ViteManifest {
	fn from(value: BTreeMap<String, ViteManifestChunk>) -> Self {
		Self(value)
	}
}

impl Deref for ViteManifest {
	type Target = BTreeMap<String, ViteManifestChunk>;

	fn deref(&self) -> &Self::Target {
		&self.0
	}
}

/// Transitive Vite dependencies for one manifest import path.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct ViteDeps {
	/// Import path used as the dependency root.
	pub import_path: String,
	/// Ordered transitive module output paths.
	pub modules: Vec<String>,
	/// Ordered transitive CSS bundle output paths.
	pub css_bundles: Vec<String>,
}

/// Vite build artifacts projected into Vorma generation artifacts.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViteBuildArtifacts {
	public_filepaths: Vec<String>,
	client_entry: ClientModuleArtifacts,
	client_core_assets: ClientCoreArtifacts,
	view_module_outputs: BTreeMap<String, ClientModuleArtifacts>,
}

impl ViteBuildArtifacts {
	/// Manifest public file paths produced by Vite.
	pub fn public_filepaths(&self) -> &[String] {
		&self.public_filepaths
	}

	/// Browser entry module artifacts.
	pub fn client_entry(&self) -> &ClientModuleArtifacts {
		&self.client_entry
	}

	/// Vorma client core module and WASM artifacts.
	pub fn client_core_assets(&self) -> &ClientCoreArtifacts {
		&self.client_core_assets
	}

	/// View pattern to browser module artifacts.
	pub fn view_module_outputs(&self) -> &BTreeMap<String, ClientModuleArtifacts> {
		&self.view_module_outputs
	}
}

/// Project Vite manifest outputs into build-generation artifacts.
pub fn project_vite_manifest(
	bundle: &ProjectionBundle,
	plan: &BuildProjectionPlan,
	manifest: &ViteManifest,
) -> Result<ViteBuildArtifacts, ViteManifestError> {
	let entry_key = manifest_import_key(plan, plan.vite_inputs().entry_file())?;
	let client_entry = client_module_artifacts(
		plan.vite_inputs().public_static_base(),
		manifest,
		&entry_key,
	)?;
	let client_core_assets = client_core_assets(plan.vite_inputs().public_static_base(), manifest)?;
	let mut view_module_outputs = BTreeMap::new();
	for view in bundle.view_modules() {
		let import_key = manifest_import_key(plan, view.client_file())?;
		view_module_outputs.insert(
			view.pattern().to_owned(),
			client_module_artifacts(
				plan.vite_inputs().public_static_base(),
				manifest,
				&import_key,
			)?,
		);
	}
	Ok(ViteBuildArtifacts {
		public_filepaths: vite_public_filepaths(plan.vite_inputs().public_static_base(), manifest)?,
		client_entry,
		client_core_assets,
		view_module_outputs,
	})
}

/// Vite manifest projection error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ViteManifestError {
	/// Manifest file could not be read.
	Read {
		/// Manifest path.
		path: String,
		/// Filesystem error message.
		message: String,
	},
	/// Manifest JSON could not be decoded.
	Decode {
		/// JSON error message.
		message: String,
	},
	/// A manifest import was referenced but absent.
	MissingImport {
		/// Missing manifest import path.
		import_path: String,
	},
	/// A configured app import path could not be expressed relative to the JS package root.
	ImportPath {
		/// Rejected import path.
		path: String,
	},
	/// A Vite output path was invalid.
	InvalidOutputPath {
		/// Output path label.
		label: &'static str,
		/// Rejected output path.
		path: String,
	},
	/// Vorma client core WASM asset was not found.
	MissingClientCoreWasm,
	/// Vorma client core WASM wrapper module was not found.
	MissingClientCoreModule,
}

impl std::fmt::Display for ViteManifestError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Read { path, message } => write!(f, "read Vite manifest {path}: {message}"),
			Self::Decode { message } => write!(f, "decode Vite manifest: {message}"),
			Self::MissingImport { import_path } => {
				write!(
					f,
					"Vite manifest import {import_path:?} is referenced but missing"
				)
			}
			Self::ImportPath { path } => {
				write!(f, "error getting Vite manifest import key for {path}")
			}
			Self::InvalidOutputPath { label, path } => {
				write!(
					f,
					"Vite manifest {label} contains an invalid output path: {path:?}"
				)
			}
			Self::MissingClientCoreWasm => {
				f.write_str("Vite manifest does not contain Vorma client WASM asset")
			}
			Self::MissingClientCoreModule => {
				f.write_str("Vite manifest does not contain Vorma client WASM wrapper module")
			}
		}
	}
}

impl std::error::Error for ViteManifestError {}

fn client_module_artifacts(
	public_static_base: &str,
	manifest: &ViteManifest,
	import_path: &str,
) -> Result<ClientModuleArtifacts, ViteManifestError> {
	let own_chunk = manifest
		.get(import_path)
		.ok_or_else(|| ViteManifestError::MissingImport {
			import_path: import_path.to_owned(),
		})?;
	let deps = manifest.find_all_deps(import_path)?;
	let dep_urls = deps
		.modules
		.iter()
		.map(|path| vite_public_url(public_static_base, path, "dependency module file"))
		.collect::<Result<Vec<_>, _>>()?;
	let css_bundle_urls = deps
		.css_bundles
		.iter()
		.map(|path| vite_public_url(public_static_base, path, "CSS bundle file"))
		.collect::<Result<Vec<_>, _>>()?;
	Ok(ClientModuleArtifacts::new(
		vite_public_url(public_static_base, &own_chunk.file, "module file")?,
		dep_urls,
		css_bundle_urls,
	))
}

fn client_core_assets(
	public_static_base: &str,
	manifest: &ViteManifest,
) -> Result<ClientCoreArtifacts, ViteManifestError> {
	let mut wasm_file = None;
	for (key, chunk) in manifest.iter() {
		let source = if chunk.src.is_empty() {
			key.as_str()
		} else {
			chunk.src.as_str()
		};
		if slash_basename(source) == CLIENT_CORE_WASM_SOURCE_FILENAME {
			wasm_file = Some(validate_vite_output_path(
				&chunk.file,
				"Vorma client WASM file",
			)?);
			break;
		}
	}
	let wasm_file = wasm_file.ok_or(ViteManifestError::MissingClientCoreWasm)?;
	for chunk in manifest.values() {
		if chunk.assets.iter().any(|asset| asset == wasm_file) {
			return Ok(ClientCoreArtifacts::new(
				vite_public_url(public_static_base, &chunk.file, "Vorma client module file")?,
				vite_public_url(public_static_base, wasm_file, "Vorma client WASM file")?,
			));
		}
	}
	Err(ViteManifestError::MissingClientCoreModule)
}

fn vite_public_filepaths(
	public_static_base: &str,
	manifest: &ViteManifest,
) -> Result<Vec<String>, ViteManifestError> {
	let mut paths = BTreeSet::new();
	for chunk in manifest.values() {
		paths.insert(vite_public_url(
			public_static_base,
			&chunk.file,
			"chunk file",
		)?);
		for css in &chunk.css {
			paths.insert(vite_public_url(public_static_base, css, "CSS bundle file")?);
		}
		for asset in &chunk.assets {
			paths.insert(vite_public_url(public_static_base, asset, "asset file")?);
		}
	}
	Ok(paths.into_iter().collect())
}

fn manifest_import_key(
	plan: &BuildProjectionPlan,
	import_path: &str,
) -> Result<String, ViteManifestError> {
	let root_dir = PathBuf::from(plan.workspace().root_dir());
	let package_root =
		resolve_workspace_path(&root_dir, plan.vite_inputs().js_package_manager_dir());
	let import_path = resolve_workspace_path(&root_dir, import_path);
	pathdiff::diff_paths(&import_path, &package_root)
		.map(|path| path.to_slash_lossy().into_owned())
		.ok_or_else(|| ViteManifestError::ImportPath {
			path: import_path.display().to_string(),
		})
}

fn vite_public_url(
	public_static_base: &str,
	path: &str,
	label: &'static str,
) -> Result<String, ViteManifestError> {
	let path = validate_vite_output_path(path, label)?;
	if public_static_base == "/" {
		return Ok(format!("/{path}"));
	}
	Ok(format!("{public_static_base}{path}"))
}

fn validate_vite_output_path<'a>(
	path: &'a str,
	label: &'static str,
) -> Result<&'a str, ViteManifestError> {
	if path.is_empty()
		|| path.starts_with('/')
		|| path.contains('\\')
		|| path
			.split('/')
			.any(|segment| segment.is_empty() || segment == "." || segment == "..")
	{
		return Err(ViteManifestError::InvalidOutputPath {
			label,
			path: path.to_owned(),
		});
	}
	Ok(path)
}

fn slash_basename(path: &str) -> &str {
	path.rsplit('/').next().unwrap_or(path)
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};

	use super::*;
	use crate::test_support::route_type_contract;

	const TEST_CARGO_PACKAGE: &str = "example-app";
	const TEST_CARGO_BIN: &str = "example-server";
	const TEST_ROOT_DIR: &str = ".";
	const TEST_DIST_DIR: &str = "dist";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_ENTRY_FILE: &str = "src/entry.tsx";
	const TEST_VIEW_FILE: &str = "src/root.tsx";
	const TEST_WASM_FILE: &str = "assets/vorma_client_wasm_bg.wasm";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn bundle(
		js_package_manager_dir: &str,
		entry_file: &str,
		view_file: &str,
	) -> (ProjectionBundle, BuildProjectionPlan) {
		let mut declarations = FrameworkDeclarations::new(
			FrameworkConfig::new(TEST_PUBLIC_STATIC_BASE).with_build_inputs(BuildInputConfig::new(
				ServerBuildTarget::new(TEST_CARGO_PACKAGE, TEST_CARGO_BIN),
				TEST_ROOT_DIR,
				TEST_DIST_DIR,
				FrontendBuildInputs::new(
					"react",
					"pnpm",
					js_package_manager_dir,
					"vite.config.ts",
					entry_file,
					"public",
					"src/critical.css",
				),
				"src/vorma.gen.ts",
				DevWatchConfig::default(),
			)),
		);
		declarations.add_view(ViewDeclaration::new(
			"/",
			view_file,
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		let bundle = ProjectionBundle::compile(&FrameworkGraph::compile(declarations).unwrap());
		let plan = BuildProjectionPlan::compile(&bundle).unwrap();
		(bundle, plan)
	}

	fn insert_client_core(manifest: &mut BTreeMap<String, ViteManifestChunk>) {
		manifest.insert(
			"src/wasm-wrapper.ts".to_owned(),
			ViteManifestChunk {
				file: "assets/wasm-wrapper.js".to_owned(),
				assets: vec![TEST_WASM_FILE.to_owned()],
				..ViteManifestChunk::default()
			},
		);
		manifest.insert(
			"pkg/vorma_client_wasm_bg.wasm".to_owned(),
			ViteManifestChunk {
				src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
				file: TEST_WASM_FILE.to_owned(),
				..ViteManifestChunk::default()
			},
		);
	}

	#[test]
	fn vite_manifest_projects_entry_views_core_and_public_outputs() {
		let (bundle, plan) = bundle(".", TEST_ENTRY_FILE, TEST_VIEW_FILE);
		let mut manifest = BTreeMap::from([
			(
				TEST_ENTRY_FILE.to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					imports: vec!["src/shared.ts".to_owned()],
					css: vec!["assets/entry.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				TEST_VIEW_FILE.to_owned(),
				ViteManifestChunk {
					file: "assets/root.js".to_owned(),
					imports: vec!["src/shared.ts".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"src/shared.ts".to_owned(),
				ViteManifestChunk {
					file: "assets/shared.js".to_owned(),
					css: vec!["assets/shared.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
		]);
		insert_client_core(&mut manifest);
		let manifest = ViteManifest::from(manifest);

		let artifacts = project_vite_manifest(&bundle, &plan, &manifest).unwrap();

		assert_eq!(
			artifacts.client_entry().import_url(),
			"/static/assets/entry.js"
		);
		assert_eq!(
			artifacts.client_entry().dep_urls(),
			["/static/assets/entry.js", "/static/assets/shared.js"]
		);
		assert_eq!(
			artifacts.client_entry().css_bundle_urls(),
			["/static/assets/entry.css", "/static/assets/shared.css"]
		);
		assert_eq!(
			artifacts.view_module_outputs()["/"].import_url(),
			"/static/assets/root.js"
		);
		assert_eq!(
			artifacts.client_core_assets().wasm_url(),
			"/static/assets/vorma_client_wasm_bg.wasm"
		);
		assert!(
			artifacts
				.public_filepaths()
				.contains(&"/static/assets/shared.css".to_owned())
		);
	}

	#[test]
	fn vite_manifest_keys_are_relative_to_js_package_manager_dir() {
		let (bundle, plan) = bundle("frontend", "app/src/entry.tsx", "app/src/root.tsx");
		let mut manifest = BTreeMap::from([
			(
				"../app/src/entry.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/entry.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
			(
				"../app/src/root.tsx".to_owned(),
				ViteManifestChunk {
					file: "assets/root.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
		]);
		insert_client_core(&mut manifest);

		let artifacts =
			project_vite_manifest(&bundle, &plan, &ViteManifest::from(manifest)).unwrap();

		assert_eq!(
			artifacts.client_entry().import_url(),
			"/static/assets/entry.js"
		);
		assert_eq!(
			artifacts.view_module_outputs()["/"].import_url(),
			"/static/assets/root.js"
		);
	}

	#[test]
	fn vite_manifest_rejects_invalid_output_paths() {
		let (bundle, plan) = bundle(".", TEST_ENTRY_FILE, TEST_VIEW_FILE);
		let mut manifest = BTreeMap::from([
			(
				TEST_ENTRY_FILE.to_owned(),
				ViteManifestChunk {
					file: "../entry.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
			(
				TEST_VIEW_FILE.to_owned(),
				ViteManifestChunk {
					file: "assets/root.js".to_owned(),
					..ViteManifestChunk::default()
				},
			),
		]);
		insert_client_core(&mut manifest);

		let error =
			project_vite_manifest(&bundle, &plan, &ViteManifest::from(manifest)).unwrap_err();

		assert!(matches!(error, ViteManifestError::InvalidOutputPath { .. }));
	}
}
