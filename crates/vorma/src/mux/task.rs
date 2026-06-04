use std::future::Future;
use std::marker::PhantomData;
use std::pin::Pin;
use std::sync::Arc;

use http::StatusCode;
#[cfg(test)]
use serde::Serialize;
use serde_json::Value;
use vorma_tasks::{Error as TaskError, ExecCtx};

use super::context::{RequestBase, RequestCtx};
#[cfg(test)]
use super::error::InputError;
use super::error::RouteExecutionError;
#[cfg(test)]
use super::input::InputParser;
use super::request::RawRequest;
use crate::response::ResponseEffects;

type RouteTaskFuture<'a, E> =
	Pin<Box<dyn Future<Output = Result<Value, RouteExecutionError<E>>> + Send + 'a>>;

#[cfg(test)]
type TaskMarker<S, E, I, O> = fn() -> (S, E, I, O);
type ErasedTaskMarker<S, E, F> = fn() -> (S, E, F);

pub(crate) trait ErasedTask<S, E>: Send + Sync {
	fn run<'a>(&'a self, request: RawRequest, base: RequestBase<S, E>) -> RouteTaskFuture<'a, E>;
}

#[cfg(test)]
pub(in crate::mux) fn typed_handler<S, E, I, F, Fut, O>(
	parser: InputParser<I>,
	handler: F,
) -> Arc<dyn ErasedTask<S, E>>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	I: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, RouteExecutionError<E>>> + Send + 'static,
	O: Serialize + Send + Sync + 'static,
{
	Arc::new(FnTask::<S, E, I, F, O>::new(parser, handler))
}

pub(crate) fn erased_handler<S, E, F, Fut>(handler: F) -> Arc<dyn ErasedTask<S, E>>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, super::None>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<Value, RouteExecutionError<E>>> + Send + 'static,
{
	Arc::new(FnErasedTask::<S, E, F> {
		handler,
		_marker: PhantomData,
	})
}

struct ErasedTaskRun<E> {
	pub(in crate::mux) output: Result<Value, RouteExecutionError<E>>,
	pub(in crate::mux) effects: ResponseEffects,
}

impl<E> ErasedTaskRun<E> {
	fn into_parts(
		self,
	) -> (
		Option<Value>,
		Option<RouteExecutionError<E>>,
		ResponseEffects,
	) {
		let (data, error) = match self.output {
			Ok(data) => (Some(data), Option::None),
			Err(error) => (Option::None, Some(error)),
		};
		(data, error, self.effects)
	}
}

pub(in crate::mux) struct HandlerExecution<E> {
	pub(in crate::mux) data: Option<Value>,
	pub(in crate::mux) error: Option<RouteExecutionError<E>>,
	pub(in crate::mux) effects: ResponseEffects,
}

pub(in crate::mux) async fn run_handler_and_collect_effects<S, E>(
	handler: Arc<dyn ErasedTask<S, E>>,
	request: RawRequest,
	base: RequestBase<S, E>,
) -> HandlerExecution<E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let response_effects = base.response_effects.clone();
	let output = handler.run(request, base).await;
	let mut effects = response_effects
		.lock()
		.expect("response effects lock poisoned")
		.clone();
	record_bad_request_input_error(&mut effects, &output);
	let (data, error, effects) = ErasedTaskRun { output, effects }.into_parts();
	let effects = response_effects_for_task_output(error.is_some(), effects);
	HandlerExecution {
		data,
		error,
		effects,
	}
}

pub(in crate::mux) async fn run_with_exec_cancellation<E, F, T>(
	exec_ctx: &ExecCtx<E>,
	future: F,
) -> Result<T, TaskError<E>>
where
	E: Send + Sync + 'static,
	F: Future<Output = T>,
{
	if exec_ctx.is_cancelled() {
		return Err(TaskError::Cancelled);
	}

	let output = tokio::select! {
		output = future => output,
		_ = exec_ctx.cancel_token().cancelled() => {
			return Err(TaskError::Cancelled);
		}
	};

	if exec_ctx.is_cancelled() {
		return Err(TaskError::Cancelled);
	}

	Ok(output)
}

fn record_bad_request_input_error<E>(
	effects: &mut ResponseEffects,
	output: &Result<Value, RouteExecutionError<E>>,
) {
	if let Err(RouteExecutionError::Input(error)) = output
		&& error.is_bad_request()
		&& !effects.is_terminal_response()
	{
		effects.set_status(StatusCode::BAD_REQUEST, Some(error.to_string()));
	}
}

pub(in crate::mux) fn response_effects_for_task_output(
	has_error: bool,
	effects: ResponseEffects,
) -> ResponseEffects {
	if has_error && !effects.is_terminal_response() {
		return ResponseEffects::new();
	}
	effects
}

#[cfg(test)]
pub(in crate::mux) struct FnTask<S, E, I, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	parser: InputParser<I>,
	handler: F,
	_marker: PhantomData<TaskMarker<S, E, I, O>>,
}

#[cfg(test)]
impl<S, E, I, F, O> FnTask<S, E, I, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub(in crate::mux) fn new(parser: InputParser<I>, handler: F) -> Self {
		Self {
			parser,
			handler,
			_marker: PhantomData,
		}
	}
}

struct FnErasedTask<S, E, F>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	handler: F,
	_marker: PhantomData<ErasedTaskMarker<S, E, F>>,
}

impl<S, E, F, Fut> ErasedTask<S, E> for FnErasedTask<S, E, F>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, super::None>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<Value, RouteExecutionError<E>>> + Send + 'static,
{
	fn run<'a>(&'a self, request: RawRequest, base: RequestBase<S, E>) -> RouteTaskFuture<'a, E> {
		Box::pin(async move {
			(self.handler)(RequestCtx {
				matched_pattern: base.matched_pattern,
				params: base.params,
				splat_values: base.splat_values,
				state: base.state,
				exec_ctx: base.exec_ctx,
				public_filemap: base.public_filemap,
				response_effects: base.response_effects,
				request,
				input: super::None,
			})
			.await
		})
	}
}

#[cfg(test)]
impl<S, E, I, F, Fut, O> ErasedTask<S, E> for FnTask<S, E, I, F, O>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	I: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, RouteExecutionError<E>>> + Send + 'static,
	O: Serialize + Send + Sync + 'static,
{
	fn run<'a>(&'a self, request: RawRequest, base: RequestBase<S, E>) -> RouteTaskFuture<'a, E> {
		Box::pin(run_typed_handler(
			&self.parser,
			&self.handler,
			request,
			base,
		))
	}
}

#[cfg(test)]
async fn run_typed_handler<S, E, I, F, Fut, O>(
	parser: &InputParser<I>,
	handler: &F,
	request: RawRequest,
	base: RequestBase<S, E>,
) -> Result<Value, RouteExecutionError<E>>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	I: Send + Sync + 'static,
	F: Fn(RequestCtx<S, E, I>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, RouteExecutionError<E>>> + Send + 'static,
	O: Serialize + Send + Sync + 'static,
{
	let input = match parser.parse(&request).await {
		Ok(input) => input,
		Err(error) => return Err(RouteExecutionError::Input(error)),
	};
	let ctx = RequestCtx {
		matched_pattern: base.matched_pattern,
		params: base.params,
		splat_values: base.splat_values,
		state: base.state,
		exec_ctx: base.exec_ctx,
		public_filemap: base.public_filemap,
		response_effects: base.response_effects,
		request,
		input,
	};
	match handler(ctx).await {
		Ok(output) => serde_json::to_value(output).map_err(|err| {
			RouteExecutionError::Input(InputError::internal(format!(
				"serialize route output: {err}"
			)))
		}),
		Err(error) => Err(error),
	}
}
