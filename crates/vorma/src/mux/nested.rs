use std::collections::BTreeMap;
#[cfg(test)]
use std::future::Future;
use std::sync::{Arc, Mutex};

#[cfg(test)]
use serde::Serialize;
use serde_json::Value;
use vorma_matcher::{Matcher, MatcherBuilder, NestedMatches, Options as MatcherOptions, Params};
#[cfg(test)]
use vorma_tasks::Result as TaskResult;
use vorma_tasks::{CancelToken, ExecCtx};

use crate::response::Proxy;

#[cfg(test)]
use super::context::None;
use super::context::RequestBase;
#[cfg(test)]
use super::context::RequestCtx;
use super::error::{Error, RouteExecutionError};
#[cfg(test)]
use super::input::InputParser;
use super::middleware::{TaskMiddlewareInvocation, TaskMw, run_task_middleware_entries};
use super::request::RawRequest;
#[cfg(test)]
use super::task::typed_task_handler;
use super::task::{
	ErasedTask, proxy_for_task_output, record_bad_request_input_error, run_erased_task,
	run_with_exec_cancellation,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedOptions {
	pub dynamic_param_prefix: char,
	pub splat_segment_identifier: char,
	pub explicit_index_segment_identifier: String,
}

impl Default for NestedOptions {
	fn default() -> Self {
		Self {
			dynamic_param_prefix: ':',
			splat_segment_identifier: '*',
			explicit_index_segment_identifier: String::new(),
		}
	}
}

pub struct NestedRoute<S = (), E = Box<dyn std::error::Error + Send + Sync>> {
	original_pattern: String,
	task_handler: Option<Arc<dyn ErasedTask<S, E>>>,
}

impl<S, E> Clone for NestedRoute<S, E> {
	fn clone(&self) -> Self {
		Self {
			original_pattern: self.original_pattern.clone(),
			task_handler: self.task_handler.clone(),
		}
	}
}

impl<S, E> NestedRoute<S, E> {
	#[cfg(test)]
	pub fn without_task_handler(pattern: impl Into<String>) -> Self {
		Self {
			original_pattern: pattern.into(),
			task_handler: Option::None,
		}
	}

	pub fn original_pattern(&self) -> &str {
		&self.original_pattern
	}

	#[cfg(test)]
	pub fn has_task_handler(&self) -> bool {
		self.task_handler.is_some()
	}
}

pub struct NestedRouter<S = (), E = Box<dyn std::error::Error + Send + Sync>> {
	inner: NestedInner<S, E>,
}

impl<S, E> NestedRouter<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub fn new(options: NestedOptions) -> Result<Self, Error> {
		let matcher_options = MatcherOptions {
			dynamic_param_prefix: options.dynamic_param_prefix,
			splat_segment_identifier: options.splat_segment_identifier,
			explicit_index_segment_identifier: options.explicit_index_segment_identifier,
		};
		let matcher_builder =
			Matcher::builder(matcher_options.clone()).map_err(Error::InvalidMatcherOptions)?;
		let matcher = matcher_builder.clone().finish();
		Ok(Self {
			inner: NestedInner {
				matcher_builder,
				matcher,
				routes: BTreeMap::new(),
				task_mws: Vec::new(),
			},
		})
	}

	#[cfg(test)]
	pub fn add_task_handler<I, F, Fut, O>(
		&mut self,
		pattern: impl Into<String>,
		parser: InputParser<I>,
		handler: F,
	) -> Result<(), Error>
	where
		I: Send + Sync + 'static,
		F: Fn(RequestCtx<S, E, I>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<O, RouteExecutionError<E>>> + Send + 'static,
		O: Serialize + Send + Sync + 'static,
	{
		self.add_task_handler_entry(pattern, typed_task_handler(parser, handler))
	}

	pub(crate) fn add_task_handler_entry(
		&mut self,
		pattern: impl Into<String>,
		handler: Arc<dyn ErasedTask<S, E>>,
	) -> Result<(), Error> {
		self.add_route(NestedRoute::<S, E> {
			original_pattern: pattern.into(),
			task_handler: Some(handler),
		})
	}

	#[cfg(test)]
	pub(crate) fn use_task_middleware<F, Fut, O>(&mut self, handler: F) -> Result<(), Error>
	where
		F: Fn(RequestCtx<S, E, None>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		self.use_task_middleware_entry(&TaskMw::new(handler));
		Ok(())
	}

	pub(crate) fn use_task_middleware_entry(&mut self, entry: &TaskMw<S, E>) {
		self.inner.task_mws.push(entry.clone());
	}

	#[cfg(test)]
	pub fn add_pattern_without_handler(&mut self, pattern: impl Into<String>) -> Result<(), Error> {
		self.add_route(NestedRoute::<S, E>::without_task_handler(pattern))
	}

	fn add_route(&mut self, route: NestedRoute<S, E>) -> Result<(), Error> {
		if self.inner.routes.contains_key(route.original_pattern()) {
			return Err(Error::DuplicatePattern(route.original_pattern().to_owned()));
		}
		self.inner
			.matcher_builder
			.register_pattern(route.original_pattern())
			.map_err(Error::InvalidPattern)?;
		self.inner.matcher = self.inner.matcher_builder.clone().finish();
		self.inner
			.routes
			.insert(route.original_pattern.clone(), route);
		Ok(())
	}

	#[cfg(test)]
	pub fn is_registered(&self, pattern: &str) -> Result<bool, Error> {
		Ok(self.inner.routes.contains_key(pattern))
	}

	#[cfg(test)]
	pub fn has_task_handler(&self, pattern: &str) -> Result<bool, Error> {
		Ok(self
			.inner
			.routes
			.get(pattern)
			.is_some_and(NestedRoute::has_task_handler))
	}

	#[cfg(test)]
	pub fn all_routes(&self) -> Result<BTreeMap<String, NestedRoute<S, E>>, Error> {
		Ok(self.inner.routes.clone())
	}

	pub fn find_nested_matches(&self, path: &str) -> Result<Option<NestedMatches>, Error> {
		Ok(self.inner.matcher.find_nested_matches(path))
	}

	pub async fn run_nested_tasks(
		&self,
		state: Arc<S>,
		exec_ctx: ExecCtx<E>,
		request: RawRequest,
		find_results: NestedMatches,
		public_filemap: Arc<BTreeMap<String, String>>,
	) -> Result<NestedTasksResults<E>, Error> {
		let matches = find_results.matches;
		let (middleware_entries, matched_routes) = self.matched_task_inputs(&matches)?;
		let mut results = NestedTasksResults {
			middleware_proxy: Option::None,
			params: find_results.params,
			splat_values: find_results.splat_values,
			results: Vec::with_capacity(matches.len()),
		};
		let mut bound = Vec::new();

		for (matched, route) in matches.into_iter().zip(matched_routes) {
			let pattern = matched.pattern.original_pattern().to_owned();
			let index = results.results.len();
			results.results.push(NestedTasksResult {
				#[cfg(test)]
				pattern: pattern.clone(),
				data: Option::None,
				error: Option::None,
				response_proxy: Option::None,
				ran_task: false,
			});

			let Some(handler) = route.task_handler else {
				continue;
			};
			bound.push(NestedBoundTask {
				index,
				pattern,
				handler,
			});
		}

		let params = results.params.clone();
		let splat_values = results.splat_values.clone();
		let middleware_proxy = run_task_middleware_entries(
			&request,
			state.clone(),
			exec_ctx.clone(),
			public_filemap.clone(),
			params.clone(),
			splat_values.clone(),
			middleware_entries,
		)
		.await?;
		let middleware_is_terminal = middleware_proxy.is_terminal_response();
		results.middleware_proxy = Some(middleware_proxy);
		if middleware_is_terminal {
			if results.results.is_empty() {
				return Ok(results);
			}
			for result in &mut results.results {
				result.response_proxy = Some(Proxy::new());
			}
			return Ok(results);
		}
		run_nested_bound(
			NestedRunCtx {
				state,
				exec_ctx,
				request,
				public_filemap,
				params,
				splat_values,
			},
			&mut results,
			bound,
		)
		.await?;
		Ok(results)
	}

	fn matched_task_inputs(
		&self,
		matches: &[vorma_matcher::NestedMatch],
	) -> Result<NestedTaskInputs<S, E>, Error> {
		let deepest_pattern = matches
			.last()
			.ok_or_else(|| {
				Error::Invariant("nested matcher returned an empty match stack".to_owned())
			})?
			.pattern
			.original_pattern();
		let mut middleware_entries = Vec::with_capacity(self.inner.task_mws.len());
		for entry in &self.inner.task_mws {
			middleware_entries.push(TaskMiddlewareInvocation::new(entry, deepest_pattern));
		}
		let matched_routes = matches
			.iter()
			.map(|matched| {
				self.inner
					.routes
					.get(matched.pattern.original_pattern())
					.cloned()
					.ok_or_else(|| {
						Error::Invariant(format!(
							"nested matcher returned unregistered pattern `{}`",
							matched.pattern.original_pattern()
						))
					})
			})
			.collect::<Result<Vec<_>, _>>()?;
		Ok((middleware_entries, matched_routes))
	}
}

impl<S, E> Default for NestedRouter<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn default() -> Self {
		Self::new(NestedOptions::default()).expect("default nested matcher options are valid")
	}
}

struct NestedInner<S, E> {
	matcher_builder: MatcherBuilder,
	matcher: Matcher,
	routes: BTreeMap<String, NestedRoute<S, E>>,
	task_mws: Vec<TaskMw<S, E>>,
}

struct NestedBoundTask<S, E> {
	index: usize,
	pattern: String,
	handler: Arc<dyn ErasedTask<S, E>>,
}

type NestedTaskInputs<S, E> = (Vec<TaskMiddlewareInvocation<S, E>>, Vec<NestedRoute<S, E>>);

#[derive(Clone)]
struct NestedRunCtx<S, E> {
	state: Arc<S>,
	exec_ctx: ExecCtx<E>,
	request: RawRequest,
	public_filemap: Arc<BTreeMap<String, String>>,
	params: Params,
	splat_values: Vec<String>,
}

pub struct NestedTasksResults<E> {
	middleware_proxy: Option<Proxy>,
	params: Params,
	splat_values: Vec<String>,
	results: Vec<NestedTasksResult<E>>,
}

impl<E> NestedTasksResults<E> {
	pub fn results(&self) -> &[NestedTasksResult<E>] {
		&self.results
	}

	pub fn middleware_proxy(&self) -> Option<&Proxy> {
		self.middleware_proxy.as_ref()
	}
}

pub struct NestedTasksResult<E> {
	#[cfg(test)]
	pattern: String,
	data: Option<Value>,
	error: Option<RouteExecutionError<E>>,
	response_proxy: Option<Proxy>,
	ran_task: bool,
}

impl<E> NestedTasksResult<E> {
	#[cfg(test)]
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	pub fn data(&self) -> Option<&Value> {
		self.data.as_ref()
	}

	pub fn error(&self) -> Option<&RouteExecutionError<E>> {
		self.error.as_ref()
	}

	pub fn response_proxy(&self) -> Option<&Proxy> {
		self.response_proxy.as_ref()
	}

	pub fn ran_task(&self) -> bool {
		self.ran_task
	}
}

struct NestedTaskOutput<E> {
	index: usize,
	data: Option<Value>,
	error: Option<RouteExecutionError<E>>,
	proxy: Proxy,
}

impl<E> Clone for NestedTaskOutput<E>
where
	RouteExecutionError<E>: Clone,
{
	fn clone(&self) -> Self {
		Self {
			index: self.index,
			data: self.data.clone(),
			error: self.error.clone(),
			proxy: self.proxy.clone(),
		}
	}
}

async fn run_nested_bound<S, E>(
	ctx: NestedRunCtx<S, E>,
	results: &mut NestedTasksResults<E>,
	bound: Vec<NestedBoundTask<S, E>>,
) -> Result<(), Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	match bound.len() {
		0 => return Ok(()),
		1 => {
			let output =
				run_one_nested_bound(ctx, bound.into_iter().next().expect("one bound task"))
					.await?;
			apply_nested_output(results, output)?;
			return Ok(());
		}
		_ => {}
	}

	let mut current_exec_ctx = ctx.exec_ctx;
	let bound_len = bound.len();
	let mut handles = Vec::with_capacity(bound_len);
	for (bound_index, bound_task) in bound.into_iter().enumerate() {
		let task_exec_ctx = current_exec_ctx.clone();
		let cancel_descendants = if bound_index < bound_len - 1 {
			let descendant_exec_ctx = current_exec_ctx.child();
			let cancel = descendant_exec_ctx.cancel_token().clone();
			current_exec_ctx = descendant_exec_ctx;
			Some(cancel)
		} else {
			Option::None
		};
		let run_ctx = NestedRunCtx {
			state: ctx.state.clone(),
			exec_ctx: task_exec_ctx,
			request: ctx.request.clone(),
			public_filemap: ctx.public_filemap.clone(),
			params: ctx.params.clone(),
			splat_values: ctx.splat_values.clone(),
		};
		handles.push(tokio::spawn(async move {
			run_one_nested_bound_with_cancel(run_ctx, bound_task, cancel_descendants).await
		}));
	}

	let mut outputs = Vec::with_capacity(handles.len());
	for handle in handles {
		outputs.push(
			handle
				.await
				.map_err(|error| Error::TaskJoin(error.to_string()))??,
		);
	}
	outputs.sort_by_key(|output| output.index);
	for output in outputs {
		apply_nested_output(results, output)?;
	}
	Ok(())
}

async fn run_one_nested_bound<S, E>(
	ctx: NestedRunCtx<S, E>,
	bound_task: NestedBoundTask<S, E>,
) -> Result<NestedTaskOutput<E>, Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	run_one_nested_bound_with_cancel(ctx, bound_task, Option::None).await
}

async fn run_one_nested_bound_with_cancel<S, E>(
	ctx: NestedRunCtx<S, E>,
	bound_task: NestedBoundTask<S, E>,
	cancel_descendants: Option<CancelToken>,
) -> Result<NestedTaskOutput<E>, Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let proxy = Arc::new(Mutex::new(Proxy::new()));
	let exec_ctx = ctx.exec_ctx;
	let handler = bound_task.handler;
	let pattern = bound_task.pattern.clone();
	let index = bound_task.index;
	let proxy_for_task = proxy.clone();
	let request_for_task = ctx.request.clone();
	let state_for_task = ctx.state.clone();
	let public_filemap_for_task = ctx.public_filemap.clone();
	let params_for_task = ctx.params.clone();
	let splat_values_for_task = ctx.splat_values.clone();
	let handler_exec_ctx = exec_ctx.clone();
	match run_with_exec_cancellation(&exec_ctx, async move {
		let mut handler_run = run_erased_task(
			handler,
			request_for_task,
			RequestBase {
				matched_pattern: pattern,
				params: params_for_task,
				splat_values: splat_values_for_task,
				state: state_for_task,
				exec_ctx: handler_exec_ctx,
				public_filemap: public_filemap_for_task,
				response_proxy: proxy_for_task,
			},
		)
		.await;
		record_bad_request_input_error(&mut handler_run.proxy, &handler_run.output);
		let should_cancel_descendants =
			handler_run.output.is_err() || handler_run.proxy.is_terminal_response();
		if should_cancel_descendants && let Some(cancel) = cancel_descendants {
			cancel.cancel();
		}
		let (data, error, proxy) = handler_run.into_parts();
		let proxy = proxy_for_task_output(error.is_some(), proxy);
		NestedTaskOutput {
			index,
			data,
			error,
			proxy,
		}
	})
	.await
	{
		Ok(output) => Ok(output),
		Err(error) => Ok(NestedTaskOutput {
			index,
			data: Option::None,
			error: Some(RouteExecutionError::Task(error)),
			proxy: Proxy::new(),
		}),
	}
}

fn apply_nested_output<E>(
	results: &mut NestedTasksResults<E>,
	output: NestedTaskOutput<E>,
) -> Result<(), Error> {
	let Some(result) = results.results.get_mut(output.index) else {
		return Err(Error::Invariant(format!(
			"nested task output index {} out of range for {} results",
			output.index,
			results.results.len()
		)));
	};
	result.data = output.data;
	result.error = output.error;
	result.response_proxy = Some(output.proxy);
	result.ran_task = true;
	Ok(())
}
