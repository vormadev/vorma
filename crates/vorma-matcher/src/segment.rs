pub(crate) const SCORE_STATIC: i32 = 2;
pub(crate) const SCORE_DYNAMIC: i32 = 1;

/// One slash-separated segment of a pattern, after normalization.
///
/// Read these off [`Pattern::normalized_segments`] when an application
/// or framework needs to inspect a registered pattern's shape directly
/// — for example rendering a route table, or deriving a URL-building
/// helper from the same segments the matcher itself matched against.
/// Most applications never construct or inspect a `Segment`; they only
/// ever see the *matched* result ([`Params`], [`SplatValues`]) that
/// matching produces from a concrete request path.
///
/// [`Pattern::normalized_segments`]: crate::Pattern::normalized_segments
/// [`Params`]: crate::Params
/// [`SplatValues`]: crate::SplatValues
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Segment {
	/// This segment's canonical text. The meaning depends on
	/// [`Segment::kind`]: for [`SegmentKind::Static`] and
	/// [`SegmentKind::Index`] it is the literal path text the segment
	/// matches; for [`SegmentKind::Dynamic`] it is the captured
	/// parameter's name prefixed with `:` (for example `:user_id`,
	/// regardless of the [`Options::dynamic_param_prefix`] the pattern
	/// was registered under); for [`SegmentKind::Splat`] it is always
	/// `*`.
	///
	/// [`Options::dynamic_param_prefix`]: crate::Options::dynamic_param_prefix
	pub normalized_value: String,
	/// This segment's classification, deciding both how it matches and
	/// how much it contributes to [`compare_specificity`]'s ordering.
	///
	/// [`compare_specificity`]: crate::compare_specificity
	pub kind: SegmentKind,
}

/// Classification of one normalized pattern [`Segment`].
///
/// A segment's kind decides two things: what path text it accepts at
/// match time, and how much it weighs in [`compare_specificity`]'s
/// specificity score (static and index segments score highest, dynamic
/// scores next, splat scores lowest — see the crate-level docs for the
/// full ordering).
///
/// [`compare_specificity`]: crate::compare_specificity
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SegmentKind {
	/// A literal path segment (for example `users` in `/users/:id`).
	/// Matches only that exact text.
	Static,
	/// A dynamic parameter segment (for example `:id`). Matches any one
	/// non-empty path segment and captures it into [`Params`] under the
	/// parameter's name.
	///
	/// [`Params`]: crate::Params
	Dynamic,
	/// A splat segment (`*`), always the final segment of its pattern.
	/// Matches one or more trailing path segments and captures them
	/// into [`SplatValues`].
	///
	/// [`SplatValues`]: crate::SplatValues
	Splat,
	/// An index segment: the empty final segment produced by a
	/// trailing slash (or, under a non-default
	/// [`Options::explicit_index_segment_identifier`], that identifier
	/// spelled explicitly). An index pattern claims its parent path —
	/// see the crate-level docs on nested matching.
	///
	/// [`Options::explicit_index_segment_identifier`]: crate::Options::explicit_index_segment_identifier
	Index,
}

impl SegmentKind {
	/// Stable lowercase label for this segment kind (`"static"`,
	/// `"dynamic"`, `"splat"`, or `"index"`). Useful for logging,
	/// diagnostics, or serializing a route table without depending on
	/// this crate's `Debug` output shape.
	///
	/// ```
	/// use vorma_matcher::SegmentKind;
	///
	/// assert_eq!(SegmentKind::Dynamic.as_str(), "dynamic");
	/// ```
	pub fn as_str(self) -> &'static str {
		match self {
			Self::Static => "static",
			Self::Dynamic => "dynamic",
			Self::Splat => "splat",
			Self::Index => "index",
		}
	}
}
