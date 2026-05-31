pub(crate) const SCORE_STATIC: i32 = 2;
pub(crate) const SCORE_DYNAMIC: i32 = 1;

/// Normalized route segment metadata.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Segment {
	pub normalized_value: String,
	pub kind: SegmentKind,
}

/// Kind of normalized route segment.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SegmentKind {
	Static,
	Dynamic,
	Splat,
	Index,
}

impl SegmentKind {
	pub fn as_str(self) -> &'static str {
		match self {
			Self::Static => "static",
			Self::Dynamic => "dynamic",
			Self::Splat => "splat",
			Self::Index => "index",
		}
	}
}
