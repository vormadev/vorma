//! Immutable request execution plan.
//!
//! Framework-integration surface:
//! [`compile`](crate::execution_plan::ExecutionPlan::compile) takes a
//! validated [`crate::framework_graph::FrameworkGraph`] and projects it
//! into the immutable `vorma_matcher` matchers (one [`vorma_matcher::FlatMatcher`]
//! per HTTP method for resources, one [`vorma_matcher::NestedMatcher`] for
//! views) plus handler-lookup tables — the shape `vorma`'s request
//! dispatch actually runs against at request time, compiled once when an
//! app's committed runtime snapshot is built rather than re-derived per
//! request. An application author never constructs an `ExecutionPlan`
//! directly. Route-matching semantics themselves (specificity ordering,
//! the dirty-path rule, index-pattern claiming) belong to `vorma_matcher`
//! and are not repeated here; this module's job is wiring compiled
//! matchers to the right handler and type-contract facts.

use std::collections::HashMap;
use std::sync::Arc;

use http::Method;
use std::sync::Arc as StdArc;

use vorma_matcher::{
	FlatMatcher, MatcherBuilder, NestedMatcher, Params, Pattern, SplatValues, ensure_leading_slash,
};

use crate::contracts::{TypeDef, TypeRefContract};
use crate::framework_graph::{FrameworkGraph, GraphError, HandlerId};
use crate::graph_patterns::{EXPLICIT_INDEX_SEGMENT_IDENTIFIER, matcher_builder};

/// Immutable precomputed execution plan.
///
/// Compiled once from a [`crate::framework_graph::FrameworkGraph`]
/// (see [`Self::compile`]) and then reused for every request: resource
/// routes are matched per-method through a flat matcher
/// ([`Self::match_resource`]), view routes through a nested matcher that
/// resolves the full outermost-to-innermost layout chain in one call
/// ([`Self::match_views`]), and middleware applicability is a linear scan
/// in declaration order ([`Self::matching_middleware_ids`]) since
/// middleware ordering is itself observable behavior.
#[derive(Clone, Debug)]
pub struct ExecutionPlan {
	type_defs: Vec<TypeDef>,
	resource_matchers: HashMap<Method, FlatMatcher>,
	resource_routes: HashMap<Method, HashMap<String, Arc<ResourcePlanNode>>>,
	view_matcher: NestedMatcher,
	view_routes: HashMap<String, Arc<ViewPlanNode>>,
	middlewares: Vec<MiddlewarePlanNode>,
}

impl ExecutionPlan {
	/// Compile a framework graph into immutable matchers and handler plans.
	pub fn compile(graph: &FrameworkGraph) -> Result<Self, PlanError> {
		let mut resource_builders = HashMap::new();
		let mut resource_routes = HashMap::new();
		for resource in graph.resources() {
			let builder = resource_builders
				.entry(resource.method().clone())
				.or_insert_with(resource_matcher_builder);
			builder
				.register_pattern(resource.pattern())
				.map_err(|reason| PlanError::InvalidResourcePattern {
					pattern: resource.pattern().to_owned(),
					reason,
				})?;
			resource_routes
				.entry(resource.method().clone())
				.or_insert_with(HashMap::new)
				.insert(
					resource.pattern().to_owned(),
					Arc::new(ResourcePlanNode {
						pattern: resource.pattern().to_owned(),
						handler_id: resource.handler_id().clone(),
						input_type: resource.type_contract().input().clone(),
					}),
				);
		}
		let mut view_builder = view_matcher_builder();
		let mut view_routes = HashMap::new();
		for view in graph.views() {
			view_builder
				.register_pattern(view.pattern())
				.map_err(|reason| PlanError::InvalidViewPattern {
					pattern: view.pattern().to_owned(),
					reason,
				})?;
			view_routes.insert(
				view.pattern().to_owned(),
				Arc::new(ViewPlanNode {
					pattern: view.pattern().to_owned(),
					handler_id: view.handler_id().clone(),
					input_type: view.type_contract().input().clone(),
				}),
			);
		}
		let mut middlewares = Vec::with_capacity(graph.middlewares().len());
		for middleware in graph.middlewares() {
			let scope_matcher = if middleware.patterns().is_empty() {
				None
			} else {
				let mut builder = resource_matcher_builder();
				for pattern in middleware.patterns() {
					builder.register_pattern(pattern).map_err(|reason| {
						PlanError::InvalidMiddlewarePattern {
							pattern: pattern.clone(),
							reason,
						}
					})?;
				}
				Some(builder.finish_flat())
			};
			middlewares.push(MiddlewarePlanNode {
				handler_id: middleware.handler_id().clone(),
				methods: middleware.methods().to_vec(),
				scope_matcher,
			});
		}
		Ok(Self {
			type_defs: graph.type_defs().to_vec(),
			resource_matchers: resource_builders
				.into_iter()
				.map(|(method, builder)| (method, builder.finish_flat()))
				.collect(),
			resource_routes,
			view_matcher: view_builder.finish_nested(),
			view_routes,
			middlewares,
		})
	}

	/// Middleware handlers that apply to this request, in declaration
	/// order. Both filters an app declared for a middleware — a method
	/// list and a scope-pattern list — are optional and AND together: no
	/// method filter means "any method", no pattern filter means "any
	/// path".
	/*
	Scope patterns are URL patterns, matched against the request path
	exactly as received — ONE observable space, no rewriting, the same
	space resource and view patterns live in.
	*/
	pub fn matching_middleware_ids(&self, method: &Method, path: &str) -> Vec<HandlerId> {
		let path = ensure_leading_slash(path);
		self.middlewares
			.iter()
			.filter(|middleware| {
				if !middleware.methods.is_empty() && !middleware.methods.contains(method) {
					return false;
				}
				match &middleware.scope_matcher {
					None => true,
					Some(matcher) => matcher.find_best_match(&path).is_some(),
				}
			})
			.map(|middleware| middleware.handler_id.clone())
			.collect()
	}

	/// Type definitions available to route input decoders.
	pub fn type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}

	/// Match a resource request path and method.
	///
	/// A `HEAD` request first tries a route explicitly registered for
	/// `HEAD`; if none matches, it falls back to the `GET` route for the
	/// same path (standard HTTP practice — a `GET` handler answers `HEAD`
	/// by running normally and discarding the body) and the returned
	/// [`ResourceMatch::head_fallback_to_get`] reports which happened.
	pub fn match_resource(&self, method: &Method, path: &str) -> Option<ResourceMatch> {
		let path = ensure_leading_slash(path);
		if *method == Method::HEAD {
			if let Some(matcher) = self.resource_matchers.get(&Method::HEAD)
				&& let Some(found) = matcher.find_best_match(&path)
			{
				let route = self.resource_route(&Method::HEAD, found.pattern.original_pattern());
				return Some(ResourceMatch {
					method: Method::HEAD,
					params: found.params,
					splat_values: found.splat_values,
					head_fallback_to_get: false,
					route: Arc::clone(route),
					matched_pattern: found.pattern,
				});
			}
			let matcher = self.resource_matchers.get(&Method::GET)?;
			return matcher.find_best_match(&path).map(|found| {
				let route = self.resource_route(&Method::GET, found.pattern.original_pattern());
				ResourceMatch {
					method: Method::GET,
					params: found.params,
					splat_values: found.splat_values,
					head_fallback_to_get: true,
					route: Arc::clone(route),
					matched_pattern: found.pattern,
				}
			});
		}
		self.resource_matchers
			.get(method)?
			.find_best_match(&path)
			.map(|found| {
				let route = self.resource_route(method, found.pattern.original_pattern());
				ResourceMatch {
					method: method.clone(),
					params: found.params,
					splat_values: found.splat_values,
					head_fallback_to_get: false,
					route: Arc::clone(route),
					matched_pattern: found.pattern,
				}
			})
	}

	/// Return methods with a resource route matching this path.
	///
	/// Powers a `405 Method Not Allowed` response's `Allow` header: a
	/// path that resolves no route at all is `404`, but a path that
	/// resolves a route for *some* method and not the one requested is
	/// `405`, and the distinction depends on knowing every method that
	/// would have matched. `HEAD` is added automatically whenever `GET`
	/// is present and `HEAD` was not separately registered, mirroring
	/// [`Self::match_resource`]'s fallback. Sorted for a stable, readable
	/// `Allow` header rather than hash-map iteration order.
	pub fn allowed_resource_methods(&self, path: &str) -> Vec<Method> {
		let path = ensure_leading_slash(path);
		let mut methods = Vec::new();
		for (method, matcher) in &self.resource_matchers {
			if matcher.find_best_match(&path).is_some() {
				methods.push(method.clone());
			}
		}
		if methods.contains(&Method::GET) && !methods.contains(&Method::HEAD) {
			methods.push(Method::HEAD);
		}
		methods.sort_by(|left, right| left.as_str().cmp(right.as_str()));
		methods
	}

	/// Match a view request path.
	///
	/// Resolves the full nested layout chain in one call — see
	/// [`vorma_matcher::NestedMatcher::find_nested_matches`] for the
	/// underlying algorithm (index-pattern claiming, ancestor-layout
	/// ordering, the catch-all cover rule). `None` means no view,
	/// including no catch-all, matched this path at all.
	pub fn match_views(&self, path: &str) -> Option<ViewMatches> {
		let found = self.view_matcher.find_nested_matches(path)?;
		let leaf_pattern = StdArc::clone(
			&found
				.matches
				.last()
				.expect("a nested match result carries at least one match")
				.pattern,
		);
		let nodes = found
			.matches
			.into_iter()
			.map(|matched| {
				let route = self.view_route(matched.pattern.original_pattern());
				ViewExecutionNode {
					route: Arc::clone(route),
				}
			})
			.collect();
		Some(ViewMatches {
			nodes,
			params: found.params,
			splat_values: found.splat_values,
			leaf_pattern,
		})
	}

	fn resource_route(&self, method: &Method, pattern: &str) -> &Arc<ResourcePlanNode> {
		self.resource_routes
			.get(method)
			.and_then(|routes| routes.get(pattern))
			.expect("graph and matcher resource metadata should agree")
	}

	fn view_route(&self, pattern: &str) -> &Arc<ViewPlanNode> {
		self.view_routes
			.get(pattern)
			.expect("graph and matcher view metadata should agree")
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ResourcePlanNode {
	pattern: String,
	handler_id: HandlerId,
	input_type: TypeRefContract,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ViewPlanNode {
	pattern: String,
	handler_id: HandlerId,
	input_type: TypeRefContract,
}

/*
One flat matcher per scoped middleware: the run decision is a plain
"does the request path match any of its patterns", in the same grammar
routes use. Empty filters mean unrestricted; filters AND together.
*/
#[derive(Clone, Debug)]
struct MiddlewarePlanNode {
	handler_id: HandlerId,
	methods: Vec<Method>,
	scope_matcher: Option<FlatMatcher>,
}

/// Resource route match.
///
/// Everything `vorma`'s request dispatch needs once
/// [`ExecutionPlan::match_resource`] finds a route: which handler to run,
/// the captured params/splat values, and — when a matched path could also
/// have resolved a view (both share the GET/HEAD URL space) — the
/// normalized pattern needed to adjudicate which one actually wins by
/// specificity.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceMatch {
	method: Method,
	params: Params,
	splat_values: SplatValues,
	head_fallback_to_get: bool,
	route: Arc<ResourcePlanNode>,
	matched_pattern: StdArc<Pattern>,
}

impl ResourceMatch {
	/// Matched handler method.
	pub fn method(&self) -> &Method {
		&self.method
	}

	/// Matched normalized pattern, for specificity adjudication.
	///
	/// Compare against a candidate view's
	/// [`ViewMatches::leaf_pattern`] with [`vorma_matcher::compare_specificity`]
	/// when a path could resolve either — graph validation already
	/// guarantees the comparison never comes back tied.
	pub fn matched_pattern(&self) -> &Pattern {
		self.matched_pattern.as_ref()
	}

	/// Matched pattern.
	pub fn pattern(&self) -> &str {
		&self.route.pattern
	}

	/// Captured params.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Captured splat values.
	pub fn splat_values(&self) -> &SplatValues {
		&self.splat_values
	}

	/// Whether a HEAD request used the GET handler.
	pub fn head_fallback_to_get(&self) -> bool {
		self.head_fallback_to_get
	}

	/// Runtime handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.route.handler_id
	}

	/// Input type contract for this resource handler.
	pub fn input_type(&self) -> &TypeRefContract {
		&self.route.input_type
	}
}

/// Matched view execution node.
///
/// One entry in a [`ViewMatches`] chain — one layout level's handler
/// facts. Every node in a chain shares the same captured
/// [`ViewMatches::params`]/[`ViewMatches::splat_values`]; only the
/// handler identity and input type contract vary per node, since those
/// come from each level's own declaration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewExecutionNode {
	route: Arc<ViewPlanNode>,
}

impl ViewExecutionNode {
	/// Matched view pattern.
	pub fn pattern(&self) -> &str {
		&self.route.pattern
	}

	/// Runtime handler identifier.
	pub fn handler_id(&self) -> &HandlerId {
		&self.route.handler_id
	}

	/// Input type contract for this view handler.
	pub fn input_type(&self) -> &TypeRefContract {
		&self.route.input_type
	}
}

/// Nested view match chain.
///
/// One request path resolves to a chain of nested layouts, outermost
/// (root) to innermost (the deepest matched view) — this is that chain,
/// plus the params/splat values captured across all of them (a request
/// path is captured once against the full nested pattern chain, not once
/// per level). [`ViewMatches`] is `vorma`'s per-request unit of view
/// dispatch: every node runs, and every node's data feeds the browser's
/// [`crate::wire::ViewPayload`] in the same outermost-to-innermost order.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewMatches {
	nodes: Vec<ViewExecutionNode>,
	params: Params,
	splat_values: SplatValues,
	leaf_pattern: StdArc<Pattern>,
}

impl ViewMatches {
	/// Matched view execution nodes from outermost to innermost.
	pub fn nodes(&self) -> &[ViewExecutionNode] {
		&self.nodes
	}

	/// Innermost matched normalized pattern, for specificity adjudication.
	///
	/// See [`ResourceMatch::matched_pattern`] for the comparison this
	/// feeds.
	pub fn leaf_pattern(&self) -> &Pattern {
		self.leaf_pattern.as_ref()
	}

	/// Matched patterns from outermost to innermost.
	pub fn patterns(&self) -> Vec<&str> {
		self.nodes.iter().map(ViewExecutionNode::pattern).collect()
	}

	/// Captured params.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Captured splat values.
	pub fn splat_values(&self) -> &SplatValues {
		&self.splat_values
	}
}

/// Execution plan compile error.
///
/// [`ExecutionPlan::compile`] re-registers every already-graph-validated
/// pattern with `vorma_matcher` in order to build the actual matchers, so
/// in practice these variants fire only if graph validation and matcher
/// registration ever disagreed about a pattern's validity — a framework
/// consistency bug, not a shape an application author's own mistake would
/// normally reach (graph compilation already rejects invalid patterns
/// earlier, with the same reason text).
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PlanError {
	/// Resource pattern is invalid.
	InvalidResourcePattern {
		/// Rejected resource pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// Middleware scope pattern is invalid.
	InvalidMiddlewarePattern {
		/// Rejected scope pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// View pattern is invalid.
	InvalidViewPattern {
		/// Rejected view pattern.
		pattern: String,
		/// Matcher rejection reason.
		reason: String,
	},
	/// Graph failed to normalize.
	Graph {
		/// Source graph normalization error.
		source: GraphError,
	},
}

impl std::fmt::Display for PlanError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidResourcePattern { pattern, reason } => {
				write!(f, "invalid resource pattern {pattern:?}: {reason}")
			}
			Self::InvalidMiddlewarePattern { pattern, reason } => {
				write!(f, "invalid middleware scope pattern {pattern:?}: {reason}")
			}
			Self::InvalidViewPattern { pattern, reason } => {
				write!(f, "invalid view pattern {pattern:?}: {reason}")
			}
			Self::Graph { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for PlanError {}

fn resource_matcher_builder() -> MatcherBuilder {
	matcher_builder(String::new()).expect("static matcher options should be valid")
}

fn view_matcher_builder() -> MatcherBuilder {
	matcher_builder(EXPLICIT_INDEX_SEGMENT_IDENTIFIER.to_owned())
		.expect("static matcher options should be valid")
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::framework_graph::{
		FrameworkDeclarations, HandlerId, MiddlewareDeclaration, ResourceDeclaration,
		ViewDeclaration,
	};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	#[test]
	fn plan_matches_resources_at_full_url_patterns_and_head_get_fallback() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/stories/:id",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();

		let matched = plan
			.match_resource(&Method::HEAD, "/api/stories/123")
			.unwrap();

		assert_eq!(matched.method(), Method::GET);
		assert_eq!(matched.pattern(), "/api/stories/:id");
		assert_eq!(matched.params()["id"], "123");
		assert_eq!(matched.handler_id().as_str(), "resource");
		assert_eq!(
			plan.matching_middleware_ids(&Method::POST, "/items/9")[0].as_str(),
			"middleware"
		);
		assert!(matched.head_fallback_to_get());
		assert_eq!(
			plan.allowed_resource_methods("/api/stories/123"),
			vec![Method::GET, Method::HEAD]
		);
		assert!(plan.allowed_resource_methods("/static/app.css").is_empty());
	}

	#[test]
	fn plan_matches_nested_views_with_handler_facts() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("story"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();

		let matched = plan.match_views("/stories/123").unwrap();

		assert_eq!(matched.patterns(), ["/", "/stories/:id"]);
		assert_eq!(matched.params()["id"], "123");
		assert_eq!(matched.nodes()[1].handler_id().as_str(), "story");
	}
}
