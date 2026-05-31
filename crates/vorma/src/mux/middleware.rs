use std::collections::BTreeMap;
use std::future::Future;
use std::marker::PhantomData;
use std::pin::Pin;
use std::sync::{Arc, Mutex};

use http::StatusCode;
use vorma_matcher::Params;
use vorma_tasks::{CancelToken, Error as TaskError, ExecCtx, Result as TaskResult};

use crate::response::{Proxy, merge_proxy_responses};

use super::context::{None, RequestCtx};
use super::error::Error;
use super::request::RawRequest;
use super::task::{proxy_for_task_output, run_with_exec_cancellation};

type TaskMiddlewareFuture<'a, E> = Pin<Box<dyn Future<Output = TaskResult<(), E>> + Send + 'a>>;

type TaskMiddlewareMarker<S, E, O> = fn() -> (S, E, O);

trait ErasedTaskMiddleware<S, E>: Send + Sync {
	fn run<'a>(&'a self, ctx: RequestCtx<S, E, None>) -> TaskMiddlewareFuture<'a, E>;
}

struct FnTaskMiddleware<S, E, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	handler: F,
	_marker: PhantomData<TaskMiddlewareMarker<S, E, O>>,
}

impl<S, E, F, Fut, O> ErasedTaskMiddleware<S, E> for FnTaskMiddleware<S, E, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, None>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
	O: Send + Sync + 'static,
{
	fn run<'a>(&'a self, ctx: RequestCtx<S, E, None>) -> TaskMiddlewareFuture<'a, E> {
		Box::pin(async move { (self.handler)(ctx).await.map(|_| ()) })
	}
}

pub(crate) struct TaskMw<S, E> {
	mw: Arc<dyn ErasedTaskMiddleware<S, E>>,
}

impl<S, E> Clone for TaskMw<S, E> {
	fn clone(&self) -> Self {
		Self {
			mw: self.mw.clone(),
		}
	}
}

impl<S, E> TaskMw<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub(crate) fn new<F, Fut, O>(handler: F) -> Self
	where
		F: Fn(RequestCtx<S, E, None>) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
		O: Send + Sync + 'static,
	{
		Self {
			mw: Arc::new(FnTaskMiddleware::<S, E, F, O> {
				handler,
				_marker: PhantomData,
			}),
		}
	}
}

pub(in crate::mux) struct TaskMiddlewareInvocation<S, E> {
	entry: TaskMw<S, E>,
	matched_pattern: String,
}

impl<S, E> TaskMiddlewareInvocation<S, E> {
	pub(in crate::mux) fn new(entry: &TaskMw<S, E>, matched_pattern: &str) -> Self {
		Self {
			entry: entry.clone(),
			matched_pattern: matched_pattern.to_owned(),
		}
	}
}

struct TaskMwOutput<E> {
	index: usize,
	proxy: Proxy,
	error: Option<TaskError<E>>,
}

impl<E> Clone for TaskMwOutput<E>
where
	TaskError<E>: Clone,
{
	fn clone(&self) -> Self {
		Self {
			index: self.index,
			proxy: self.proxy.clone(),
			error: self.error.clone(),
		}
	}
}

pub(in crate::mux) async fn run_task_middleware_entries<S, E>(
	request: &RawRequest,
	state: Arc<S>,
	exec_ctx: ExecCtx<E>,
	public_filemap: Arc<BTreeMap<String, String>>,
	params: Params,
	splat_values: Vec<String>,
	middleware_entries: Vec<TaskMiddlewareInvocation<S, E>>,
) -> Result<Proxy, Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	if middleware_entries.is_empty() {
		return Ok(Proxy::new());
	}
	let middleware_exec_ctxs = (0..middleware_entries.len())
		.map(|_| exec_ctx.child())
		.collect::<Vec<_>>();
	let cancel_middleware = Arc::new(
		middleware_exec_ctxs
			.iter()
			.map(|exec_ctx| exec_ctx.cancel_token().clone())
			.collect::<Vec<_>>(),
	);
	let mut handles = Vec::with_capacity(middleware_exec_ctxs.len());

	for (index, (invocation, exec_ctx)) in middleware_entries
		.into_iter()
		.zip(middleware_exec_ctxs)
		.enumerate()
	{
		let proxy = Arc::new(Mutex::new(Proxy::new()));
		let ctx = RequestCtx {
			matched_pattern: invocation.matched_pattern,
			params: params.clone(),
			splat_values: splat_values.clone(),
			state: state.clone(),
			exec_ctx: exec_ctx.clone(),
			public_filemap: public_filemap.clone(),
			response_proxy: proxy.clone(),
			request: request.clone(),
			input: None,
		};
		let middleware = invocation.entry.mw.clone();
		let cancel_middleware = cancel_middleware.clone();
		handles.push((
			index,
			tokio::spawn(async move {
				let ctx = ctx.clone_for_task(exec_ctx.clone(), None);
				run_with_exec_cancellation(&exec_ctx, async move {
					let output = middleware.run(ctx).await;
					let proxy = proxy.lock().expect("response proxy lock poisoned").clone();
					let should_cancel_later = output.is_err() || proxy.is_terminal_response();
					if should_cancel_later {
						cancel_later(&cancel_middleware, index);
					}
					match output {
						Ok(()) => TaskMwOutput {
							index,
							proxy,
							error: Option::None,
						},
						Err(error) => TaskMwOutput {
							index,
							proxy: proxy_for_task_output(true, proxy),
							error: Some(error),
						},
					}
				})
				.await
			}),
		));
	}

	let mut outputs = Vec::with_capacity(handles.len());
	let mut errors = Vec::new();
	for (index, handle) in handles {
		let output = handle
			.await
			.map_err(|error| Error::TaskJoin(error.to_string()))?;
		match output {
			Ok(output) => {
				if let Some(error) = output.error.clone() {
					cancel_later(&cancel_middleware, output.index);
					errors.push((output.index, error));
				}
				outputs.push(output);
			}
			Err(error) => {
				cancel_later(&cancel_middleware, index);
				errors.push((index, error));
			}
		}
	}

	outputs.sort_by_key(|output| output.index);
	if let Some(first_terminal_index) = first_terminal_middleware_index(&outputs, &errors) {
		let eligible_proxies = outputs
			.iter()
			.filter(|output| output.index <= first_terminal_index)
			.map(|output| output.proxy.clone())
			.collect::<Vec<_>>();
		let mut merged_terminal_proxy = merge_owned_proxy_responses(eligible_proxies);
		if merged_terminal_proxy.is_terminal_response() {
			return Ok(merged_terminal_proxy);
		}
		set_internal_server_error(&mut merged_terminal_proxy);
		return Ok(merged_terminal_proxy);
	}
	Ok(merge_owned_proxy_responses(
		outputs.into_iter().map(|output| output.proxy).collect(),
	))
}

fn first_terminal_middleware_index<E>(
	outputs: &[TaskMwOutput<E>],
	errors: &[(usize, TaskError<E>)],
) -> Option<usize> {
	let first_proxy_index = outputs
		.iter()
		.find(|output| output.proxy.is_terminal_response())
		.map(|output| output.index);
	let first_error_index = errors
		.iter()
		.filter(|(_, error)| !error.is_cancelled())
		.map(|(index, _)| *index)
		.min();

	match (first_proxy_index, first_error_index) {
		(Some(proxy_index), Some(error_index)) => Some(proxy_index.min(error_index)),
		(Some(proxy_index), Option::None) => Some(proxy_index),
		(Option::None, Some(error_index)) => Some(error_index),
		(Option::None, Option::None) => errors.iter().map(|(index, _)| *index).min(),
	}
}

fn cancel_later(tokens: &[CancelToken], own_index: usize) {
	for (index, token) in tokens.iter().enumerate() {
		if index > own_index {
			token.cancel();
		}
	}
}

fn set_internal_server_error(proxy: &mut Proxy) {
	proxy.set_status(
		StatusCode::INTERNAL_SERVER_ERROR,
		Some("Internal Server Error".to_owned()),
	);
}

pub(in crate::mux) fn merge_owned_proxy_responses(proxies: Vec<Proxy>) -> Proxy {
	let refs = proxies.iter().map(Some).collect::<Vec<_>>();
	merge_proxy_responses(&refs)
}
