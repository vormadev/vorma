use serde::{Deserialize, Serialize};

#[derive(Default)]
struct ContractState {
	prefix: String,
}

vorma::app!(mod contract_app for crate::ContractState);

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
struct ContractViewInput {
	#[serde(default)]
	q: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct ContractViewOutput {
	message: String,
	query: Option<String>,
}

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
struct ContractResourceInput {
	name: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct ContractResourceOutput {
	greeting: String,
}

const CONTRACT_VIEW: contract_app::View = contract_app::view! {
	client_file: "src/client/views/contract.view.tsx";
	pattern: "/contract/:id";
	input: ContractViewInput;
	output: ContractViewOutput;

	handler: |ctx| {
		let _ = ctx.request().uri();
		ctx.head()
			.title("Contract view")
			.description("External app declaration contract");
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-vorma-contract-view"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(ContractViewOutput {
			message: format!("{}{}", ctx.state().prefix, ctx.param("id")),
			query: ctx.input().q.clone(),
		})
	};
};

const CONTRACT_RESOURCE: contract_app::Resource = contract_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/contract/:id";
	input: ContractResourceInput;
	output: ContractResourceOutput;

	handler: |ctx| {
		let _ = ctx.request().uri();
		ctx.response().set_status(vorma::HttpStatusCode::CREATED);
		Ok(ContractResourceOutput {
			greeting: format!(
				"{}{}:{}",
				ctx.state().prefix,
				ctx.param("id"),
				ctx.input().name
			),
		})
	};
};

fn app_config() -> vorma::AppConfig<ContractState> {
	vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		server_target: vorma::ServerTarget {
			cargo_package: "contract-package".to_owned(),
			cargo_bin: "contract-server".to_owned(),
		},
		dist_dir: "target/vorma-contract-test-dist".to_owned(),
		public_static_base: "/assets/".to_owned(),
		frontend_config: vorma::FrontendConfig {
			ui_variant: vorma::UiVariant::React,
			js_package_manager_base_cmd: "pnpm exec".to_owned(),
			js_package_manager_dir: ".".to_owned(),
			vite_config_file: "vite.config.ts".to_owned(),
			entry_file: "src/client/entry.tsx".to_owned(),
			public_static_src_dir: "public".to_owned(),
			critical_css_file: "src/client/styles/critical.css".to_owned(),
		},
		ts_gen_config: vorma::TsGenConfig {
			out_file: "src/client/vorma.gen.ts".to_owned(),
			..vorma::TsGenConfig::default()
		},
		dev_watch_config: vorma::DevWatchConfig::default(),
		state: ContractState {
			prefix: "contract-".to_owned(),
		},
		views: contract_app::views![CONTRACT_VIEW],
		resources: contract_app::resources![CONTRACT_RESOURCE],
		middlewares: contract_app::middlewares![contract_app::Middleware::new(|ctx| async move {
			let _ = ctx.request().uri();
			let _ = ctx.request().path();
			ctx.response().set_header(
				vorma::HttpHeaderName::from_static("x-vorma-contract-middleware"),
				vorma::HttpHeaderValue::from_static("1"),
			);
			Ok(())
		})],
		tasks_options: vorma::TasksOptions::default(),
		document: contract_app::DocumentBuilder::new(|ctx| async move {
			let mut document = vorma::Document::new();
			document.html().lang("en");
			document.body().data("contract", "app-declaration");
			document
				.head()
				.meta_charset("utf-8")
				.title("Contract app")
				.description("External app declaration contract");
			let _ = ctx.request().uri();
			let _ = ctx.request().path();
			Ok(document)
		}),
		request_body_limit: 1024 * 1024,
	}
}

#[test]
fn external_crate_can_declare_complete_app_config() {
	let config = app_config();
	assert_eq!(config.frontend_config.ui_variant, vorma::UiVariant::React);

	let result = contract_app::App::from_config(config);
	assert!(result.is_err());
}
