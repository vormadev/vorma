//! Live build-state protocol: the app binary's graph emission consumed by dev epoch rebuilds.
//!
//! Framework-integration surface and **frozen** — the wire protocol
//! between a running (or freshly rebuilt) app binary and the `vorma-build`
//! process orchestrating a dev session. In dev mode, the app server binary
//! is invoked with
//! [`LIVE_BUILD_STATE_ENV_KEY`](crate::live_state::LIVE_BUILD_STATE_ENV_KEY)
//! set and, instead of serving requests, emits its compiled
//! [`crate::framework_graph::FrameworkGraph`] as JSON
//! ([`LiveBuildState`](crate::live_state::LiveBuildState)) so the build
//! side can regenerate manifests and TypeScript from the app's *actual
//! current* declared graph — the same binary the dev session is about to
//! serve — rather than re-deriving that shape by some other means. See
//! the maintainer reminders: during dev builds the app-server binary is
//! the only app-linked binary, compiled once per iteration, and this
//! protocol is how its own graph gets back out of it.
//!
//! [`LIVE_BUILD_STATE_PROTOCOL`](crate::live_state::LIVE_BUILD_STATE_PROTOCOL)
//! exists because the app binary and the build process can independently
//! drift out of version lockstep (a stale build binary against a
//! freshly-rebuilt app, or vice versa); an app binary that emitted this
//! JSON before the version field existed deserializes as protocol `0`
//! and is rejected by the same version check that catches any other
//! mismatch — never a generic parse failure that would leave a developer
//! guessing why their dev session broke.

use crate::contracts::{DocumentContract, RouteTypeContract, TypeDef};
use crate::framework_graph::{
	BuildInputConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph, GraphError,
	HandlerId, HandlerIdError, MiddlewareDeclaration, ResourceDeclaration, ResourceKind,
	ServerBuildTarget, StaticAssetDeclaration, ViewDeclaration,
};
use http::Method;
use serde::{Deserialize, Serialize};

/// Live build-state wire protocol version.
/*
The `vorma`/`vorma-build` pair ships version-locked, so this only moves when
the live-state JSON shape changes incompatibly. Its job is to turn "an app
binary built against a different vorma answered the build" into a precise
error instead of a serde parse failure. Emitters built before the field
existed deserialize as protocol 0 and are rejected by the same check.
*/
pub const LIVE_BUILD_STATE_PROTOCOL: u32 = 3;

/// Environment key requesting live build-state JSON from the build entry.
pub const LIVE_BUILD_STATE_ENV_KEY: &str = "__VORMA_LIVE_BUILD_STATE";
/// Environment value requesting live build-state JSON from the build entry.
pub const LIVE_BUILD_STATE_ENV_VALUE: &str = "1";

/// Live build-state facts emitted by a build-entry process in dev mode.
///
/// The wire shape of one live-state emission: a protocol version (see the
/// module docs), a serialized [`crate::framework_graph::FrameworkGraph`],
/// and a precomputed root-document hash source string used to detect
/// whether the app's document shell itself changed between rebuilds.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct LiveBuildState {
	#[serde(default)]
	protocol: u32,
	graph: LiveFrameworkGraph,
	root_document_hash_source: String,
}

impl LiveBuildState {
	/// Build live-state facts from graph and precomputed root document hash source.
	pub fn from_graph(graph: FrameworkGraph, root_document_hash_source: impl Into<String>) -> Self {
		Self {
			protocol: LIVE_BUILD_STATE_PROTOCOL,
			graph: LiveFrameworkGraph::from_framework_graph(&graph),
			root_document_hash_source: root_document_hash_source.into(),
		}
	}

	/// Recompile the canonical framework graph from serialized live-state declarations.
	pub fn framework_graph(&self) -> Result<FrameworkGraph, LiveBuildStateError> {
		self.graph.to_framework_graph()
	}

	/// Declared app-server cargo target carried by this live state, if any.
	pub fn server_build_target(&self) -> Option<&ServerBuildTarget> {
		self.graph
			.build_inputs
			.as_ref()
			.map(BuildInputConfig::server_target)
	}

	/// Root document hash source for this live-state generation.
	///
	/// Not itself a hash — the build side blake3-hashes this string to
	/// produce [`crate::runtime_manifest::RuntimeManifest::root_document_shell_hash`],
	/// the value the committed manifest actually carries. Precomputing
	/// the source string here (rather than re-deriving it from the app's
	/// document builder later) keeps the app binary as the single source
	/// of truth for what its own document shell renders as.
	pub fn root_document_hash_source(&self) -> &str {
		&self.root_document_hash_source
	}

	/// Encode live-state facts as JSON bytes.
	pub fn to_json_bytes(&self) -> Result<Vec<u8>, LiveBuildStateError> {
		serde_json::to_vec(self).map_err(|source| LiveBuildStateError::Serialize {
			message: source.to_string(),
		})
	}

	/// Decode live-state facts from JSON bytes.
	///
	/// Checked, in order: whether `bytes` is instead an app error envelope
	/// (an app whose graph compilation itself failed emits `{"error":
	/// "..."}` rather than a `LiveBuildState`, surfaced here as
	/// [`LiveBuildStateError::App`]); the protocol version; that the
	/// embedded graph still recompiles cleanly (re-running
	/// [`crate::framework_graph::FrameworkGraph::compile`]'s own
	/// validation, not just a JSON shape check); and that the root
	/// document hash source is non-empty. A `LiveBuildState` returned from
	/// this function is one the caller can trust without further
	/// validation.
	pub fn from_json_bytes(bytes: &[u8]) -> Result<Self, LiveBuildStateError> {
		#[derive(Deserialize)]
		struct ErrorEnvelope {
			#[serde(default)]
			error: String,
		}

		let value = serde_json::from_slice::<serde_json::Value>(bytes).map_err(|source| {
			LiveBuildStateError::Parse {
				message: source.to_string(),
			}
		})?;
		let envelope =
			serde_json::from_value::<ErrorEnvelope>(value.clone()).map_err(|source| {
				LiveBuildStateError::Parse {
					message: source.to_string(),
				}
			})?;
		if !envelope.error.is_empty() {
			return Err(LiveBuildStateError::App {
				message: envelope.error,
			});
		}
		let state =
			serde_json::from_value::<Self>(value).map_err(|source| LiveBuildStateError::Parse {
				message: source.to_string(),
			})?;
		if state.protocol != LIVE_BUILD_STATE_PROTOCOL {
			return Err(LiveBuildStateError::ProtocolMismatch {
				expected: LIVE_BUILD_STATE_PROTOCOL,
				found: state.protocol,
			});
		}
		state.framework_graph()?;
		if state.root_document_hash_source.trim().is_empty() {
			return Err(LiveBuildStateError::MissingRootDocumentHashSource);
		}
		Ok(state)
	}

	/// Encode an app error response as live-state JSON bytes.
	///
	/// The counterpart to [`Self::from_json_bytes`]'s error-envelope
	/// handling: an app whose graph compilation itself fails (an app
	/// author's route declaration mistake, not a live-state protocol
	/// issue) emits this instead of a normal [`LiveBuildState`], so the
	/// build side can surface the app's own compile error to the
	/// developer rather than a confusing downstream JSON-shape failure.
	pub fn error_json_bytes(message: impl Into<String>) -> Result<Vec<u8>, LiveBuildStateError> {
		#[derive(Serialize)]
		struct ErrorEnvelope {
			error: String,
		}

		serde_json::to_vec(&ErrorEnvelope {
			error: message.into(),
		})
		.map_err(|source| LiveBuildStateError::Serialize {
			message: source.to_string(),
		})
	}
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct LiveFrameworkGraph {
	public_static_base: String,
	build_inputs: Option<BuildInputConfig>,
	document: DocumentContract,
	type_defs: Vec<TypeDef>,
	middlewares: Vec<LiveMiddlewareDeclaration>,
	views: Vec<LiveViewDeclaration>,
	resources: Vec<LiveResourceDeclaration>,
	static_assets: Vec<LiveStaticAssetDeclaration>,
}

impl LiveFrameworkGraph {
	fn from_framework_graph(graph: &FrameworkGraph) -> Self {
		Self {
			public_static_base: graph.config().public_static_base().to_owned(),
			build_inputs: graph.config().build_inputs().cloned(),
			document: graph.document().clone(),
			type_defs: graph.type_defs().to_vec(),
			middlewares: graph
				.middlewares()
				.iter()
				.map(|middleware| LiveMiddlewareDeclaration {
					patterns: middleware.patterns().to_vec(),
					methods: middleware
						.methods()
						.iter()
						.map(|method| method.as_str().to_owned())
						.collect(),
					handler_id: middleware.handler_id().as_str().to_owned(),
				})
				.collect(),
			views: graph
				.views()
				.iter()
				.map(|view| LiveViewDeclaration {
					pattern: view.pattern().to_owned(),
					client_file: view.client_file().to_owned(),
					search_schema: view.search_schema().clone(),
					type_contract: view.type_contract().clone(),
					handler_id: view.handler_id().as_str().to_owned(),
				})
				.collect(),
			resources: graph
				.resources()
				.iter()
				.map(|resource| LiveResourceDeclaration {
					method: resource.method().as_str().to_owned(),
					pattern: resource.pattern().to_owned(),
					kind: resource.kind(),
					input_schema: resource.input_schema().cloned(),
					type_contract: resource.type_contract().clone(),
					handler_id: resource.handler_id().as_str().to_owned(),
				})
				.collect(),
			static_assets: graph
				.static_assets()
				.iter()
				.map(|asset| LiveStaticAssetDeclaration {
					source_path: asset.source_path().to_owned(),
					public_path: asset.public_path().to_owned(),
				})
				.collect(),
		}
	}

	fn to_framework_graph(&self) -> Result<FrameworkGraph, LiveBuildStateError> {
		let mut config = FrameworkConfig::new(&self.public_static_base);
		if let Some(build_inputs) = self.build_inputs.clone() {
			config = config.with_build_inputs(build_inputs);
		}
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.set_document(self.document.clone());
		for type_def in &self.type_defs {
			declarations.add_type_def(type_def.clone());
		}
		for middleware in &self.middlewares {
			let mut methods = Vec::with_capacity(middleware.methods.len());
			for method in &middleware.methods {
				methods.push(http::Method::from_bytes(method.as_bytes()).map_err(|_| {
					LiveBuildStateError::InvalidHttpMethod {
						method: method.clone(),
					}
				})?);
			}
			declarations.add_middleware(
				MiddlewareDeclaration::new(handler_id(&middleware.handler_id)?)
					.with_patterns(middleware.patterns.iter().cloned())
					.with_methods(methods),
			);
		}
		for view in &self.views {
			declarations.add_view(ViewDeclaration::new(
				&view.pattern,
				&view.client_file,
				view.search_schema.clone(),
				view.type_contract.clone(),
				handler_id(&view.handler_id)?,
			));
		}
		for resource in &self.resources {
			declarations.add_resource(ResourceDeclaration::new(
				http_method(&resource.method)?,
				&resource.pattern,
				resource.kind,
				resource.input_schema.clone(),
				resource.type_contract.clone(),
				handler_id(&resource.handler_id)?,
			));
		}
		for asset in &self.static_assets {
			declarations.add_static_asset(StaticAssetDeclaration::new(
				&asset.source_path,
				&asset.public_path,
			));
		}
		FrameworkGraph::compile(declarations)
			.map_err(|source| LiveBuildStateError::Graph { source })
	}
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct LiveMiddlewareDeclaration {
	patterns: Vec<String>,
	methods: Vec<String>,
	handler_id: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct LiveViewDeclaration {
	pattern: String,
	client_file: String,
	search_schema: serde_json::Value,
	type_contract: RouteTypeContract,
	handler_id: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct LiveResourceDeclaration {
	method: String,
	pattern: String,
	kind: Option<ResourceKind>,
	input_schema: Option<serde_json::Value>,
	type_contract: RouteTypeContract,
	handler_id: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
struct LiveStaticAssetDeclaration {
	source_path: String,
	public_path: String,
}

/// Live build-state protocol error.
///
/// Returned by [`LiveBuildState::from_json_bytes`], distinguishing three
/// different failure sources a build side needs to react to differently:
/// an actual protocol-level problem
/// ([`Self::ProtocolMismatch`]/[`Self::Parse`]/[`Self::Serialize`]/[`Self::InvalidHandlerId`]/[`Self::InvalidHttpMethod`]/[`Self::MissingRootDocumentHashSource`]),
/// the emitting app's own graph failing to recompile
/// ([`Self::Graph`]), or the emitting app reporting its own error
/// directly ([`Self::App`], the counterpart to
/// [`LiveBuildState::error_json_bytes`]).
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LiveBuildStateError {
	/// Emitter spoke a different live-state protocol version.
	ProtocolMismatch {
		/// Protocol version this build crate expects.
		expected: u32,
		/// Protocol version the app binary emitted.
		found: u32,
	},
	/// Root document hash source could not be built.
	RootDocumentHashSource {
		/// Error message.
		message: String,
	},
	/// Live-state JSON serialization failed.
	Serialize {
		/// Error message.
		message: String,
	},
	/// Live-state JSON parsing failed.
	Parse {
		/// Error message.
		message: String,
	},
	/// Build entry returned an app error envelope.
	App {
		/// Error message.
		message: String,
	},
	/// Root document hash source was empty.
	MissingRootDocumentHashSource,
	/// Serialized handler id was invalid.
	InvalidHandlerId {
		/// Rejected handler id.
		handler_id: String,
	},
	/// Serialized HTTP method was invalid.
	InvalidHttpMethod {
		/// Rejected method.
		method: String,
	},
	/// Serialized graph facts failed canonical graph validation.
	Graph {
		/// Source graph error.
		source: GraphError,
	},
}

impl std::fmt::Display for LiveBuildStateError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::ProtocolMismatch { expected, found } => write!(
				f,
				"app binary emitted live-state protocol {found} but this build \
				expects {expected}; align the app's `vorma` and `vorma-build` \
				versions and rebuild"
			),
			Self::RootDocumentHashSource { message } => {
				write!(f, "build root document hash source: {message}")
			}
			Self::Serialize { message } => write!(f, "serialize live build state: {message}"),
			Self::Parse { message } => write!(f, "parse live build state: {message}"),
			Self::App { message } => write!(f, "{message}"),
			Self::MissingRootDocumentHashSource => {
				f.write_str("live build state missing root document hash source")
			}
			Self::InvalidHandlerId { handler_id } => {
				write!(f, "invalid live build-state handler id {handler_id:?}")
			}
			Self::InvalidHttpMethod { method } => {
				write!(f, "invalid live build-state HTTP method {method:?}")
			}
			Self::Graph { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for LiveBuildStateError {}

fn handler_id(handler_id: &str) -> Result<HandlerId, LiveBuildStateError> {
	HandlerId::new(handler_id).map_err(|source| match source {
		HandlerIdError::Empty => LiveBuildStateError::InvalidHandlerId {
			handler_id: handler_id.to_owned(),
		},
	})
}

fn http_method(method: &str) -> Result<Method, LiveBuildStateError> {
	Method::from_bytes(method.as_bytes()).map_err(|_| LiveBuildStateError::InvalidHttpMethod {
		method: method.to_owned(),
	})
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use crate::contracts::{
		DocumentAttributeContract, DocumentElementContract, FieldDef, TypeRefContract,
	};
	use crate::framework_graph::{DevWatchConfig, FrontendBuildInputs, ServerBuildTarget};

	use super::*;

	const TEST_ROOT_DOCUMENT_HASH_SOURCE: &str = "{\"html\":[],\"head\":[]}";

	fn route_type_contract() -> RouteTypeContract {
		RouteTypeContract::new(
			TypeRefContract::named("Input"),
			TypeRefContract::named("Output"),
		)
	}

	fn test_graph() -> FrameworkGraph {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "server"),
			".",
			"dist",
			FrontendBuildInputs::new(
				"react",
				"pnpm exec",
				".",
				"vite.config.ts",
				"src/entry.tsx",
				"public",
				"src/critical.css",
			),
			"src/vorma.gen.ts",
			DevWatchConfig::new(
				vec!["src/**/*.rs".to_owned()],
				vec!["Cargo.toml".to_owned()],
				vec!["content/**/*.md".to_owned()],
			),
		));
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.set_document(DocumentContract::new(
			vec![DocumentAttributeContract::new("lang", "en", false, false)],
			Vec::new(),
			vec![
				DocumentElementContract::new("meta")
					.with_attributes(BTreeMap::from([("charset".to_owned(), "utf-8".to_owned())])),
			],
			Vec::new(),
			Vec::new(),
		));
		declarations.add_type_def(TypeDef::record(
			"Input",
			vec![FieldDef::new("id", TypeRefContract::String, false)],
		));
		declarations.add_type_def(TypeDef::alias("Output", TypeRefContract::Unknown));
		declarations.add_middleware(
			MiddlewareDeclaration::new(HandlerId::new("middleware:root").unwrap())
				.with_patterns(["/users/*"])
				.with_methods([http::Method::GET]),
		);
		declarations.add_view(ViewDeclaration::new(
			"/users/:id",
			"src/users.tsx",
			serde_json::json!({"type":"object"}),
			route_type_contract(),
			HandlerId::new("view:users").unwrap(),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::POST,
			"/users/:id",
			Some(ResourceKind::Mutation),
			Some(serde_json::json!({"body":"UserInput"})),
			route_type_contract(),
			HandlerId::new("resource:update-user").unwrap(),
		));
		declarations.add_static_asset(StaticAssetDeclaration::new("logo.svg", "/static/logo.svg"));
		FrameworkGraph::compile(declarations).unwrap()
	}

	#[test]
	fn live_build_state_round_trips_graph_through_json() {
		let graph = test_graph();
		let state = LiveBuildState::from_graph(graph.clone(), TEST_ROOT_DOCUMENT_HASH_SOURCE);
		let encoded = state.to_json_bytes().unwrap();
		let decoded = LiveBuildState::from_json_bytes(&encoded).unwrap();

		assert_eq!(decoded.framework_graph().unwrap(), graph);
		assert_eq!(
			decoded.root_document_hash_source(),
			TEST_ROOT_DOCUMENT_HASH_SOURCE
		);
	}

	#[test]
	fn live_build_state_rejects_error_envelope() {
		let error = LiveBuildState::from_json_bytes(br#"{"error":"boom"}"#).unwrap_err();

		assert_eq!(
			error,
			LiveBuildStateError::App {
				message: "boom".to_owned()
			}
		);
	}

	#[test]
	fn rejects_mismatched_live_state_protocol_precisely() {
		let state = LiveBuildState::from_graph(test_graph(), "hash-source");
		let mut value =
			serde_json::from_slice::<serde_json::Value>(&state.to_json_bytes().unwrap()).unwrap();
		assert_eq!(
			value["protocol"],
			serde_json::json!(LIVE_BUILD_STATE_PROTOCOL)
		);

		value["protocol"] = serde_json::json!(LIVE_BUILD_STATE_PROTOCOL + 1);
		let error =
			LiveBuildState::from_json_bytes(&serde_json::to_vec(&value).unwrap()).unwrap_err();
		assert_eq!(
			error,
			LiveBuildStateError::ProtocolMismatch {
				expected: LIVE_BUILD_STATE_PROTOCOL,
				found: LIVE_BUILD_STATE_PROTOCOL + 1,
			}
		);

		/*
		Emitters that predate the field deserialize as protocol 0 and must
		hit the same precise error, not a generic parse failure.
		*/
		value.as_object_mut().unwrap().remove("protocol");
		let error =
			LiveBuildState::from_json_bytes(&serde_json::to_vec(&value).unwrap()).unwrap_err();
		assert_eq!(
			error,
			LiveBuildStateError::ProtocolMismatch {
				expected: LIVE_BUILD_STATE_PROTOCOL,
				found: 0,
			}
		);
	}
}
