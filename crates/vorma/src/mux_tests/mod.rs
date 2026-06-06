use std::collections::BTreeMap;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use http::{HeaderName, HeaderValue, Method, StatusCode};
use serde_json::Value;
use tokio::sync::Notify;
use vorma_tasks::{CancelToken, Error as TaskError, ExecCtx, Task, Tasks, TasksOptions};

use crate::mux::{
	InputError, InputParser, NestedOptions, NestedRouter, None, RawRequest, RequestCtx,
	RouteExecutionError, Router, ViewExecutionTerminalBoundary,
};

#[derive(Clone)]
struct TestEnv {
	shared_runs: Arc<AtomicUsize>,
}

#[derive(Clone, Debug, Eq, Hash, PartialEq)]
struct SharedInput;

fn default_runtime() -> (Arc<TestEnv>, ExecCtx<&'static str>) {
	let tasks = Tasks::new(TasksOptions::default());
	(
		Arc::new(TestEnv {
			shared_runs: Arc::new(AtomicUsize::new(0)),
		}),
		tasks.exec_ctx(CancelToken::new()),
	)
}

fn test_runtime(shared_runs: Arc<AtomicUsize>) -> (Arc<TestEnv>, ExecCtx<&'static str>) {
	let tasks = Tasks::new(TasksOptions::default());
	(
		Arc::new(TestEnv { shared_runs }),
		tasks.exec_ctx(CancelToken::new()),
	)
}

fn no_input() -> InputParser<()> {
	InputParser::default_input()
}

fn empty_public_filemap() -> Arc<BTreeMap<String, String>> {
	Arc::new(BTreeMap::new())
}

fn api_request(path: &str) -> RawRequest {
	if path == "/" {
		return RawRequest::get("/api");
	}
	RawRequest::get(format!("/api{path}"))
}

#[derive(Clone, Default)]
struct GreetingInput {
	name: String,
}

mod middleware;
mod nested_views;
mod router_and_resources;
