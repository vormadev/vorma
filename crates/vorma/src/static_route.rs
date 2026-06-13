//! Static macro route adapters over the fresh execution engine.

use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use serde::Serialize;
use serde::de::DeserializeOwned;

use vorma_matcher::Params as MatcherParams;

/// Captured path parameters exposed to middleware.
#[derive(Clone, Copy, Debug)]
pub struct Params<'a> {
	inner: &'a MatcherParams,
}

impl<'a> Params<'a> {
	/// Return the value for a named parameter.
	pub fn get(&self, key: &str) -> Option<&'a str> {
		self.inner.get(key)
	}

	/// Iterate over parameter name/value pairs.
	pub fn iter(&self) -> impl Iterator<Item = (&'a str, &'a str)> {
		self.inner.iter()
	}

	/// Number of captured parameters.
	pub fn len(&self) -> usize {
		self.inner.len()
	}

	/// Whether no parameters were captured.
	pub fn is_empty(&self) -> bool {
		self.inner.is_empty()
	}
}

use crate::execution_engine::{
	HandlerExecutionError, HandlerFuture, HandlerInput, HandlerOutput, RuntimeHandler,
};
use crate::form_data::{DECODED_FORM_DATA_MISSING_MESSAGE, FormData};
use crate::route_input::{ResourceInput, ViewInput};
use crate::typed_handler::{TypedHandlerContext, handler_output_with_effects};
use vorma_tasks::ExecCtx;

/// Future returned by typed macro handlers.
#[doc(hidden)]
pub type RouteFuture<O, X> = Pin<Box<dyn Future<Output = Result<O, X>> + Send + 'static>>;

/// Future returned by erased macro route handlers.
#[doc(hidden)]
pub type ErasedRouteFuture =
	Pin<Box<dyn Future<Output = Result<HandlerOutput, StaticRouteError>> + Send + 'static>>;

/// Request context passed to erased macro route handlers before input type recovery.
#[doc(hidden)]
pub struct ErasedRequestCtx<S> {
	state: Arc<S>,
	input: HandlerInput,
	exec_ctx: ExecCtx<crate::Error>,
}

impl<S> ErasedRequestCtx<S> {
	fn new(state: Arc<S>, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> Self {
		Self {
			state,
			input,
			exec_ctx,
		}
	}

	fn into_typed_context<I>(self) -> Result<TypedHandlerContext<S, I>, HandlerExecutionError>
	where
		I: DeserializeOwned,
	{
		TypedHandlerContext::try_new(self.state, self.input, self.exec_ctx)
	}
}

/// Erased macro route handler.
#[doc(hidden)]
pub type ErasedRouteHandler<S> = fn(ErasedRequestCtx<S>) -> ErasedRouteFuture;

/// Typed parameter extraction contract implemented by generated route param structs.
#[doc(hidden)]
pub trait PathParams: Sized + Send + Sync + 'static {
	/// Recover typed route params from matcher params.
	fn from_raw_path_params(params: &MatcherParams) -> Result<Self, InputError>;
}

impl PathParams for () {
	fn from_raw_path_params(_: &MatcherParams) -> Result<Self, InputError> {
		Ok(())
	}
}

/// Macro route input error.
#[doc(hidden)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InputError {
	message: String,
}

impl InputError {
	/// Create a bad-request route input error.
	pub fn bad_request(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}
}

impl std::fmt::Display for InputError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		f.write_str(&self.message)
	}
}

impl std::error::Error for InputError {}

/// Context passed to middleware handlers.
pub struct MiddlewareCtx<S> {
	inner: TypedHandlerContext<S, ()>,
	exec_ctx: ExecCtx<crate::Error>,
}

impl<S> MiddlewareCtx<S> {
	pub(crate) fn new(inner: TypedHandlerContext<S, ()>, exec_ctx: ExecCtx<crate::Error>) -> Self {
		Self { inner, exec_ctx }
	}

	/// Route parameters for the matched view or resource.
	pub fn params(&self) -> Params<'_> {
		Params {
			inner: self.inner.handler_input().params(),
		}
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		self.inner.splat_values()
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Current HTTP request.
	pub fn request(&self) -> crate::HttpRequest<'_> {
		self.inner.request()
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<crate::Error> {
		&self.exec_ctx
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner
			.public_url(src_path)
			.map_err(|source| crate::Error::new(source.to_string()))
	}

	/// Mutable response-effect handle for headers and cookies.
	pub fn response(&self) -> crate::ResponseHandle<'_> {
		self.inner.response()
	}

	/// Exit the request with a redirect (default redirect status).
	pub fn redirect<O>(&self, location: impl AsRef<str>) -> Result<O, crate::HttpExit> {
		self.redirect_with_status(location, None)
	}

	/// Exit the request with a redirect using an explicit 3xx status.
	pub fn redirect_with_status<O>(
		&self,
		location: impl AsRef<str>,
		status: impl Into<Option<http::StatusCode>>,
	) -> Result<O, crate::HttpExit> {
		let location = location.as_ref();
		match self.inner.apply_redirect(location, status.into()) {
			Ok(()) => Err(crate::HttpExit::redirected(location)),
			Err(message) => Err(crate::HttpExit::err(format!("redirect failed: {message}"))),
		}
	}

	/// Mutable head-effect handle.
	pub fn head(&self) -> crate::HeadHandle {
		self.inner.head()
	}
}

/// Static route adapter error.
#[doc(hidden)]
#[derive(Debug)]
pub enum StaticRouteError {
	/// Typed input or output handling failed.
	Handler {
		/// Source handler execution error.
		source: HandlerExecutionError,
	},
	/// Generated path parameter extraction failed.
	Params {
		/// Source input error.
		source: InputError,
	},
	/// Application handler returned an error.
	Task {
		/// Source application task error.
		source: vorma_tasks::Error<crate::Error>,
	},
}

impl StaticRouteError {
	fn into_handler_execution_error(self) -> HandlerExecutionError {
		match self {
			Self::Handler { source } => source,
			Self::Params { source } => HandlerExecutionError::new(source.to_string()),
			Self::Task { source } => HandlerExecutionError::from_task_error(source),
		}
	}
}

/// Context passed to resource handlers declared by the public macros.
pub struct ResourceCtx<S, I, P = ()> {
	inner: TypedHandlerContext<S, I>,
	params: P,
	exec_ctx: ExecCtx<crate::Error>,
}

impl<S, I, P> ResourceCtx<S, I, P> {
	fn new(inner: TypedHandlerContext<S, I>, params: P, exec_ctx: ExecCtx<crate::Error>) -> Self {
		Self {
			inner,
			params,
			exec_ctx,
		}
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Parsed resource input.
	pub fn input(&self) -> &I {
		self.inner.input()
	}

	/// Typed route parameters for macro-generated parameter structs.
	pub fn params(&self) -> &P {
		&self.params
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		self.inner.splat_values()
	}

	/// Current HTTP request.
	pub fn request(&self) -> crate::HttpRequest<'_> {
		self.inner.request()
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<crate::Error> {
		&self.exec_ctx
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner
			.public_url(src_path)
			.map_err(|source| crate::Error::new(source.to_string()))
	}

	/// Mutable response-effect handle for success status, headers, and cookies.
	pub fn response(&self) -> crate::ResourceResponseHandle<'_> {
		self.inner.resource_response()
	}

	/// Exit this handler with a redirect (default redirect status).
	pub fn redirect<O>(&self, location: impl AsRef<str>) -> Result<O, crate::HttpExit> {
		self.redirect_with_status(location, None)
	}

	/// Exit this handler with a redirect using an explicit 3xx status.
	pub fn redirect_with_status<O>(
		&self,
		location: impl AsRef<str>,
		status: impl Into<Option<http::StatusCode>>,
	) -> Result<O, crate::HttpExit> {
		let location = location.as_ref();
		match self.inner.apply_redirect(location, status.into()) {
			Ok(()) => Err(crate::HttpExit::redirected(location)),
			Err(message) => Err(crate::HttpExit::err(format!("redirect failed: {message}"))),
		}
	}
}

/// Context passed to view handlers declared by the public macros.
pub struct ViewCtx<S, I, P = ()> {
	inner: TypedHandlerContext<S, I>,
	params: P,
	exec_ctx: ExecCtx<crate::Error>,
}

impl<S, I, P> ViewCtx<S, I, P> {
	fn new(inner: TypedHandlerContext<S, I>, params: P, exec_ctx: ExecCtx<crate::Error>) -> Self {
		Self {
			inner,
			params,
			exec_ctx,
		}
	}

	/// Shared application state.
	pub fn state(&self) -> &S {
		self.inner.state()
	}

	/// Parsed view input.
	pub fn input(&self) -> &I {
		self.inner.input()
	}

	/// Typed route parameters for macro-generated parameter structs.
	pub fn params(&self) -> &P {
		&self.params
	}

	/// String lookup for a named route parameter. Missing parameters return an empty string.
	pub fn param(&self, key: &str) -> &str {
		self.inner.param(key).unwrap_or("")
	}

	/// Captured splat values for splat patterns.
	pub fn splat_values(&self) -> &vorma_matcher::SplatValues {
		self.inner.splat_values()
	}

	/// Current HTTP request.
	pub fn request(&self) -> crate::HttpRequest<'_> {
		self.inner.request()
	}

	/// Per-request task execution context.
	pub fn exec_ctx(&self) -> &ExecCtx<crate::Error> {
		&self.exec_ctx
	}

	/// Resolve a public static source path through the runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		self.inner
			.public_url(src_path)
			.map_err(|source| crate::Error::new(source.to_string()))
	}

	/// Mutable response-effect handle for headers and cookies.
	pub fn response(&self) -> crate::ResponseHandle<'_> {
		self.inner.response()
	}

	/// Exit this handler with a redirect (default redirect status).
	pub fn redirect<O>(&self, location: impl AsRef<str>) -> Result<O, crate::ViewExit> {
		self.redirect_with_status(location, None)
	}

	/// Exit this handler with a redirect using an explicit 3xx status.
	pub fn redirect_with_status<O>(
		&self,
		location: impl AsRef<str>,
		status: impl Into<Option<http::StatusCode>>,
	) -> Result<O, crate::ViewExit> {
		let location = location.as_ref();
		match self.inner.apply_redirect(location, status.into()) {
			Ok(()) => Err(crate::ViewExit::redirected(location)),
			Err(message) => Err(crate::ViewExit::err(format!("redirect failed: {message}"))),
		}
	}

	/// Mutable head-effect handle for this view.
	pub fn head(&self) -> crate::HeadHandle {
		self.inner.head()
	}
}

type StaticViewHandler<S, I, P, O> = fn(ViewCtx<S, I, P>) -> RouteFuture<O, crate::ViewExit>;
type StaticResourceHandler<S, I, P, O> =
	fn(ResourceCtx<S, I, P>) -> RouteFuture<O, crate::HttpExit>;

/// Input recovery contract used by static macro resource handlers.
#[doc(hidden)]
pub trait StaticResourceInput: ResourceInput + Sized {
	/// Recover the concrete resource input type from decoded runtime input.
	fn into_typed_resource_context<S>(
		ctx: ErasedRequestCtx<S>,
	) -> Result<TypedHandlerContext<S, Self>, HandlerExecutionError>
	where
		S: Send + Sync + 'static;
}

impl<I> StaticResourceInput for I
where
	I: ResourceInput + DeserializeOwned,
{
	fn into_typed_resource_context<S>(
		ctx: ErasedRequestCtx<S>,
	) -> Result<TypedHandlerContext<S, Self>, HandlerExecutionError>
	where
		S: Send + Sync + 'static,
	{
		ctx.into_typed_context::<I>()
	}
}

impl StaticResourceInput for FormData {
	fn into_typed_resource_context<S>(
		ctx: ErasedRequestCtx<S>,
	) -> Result<TypedHandlerContext<S, Self>, HandlerExecutionError>
	where
		S: Send + Sync + 'static,
	{
		let form_data = ctx
			.input
			.decoded_input()
			.parsed_form_data()
			.cloned()
			.ok_or_else(|| HandlerExecutionError::new(DECODED_FORM_DATA_MISSING_MESSAGE))?;
		Ok(TypedHandlerContext::from_typed_input(
			ctx.state,
			ctx.input,
			form_data,
			ctx.exec_ctx,
		))
	}
}

/// Run a macro-generated static view handler.
#[doc(hidden)]
pub fn run_static_view<S, I, P, O>(
	ctx: ErasedRequestCtx<S>,
	handler: StaticViewHandler<S, I, P, O>,
) -> ErasedRouteFuture
where
	S: Send + Sync + 'static,
	I: ViewInput,
	P: Clone + PathParams,
	O: Serialize + Send + Sync + 'static,
{
	Box::pin(async move {
		let exec_ctx = ctx.exec_ctx.clone();
		let typed_context = ctx
			.into_typed_context::<I>()
			.map_err(|source| StaticRouteError::Handler { source })?;
		let params = P::from_raw_path_params(typed_context.handler_input().params())
			.map_err(|source| StaticRouteError::Params { source })?;
		let effects = typed_context.response_effects();
		let output = match handler(ViewCtx::new(typed_context, params, exec_ctx)).await {
			Ok(output) => output,
			Err(source) => {
				return Err(StaticRouteError::Handler {
					source: view_exit_with_effects(source, &effects),
				});
			}
		};
		handler_output_with_effects(output, &effects)
			.map_err(|source| StaticRouteError::Handler { source })
	})
}

/// Run a macro-generated static resource handler.
#[doc(hidden)]
pub fn run_static_resource<S, I, P, O>(
	ctx: ErasedRequestCtx<S>,
	handler: StaticResourceHandler<S, I, P, O>,
) -> ErasedRouteFuture
where
	S: Send + Sync + 'static,
	I: StaticResourceInput,
	P: Clone + PathParams,
	O: Serialize + Send + Sync + 'static,
{
	Box::pin(async move {
		let exec_ctx = ctx.exec_ctx.clone();
		let typed_context = I::into_typed_resource_context(ctx)
			.map_err(|source| StaticRouteError::Handler { source })?;
		let params = P::from_raw_path_params(typed_context.handler_input().params())
			.map_err(|source| StaticRouteError::Params { source })?;
		let effects = typed_context.response_effects();
		let output = match handler(ResourceCtx::new(typed_context, params, exec_ctx)).await {
			Ok(output) => output,
			Err(source) => {
				return Err(StaticRouteError::Handler {
					source: http_exit_with_effects(source, &effects),
				});
			}
		};
		handler_output_with_effects(output, &effects)
			.map_err(|source| StaticRouteError::Handler { source })
	})
}

/// Build a runtime handler from an erased static macro route handler.
#[doc(hidden)]
pub fn static_route_runtime_handler<S>(
	state: Arc<S>,
	handler: ErasedRouteHandler<S>,
) -> StaticRouteRuntimeHandler<S>
where
	S: Send + Sync + 'static,
{
	StaticRouteRuntimeHandler { state, handler }
}

/// Build a runtime handler from a public middleware context handler.
#[doc(hidden)]
pub fn middleware_runtime_handler<S, F, Fut, O>(
	state: Arc<S>,
	handler: F,
) -> MiddlewareRuntimeHandler<S, F>
where
	S: Send + Sync + 'static,
	F: Fn(MiddlewareCtx<S>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::HttpExit>> + Send + 'static,
	O: Send + Sync + 'static,
{
	MiddlewareRuntimeHandler { state, handler }
}

/// Runtime handler adapter for macro-generated static route handlers.
#[doc(hidden)]
pub struct StaticRouteRuntimeHandler<S> {
	state: Arc<S>,
	handler: ErasedRouteHandler<S>,
}

impl<S> RuntimeHandler for StaticRouteRuntimeHandler<S>
where
	S: Send + Sync + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let ctx = ErasedRequestCtx::new(Arc::clone(&self.state), input, exec_ctx);
		let future = (self.handler)(ctx);
		Box::pin(async move {
			future
				.await
				.map_err(StaticRouteError::into_handler_execution_error)
		})
	}
}

/// Runtime handler adapter for public middleware context handlers.
#[doc(hidden)]
pub struct MiddlewareRuntimeHandler<S, F> {
	state: Arc<S>,
	handler: F,
}

impl<S, F, Fut, O> RuntimeHandler for MiddlewareRuntimeHandler<S, F>
where
	S: Send + Sync + 'static,
	F: Fn(MiddlewareCtx<S>) -> Fut + Send + Sync + 'static,
	Fut: Future<Output = Result<O, crate::HttpExit>> + Send + 'static,
	O: Send + Sync + 'static,
{
	fn call(&self, input: HandlerInput, exec_ctx: ExecCtx<crate::Error>) -> HandlerFuture {
		let typed_context =
			match TypedHandlerContext::try_new(Arc::clone(&self.state), input, exec_ctx.clone()) {
				Ok(context) => context,
				Err(error) => return Box::pin(async { Err(error) }),
			};
		let effects = typed_context.response_effects();
		let future = (self.handler)(MiddlewareCtx::new(typed_context, exec_ctx));
		Box::pin(async move {
			if let Err(source) = future.await {
				return Err(http_exit_with_effects(source, &effects));
			}
			let effects = effects
				.lock()
				.expect("middleware response effects lock poisoned")
				.clone();
			Ok(HandlerOutput::empty().with_effects(effects))
		})
	}
}

/*
Redirect exits carry their outcome in the response effects (set at
ctx.redirect time, where request facts live); the engine's terminal-effects
arm handles them. Error exits carry their facts on the exit value itself.
*/
fn view_exit_with_effects(
	exit: crate::ViewExit,
	effects: &std::sync::Arc<std::sync::Mutex<crate::response_finalizer::ResponseEffects>>,
) -> HandlerExecutionError {
	let effects = effects
		.lock()
		.expect("static route response effects lock poisoned")
		.clone();
	if exit.is_redirect() {
		return HandlerExecutionError::new(exit.to_string()).with_effects(effects);
	}
	let error = match exit.client_msg() {
		Some(client_msg) => {
			HandlerExecutionError::with_client_message(exit.to_string(), client_msg)
		}
		None => HandlerExecutionError::new(exit.to_string()),
	};
	error.with_effects(effects)
}

fn http_exit_with_effects(
	exit: crate::HttpExit,
	effects: &std::sync::Arc<std::sync::Mutex<crate::response_finalizer::ResponseEffects>>,
) -> HandlerExecutionError {
	let effects = effects
		.lock()
		.expect("static route response effects lock poisoned")
		.clone();
	if exit.is_redirect() {
		return HandlerExecutionError::new(exit.to_string()).with_effects(effects);
	}
	let mut error = match exit.client_msg() {
		Some(client_msg) => {
			HandlerExecutionError::with_client_message(exit.to_string(), client_msg)
		}
		None => HandlerExecutionError::new(exit.to_string()),
	};
	if let Some(status) = exit.status() {
		error = error.with_status(status);
	}
	error.with_effects(effects)
}
