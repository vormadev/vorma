//! Rust full-stack framework runtime and app declaration API.
//!
//! Application code declares views (HTML-rendering routes), resources (JSON/binary API
//! routes), middleware, a document shell, and build/runtime configuration through this
//! crate. `vorma-build` consumes the exact same declarations to generate the client
//! manifest and the TypeScript contract file; [`RuntimeHost`] serves the committed result
//! as the Tower [`Service`](tower_service::Service) an app mounts into its HTTP stack. Per
//! the framework's documentation strategy, this crate's rust-doc comments — together with
//! `examples/board`, the tutorial app that exercises the whole public surface — are the
//! user manual; there is no separate hand-written guide beyond a small set of
//! supplemental docs for things code comments cannot cover (hosting, adapter differences).
//!
//! # Getting started
//!
//! [`app!`] declares a typed local module of app aliases and declaration macros bound to
//! one application state type, then [`AppConfig`] assembles a runtime host from views,
//! resources, and middleware built with those aliases:
//!
//! ```
//! use serde::{Deserialize, Serialize};
//!
//! // A real app's state is usually a struct of shared handles (a database pool, for
//! // example); this example uses `()` to keep the state type out of the way.
//! vorma::app!(mod app for ());
//!
//! #[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
//! struct HomeInput {}
//!
//! #[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
//! struct HomeOutput {
//!     message: String,
//! }
//!
//! const HOME_VIEW: app::View = app::view! {
//!     client_file: "src/client/views/home.view.tsx";
//!     pattern: "/";
//!     input: HomeInput;
//!     output: HomeOutput;
//!
//!     handler: |ctx| {
//!         ctx.head().title("Home");
//!         Ok(HomeOutput { message: "Hello from Vorma".to_owned() })
//!     };
//! };
//!
//! fn app_config() -> vorma::AppConfig<()> {
//!     vorma::AppConfig {
//!         views: app::views![HOME_VIEW],
//!         ..vorma::AppConfig::default()
//!     }
//! }
//!
//! # #[tokio::main(flavor = "current_thread")]
//! # async fn main() -> vorma::Result<()> {
//! let app = vorma::testing::TestApp::from_config(app_config())?;
//! let response = app.get("/").await;
//! assert_eq!(response.status(), vorma::HttpStatusCode::OK);
//! # Ok(())
//! # }
//! ```
//!
//! A real app builds [`AppConfig`] once, in one function both the production server
//! binary and the build command call, so every consumer compiles the exact same route
//! graph (`examples/board` names this pattern `app_config` and keeps every Vorma-specific
//! choice behind it). [`App::from_config`] turns that declaration into a [`RuntimeHost`]
//! ready to mount as a Tower service; `vorma_build::run` consumes the same declaration
//! at build time to write the committed manifest and the generated TypeScript contract
//! (`vorma-build` depends on this crate, not the reverse, so its API cannot be linked
//! from here — see its own crate documentation).
//!
//! # Views, resources, and middleware
//!
//! A **view** ([`View`], declared with [`app!`]'s generated `view!` macro) renders one
//! nested segment of an HTML page: `input` is decoded from the URL (path params and query
//! string), `output` becomes both the server-rendered fragment's data and the JSON payload
//! the client-side router fetches on navigation, and the `handler` closure receives a
//! [`ViewCtx`]. A **resource** ([`Resource`], declared with the generated `resource!`
//! macro) is an ordinary JSON or binary API endpoint bound to one HTTP method: `output` is
//! the response body (or [`ResourceBody`] for raw bytes), and the handler receives a
//! [`ResourceCtx`]. Both `input` types must implement `Deserialize` + [`tsgen::Type`]
//! (usually via `#[derive(TsGen)]`, whose full contract is documented on the derive
//! itself) so the same shape drives request decoding, generated TypeScript, and — for
//! views — the URL search-param schema the client-side router reads. **Middleware**
//! ([`Middleware`]) runs before every view/resource its patterns and methods select,
//! receiving a [`MiddlewareCtx`]; see [`Middleware::new`] for the exact contract, and the
//! [`middleware`] module for optional Tower-layer building blocks (secure headers,
//! request IDs, ETags, compression) that compose alongside it in the surrounding HTTP
//! stack rather than through this crate's own middleware system.
//!
//! Views and resources within one phase, and every registered middleware, always execute
//! in parallel — this is a framework contract app authors can rely on, not an incidental
//! optimization that might change; see [`ViewCtx`]/[`ResourceCtx`]/[`MiddlewareCtx`] for
//! what each handler receives and [`testing`] for exercising the whole pipeline in tests.
//!
//! # Errors and early exits
//!
//! Handlers return `Result<Output, Exit>`: views use [`ViewExit`] (no HTTP status concept
//! — a view is a framework-owned rendering protocol segment, not a standalone HTTP
//! response) and resources/middleware use [`HttpExit`] (a real HTTP boundary, so it
//! carries a status). Both distinguish a server-side record (`err`, for logs — never sent
//! to the client) from an optional client-visible message (`with_client_msg` — absent that,
//! the client sees a generic framework message), and both convert from [`Error`] and
//! boxed errors via `?`, so an ordinary fallible handler body just propagates. See
//! [`ViewExit`]/[`HttpExit`] for the full contract, including how task-runtime errors
//! convert.
//!
//! # Background work: the `tasks` module
//!
//! [`ViewCtx::exec_ctx`]/[`ResourceCtx::exec_ctx`]/[`MiddlewareCtx::exec_ctx`] hand every
//! handler an [`ExecCtx`](tasks::ExecCtx) scoped to that one request — the framework
//! constructs the underlying [`Tasks`](tasks::Tasks) runtime and opens one
//! [`ExecCtx`](tasks::ExecCtx) per request, so application code never builds its own.
//! Declare cached, deduplicated, or parallel-fanned-out work with
//! [`tasks::task!`] and resolve it through that context; see the [`tasks`] module for the
//! consumer-facing re-export surface, and the standalone `vorma-tasks` crate's own
//! documentation for the full doctrine (the three cache policies, cancellation,
//! true spawned parallelism, cycle detection) — `vorma-tasks` is a sovereign
//! general-purpose crate this framework builds on, not a request-only helper, so its docs
//! are the canonical source for that doctrine rather than a copy living here.
//!
//! # Testing
//!
//! [`testing::TestApp`] boots a complete application from its [`AppConfig`] declaration
//! without requiring build artifacts on disk, and serves requests through the same
//! committed pipeline production uses. See the [`testing`] module for the full harness,
//! including [`testing::TestSession`] for cookie-continuation auth-flow tests.

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
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
mod resource_body;
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

/// TypeScript type model: the trait [`derive(TsGen)`](TsGen) implements and the type
/// tree it produces, consumed by `vorma-build`'s TypeScript generator and by
/// [`TsGenConfig::extra_types`]/[`TsGenConfig::extra_ts`] for types/declarations an app
/// wants exported without a route referencing them.
pub use vorma_contract::tsgen;

/// Build/runtime configuration types assembled into [`AppConfig`]'s frontend, TypeScript
/// generation, and dev-watcher fields.
pub use config::{DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig, UiVariant};
/// Cookie type shared by [`ResponseHandle::set_cookie`]/[`ResourceResponseHandle::set_cookie`]
/// and [`testing`]'s cookie helpers — a re-export of the `cookie` crate's own type so
/// handler code and tests build cookies through one shared, full-featured API.
pub use cookie::Cookie as HttpCookie;
/// Document shell types: [`Document`] declares the root `<html>`/`<body>` attributes and
/// default head elements; [`DocumentBuilder`] wraps an async function that builds one from
/// a [`DocumentBuildCtx`] per request. See [`DocumentBuilder::new`] for the full contract.
pub use document_builder::{
	Document, DocumentAttributes, DocumentBuildContext as DocumentBuildCtx, DocumentBuilder,
};
/// The plain framework/setup error type ([`Error`]) and its boxed-source alias
/// ([`BoxError`]) — for config validation and helper failures, distinct from the handler
/// early-exit types ([`ViewExit`]/[`HttpExit`]); both convert from this type via `?`.
pub use error::{BoxError, Error};
/// Handler early-exit types: the only path to error and redirect outcomes from a
/// view/resource/middleware handler. See [`ViewExit`]/[`HttpExit`] for the full contract.
pub use exit::{HttpExit, ViewExit};
/// Parsed form/multipart request input, usable as a route's `input` type in place of a
/// typed struct. See [`FormData`] for the full field/file access API.
pub use form_data::{FormData, FormField, FormFile};
/// Generated-client revalidation-behavior override for a [`resource!`](app!) declaration's
/// optional `kind` field: [`ResourceKind::Query`] (reads) or [`ResourceKind::Mutation`]
/// (writes) — the generated TypeScript client uses this to decide whether calling this
/// resource should revalidate other cached reads. Without an explicit `kind`, the
/// framework infers one from the HTTP method: `GET`/`HEAD` default to
/// [`ResourceKind::Query`], every other method defaults to [`ResourceKind::Mutation`].
/// Set `kind` explicitly only when a resource's real behavior disagrees with that default
/// (a `POST` search endpoint that only reads, for example).
pub use framework_graph::ResourceKind;
/// Low-level head/document element builder primitives underlying [`HeadBuilder`] and the
/// handler [`HeadHandle`]; reach for [`HeadBuilder`]'s named helpers (`title`, `meta`,
/// `link`, and friends) first — these markers exist for elements those helpers do not
/// cover.
pub use head::{
	HeadAttr, HeadBooleanAttribute, HeadBuilder, HeadInnerHtml, HeadSelfClosing, HeadTag,
	HeadTextContent, HtmlElementDef,
};
/// The app declaration API: [`AppConfig`] is what an app builds and hands to
/// [`App::from_config`]; [`View`]/[`Resource`]/[`Middleware`] and their collections
/// ([`Views`]/[`Resources`]/[`Middlewares`]) are what [`app!`]'s `view!`/`resource!`/
/// `views!`/`resources!`/`middlewares!` macros construct. See the
/// [crate-root example](crate#getting-started).
pub use public_app::{App, AppConfig, Middleware, Middlewares, Resource, Resources, View, Views};
/// Read-only HTTP request views handed to handlers and document builders. See
/// [`HttpRequest`] for the full accessor set.
pub use request::{HttpRequest, HttpSearchParams};
/// Raw resource response output: [`ResourceBody`] returns bytes with an explicit content
/// type in place of a serialized `output` type; [`ResourceOutput`] is the sealed trait
/// every valid resource `output` type implements (either automatically, for any
/// `Serialize` type, or via [`ResourceBody`]) — it cannot be implemented outside this
/// crate, so a resource's `output` is always one of those two shapes.
pub use resource_body::{ResourceBody, ResourceOutput};
/// The Tower service an app mounts into its HTTP stack. See [`RuntimeHost`] and
/// [`App::from_config`].
pub use runtime_host::RuntimeHost;
/// Response header carrying the expected Vorma client build ID.
pub const CLIENT_BUILD_ID_HEADER_KEY: &str = response_finalizer::CLIENT_BUILD_ID_HEADER;
/// Prefix added to hashed public static asset output filenames.
pub const PUBLIC_STATIC_OUT_NAME_PREFIX: &str = constants::PUBLIC_STATIC_OUT_NAME_PREFIX;
/// Default maximum request body size collected by Vorma handlers, used by
/// [`AppConfig::request_body_limit`] when an app does not set one explicitly.
pub const DEFAULT_REQUEST_BODY_LIMIT: usize = app_assembly::DEFAULT_REQUEST_BODY_LIMIT;
/// HTTP type aliases re-exported from the `http` crate for use in handler signatures
/// (route methods, status codes, header names/values/maps) without an app needing its own
/// direct dependency on `http`.
pub use http::{
	HeaderMap as HttpHeaderMap, HeaderName as HttpHeaderName, HeaderValue as HttpHeaderValue,
	Method as HttpMethod, StatusCode as HttpStatusCode,
};
#[doc(hidden)]
pub use route_input::{ResourceInput, ViewInput};
/// Route handler context types: what [`app!`]'s `view!`/`resource!`-declared handlers and
/// [`Middleware::new`] closures receive. See [`ViewCtx`]/[`ResourceCtx`]/[`MiddlewareCtx`]
/// for the full per-context accessor set.
pub use static_route::{MiddlewareCtx, Params, ResourceCtx, ViewCtx};
/// Mutable per-request effect handles reached through a handler ctx's `head()`/
/// `response()` accessors: [`HeadHandle`] queues document head elements,
/// [`ResponseHandle`]/[`ResourceResponseHandle`] set response headers/cookies/status.
/// These are not constructed directly by app code.
pub use typed_handler::{
	TypedHeadHandle as HeadHandle, TypedResourceResponseHandle as ResourceResponseHandle,
	TypedResponseHandle as ResponseHandle,
};

/// Result type returned by Vorma public helpers.
pub type Result<T> = std::result::Result<T, Error>;

/// Whether the current process is running as a Vorma build/live-state entry.
///
/// The exact same app server binary doubles as the build's declaration emitter: when the
/// build process's env key is set, [`App::from_config`] serializes the compiled
/// declaration graph to stdout and exits instead of constructing a [`RuntimeHost`]. An app
/// rarely needs to call this directly — it exists mainly for app-authored setup code
/// (logging init, telemetry) that should only run for an actual serving process, not the
/// short-lived build/live-state invocation.
pub fn is_build() -> bool {
	envutil::is_build()
}

/// Whether the current process is running an app server under Vorma dev mode.
///
/// Distinguishes a dev-mode run (the dev server's live-reload/dev-watch behavior applies)
/// from a production run serving committed build output. Most apps never branch on this
/// directly — [`RuntimeHost`] already reads it internally to pick dev vs. prod manifest
/// loading — but it is available for app-authored behavior that should differ between the
/// two (for example, a more verbose local logger).
pub fn is_dev() -> bool {
	envutil::is_dev()
}

/// Read `PORT` and return `0.0.0.0:<PORT>` for deployment-style app servers.
///
/// Convenience for platforms that inject the listen port through a `PORT` environment
/// variable (Fly.io, Render, Heroku, and similar): binding `0.0.0.0` (all interfaces) is
/// what those platforms' proxies expect. Returns an [`Error`] when `PORT` is unset or is
/// not a valid `u16`.
pub fn bind_addr() -> Result<std::net::SocketAddr> {
	network::bind_addr()
}
/// Derive [`tsgen::Type`] for a route input/output type from its existing
/// `Serialize`/`Deserialize` shape. See [`macro@TsGen`]'s own documentation for the full
/// contract: supported shapes, field optionality rules, every rejection, and the
/// shared-type-across-both-phases resolution rule.
pub use vorma_macros::TsGen;
#[doc(hidden)]
pub use vorma_macros::{__vorma_resource, __vorma_view};

/// The consumer-facing surface of the standalone `vorma-tasks` runtime.
///
/// A request's [`ExecCtx`](tasks::ExecCtx) arrives pre-built on every handler ctx (see
/// [`ViewCtx::exec_ctx`]/[`ResourceCtx::exec_ctx`]/[`MiddlewareCtx::exec_ctx`]); an app
/// never constructs its own [`Tasks`](tasks::Tasks) runtime or an
/// [`ExecCtx`](tasks::ExecCtx) for request handling. This module re-exports exactly what a
/// handler needs to resolve [`task!`](tasks::task)-declared work: the macro itself,
/// [`Task`](tasks::Task) and its run methods, [`ParallelBatch`](tasks::ParallelBatch) for
/// fanning out independent work in parallel, [`CancelToken`](tasks::CancelToken) for
/// reading/propagating cancellation, and the observer/override/clock types for advanced
/// instrumentation and testing.
///
/// `vorma-tasks` is a sovereign, general-purpose crate — it models CLI and build-system
/// work as naturally as request handling, and Vorma is one consumer of it, not its only
/// reason to exist — so the full doctrine (the three cache policies and their exact
/// retention semantics, single-flight coalescing, true spawned parallelism and when
/// [`ParallelBatch`](tasks::ParallelBatch) earns its spawn cost, cooperative cancellation,
/// cycle detection, the `task!` static-node model) lives in `vorma-tasks`'s own crate
/// documentation rather than duplicated here; start there, then come back to this module
/// for the exact re-exported names available through `vorma::tasks::*`.
pub mod tasks {
	pub use vorma_tasks::{
		CancelToken, Clock, ClockInstant, Error, ExecCtx, ParallelBatch, ParallelBatchOutputHandle,
		ParallelBatchOutputs, Result, SystemClock, Task, TaskEvent, TaskEventKind,
		TaskEventOutcome, TaskId, TaskObserver, TaskOverrideMode, TaskOverrides, TaskRunSource,
		Tasks, TasksOptions, task,
	};
}

/// Helpers for constructing low-level HTML attributes in head/document builders.
///
/// [`HeadBuilder`]'s named helpers (`attr`, `bool_attr`, `property`, `name`, and friends)
/// already cover attribute construction inside a [`HeadBuilder`]/[`Document`] call chain;
/// reach for [`HtmlAttribute`] only when building a raw [`HtmlElementDef`] list outside
/// that context — for example, passing attributes into [`HeadHandle::add`] or
/// [`DocumentAttributes`] helpers that accept [`HtmlElementDef`]s directly.
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
///
/// Every other head/document text path (attribute values, text content) is escaped by
/// default. `style_content`/`script_content` are the deliberate escape hatch for
/// `<style>`/`<script>` element bodies that must render as raw markup: content passed to
/// `script_content` (or any non-`<style>` element's inner HTML) is written out completely
/// unescaped, and `style_content`'s content only ever gets one narrow structural guard
/// (a literal `</style` breakout sequence is neutralized so it cannot end the element
/// early) — neither is sanitization. Only pass content the application itself controls
/// through either helper; never pass unsanitized user input.
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
///
/// `app!(mod app for MyState)` generates a module (named `app` here, but any identifier
/// works, and it accepts a visibility modifier: `app!(pub mod app for MyState)`) whose
/// `App`/`View`/`Resource`/`Views`/`Resources`/`Middleware`/`Middlewares`/`ViewCtx`/
/// `ResourceCtx`/`MiddlewareCtx`/`DocumentBuilder`/`DocumentBuildCtx` are the crate-root
/// types of the same name, pre-applied to `MyState` so route/handler declarations never
/// repeat the state type parameter — see the [crate-root example](crate#getting-started)
/// for the shape end to end. It also re-exports five declaration macros scoped to that
/// state: `view!`, `resource!`, `views!`, `resources!`, and `middlewares!`.
///
/// `MyState` may be any type path valid at the call site — a bare local name, a
/// `use`-imported name, a `self::`- or `crate::`-qualified path, or any other in-scope
/// path all resolve exactly as they would in ordinary Rust, regardless of where the type
/// is declared relative to the `app!` call.
///
/// # `view!` and `resource!`
///
/// These build one [`View`]/[`Resource`] from a fixed set of semicolon-terminated fields,
/// always in this order:
///
/// - `client_file` (`view!` only) — the frontend view module's path, matched against the
///   generated client manifest.
/// - `kind` (`resource!` only, optional) — a [`ResourceKind`] override for the generated
///   TypeScript client's default revalidation behavior; omit it to let the framework infer
///   one from the method (see [`ResourceKind`]'s own docs for the exact inference rule).
/// - `method` (`resource!` only) — the [`HttpMethod`] this resource handles.
/// - `pattern` — the route pattern, in the same syntax the underlying matcher accepts
///   (`:name` for a dynamic segment, `*` for a trailing splat, `_index` for an explicit
///   index segment). Every dynamic segment becomes a strongly typed field on the ctx's
///   [`params()`](ViewCtx::params) struct, generated for you — `pattern: "/shelves/:id"`
///   means the handler can call `ctx.params().id` as well as the untyped
///   [`param("id")`](ViewCtx::param).
/// - `input` — the route's input type, decoded from path params + query string (views) or
///   the request body/query (resources); must implement `Deserialize` and
///   [`tsgen::Type`] (`#[derive(TsGen)]` derives both alongside `Deserialize`/`Serialize`).
///   Use `()` for no meaningful input, or [`FormData`] to receive raw parsed form/
///   multipart input instead of a typed struct.
/// - `output` — the route's output type, serialized as the response body (views: also the
///   fragment's render data); must implement `Serialize` + [`tsgen::Type`], or be
///   [`ResourceBody`] for a resource returning raw bytes with an explicit content type.
/// - `handler` — a `|ctx| { ... }` closure body (not a real closure — the macro rewrites
///   it into an async function pointer, so it cannot capture its environment) whose last
///   expression is `Ok(output)` or an early `Err(exit)`; `ctx` is a [`ViewCtx`]/
///   [`ResourceCtx`] typed to this route's `input`/params/state.
///
/// ```
/// # use serde::{Deserialize, Serialize};
/// vorma::app!(mod app for ());
///
/// #[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
/// struct ShelfInput {}
///
/// #[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
/// struct ShelfOutput {
///     shelf_id: String,
/// }
///
/// const SHELF_VIEW: app::View = app::view! {
///     client_file: "src/client/views/shelf.view.tsx";
///     pattern: "/shelves/:shelf_id";
///     input: ShelfInput;
///     output: ShelfOutput;
///
///     handler: |ctx| {
///         Ok(ShelfOutput { shelf_id: ctx.params().shelf_id.clone() })
///     };
/// };
///
/// const DELETE_SHELF: app::Resource = app::resource! {
///     kind: vorma::ResourceKind::Mutation;
///     method: vorma::HttpMethod::DELETE;
///     pattern: "/api/shelves/:shelf_id";
///     input: ();
///     output: ();
///
///     handler: |ctx| {
///         let _shelf_id = &ctx.params().shelf_id;
///         Ok(())
///     };
/// };
///
/// # fn main() {}
/// ```
///
/// # `views!`, `resources!`, and `middlewares!`
///
/// Each collects its declarations into the corresponding [`Views`]/[`Resources`]/
/// [`Middlewares`] collection for [`AppConfig`]: `app::views![VIEW_A, VIEW_B]`,
/// `app::resources![]` for none. See the [crate-root example](crate#getting-started).
#[macro_export]
macro_rules! app {
	($vis:vis mod $module:ident for $state:ty) => {
		#[doc(hidden)]
		#[allow(dead_code)]
		type __VormaAppState = $state;

		$vis mod $module {
			#[allow(dead_code)]
			pub type State = super::__VormaAppState;
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

	const MACRO_RAW_RESOURCE: macro_app::Resource = macro_app::resource! {
		kind: crate::ResourceKind::Query;
		method: crate::HttpMethod::GET;
		pattern: "/macro-file/:id";
		input: ();
		output: crate::ResourceBody;

		handler: |ctx| {
			let _ = ctx.params().id;
			Ok(crate::ResourceBody::new(
				crate::HttpHeaderValue::from_static("text/plain"),
				b"raw-body".to_vec(),
			))
		};
	};

	#[test]
	fn public_declaration_macros_lower_into_fresh_app_assembly() {
		let app = crate::App::from_app_config(crate::AppConfig {
			state: MacroState {
				prefix: "macro-".to_owned(),
			},
			views: macro_app::views![MACRO_VIEW],
			resources: macro_app::resources![
				MACRO_RESOURCE,
				MACRO_FORM_DATA_RESOURCE,
				MACRO_RAW_RESOURCE,
			],
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
		assert_eq!(declarations.resources().len(), 3);
		assert_eq!(declarations.middlewares().len(), 1);
	}

	#[test]
	fn hidden_build_graph_helper_compiles_public_app_config() {
		let graph = crate::build_interface::app_build_graph(crate::AppConfig {
			state: MacroState {
				prefix: "build-".to_owned(),
			},
			views: macro_app::views![MACRO_VIEW],
			resources: macro_app::resources![
				MACRO_RESOURCE,
				MACRO_FORM_DATA_RESOURCE,
				MACRO_RAW_RESOURCE,
			],
			middlewares: macro_app::middlewares![],
			..crate::AppConfig::default()
		})
		.unwrap();

		assert_eq!(graph.views().len(), 1);
		assert_eq!(graph.resources().len(), 3);
		let raw = graph
			.resources()
			.iter()
			.find(|resource| resource.pattern() == "/macro-file/:id")
			.expect("raw resource declaration");
		assert_eq!(
			raw.type_contract().output(),
			&crate::contracts::TypeRefContract::Blob
		);
	}
}

/// Build-facing runtime interface consumed by `vorma-build`.
#[doc(hidden)]
/*
The named contract between paired `vorma`/`vorma-build` versions: real,
documented surface, but not for application code — apps declare through the
public API and never need these. The two crates ship version-locked, so this
interface carries no cross-version stability promise.
*/
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
