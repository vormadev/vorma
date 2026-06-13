//! Framework-graph shape, contract, and asset validation.

use std::cmp::Ordering;
use std::collections::{BTreeSet, HashMap};

use http::Method;
use vorma_matcher::{
	FlatMatcher, MatcherBuilder, Pattern, compare_specificity, ensure_leading_and_trailing_slash,
	find_overlap,
};

use crate::graph_patterns::{
	EXPLICIT_INDEX_SEGMENT_IDENTIFIER, matcher_builder, params_for_pattern,
	view_parents_for_patterns,
};

use crate::contracts::{RawTsPart, RouteTypeContract, TypeDef, TypeRefContract};
use crate::framework_graph::GraphConfig;
use crate::framework_graph::{
	ConfigField, GraphError, MiddlewareDeclaration, ResourceDeclaration, ResourceKind,
	StaticAssetDeclaration, StaticAssetNode, ViewDeclaration,
};
pub(crate) fn normalize_base_path(value: String, field: ConfigField) -> Result<String, GraphError> {
	let trimmed = value.trim();
	if trimmed.is_empty() {
		return Err(GraphError::InvalidConfig {
			field: field.as_str(),
			value,
		});
	}
	Ok(ensure_leading_and_trailing_slash(trimmed))
}

pub(crate) fn validate_middleware_shapes(
	middlewares: &[MiddlewareDeclaration],
) -> Result<(), GraphError> {
	/*
	Scope patterns use the flat (resource-style) matcher: no index
	segment. Empty filter lists are valid (unrestricted); empty pattern
	STRINGS are not.
	*/
	let mut matcher_builder = matcher_builder(String::new())?;
	for middleware in middlewares {
		for pattern in &middleware.patterns {
			if pattern.trim().is_empty() {
				return Err(GraphError::InvalidMiddlewarePattern {
					pattern: pattern.clone(),
					reason: "middleware scope pattern must not be empty".to_owned(),
				});
			}
			matcher_builder
				.register_pattern(pattern)
				.map_err(|reason| GraphError::InvalidMiddlewarePattern {
					pattern: pattern.clone(),
					reason,
				})?;
		}
	}
	Ok(())
}

pub(crate) fn validate_view_shapes(
	views: &[ViewDeclaration],
	all_view_patterns: &[String],
) -> Result<Vec<ViewShape>, GraphError> {
	let mut seen_patterns = BTreeSet::new();
	let mut matcher_builder = matcher_builder(EXPLICIT_INDEX_SEGMENT_IDENTIFIER.to_owned())?;
	let mut shapes = Vec::with_capacity(views.len());
	for view in views {
		if view.pattern.trim().is_empty() {
			return Err(GraphError::InvalidViewPattern {
				pattern: view.pattern.clone(),
				reason: "view pattern must not be empty".to_owned(),
			});
		}
		let normalized_pattern = matcher_builder
			.normalize_pattern(&view.pattern)
			.map_err(|reason| GraphError::InvalidViewPattern {
				pattern: view.pattern.clone(),
				reason,
			})?
			.normalized_pattern()
			.to_owned();
		if !seen_patterns.insert(normalized_pattern) {
			return Err(GraphError::DuplicateViewPattern {
				pattern: view.pattern.clone(),
			});
		}
		matcher_builder
			.register_pattern(&view.pattern)
			.map_err(|_| GraphError::DuplicateViewPattern {
				pattern: view.pattern.clone(),
			})?;
		if safe_relative_path(&view.client_file).is_none() {
			return Err(GraphError::InvalidClientFile {
				path: view.client_file.clone(),
			});
		}
		shapes.push(ViewShape {
			parent_patterns: view_parents_for_patterns(&view.pattern, all_view_patterns),
			params: params_for_pattern(&view.pattern),
		});
	}
	Ok(shapes)
}

pub(crate) fn validate_resource_shapes(
	resources: &[ResourceDeclaration],
) -> Result<(Vec<ResourceShape>, HashMap<Method, FlatMatcher>), GraphError> {
	let mut validators = HashMap::<Method, ResourceMethodValidator>::new();
	let mut shapes = Vec::with_capacity(resources.len());
	for resource in resources {
		if resource.pattern.trim().is_empty() {
			return Err(GraphError::InvalidResourcePattern {
				pattern: resource.pattern.clone(),
				reason: "resource pattern must not be empty".to_owned(),
			});
		}
		validators
			.entry(resource.method.clone())
			.or_insert_with(ResourceMethodValidator::new)
			.register(resource.method.clone(), &resource.pattern)?;
		shapes.push(ResourceShape {
			default_kind: default_resource_kind(&resource.method),
			params: params_for_pattern(&resource.pattern),
		});
	}
	let matchers = validators
		.into_iter()
		.map(|(method, validator)| (method, validator.into_matcher()))
		.collect();
	Ok((shapes, matchers))
}

/*
Views and GET/HEAD resources share one URL space, adjudicated by the
matcher's specificity order — overlap between them is the normal
condition, exactly as it is between two views. The only illegal state is
a specificity tie on a shared path (identical shape), where no route can
win: the cross-table form of the route shape collision that registration
already rejects within each table. find_overlap supplies the shared-path
witness, so a violation names its own evidence.
*/
pub(crate) fn validate_resource_reachability(
	views: &[ViewDeclaration],
	resources: &[ResourceDeclaration],
	resource_matchers: &HashMap<Method, FlatMatcher>,
	public_static_base: &str,
) -> Result<(), GraphError> {
	let view_normalizer = matcher_builder(EXPLICIT_INDEX_SEGMENT_IDENTIFIER.to_owned())?;
	let flat_normalizer = matcher_builder(String::new())?;

	for resource in resources {
		if resource.method != Method::GET && resource.method != Method::HEAD {
			continue;
		}
		let resource_pattern = flat_normalizer
			.normalize_pattern(&resource.pattern)
			.map_err(|reason| GraphError::InvalidResourcePattern {
				pattern: resource.pattern.clone(),
				reason,
			})?;
		for view in views {
			let view_pattern =
				view_normalizer
					.normalize_pattern(&view.pattern)
					.map_err(|reason| GraphError::InvalidViewPattern {
						pattern: view.pattern.clone(),
						reason,
					})?;
			if compare_specificity(&view_pattern, &resource_pattern) != Ordering::Equal {
				continue;
			}
			let nested = solo_nested_matcher(&view_pattern)?;
			let flat = solo_flat_matcher(&resource_pattern)?;
			if let Some(overlap) = find_overlap(&nested, &flat) {
				return Err(GraphError::ResourceViewSpecificityTie {
					method: resource.method.clone(),
					resource_pattern: overlap.right_pattern().to_owned(),
					view_pattern: overlap.left_pattern().to_owned(),
					example_path: overlap.example_path().to_owned(),
				});
			}
		}
	}

	/*
	The public static base stays a reserved prefix for assets — a space
	partition, not a specificity question, since assets are concrete
	manifest paths rather than patterns. A root static base opts out of
	the partition (it makes every path an asset path), the same latitude
	the retired config checks gave it.
	*/
	if public_static_base != "/" {
		for method in [Method::GET, Method::HEAD] {
			let Some(resource_matcher) = resource_matchers.get(&method) else {
				continue;
			};
			let static_space = static_base_matcher(public_static_base)?;
			if let Some(overlap) = find_overlap(resource_matcher, &static_space) {
				return Err(GraphError::ResourceInsidePublicStaticBase {
					method,
					resource_pattern: overlap.left_pattern().to_owned(),
					example_path: overlap.example_path().to_owned(),
				});
			}
		}
	}
	Ok(())
}

fn solo_nested_matcher(pattern: &Pattern) -> Result<vorma_matcher::NestedMatcher, GraphError> {
	let mut builder = matcher_builder(String::new())?;
	builder
		.register_pattern(pattern.normalized_pattern())
		.map_err(|reason| GraphError::InvalidViewPattern {
			pattern: pattern.normalized_pattern().to_owned(),
			reason,
		})?;
	Ok(builder.finish_nested())
}

fn solo_flat_matcher(pattern: &Pattern) -> Result<FlatMatcher, GraphError> {
	let mut builder = matcher_builder(String::new())?;
	builder
		.register_pattern(pattern.normalized_pattern())
		.map_err(|reason| GraphError::InvalidResourcePattern {
			pattern: pattern.normalized_pattern().to_owned(),
			reason,
		})?;
	Ok(builder.finish_flat())
}

// The public static base as a flat matcher claiming everything strictly
// beneath it: a base of "/static/" becomes a single splat pattern under
// that prefix.
fn static_base_matcher(public_static_base: &str) -> Result<FlatMatcher, GraphError> {
	let pattern = format!("{public_static_base}*");
	let mut builder = matcher_builder(String::new())?;
	builder
		.register_pattern(&pattern)
		.map_err(|reason| GraphError::InvalidConfig {
			field: ConfigField::PublicStaticBase.as_str(),
			value: format!("{public_static_base} ({reason})"),
		})?;
	Ok(builder.finish_flat())
}

pub(crate) fn validate_type_contracts(
	type_defs: &[TypeDef],
	views: &[ViewDeclaration],
	resources: &[ResourceDeclaration],
) -> Result<(), GraphError> {
	let mut keys = BTreeSet::new();
	let mut names = BTreeSet::new();
	let mut declared = BTreeSet::new();
	for type_def in type_defs {
		let (key, name) = type_def_key_and_name(type_def);
		if key.trim().is_empty() || name.trim().is_empty() {
			return Err(GraphError::InvalidTypeDef {
				key: key.to_owned(),
				name: name.to_owned(),
				reason: "type key and name must not be empty",
			});
		}
		if !valid_typescript_type_identifier(name) {
			return Err(GraphError::InvalidTypeDef {
				key: key.to_owned(),
				name: name.to_owned(),
				reason: "type name must be a valid TypeScript identifier",
			});
		}
		if !keys.insert(key.to_owned()) {
			return Err(GraphError::DuplicateTypeKey {
				key: key.to_owned(),
			});
		}
		if !names.insert(name.to_owned()) {
			return Err(GraphError::DuplicateTypeName {
				name: name.to_owned(),
			});
		}
		declared.insert((key.to_owned(), name.to_owned()));
		validate_type_def_shape(type_def)?;
	}
	for type_def in type_defs {
		validate_type_def_refs(type_def, &declared)?;
	}
	for view in views {
		validate_route_type_contract_refs(view.type_contract(), &declared)?;
	}
	for resource in resources {
		validate_route_type_contract_refs(resource.type_contract(), &declared)?;
	}
	Ok(())
}

pub(crate) fn type_def_key_and_name(type_def: &TypeDef) -> (&str, &str) {
	match type_def {
		TypeDef::Alias { key, name, .. }
		| TypeDef::Record { key, name, .. }
		| TypeDef::StringEnum { key, name, .. }
		| TypeDef::Raw { key, name, .. } => (key, name),
	}
}

pub(crate) fn validate_type_def_shape(type_def: &TypeDef) -> Result<(), GraphError> {
	let (key, name) = type_def_key_and_name(type_def);
	match type_def {
		TypeDef::Record { fields, .. } => {
			let mut field_names = BTreeSet::new();
			for field in fields {
				if field.name().is_empty() || !field_names.insert(field.name().to_owned()) {
					return Err(GraphError::InvalidTypeDef {
						key: key.to_owned(),
						name: name.to_owned(),
						reason: "record field names must be non-empty and unique",
					});
				}
			}
		}
		TypeDef::StringEnum { variants, .. } => {
			let mut seen_variants = BTreeSet::new();
			for variant in variants {
				if !seen_variants.insert(variant) {
					return Err(GraphError::InvalidTypeDef {
						key: key.to_owned(),
						name: name.to_owned(),
						reason: "string enum variants must be unique",
					});
				}
			}
		}
		TypeDef::Alias { .. } | TypeDef::Raw { .. } => {}
	}
	Ok(())
}

pub(crate) fn validate_type_def_refs(
	type_def: &TypeDef,
	declared: &BTreeSet<(String, String)>,
) -> Result<(), GraphError> {
	match type_def {
		TypeDef::Alias { target, .. } => validate_type_ref(target, declared),
		TypeDef::Record { fields, .. } => {
			for field in fields {
				validate_type_ref(field.type_ref(), declared)?;
			}
			Ok(())
		}
		TypeDef::Raw { body, .. } => {
			for part in body {
				if let RawTsPart::TypeRef(type_ref) = part {
					validate_type_ref(type_ref, declared)?;
				}
			}
			Ok(())
		}
		TypeDef::StringEnum { .. } => Ok(()),
	}
}

pub(crate) fn validate_route_type_contract_refs(
	contract: &RouteTypeContract,
	declared: &BTreeSet<(String, String)>,
) -> Result<(), GraphError> {
	validate_type_ref(contract.input(), declared)?;
	validate_type_ref(contract.output(), declared)
}

pub(crate) fn validate_type_ref(
	type_ref: &TypeRefContract,
	declared: &BTreeSet<(String, String)>,
) -> Result<(), GraphError> {
	match type_ref {
		TypeRefContract::Named { key, name } => {
			if declared.contains(&(key.clone(), name.clone())) {
				return Ok(());
			}
			Err(GraphError::UnknownNamedType {
				key: key.clone(),
				name: name.clone(),
			})
		}
		TypeRefContract::Array(inner) | TypeRefContract::Nullable(inner) => {
			validate_type_ref(inner, declared)
		}
		TypeRefContract::Map(key, value) => {
			validate_type_ref(key, declared)?;
			validate_type_ref(value, declared)
		}
		TypeRefContract::Union(types) => {
			for type_ref in types {
				validate_type_ref(type_ref, declared)?;
			}
			Ok(())
		}
		TypeRefContract::Raw(parts) => {
			for part in parts {
				if let RawTsPart::TypeRef(type_ref) = part {
					validate_type_ref(type_ref, declared)?;
				}
			}
			Ok(())
		}
		TypeRefContract::Unit
		| TypeRefContract::Null
		| TypeRefContract::Unknown
		| TypeRefContract::Bool
		| TypeRefContract::String
		| TypeRefContract::Number
		| TypeRefContract::Integer
		| TypeRefContract::FormData
		| TypeRefContract::StringLiteral(_) => Ok(()),
	}
}

pub(crate) fn valid_typescript_type_identifier(name: &str) -> bool {
	let mut chars = name.chars();
	match chars.next() {
		Some(ch) if ch == '_' || ch == '$' || ch.is_ascii_alphabetic() => {}
		_ => return false,
	}
	chars.all(|ch| ch == '_' || ch == '$' || ch.is_ascii_alphanumeric())
}

pub(crate) fn validate_static_assets(
	config: &GraphConfig,
	assets: Vec<StaticAssetDeclaration>,
) -> Result<Vec<StaticAssetNode>, GraphError> {
	let mut seen_public_paths = BTreeSet::new();
	let mut nodes = Vec::with_capacity(assets.len());
	for asset in assets {
		if safe_relative_path(&asset.source_path).is_none() {
			return Err(GraphError::InvalidStaticAsset {
				path: asset.source_path,
			});
		}
		if !asset.public_path.starts_with(config.public_static_base()) {
			return Err(GraphError::InvalidStaticAsset {
				path: asset.public_path,
			});
		}
		if safe_relative_path(asset.public_path.trim_start_matches('/')).is_none()
			|| !seen_public_paths.insert(asset.public_path.clone())
		{
			return Err(GraphError::InvalidStaticAsset {
				path: asset.public_path,
			});
		}
		nodes.push(StaticAssetNode {
			source_path: asset.source_path,
			public_path: asset.public_path,
		});
	}
	Ok(nodes)
}

pub(crate) struct ViewShape {
	pub(crate) parent_patterns: Vec<String>,
	pub(crate) params: Vec<String>,
}

pub(crate) struct ResourceShape {
	pub(crate) default_kind: ResourceKind,
	pub(crate) params: Vec<String>,
}

pub(crate) struct ResourceMethodValidator {
	seen_patterns: BTreeSet<String>,
	matcher_builder: MatcherBuilder,
}

impl ResourceMethodValidator {
	fn new() -> Self {
		Self {
			seen_patterns: BTreeSet::new(),
			matcher_builder: matcher_builder(String::new())
				.expect("static matcher options should be valid"),
		}
	}

	fn register(&mut self, method: Method, pattern: &str) -> Result<(), GraphError> {
		let normalized_pattern = self
			.matcher_builder
			.normalize_pattern(pattern)
			.map_err(|reason| GraphError::InvalidResourcePattern {
				pattern: pattern.to_owned(),
				reason,
			})?
			.normalized_pattern()
			.to_owned();
		if !self.seen_patterns.insert(normalized_pattern) {
			return Err(GraphError::DuplicateResourcePattern {
				method,
				pattern: pattern.to_owned(),
			});
		}
		self.matcher_builder
			.register_pattern(pattern)
			.map(|_| ())
			.map_err(|_| GraphError::DuplicateResourcePattern {
				method,
				pattern: pattern.to_owned(),
			})
	}

	fn into_matcher(self) -> FlatMatcher {
		self.matcher_builder.finish_flat()
	}
}

pub(crate) fn default_resource_kind(method: &Method) -> ResourceKind {
	if *method == Method::GET || *method == Method::HEAD {
		return ResourceKind::Query;
	}
	ResourceKind::Mutation
}

pub(crate) fn safe_relative_path(path: &str) -> Option<&str> {
	if path.is_empty() || path.starts_with('/') {
		return None;
	}
	if path.split('/').any(|segment| {
		segment.is_empty() || segment == "." || segment == ".." || segment.contains('\\')
	}) {
		return None;
	}
	Some(path)
}
