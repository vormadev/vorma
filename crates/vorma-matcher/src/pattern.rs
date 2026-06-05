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
	pub(crate) num_dynamic_param_segs: usize,
}

impl Pattern {
	pub(crate) fn new(
		original_pattern: String,
		normalized_pattern: String,
		normalized_segments: Vec<InternalSegment>,
		last_seg_type: Option<SegmentKind>,
		num_dynamic_param_segs: usize,
	) -> Self {
		let segment_count = normalized_segments.len();
		Self {
			original_pattern,
			normalized_pattern,
			normalized_segments,
			last_seg_type,
			last_seg_is_non_root_splat: last_seg_type == Some(SegmentKind::Splat)
				&& segment_count > 1,
			last_seg_is_index: last_seg_type == Some(SegmentKind::Index),
			num_dynamic_param_segs,
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
	pub(crate) fn best_match_rank(&self) -> u16 {
		match self.kind {
			SegmentKind::Static | SegmentKind::Index => SCORE_STATIC as u16,
			SegmentKind::Dynamic => SCORE_DYNAMIC as u16,
			SegmentKind::Splat => 0,
		}
	}
}

pub(crate) fn is_static(segs: &[InternalSegment]) -> bool {
	segs.iter()
		.all(|s| s.kind != SegmentKind::Splat && s.kind != SegmentKind::Dynamic)
}
