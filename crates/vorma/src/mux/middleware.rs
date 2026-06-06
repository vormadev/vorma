use std::collections::BTreeMap;
use std::future::Future;
use std::marker::PhantomData;
use std::pin::Pin;
use std::sync::Arc;

use http::StatusCode;
use vorma_matcher::Params;
use vorma_tasks::{Error as TaskError, ExecCtx, Result as TaskResult};

use crate::response::{ResponseEffects, ResponseEffectsCollector, merge_response_effects};

use super::context::{None, RequestCtx};
use super::error::Error;
use super::ordered_parallel::{OrderedTaskContexts, run_ordered_parallel};
use super::request::RawRequest;
use super::task::{response_effects_for_task_output, run_with_exec_cancellation};

type MiddlewareFuture<'a, E> = Pin<Box<dyn Future<Output = TaskResult<(), E>> + Send + 'a>>;

type MiddlewareMarker<S, E, O> = fn() -> (S, E, O);

trait ErasedMiddleware<S, E>: Send + Sync {
	fn run<'a>(&'a self, ctx: RequestCtx<S, E, None>) -> MiddlewareFuture<'a, E>;
}

struct FnMiddleware<S, E, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	handler: F,
	_marker: PhantomData<MiddlewareMarker<S, E, O>>,
}

impl<S, E, F, Fut, O> ErasedMiddleware<S, E> for FnMiddleware<S, E, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, None>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = TaskResult<O, E>> + Send + 'static,
	O: Send + Sync + 'static,
{
	fn run<'a>(&'a self, ctx: RequestCtx<S, E, None>) -> MiddlewareFuture<'a, E> {
		Box::pin(async move { (self.handler)(ctx).await.map(|_| ()) })
	}
}

pub(crate) struct Middleware<S, E> {
	mw: Arc<dyn ErasedMiddleware<S, E>>,
}

impl<S, E> Clone for Middleware<S, E> {
	fn clone(&self) -> Self {
		Self {
			mw: self.mw.clone(),
		}
	}
}

impl<S, E> Middleware<S, E>
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
			mw: Arc::new(FnMiddleware::<S, E, F, O> {
				handler,
				_marker: PhantomData,
			}),
		}
	}
}

pub(in crate::mux) struct MiddlewareInvocation<S, E> {
	entry: Middleware<S, E>,
	matched_pattern: String,
}

impl<S, E> MiddlewareInvocation<S, E> {
	pub(in crate::mux) fn new(entry: &Middleware<S, E>, matched_pattern: &str) -> Self {
		Self {
			entry: entry.clone(),
			matched_pattern: matched_pattern.to_owned(),
		}
	}
}

struct MiddlewareOutput<E> {
	index: usize,
	effects: ResponseEffects,
	error: Option<TaskError<E>>,
}

impl<E> Clone for MiddlewareOutput<E>
where
	TaskError<E>: Clone,
{
	fn clone(&self) -> Self {
		Self {
			index: self.index,
			effects: self.effects.clone(),
			error: self.error.clone(),
		}
	}
}

pub(in crate::mux) async fn run_middleware_entries<S, E>(
	request: &RawRequest,
	state: Arc<S>,
	exec_ctx: ExecCtx<E>,
	public_filemap: Arc<BTreeMap<String, String>>,
	params: Params,
	splat_values: Vec<String>,
	middleware_entries: Vec<MiddlewareInvocation<S, E>>,
) -> Result<ResponseEffects, Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	if middleware_entries.is_empty() {
		return Ok(ResponseEffects::new());
	}
	let task_contexts = OrderedTaskContexts::sibling_children(&exec_ctx, middleware_entries.len());
	let cancellation = task_contexts.cancellation();
	let request = request.clone();
	let run_outputs = run_ordered_parallel(middleware_entries, task_contexts, {
		let state = state.clone();
		let public_filemap = public_filemap.clone();
		let params = params.clone();
		let splat_values = splat_values.clone();
		move |invocation, task_ctx| {
			let state = state.clone();
			let public_filemap = public_filemap.clone();
			let params = params.clone();
			let splat_values = splat_values.clone();
			let request = request.clone();
			async move {
				let index = task_ctx.index();
				let exec_ctx = task_ctx.exec_ctx();
				let effects = ResponseEffectsCollector::new();
				let ctx = RequestCtx {
					matched_pattern: invocation.matched_pattern,
					params,
					splat_values,
					state,
					exec_ctx: exec_ctx.clone(),
					public_filemap,
					response_effects: effects.clone(),
					request,
					input: None,
				};
				let middleware = invocation.entry.mw.clone();
				let ctx = ctx.clone_for_task(exec_ctx.clone(), None);
				run_with_exec_cancellation(&exec_ctx, async move {
					let output = middleware.run(ctx).await;
					let effects = effects.snapshot();
					let should_cancel_later = output.is_err() || effects.is_terminal_response();
					if should_cancel_later {
						task_ctx.cancel_later();
					}
					match output {
						Ok(()) => MiddlewareOutput {
							index,
							effects,
							error: Option::None,
						},
						Err(error) => MiddlewareOutput {
							index,
							effects: response_effects_for_task_output(true, effects),
							error: Some(error),
						},
					}
				})
				.await
			}
		}
	})
	.await?;

	let run_outputs = run_outputs.into_vec();
	let mut outputs = Vec::with_capacity(run_outputs.len());
	let mut errors = Vec::new();
	for (index, output) in run_outputs {
		match output {
			Ok(output) => {
				if let Some(error) = output.error.clone() {
					cancellation.cancel_later(output.index);
					errors.push((output.index, error));
				}
				outputs.push(output);
			}
			Err(error) => {
				cancellation.cancel_later(index);
				errors.push((index, error));
			}
		}
	}

	outputs.sort_by_key(|output| output.index);
	if let Some(first_terminal_index) = first_terminal_middleware_index(&outputs, &errors) {
		let eligible_effects = outputs
			.iter()
			.filter(|output| output.index <= first_terminal_index)
			.map(|output| output.effects.clone())
			.collect::<Vec<_>>();
		let mut merged_terminal_effects = merge_owned_response_effects(eligible_effects);
		if merged_terminal_effects.is_terminal_response() {
			return Ok(merged_terminal_effects);
		}
		set_internal_server_error(&mut merged_terminal_effects);
		return Ok(merged_terminal_effects);
	}
	Ok(merge_owned_response_effects(
		outputs.into_iter().map(|output| output.effects).collect(),
	))
}

fn first_terminal_middleware_index<E>(
	outputs: &[MiddlewareOutput<E>],
	errors: &[(usize, TaskError<E>)],
) -> Option<usize> {
	let first_terminal_effects_index = outputs
		.iter()
		.find(|output| output.effects.is_terminal_response())
		.map(|output| output.index);
	let first_error_index = errors
		.iter()
		.filter(|(_, error)| !error.is_cancelled())
		.map(|(index, _)| *index)
		.min();

	match (first_terminal_effects_index, first_error_index) {
		(Some(effects_index), Some(error_index)) => Some(effects_index.min(error_index)),
		(Some(effects_index), Option::None) => Some(effects_index),
		(Option::None, Some(error_index)) => Some(error_index),
		(Option::None, Option::None) => errors.iter().map(|(index, _)| *index).min(),
	}
}

fn set_internal_server_error(effects: &mut ResponseEffects) {
	effects.set_status(
		StatusCode::INTERNAL_SERVER_ERROR,
		Some("Internal Server Error".to_owned()),
	);
}

pub(in crate::mux) fn merge_owned_response_effects(
	effects_list: Vec<ResponseEffects>,
) -> ResponseEffects {
	let refs = effects_list.iter().map(Some).collect::<Vec<_>>();
	merge_response_effects(&refs)
}
