use std::cmp::Ordering;
use std::sync::Arc;

use crate::segment::{SCORE_DYNAMIC, SCORE_STATIC, Segment, SegmentKind};

/// Registered route pattern.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Pattern {
	pub(crate) original_pattern: String,
	pub(crate) normalized_pattern: String,
	pub(crate) normalized_segments: Vec<InternalSegment>,
	pub(crate) last_seg_type: Option<SegmentKind>,
	pub(crate) last_seg_is_non_root_splat: bool,
	pub(crate) last_seg_is_index: bool,
	pub(crate) specificity_score: u32,
	pub(crate) is_static: bool,
	pub(crate) is_root_catch_all: bool,
	// Positions and names of dynamic segments, precomputed so capture
	// building walks exactly the capturing positions. Names are shared
	// into params as refcount bumps, never fresh allocations.
	pub(crate) param_positions: Vec<(usize, Arc<str>)>,
	// Index of the splat tail when this pattern captures one.
	pub(crate) splat_start: Option<usize>,
}

impl Pattern {
	pub(crate) fn new(
		original_pattern: String,
		normalized_pattern: String,
		normalized_segments: Vec<InternalSegment>,
		last_seg_type: Option<SegmentKind>,
	) -> Self {
		let segment_count = normalized_segments.len();
		let specificity_score = normalized_segments
			.iter()
			.map(InternalSegment::best_match_rank)
			.sum();
		let pattern_is_static = is_static(&normalized_segments);
		let is_root_catch_all = normalized_pattern == "/*";
		let last_seg_is_non_root_splat =
			last_seg_type == Some(SegmentKind::Splat) && segment_count > 1;
		let param_positions: Vec<(usize, Arc<str>)> = normalized_segments
			.iter()
			.enumerate()
			.filter(|(_, seg)| seg.kind == SegmentKind::Dynamic)
			.map(|(i, seg)| (i, Arc::from(&seg.normalized_value[1..])))
			.collect();
		let splat_start = if is_root_catch_all || last_seg_is_non_root_splat {
			Some(segment_count - 1)
		} else {
			None
		};
		Self {
			original_pattern,
			normalized_pattern,
			normalized_segments,
			last_seg_type,
			last_seg_is_non_root_splat,
			last_seg_is_index: last_seg_type == Some(SegmentKind::Index),
			specificity_score,
			is_static: pattern_is_static,
			is_root_catch_all,
			param_positions,
			splat_start,
		}
	}

	/// Pattern text as originally registered.
	pub fn original_pattern(&self) -> &str {
		&self.original_pattern
	}

	/// Canonical pattern text after matcher normalization.
	pub fn normalized_pattern(&self) -> &str {
		&self.normalized_pattern
	}

	/// Canonical segment metadata after matcher normalization.
	pub fn normalized_segments(&self) -> Vec<Segment> {
		self.normalized_segments
			.iter()
			.map(|s| Segment {
				normalized_value: s.normalized_value.clone(),
				kind: s.kind,
			})
			.collect()
	}

	pub(crate) fn specificity_score(&self) -> u32 {
		self.specificity_score
	}

	pub(crate) fn shape_key(&self) -> String {
		let mut out = String::new();
		for (i, seg) in self.normalized_segments.iter().enumerate() {
			if i > 0 {
				out.push('/');
			}
			match seg.kind {
				SegmentKind::Dynamic => out.push_str("D:"),
				SegmentKind::Splat => out.push_str("P:"),
				SegmentKind::Index => out.push_str("I:"),
				SegmentKind::Static => {
					out.push_str("S:");
					out.push_str(&seg.normalized_value);
				}
			}
		}
		out
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct InternalSegment {
	pub(crate) normalized_value: String,
	pub(crate) kind: SegmentKind,
}

impl InternalSegment {
	pub(crate) fn best_match_rank(&self) -> u32 {
		match self.kind {
			SegmentKind::Static | SegmentKind::Index => SCORE_STATIC as u32,
			SegmentKind::Dynamic => SCORE_DYNAMIC as u32,
			SegmentKind::Splat => 0,
		}
	}
}

pub(crate) fn is_static(segs: &[InternalSegment]) -> bool {
	segs.iter()
		.all(|s| s.kind != SegmentKind::Splat && s.kind != SegmentKind::Dynamic)
}

/// Total specificity order over registered patterns.
///
/// This is the one ordering the matcher resolves competing matches with:
/// higher total segment score wins (static and index segments score 2,
/// dynamic 1, splat 0); then the leftmost position whose segment ranks
/// differ; then a trailing splat loses to any non-splat ending; then the
/// longer pattern wins.
///
/// `Ordering::Equal` means neither pattern can outrank the other. For two
/// patterns that can match a common path, that is exactly "identical
/// shape" — the matcher has no principled winner, which is the same
/// condition pattern registration rejects within one matcher as a route
/// shape collision.
pub fn compare_specificity(a: &Pattern, b: &Pattern) -> Ordering {
	let score = a.specificity_score().cmp(&b.specificity_score());
	if score != Ordering::Equal {
		return score;
	}
	for i in 0..a.normalized_segments.len().min(b.normalized_segments.len()) {
		let rank = a.normalized_segments[i]
			.best_match_rank()
			.cmp(&b.normalized_segments[i].best_match_rank());
		if rank != Ordering::Equal {
			return rank;
		}
	}
	if a.last_seg_type != b.last_seg_type {
		if a.last_seg_type == Some(SegmentKind::Splat) {
			return Ordering::Less;
		}
		if b.last_seg_type == Some(SegmentKind::Splat) {
			return Ordering::Greater;
		}
	}
	a.normalized_segments
		.len()
		.cmp(&b.normalized_segments.len())
}
