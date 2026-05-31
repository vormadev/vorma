use std::path::PathBuf;

use vorma::{FrontendConfig, TsGenConfig};

pub(crate) fn root_dir() -> PathBuf {
	std::env::current_dir().unwrap()
}

pub(crate) fn path_config() -> vorma::PathConfig {
	vorma::PathConfig {
		public_static_base: "/static/".to_owned(),
		api_base: "/api/".to_owned(),
	}
}

pub(crate) fn frontend_config() -> FrontendConfig {
	FrontendConfig {
		ui_variant: "react".to_owned(),
		js_package_manager_base_cmd: "pnpm exec".to_owned(),
		js_package_manager_dir: ".".to_owned(),
		entry_file: "src/entry.tsx".to_owned(),
		public_static_src_dir: "public".to_owned(),
		critical_css_file: String::new(),
		..FrontendConfig::default()
	}
}

pub(crate) fn ts_gen_config() -> TsGenConfig {
	TsGenConfig {
		out_file: "src/vorma.gen.ts".to_owned(),
		..TsGenConfig::default()
	}
}
