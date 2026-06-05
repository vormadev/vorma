use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::ops::Deref;
use std::path::Path;

use serde::{Deserialize, Serialize};

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

/// Transitive module and CSS dependencies for one Vite import path.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct DepsResult {
	/// Import path used as the dependency root.
	pub import_path: String,
	/// Ordered transitive module output paths.
	pub modules: Vec<String>,
	/// Ordered transitive CSS bundle output paths.
	pub css_bundles: Vec<String>,
}

impl ViteManifest {
	/// Read and parse a Vite manifest file.
	pub fn read(manifest_path: impl AsRef<Path>) -> Result<Self, String> {
		let data = fs::read(manifest_path).map_err(|err| err.to_string())?;
		serde_json::from_slice(&data).map_err(|err| err.to_string())
	}

	/// Find the transitive static module and CSS dependencies for one import path.
	pub fn find_all_deps(&self, import_path: &str) -> Result<DepsResult, String> {
		let mut seen = BTreeSet::new();
		let mut results = Vec::new();
		self.recurse(import_path, &mut seen, &mut results)?;

		let mut seen_modules = BTreeSet::new();
		let mut modules = Vec::with_capacity(results.len());
		let mut seen_css_bundles = BTreeSet::new();
		let mut css_bundles = Vec::with_capacity(results.len());

		for res in &results {
			if let Some(chunk) = self.0.get(res) {
				if seen_modules.insert(chunk.file.clone()) {
					modules.push(chunk.file.clone());
				}
				for css in &chunk.css {
					if seen_css_bundles.insert(css.clone()) {
						css_bundles.push(css.clone());
					}
				}
			}
		}

		Ok(DepsResult {
			import_path: import_path.to_owned(),
			modules,
			css_bundles,
		})
	}

	fn recurse(
		&self,
		import_path: &str,
		seen: &mut BTreeSet<String>,
		results: &mut Vec<String>,
	) -> Result<(), String> {
		if !seen.insert(import_path.to_owned()) {
			return Ok(());
		}
		let Some(chunk) = self.0.get(import_path) else {
			return Err(format!(
				"Vite manifest import {import_path:?} is referenced but missing"
			));
		};
		results.push(import_path.to_owned());

		for imp in &chunk.imports {
			self.recurse(imp, seen, results)?;
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

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn vite_manifest_find_all_deps_preserves_manifest_order() {
		let manifest = ViteManifest::from(BTreeMap::from([
			(
				"entry.ts".to_owned(),
				ViteManifestChunk {
					file: "entry.js".to_owned(),
					css: vec!["entry.css".to_owned(), "shared.css".to_owned()],
					imports: vec!["dep_a.ts".to_owned(), "dep_b.ts".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"dep_a.ts".to_owned(),
				ViteManifestChunk {
					file: "dep-a.js".to_owned(),
					css: vec!["dep-a.css".to_owned(), "shared.css".to_owned()],
					imports: vec!["dep_c.ts".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"dep_b.ts".to_owned(),
				ViteManifestChunk {
					file: "dep-b.js".to_owned(),
					css: vec!["dep-b.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
			(
				"dep_c.ts".to_owned(),
				ViteManifestChunk {
					file: "dep-c.js".to_owned(),
					css: vec!["dep-c.css".to_owned()],
					..ViteManifestChunk::default()
				},
			),
		]));

		let got = manifest.find_all_deps("entry.ts").unwrap();

		assert_eq!(
			got.modules,
			vec!["entry.js", "dep-a.js", "dep-c.js", "dep-b.js"]
		);
		assert_eq!(
			got.css_bundles,
			vec![
				"entry.css",
				"shared.css",
				"dep-a.css",
				"dep-c.css",
				"dep-b.css"
			]
		);
	}

	#[test]
	fn vite_manifest_find_all_deps_errors_on_missing_import() {
		let manifest = ViteManifest::from(BTreeMap::from([(
			"entry.ts".to_owned(),
			ViteManifestChunk {
				file: "entry.js".to_owned(),
				imports: vec!["missing.ts".to_owned()],
				..ViteManifestChunk::default()
			},
		)]));

		let error = manifest.find_all_deps("entry.ts").unwrap_err();

		assert_eq!(
			error,
			"Vite manifest import \"missing.ts\" is referenced but missing"
		);
	}
}
