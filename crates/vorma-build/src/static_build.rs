use std::collections::BTreeSet;
use std::fmt;
use std::io;
use std::sync::Arc;

use vorma::__private::Config;

use crate::build_cancel::BuildCancel;
use crate::config::{ConfigError, VormaCfg, to_cfg};
use crate::cssbundle;
use crate::generation::{LiveMetadata, StaticEffects, StaticMetadata};
use crate::staticproc;
use crate::ts_gen::{StaticTsResult, TsGenWriteInput, render_ts_public_url_setup_section};

#[derive(Clone, Debug)]
pub(crate) struct StaticBuildInput<'a> {
	pub(crate) config: &'a Config,
	pub(crate) previous_static: Option<&'a StaticMetadata>,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) includes_client_revalidate: bool,
}

#[derive(Clone, Debug)]
pub(crate) struct PreparedStaticBuild {
	pub(crate) pub_files: staticproc::Files,
	pub(crate) metadata: StaticMetadata,
	pub(crate) effects: StaticEffects,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct PublishedStaticOutputs {
	pub(crate) metadata: StaticMetadata,
	pub(crate) effects: StaticEffects,
}

#[derive(Clone, Debug, PartialEq)]
pub(crate) enum StaticPublishOutcome {
	Published(PublishedStaticOutputs),
	CancelledBeforePublish,
	CancelledAfterPublish(PublishedStaticOutputs),
}

#[derive(Debug)]
pub(crate) enum StaticBuildError {
	Config {
		phase: &'static str,
		source: ConfigError,
	},
	Cancelled {
		source: String,
	},
	PublicStaticFiles {
		source: String,
	},
	CriticalCss {
		source: cssbundle::Error,
	},
	PublicOutputReconcile {
		source: io::Error,
	},
	TypeScriptWrite {
		source: String,
	},
}

impl fmt::Display for StaticBuildError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Config { phase, source } => {
				write!(f, "error converting config for {phase}: {source}")
			}
			Self::Cancelled { source } => f.write_str(source),
			Self::PublicStaticFiles { source } => {
				write!(f, "error collecting public static files: {source}")
			}
			Self::CriticalCss { source } => write!(f, "error bundling critical CSS: {source}"),
			Self::PublicOutputReconcile { source } => {
				write!(f, "error reconciling public static output: {source}")
			}
			Self::TypeScriptWrite { source } => {
				write!(f, "error writing generated TypeScript: {source}")
			}
		}
	}
}

impl std::error::Error for StaticBuildError {}

pub(crate) fn prepare_static_build(
	input: &StaticBuildInput<'_>,
) -> Result<PreparedStaticBuild, StaticBuildError> {
	let cfg = to_cfg(input.config).map_err(|source| StaticBuildError::Config {
		phase: "static build preparation",
		source,
	})?;
	input
		.build_cancel
		.check("before preparing static build")
		.map_err(|source| StaticBuildError::Cancelled { source })?;

	let pub_files = cfg
		.collect_physical_pub_files()
		.map_err(|source| StaticBuildError::PublicStaticFiles { source })?;
	let public_filemap = cfg.to_pub_fm(&pub_files);
	let critical_css_result = bundle_critical_css(&cfg, &public_filemap)?;
	let css_files_to_watch = css_files_to_watch(&cfg, &critical_css_result);
	let critical_css = critical_css_result.css;
	let generated_ts = StaticTsResult {
		public_url_setup_section: render_ts_public_url_setup_section(&public_filemap),
	};

	let previous_public_filemap = input
		.previous_static
		.map(|metadata| &metadata.public_filemap);
	let previous_critical_css = input
		.previous_static
		.map(|metadata| metadata.critical_css.as_str());

	let effects = StaticEffects {
		public_filemap_changed: previous_public_filemap != Some(&public_filemap),
		critical_css_changed: previous_critical_css != Some(critical_css.as_str()),
		includes_client_revalidate: input.includes_client_revalidate,
	};

	Ok(PreparedStaticBuild {
		pub_files,
		metadata: StaticMetadata {
			generated_ts,
			public_filemap,
			critical_css,
			css_files_to_watch,
		},
		effects,
	})
}

pub(crate) fn publish_static_outputs(
	config: &Config,
	live: &LiveMetadata,
	prepared: PreparedStaticBuild,
	build_cancel: &BuildCancel,
) -> Result<StaticPublishOutcome, StaticBuildError> {
	publish_static_outputs_with_after_write(config, live, prepared, build_cancel, || {})
}

fn publish_static_outputs_with_after_write(
	config: &Config,
	live: &LiveMetadata,
	prepared: PreparedStaticBuild,
	build_cancel: &BuildCancel,
	after_outputs_written: impl FnOnce(),
) -> Result<StaticPublishOutcome, StaticBuildError> {
	let cfg = to_cfg(config).map_err(|source| StaticBuildError::Config {
		phase: "static output publication",
		source,
	})?;
	if build_cancel.is_cancelled() {
		return Ok(StaticPublishOutcome::CancelledBeforePublish);
	}

	staticproc::reconcile(cfg.pub_out(), &prepared.pub_files)
		.map_err(|source| StaticBuildError::PublicOutputReconcile { source })?;
	crate::ts_gen::write_ts_gen_out_file(
		&cfg,
		&TsGenWriteInput {
			live_ts_result: live.generated_ts.clone(),
			static_ts_result: prepared.metadata.generated_ts.clone(),
		},
	)
	.map_err(|source| StaticBuildError::TypeScriptWrite { source })?;

	after_outputs_written();

	let published = PublishedStaticOutputs {
		metadata: prepared.metadata,
		effects: prepared.effects,
	};
	if build_cancel.is_cancelled() {
		return Ok(StaticPublishOutcome::CancelledAfterPublish(published));
	}
	Ok(StaticPublishOutcome::Published(published))
}

fn bundle_critical_css(
	cfg: &VormaCfg<'_>,
	public_filemap: &std::collections::BTreeMap<String, String>,
) -> Result<cssbundle::BundleOutput, StaticBuildError> {
	let Some(entry) = cfg.critical_css_entry() else {
		return Ok(cssbundle::BundleOutput::default());
	};

	cssbundle::bundle(cssbundle::BundleArgs {
		entry_path: entry.into(),
		source_root_dir: cfg.root_dir().into(),
		public_url_map: public_filemap,
	})
	.map_err(|source| StaticBuildError::CriticalCss { source })
}

fn css_files_to_watch(
	cfg: &VormaCfg<'_>,
	critical_css_result: &cssbundle::BundleOutput,
) -> BTreeSet<std::path::PathBuf> {
	let mut css_files_to_watch = BTreeSet::new();
	if let Some(entry) = cfg.critical_css_entry() {
		css_files_to_watch.insert(entry.into());
	}
	css_files_to_watch.extend(critical_css_result.imports.iter().map(Into::into));
	css_files_to_watch
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::path::PathBuf;
	use std::sync::atomic::Ordering;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-static-build-{name}-{nonce}"));
		fs::create_dir_all(root.join("public")).unwrap();
		fs::create_dir_all(root.join("src/client")).unwrap();
		root
	}

	fn config(root: PathBuf) -> Config {
		Config {
			root_dir: root,
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/assets/".to_owned(),
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
	fn static_build_prepares_publishes_and_writes_ts_once() {
		let root = temp_root("prepare-publish");
		fs::write(root.join("public/logo.svg"), "<svg />").unwrap();
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		let live = LiveMetadata::default();
		let cancel = Arc::new(BuildCancel::default());

		let prepared = prepare_static_build(&StaticBuildInput {
			config: &config,
			previous_static: None,
			build_cancel: Arc::clone(&cancel),
			includes_client_revalidate: false,
		})
		.unwrap();
		assert!(!PathBuf::from(cfg.pub_out()).exists());
		assert!(!PathBuf::from(cfg.ts_gen_out_file()).exists());
		assert!(!PathBuf::from(cfg.manifest_json_out(true)).exists());
		let outcome = publish_static_outputs(&config, &live, prepared, &cancel).unwrap();
		let StaticPublishOutcome::Published(published) = outcome else {
			panic!("static outputs should publish");
		};

		assert!(published.metadata.public_filemap.contains_key("logo.svg"));
		assert!(published.effects.public_filemap_changed);
		assert!(PathBuf::from(cfg.pub_out()).exists());
		assert!(root.join("src/client/vorma.gen.ts").exists());
		assert!(!PathBuf::from(cfg.manifest_json_out(true)).exists());

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn static_publish_reports_cancel_before_writing_outputs() {
		let root = temp_root("cancel-before-publish");
		fs::write(root.join("public/logo.svg"), "<svg />").unwrap();
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		let live = LiveMetadata::default();
		let cancel = Arc::new(BuildCancel::default());
		let prepared = prepare_static_build(&StaticBuildInput {
			config: &config,
			previous_static: None,
			build_cancel: Arc::clone(&cancel),
			includes_client_revalidate: false,
		})
		.unwrap();

		cancel.store(true, Ordering::SeqCst);
		let outcome = publish_static_outputs(&config, &live, prepared, &cancel).unwrap();

		assert_eq!(outcome, StaticPublishOutcome::CancelledBeforePublish);
		assert!(!PathBuf::from(cfg.pub_out()).exists());
		assert!(!PathBuf::from(cfg.ts_gen_out_file()).exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn static_publish_reports_cancel_after_writing_outputs() {
		let root = temp_root("cancel-after-publish");
		fs::write(root.join("public/logo.svg"), "<svg />").unwrap();
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();
		let live = LiveMetadata::default();
		let cancel = Arc::new(BuildCancel::default());
		let prepared = prepare_static_build(&StaticBuildInput {
			config: &config,
			previous_static: None,
			build_cancel: Arc::clone(&cancel),
			includes_client_revalidate: false,
		})
		.unwrap();
		let cancel_for_hook = Arc::clone(&cancel);

		let outcome =
			publish_static_outputs_with_after_write(&config, &live, prepared, &cancel, move || {
				cancel_for_hook.store(true, Ordering::SeqCst);
			})
			.unwrap();
		let StaticPublishOutcome::CancelledAfterPublish(published) = outcome else {
			panic!("static outputs should report cancellation after publication");
		};

		assert!(published.metadata.public_filemap.contains_key("logo.svg"));
		assert!(published.effects.public_filemap_changed);
		assert!(PathBuf::from(cfg.pub_out()).exists());
		assert!(PathBuf::from(cfg.ts_gen_out_file()).exists());
		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn absent_critical_css_does_not_watch_dot_sentinel() {
		let root = temp_root("absent-critical-css");
		let config = config(root.clone());
		let cfg = to_cfg(&config).unwrap();

		let watched = css_files_to_watch(&cfg, &cssbundle::BundleOutput::default());

		assert!(watched.is_empty());
		fs::remove_dir_all(root).unwrap();
	}
}
