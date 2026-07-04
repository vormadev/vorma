/// Pattern-text syntax passed to [`MatcherBuilder::new`].
///
/// Options only change how *pattern text* is parsed at registration
/// time — the matching algorithms, specificity ordering, and dirty-path
/// rule are identical for every configuration. [`Options::default`]
/// (`:name` for dynamic segments, `*` for a splat, no explicit index
/// spelling) is the right choice unless an application's existing route
/// syntax predates adopting this crate and cannot change.
///
/// ```
/// use vorma_matcher::{MatcherBuilder, Options};
///
/// // A route file convention that names its index routes "index"
/// // rather than leaving them as a bare trailing slash.
/// let mut builder = MatcherBuilder::new(Options {
///     explicit_index_segment_identifier: "index".to_owned(),
///     ..Options::default()
/// })?;
/// let pattern = builder.register_pattern("/docs/index")?;
/// assert_eq!(pattern.normalized_pattern(), "/docs/");
/// # Ok::<(), String>(())
/// ```
///
/// [`MatcherBuilder::new`]: crate::MatcherBuilder::new
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Options {
	/// Character that marks a dynamic parameter segment. A pattern
	/// segment beginning with this character (for example `:user_id`
	/// under the default `:`) captures one non-empty path segment under
	/// the name that follows the prefix.
	pub dynamic_param_prefix: char,
	/// Character that spells a splat segment when it is a pattern
	/// segment's entire content (for example a lone `*` under the
	/// default). A splat must be the final segment of its pattern and
	/// captures one or more trailing path segments.
	pub splat_segment_identifier: char,
	/// Text that spells an index segment explicitly, as an alternative
	/// to the empty trailing segment (`/parent/` for an index under
	/// `parent`). Left empty (the default), only the empty-segment
	/// spelling is recognized. Set to a non-empty value (for example
	/// `"index"`, spelling `/parent/index`) when an application's route
	/// file convention names its index routes rather than leaving them
	/// bare.
	pub explicit_index_segment_identifier: String,
}

impl Default for Options {
	/// Dynamic segments spelled `:name`, a splat spelled `*`, and index
	/// segments spelled as a bare trailing slash with no explicit text.
	fn default() -> Self {
		Self {
			dynamic_param_prefix: ':',
			splat_segment_identifier: '*',
			explicit_index_segment_identifier: String::new(),
		}
	}
}
