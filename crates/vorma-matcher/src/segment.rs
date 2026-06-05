pub(crate) const SCORE_STATIC: i32 = 2;
pub(crate) const SCORE_DYNAMIC: i32 = 1;

/// Normalized route segment metadata.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Segment {
	/// Canonical segment text used by the matcher.
	pub normalized_value: String,
	/// Segment classification.
	pub kind: SegmentKind,
}

/// Kind of normalized route segment.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SegmentKind {
	/// Literal static path segment.
	Static,
	/// Dynamic parameter segment.
	Dynamic,
	/// Splat segment that captures the remaining path.
	Splat,
	/// Index segment.
	Index,
}

impl SegmentKind {
	/// Stable lowercase label for this segment kind.
	pub fn as_str(self) -> &'static str {
		match self {
			Self::Static => "static",
			Self::Dynamic => "dynamic",
			Self::Splat => "splat",
			Self::Index => "index",
		}
	}
}
