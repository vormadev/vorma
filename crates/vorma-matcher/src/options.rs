/// Matcher configuration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Options {
	pub dynamic_param_prefix: char,
	pub splat_segment_identifier: char,
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
