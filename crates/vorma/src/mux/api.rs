use std::collections::{BTreeMap, BTreeSet, HashMap};
#[cfg(test)]
use std::future::Future;
use std::sync::{Arc, Mutex};

use http::Method;
#[cfg(test)]
use serde::Serialize;
use serde_json::Value;
use vorma_matcher::{Matcher, MatcherBuilder, Options as MatcherOptions};
use vorma_tasks::ExecCtx;
#[cfg(test)]
use vorma_tasks::Result as TaskResult;

use crate::config::normalize_api_mount_root;
use crate::response::ResponseEffects;

#[cfg(test)]
use super::context::None;
use super::context::RequestBase;
#[cfg(test)]
use super::context::RequestCtx;
use super::error::{Error, RouteExecutionError};
#[cfg(test)]
use super::input::InputParser;
use super::middleware::{
	Middleware, MiddlewareInvocation, merge_owned_response_effects, run_middleware_entries,
};
use super::request::{RawRequest, RouteMatch, route_match};
#[cfg(test)]
use super::task::typed_handler;
use super::task::{ErasedTask, run_handler_and_collect_effects};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Options {
	pub mount_root: String,
	pub dynamic_param_prefix: char,
	pub splat_segment_identifier: char,
}

impl Default for Options {
	fn default() -> Self {
		Self {
			mount_root: "/api/".to_owned(),
			dynamic_param_prefix: ':',
			splat_segment_identifier: '*',
		}
	}
}

pub struct Router<S = (), E = Box<dyn std::error::Error + Send + Sync>> {
	matcher_options: MatcherOptions,
	mount_root: String,
	method_matchers: HashMap<Method, MethodMatcher<S, E>>,
	middlewares: Vec<Middleware<S, E>>,
}

impl<S, E> Router<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub fn new(options: Options) -> Result<Self, Error> {
		let matcher_options = MatcherOptions {
			dynamic_param_prefix: options.dynamic_param_prefix,
			splat_segment_identifier: options.splat_segment_identifier,
			explicit_index_segment_identifier: String::new(),
		};
		Matcher::builder(matcher_options.clone()).map_err(Error::InvalidMatcherOptions)?;
		let mount_root =
			normalize_api_mount_root(&options.mount_root).map_err(Error::InvalidMountRoot)?;
		Ok(Self {
			matcher_options,
			mount_root,
			method_matchers: HashMap::new(),
			middlewares: Vec::new(),
		})
	}

	#[cfg(test)]
	pub fn add_handler<I, F, Fut, O>(
		&mut self,
		method: Method,
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
		self.add_handler_entry(method, pattern, typed_handler(parser, handler))
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
		self.middlewares.push(entry.clone());
	}

	pub(crate) fn add_handler_entry(
		&mut self,
		method: Method,
		pattern: impl Into<String>,
		handler: Arc<dyn ErasedTask<S, E>>,
	) -> Result<(), Error> {
		let pattern = pattern.into();
		let method_matcher = self.get_or_create_method_matcher(method.clone())?;
		if method_matcher.routes.contains_key(&pattern) {
			return Err(Error::DuplicatePattern(pattern));
		}
		method_matcher
			.matcher_builder
			.register_pattern(&pattern)
			.map_err(Error::InvalidPattern)?;
		method_matcher.matcher = method_matcher.matcher_builder.clone().finish();
		method_matcher
			.routes
			.insert(pattern, RouteEntry { handler });
		Ok(())
	}

	pub fn find_best(&self, method: &Method, path: &str) -> Option<RouteMatch> {
		let path = self.path_for_matching(path)?;
		if method == Method::HEAD {
			if let Some(method_matcher) = self.method_matchers.get(&Method::HEAD)
				&& let Some(found) = method_matcher.matcher.find_best_match(&path)
			{
				return Some(route_match(Method::HEAD, found, false));
			}
			let method_matcher = self.method_matchers.get(&Method::GET)?;
			return method_matcher
				.matcher
				.find_best_match(&path)
				.map(|found| route_match(Method::GET, found, true));
		}
		let method_matcher = self.method_matchers.get(method)?;
		method_matcher
			.matcher
			.find_best_match(&path)
			.map(|found| route_match(method.clone(), found, false))
	}

	pub(crate) fn allowed_methods_for_path(&self, path: &str) -> BTreeSet<String> {
		let Some(path) = self.path_for_matching(path) else {
			return BTreeSet::new();
		};
		let mut methods = BTreeSet::new();
		for (method, method_matcher) in &self.method_matchers {
			if method_matcher.matcher.find_best_match(&path).is_some() {
				methods.insert(method.as_str().to_owned());
			}
		}
		if methods.contains(Method::GET.as_str()) {
			methods.insert(Method::HEAD.as_str().to_owned());
		}
		methods
	}

	pub(crate) fn allowed_methods(&self) -> BTreeSet<String> {
		let mut methods = self
			.method_matchers
			.keys()
			.map(|method| method.as_str().to_owned())
			.collect::<BTreeSet<_>>();
		if methods.contains(Method::GET.as_str()) {
			methods.insert(Method::HEAD.as_str().to_owned());
		}
		methods
	}

	pub(crate) fn method_is_allowed(&self, method: &Method) -> bool {
		self.method_matchers.contains_key(method)
			|| (method == Method::HEAD && self.method_matchers.contains_key(&Method::GET))
	}

	pub async fn execute_route(
		&self,
		request: RawRequest,
		state: Arc<S>,
		exec_ctx: ExecCtx<E>,
		public_filemap: Arc<BTreeMap<String, String>>,
	) -> Result<Option<TaskRouteResult<E>>, Error> {
		let Some(route_match) = self.find_best(request.method(), request.path()) else {
			return Ok(Option::None);
		};
		let method_matcher = self
			.method_matchers
			.get(route_match.method())
			.ok_or_else(|| Error::RouteNotFound(route_match.original_pattern().to_owned()))?;
		let route = method_matcher
			.routes
			.get(route_match.original_pattern())
			.ok_or_else(|| Error::RouteNotFound(route_match.original_pattern().to_owned()))?;

		let middleware_entries = self.collect_middleware(route_match.original_pattern());
		let middleware_effects = self
			.run_middleware(
				&request,
				&route_match,
				state.clone(),
				exec_ctx.clone(),
				public_filemap.clone(),
				middleware_entries,
			)
			.await?;

		if middleware_effects.is_terminal_response() {
			return Ok(Some(TaskRouteResult {
				#[cfg(test)]
				route_match,
				data: Option::None,
				error: Option::None,
				response_effects: middleware_effects.clone(),
				middleware_effects,
			}));
		}

		let effects = Arc::new(Mutex::new(ResponseEffects::new()));
		let handler_execution = run_handler_and_collect_effects(
			route.handler.clone(),
			request,
			RequestBase {
				matched_pattern: route_match.original_pattern().to_owned(),
				params: route_match.params().clone(),
				splat_values: route_match.splat_values().to_vec(),
				state,
				exec_ctx,
				public_filemap,
				response_effects: effects,
			},
		)
		.await;
		let response_effects = merge_owned_response_effects(vec![
			middleware_effects.clone(),
			handler_execution.effects,
		]);
		Ok(Some(TaskRouteResult {
			#[cfg(test)]
			route_match,
			data: handler_execution.data,
			error: handler_execution.error,
			response_effects,
			middleware_effects,
		}))
	}

	#[cfg(test)]
	pub fn dynamic_param_prefix(&self) -> char {
		self.matcher_options.dynamic_param_prefix
	}

	#[cfg(test)]
	pub fn splat_segment_identifier(&self) -> char {
		self.matcher_options.splat_segment_identifier
	}

	pub(crate) fn mount_root(&self) -> &str {
		&self.mount_root
	}

	fn collect_middleware(&self, matched_pattern: &str) -> Vec<MiddlewareInvocation<S, E>> {
		let mut out = Vec::with_capacity(self.middlewares.len());
		for entry in &self.middlewares {
			out.push(MiddlewareInvocation::new(entry, matched_pattern));
		}
		out
	}

	async fn run_middleware(
		&self,
		request: &RawRequest,
		route_match: &RouteMatch,
		state: Arc<S>,
		exec_ctx: ExecCtx<E>,
		public_filemap: Arc<BTreeMap<String, String>>,
		middleware_entries: Vec<MiddlewareInvocation<S, E>>,
	) -> Result<ResponseEffects, Error> {
		run_middleware_entries(
			request,
			state,
			exec_ctx,
			public_filemap,
			route_match.params().clone(),
			route_match.splat_values().to_vec(),
			middleware_entries,
		)
		.await
	}

	fn get_or_create_method_matcher(
		&mut self,
		method: Method,
	) -> Result<&mut MethodMatcher<S, E>, Error> {
		if !self.method_matchers.contains_key(&method) {
			self.method_matchers.insert(
				method.clone(),
				MethodMatcher {
					matcher_builder: Matcher::builder(self.matcher_options.clone())
						.map_err(Error::InvalidMatcherOptions)?,
					matcher: Matcher::builder(self.matcher_options.clone())
						.map_err(Error::InvalidMatcherOptions)?
						.finish(),
					routes: BTreeMap::new(),
				},
			);
		}
		Ok(self
			.method_matchers
			.get_mut(&method)
			.expect("method matcher inserted"))
	}

	fn path_for_matching(&self, path: &str) -> Option<String> {
		if path == self.mount_root.trim_end_matches('/') {
			return Some("/".to_owned());
		}
		let stripped = path.strip_prefix(&self.mount_root)?;
		if stripped.is_empty() {
			return Some("/".to_owned());
		}
		Some(format!("/{stripped}"))
	}
}

impl<S, E> Default for Router<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn default() -> Self {
		Self::new(Options::default()).expect("default matcher options are valid")
	}
}

struct MethodMatcher<S, E> {
	matcher_builder: MatcherBuilder,
	matcher: Matcher,
	routes: BTreeMap<String, RouteEntry<S, E>>,
}

struct RouteEntry<S, E> {
	handler: Arc<dyn ErasedTask<S, E>>,
}

pub struct TaskRouteResult<E> {
	#[cfg(test)]
	route_match: RouteMatch,
	data: Option<Value>,
	error: Option<RouteExecutionError<E>>,
	response_effects: ResponseEffects,
	middleware_effects: ResponseEffects,
}

impl<E> TaskRouteResult<E> {
	#[cfg(test)]
	pub(crate) fn route_match(&self) -> &RouteMatch {
		&self.route_match
	}

	pub fn data(&self) -> Option<&Value> {
		self.data.as_ref()
	}

	pub fn error(&self) -> Option<&RouteExecutionError<E>> {
		self.error.as_ref()
	}

	pub fn response_effects(&self) -> &ResponseEffects {
		&self.response_effects
	}

	pub(crate) fn middleware_effects(&self) -> &ResponseEffects {
		&self.middleware_effects
	}
}
