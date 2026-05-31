use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use serde_json::Value;
use vorma::__private::Config;
use vorma::__private::manifest::Manifest;

use crate::config::ConfigView;
use crate::ts_gen::{LiveTsResult, StaticTsResult};
use crate::ts_modules::TsRoute;

#[derive(Clone, Debug, Default, PartialEq)]
pub(crate) struct LiveMetadata {
	pub(crate) generated_ts: LiveGeneratedTs,
	pub(crate) route_modules: BTreeMap<String, RouteModule>,
	pub(crate) search_schemas: BTreeMap<String, Value>,
	pub(crate) root_document_hash_source: String,
}

pub(crate) type LiveGeneratedTs = LiveTsResult;
pub(crate) type StaticGeneratedTs = StaticTsResult;
pub(crate) type RouteModule = TsRoute;

#[derive(Clone, Debug, Default, PartialEq)]
pub(crate) struct StaticMetadata {
	pub(crate) generated_ts: StaticGeneratedTs,
	pub(crate) public_filemap: BTreeMap<String, String>,
	pub(crate) critical_css: String,
	pub(crate) css_files_to_watch: BTreeSet<PathBuf>,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub(crate) struct StaticEffects {
	pub(crate) public_filemap_changed: bool,
	pub(crate) critical_css_changed: bool,
	pub(crate) includes_client_revalidate: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct BuildArtifacts {
	pub(crate) build_entry_executable: PathBuf,
	pub(crate) mode: BuildArtifactMode,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum BuildArtifactMode {
	Dev { app_server_executable: PathBuf },
	Prod,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct GenerationCandidate {
	pub(crate) config: Config,
	pub(crate) live: LiveMetadata,
	pub(crate) static_metadata: StaticMetadata,
	pub(crate) manifest: Option<Manifest>,
	pub(crate) artifacts: BuildArtifacts,
	pub(crate) static_effects: StaticEffects,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct CommittedGeneration {
	config: Config,
	live: LiveMetadata,
	static_metadata: StaticMetadata,
	manifest: Option<Manifest>,
	artifacts: BuildArtifacts,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct DevMuxGeneration {
	pub(crate) config: Config,
	pub(crate) route_modules: BTreeMap<String, RouteModule>,
	pub(crate) public_filemap: BTreeMap<String, String>,
}

impl GenerationCandidate {
	pub(crate) fn commit(self) -> CommittedGeneration {
		CommittedGeneration {
			config: self.config,
			live: self.live,
			static_metadata: self.static_metadata,
			manifest: self.manifest,
			artifacts: self.artifacts,
		}
	}
}

impl CommittedGeneration {
	pub(crate) fn config(&self) -> &Config {
		&self.config
	}

	pub(crate) fn config_view(&self) -> Result<ConfigView<'_>, String> {
		ConfigView::new(&self.config)
	}

	pub(crate) fn live(&self) -> &LiveMetadata {
		&self.live
	}

	pub(crate) fn static_metadata(&self) -> &StaticMetadata {
		&self.static_metadata
	}

	pub(crate) fn replace_static_metadata(&mut self, static_metadata: StaticMetadata) {
		self.static_metadata = static_metadata;
		self.manifest = None;
	}

	pub(crate) fn set_manifest(&mut self, manifest: Manifest) {
		self.manifest = Some(manifest);
	}

	pub(crate) fn app_server_executable(&self) -> Option<&PathBuf> {
		match &self.artifacts.mode {
			BuildArtifactMode::Dev {
				app_server_executable,
			} => Some(app_server_executable),
			BuildArtifactMode::Prod => None,
		}
	}

	pub(crate) fn dev_mux_generation(&self) -> DevMuxGeneration {
		DevMuxGeneration {
			config: self.config.clone(),
			route_modules: self.live.route_modules.clone(),
			public_filemap: self.static_metadata.public_filemap.clone(),
		}
	}
}

#[cfg(test)]
mod tests {
	use std::path::PathBuf;

	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;

	fn config() -> Config {
		Config {
			root_dir: std::env::current_dir().unwrap(),
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

	#[test]
	fn committing_candidate_preserves_only_generation_data() {
		let candidate = GenerationCandidate {
			config: config(),
			live: LiveMetadata {
				route_modules: BTreeMap::from([(
					"/".to_owned(),
					RouteModule {
						pattern: "/".to_owned(),
						import_path: "src/root.tsx".to_owned(),
						deps: Vec::new(),
					},
				)]),
				..LiveMetadata::default()
			},
			static_metadata: StaticMetadata {
				public_filemap: BTreeMap::from([(
					"logo.svg".to_owned(),
					"/static/vorma_out_logo_hash.svg".to_owned(),
				)]),
				..StaticMetadata::default()
			},
			manifest: None,
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/build-entry"),
				mode: BuildArtifactMode::Dev {
					app_server_executable: PathBuf::from("/tmp/app-server"),
				},
			},
			static_effects: StaticEffects {
				public_filemap_changed: true,
				critical_css_changed: false,
				includes_client_revalidate: false,
			},
		};

		let committed = candidate.commit();
		let dev_mux = committed.dev_mux_generation();

		assert_eq!(
			committed.app_server_executable(),
			Some(&PathBuf::from("/tmp/app-server"))
		);
		assert_eq!(dev_mux.route_modules["/"].import_path, "src/root.tsx");
		assert_eq!(
			dev_mux.public_filemap["logo.svg"],
			"/static/vorma_out_logo_hash.svg"
		);
	}

	#[test]
	fn replacing_static_metadata_invalidates_committed_manifest() {
		let mut committed = GenerationCandidate {
			config: config(),
			live: LiveMetadata::default(),
			static_metadata: StaticMetadata::default(),
			manifest: Some(Manifest {
				vorma_version: "old".to_owned(),
				..Manifest::default()
			}),
			artifacts: BuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/build-entry"),
				mode: BuildArtifactMode::Dev {
					app_server_executable: PathBuf::from("/tmp/app-server"),
				},
			},
			static_effects: StaticEffects::default(),
		}
		.commit();

		committed.replace_static_metadata(StaticMetadata {
			public_filemap: BTreeMap::from([(
				"new.css".to_owned(),
				"/static/vorma_out_new.css".to_owned(),
			)]),
			..StaticMetadata::default()
		});

		assert!(committed.manifest.is_none());
		assert_eq!(
			committed.static_metadata.public_filemap["new.css"],
			"/static/vorma_out_new.css"
		);
	}
}
