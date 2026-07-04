use std::cmp::Ordering;
use std::sync::Arc;

use crate::segment::{SCORE_DYNAMIC, SCORE_STATIC, Segment, SegmentKind};

/// One validated, normalized route pattern.
///
/// Produced by [`MatcherBuilder::normalize_pattern`] or
/// [`MatcherBuilder::register_pattern`], and returned inside every
/// match result ([`Match::pattern`], [`NestedMatch::pattern`]). A
/// `Pattern` is inert data — it does not itself match anything; the
/// matcher types hold patterns internally and consult
/// [`compare_specificity`] to arbitrate between them. Applications
/// mostly read a matched `Pattern` back out of a match result to learn
/// which route was selected (`normalized_pattern()`, for logging,
/// metrics, or dispatch keyed on the route rather than the raw path).
///
/// [`MatcherBuilder::normalize_pattern`]: crate::MatcherBuilder::normalize_pattern
/// [`MatcherBuilder::register_pattern`]: crate::MatcherBuilder::register_pattern
/// [`Match::pattern`]: crate::Match::pattern
/// [`NestedMatch::pattern`]: crate::NestedMatch::pattern
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

	/// The pattern text exactly as the application passed it to
	/// [`MatcherBuilder::register_pattern`] or
	/// [`MatcherBuilder::normalize_pattern`] — before any
	/// normalization. Useful for error messages and diagnostics that
	/// should echo back what the application actually wrote.
	///
	/// [`MatcherBuilder::register_pattern`]: crate::MatcherBuilder::register_pattern
	/// [`MatcherBuilder::normalize_pattern`]: crate::MatcherBuilder::normalize_pattern
	pub fn original_pattern(&self) -> &str {
		&self.original_pattern
	}

	/// This pattern's canonical text after normalization: absolute,
	/// dynamic segments spelled `:name` and splats spelled `*`
	/// regardless of the [`Options`] the pattern was registered under,
	/// and no redundant trailing slash unless the pattern is an index
	/// pattern (whose trailing slash is load-bearing, not redundant).
	/// Two patterns that normalize to the same text are the same
	/// pattern as far as the matcher is concerned — this is the value
	/// [`MatcherBuilder::register_pattern`] keys registration on, and
	/// the value a matched result's route should be logged, compared,
	/// or dispatched on rather than the raw request path.
	///
	/// [`Options`]: crate::Options
	/// [`MatcherBuilder::register_pattern`]: crate::MatcherBuilder::register_pattern
	pub fn normalized_pattern(&self) -> &str {
		&self.normalized_pattern
	}

	/// This pattern's segments after normalization, one [`Segment`] per
	/// slash-separated position. Reach for this when an application
	/// needs to inspect a pattern's shape directly (for example
	/// building a URL from named parameters using the same segment
	/// order the matcher itself matched against) rather than just the
	/// flat [`Pattern::normalized_pattern`] text.
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
///
/// Matching consults this ordering internally; applications typically
/// never need to call it directly. It is exposed for tooling built on
/// top of the matcher — for example explaining *why* one route was
/// chosen over another that also matched, or building a route-table
/// linter — where the ordering itself, not just its outcome inside one
/// match call, is the thing being inspected.
///
/// ```
/// use std::cmp::Ordering;
/// use vorma_matcher::{MatcherBuilder, Options, compare_specificity};
///
/// let builder = MatcherBuilder::new(Options::default())?;
/// let export = builder.normalize_pattern("/users/export")?;
/// let dynamic = builder.normalize_pattern("/users/:user_id")?;
/// assert_eq!(
///     compare_specificity(&export, &dynamic),
///     Ordering::Greater,
///     "a static segment outranks a dynamic one at the same position"
/// );
/// # Ok::<(), String>(())
/// ```
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
