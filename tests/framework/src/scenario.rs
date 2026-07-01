use std::collections::BTreeMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use axum::body::Body;
use axum::extract::State;
use axum::http::{Method, Request, Response, StatusCode};
use axum::response::IntoResponse;
use axum::routing::any;
use http_body_util::BodyExt;
use serde::{Deserialize, Serialize};
use url::form_urlencoded;

#[allow(dead_code)]
const VIEW_ROOT_PATTERN: &str = "/";
#[allow(dead_code)]
const VIEW_COUNTER_PATTERN: &str = "/counter";
#[allow(dead_code)]
const VIEW_SLOW_PATTERN: &str = "/slow";
#[allow(dead_code)]
const VIEW_ECHO_PATTERN: &str = "/echo";
#[allow(dead_code)]
const VIEW_ITEM_PATTERN: &str = "/items/:id";
#[allow(dead_code)]
const VIEW_CLIENT_PATTERN: &str = "/client/:id";
#[allow(dead_code)]
const VIEW_NESTED_PATTERN: &str = "/nested";
#[allow(dead_code)]
const VIEW_NESTED_DETAIL_PATTERN: &str = "/nested/:id/details";
#[allow(dead_code)]
const VIEW_FAIL_PATTERN: &str = "/fail";

#[allow(dead_code)]
const RESOURCE_ECHO_PATTERN: &str = "/echo";
#[allow(dead_code)]
const RESOURCE_COUNT_PATTERN: &str = "/count";
#[allow(dead_code)]
const RESOURCE_FORM_PATTERN: &str = "/form";
#[allow(dead_code)]
const RESOURCE_FORM_QUERY_PATTERN: &str = "/form-query";
#[allow(dead_code)]
const RESOURCE_SERVER_MARKER_PATTERN: &str = "/server-marker";
const ECHO_RESOURCE_FAIL_MESSAGE: &str = "__bombadil_fail__";
const VARIANT_ENV_KEY: &str = "VORMA_BOMBADIL_VARIANT";
const DEPLOYMENT_ENV_KEY: &str = "VORMA_BOMBADIL_DEPLOYMENT";
const MODE_ENV_KEY: &str = "VORMA_BOMBADIL_MODE";
const PANIC_ENDPOINT_ENV_KEY: &str = "VORMA_BOMBADIL_ENABLE_PANIC_ENDPOINT";
const EXIT_ENDPOINT_ENV_KEY: &str = "VORMA_BOMBADIL_ENABLE_EXIT_ENDPOINT";
const VARIANT_REACT: &str = vorma::UiVariant::React.as_str();
const VARIANT_PREACT: &str = vorma::UiVariant::Preact.as_str();
const VARIANT_SOLID: &str = vorma::UiVariant::Solid.as_str();
const DEPLOYMENT_A: &str = "A";
const DEPLOYMENT_B: &str = "B";
const MODE_DEV: &str = "dev";
const MODE_PROD: &str = "prod";
const SWITCH_PATH: &str = "/__bombadil/switch";
const DEPLOYMENT_PATH: &str = "/__bombadil/deployment";
const PANIC_PATH: &str = "/__bombadil/panic";
const EXIT_PATH: &str = "/__bombadil/exit";
const SERVER_ENTRY_BIN: &str = "framework-serve";
const RENDER_ENTRY: &str = "vorma.entry.ts";
#[allow(dead_code)]
const VIEW_MODULE_ROOT: &str = "components/routes";
const REQUEST_BODY_LIMIT: usize = 16 * 1024 * 1024;

vorma::app!(pub mod app for crate::scenario::AppState);

#[derive(Clone, Copy)]
pub struct Variant {
	pub ui_variant: vorma::UiVariant,
	pub dist_dir: &'static str,
	pub ts_gen_out_file: &'static str,
	pub dev_ts_gen_out_file: &'static str,
	pub vite_config_file: &'static str,
}

#[derive(Clone, Copy)]
struct DeploymentVariant {
	name: &'static str,
	dist_suffix: &'static str,
	data_suffix: &'static str,
}

pub struct AppState {
	deployment: DeploymentVariant,
}

#[derive(Clone)]
struct Deployment {
	handler: vorma::RuntimeHost<AppState>,
}

#[derive(Clone)]
struct Switchboard {
	inner: Arc<RwLock<SwitchboardInner>>,
}

struct SwitchboardInner {
	deployments: BTreeMap<String, Deployment>,
	current: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct RootData {
	Name: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct CounterInput {
	#[serde(default, rename = "n")]
	N: i32,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct CounterData {
	Value: i32,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct SlowInput {
	#[serde(default, rename = "delay_ms")]
	DelayMS: i32,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct SlowData {
	DelayMS: i32,
	Stamp: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct EchoData {
	Message: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ItemData {
	ID: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ClientData {
	ID: String,
	ServerStamp: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct NestedData {
	Section: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct NestedDetailData {
	ID: String,
	Section: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct CountResourceInput {
	#[serde(default, rename = "delta")]
	Delta: i32,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct CountResourceData {
	Next: i32,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct EchoResourceInput {
	#[serde(default)]
	Message: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct EchoResourceData {
	Message: String,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct FormResourceData {
	Accepted: bool,
	Title: Option<String>,
	Tags: Vec<String>,
	Deployment: String,
}

#[allow(non_snake_case)]
#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ServerMarkerData {
	Marker: String,
	Deployment: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct EmptyData {}

const DEPLOYMENT_A_VARIANT: DeploymentVariant = DeploymentVariant {
	name: DEPLOYMENT_A,
	dist_suffix: "a",
	data_suffix: "from-A",
};

const DEPLOYMENT_B_VARIANT: DeploymentVariant = DeploymentVariant {
	name: DEPLOYMENT_B,
	dist_suffix: "b",
	data_suffix: "from-B",
};

pub const REACT: Variant = Variant {
	ui_variant: vorma::UiVariant::React,
	dist_dir: ".dist.react",
	ts_gen_out_file: "vorma.react.gen.ts",
	dev_ts_gen_out_file: "vorma.react.dev.gen.ts",
	vite_config_file: "vite.react.config.ts",
};

pub const PREACT: Variant = Variant {
	ui_variant: vorma::UiVariant::Preact,
	dist_dir: ".dist.preact",
	ts_gen_out_file: "vorma.preact.gen.ts",
	dev_ts_gen_out_file: "vorma.preact.dev.gen.ts",
	vite_config_file: "vite.preact.config.ts",
};

pub const SOLID: Variant = Variant {
	ui_variant: vorma::UiVariant::Solid,
	dist_dir: ".dist.solid",
	ts_gen_out_file: "vorma.solid.gen.ts",
	dev_ts_gen_out_file: "vorma.solid.dev.gen.ts",
	vite_config_file: "vite.solid.config.ts",
};

pub const ROOT: app::View = app::view! {
	client_file: "components/routes/root.ts";
	pattern: "/";
	input: ();
	output: RootData;
	handler: |ctx| {
		ctx.head().title("Vorma Framework Test App");
		Ok(RootData {
			Name: "root".to_owned(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const COUNTER: app::View = app::view! {
	client_file: "components/routes/counter.ts";
	pattern: "/counter";
	input: CounterInput;
	output: CounterData;
	handler: |ctx| {
		let value = ctx.input().N.clamp(-5, 5);
		ctx.head().title(format!("Counter {value}"));
		Ok(CounterData {
			Value: value,
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const SLOW: app::View = app::view! {
	client_file: "components/routes/slow.ts";
	pattern: "/slow";
	input: SlowInput;
	output: SlowData;
	handler: |ctx| {
		let delay_ms = ctx.input().DelayMS.clamp(0, 250);
		std::thread::sleep(Duration::from_millis(delay_ms as u64));
		ctx.head().title("Slow View");
		Ok(SlowData {
			DelayMS: delay_ms,
			Stamp: unix_nanos(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const ECHO: app::View = app::view! {
	client_file: "components/routes/echo.ts";
	pattern: "/echo";
	input: ();
	output: EchoData;
	handler: |ctx| {
		ctx.head().title("Echo");
		Ok(EchoData {
			Message: "ready".to_owned(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const ITEM: app::View = app::view! {
	client_file: "components/routes/item.ts";
	pattern: "/items/:id";
	input: ();
	output: ItemData;
	handler: |ctx| {
		let id = ctx.param("id").to_owned();
		ctx.head().title(format!("Item {id}"));
		Ok(ItemData {
			ID: id,
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const CLIENT: app::View = app::view! {
	client_file: "components/routes/client.ts";
	pattern: "/client/:id";
	input: ();
	output: ClientData;
	handler: |ctx| {
		let id = ctx.param("id").to_owned();
		ctx.head().title(format!("Client {id}"));
		Ok(ClientData {
			ID: id,
			ServerStamp: unix_nanos(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const NESTED: app::View = app::view! {
	client_file: "components/routes/nested.ts";
	pattern: "/nested";
	input: ();
	output: NestedData;
	handler: |ctx| {
		ctx.head().title("Nested");
		Ok(NestedData {
			Section: "nested".to_owned(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const NESTED_DETAIL: app::View = app::view! {
	client_file: "components/routes/nested_detail.ts";
	pattern: "/nested/:id/details";
	input: ();
	output: NestedDetailData;
	handler: |ctx| {
		let id = ctx.param("id").to_owned();
		ctx.head().title(format!("Nested {id}"));
		Ok(NestedDetailData {
			ID: id,
			Section: "nested".to_owned(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const FAIL: app::View = app::view! {
	client_file: "components/routes/fail.ts";
	pattern: "/fail";
	input: ();
	output: EmptyData;
	handler: |_ctx| {
		/*
		Views have no status vocabulary: the segment errors, the page still
		renders, and the boundary shows the explicit client message.
		*/
		Err::<EmptyData, _>(
			vorma::ViewExit::err("fixture view handler failed (server-side record)")
				.with_client_msg("Fixture view handler failed on purpose."),
		)
	};
};

pub const COUNT_RESOURCE: app::Resource = app::resource! {
	method: vorma::HttpMethod::GET;
	pattern: "/api/count";
	input: CountResourceInput;
	output: CountResourceData;
	handler: |ctx| {
		let next = ctx.input().Delta.clamp(-5, 5);
		Ok(CountResourceData {
			Next: next,
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const ECHO_RESOURCE: app::Resource = app::resource! {
	method: vorma::HttpMethod::POST;
	pattern: "/api/echo";
	input: EchoResourceInput;
	output: EchoResourceData;
	handler: |ctx| {
		if ctx.input().Message == ECHO_RESOURCE_FAIL_MESSAGE {
			/*
			A rejection is one returned value — no fabricated output data.
			*/
			return Err(
				vorma::HttpExit::err("fixture echo rejected on purpose (server record)")
					.with_status(vorma::HttpStatusCode::CONFLICT)
					.with_client_msg("Fixture resource failed on purpose."),
			);
		}
		Ok(EchoResourceData {
			Message: ctx.input().Message.clone(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

pub const FORM_RESOURCE: app::Resource = app::resource! {
	method: vorma::HttpMethod::POST;
	pattern: "/api/form";
	input: vorma::FormData;
	output: FormResourceData;
	handler: |ctx| {
		Ok(form_resource(&ctx))
	};
};

pub const FORM_QUERY_RESOURCE: app::Resource = app::resource! {
	method: vorma::HttpMethod::GET;
	pattern: "/api/form-query";
	input: vorma::FormData;
	output: FormResourceData;
	handler: |ctx| {
		Ok(form_resource(&ctx))
	};
};

pub const SERVER_MARKER_RESOURCE: app::Resource = app::resource! {
	method: vorma::HttpMethod::GET;
	pattern: "/api/server-marker";
	input: ();
	output: ServerMarkerData;
	handler: |ctx| {
		Ok(ServerMarkerData {
			Marker: crate::dev_marker::DEV_SERVER_MARKER.to_owned(),
			Deployment: ctx.state().deployment.data_suffix.to_owned(),
		})
	};
};

impl Variant {
	pub fn build(self) -> Result<(), String> {
		let deployment = selected_deployment();
		vorma_build::run(move || self.config(deployment))
	}

	pub async fn serve_from_disk(self) -> vorma::Result<()> {
		let board = self.switchboard([DEPLOYMENT_A_VARIANT, DEPLOYMENT_B_VARIANT])?;
		self.serve_board(board).await
	}

	pub async fn serve_selected_from_disk(self) -> vorma::Result<()> {
		let deployment = selected_deployment();
		let board = self.switchboard([deployment])?;
		self.serve_board(board).await
	}

	fn switchboard(
		self,
		deployments: impl IntoIterator<Item = DeploymentVariant>,
	) -> vorma::Result<Switchboard> {
		let mut handlers = BTreeMap::new();
		let mut first = None;
		for deployment in deployments {
			let config = self.config(deployment)?;
			let handler = vorma::App::from_config(config)?;
			first.get_or_insert(deployment.name);
			handlers.insert(deployment.name.to_owned(), Deployment { handler });
		}
		Ok(Switchboard {
			inner: Arc::new(RwLock::new(SwitchboardInner {
				deployments: handlers,
				current: first.unwrap_or(DEPLOYMENT_A).to_owned(),
			})),
		})
	}

	async fn serve_board(self, board: Switchboard) -> vorma::Result<()> {
		let addr = vorma::bind_addr()?;
		let app = axum::Router::new()
			.fallback(any(switchboard_handler))
			.with_state(board);

		println!(
			"Starting {} framework test server at http://localhost:{}",
			self.ui_variant.as_str(),
			addr.port(),
		);
		axum::serve(
			tokio::net::TcpListener::bind(addr).await.map_err(|error| {
				vorma::Error::new(format!("bind framework test server: {error}"))
			})?,
			app,
		)
		.await
		.map_err(|error| vorma::Error::new(format!("Application server failed: {error}")))
	}

	fn config(self, deployment: DeploymentVariant) -> vorma::Result<vorma::AppConfig<AppState>> {
		let ts_gen_out_file = if selected_mode() == MODE_DEV {
			self.dev_ts_gen_out_file
		} else {
			self.ts_gen_out_file
		};

		Ok(vorma::AppConfig {
			root_dir: env!("CARGO_MANIFEST_DIR").into(),
			server_target: vorma::ServerTarget {
				cargo_package: env!("CARGO_PKG_NAME").to_owned(),
				cargo_bin: SERVER_ENTRY_BIN.to_owned(),
			},
			dist_dir: self.deployment_dist_dir(deployment),
			public_static_base: "/".to_owned(),
			frontend_config: vorma::FrontendConfig {
				ui_variant: self.ui_variant,
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: self.vite_config_file.to_owned(),
				entry_file: RENDER_ENTRY.to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "shared/styles/main.critical.css".to_owned(),
			},
			ts_gen_config: vorma::TsGenConfig {
				out_file: ts_gen_out_file.to_owned(),
				..vorma::TsGenConfig::default()
			},
			dev_watch_config: vorma::DevWatchConfig {
				watch_patterns: vec![".".to_owned(), "!.bombadil/**".to_owned()],
				on_change_recompile_server: vec!["src/**/*.rs".to_owned()],
				on_change_client_revalidate: Vec::new(),
			},
			state: AppState { deployment },
			views: views(),
			resources: resources(),
			middlewares: middlewares(),
			tasks_options: vorma::tasks::TasksOptions::default(),
			document: document(),
			request_body_limit: REQUEST_BODY_LIMIT,
		})
	}

	fn deployment_dist_dir(self, deployment: DeploymentVariant) -> String {
		if selected_mode() == MODE_DEV {
			return format!("{}.dev.{}", self.dist_dir, deployment.dist_suffix);
		}
		format!("{}.{}", self.dist_dir, deployment.dist_suffix)
	}
}

impl Switchboard {
	async fn serve(&self, request: Request<Body>) -> Response<Body> {
		let path = request.uri().path().to_owned();
		if path.starts_with("/__bombadil/") {
			return self.serve_control(request);
		}

		let handler = {
			let inner = self.inner.read().expect("switchboard lock poisoned");
			inner
				.deployments
				.get(&inner.current)
				.map(|deployment| deployment.handler.clone())
		};
		let Some(handler) = handler else {
			return empty_response(StatusCode::NOT_FOUND);
		};
		let (parts, body) = request.into_parts();
		let body = match body.collect().await {
			Ok(body) => body.to_bytes(),
			Err(error) => {
				return text_response(
					StatusCode::INTERNAL_SERVER_ERROR,
					format!("error reading request body: {error}"),
				);
			}
		};
		let request = Request::from_parts(parts, body);
		match handler.handle_request(request).await {
			Ok(response) => response.map(Body::from),
			Err(error) => text_response(StatusCode::INTERNAL_SERVER_ERROR, error),
		}
	}

	fn serve_control(&self, request: Request<Body>) -> Response<Body> {
		match request.uri().path() {
			PANIC_PATH => {
				if env_bool(PANIC_ENDPOINT_ENV_KEY) {
					panic!("framework test panic endpoint");
				}
				empty_response(StatusCode::NOT_FOUND)
			}
			EXIT_PATH => {
				if env_bool(EXIT_ENDPOINT_ENV_KEY) {
					std::thread::spawn(|| {
						std::thread::sleep(Duration::from_millis(50));
						std::process::exit(86);
					});
					return empty_response(StatusCode::NO_CONTENT);
				}
				empty_response(StatusCode::NOT_FOUND)
			}
			DEPLOYMENT_PATH => self.deployment_response(),
			SWITCH_PATH => {
				if request.method() != Method::POST {
					return empty_response(StatusCode::METHOD_NOT_ALLOWED);
				}
				self.switch_to(query_param(request.uri().query(), "to").as_deref());
				self.deployment_response()
			}
			_ => empty_response(StatusCode::NOT_FOUND),
		}
	}

	fn switch_to(&self, to: Option<&str>) {
		let mut inner = self.inner.write().expect("switchboard lock poisoned");
		if let Some(to) = to
			&& inner.deployments.contains_key(to)
		{
			inner.current = to.to_owned();
			return;
		}
		if inner.deployments.len() <= 1 {
			return;
		}
		if inner.current == DEPLOYMENT_A {
			inner.current = DEPLOYMENT_B.to_owned();
			return;
		}
		inner.current = DEPLOYMENT_A.to_owned();
	}

	fn deployment_response(&self) -> Response<Body> {
		let current = {
			let inner = self.inner.read().expect("switchboard lock poisoned");
			inner.current.clone()
		};
		json_response(serde_json::json!({ "deployment": current }))
	}
}

async fn switchboard_handler(
	State(board): State<Switchboard>,
	request: Request<Body>,
) -> impl IntoResponse {
	board.serve(request).await
}

pub fn selected_variant() -> Variant {
	match std::env::var(VARIANT_ENV_KEY).unwrap_or_default().as_str() {
		VARIANT_REACT => REACT,
		VARIANT_PREACT => PREACT,
		VARIANT_SOLID => SOLID,
		_ => panic!(
			"{VARIANT_ENV_KEY} must be one of: {VARIANT_REACT}, {VARIANT_PREACT}, {VARIANT_SOLID}",
		),
	}
}

fn selected_deployment() -> DeploymentVariant {
	match std::env::var(DEPLOYMENT_ENV_KEY)
		.unwrap_or_default()
		.as_str()
	{
		"" | DEPLOYMENT_A => DEPLOYMENT_A_VARIANT,
		DEPLOYMENT_B => DEPLOYMENT_B_VARIANT,
		_ => panic!("{DEPLOYMENT_ENV_KEY} must be one of: {DEPLOYMENT_A}, {DEPLOYMENT_B}"),
	}
}

fn selected_mode() -> String {
	match std::env::var(MODE_ENV_KEY).unwrap_or_default().as_str() {
		"" | MODE_PROD => MODE_PROD.to_owned(),
		MODE_DEV => MODE_DEV.to_owned(),
		_ => panic!("{MODE_ENV_KEY} must be one of: {MODE_PROD}, {MODE_DEV}"),
	}
}

fn views() -> app::Views {
	app::views![
		ROOT,
		COUNTER,
		SLOW,
		ECHO,
		ITEM,
		CLIENT,
		NESTED,
		NESTED_DETAIL,
		FAIL,
	]
}

fn resources() -> app::Resources {
	app::resources![
		COUNT_RESOURCE,
		ECHO_RESOURCE,
		FORM_RESOURCE,
		FORM_QUERY_RESOURCE,
		SERVER_MARKER_RESOURCE,
	]
}

fn middlewares() -> app::Middlewares {
	app::middlewares![]
}

fn document() -> app::DocumentBuilder {
	app::DocumentBuilder::new(|_| async move {
		let mut document = vorma::Document::new();
		document
			.head()
			.meta_charset("utf-8")
			.meta_name_content("viewport", "width=device-width, initial-scale=1")
			.title("Vorma Framework Test App")
			.description("A small Vorma app for framework runtime testing.");
		Ok(document)
	})
}

fn form_resource(ctx: &app::ResourceCtx<vorma::FormData>) -> FormResourceData {
	FormResourceData {
		Accepted: true,
		Title: ctx.input().text("title").map(ToOwned::to_owned),
		Tags: ctx.input().texts("tag").map(ToOwned::to_owned).collect(),
		Deployment: ctx.state().deployment.data_suffix.to_owned(),
	}
}

fn query_param(query: Option<&str>, key: &str) -> Option<String> {
	form_urlencoded::parse(query.unwrap_or_default().as_bytes())
		.find_map(|(name, value)| (name == key).then(|| value.into_owned()))
}

fn empty_response(status: StatusCode) -> Response<Body> {
	Response::builder()
		.status(status)
		.body(Body::empty())
		.expect("empty response should build")
}

fn text_response(status: StatusCode, text: impl Into<String>) -> Response<Body> {
	Response::builder()
		.status(status)
		.body(Body::from(text.into()))
		.expect("text response should build")
}

fn json_response(value: serde_json::Value) -> Response<Body> {
	Response::builder()
		.status(StatusCode::OK)
		.header("content-type", "application/json")
		.body(Body::from(value.to_string()))
		.expect("JSON response should build")
}

fn env_bool(key: &str) -> bool {
	matches!(
		std::env::var(key).unwrap_or_default().as_str(),
		"1" | "t" | "T" | "TRUE" | "true" | "True"
	)
}

fn unix_nanos() -> String {
	SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.unwrap_or_default()
		.as_nanos()
		.to_string()
}
