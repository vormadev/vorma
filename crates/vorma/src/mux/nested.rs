use std::collections::BTreeMap;
#[cfg(test)]
use std::future::Future;
use std::sync::{Arc, Mutex};

#[cfg(test)]
use serde::Serialize;
use serde_json::Value;
use vorma_matcher::{Matcher, MatcherBuilder, NestedMatches, Options as MatcherOptions, Params};
use vorma_tasks::ExecCtx;
#[cfg(test)]
use vorma_tasks::Result as TaskResult;

use crate::response::ResponseEffects;

#[cfg(test)]
use super::context::None;
use super::context::RequestBase;
#[cfg(test)]
use super::context::RequestCtx;
use super::error::{Error, RouteExecutionError};
#[cfg(test)]
use super::input::InputParser;
use super::middleware::{Middleware, MiddlewareInvocation, run_middleware_entries};
use super::ordered_parallel::{OrderedTaskContexts, OrderedTaskCtx, run_ordered_parallel};
use super::request::RawRequest;
#[cfg(test)]
use super::task::typed_handler;
use super::task::{ErasedTask, run_handler_and_collect_effects, run_with_exec_cancellation};

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
	handler: Option<Arc<dyn ErasedTask<S, E>>>,
}

impl<S, E> Clone for NestedRoute<S, E> {
	fn clone(&self) -> Self {
		Self {
			original_pattern: self.original_pattern.clone(),
			handler: self.handler.clone(),
		}
	}
}

impl<S, E> NestedRoute<S, E> {
	#[cfg(test)]
	pub fn without_handler(pattern: impl Into<String>) -> Self {
		Self {
			original_pattern: pattern.into(),
			handler: Option::None,
		}
	}

	pub fn original_pattern(&self) -> &str {
		&self.original_pattern
	}

	#[cfg(test)]
	pub fn has_handler(&self) -> bool {
		self.handler.is_some()
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
				middlewares: Vec::new(),
			},
		})
	}

	#[cfg(test)]
	pub fn add_handler<I, F, Fut, O>(
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
		self.add_handler_entry(pattern, typed_handler(parser, handler))
	}

	pub(crate) fn add_handler_entry(
		&mut self,
		pattern: impl Into<String>,
		handler: Arc<dyn ErasedTask<S, E>>,
	) -> Result<(), Error> {
		self.add_route(NestedRoute::<S, E> {
			original_pattern: pattern.into(),
			handler: Some(handler),
		})
	}

	#[cfg(test)]
	pub(crate) fn use_middleware<F, Fut, O>(&mut self, handler: F) -> Result<(), Error>
	where
		F: Fn(RequestCtx<S, E, None>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		self.use_middleware_entry(&Middleware::new(handler));
		Ok(())
	}

	pub(crate) fn use_middleware_entry(&mut self, entry: &Middleware<S, E>) {
		self.inner.middlewares.push(entry.clone());
	}

	#[cfg(test)]
	pub fn add_pattern_without_handler(&mut self, pattern: impl Into<String>) -> Result<(), Error> {
		self.add_route(NestedRoute::<S, E>::without_handler(pattern))
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
	pub fn has_handler(&self, pattern: &str) -> Result<bool, Error> {
		Ok(self
			.inner
			.routes
			.get(pattern)
			.is_some_and(NestedRoute::has_handler))
	}

	#[cfg(test)]
	pub fn all_routes(&self) -> Result<BTreeMap<String, NestedRoute<S, E>>, Error> {
		Ok(self.inner.routes.clone())
	}

	pub fn find_nested_matches(&self, path: &str) -> Result<Option<NestedMatches>, Error> {
		Ok(self.inner.matcher.find_nested_matches(path))
	}

	pub async fn execute_view_stack(
		&self,
		state: Arc<S>,
		exec_ctx: ExecCtx<E>,
		request: RawRequest,
		find_results: NestedMatches,
		public_filemap: Arc<BTreeMap<String, String>>,
	) -> Result<ViewStackExecution<E>, Error> {
		let matches = find_results.matches;
		let matched_patterns = matches
			.iter()
			.map(|matched| matched.pattern.original_pattern().to_owned())
			.collect::<Vec<_>>();
		let (middleware_entries, matched_routes) = self.matched_task_inputs(&matches)?;
		let params = find_results.params;
		let splat_values = find_results.splat_values;
		let mut view_results = Vec::with_capacity(matches.len());
		let mut bound = Vec::new();

		for (matched, route) in matches.into_iter().zip(matched_routes) {
			let pattern = matched.pattern.original_pattern().to_owned();
			let index = view_results.len();
			view_results.push(ViewExecutionResult {
				#[cfg(test)]
				pattern: pattern.clone(),
				data: Option::None,
				error: Option::None,
				response_effects: Option::None,
				ran_task: false,
			});

			let Some(handler) = route.handler else {
				continue;
			};
			bound.push(NestedBoundTask {
				index,
				pattern,
				handler,
			});
		}

		let middleware_effects = run_middleware_entries(
			&request,
			state.clone(),
			exec_ctx.clone(),
			public_filemap.clone(),
			params.clone(),
			splat_values.clone(),
			middleware_entries,
		)
		.await?;
		let middleware_is_terminal = middleware_effects.is_terminal_response();
		if middleware_is_terminal {
			for result in &mut view_results {
				result.response_effects = Some(ResponseEffects::new());
			}
			return Ok(ViewStackExecution {
				matched_patterns,
				params,
				splat_values,
				middleware_effects,
				view_results,
				terminal_boundary: Some(ViewStackTerminalBoundary::Middleware),
			});
		}
		run_nested_bound(
			NestedRunCtx {
				state,
				exec_ctx,
				request,
				public_filemap,
				params: params.clone(),
				splat_values: splat_values.clone(),
			},
			&mut view_results,
			bound,
		)
		.await?;
		let terminal_boundary = terminal_view_boundary(&view_results);
		Ok(ViewStackExecution {
			matched_patterns,
			params,
			splat_values,
			middleware_effects,
			view_results,
			terminal_boundary,
		})
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
		let mut middleware_entries = Vec::with_capacity(self.inner.middlewares.len());
		for entry in &self.inner.middlewares {
			middleware_entries.push(MiddlewareInvocation::new(entry, deepest_pattern));
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
	middlewares: Vec<Middleware<S, E>>,
}

struct NestedBoundTask<S, E> {
	index: usize,
	pattern: String,
	handler: Arc<dyn ErasedTask<S, E>>,
}

type NestedTaskInputs<S, E> = (Vec<MiddlewareInvocation<S, E>>, Vec<NestedRoute<S, E>>);

#[derive(Clone)]
struct NestedRunCtx<S, E> {
	state: Arc<S>,
	exec_ctx: ExecCtx<E>,
	request: RawRequest,
	public_filemap: Arc<BTreeMap<String, String>>,
	params: Params,
	splat_values: Vec<String>,
}

pub struct ViewStackExecution<E> {
	matched_patterns: Vec<String>,
	params: Params,
	splat_values: Vec<String>,
	middleware_effects: ResponseEffects,
	view_results: Vec<ViewExecutionResult<E>>,
	terminal_boundary: Option<ViewStackTerminalBoundary>,
}

impl<E> ViewStackExecution<E> {
	pub fn matched_patterns(&self) -> &[String] {
		&self.matched_patterns
	}

	pub fn params(&self) -> &Params {
		&self.params
	}

	pub fn splat_values(&self) -> &[String] {
		&self.splat_values
	}

	pub fn view_results(&self) -> &[ViewExecutionResult<E>] {
		&self.view_results
	}

	pub fn middleware_effects(&self) -> &ResponseEffects {
		&self.middleware_effects
	}

	pub fn terminal_boundary(&self) -> Option<ViewStackTerminalBoundary> {
		self.terminal_boundary
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ViewStackTerminalBoundary {
	Middleware,
	View { index: usize },
}

pub struct ViewExecutionResult<E> {
	#[cfg(test)]
	pattern: String,
	data: Option<Value>,
	error: Option<RouteExecutionError<E>>,
	response_effects: Option<ResponseEffects>,
	ran_task: bool,
}

impl<E> ViewExecutionResult<E> {
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

	pub fn response_effects(&self) -> Option<&ResponseEffects> {
		self.response_effects.as_ref()
	}

	pub fn ran_task(&self) -> bool {
		self.ran_task
	}
}

struct NestedTaskOutput<E> {
	index: usize,
	data: Option<Value>,
	error: Option<RouteExecutionError<E>>,
	effects: ResponseEffects,
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
			effects: self.effects.clone(),
		}
	}
}

async fn run_nested_bound<S, E>(
	ctx: NestedRunCtx<S, E>,
	view_results: &mut [ViewExecutionResult<E>],
	bound: Vec<NestedBoundTask<S, E>>,
) -> Result<(), Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	if bound.is_empty() {
		return Ok(());
	}

	let task_contexts = OrderedTaskContexts::descendant_chain(ctx.exec_ctx.clone(), bound.len());
	let outputs = run_ordered_parallel(bound, task_contexts, move |bound_task, task_ctx| {
		let ctx = NestedRunCtx {
			state: ctx.state.clone(),
			exec_ctx: task_ctx.exec_ctx(),
			request: ctx.request.clone(),
			public_filemap: ctx.public_filemap.clone(),
			params: ctx.params.clone(),
			splat_values: ctx.splat_values.clone(),
		};
		async move { run_one_nested_bound_with_cancel(ctx, bound_task, task_ctx).await }
	})
	.await?;
	for (_, output) in outputs.into_vec() {
		let output = output?;
		apply_nested_output(view_results, output)?;
	}
	Ok(())
}

async fn run_one_nested_bound_with_cancel<S, E>(
	ctx: NestedRunCtx<S, E>,
	bound_task: NestedBoundTask<S, E>,
	task_ctx: OrderedTaskCtx<E>,
) -> Result<NestedTaskOutput<E>, Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let effects = Arc::new(Mutex::new(ResponseEffects::new()));
	let exec_ctx = ctx.exec_ctx;
	let handler = bound_task.handler;
	let pattern = bound_task.pattern.clone();
	let index = bound_task.index;
	let effects_for_task = effects.clone();
	let request_for_task = ctx.request.clone();
	let state_for_task = ctx.state.clone();
	let public_filemap_for_task = ctx.public_filemap.clone();
	let params_for_task = ctx.params.clone();
	let splat_values_for_task = ctx.splat_values.clone();
	let handler_exec_ctx = exec_ctx.clone();
	match run_with_exec_cancellation(&exec_ctx, async move {
		let handler_execution = run_handler_and_collect_effects(
			handler,
			request_for_task,
			RequestBase {
				matched_pattern: pattern,
				params: params_for_task,
				splat_values: splat_values_for_task,
				state: state_for_task,
				exec_ctx: handler_exec_ctx,
				public_filemap: public_filemap_for_task,
				response_effects: effects_for_task,
			},
		)
		.await;
		let should_cancel_descendants =
			handler_execution.error.is_some() || handler_execution.effects.is_terminal_response();
		if should_cancel_descendants {
			task_ctx.cancel_later();
		}
		NestedTaskOutput {
			index,
			data: handler_execution.data,
			error: handler_execution.error,
			effects: handler_execution.effects,
		}
	})
	.await
	{
		Ok(output) => Ok(output),
		Err(error) => Ok(NestedTaskOutput {
			index,
			data: Option::None,
			error: Some(RouteExecutionError::Task(error)),
			effects: ResponseEffects::new(),
		}),
	}
}

fn terminal_view_boundary<E>(
	view_results: &[ViewExecutionResult<E>],
) -> Option<ViewStackTerminalBoundary> {
	view_results.iter().enumerate().find_map(|(index, result)| {
		if result.error().is_some()
			|| result
				.response_effects()
				.is_some_and(ResponseEffects::is_terminal_response)
		{
			return Some(ViewStackTerminalBoundary::View { index });
		}
		Option::None
	})
}

fn apply_nested_output<E>(
	view_results: &mut [ViewExecutionResult<E>],
	output: NestedTaskOutput<E>,
) -> Result<(), Error> {
	let view_results_len = view_results.len();
	let Some(result) = view_results.get_mut(output.index) else {
		return Err(Error::Invariant(format!(
			"nested task output index {} out of range for {} results",
			output.index, view_results_len
		)));
	};
	result.data = output.data;
	result.error = output.error;
	result.response_effects = Some(output.effects);
	result.ran_task = true;
	Ok(())
}
