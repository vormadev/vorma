//! Owned-request-to-response engine benchmarks.
//!
//! Measures the committed runtime pipeline from an already-collected
//! `http::Request<Bytes>` to its finished `http::Response<Bytes>` — the
//! same call `vorma::testing::TestApp` and `CommittedRuntimeService`
//! expose to real adapters, with no socket and no HTTP transport layer
//! (body collection/limits and tower wiring are adapter-facing concerns
//! and stay out of this fixture). Rows rotate a small set of realistic
//! paths per case, mirroring the matcher benchmark suite's shape.

use std::hint::black_box;

use bytes::Bytes;
use http::{Method, Request, Response};
use serde::{Deserialize, Serialize};

// This crate links vorma, whose default `mimalloc` feature already
// declares the global allocator app binaries run with; a bench binary
// of a vorma-linking crate must not declare a second one.

use vorma::testing::TestApp;

vorma::app!(mod engine_bench_app for ());

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
struct EmptyInput {}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct StaticViewOutput {
	message: String,
}

const STATIC_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/static.view.tsx";
	pattern: "/dashboard";
	input: EmptyInput;
	output: StaticViewOutput;

	handler: |ctx| {
		ctx.head().title("Dashboard");
		Ok(StaticViewOutput {
			message: "welcome".to_owned(),
		})
	};
};

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct ShelfOutput {
	shelf_id: String,
}

const SHELF_ROOT_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/shelf_root.view.tsx";
	pattern: "/";
	input: EmptyInput;
	output: ShelfOutput;

	handler: |ctx| {
		ctx.head().title("Shelves");
		Ok(ShelfOutput {
			shelf_id: String::new(),
		})
	};
};

const SHELF_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/shelf.view.tsx";
	pattern: "/shelves/:shelf_id";
	input: EmptyInput;
	output: ShelfOutput;

	handler: |ctx| {
		Ok(ShelfOutput {
			shelf_id: ctx.param("shelf_id").to_owned(),
		})
	};
};

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct BoardOutput {
	shelf_id: String,
	board_id: String,
}

const BOARD_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/board.view.tsx";
	pattern: "/shelves/:shelf_id/boards/:board_id";
	input: EmptyInput;
	output: BoardOutput;

	handler: |ctx| {
		Ok(BoardOutput {
			shelf_id: ctx.param("shelf_id").to_owned(),
			board_id: ctx.param("board_id").to_owned(),
		})
	};
};

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct CardOutput {
	shelf_id: String,
	board_id: String,
	card_id: String,
}

const CARD_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/card.view.tsx";
	pattern: "/shelves/:shelf_id/boards/:board_id/cards/:card_id";
	input: EmptyInput;
	output: CardOutput;

	handler: |ctx| {
		Ok(CardOutput {
			shelf_id: ctx.param("shelf_id").to_owned(),
			board_id: ctx.param("board_id").to_owned(),
			card_id: ctx.param("card_id").to_owned(),
		})
	};
};

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
struct NotFoundInput {}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct NotFoundOutput {
	requested_path: String,
}

const CATCH_ALL_VIEW: engine_bench_app::View = engine_bench_app::view! {
	client_file: "src/client/views/not_found.view.tsx";
	pattern: "/*";
	input: NotFoundInput;
	output: NotFoundOutput;

	handler: |ctx| {
		ctx.head().title("Not found");
		Ok(NotFoundOutput {
			requested_path: ctx.request().path().to_owned(),
		})
	};
};

#[derive(Clone, Debug, Deserialize, Serialize, vorma::TsGen)]
struct CardInput {
	title: String,
	position: i64,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct CardResourceOutput {
	card_id: String,
	title: String,
	position: i64,
}

const CARD_RESOURCE: engine_bench_app::Resource = engine_bench_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/shelves/:shelf_id/boards/:board_id/cards/:card_id";
	input: CardInput;
	output: CardResourceOutput;

	handler: |ctx| {
		Ok(CardResourceOutput {
			card_id: ctx.param("card_id").to_owned(),
			title: ctx.input().title.clone(),
			position: ctx.input().position,
		})
	};
};

const ATTACHMENT_RESOURCE: engine_bench_app::Resource = engine_bench_app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/shelves/:shelf_id/attachment";
	input: ();
	output: vorma::ResourceBody;

	handler: |ctx| {
		let _ = ctx.param("shelf_id");
		Ok(vorma::ResourceBody::new(
			vorma::HttpHeaderValue::from_static("application/octet-stream"),
			ATTACHMENT_BYTES.to_vec(),
		))
	};
};

// Representative small binary payload: large enough that copying it is
// not free, small enough to stay a realistic single-resource response.
const ATTACHMENT_BYTES: [u8; 4096] = [0x5au8; 4096];

const MIDDLEWARE_ONE: fn() -> engine_bench_app::Middleware = || {
	engine_bench_app::Middleware::new(|ctx| async move {
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-engine-bench-mw-one"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(())
	})
};

const MIDDLEWARE_TWO: fn() -> engine_bench_app::Middleware = || {
	engine_bench_app::Middleware::new(|ctx| async move {
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-engine-bench-mw-two"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(())
	})
};

fn base_app_config() -> vorma::AppConfig<()> {
	vorma::AppConfig::default()
}

// App with a full route surface plus the root catch-all, so unmatched
// GET/HEAD paths render the branded not-found view (HTTP 200) instead of
// a bare classifier miss.
fn app_with_catch_all() -> TestApp {
	TestApp::from_config(vorma::AppConfig {
		views: engine_bench_app::views![
			STATIC_VIEW,
			SHELF_ROOT_VIEW,
			SHELF_VIEW,
			BOARD_VIEW,
			CARD_VIEW,
			CATCH_ALL_VIEW,
		],
		resources: engine_bench_app::resources![CARD_RESOURCE, ATTACHMENT_RESOURCE],
		middlewares: engine_bench_app::middlewares![MIDDLEWARE_ONE(), MIDDLEWARE_TWO()],
		..base_app_config()
	})
	.expect("engine bench app with catch-all should boot")
}

// Same route surface, minus the catch-all, so an unmatched GET/HEAD path
// is a bare classifier miss (`RequestTarget::NotFound`) rather than a
// rendered fallback view.
fn app_without_catch_all() -> TestApp {
	TestApp::from_config(vorma::AppConfig {
		views: engine_bench_app::views![
			STATIC_VIEW,
			SHELF_ROOT_VIEW,
			SHELF_VIEW,
			BOARD_VIEW,
			CARD_VIEW
		],
		resources: engine_bench_app::resources![CARD_RESOURCE, ATTACHMENT_RESOURCE],
		middlewares: engine_bench_app::middlewares![MIDDLEWARE_ONE(), MIDDLEWARE_TWO()],
		..base_app_config()
	})
	.expect("engine bench app without catch-all should boot")
}

// Same route surface and catch-all as `app_with_catch_all`, with zero
// registered middlewares — the control the middleware-chain row diffs
// against, since every other row in this fixture already runs through
// both middlewares (they are unscoped and apply to all requests).
fn app_with_catch_all_no_middleware() -> TestApp {
	TestApp::from_config(vorma::AppConfig {
		views: engine_bench_app::views![
			STATIC_VIEW,
			SHELF_ROOT_VIEW,
			SHELF_VIEW,
			BOARD_VIEW,
			CARD_VIEW,
			CATCH_ALL_VIEW,
		],
		resources: engine_bench_app::resources![CARD_RESOURCE, ATTACHMENT_RESOURCE],
		middlewares: engine_bench_app::Middlewares::new(),
		..base_app_config()
	})
	.expect("engine bench app without middleware should boot")
}

fn get_request(path_and_query: &str) -> Request<Bytes> {
	Request::builder()
		.method(Method::GET)
		.uri(path_and_query)
		.body(Bytes::new())
		.expect("bench request URI should be valid")
}

fn head_request(path_and_query: &str) -> Request<Bytes> {
	Request::builder()
		.method(Method::HEAD)
		.uri(path_and_query)
		.body(Bytes::new())
		.expect("bench request URI should be valid")
}

fn post_json_request(path_and_query: &str, body: &CardInput) -> Request<Bytes> {
	Request::builder()
		.method(Method::POST)
		.uri(path_and_query)
		.header(http::header::CONTENT_TYPE, "application/json")
		.body(Bytes::from(
			serde_json::to_vec(body).expect("bench request body should serialize"),
		))
		.expect("bench request URI should be valid")
}

fn put_request(path_and_query: &str) -> Request<Bytes> {
	Request::builder()
		.method(Method::PUT)
		.uri(path_and_query)
		.body(Bytes::new())
		.expect("bench request URI should be valid")
}

fn runtime() -> tokio::runtime::Runtime {
	tokio::runtime::Builder::new_multi_thread()
		.enable_all()
		.build()
		.expect("benchmark runtime builds")
}

async fn drive(app: &TestApp, request: Request<Bytes>) -> Response<Bytes> {
	black_box(app.handle_request(black_box(request)).await)
}

fn main() {
	let mut bench = vorma_bench::Bench::new(env!("CARGO_PKG_NAME"));
	let rt = runtime();

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = ["/dashboard", "/dashboard?x=1"];
		let mut i = 0usize;
		bench.bench_async("static_view_render", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = [
			"/shelves/1/boards/2/cards/3",
			"/shelves/alpha/boards/beta/cards/gamma",
		];
		let mut i = 0usize;
		bench.bench_async("nested_dynamic_view_chain_4_deep", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let bodies = [
			CardInput {
				title: "Ship the release".to_owned(),
				position: 1,
			},
			CardInput {
				title: "Write the report".to_owned(),
				position: 2,
			},
		];
		let mut i = 0usize;
		bench.bench_async("json_resource_small_input_output", &rt, move || {
			let body = &bodies[i % bodies.len()];
			let request = post_json_request("/api/shelves/1/boards/2/cards/3", body);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = ["/api/shelves/1/attachment", "/api/shelves/2/attachment"];
		let mut i = 0usize;
		bench.bench_async("resource_body_binary_resource", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		// Every row above already runs through the two registered
		// middlewares (they are unscoped and apply to all requests), so
		// this pair isolates that cost directly: identical route and
		// paths, with and without the middleware chain registered.
		let app = app_with_catch_all();
		let app = &app;
		let paths = ["/dashboard", "/dashboard?x=1"];
		let mut i = 0usize;
		bench.bench_async("request_through_middleware_chain_2", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all_no_middleware();
		let app = &app;
		let paths = ["/dashboard", "/dashboard?x=1"];
		let mut i = 0usize;
		bench.bench_async(
			"request_through_middleware_chain_0_control",
			&rt,
			move || {
				let request = get_request(paths[i % paths.len()]);
				i += 1;
				async move {
					drive(app, request).await;
				}
			},
		);
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = ["/does-not-exist", "/also/missing/path"];
		let mut i = 0usize;
		bench.bench_async("not_found_catch_all_view", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_without_catch_all();
		let app = &app;
		let paths = ["/does-not-exist", "/also/missing/path"];
		let mut i = 0usize;
		bench.bench_async("not_found_bare_no_catch_all", &rt, move || {
			let request = get_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = ["/api/shelves/1/attachment", "/api/shelves/2/attachment"];
		let mut i = 0usize;
		bench.bench_async("head_request_to_resource", &rt, move || {
			let request = head_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}

	{
		let app = app_with_catch_all();
		let app = &app;
		let paths = [
			"/api/shelves/1/boards/2/cards/3",
			"/api/shelves/4/boards/5/cards/6",
		];
		let mut i = 0usize;
		bench.bench_async("method_not_allowed", &rt, move || {
			/*
			The card resource only serves POST. GET/HEAD on this path
			still succeeds (the root catch-all view matches every path,
			so GET/HEAD are always in the classifier's allowed set once
			one is registered) — a genuine 405 needs a method the
			classifier never widens for, so this row uses PUT against
			the known-but-wrong-method resource path.
			*/
			let request = put_request(paths[i % paths.len()]);
			i += 1;
			async move {
				drive(app, request).await;
			}
		});
	}
}
