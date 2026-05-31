use std::collections::HashMap;

use crate::pattern::Pattern;
use crate::segment::SegmentKind;

/// Captured dynamic and splat route parameters.
pub type Params = HashMap<String, String>;

/// Best single route match.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Match {
	pub pattern: Pattern,
	pub params: Params,
	pub splat_values: Vec<String>,
	pub(crate) score: u16,
}

impl Match {
	pub(crate) fn from_registered(pattern: Pattern, score: u16) -> Self {
		Self {
			pattern,
			params: Params::new(),
			splat_values: Vec::new(),
			score,
		}
	}

	pub(crate) fn better_than(&self, other: &Match) -> bool {
		if self.score != other.score {
			return self.score > other.score;
		}
		for i in 0..self
			.pattern
			.normalized_segments
			.len()
			.min(other.pattern.normalized_segments.len())
		{
			let left = self.pattern.normalized_segments[i].best_match_rank();
			let right = other.pattern.normalized_segments[i].best_match_rank();
			if left != right {
				return left > right;
			}
		}
		if self.pattern.last_seg_type != other.pattern.last_seg_type {
			if self.pattern.last_seg_type == Some(SegmentKind::Splat) {
				return false;
			}
			if other.pattern.last_seg_type == Some(SegmentKind::Splat) {
				return true;
			}
		}
		if self.pattern.normalized_segments.len() != other.pattern.normalized_segments.len() {
			return self.pattern.normalized_segments.len()
				> other.pattern.normalized_segments.len();
		}
		false
	}
}

/// One matched ancestor or leaf pattern in a nested match chain.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatch {
	pub pattern: Pattern,
	pub(crate) params: Params,
	pub(crate) splat_values: Vec<String>,
}

impl NestedMatch {
	pub(crate) fn from_registered(pattern: Pattern) -> Self {
		Self {
			pattern,
			params: Params::new(),
			splat_values: Vec::new(),
		}
	}

	pub fn params(&self) -> Params {
		self.params.clone()
	}

	pub fn splat_values(&self) -> Vec<String> {
		self.splat_values.clone()
	}
}

/// Ordered nested match chain plus shared captures.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatches {
	pub params: Params,
	pub splat_values: Vec<String>,
	pub matches: Vec<NestedMatch>,
}
