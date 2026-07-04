//! Canonical private framework graph.
//!
//! Framework-integration surface — the shared truth `vorma` and
//! `vorma-build` compile an app's declarations into. Two shapes live
//! here: the `*Declaration` types
//! ([`MiddlewareDeclaration`](crate::framework_graph::MiddlewareDeclaration),
//! [`ViewDeclaration`](crate::framework_graph::ViewDeclaration),
//! [`ResourceDeclaration`](crate::framework_graph::ResourceDeclaration),
//! [`StaticAssetDeclaration`](crate::framework_graph::StaticAssetDeclaration))
//! collected into [`FrameworkDeclarations`](crate::framework_graph::FrameworkDeclarations)
//! are what `vorma`'s app-declaration macros build as an application
//! author writes routes;
//! [`compile`](crate::framework_graph::FrameworkGraph::compile) normalizes and
//! validates that raw input into the canonical `*Node` types
//! ([`MiddlewareNode`](crate::framework_graph::MiddlewareNode),
//! [`ViewNode`](crate::framework_graph::ViewNode),
//! [`ResourceNode`](crate::framework_graph::ResourceNode),
//! [`StaticAssetNode`](crate::framework_graph::StaticAssetNode)) every
//! later build/runtime stage — [`crate::execution_plan`],
//! [`crate::live_state`], generated TypeScript — actually reads. An
//! application author never touches either shape directly; `vorma`'s own
//! declaration API is what they write.
//!
//! # What graph compilation actually validates
//!
//! [`compile`](crate::framework_graph::FrameworkGraph::compile) is the
//! one place all of an app's cross-route invariants get checked at once,
//! each with its own [`GraphError`](crate::framework_graph::GraphError)
//! variant naming exactly what went wrong:
//!
//! - Every middleware/view/resource pattern parses under `vorma_matcher`'s
//!   grammar.
//! - View patterns are shape-unique (registering two views whose
//!   normalized shape collides is a compile error, not a silent shadow).
//! - Resource patterns are shape-unique per HTTP method — the same path
//!   shape may be a `GET` and a `POST` resource, never two `GET`s.
//! - A `GET`/`HEAD` resource and a view are allowed to overlap in the
//!   normal case (specificity adjudicates at request time — see
//!   [`crate::execution_plan`]), but a specificity **tie** between them
//!   is rejected up front, since neither could ever win at request time.
//! - `GET`/`HEAD` resources may not live inside the reserved public
//!   static asset space.
//! - Every [`crate::contracts::TypeRefContract::Named`] reference —
//!   whether inside a declared [`crate::contracts::TypeDef`] or a route's
//!   [`crate::contracts::RouteTypeContract`] — resolves to an
//!   actually-declared, uniquely-keyed type definition. Two definitions
//!   under different keys may not export the same TypeScript name, with
//!   one exception: one `TsGen`-derived Rust type's Serialize-phase and
//!   Deserialize-phase definitions share a name by construction, and
//!   collapse into one emitted definition when they are shape-equivalent
//!   (see [`crate::contracts::TypeDef::classify_shared_name_with`]) —
//!   a shape-divergent pair is still rejected, just with a different
//!   [`GraphError`](crate::framework_graph::GraphError) variant that
//!   teaches the two-distinct-Rust-types resolution.
//! - Static asset declarations resolve to safe relative paths and public
//!   paths inside the configured static base.
//!
//! A graph that compiles is guaranteed internally consistent; nothing
//! downstream re-checks these invariants.

use crate::contracts::{DocumentContract, RouteTypeContract, TypeDef};
use crate::graph_validation::{
	normalize_base_path, validate_middleware_shapes, validate_resource_reachability,
	validate_resource_shapes, validate_static_assets, validate_type_contracts,
	validate_view_shapes,
};
use http::Method;
use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Stable identifier for a declared runtime handler.
///
/// The graph's declaration types ([`MiddlewareDeclaration`],
/// [`ViewDeclaration`], [`ResourceDeclaration`]) carry one of these each,
/// rather than the handler function itself — the graph is a plain
/// serializable description of an app's shape (this is what the
/// build/dev-time [`crate::live_state`] protocol ships as JSON), and a
/// `HandlerId` is the stable string that lets the runtime crate look the
/// actual async handler back up once a request needs to run it.
#[derive(Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct HandlerId(String);

impl HandlerId {
	/// Create a validated handler identifier.
	///
	/// Trims surrounding whitespace and rejects a result that is then
	/// empty — the identifier's only structural requirement, since its
	/// value is otherwise app-declared free text.
	pub fn new(value: impl Into<String>) -> Result<Self, HandlerIdError> {
		let value = value.into();
		let trimmed = value.trim();
		if trimmed.is_empty() {
			return Err(HandlerIdError::Empty);
		}
		Ok(Self(trimmed.to_owned()))
	}

	/// Handler identifier string.
	pub fn as_str(&self) -> &str {
		&self.0
	}
}

/// Handler identifier construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HandlerIdError {
	/// Handler identifier was empty after trimming.
	Empty,
}

/// Generated-client classification for a resource.
///
/// Drives client-side cache/revalidation behavior in the generated
/// TypeScript API — a `Query` is treated as safe to cache and read
/// freely, a `Mutation` as something that should invalidate cached data
/// after it runs. Every resource has one automatically: `GET`/`HEAD`
/// default to `Query`, every other method defaults to `Mutation` (see
/// [`ResourceDeclaration::kind`]/[`ResourceNode::default_kind`]). An
/// explicit override is only worth declaring — and only emitted into
/// generated output — when it disagrees with that default: a `GET`
/// endpoint with side effects that should still invalidate cached data,
/// or a `POST` endpoint that is actually read-only (a search endpoint,
/// say) and safe to treat as a `Query`. This is a deliberate tradeoff to
/// keep the generated view/resource arrays type-level-only and
/// non-exported (not shipped to the client at runtime, to avoid inflating
/// the bundle) — see the maintainer reminders on generated-client
/// surface for the full rationale.
#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub enum ResourceKind {
	/// Read-shaped resource.
	Query,
	/// Write-shaped resource.
	Mutation,
}

impl ResourceKind {
	/// Return the generated TypeScript literal for this resource kind.
	pub fn as_str(self) -> &'static str {
		match self {
			Self::Query => "query",
			Self::Mutation => "mutation",
		}
	}
}

/// Top-level framework configuration that changes runtime or generated contracts.
///
/// [`Self::build_inputs`] is deliberately optional: a runtime-only graph
/// (compiled inside an already-running app server, never asked to emit
/// build artifacts) carries `None` here, while a build-time graph
/// (compiled by `vorma-build` or requested via [`crate::live_state`])
/// carries `Some`. The two are the same graph shape either way; only
/// whether build/dev projection is possible differs.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FrameworkConfig {
	public_static_base: String,
	build_inputs: Option<BuildInputConfig>,
}

impl FrameworkConfig {
	/// Create framework configuration from application declarations.
	pub fn new(public_static_base: impl Into<String>) -> Self {
		Self {
			public_static_base: public_static_base.into(),
			build_inputs: None,
		}
	}

	/// Attach build/dev inputs that must be projected from the canonical graph.
	pub fn with_build_inputs(mut self, build_inputs: BuildInputConfig) -> Self {
		self.build_inputs = Some(build_inputs);
		self
	}

	/// Declared public static base before normalization.
	pub fn public_static_base(&self) -> &str {
		&self.public_static_base
	}

	/// Build/dev inputs, when this graph is intended for build projection.
	pub fn build_inputs(&self) -> Option<&BuildInputConfig> {
		self.build_inputs.as_ref()
	}
}

impl Default for FrameworkConfig {
	fn default() -> Self {
		Self::new("/static")
	}
}

/// Build/dev inputs that belong to the canonical framework graph.
///
/// Everything `vorma-build` needs to turn a compiled graph into a real
/// build: where the app server binary lives ([`Self::server_target`]),
/// project directories, frontend build inputs, and where generated
/// TypeScript should be written. Present on a graph exactly when
/// [`FrameworkConfig::build_inputs`] is `Some`.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct BuildInputConfig {
	server_target: ServerBuildTarget,
	root_dir: String,
	dist_dir: String,
	frontend_inputs: FrontendBuildInputs,
	generated_typescript_file: String,
	generated_typescript_extra_source: String,
	dev_watch: DevWatchConfig,
}

impl BuildInputConfig {
	/// Create build/dev input configuration.
	pub fn new(
		server_target: ServerBuildTarget,
		root_dir: impl Into<String>,
		dist_dir: impl Into<String>,
		frontend_inputs: FrontendBuildInputs,
		generated_typescript_file: impl Into<String>,
		dev_watch: DevWatchConfig,
	) -> Self {
		Self {
			server_target,
			root_dir: root_dir.into(),
			dist_dir: dist_dir.into(),
			frontend_inputs,
			generated_typescript_file: generated_typescript_file.into(),
			generated_typescript_extra_source: String::new(),
			dev_watch,
		}
	}

	/// Attach raw supplemental generated TypeScript source.
	///
	/// Carries the already-rendered output of an app's
	/// [`crate::tsgen::TsDrafter`] (`AppConfig::ts_gen_config.extra_ts` in
	/// `vorma`, rendered through its `Display` impl) through the graph so
	/// `vorma-build` can append it to the generated `vorma.gen.ts` file
	/// alongside the route/resource-derived contracts.
	pub fn with_generated_typescript_extra_source(
		mut self,
		generated_typescript_extra_source: impl Into<String>,
	) -> Self {
		self.generated_typescript_extra_source = generated_typescript_extra_source.into();
		self
	}

	/// Cargo target for the application server.
	pub fn server_target(&self) -> &ServerBuildTarget {
		&self.server_target
	}

	/// Application root directory declaration.
	pub fn root_dir(&self) -> &str {
		&self.root_dir
	}

	/// Build output directory declaration.
	pub fn dist_dir(&self) -> &str {
		&self.dist_dir
	}

	/// Frontend build input declarations.
	pub fn frontend_inputs(&self) -> &FrontendBuildInputs {
		&self.frontend_inputs
	}

	/// Generated TypeScript output file declaration.
	pub fn generated_typescript_file(&self) -> &str {
		&self.generated_typescript_file
	}

	/// Raw supplemental generated TypeScript source.
	pub fn generated_typescript_extra_source(&self) -> &str {
		&self.generated_typescript_extra_source
	}

	/// Dev watcher declarations.
	pub fn dev_watch(&self) -> &DevWatchConfig {
		&self.dev_watch
	}
}

/// Cargo application server target.
///
/// Identifies which cargo package/binary `vorma-build` compiles as the
/// app server. See the maintainer reminders on the dev rebuild protocol:
/// this target is read once at session start and bound for that session —
/// a mid-session change fails the rebuild with a restart instruction
/// rather than silently compiling a different target.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ServerBuildTarget {
	cargo_package: String,
	cargo_bin: String,
}

impl ServerBuildTarget {
	/// Create a cargo server target.
	pub fn new(cargo_package: impl Into<String>, cargo_bin: impl Into<String>) -> Self {
		Self {
			cargo_package: cargo_package.into(),
			cargo_bin: cargo_bin.into(),
		}
	}

	/// Cargo package containing the server binary.
	pub fn cargo_package(&self) -> &str {
		&self.cargo_package
	}

	/// Cargo binary name for the server.
	pub fn cargo_bin(&self) -> &str {
		&self.cargo_bin
	}
}

/// Frontend files and directories consumed by the build compiler.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct FrontendBuildInputs {
	ui_variant: String,
	js_package_manager_base_cmd: String,
	js_package_manager_dir: String,
	vite_config_file: String,
	entry_file: String,
	public_static_source_dir: String,
	critical_css_file: String,
}

impl FrontendBuildInputs {
	/// Create frontend build inputs.
	pub fn new(
		ui_variant: impl Into<String>,
		js_package_manager_base_cmd: impl Into<String>,
		js_package_manager_dir: impl Into<String>,
		vite_config_file: impl Into<String>,
		entry_file: impl Into<String>,
		public_static_source_dir: impl Into<String>,
		critical_css_file: impl Into<String>,
	) -> Self {
		Self {
			ui_variant: ui_variant.into(),
			js_package_manager_base_cmd: js_package_manager_base_cmd.into(),
			js_package_manager_dir: js_package_manager_dir.into(),
			vite_config_file: vite_config_file.into(),
			entry_file: entry_file.into(),
			public_static_source_dir: public_static_source_dir.into(),
			critical_css_file: critical_css_file.into(),
		}
	}

	/// Stable UI adapter variant label.
	pub fn ui_variant(&self) -> &str {
		&self.ui_variant
	}

	/// JavaScript package-manager command prefix.
	pub fn js_package_manager_base_cmd(&self) -> &str {
		&self.js_package_manager_base_cmd
	}

	/// JavaScript package-manager working directory.
	pub fn js_package_manager_dir(&self) -> &str {
		&self.js_package_manager_dir
	}

	/// Vite config file.
	pub fn vite_config_file(&self) -> &str {
		&self.vite_config_file
	}

	/// Browser entry module file.
	pub fn entry_file(&self) -> &str {
		&self.entry_file
	}

	/// Public static source directory.
	pub fn public_static_source_dir(&self) -> &str {
		&self.public_static_source_dir
	}

	/// Critical CSS source file.
	pub fn critical_css_file(&self) -> &str {
		&self.critical_css_file
	}
}

/// Dev watcher source classification declarations.
///
/// Classifies watched file changes into two effects, checked against
/// [`Self::server_recompile_patterns`] and
/// [`Self::client_revalidate_patterns`] independently — a change can
/// require either, both, or (if it matches neither pattern list but is
/// still inside [`Self::watch_patterns`]) just a generation-epoch bump
/// with no further action. See the maintainer reminders on the dev
/// server's event-driven, least-necessary-work lifecycle: which effect
/// fires must never be inferred from anything beyond these declared
/// patterns.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct DevWatchConfig {
	watch_patterns: Vec<String>,
	server_recompile_patterns: Vec<String>,
	client_revalidate_patterns: Vec<String>,
}

impl DevWatchConfig {
	/// Create dev watcher declarations.
	pub fn new(
		watch_patterns: Vec<String>,
		server_recompile_patterns: Vec<String>,
		client_revalidate_patterns: Vec<String>,
	) -> Self {
		Self {
			watch_patterns,
			server_recompile_patterns,
			client_revalidate_patterns,
		}
	}

	/// Paths/globs watched by dev.
	pub fn watch_patterns(&self) -> &[String] {
		&self.watch_patterns
	}

	/// Paths/globs that require rebuilding the app server.
	pub fn server_recompile_patterns(&self) -> &[String] {
		&self.server_recompile_patterns
	}

	/// Paths/globs that require browser route revalidation.
	pub fn client_revalidate_patterns(&self) -> &[String] {
		&self.client_revalidate_patterns
	}
}

/// Complete public-API declaration input before graph normalization.
///
/// The mutable collection an app's routes accumulate into before
/// [`FrameworkGraph::compile`] validates and normalizes them once, all
/// together, so cross-declaration invariants (pattern collisions, type
/// reference resolution, resource/view specificity ties) can be checked
/// with full knowledge of every other declaration rather than one at a
/// time as they arrive. Nothing here is validated yet — a
/// `FrameworkDeclarations` can hold contradictory or malformed input; only
/// [`FrameworkGraph::compile`] enforces the graph's invariants.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct FrameworkDeclarations {
	config: FrameworkConfig,
	document: DocumentContract,
	type_defs: Vec<TypeDef>,
	middlewares: Vec<MiddlewareDeclaration>,
	views: Vec<ViewDeclaration>,
	resources: Vec<ResourceDeclaration>,
	static_assets: Vec<StaticAssetDeclaration>,
}

impl FrameworkDeclarations {
	/// Create declarations with explicit framework configuration.
	pub fn new(config: FrameworkConfig) -> Self {
		Self {
			config,
			document: DocumentContract::default(),
			type_defs: Vec::new(),
			middlewares: Vec::new(),
			views: Vec::new(),
			resources: Vec::new(),
			static_assets: Vec::new(),
		}
	}

	/// Add a middleware declaration.
	pub fn add_middleware(&mut self, middleware: MiddlewareDeclaration) {
		self.middlewares.push(middleware);
	}

	/// Add a view declaration.
	pub fn add_view(&mut self, view: ViewDeclaration) {
		self.views.push(view);
	}

	/// Add a resource declaration.
	pub fn add_resource(&mut self, resource: ResourceDeclaration) {
		self.resources.push(resource);
	}

	/// Add a static asset declaration.
	pub fn add_static_asset(&mut self, asset: StaticAssetDeclaration) {
		self.static_assets.push(asset);
	}

	/// Set the document contract.
	pub fn set_document(&mut self, document: DocumentContract) {
		self.document = document;
	}

	/// Add a TypeScript definition contract.
	pub fn add_type_def(&mut self, type_def: TypeDef) {
		self.type_defs.push(type_def);
	}

	/// Framework configuration.
	pub fn config(&self) -> &FrameworkConfig {
		&self.config
	}

	/// Document contract.
	pub fn document(&self) -> &DocumentContract {
		&self.document
	}

	/// TypeScript definition contracts.
	pub fn type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}

	/// Middleware declarations.
	pub fn middlewares(&self) -> &[MiddlewareDeclaration] {
		&self.middlewares
	}

	/// View declarations.
	pub fn views(&self) -> &[ViewDeclaration] {
		&self.views
	}

	/// Resource declarations.
	pub fn resources(&self) -> &[ResourceDeclaration] {
		&self.resources
	}

	/// Static asset declarations.
	pub fn static_assets(&self) -> &[StaticAssetDeclaration] {
		&self.static_assets
	}
}

/// Declared middleware input before graph normalization.
/*
Filters are optional and AND together: an empty `patterns` list means
"any route", an empty `methods` list means "any method". Each pattern is
the same grammar routes use (flat matching, no index segment).
*/
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MiddlewareDeclaration {
	pub(crate) handler_id: HandlerId,
	pub(crate) patterns: Vec<String>,
	pub(crate) methods: Vec<http::Method>,
}

impl MiddlewareDeclaration {
	/// Create an unrestricted middleware declaration.
	pub fn new(handler_id: HandlerId) -> Self {
		Self {
			handler_id,
			patterns: Vec::new(),
			methods: Vec::new(),
		}
	}

	/// Restrict to requests whose path matches one of these patterns.
	pub fn with_patterns<I>(mut self, patterns: I) -> Self
	where
		I: IntoIterator,
		I::Item: Into<String>,
	{
		self.patterns = patterns.into_iter().map(Into::into).collect();
		self
	}

	/// Restrict to requests using one of these methods.
	pub fn with_methods<I>(mut self, methods: I) -> Self
	where
		I: IntoIterator<Item = http::Method>,
	{
		self.methods = methods.into_iter().collect();
		self
	}

	/// Declared scope patterns (empty = any route).
	pub fn patterns(&self) -> &[String] {
		&self.patterns
	}

	/// Declared method filter (empty = any method).
	pub fn methods(&self) -> &[http::Method] {
		&self.methods
	}

	/// Declared handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Declared view input before graph normalization.
///
/// One layout-nesting-capable route: a pattern, the client module that
/// renders it, its query-string [`Self::search_schema`], and its
/// input/output [`RouteTypeContract`]. Views share one URL space with
/// GET/HEAD resources and are matched through `vorma_matcher`'s nested
/// matcher, so several may cover the same request path at once, from
/// outermost layout to innermost leaf — see [`crate::execution_plan`] for
/// how a request resolves the full chain.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewDeclaration {
	pub(crate) pattern: String,
	pub(crate) client_file: String,
	pub(crate) search_schema: Value,
	pub(crate) type_contract: RouteTypeContract,
	pub(crate) handler_id: HandlerId,
}

impl ViewDeclaration {
	/// Create a view declaration.
	pub fn new(
		pattern: impl Into<String>,
		client_file: impl Into<String>,
		search_schema: Value,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
	) -> Self {
		Self {
			pattern: pattern.into(),
			client_file: client_file.into(),
			search_schema,
			type_contract,
			handler_id,
		}
	}

	/// Declared route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Declared client module source path.
	pub fn client_file(&self) -> &str {
		&self.client_file
	}

	/// Declared search schema.
	///
	/// The default shape of this view's query-string input, derived from
	/// its input type's `Deserialize` implementation — what the browser
	/// runtime uses to parse a URL's search params into the view's typed
	/// input before a client-side navigation, and what ships to the
	/// browser as [`crate::wire::ViewPayload::search_schemas`].
	pub fn search_schema(&self) -> &Value {
		&self.search_schema
	}

	/// Declared route type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}

	/// Declared handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Declared resource input before graph normalization.
///
/// One data endpoint: an HTTP method, a pattern, an optional
/// [`ResourceKind`] override, an optional JSON input schema, and its
/// input/output [`RouteTypeContract`]. A `GET`/`HEAD` resource shares its
/// URL space with views (see [`ViewDeclaration`]); every other method's
/// resource space is exclusively its own.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceDeclaration {
	pub(crate) method: Method,
	pub(crate) pattern: String,
	pub(crate) kind: Option<ResourceKind>,
	pub(crate) input_schema: Option<Value>,
	pub(crate) type_contract: RouteTypeContract,
	pub(crate) handler_id: HandlerId,
}

impl ResourceDeclaration {
	/// Create a resource declaration.
	pub fn new(
		method: Method,
		pattern: impl Into<String>,
		kind: Option<ResourceKind>,
		input_schema: Option<Value>,
		type_contract: RouteTypeContract,
		handler_id: HandlerId,
	) -> Self {
		Self {
			method,
			pattern: pattern.into(),
			kind,
			input_schema,
			type_contract,
			handler_id,
		}
	}

	/// Declared HTTP method.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// Declared route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Explicit resource kind override.
	///
	/// `None` means this resource uses [`ResourceNode::default_kind`]'s
	/// method-derived classification with nothing further to declare —
	/// see [`ResourceKind`] for when an override is worth reaching for.
	pub fn kind(&self) -> Option<ResourceKind> {
		self.kind
	}

	/// Declared input schema.
	///
	/// The default-shaped JSON derived from this resource's input type,
	/// the same way [`ViewDeclaration::search_schema`] is derived for a
	/// view — `None` when the input type has no such shape to offer (for
	/// example `FormData`). Carried through the graph and
	/// [`crate::live_state`] protocol; unlike a view's search schema, it
	/// is not currently rendered into generated TypeScript output.
	pub fn input_schema(&self) -> Option<&Value> {
		self.input_schema.as_ref()
	}

	/// Declared route type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}

	/// Declared handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Declared static asset before graph normalization.
///
/// One content-hashed public file: where it lives in the project
/// ([`Self::source_path`]) and the public request path it is served at
/// after hashing ([`Self::public_path`]), which must fall inside the
/// configured public static base — see [`FrameworkConfig::public_static_base`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticAssetDeclaration {
	pub(crate) source_path: String,
	pub(crate) public_path: String,
}

impl StaticAssetDeclaration {
	/// Create a static asset declaration.
	pub fn new(source_path: impl Into<String>, public_path: impl Into<String>) -> Self {
		Self {
			source_path: source_path.into(),
			public_path: public_path.into(),
		}
	}

	/// Framework-relative source path.
	pub fn source_path(&self) -> &str {
		&self.source_path
	}

	/// Public request path.
	pub fn public_path(&self) -> &str {
		&self.public_path
	}
}

/// Normalized framework configuration.
///
/// [`FrameworkConfig`]'s validated, canonical counterpart — for example
/// [`Self::public_static_base`] is guaranteed to carry both a leading and
/// trailing slash here, where the declaration-time value was raw
/// app-authored text.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GraphConfig {
	public_static_base: String,
	build_inputs: Option<BuildInputConfig>,
}

impl GraphConfig {
	/// Normalized public static base.
	pub fn public_static_base(&self) -> &str {
		&self.public_static_base
	}

	/// Build/dev inputs projected from the graph, when configured.
	pub fn build_inputs(&self) -> Option<&BuildInputConfig> {
		self.build_inputs.as_ref()
	}
}

/// Canonical middleware graph node.
///
/// [`MiddlewareDeclaration`]'s normalized counterpart — same fields,
/// carried through graph compilation once every declared scope pattern
/// has been confirmed to parse.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MiddlewareNode {
	patterns: Vec<String>,
	methods: Vec<http::Method>,
	handler_id: HandlerId,
}

impl MiddlewareNode {
	/// Scope patterns (empty = any route).
	pub fn patterns(&self) -> &[String] {
		&self.patterns
	}

	/// Method filter (empty = any method).
	pub fn methods(&self) -> &[http::Method] {
		&self.methods
	}

	/// Runtime handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Canonical view graph node.
///
/// [`ViewDeclaration`]'s normalized counterpart, with
/// [`Self::parent_patterns`]/[`Self::params`] derived once at compile
/// time rather than re-derived on every request. Every view's chain of
/// ancestors is fully determined by route shape alone — there is no
/// separate "declare your parent" step; a shorter pattern that this
/// view's path shape falls under is automatically its ancestor.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewNode {
	pattern: String,
	parent_patterns: Vec<String>,
	params: Vec<String>,
	client_file: String,
	search_schema: Value,
	type_contract: RouteTypeContract,
	handler_id: HandlerId,
}

impl ViewNode {
	/// Canonical route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Canonical parent route patterns from outermost to innermost.
	///
	/// Derived from route shape alone: every other declared view pattern
	/// whose scope contains this one (a non-splat-terminated pattern that
	/// this view's path shape falls under), sorted shallowest-first.
	/// `vorma`'s runtime dispatch relies on this ordering directly for
	/// [`crate::execution_plan::ExecutionPlan::match_views`], so it is
	/// computed once here rather than re-derived per request.
	pub fn parent_patterns(&self) -> &[String] {
		&self.parent_patterns
	}

	/// Dynamic parameter names in declaration order.
	pub fn params(&self) -> &[String] {
		&self.params
	}

	/// Client module source path.
	pub fn client_file(&self) -> &str {
		&self.client_file
	}

	/// Search schema projected for runtime payloads.
	pub fn search_schema(&self) -> &Value {
		&self.search_schema
	}

	/// Route type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}

	/// Runtime view handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Canonical resource graph node.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceNode {
	method: Method,
	pattern: String,
	kind: Option<ResourceKind>,
	default_kind: ResourceKind,
	input_schema: Option<Value>,
	type_contract: RouteTypeContract,
	params: Vec<String>,
	handler_id: HandlerId,
}

impl ResourceNode {
	/// HTTP method.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// Canonical route pattern.
	pub fn pattern(&self) -> &str {
		&self.pattern
	}

	/// Explicit resource kind override.
	pub fn kind(&self) -> Option<ResourceKind> {
		self.kind
	}

	/// Default generated-client resource kind.
	///
	/// `GET`/`HEAD` default to [`ResourceKind::Query`]; every other
	/// method defaults to [`ResourceKind::Mutation`]. Generated
	/// TypeScript only emits an explicit `kind` when
	/// [`Self::kind`] disagrees with this default — see [`ResourceKind`].
	pub fn default_kind(&self) -> ResourceKind {
		self.default_kind
	}

	/// Resource input schema, when its input type has one.
	///
	/// See [`crate::framework_graph::ResourceDeclaration::input_schema`]
	/// for how this is derived and its current (not yet consumed by
	/// generated TypeScript) status.
	pub fn input_schema(&self) -> Option<&Value> {
		self.input_schema.as_ref()
	}

	/// Route type contract.
	pub fn type_contract(&self) -> &RouteTypeContract {
		&self.type_contract
	}

	/// Dynamic parameter names in declaration order.
	pub fn params(&self) -> &[String] {
		&self.params
	}

	/// Runtime resource handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.handler_id
	}
}

/// Canonical static asset graph node.
///
/// [`StaticAssetDeclaration`]'s normalized counterpart, guaranteed at this
/// point to sit inside the graph's configured public static base.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticAssetNode {
	pub(crate) source_path: String,
	pub(crate) public_path: String,
}

impl StaticAssetNode {
	/// Framework-relative source path.
	pub fn source_path(&self) -> &str {
		&self.source_path
	}

	/// Public request path.
	pub fn public_path(&self) -> &str {
		&self.public_path
	}
}

/// Canonical framework graph.
///
/// The validated, normalized shape of an entire app — every route,
/// middleware, static asset, and document default, cross-checked for
/// consistency as a whole (see the module docs for the exact invariant
/// list). Every later stage — [`crate::execution_plan`], generated
/// TypeScript, the committed [`crate::runtime_manifest`], the
/// [`crate::live_state`] wire protocol — starts from one of these, never
/// from raw declarations, so downstream code can assume a graph's
/// invariants hold without re-checking them.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FrameworkGraph {
	config: GraphConfig,
	document: DocumentContract,
	type_defs: Vec<TypeDef>,
	middlewares: Vec<MiddlewareNode>,
	views: Vec<ViewNode>,
	resources: Vec<ResourceNode>,
	static_assets: Vec<StaticAssetNode>,
}

impl FrameworkGraph {
	/// Normalize declarations into a canonical graph.
	///
	/// See the module docs for the full list of invariants this checks,
	/// run in a fixed order and returning on the first violation found —
	/// a declaration set with more than one problem reports only the
	/// first, named precisely enough by its [`GraphError`] variant to fix
	/// without further investigation.
	pub fn compile(declarations: FrameworkDeclarations) -> Result<Self, GraphError> {
		let public_static_base = normalize_base_path(
			declarations.config.public_static_base,
			ConfigField::PublicStaticBase,
		)?;
		let config = GraphConfig {
			public_static_base,
			build_inputs: declarations.config.build_inputs,
		};
		validate_middleware_shapes(&declarations.middlewares)?;
		let all_view_patterns = declarations
			.views
			.iter()
			.map(|view| view.pattern.clone())
			.collect::<Vec<_>>();
		let view_shapes = validate_view_shapes(&declarations.views, &all_view_patterns)?;
		let (resource_shapes, resource_matchers) =
			validate_resource_shapes(&declarations.resources)?;
		validate_resource_reachability(
			&declarations.views,
			&declarations.resources,
			&resource_matchers,
			&config.public_static_base,
		)?;
		let type_defs = validate_type_contracts(
			&declarations.type_defs,
			&declarations.views,
			&declarations.resources,
		)?;
		let static_assets = validate_static_assets(&config, declarations.static_assets)?;
		let middlewares = declarations
			.middlewares
			.into_iter()
			.map(|middleware| MiddlewareNode {
				patterns: middleware.patterns,
				methods: middleware.methods,
				handler_id: middleware.handler_id,
			})
			.collect::<Vec<_>>();
		Ok(Self {
			config,
			document: declarations.document,
			type_defs,
			views: declarations
				.views
				.into_iter()
				.zip(view_shapes)
				.map(|(view, shape)| ViewNode {
					pattern: view.pattern,
					parent_patterns: shape.parent_patterns,
					params: shape.params,
					client_file: view.client_file,
					search_schema: view.search_schema,
					type_contract: view.type_contract,
					handler_id: view.handler_id,
				})
				.collect(),
			resources: declarations
				.resources
				.into_iter()
				.zip(resource_shapes)
				.map(|(resource, shape)| ResourceNode {
					method: resource.method,
					pattern: resource.pattern,
					kind: resource.kind,
					default_kind: shape.default_kind,
					input_schema: resource.input_schema,
					type_contract: resource.type_contract,
					params: shape.params,
					handler_id: resource.handler_id,
				})
				.collect(),
			static_assets,
			middlewares,
		})
	}

	/// Normalized framework configuration.
	pub fn config(&self) -> &GraphConfig {
		&self.config
	}

	/// Canonical document contract.
	pub fn document(&self) -> &DocumentContract {
		&self.document
	}

	/// Canonical TypeScript definition contracts.
	pub fn type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}

	/// Canonical middleware nodes.
	pub fn middlewares(&self) -> &[MiddlewareNode] {
		&self.middlewares
	}

	/// Canonical view nodes.
	pub fn views(&self) -> &[ViewNode] {
		&self.views
	}

	/// Canonical resource nodes.
	pub fn resources(&self) -> &[ResourceNode] {
		&self.resources
	}

	/// Canonical static asset nodes.
	pub fn static_assets(&self) -> &[StaticAssetNode] {
		&self.static_assets
	}
}

/// Framework graph normalization error.
///
/// Returned by [`FrameworkGraph::compile`] — one variant per invariant
/// listed in the module docs, each naming the exact declaration and
/// reason so the underlying app-authoring mistake is fixable from the
/// error alone.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum GraphError {
	/// Framework configuration is invalid.
	InvalidConfig {
		/// Rejected configuration field.
		field: &'static str,
		/// Rejected configuration value.
		value: String,
	},
	/// A middleware declaration is invalid.
	InvalidMiddlewarePattern {
		/// Rejected middleware scope pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// A view declaration is invalid.
	InvalidViewPattern {
		/// Rejected view pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// A resource declaration is invalid.
	InvalidResourcePattern {
		/// Rejected resource pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// Duplicate view pattern.
	DuplicateViewPattern {
		/// Repeated view pattern.
		pattern: String,
	},
	/// Duplicate or shape-colliding resource pattern for one method.
	DuplicateResourcePattern {
		/// HTTP method whose resource table collided.
		method: Method,
		/// Repeated or colliding resource pattern.
		pattern: String,
	},
	/// View client module path is invalid.
	InvalidClientFile {
		/// Rejected client module path.
		path: String,
	},
	/// Type definition key was declared more than once.
	DuplicateTypeKey {
		/// Repeated type identity key.
		key: String,
	},
	/// Type definition TypeScript name was declared more than once by
	/// unrelated type definitions.
	///
	/// Not raised when the two colliding definitions are recognized as one
	/// `TsGen`-derived Rust type's Serialize-phase and Deserialize-phase
	/// shapes (see [`crate::contracts::TypeDef::classify_shared_name_with`]) —
	/// a shape-equivalent pair collapses into one definition instead; a
	/// shape-divergent pair is [`Self::DivergentTypePhaseShapes`], not this.
	DuplicateTypeName {
		/// Repeated exported TypeScript name.
		name: String,
	},
	/// One Rust type's Serialize-phase and Deserialize-phase shapes
	/// disagree.
	///
	/// Raised only for a type reached as both a route input and a route
	/// output (or otherwise registered under both phases) whose two
	/// generated shapes are not structurally equivalent — the natural
	/// cause is a field that is optional under one phase and required
	/// under the other (`#[serde(default)]` makes a field optional on
	/// Deserialize only; `#[serde(skip_serializing_if = "...")]` makes a
	/// field optional on Serialize only). Fix by giving each phase its own
	/// Rust type instead of sharing one across both.
	DivergentTypePhaseShapes {
		/// Exported TypeScript name shared by both phases.
		name: String,
		/// Serialize-phase identity key.
		serialize_key: String,
		/// Deserialize-phase identity key.
		deserialize_key: String,
	},
	/// Type definition facts were invalid.
	InvalidTypeDef {
		/// Type identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// Rejection reason.
		reason: &'static str,
	},
	/// A named type reference pointed at no declared type definition.
	UnknownNamedType {
		/// Missing type identity key.
		key: String,
		/// Missing exported TypeScript name.
		name: String,
	},
	/// Static asset declaration is invalid.
	InvalidStaticAsset {
		/// Rejected static asset path.
		path: String,
	},
	/// Matcher configuration is invalid.
	InvalidMatcherOptions {
		/// Matcher rejection reason.
		reason: String,
	},
	/// A GET/HEAD resource and a view tie in specificity on a shared path.
	ResourceViewSpecificityTie {
		/// Resource HTTP method.
		method: Method,
		/// Normalized resource pattern.
		resource_pattern: String,
		/// Normalized view pattern of identical specificity.
		view_pattern: String,
		/// Concrete request path both routes match.
		example_path: String,
	},
	/// A GET/HEAD resource shares a request path with the public static base.
	ResourceInsidePublicStaticBase {
		/// Resource HTTP method.
		method: Method,
		/// Normalized resource pattern.
		resource_pattern: String,
		/// Concrete request path inside the public static base space.
		example_path: String,
	},
}

impl std::fmt::Display for GraphError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidConfig { field, value } => {
				write!(f, "invalid framework config {field} value {value:?}")
			}
			Self::InvalidMiddlewarePattern { pattern, reason } => {
				write!(f, "invalid middleware pattern {pattern:?}: {reason}")
			}
			Self::InvalidViewPattern { pattern, reason } => {
				write!(f, "invalid view pattern {pattern:?}: {reason}")
			}
			Self::InvalidResourcePattern { pattern, reason } => {
				write!(f, "invalid resource pattern {pattern:?}: {reason}")
			}
			Self::DuplicateViewPattern { pattern } => {
				write!(f, "duplicate view pattern {pattern:?}")
			}
			Self::DuplicateResourcePattern { method, pattern } => {
				write!(f, "duplicate {method} resource pattern {pattern:?}")
			}
			Self::InvalidClientFile { path } => {
				write!(f, "invalid view client module path {path:?}")
			}
			Self::DuplicateTypeKey { key } => write!(f, "duplicate type key {key:?}"),
			Self::DuplicateTypeName { name } => {
				write!(f, "duplicate TypeScript type name {name:?}")
			}
			Self::DivergentTypePhaseShapes {
				name,
				serialize_key,
				deserialize_key,
			} => {
				write!(
					f,
					"type {name:?} is used as both a route input and a route output, but its serialized (output) and deserialized (input) shapes differ (for example, a #[serde(default)] field is optional on input but required on output) — use two distinct Rust types, one for each shape, instead of sharing {name:?} across both (serialize key {serialize_key:?}, deserialize key {deserialize_key:?})"
				)
			}
			Self::InvalidTypeDef { key, name, reason } => {
				write!(f, "invalid type definition {key:?} / {name:?}: {reason}")
			}
			Self::UnknownNamedType { key, name } => {
				write!(f, "unknown named type reference {key:?} / {name:?}")
			}
			Self::InvalidStaticAsset { path } => {
				write!(f, "invalid static asset path {path:?}")
			}
			Self::InvalidMatcherOptions { reason } => {
				write!(f, "invalid matcher options: {reason}")
			}
			Self::ResourceViewSpecificityTie {
				method,
				resource_pattern,
				view_pattern,
				example_path,
			} => {
				write!(
					f,
					"{method} resource {resource_pattern:?} and view {view_pattern:?} both match {example_path:?} at identical specificity, so neither can win; make one more specific"
				)
			}
			Self::ResourceInsidePublicStaticBase {
				method,
				resource_pattern,
				example_path,
			} => {
				write!(
					f,
					"{method} resource {resource_pattern:?} matches {example_path:?} inside the public static base, which is reserved for static assets"
				)
			}
		}
	}
}

impl std::error::Error for GraphError {}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum ConfigField {
	PublicStaticBase,
}

impl ConfigField {
	pub(crate) fn as_str(self) -> &'static str {
		match self {
			Self::PublicStaticBase => "public_static_base",
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::contracts::{FieldDef, TypeRefContract};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	#[test]
	fn graph_derives_route_facts_once() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("root.middleware")));
		declarations.add_middleware(
			MiddlewareDeclaration::new(handler_id("users.middleware"))
				.with_patterns(["/users", "/users/*"]),
		);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root.view"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/users/:id/*",
			"user.tsx",
			serde_json::json!({"id": "s"}),
			route_type_contract(),
			handler_id("user.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/users/:id",
			None,
			Some(serde_json::json!({"body": "user"})),
			route_type_contract(),
			handler_id("user.resource"),
		));
		declarations.add_static_asset(StaticAssetDeclaration::new(
			"app.css",
			"/static/app.hash.css",
		));

		let graph = FrameworkGraph::compile(declarations).unwrap();

		assert_eq!(graph.config().public_static_base(), "/static/");
		assert_eq!(graph.views()[1].parent_patterns(), ["/"]);
		assert_eq!(graph.views()[1].params(), ["id"]);
		assert_eq!(graph.middlewares()[1].patterns(), ["/users", "/users/*"]);
		assert_eq!(graph.resources()[0].default_kind(), ResourceKind::Mutation);
		assert_eq!(graph.resources()[0].params(), ["id"]);
		assert_eq!(
			graph.static_assets()[0].public_path(),
			"/static/app.hash.css"
		);
	}

	#[test]
	fn graph_orders_view_parents_from_outermost_to_innermost() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/users/:id/profile",
			"profile.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("profile"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/users/:id",
			"user.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("user"),
		));

		let graph = FrameworkGraph::compile(declarations).unwrap();

		assert_eq!(graph.views()[0].parent_patterns(), ["/", "/users/:id"]);
	}

	#[test]
	fn graph_derives_view_parent_relationships_from_route_shape() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/users/:id/profile",
			"profile.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("profile"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/users/:user_id",
			"user.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("user"),
		));

		let graph = FrameworkGraph::compile(declarations).unwrap();

		assert_eq!(graph.views()[0].parent_patterns(), ["/users/:user_id"]);
	}

	#[test]
	fn graph_rejects_colliding_resource_shapes_by_method() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/users/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("one"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/users/:user_id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("two"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::DuplicateResourcePattern {
				method: Method::GET,
				..
			}
		));
	}

	#[test]
	fn graph_reports_invalid_resource_patterns_as_invalid_patterns() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"relative/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("bad"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(error, GraphError::InvalidResourcePattern { .. }));
	}

	#[test]
	fn graph_reports_colliding_view_shapes_as_duplicate_views() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/users/:id",
			"user.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("user"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/users/:slug",
			"user_by_slug.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("user.slug"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(error, GraphError::DuplicateViewPattern { .. }));
	}

	/*
	Overlap between views and GET resources is the normal condition,
	adjudicated by specificity exactly as it is between two views: the
	static resource owns its one path, the dynamic view owns the rest.
	*/
	#[test]
	fn graph_allows_get_resources_that_specificity_can_order_against_views() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/:story_id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/s/export",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("export.resource"),
		));

		FrameworkGraph::compile(declarations).unwrap();
	}

	#[test]
	fn graph_allows_get_resources_under_view_splats() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/bob/*",
			"bob.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("bob.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/bob/sally",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("sally.resource"),
		));

		FrameworkGraph::compile(declarations).unwrap();
	}

	#[test]
	fn graph_allows_get_resources_alongside_a_root_catch_all_view() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root.view"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/*",
			"not_found.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("not.found.view"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/about",
			"about.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("about.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/items",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("items.resource"),
		));

		let graph = FrameworkGraph::compile(declarations).unwrap();

		assert_eq!(graph.views()[1].parent_patterns(), ["/"]);
		assert_eq!(graph.views()[2].parent_patterns(), ["/"]);
	}

	#[test]
	fn graph_rejects_specificity_ties_between_views_and_get_resources() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/:story_id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/s/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("story.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		match error {
			GraphError::ResourceViewSpecificityTie {
				method,
				resource_pattern,
				view_pattern,
				example_path,
			} => {
				assert_eq!(method, Method::GET);
				assert_eq!(resource_pattern, "/s/:id");
				assert_eq!(view_pattern, "/s/:story_id");
				assert!(example_path.starts_with("/s/"));
			}
			other => panic!("expected ResourceViewSpecificityTie, got {other:?}"),
		}
	}

	#[test]
	fn graph_rejects_identical_view_and_get_resource_patterns() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/health",
			"health.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("health.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/health",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("health.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::ResourceViewSpecificityTie {
				method: Method::GET,
				..
			}
		));
	}

	/*
	The shadowing axis is the HTTP method (dispatch), never ResourceKind
	(a generated-client classification): views serve GET/HEAD requests,
	so only GET/HEAD-method resources can collide with them.
	*/
	#[test]
	fn graph_allows_non_get_head_method_resources_at_view_urls() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/:story_id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/s/:story_id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("story.resource"),
		));
		// A POST-method query collides no more than any other POST.
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/s/:story_id/related",
			Some(ResourceKind::Query),
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("related.resource"),
		));

		FrameworkGraph::compile(declarations).unwrap();
	}

	#[test]
	fn graph_rejects_get_method_specificity_ties_regardless_of_kind() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/s/:story_id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/s/:sid",
			Some(ResourceKind::Mutation),
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("refresh.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::ResourceViewSpecificityTie {
				method: Method::GET,
				..
			}
		));
	}

	#[test]
	fn graph_rejects_head_resource_view_specificity_ties() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/docs",
			"docs.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("docs.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::HEAD,
			"/docs",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("docs.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::ResourceViewSpecificityTie {
				method: Method::HEAD,
				..
			}
		));
	}

	#[test]
	fn graph_rejects_get_resources_inside_public_static_base() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/static/data.json",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("data.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		match error {
			GraphError::ResourceInsidePublicStaticBase {
				method,
				resource_pattern,
				example_path,
			} => {
				assert_eq!(method, Method::GET);
				assert_eq!(resource_pattern, "/static/data.json");
				assert_eq!(example_path, "/static/data.json");
			}
			other => panic!("expected ResourceInsidePublicStaticBase, got {other:?}"),
		}
	}

	#[test]
	fn graph_with_root_static_base_opts_out_of_the_reserved_prefix() {
		let mut declarations = FrameworkDeclarations::new(FrameworkConfig::new("/"));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/items",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("items.resource"),
		));

		FrameworkGraph::compile(declarations).unwrap();
	}

	#[test]
	fn graph_rejects_invalid_view_client_file_paths() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"../root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(error, GraphError::InvalidClientFile { .. }));
	}

	#[test]
	fn graph_rejects_static_assets_outside_public_capability_base() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_static_asset(StaticAssetDeclaration::new("app.css", "/other/app.css"));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(error, GraphError::InvalidStaticAsset { .. }));
	}

	#[test]
	fn graph_rejects_unresolved_named_type_references() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			RouteTypeContract::new(
				TypeRefContract::Named {
					key: "Missing".to_owned(),
					name: "Missing".to_owned(),
				},
				TypeRefContract::Unknown,
			),
			handler_id("root.view"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::UnknownNamedType { key, name }
				if key == "Missing" && name == "Missing"
		));
	}

	#[test]
	fn graph_rejects_duplicate_type_keys_and_names() {
		let mut duplicate_keys = FrameworkDeclarations::default();
		duplicate_keys.add_type_def(TypeDef::Alias {
			key: "User".to_owned(),
			name: "User".to_owned(),
			target: TypeRefContract::String,
		});
		duplicate_keys.add_type_def(TypeDef::Alias {
			key: "User".to_owned(),
			name: "Account".to_owned(),
			target: TypeRefContract::String,
		});
		let mut duplicate_names = FrameworkDeclarations::default();
		duplicate_names.add_type_def(TypeDef::Alias {
			key: "User".to_owned(),
			name: "User".to_owned(),
			target: TypeRefContract::String,
		});
		duplicate_names.add_type_def(TypeDef::Alias {
			key: "Account".to_owned(),
			name: "User".to_owned(),
			target: TypeRefContract::String,
		});

		let duplicate_key_error = FrameworkGraph::compile(duplicate_keys).unwrap_err();
		let duplicate_name_error = FrameworkGraph::compile(duplicate_names).unwrap_err();

		assert!(matches!(
			duplicate_key_error,
			GraphError::DuplicateTypeKey { key } if key == "User"
		));
		assert!(matches!(
			duplicate_name_error,
			GraphError::DuplicateTypeName { name } if name == "User"
		));
	}

	/*
	Mirrors the real `TsGen` derive's key convention
	(`{module_path}::{TypeIdent}::serialize`/`::deserialize`) precisely,
	since `TypeDef::classify_shared_name_with`'s pairing test depends on it
	— these tests would not exercise the real boundary if they used
	arbitrary key strings instead.
	*/
	fn shared_user_serialize_key() -> String {
		"app::model::SharedUser::serialize".to_owned()
	}

	fn shared_user_deserialize_key() -> String {
		"app::model::SharedUser::deserialize".to_owned()
	}

	// A named `address` field, present identically on both phases, proving
	// the nested-`Named`-ref comparison ignores the nested field's own
	// phase-suffixed key (which legitimately differs) and compares only
	// its name.
	fn shared_user_address_field(phase_key: &str) -> FieldDef {
		FieldDef::new(
			"address",
			TypeRefContract::Named {
				key: format!("app::model::Address::{phase_key}"),
				name: "Address".to_owned(),
			},
			false,
		)
	}

	#[test]
	fn graph_collapses_shared_type_used_as_both_route_input_and_output() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_serialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![
				FieldDef::new("name", TypeRefContract::String, false),
				shared_user_address_field("serialize"),
			],
		});
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_deserialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![
				FieldDef::new("name", TypeRefContract::String, false),
				shared_user_address_field("deserialize"),
			],
		});
		// `Address` is itself `TsGen`-derived and nested inside
		// `SharedUser`, so it is ALSO reached at both phases and must
		// ALSO collapse — proving the fix handles a nested shared type,
		// not only a top-level one.
		declarations.add_type_def(TypeDef::Record {
			key: "app::model::Address::serialize".to_owned(),
			name: "Address".to_owned(),
			fields: vec![FieldDef::new("city", TypeRefContract::String, false)],
		});
		declarations.add_type_def(TypeDef::Record {
			key: "app::model::Address::deserialize".to_owned(),
			name: "Address".to_owned(),
			fields: vec![FieldDef::new("city", TypeRefContract::String, false)],
		});
		// Reached as a route OUTPUT (Serialize phase) here...
		declarations.add_view(ViewDeclaration::new(
			"/profile",
			"profile.tsx",
			serde_json::json!({}),
			RouteTypeContract::new(
				TypeRefContract::Unit,
				TypeRefContract::Named {
					key: shared_user_serialize_key(),
					name: "SharedUser".to_owned(),
				},
			),
			handler_id("profile.view"),
		));
		// ...and as a route INPUT (Deserialize phase) here — the exact
		// shape P013 reproduced as `DuplicateTypeName`.
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/profile",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(
				TypeRefContract::Named {
					key: shared_user_deserialize_key(),
					name: "SharedUser".to_owned(),
				},
				TypeRefContract::Unknown,
			),
			handler_id("profile.resource"),
		));

		let graph = FrameworkGraph::compile(declarations).unwrap();

		let shared_user_defs = graph
			.type_defs()
			.iter()
			.filter(|type_def| type_def.name() == "SharedUser")
			.count();
		assert_eq!(
			shared_user_defs, 1,
			"a structurally shared type used as both a route input and output \
			 must export exactly one TypeScript type definition"
		);
		let address_defs = graph
			.type_defs()
			.iter()
			.filter(|type_def| type_def.name() == "Address")
			.count();
		assert_eq!(
			address_defs, 1,
			"a nested shared type reached at both phases through the outer \
			 shared type must also collapse to exactly one definition"
		);
	}

	#[test]
	fn graph_rejects_structurally_divergent_shared_type_phases_with_teaching_error() {
		let mut declarations = FrameworkDeclarations::default();
		// The natural divergence case: `#[serde(default)]` makes `nickname`
		// optional on Deserialize but the field is required on Serialize —
		// the two phases render genuinely different TypeScript shapes even
		// though they share a name, by design.
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_serialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![FieldDef::new("nickname", TypeRefContract::String, false)],
		});
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_deserialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![FieldDef::new("nickname", TypeRefContract::String, true)],
		});
		declarations.add_view(ViewDeclaration::new(
			"/profile",
			"profile.tsx",
			serde_json::json!({}),
			RouteTypeContract::new(
				TypeRefContract::Unit,
				TypeRefContract::Named {
					key: shared_user_serialize_key(),
					name: "SharedUser".to_owned(),
				},
			),
			handler_id("profile.view"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/profile",
			None,
			Some(serde_json::json!({})),
			RouteTypeContract::new(
				TypeRefContract::Named {
					key: shared_user_deserialize_key(),
					name: "SharedUser".to_owned(),
				},
				TypeRefContract::Unknown,
			),
			handler_id("profile.resource"),
		));

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		let GraphError::DivergentTypePhaseShapes {
			name,
			serialize_key,
			deserialize_key,
		} = error
		else {
			panic!("expected DivergentTypePhaseShapes, got {error:?}");
		};
		assert_eq!(name, "SharedUser");
		assert_eq!(serialize_key, shared_user_serialize_key());
		assert_eq!(deserialize_key, shared_user_deserialize_key());
		let message = GraphError::DivergentTypePhaseShapes {
			name,
			serialize_key,
			deserialize_key,
		}
		.to_string();
		assert!(
			message.contains("use two distinct Rust types"),
			"error must teach the two-distinct-Rust-types resolution: {message}"
		);
		assert!(
			message.contains("SharedUser"),
			"error must name the colliding type: {message}"
		);
	}

	// Defensive boundary test: even if two keys happen to look like a
	// phase pair by string shape, a genuinely different declared shape
	// under a THIRD, unrelated key claiming the same name is still a
	// plain `DuplicateTypeName` — dedup never chains through more than one
	// recognized pairing per name.
	#[test]
	fn graph_rejects_unrelated_type_claiming_a_name_already_used_by_a_phase_pair() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_serialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![FieldDef::new("name", TypeRefContract::String, false)],
		});
		declarations.add_type_def(TypeDef::Record {
			key: shared_user_deserialize_key(),
			name: "SharedUser".to_owned(),
			fields: vec![FieldDef::new("name", TypeRefContract::String, false)],
		});
		declarations.add_type_def(TypeDef::Alias {
			key: "app::other::UnrelatedSharedUser".to_owned(),
			name: "SharedUser".to_owned(),
			target: TypeRefContract::String,
		});

		let error = FrameworkGraph::compile(declarations).unwrap_err();

		assert!(matches!(
			error,
			GraphError::DuplicateTypeName { name } if name == "SharedUser"
		));
	}

	#[test]
	fn graph_rejects_invalid_type_definition_shapes() {
		let mut duplicate_fields = FrameworkDeclarations::default();
		duplicate_fields.add_type_def(TypeDef::Record {
			key: "User".to_owned(),
			name: "User".to_owned(),
			fields: vec![
				FieldDef::new("name", TypeRefContract::String, false),
				FieldDef::new("name", TypeRefContract::String, false),
			],
		});

		let error = FrameworkGraph::compile(duplicate_fields).unwrap_err();

		assert!(matches!(error, GraphError::InvalidTypeDef { .. }));
	}
}
