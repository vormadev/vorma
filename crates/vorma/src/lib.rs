//! Rust full-stack framework runtime and app declaration API.
//!
//! Application code declares views, resources, middleware, document defaults, and
//! build/runtime config through this crate. `vorma-build` consumes the same declarations
//! to generate manifests and TypeScript contracts, while [`RuntimeHost`] serves the
//! runtime handler that users mount into their HTTP stack.

#![deny(missing_docs)]
#![forbid(unsafe_code)]

// Vorma apps run mimalloc by default (any crate in a binary's graph may
// declare the global allocator). Opting out: default-features = false.
#[cfg(feature = "mimalloc")]
#[global_allocator]
static GLOBAL_ALLOCATOR: mimalloc::MiMalloc = mimalloc::MiMalloc;

extern crate self as vorma;

mod app_assembly;
mod asset_body_provider;
mod asset_capabilities;
mod config;
mod document_builder;
mod envutil;
mod error;
mod execution_engine;
mod exit;
mod facade;
mod form_data;
mod handler_context;
mod head;
mod input_decoder;
pub mod kit;
mod live_state_emit;
/// Optional Tower middleware helpers for Vorma app servers.
pub mod middleware;
mod network;
mod payload_projection;
mod public_app;
mod request;
mod resource_response;
mod response_finalizer;
mod route_input;
mod runtime_app;
mod runtime_assets;
mod runtime_document;
mod runtime_host;
mod runtime_service;
mod runtime_snapshot;
mod search_params;
mod static_route;
#[cfg(test)]
mod test_support;
/// In-memory test harness for booting apps without build artifacts.
pub mod testing;
mod typed_handler;
mod view_response;

pub(crate) use vorma_contract::{
	constants, contracts, document_renderer, execution_plan, framework_graph, runtime_manifest,
};

/// Public TypeScript type model used by [`TsGen`] and `AppConfig` type exports.
pub use vorma_contract::tsgen;

/// Public framework configuration.
pub use config::{DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig, UiVariant};
/// Public HTTP/cookie convenience reexports.
pub use cookie::Cookie as HttpCookie;
/// Public document shell and runtime document builder API.
pub use document_builder::{
	Document, DocumentAttributes, DocumentBuildContext as DocumentBuildCtx, DocumentBuilder,
};
/// Public framework error types.
pub use error::{BoxError, Error};
/// Public handler early-exit types.
pub use exit::{HttpExit, ViewExit};
/// Public form data input types.
pub use form_data::{FormData, FormField, FormFile};
/// Public resource route classification.
pub use framework_graph::ResourceKind;
/// Public low-level head/document element helpers.
pub use head::{
	HeadAttr, HeadBooleanAttribute, HeadBuilder, HeadInnerHtml, HeadSelfClosing, HeadTag,
	HeadTextContent, HtmlElementDef,
};
/// Public app declaration API.
pub use public_app::{App, AppConfig, Middleware, Middlewares, Resource, Resources, View, Views};
/// Public read-only HTTP request wrappers.
pub use request::{HttpRequest, HttpSearchParams};
/// Public runtime host service.
pub use runtime_host::RuntimeHost;
/// Response header carrying the expected Vorma client build ID.
pub const CLIENT_BUILD_ID_HEADER_KEY: &str = response_finalizer::CLIENT_BUILD_ID_HEADER;
/// Prefix added to hashed public static asset output filenames.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = constants::PUBLIC_STATIC_OUT_NAME_PREFIX;
/// Default maximum request body size collected by Vorma handlers.
pub const DEFAULT_REQUEST_BODY_LIMIT: usize = app_assembly::DEFAULT_REQUEST_BODY_LIMIT;
/// Public HTTP type aliases.
pub use http::{
	HeaderMap as HttpHeaderMap, HeaderName as HttpHeaderName, HeaderValue as HttpHeaderValue,
	Method as HttpMethod, StatusCode as HttpStatusCode,
};
#[doc(hidden)]
pub use route_input::{ResourceInput, ViewInput};
/// Public route handler context types used by app declarations.
pub use static_route::{MiddlewareCtx, Params, ResourceCtx, ViewCtx};
/// Public typed handler context handles.
pub use typed_handler::{
	TypedHeadHandle as HeadHandle, TypedResourceResponseHandle as ResourceResponseHandle,
	TypedResponseHandle as ResponseHandle,
};

/// Result type returned by Vorma public helpers.
pub type Result<T> = std::result::Result<T, Error>;

/// Whether the current process is running as a Vorma build/live-state entry.
pub fn is_build() -> bool {
	envutil::is_build()
}

/// Whether the current process is running an app server under Vorma dev mode.
pub fn is_dev() -> bool {
	envutil::is_dev()
}

/// Read `PORT` and return `0.0.0.0:<PORT>` for deployment-style app servers.
pub fn bind_addr() -> Result<std::net::SocketAddr> {
	network::bind_addr()
}
/// Public TypeScript type derive macro.
pub use vorma_macros::TsGen;
#[doc(hidden)]
pub use vorma_macros::{__vorma_resource, __vorma_view};
/// Public task runtime API reexports.
pub use vorma_tasks::{
	CancelToken, Clock as TaskClock, ClockInstant as TaskClockInstant, Error as TaskError, ExecCtx,
	PreparedTask, Result as TaskResult, SystemClock as SystemTaskClock, Task, TaskEvent,
	TaskEventKind, TaskEventOutcome, TaskId, TaskObserver, TaskOverrideMode, TaskOverrides,
	TaskRunSource, Tasks, TasksOptions,
};

/// Helpers for constructing low-level HTML attributes in head/document builders.
pub struct HtmlAttribute;

impl HtmlAttribute {
	/// Create a normal escaped HTML attribute definition.
	pub fn attr(name: impl Into<String>, value: impl Into<String>) -> HtmlElementDef {
		head::HeadAttr::new(name, value).into()
	}

	/// Create a `type="..."` attribute definition.
	pub fn r#type(value: impl Into<String>) -> HtmlElementDef {
		Self::attr("type", value)
	}
}

/// Helpers for explicitly trusted HTML fragments.
pub struct SafeHtml;

impl SafeHtml {
	/// Create trusted style contents for a `<style>` element.
	pub fn style_content(content: impl Into<String>) -> HtmlElementDef {
		head::HeadInnerHtml(content.into()).into()
	}

	/// Create trusted script contents for a `<script>` element.
	pub fn script_content(content: impl Into<String>) -> HtmlElementDef {
		head::HeadInnerHtml(content.into()).into()
	}
}

/// Declare a typed local module of Vorma app aliases and declaration macros.
#[macro_export]
macro_rules! app {
	($vis:vis mod $module:ident for $state:ty) => {
		$vis mod $module {
			#[allow(dead_code)]
			pub type State = $state;
			#[allow(dead_code)]
			pub type App = $crate::App<State>;
			#[allow(dead_code)]
			pub type Resource = $crate::Resource<State>;
			#[allow(dead_code)]
			pub type Resources = $crate::Resources<State>;
			#[allow(dead_code)]
			pub type ResourceCtx<I = (), P = ()> =
				$crate::ResourceCtx<State, I, P>;
			#[allow(dead_code)]
			pub type DocumentBuildCtx = $crate::DocumentBuildCtx;
			#[allow(dead_code)]
			pub type DocumentBuilder = $crate::DocumentBuilder;
			#[allow(dead_code)]
			pub type MiddlewareCtx = $crate::MiddlewareCtx<State>;
			#[allow(dead_code)]
			pub type Middleware = $crate::Middleware<State>;
			#[allow(dead_code)]
			pub type Middlewares = $crate::Middlewares<State>;
			#[allow(dead_code)]
			pub type View = $crate::View<State>;
			#[allow(dead_code)]
			pub type ViewCtx<I = (), P = ()> =
				$crate::ViewCtx<State, I, P>;
			#[allow(dead_code)]
			pub type Views = $crate::Views<State>;

			#[allow(unused_imports)]
			pub use $crate::{
				__vorma_middlewares as middlewares, __vorma_resource as resource,
				__vorma_resources as resources, __vorma_view as view, __vorma_views as views,
			};
		}
	};
}

#[doc(hidden)]
#[macro_export]
macro_rules! __vorma_views {
	() => {
		$crate::Views::new()
	};
	($($view:expr),+ $(,)?) => {{
		let mut views = $crate::Views::new();
		$(
			views.push($view);
		)*
		views
	}};
}

#[doc(hidden)]
#[macro_export]
macro_rules! __vorma_resources {
	() => {
		$crate::Resources::new()
	};
	($($resource:expr),+ $(,)?) => {{
		let mut resources = $crate::Resources::new();
		$(
			resources.push($resource);
		)*
		resources
	}};
}

#[doc(hidden)]
#[macro_export]
macro_rules! __vorma_middlewares {
	() => {
		$crate::Middlewares::new()
	};
	($($middleware:expr),+ $(,)?) => {{
		let mut middlewares = $crate::Middlewares::new();
		$(
			middlewares.push($middleware);
		)*
		middlewares
	}};
}

#[cfg(test)]
mod public_macro_tests {
	use serde::{Deserialize, Serialize};

	#[derive(Default)]
	struct MacroState {
		prefix: String,
	}

	crate::app!(mod macro_app for crate::public_macro_tests::MacroState);

	#[derive(Clone, Debug, Deserialize, crate::TsGen)]
	struct MacroInput {
		name: String,
	}

	#[derive(Clone, Debug, PartialEq, Serialize, crate::TsGen)]
	struct MacroOutput {
		message: String,
	}

	const MACRO_VIEW: macro_app::View = macro_app::view! {
		client_file: "src/client/views/macro.view.tsx";
		pattern: "/macro/:id";
		input: MacroInput;
		output: MacroOutput;

		handler: |ctx| {
			ctx.head().title("Macro View");
			let _ = ctx.exec_ctx().is_cancelled();
			Ok(MacroOutput {
				message: format!(
					"{}{}:{}",
					ctx.state().prefix,
					ctx.param("id"),
					ctx.input().name
				),
			})
		};
	};

	const MACRO_RESOURCE: macro_app::Resource = macro_app::resource! {
		kind: crate::ResourceKind::Mutation;
		method: crate::HttpMethod::POST;
		pattern: "/macro/:id";
		input: MacroInput;
		output: MacroOutput;

		handler: |ctx| {
			ctx.response().set_status(crate::HttpStatusCode::CREATED);
			let _ = ctx.exec_ctx().is_cancelled();
			Ok(MacroOutput {
				message: format!(
					"{}{}:{}",
					ctx.state().prefix,
					ctx.params().id,
					ctx.input().name
				),
			})
		};
	};

	const MACRO_FORM_DATA_RESOURCE: macro_app::Resource = macro_app::resource! {
		kind: crate::ResourceKind::Mutation;
		method: crate::HttpMethod::POST;
		pattern: "/macro-form/:id";
		input: crate::FormData;
		output: MacroOutput;

		handler: |ctx| {
			ctx.response().set_status(crate::HttpStatusCode::CREATED);
			Ok(MacroOutput {
				message: format!(
					"{}{}:{}",
					ctx.state().prefix,
					ctx.params().id,
					ctx.input().text("name").unwrap_or("")
				),
			})
		};
	};

	#[test]
	fn public_declaration_macros_lower_into_fresh_app_assembly() {
		let app = crate::App::from_app_config(crate::AppConfig {
			state: MacroState {
				prefix: "macro-".to_owned(),
			},
			views: macro_app::views![MACRO_VIEW],
			resources: macro_app::resources![MACRO_RESOURCE, MACRO_FORM_DATA_RESOURCE],
			middlewares: macro_app::middlewares![macro_app::Middleware::new(|ctx| async move {
				let _ = ctx.request().path();
				let _ = ctx.exec_ctx().is_cancelled();
				Ok::<(), crate::HttpExit>(())
			})],
			..crate::AppConfig::default()
		})
		.unwrap();
		let assembly = app.into_assembly();
		let declarations = assembly.facade().declarations();

		assert_eq!(declarations.views().len(), 1);
		assert_eq!(declarations.resources().len(), 2);
		assert_eq!(declarations.middlewares().len(), 1);
	}

	#[test]
	fn hidden_build_graph_helper_compiles_public_app_config() {
		let graph = crate::build_interface::app_build_graph(crate::AppConfig {
			state: MacroState {
				prefix: "build-".to_owned(),
			},
			views: macro_app::views![MACRO_VIEW],
			resources: macro_app::resources![MACRO_RESOURCE, MACRO_FORM_DATA_RESOURCE],
			middlewares: macro_app::middlewares![],
			..crate::AppConfig::default()
		})
		.unwrap();

		assert_eq!(graph.views().len(), 1);
		assert_eq!(graph.resources().len(), 2);
	}
}

/// Hidden internal contract consumed by `vorma-build`.
#[doc(hidden)]
/*
The named contract between paired `vorma`/`vorma-build` versions: real,
documented surface, but not for application code — apps declare through the
public API and never need these. The two crates ship version-locked, so this
interface carries no cross-version stability promise.
*/
/// Build-facing runtime interface consumed by `vorma-build`.
pub mod build_interface {
	/// Runtime asset capability contracts.
	pub mod assets {
		pub use crate::asset_body_provider::*;
		pub use crate::asset_capabilities::*;
		pub use crate::runtime_assets::*;
	}

	/// Hidden runtime document build contracts.
	pub mod contracts {
		pub use crate::document_builder::*;
	}

	/// Public configuration lowering helpers.
	pub mod config {
		pub use crate::config::{Config, ConfigError, normalize_public_static_base};
	}

	/// Scoped runtime/build mode helpers for build tooling.
	pub mod env {
		pub use crate::envutil::{with_build_mode, with_dev_mode};
	}

	/// Public facade used to compile declarations into graph and handlers.
	pub mod facade {
		pub use crate::app_assembly::*;
		pub use crate::facade::*;
	}

	/// Public route input resolver contracts.
	pub mod route_input {
		pub use crate::route_input::*;
		pub use crate::search_params::schema_for_type;
	}

	pub use crate::public_app::AppBuildContract;
	pub use crate::route_input::{search_schema_resolver, type_resolver};
	pub use crate::static_route::{
		ErasedRequestCtx, ErasedRouteFuture, ErasedRouteHandler, InputError, PathParams,
		RouteFuture, StaticResourceInput, run_static_resource, run_static_view,
	};
	pub use vorma_matcher::Params;

	/// Compile public app config into the hidden build-facing contract.
	pub fn app_build_contract<S>(app_config: crate::AppConfig<S>) -> crate::Result<AppBuildContract>
	where
		S: Send + Sync + 'static,
	{
		crate::App::from_app_config(app_config)?.into_build_contract()
	}

	/// Build live-state facts from an in-process app build contract.
	pub async fn live_build_state_from_app_build_contract(
		app: AppBuildContract,
	) -> Result<
		vorma_contract::live_state::LiveBuildState,
		vorma_contract::live_state::LiveBuildStateError,
	> {
		use vorma_contract::live_state::{LiveBuildState, LiveBuildStateError};

		let (graph, document_builder) = app.into_parts();
		let root_document_hash_source = document_builder
			.build_root_document_hash_source()
			.await
			.map_err(|source| LiveBuildStateError::RootDocumentHashSource {
				message: source.to_string(),
			})?;
		Ok(LiveBuildState::from_graph(graph, root_document_hash_source))
	}

	/// Compile public app config into the canonical build graph.
	pub fn app_build_graph<S>(
		app_config: crate::AppConfig<S>,
	) -> crate::Result<vorma_contract::framework_graph::FrameworkGraph>
	where
		S: Send + Sync + 'static,
	{
		Ok(app_build_contract(app_config)?.into_parts().0)
	}

	/// Immutable runtime snapshot contracts.
	pub mod runtime {
		pub use crate::execution_engine::*;
		pub use crate::handler_context::*;
		pub use crate::input_decoder::*;
		pub use crate::payload_projection::*;
		pub use crate::resource_response::*;
		pub use crate::response_finalizer::*;
		pub use crate::runtime_app::*;
		pub use crate::runtime_document::*;
		pub use crate::runtime_host::*;
		pub use crate::runtime_service::*;
		pub use crate::runtime_snapshot::*;
		pub use crate::typed_handler::*;
		pub use crate::view_response::*;
	}
}

/*
Macro ABI only: these exact paths are emitted by `vorma-macros` into
application crates, so they must stay path-stable. Everything the build
crate consumes lives in `build_interface` instead.
*/
#[doc(hidden)]
pub mod __private {
	pub use vorma_matcher::Params;

	pub use crate::route_input::{search_schema_resolver, type_resolver};
	pub use crate::static_route::{InputError, PathParams, run_static_resource, run_static_view};
}
