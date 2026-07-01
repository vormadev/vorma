//! Canonical private framework graph.

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
#[derive(Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct HandlerId(String);

impl HandlerId {
	/// Create a validated handler identifier.
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
	pub fn kind(&self) -> Option<ResourceKind> {
		self.kind
	}

	/// Declared input schema.
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
	pub fn default_kind(&self) -> ResourceKind {
		self.default_kind
	}

	/// Resource input schema projected for generated API contracts.
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
		validate_type_contracts(
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
			type_defs: declarations.type_defs,
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
	/// Type definition TypeScript name was declared more than once.
	DuplicateTypeName {
		/// Repeated exported TypeScript name.
		name: String,
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
