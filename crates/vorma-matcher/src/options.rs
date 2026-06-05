/// Matcher configuration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Options {
	/// Prefix that marks a dynamic parameter segment.
	pub dynamic_param_prefix: char,
	/// Single-character segment that captures the remaining path.
	pub splat_segment_identifier: char,
	/// Optional segment identifier normalized as an index route.
	pub explicit_index_segment_identifier: String,
}

impl Default for Options {
	fn default() -> Self {
		Self {
			dynamic_param_prefix: ':',
			splat_segment_identifier: '*',
			explicit_index_segment_identifier: String::new(),
		}
	}
}
