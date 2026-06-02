//! Rust full-stack framework runtime and app declaration API.
//!
//! Application code declares views, resources, middleware, document defaults, and
//! build/runtime config through this crate. `vorma-build` consumes the same declarations
//! to generate manifests and TypeScript contracts, while [`RuntimeHost`] serves the
//! runtime handler that users mount into their HTTP stack.

extern crate self as vorma;

use std::net::{Ipv4Addr, SocketAddr};
use std::path::PathBuf;

use crate::config::Config;

mod api;
mod config;
mod constants;
mod core;
mod document;
mod envutil;
mod error;
mod handler;
mod head;
mod htmlutil;
mod init;
mod manifest;
pub mod middleware;
mod mux;
#[cfg(test)]
mod mux_tests;
mod request;
mod response;
mod searchparams;
mod r#static;
mod tsgen;
mod view_payload;

pub use api::{FormData, FormField, FormFile};
#[doc(hidden)]
pub use api::{ResourceInput, ViewInput};
pub use config::{DevWatchConfig, FrontendConfig, PathConfig, ServerConfig, TsGenConfig};
pub use cookie::Cookie as HttpCookie;
pub use core::{
	HeadHandle, Middleware, MiddlewareCtx, Middlewares, Params, Resource, ResourceCtx,
	ResourceKind, Resources, ResponseHandle, View, ViewCtx, Views,
};
pub use document::{Document, DocumentAttributes, DocumentBuildCtx, DocumentBuilder};
pub use error::{BoxError, Error, ViewError, ViewErrorClientMsg};
pub use head::{
	HeadAttr, HeadBooleanAttribute, HeadBuilder, HeadInnerHtml, HeadSelfClosing, HeadTag,
	HeadTextContent, HtmlElementDef,
};
pub use http::{
	HeaderName as HttpHeaderName, HeaderValue as HttpHeaderValue, Method as HttpMethod,
	StatusCode as HttpStatusCode,
};
pub use init::RuntimeHost;
pub use request::{HttpRequest, HttpSearchParams};
pub use tsgen::{
	Error as TsError, FieldDef, RawTsPart, Result as TsResult, TsDrafter, TsExtraType, Type,
	TypeDef, TypePhase, TypeRef, TypeRegistry,
};
pub use vorma_macros::TsGen;
#[doc(hidden)]
pub use vorma_macros::{__vorma_resource, __vorma_view};
pub use vorma_tasks::{
	CancelToken, Clock as TaskClock, ClockInstant as TaskClockInstant, Error as TaskError, ExecCtx,
	PreparedTask, Result as TaskResult, SystemClock as SystemTaskClock, Task, TaskEvent,
	TaskEventKind, TaskEventOutcome, TaskId, TaskObserver, TaskOverrideMode, TaskOverrides,
	TaskRunSource, Tasks, TasksOptions,
};

/// Result type returned by Vorma public helpers.
pub type Result<T> = std::result::Result<T, Error>;

/// Helpers for constructing low-level HTML attributes in head/document builders.
pub struct HtmlAttribute;

impl HtmlAttribute {
	/// Create a normal escaped HTML attribute definition.
	pub fn attr(name: impl Into<String>, value: impl Into<String>) -> HtmlElementDef {
		head::HeadAttr::new(name, value).into()
	}

	/// Create a `type="..."` attribute definition.
	pub fn type_(value: impl Into<String>) -> HtmlElementDef {
		Self::attr("type", value)
	}
}

/// Helpers for explicitly trusted HTML fragments.
pub struct SafeHtml;

impl SafeHtml {
	/// Create trusted style contents for a `<style>` element.
	pub fn style_content(content: impl Into<String>) -> [HtmlElementDef; 1] {
		[head::HeadInnerHtml(content.into()).into()]
	}
}

/// Response header carrying the expected Vorma client build ID.
pub const CLIENT_BUILD_ID_HEADER_KEY: &str = constants::X_VORMA_CLIENT_BUILD_ID;
/// Prefix added to hashed public static asset output filenames.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = constants::PUBLIC_STATIC_OUT_NAME_PREFIX;

/// Whether the current process is running as a Vorma build/live-state entry.
pub fn is_build() -> bool {
	envutil::is_build()
}

/// Whether the current process is running an app server under Vorma dev mode.
pub fn is_dev() -> bool {
	envutil::is_dev()
}

/// Complete app declaration consumed by both `vorma-build` and [`RuntimeHost`].
pub struct AppConfig<S, E = Box<dyn std::error::Error + Send + Sync>> {
	/// Absolute application root used to resolve all relative config paths.
	pub root_dir: PathBuf,
	/// Cargo package/bin identity for the user app server target.
	pub server_config: ServerConfig,
	/// Dist directory, relative to [`Self::root_dir`] unless absolute.
	pub dist_dir: String,
	/// Public static and API mount paths.
	pub path_config: PathConfig,
	/// Frontend entry, Vite, package-manager, static, and critical-CSS config.
	pub frontend_config: FrontendConfig,
	/// Generated TypeScript output and supplemental declaration config.
	pub ts_gen_config: TsGenConfig,
	/// Dev watcher include/classification patterns.
	pub dev_watch_config: DevWatchConfig,
	/// Application state shared by handlers.
	pub state: S,
	/// Registered nested views.
	pub views: Views<S, E>,
	/// Registered resources.
	pub resources: Resources<S, E>,
	/// Registered middleware.
	pub middlewares: Middlewares<S, E>,
	/// Task runtime options.
	pub tasks_options: TasksOptions<E>,
	/// Document shell/default-head builder.
	pub document: DocumentBuilder,
	/// Maximum request body bytes Vorma will collect for its own handlers.
	pub request_body_limit: usize,
}

/// Normalized app ready to become a runtime host.
pub struct App<S, E = Box<dyn std::error::Error + Send + Sync>> {
	config: Config,
	state: S,
	views: Views<S, E>,
	resources: Resources<S, E>,
	middlewares: Middlewares<S, E>,
	tasks_options: TasksOptions<E>,
	document: DocumentBuilder,
	request_body_limit: usize,
}

impl<S, E> App<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	pub(crate) fn from_app_config(app_config: AppConfig<S, E>) -> Self {
		let AppConfig {
			root_dir,
			server_config,
			dist_dir,
			path_config,
			frontend_config,
			ts_gen_config,
			dev_watch_config,
			state,
			views,
			resources,
			middlewares,
			tasks_options,
			document,
			request_body_limit,
		} = app_config;
		Self {
			config: Config {
				root_dir,
				server_config,
				dist_dir,
				path_config,
				frontend_config,
				ts_gen_config,
				dev_watch_config,
			},
			state,
			views,
			resources,
			middlewares,
			tasks_options,
			document,
			request_body_limit,
		}
	}

	pub(crate) fn into_live_state_parts(
		self,
	) -> (Config, Views<S, E>, Resources<S, E>, DocumentBuilder) {
		let Self {
			config,
			state: _,
			views,
			resources,
			middlewares: _,
			tasks_options: _,
			document,
			request_body_limit: _,
		} = self;
		(config, views, resources, document)
	}
}

/// Default maximum request body size collected by Vorma handlers.
pub const DEFAULT_REQUEST_BODY_LIMIT: usize = 16 * 1024 * 1024;

/// Read `PORT` and return `0.0.0.0:<PORT>` for deployment-style app servers.
pub fn bind_addr() -> Result<SocketAddr> {
	bind_addr_from_port(
		&std::env::var("PORT")
			.map_err(|_| Error::runtime("PORT environment variable is required"))?,
	)
}

fn bind_addr_from_port(port: &str) -> Result<SocketAddr> {
	let port = port
		.parse::<u16>()
		.map_err(|error| Error::runtime(format!("invalid PORT {port:?}: {error}")))?;
	Ok(SocketAddr::from((Ipv4Addr::UNSPECIFIED, port)))
}

impl<S, E> App<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	/// Build a runtime host from an app declaration.
	pub fn from_config(app_config: AppConfig<S, E>) -> Result<RuntimeHost<S, E>> {
		RuntimeHost::new(Self::from_app_config(app_config)).map_err(Error::runtime)
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
            pub type App = ::vorma::App<State>;
            #[allow(dead_code)]
            pub type Resource = ::vorma::Resource<State>;
            #[allow(dead_code)]
            pub type Resources = ::vorma::Resources<State>;
            #[allow(dead_code)]
            pub type ResourceCtx<I = (), P = ()> = ::vorma::ResourceCtx<State, ::vorma::BoxError, I, P>;
            #[allow(dead_code)]
            pub type DocumentBuildCtx = ::vorma::DocumentBuildCtx;
            #[allow(dead_code)]
            pub type DocumentBuilder = ::vorma::DocumentBuilder;
            #[allow(dead_code)]
            pub type MiddlewareCtx = ::vorma::MiddlewareCtx<State>;
            #[allow(dead_code)]
            pub type Middleware = ::vorma::Middleware<State>;
            #[allow(dead_code)]
            pub type Middlewares = ::vorma::Middlewares<State>;
            #[allow(dead_code)]
            pub type View = ::vorma::View<State>;
            #[allow(dead_code)]
            pub type ViewCtx<I = (), P = ()> = ::vorma::ViewCtx<State, ::vorma::BoxError, I, P>;
            #[allow(dead_code)]
            pub type Views = ::vorma::Views<State>;

            #[allow(unused_imports)]
            pub use ::vorma::{
                __vorma_resource as resource, __vorma_resources as resources,
                __vorma_middlewares as middlewares, __vorma_view as view,
                __vorma_views as views,
            };
        }
    };
}

#[doc(hidden)]
#[macro_export]
macro_rules! __vorma_views {
    () => {
        ::vorma::Views::new()
    };
    ($($view:expr),+ $(,)?) => {{
        let mut views = ::vorma::Views::new();
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
        ::vorma::Resources::new()
    };
    ($($resource:expr),+ $(,)?) => {{
        let mut resources = ::vorma::Resources::new();
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
        ::vorma::Middlewares::new()
    };
    ($($middleware:expr),+ $(,)?) => {{
        let mut middlewares = ::vorma::Middlewares::new();
        $(
            middlewares.push($middleware);
        )*
        middlewares
    }};
}

#[doc(hidden)]
pub mod __private {
	pub use crate::config::{
		Config, normalize_api_mount_root, normalize_public_static_base,
		validate_public_static_base_against_api_mount,
	};

	pub mod constants {
		pub use crate::constants::{
			ENV_KEY_IS_BUILD, ENV_KEY_IS_DEV, PROD_TMP_VITE_MANIFEST_FILENAME,
			PUBLIC_STATIC_OUT_NAME_PREFIX,
		};
	}

	pub mod document {
		pub use crate::document::{
			DocumentBuildIdentity, DocumentBuildIdentityAttribute, DocumentBuildIdentityElement,
		};
	}

	pub mod core {
		pub use crate::core::{
			Contract, ResourceEntry, ViewEntry, contract_for, default_resource_kind,
			params_for_pattern, pattern_is_splat, view_parents_for_patterns,
		};
	}

	pub mod manifest {
		pub use crate::manifest::{
			ClientCoreAssets, ClientModule, MANIFEST_STATIC_OUT_DEV, MANIFEST_STATIC_OUT_PROD,
			Manifest,
		};
	}

	pub mod tsgen {
		pub use crate::tsgen::*;
	}

	pub use crate::core::{
		ErasedRequestCtx, ErasedRouteFuture, PathParams, RouteFuture, TypeResolver,
		run_static_resource, run_static_view, search_schema_resolver, type_resolver,
	};
	pub use crate::mux::{InputError, Params};
	pub use crate::{ResourceInput, ViewInput};

	pub fn is_build() -> bool {
		crate::envutil::is_build()
	}

	pub fn is_dev() -> bool {
		crate::envutil::is_dev()
	}

	pub fn is_json_request(request: crate::HttpRequest<'_>) -> bool {
		crate::handler::is_json_request(request.uri())
	}

	pub fn app_live_state_parts<S, E>(
		app_config: crate::AppConfig<S, E>,
	) -> (
		Config,
		crate::Views<S, E>,
		crate::Resources<S, E>,
		crate::DocumentBuilder,
	)
	where
		S: Send + Sync + 'static,
		E: Send + Sync + 'static,
	{
		crate::App::from_app_config(app_config).into_live_state_parts()
	}
}

#[cfg(test)]
mod public_api_tests {
	use super::*;

	#[test]
	fn golden_root_helpers_are_available() {
		let cookie = HttpCookie::new("ping", "1");
		assert_eq!(cookie.name(), "ping");

		let error = Error::runtime("boom");
		assert_eq!(error.to_string(), "boom");
		let task_error: TaskError<BoxError> = error.into();
		assert!(matches!(task_error, TaskError::Failed(_)));
		let _: TaskResult<(), BoxError> = Ok(());

		let mut head = HeadBuilder::new();
		head.script([HtmlAttribute::type_("application/json")]);
		head.style(SafeHtml::style_content(":root{color-scheme:light dark}"));
		assert_eq!(head.elements().len(), 2);

		let _: Result<()> = Ok(());
	}

	#[test]
	fn structured_ts_helpers_are_available_from_root() {
		let mut drafter = TsDrafter::new();
		drafter.export_type("Extra", "{ ok: true }").unwrap();
		let _: TsResult<_> = drafter
			.export_const("EXTRA_FLAGS", serde_json::json!({ "ok": true }))
			.map_err(|err: TsError| err);
		drafter
			.export_const("EXTRA_FLAGS_2", serde_json::json!({ "ok": true }))
			.unwrap();
		assert!(drafter.to_string().contains("export type Extra"));
		assert!(drafter.to_string().contains("export const EXTRA_FLAGS"));

		let type_ref = TypeRef::Raw(vec![
			RawTsPart::Text("Promise<".to_owned()),
			RawTsPart::TypeRef(TypeRef::String),
			RawTsPart::Text(">".to_owned()),
		]);
		let def = TypeDef::alias("AsyncString", type_ref);
		let mut registry = TypeRegistry::default();

		assert!(registry.try_define(def).unwrap());
	}

	#[test]
	fn task_support_helpers_are_available_from_root() {
		let clock = SystemTaskClock::new();
		let at: TaskClockInstant = TaskClock::now(&clock);

		assert!(at <= TaskClock::now(&clock));
	}

	#[test]
	fn bind_addr_from_port_binds_unspecified_ipv4() {
		let addr = super::bind_addr_from_port("8080").unwrap();

		assert_eq!(addr.to_string(), "0.0.0.0:8080");
	}

	#[test]
	fn bind_addr_from_port_rejects_invalid_ports() {
		assert_eq!(
			super::bind_addr_from_port("not-a-port")
				.unwrap_err()
				.to_string(),
			"invalid PORT \"not-a-port\": invalid digit found in string"
		);
	}
}
