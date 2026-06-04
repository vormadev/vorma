use std::collections::BTreeMap;
use std::error::Error;
use std::sync::Arc;
use std::time::Duration;

use http::Method;
use http::header::{HeaderName, HeaderValue, LOCATION, SET_COOKIE};
use serde_json::json;
use tokio::sync::Notify;
use vorma_tasks::{CancelToken, Tasks, TasksOptions};

use super::*;
use crate::constants::X_VORMA_CLIENT_BUILD_ID;
use crate::error::{ViewError, ViewErrorClientMsg};
use crate::manifest::{ClientCoreAssets, ClientModule, Manifest};
use crate::mux::{InputParser, RawRequest, Router};

fn exec_ctx() -> vorma_tasks::ExecCtx<&'static str> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

fn boxed_exec_ctx() -> vorma_tasks::ExecCtx<Box<dyn Error + Send + Sync>> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

fn manifest_with_views(views: &[(&str, &str)]) -> Manifest {
	Manifest {
		client_views: views
			.iter()
			.map(|(pattern, url)| {
				(
					(*pattern).to_owned(),
					ClientModule {
						url: (*url).to_owned(),
						..ClientModule::default()
					},
				)
			})
			.collect(),
		search_schemas: views
			.iter()
			.map(|(pattern, _)| ((*pattern).to_owned(), serde_json::Value::Null))
			.collect(),
		..Manifest::default()
	}
}

fn manifest_with_client_view_modules(views: &[(&str, ClientModule)]) -> Manifest {
	Manifest {
		client_views: views
			.iter()
			.map(|(pattern, module)| ((*pattern).to_owned(), module.clone()))
			.collect(),
		search_schemas: views
			.iter()
			.map(|(pattern, _)| ((*pattern).to_owned(), serde_json::Value::Null))
			.collect(),
		..Manifest::default()
	}
}

fn resource_get(path: &str) -> RawRequest {
	RawRequest::get(api_path(path))
}

fn api_request(method: Method, path: &str) -> RawRequest {
	RawRequest::new(
		method,
		api_path(path).parse().unwrap(),
		Default::default(),
		Bytes::new(),
	)
}

fn api_path(path: &str) -> String {
	assert!(path.starts_with('/'));
	format!("/api{path}")
}

#[derive(Debug)]
struct EmptyClientMsgError;

impl ViewErrorClientMsg for EmptyClientMsgError {
	fn view_error_client_msg(&self) -> Option<&str> {
		Some("")
	}
}

mod api_response;
mod dev_assets;
mod view_response;
