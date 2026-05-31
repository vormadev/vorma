use std::collections::BTreeSet;
use std::sync::Arc;

use vorma::__private::Config;

use crate::build_cancel::BuildCancel;
use crate::config::{VormaCfg, to_cfg};
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

pub(crate) fn prepare_static_build(
	input: &StaticBuildInput<'_>,
) -> Result<PreparedStaticBuild, String> {
	let cfg = to_cfg(input.config).map_err(|err| format!("error converting config: {err}"))?;
	input.build_cancel.check("before preparing static build")?;

	let pub_files = cfg
		.collect_physical_pub_files()
		.map_err(|err| format!("error collecting public static files: {err}"))?;
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
) -> Result<(StaticMetadata, StaticEffects), String> {
	let cfg = to_cfg(config).map_err(|err| format!("error converting config: {err}"))?;
	build_cancel.check("before publishing static build outputs")?;

	staticproc::reconcile(cfg.pub_out(), &prepared.pub_files).map_err(|err| err.to_string())?;
	crate::ts_gen::write_ts_gen_out_file(
		&cfg,
		&TsGenWriteInput {
			live_ts_result: live.generated_ts.clone(),
			static_ts_result: prepared.metadata.generated_ts.clone(),
		},
	)?;

	build_cancel.check("after publishing static build outputs")?;
	Ok((prepared.metadata, prepared.effects))
}

fn bundle_critical_css(
	cfg: &VormaCfg<'_>,
	public_filemap: &std::collections::BTreeMap<String, String>,
) -> Result<cssbundle::BundleOutput, String> {
	let Some(entry) = cfg.critical_css_entry() else {
		return Ok(cssbundle::BundleOutput::default());
	};

	cssbundle::bundle(cssbundle::BundleArgs {
		entry_path: entry.into(),
		public_url_map: public_filemap,
	})
	.map_err(|err| format!("error bundling critical CSS: {err}"))
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
		let (metadata, effects) =
			publish_static_outputs(&config, &live, prepared, &cancel).unwrap();

		assert!(metadata.public_filemap.contains_key("logo.svg"));
		assert!(effects.public_filemap_changed);
		assert!(PathBuf::from(cfg.pub_out()).exists());
		assert!(root.join("src/client/vorma.gen.ts").exists());
		assert!(!PathBuf::from(cfg.manifest_json_out(true)).exists());

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
