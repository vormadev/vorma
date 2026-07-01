//! Route-pattern semantics and matcher integration for graph compilation.

use vorma_matcher::{MatcherBuilder, Options as MatcherOptions};

use crate::framework_graph::GraphError;

pub(crate) const DYNAMIC_PARAM_PREFIX: char = ':';
pub(crate) const SPLAT_SEGMENT_IDENTIFIER: char = '*';
pub(crate) const EXPLICIT_INDEX_SEGMENT_IDENTIFIER: &str = "_index";

pub(crate) fn params_for_pattern(pattern: &str) -> Vec<String> {
	pattern
		.split('/')
		.filter_map(|segment| segment.strip_prefix(DYNAMIC_PARAM_PREFIX))
		.map(ToOwned::to_owned)
		.collect()
}

pub(crate) fn view_parents_for_patterns(pattern: &str, all_patterns: &[String]) -> Vec<String> {
	let mut parents = all_patterns
		.iter()
		.filter(|candidate| {
			candidate.as_str() != pattern
				&& can_be_view_parent(candidate)
				&& route_scope_contains(candidate, pattern)
		})
		.cloned()
		.collect::<Vec<_>>();
	parents.sort_by(|left, right| {
		let left_depth = left
			.split('/')
			.filter(|segment| !segment.is_empty())
			.count();
		let right_depth = right
			.split('/')
			.filter(|segment| !segment.is_empty())
			.count();
		left_depth.cmp(&right_depth).then_with(|| left.cmp(right))
	});
	parents
}

fn can_be_view_parent(pattern: &str) -> bool {
	!route_pattern_segments(pattern)
		.last()
		.is_some_and(|segment| is_splat_segment(segment))
}

pub(crate) fn route_scope_contains(scope_pattern: &str, pattern: &str) -> bool {
	let scope_segments = route_pattern_segments(scope_pattern);
	let pattern_segments = route_pattern_segments(pattern);
	if scope_segments.is_empty() {
		return true;
	}
	scope_segments
		.iter()
		.zip(pattern_segments.iter())
		.all(|(scope_segment, pattern_segment)| {
			scope_segment_contains(scope_segment, pattern_segment)
		}) && scope_segments.len() <= pattern_segments.len()
}

pub(crate) fn route_pattern_segments(pattern: &str) -> Vec<String> {
	pattern
		.split('/')
		.filter(|segment| !segment.is_empty())
		.map(ToOwned::to_owned)
		.collect()
}

pub(crate) fn scope_segment_contains(scope_segment: &str, pattern_segment: &str) -> bool {
	if is_splat_segment(scope_segment) {
		return true;
	}
	if scope_segment.starts_with(DYNAMIC_PARAM_PREFIX) {
		return !is_splat_segment(pattern_segment);
	}
	scope_segment == pattern_segment
}

pub(crate) fn is_splat_segment(segment: &str) -> bool {
	let mut chars = segment.chars();
	chars.next() == Some(SPLAT_SEGMENT_IDENTIFIER) && chars.next().is_none()
}

pub(crate) fn matcher_builder(
	explicit_index_segment_identifier: String,
) -> Result<MatcherBuilder, GraphError> {
	MatcherBuilder::new(MatcherOptions {
		dynamic_param_prefix: DYNAMIC_PARAM_PREFIX,
		splat_segment_identifier: SPLAT_SEGMENT_IDENTIFIER,
		explicit_index_segment_identifier,
	})
	.map_err(|reason| GraphError::InvalidMatcherOptions { reason })
}
