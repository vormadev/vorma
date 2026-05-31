use http::Method;
use vorma_matcher::parse_segments;

use super::ApiRouteKind;
use crate::constants::{DYNAMIC_PARAM_PREFIX, SPLAT_SEGMENT_IDENTIFIER};

pub(super) fn validate_declared_route_pattern(kind: &str, pattern: &str) -> Result<(), String> {
	if pattern.is_empty() {
		return Err(format!("{kind} pattern must not be empty"));
	}
	if !pattern.starts_with('/') {
		return Err(format!("{kind} pattern must be absolute"));
	}
	Ok(())
}

#[cfg(test)]
pub(super) fn default_api_route_kind_for_method(method: &Method) -> ApiRouteKind {
	if method == Method::GET || method == Method::HEAD {
		ApiRouteKind::Query
	} else {
		ApiRouteKind::Mutation
	}
}

#[doc(hidden)]
pub fn default_api_route_kind(method: &str) -> ApiRouteKind {
	if method == Method::GET.as_str() || method == Method::HEAD.as_str() {
		ApiRouteKind::Query
	} else {
		ApiRouteKind::Mutation
	}
}

#[doc(hidden)]
pub fn route_params_for_pattern(pattern: &str) -> Vec<String> {
	parse_segments(pattern)
		.into_iter()
		.filter_map(|segment| {
			segment
				.strip_prefix(DYNAMIC_PARAM_PREFIX)
				.map(ToOwned::to_owned)
		})
		.collect()
}

#[doc(hidden)]
pub fn route_is_splat_pattern(pattern: &str) -> bool {
	parse_segments(pattern)
		.last()
		.is_some_and(|segment| segment == SPLAT_SEGMENT_IDENTIFIER.to_string().as_str())
}

#[doc(hidden)]
pub fn view_parents_for_patterns(pattern: &str, all_patterns: &[String]) -> Vec<String> {
	let has_root_view = all_patterns.iter().any(|candidate| candidate == "/");
	let mut parents = Vec::new();
	if pattern != "/" && has_root_view {
		parents.push("/".to_owned());
	}
	for maybe_parent in all_patterns {
		if maybe_parent == pattern {
			continue;
		}
		if pattern.starts_with(&format!("{maybe_parent}/")) {
			parents.push(maybe_parent.clone());
		}
	}
	parents
}
