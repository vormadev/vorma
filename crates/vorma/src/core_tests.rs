use std::collections::BTreeMap;
use std::sync::Arc;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use vorma_tasks::{CancelToken, Tasks, TasksOptions};

use super::*;
use crate::mux::RawRequest;
use crate::response::CLIENT_ACCEPTS_REDIRECT_HEADER;

fn exec_ctx() -> vorma_tasks::ExecCtx<&'static str> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

fn boxed_exec_ctx() -> vorma_tasks::ExecCtx<Box<dyn std::error::Error + Send + Sync>> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

crate::app!(mod macro_app for ());

#[derive(Clone, Debug, Deserialize, crate::TsGen)]
#[serde(rename_all = "camelCase")]
struct MacroStoryInput {
	draft_title: Option<String>,
}

#[derive(Clone, Debug, Serialize, crate::TsGen)]
#[serde(rename_all = "camelCase")]
struct MacroStoryOutput {
	story_id: String,
}

#[derive(Clone, Debug, Serialize, crate::TsGen)]
#[serde(rename_all = "camelCase")]
struct MacroAssetOutput {
	app_css: String,
}

pub const MACRO_STORY_VIEW: macro_app::View = macro_app::view! {
	client_file: "story.view.tsx";
	pattern: "/stories/:story_id";
	input: MacroStoryInput;
	output: MacroStoryOutput;

	handler: |ctx| {
		assert_eq!(ctx.params().story_id, "123");
		assert_eq!(ctx.input().draft_title, Option::None);
		Ok(MacroStoryOutput {
			story_id: ctx.params().story_id.clone(),
		})
	};
};

pub const MACRO_ASSET_VIEW: macro_app::View = macro_app::view! {
	client_file: "asset.view.tsx";
	pattern: "/assets";
	input: ();
	output: MacroAssetOutput;

	handler: |ctx| {
		Ok(MacroAssetOutput {
			app_css: ctx.public_url("app.css")?,
		})
	};
};

pub const MACRO_STORY_API: macro_app::Resource = macro_app::resource! {
	kind: crate::ResourceKind::Mutation;
	method: crate::HttpMethod::GET;
	pattern: "/stories/:story_id";
	input: MacroStoryInput;
	output: MacroStoryOutput;

	handler: |ctx| {
		assert_eq!(ctx.params().story_id, "456");
		assert_eq!(ctx.input().draft_title, Option::None);
		Ok(MacroStoryOutput {
			story_id: ctx.params().story_id.clone(),
		})
	};
};

#[test]
fn accepts_client_redirect_matches_bool_header_semantics() {
	let mut headers = http::HeaderMap::new();
	assert!(!accepts_client_redirect(&headers));

	headers.insert(
		CLIENT_ACCEPTS_REDIRECT_HEADER,
		http::HeaderValue::from_static("false"),
	);
	assert!(!accepts_client_redirect(&headers));

	headers.insert(
		CLIENT_ACCEPTS_REDIRECT_HEADER,
		http::HeaderValue::from_static("true"),
	);
	assert!(accepts_client_redirect(&headers));
}

#[test]
fn resource_kind_defaults_follow_method() {
	let get_route: Resource<(), &'static str> =
		Resource::without_handler(Method::GET, "/health", Option::None);
	let post_route: Resource<(), &'static str> =
		Resource::without_handler(Method::POST, "/sessions", Option::None);
	let explicit: Resource<(), &'static str> =
		Resource::without_handler(Method::POST, "/search", Some(ResourceKind::Query));

	assert_eq!(get_route.kind(), Option::None);
	assert_eq!(post_route.kind(), Option::None);
	assert_eq!(explicit.kind(), Some(ResourceKind::Query));
	assert_eq!(
		default_resource_kind_for_method(get_route.method()),
		ResourceKind::Query
	);
	assert_eq!(
		default_resource_kind_for_method(post_route.method()),
		ResourceKind::Mutation
	);
}

#[test]
fn contract_for_validates_view_patterns_before_contract_output() {
	let mut views: Views<(), &'static str> = Views::new();
	views.push(View::new(
		"",
		"bad.view.tsx",
		InputParser::<()>::default_input(),
		|_: ViewCtx<(), &'static str, ()>| async { Ok(()) },
	));
	let resources: Resources<(), &'static str> = Resources::new();

	let error = contract_for(&views, &resources).unwrap_err();

	assert!(error.starts_with("error validating app route contract:"));
	assert!(error.contains("pattern must not be empty"));
}

#[test]
fn contract_for_validates_resource_patterns_before_contract_output() {
	let views: Views<(), &'static str> = Views::new();
	let mut resources: Resources<(), &'static str> = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"",
		Option::None,
		InputParser::<()>::default_input(),
		|_: ResourceCtx<(), &'static str, ()>| async { Ok(()) },
	));

	let error = contract_for(&views, &resources).unwrap_err();

	assert!(error.starts_with("error validating app route contract:"));
	assert!(error.contains("resource pattern must not be empty"));
}

#[test]
fn contract_for_validates_resource_collisions_before_contract_output() {
	let views: Views<(), &'static str> = Views::new();
	let mut resources: Resources<(), &'static str> = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/stories/:story_id",
		Option::None,
		InputParser::<()>::default_input(),
		|_: ResourceCtx<(), &'static str, ()>| async { Ok(()) },
	));
	resources.push(Resource::new(
		Method::GET,
		"/stories/:id",
		Option::None,
		InputParser::<()>::default_input(),
		|_: ResourceCtx<(), &'static str, ()>| async { Ok(()) },
	));

	let error = contract_for(&views, &resources).unwrap_err();

	assert!(error.starts_with("error validating app route contract:"));
	assert!(error.contains("route shape collision"));
}

#[tokio::test]
async fn runtime_routes_register_views_and_resources() {
	let mut views: Views<(), &'static str> = Views::new();
	views.push(View::new(
		"/users/:id",
		"users.view.tsx",
		InputParser::<()>::default_input(),
		|ctx: ViewCtx<(), &'static str, ()>| async move {
			assert_eq!(ctx.request().path(), "/users/123");
			ctx.head().title("User");
			Ok(ctx.param("id").to_owned())
		},
	));

	let mut resources: Resources<(), &'static str> = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/users/:id",
		Option::None,
		InputParser::<()>::default_input(),
		|ctx: ResourceCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(http::StatusCode::CREATED);
			Ok(ctx.param("id").to_owned())
		},
	));

	let middlewares = Middlewares::new();
	let routes = runtime_routes_for(&views, &resources, &middlewares, "/api/").unwrap();
	let view_matches = routes
		.views
		.find_nested_matches("/users/123")
		.unwrap()
		.unwrap();
	let view_results = routes
		.views
		.run_nested_tasks(
			Arc::new(()),
			exec_ctx(),
			RawRequest::get("/users/123"),
			view_matches,
			Arc::new(std::collections::BTreeMap::new()),
		)
		.await
		.unwrap();
	assert_eq!(
		view_results.results()[0].data().and_then(Value::as_str),
		Some("123")
	);

	let api_result = routes
		.resources
		.execute_route(
			RawRequest::get("/api/users/456"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(std::collections::BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	assert_eq!(api_result.data().and_then(Value::as_str), Some("456"));
	assert_eq!(
		api_result.response_proxy().status().0,
		Some(http::StatusCode::CREATED)
	);
}

#[tokio::test]
async fn macros_define_const_routes_with_inferred_typed_params() {
	let views = macro_app::views![MACRO_STORY_VIEW];
	let resources = macro_app::resources![MACRO_STORY_API];

	let middlewares = Middlewares::new();
	let routes = runtime_routes_for(&views, &resources, &middlewares, "/api/").unwrap();
	let view_matches = routes
		.views
		.find_nested_matches("/stories/123")
		.unwrap()
		.unwrap();
	let view_results = routes
		.views
		.run_nested_tasks(
			Arc::new(()),
			boxed_exec_ctx(),
			RawRequest::get("/stories/123"),
			view_matches,
			Arc::new(std::collections::BTreeMap::new()),
		)
		.await
		.unwrap();
	assert_eq!(view_results.results()[0].data().unwrap()["storyId"], "123");

	let api_result = routes
		.resources
		.execute_route(
			RawRequest::get("/api/stories/456"),
			Arc::new(()),
			boxed_exec_ctx(),
			Arc::new(std::collections::BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	assert_eq!(api_result.data().unwrap()["storyId"], "456");
}

#[tokio::test]
async fn macros_support_public_url_in_default_error_context() {
	let views = macro_app::views![MACRO_ASSET_VIEW];
	let resources = macro_app::resources![];
	let middlewares = macro_app::middlewares![];
	let routes = runtime_routes_for(&views, &resources, &middlewares, "/api/").unwrap();
	let view_matches = routes
		.views
		.find_nested_matches("/assets")
		.unwrap()
		.unwrap();
	let view_results = routes
		.views
		.run_nested_tasks(
			Arc::new(()),
			boxed_exec_ctx(),
			RawRequest::get("/assets"),
			view_matches,
			Arc::new(BTreeMap::from([(
				"app.css".to_owned(),
				"/static/app.css".to_owned(),
			)])),
		)
		.await
		.unwrap();

	assert_eq!(
		view_results.results()[0].data().unwrap()["appCss"],
		"/static/app.css"
	);
}

#[tokio::test]
async fn public_middlewares_wire_once_and_filter_from_request_context() {
	let views = macro_app::views![MACRO_STORY_VIEW];
	let resources = macro_app::resources![MACRO_STORY_API];
	let middlewares = macro_app::middlewares![macro_app::Middleware::new(|ctx| async move {
		if ctx.request().method() != Method::GET {
			return Ok(());
		}
		if ctx.matched_pattern() != "/stories/:story_id" {
			return Ok(());
		}
		let story_id = ctx.param("story_id");
		let params = ctx.params();
		assert_eq!(params.len(), 1);
		assert!(!params.is_empty());
		assert_eq!(params.get("story_id"), Some(story_id));
		assert_eq!(
			params.iter().collect::<Vec<_>>(),
			vec![("story_id", story_id)]
		);
		let header_value = http::HeaderValue::from_str(story_id).unwrap();
		ctx.response()
			.set_header(http::HeaderName::from_static("x-vorma-story"), header_value);
		Ok(())
	})];
	let routes = runtime_routes_for(&views, &resources, &middlewares, "/api/").unwrap();

	let view_matches = routes
		.views
		.find_nested_matches("/stories/123")
		.unwrap()
		.unwrap();
	let view_results = routes
		.views
		.run_nested_tasks(
			Arc::new(()),
			boxed_exec_ctx(),
			RawRequest::get("/stories/123"),
			view_matches,
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap();
	let view_header = view_results
		.middleware_proxy()
		.unwrap()
		.header(&http::HeaderName::from_static("x-vorma-story"))
		.unwrap();
	assert_eq!(view_header.to_str().unwrap(), "123");

	let api_result = routes
		.resources
		.execute_route(
			RawRequest::get("/api/stories/456"),
			Arc::new(()),
			boxed_exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let api_header = api_result
		.response_proxy()
		.header(&http::HeaderName::from_static("x-vorma-story"))
		.unwrap();
	assert_eq!(api_header.to_str().unwrap(), "456");
}
