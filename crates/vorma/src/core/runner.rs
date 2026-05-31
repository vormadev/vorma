use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use serde::Serialize;
use serde_json::Value;
use vorma_matcher::Params;
use vorma_tasks::Result as TaskResult;

use super::{ApiCtx, ViewCtx};
use crate::api;
use crate::mux::{self, None, RequestCtx, RouteExecutionError};

#[doc(hidden)]
pub type RouteFuture<O, E> = Pin<Box<dyn Future<Output = TaskResult<O, E>> + Send + 'static>>;
#[doc(hidden)]
pub type ErasedRouteFuture<E> =
	Pin<Box<dyn Future<Output = Result<Value, RouteExecutionError<E>>> + Send + 'static>>;
#[doc(hidden)]
pub type ErasedRequestCtx<S, E> = RequestCtx<S, E, None>;
#[doc(hidden)]
pub type ErasedRouteHandler<S, E> = fn(ErasedRequestCtx<S, E>) -> ErasedRouteFuture<E>;

type StaticViewHandler<S, E, I, P, O> = fn(ViewCtx<S, E, I, P>) -> RouteFuture<O, E>;
type StaticApiRouteHandler<S, E, I, P, O> = fn(ApiCtx<S, E, I, P>) -> RouteFuture<O, E>;
pub(super) type DynamicRouteHandler<S, E> =
	dyn Fn(ErasedRequestCtx<S, E>) -> ErasedRouteFuture<E> + Send + Sync;

pub(super) enum RouteRunner<S, E> {
	Static(ErasedRouteHandler<S, E>),
	Dynamic(Arc<DynamicRouteHandler<S, E>>),
}

impl<S, E> Clone for RouteRunner<S, E> {
	fn clone(&self) -> Self {
		match self {
			Self::Static(handler) => Self::Static(*handler),
			Self::Dynamic(handler) => Self::Dynamic(handler.clone()),
		}
	}
}

#[doc(hidden)]
pub fn run_static_view<S, E, I, P, O>(
	ctx: ErasedRequestCtx<S, E>,
	handler: StaticViewHandler<S, E, I, P, O>,
) -> ErasedRouteFuture<E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	I: api::ViewInput,
	P: Clone + PathParams,
	O: Serialize + Send + Sync + 'static,
{
	Box::pin(async move {
		let input =
			api::parse_loader_input::<I>(ctx.request()).map_err(RouteExecutionError::Input)?;
		let params = P::from_raw_path_params(ctx.params()).map_err(RouteExecutionError::Input)?;
		let handler_ctx = ViewCtx::new(ctx.with_input(input), params);
		let output = handler(handler_ctx)
			.await
			.map_err(RouteExecutionError::Task)?;
		serialize_route_output(output)
	})
}

#[doc(hidden)]
pub fn run_static_api_route<S, E, I, P, O>(
	ctx: ErasedRequestCtx<S, E>,
	handler: StaticApiRouteHandler<S, E, I, P, O>,
) -> ErasedRouteFuture<E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
	I: api::ApiInput,
	P: Clone + PathParams,
	O: Serialize + Send + Sync + 'static,
{
	Box::pin(async move {
		let input = api::parse_api_input::<I>(ctx.request())
			.await
			.map_err(RouteExecutionError::Input)?;
		let params = P::from_raw_path_params(ctx.params()).map_err(RouteExecutionError::Input)?;
		let handler_ctx = ApiCtx::new(ctx.with_input(input), params);
		let output = handler(handler_ctx)
			.await
			.map_err(RouteExecutionError::Task)?;
		serialize_route_output(output)
	})
}

pub(super) async fn run_route_runner<S, E>(
	loader: RouteRunner<S, E>,
	ctx: ErasedRequestCtx<S, E>,
) -> Result<Value, RouteExecutionError<E>>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	match loader {
		RouteRunner::Static(handler) => handler(ctx).await,
		RouteRunner::Dynamic(handler) => handler(ctx).await,
	}
}

#[doc(hidden)]
pub trait PathParams: Sized + Send + Sync + 'static {
	#[doc(hidden)]
	fn from_raw_path_params(params: &Params) -> Result<Self, mux::InputError>;
}

impl PathParams for () {
	fn from_raw_path_params(_: &Params) -> Result<Self, mux::InputError> {
		Ok(())
	}
}

pub(super) fn serialize_route_output<E>(
	output: impl Serialize,
) -> Result<Value, RouteExecutionError<E>> {
	serde_json::to_value(output).map_err(|err| {
		RouteExecutionError::Input(mux::InputError::internal(format!(
			"serialize route output: {err}"
		)))
	})
}
