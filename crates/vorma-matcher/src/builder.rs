use std::collections::{HashMap, HashSet};

use crate::matcher::Matcher;
use crate::options::Options;
use crate::parse::parse_segments;
use crate::pattern::{InternalSegment, Pattern, is_static};
use crate::segment::{SCORE_DYNAMIC, SCORE_STATIC, SegmentKind};
use crate::tree::SegmentNode;

/// Validates and registers route patterns before producing an immutable matcher.
#[derive(Clone, Debug)]
pub struct MatcherBuilder {
	static_patterns: HashMap<String, Pattern>,
	dynamic_patterns: HashMap<String, Pattern>,
	root_node: SegmentNode,

	explicit_index_segment: String,
	dynamic_param_prefix: char,
	splat_segment_id: char,
	slash_index_segment: String,
	using_explicit_index_segment: bool,
}

impl MatcherBuilder {
	pub fn new(opts: Options) -> Result<Self, String> {
		validate_options(&opts)?;
		let using_explicit_index_segment = !opts.explicit_index_segment_identifier.is_empty();
		let slash_index_segment = format!("/{}", opts.explicit_index_segment_identifier);
		Ok(Self {
			static_patterns: HashMap::new(),
			dynamic_patterns: HashMap::new(),
			root_node: SegmentNode::default(),
			explicit_index_segment: opts.explicit_index_segment_identifier,
			dynamic_param_prefix: opts.dynamic_param_prefix,
			splat_segment_id: opts.splat_segment_identifier,
			slash_index_segment,
			using_explicit_index_segment,
		})
	}

	pub fn explicit_index_segment_identifier(&self) -> &str {
		&self.explicit_index_segment
	}

	pub fn dynamic_param_prefix(&self) -> char {
		self.dynamic_param_prefix
	}

	pub fn splat_segment_identifier(&self) -> char {
		self.splat_segment_id
	}

	pub fn normalize_pattern(&self, original: &str) -> Result<Pattern, String> {
		self.validate_pattern_text(original)?;

		let mut normalized = original.to_string();

		if self.using_explicit_index_segment {
			if normalized.ends_with('/') {
				if normalized != "/" {
					return Err(format!(
						"Error with pattern '{}'. With the exception of any absolute root pattern ('/'), trailing slashes are not permitted when using an explicit index segment.",
						original
					));
				}
				normalized = normalized.trim_end_matches('/').to_string();
			}
			if normalized.ends_with(&self.slash_index_segment) {
				let len = normalized.len() - self.explicit_index_segment.len();
				normalized.truncate(len);
			}
		}

		let raw_segments = parse_segments(&normalized);
		let mut segments = Vec::with_capacity(raw_segments.len());
		let mut num_dynamic = 0usize;

		let mut seen_params = HashSet::new();
		let raw_segment_count = raw_segments.len();

		for (i, segment) in raw_segments.into_iter().enumerate() {
			let segment_kind = self.classify_segment(&segment);
			let normalized_value = match segment_kind {
				SegmentKind::Dynamic => {
					num_dynamic += 1;
					let param_name = &segment[self.dynamic_param_prefix.len_utf8()..];
					validate_param_name(original, param_name)?;
					if !seen_params.insert(param_name.to_owned()) {
						return Err(format!(
							"invalid route pattern \"{original}\": duplicate param name \"{param_name}\""
						));
					}
					format!(":{param_name}")
				}
				SegmentKind::Splat => "*".to_string(),
				_ => segment,
			};
			if segment_kind == SegmentKind::Splat && i + 1 != raw_segment_count {
				return Err(format!(
					"invalid route pattern \"{original}\": splat segment must be the final segment"
				));
			}
			segments.push(InternalSegment {
				normalized_value,
				kind: segment_kind,
			});
		}

		let last_type = segments.last().map(|seg| seg.kind);
		let mut final_pattern = String::from("/");
		for (i, seg) in segments.iter().enumerate() {
			final_pattern.push_str(&seg.normalized_value);
			if i < segments.len() - 1 {
				final_pattern.push('/');
			}
		}
		if final_pattern.ends_with('/') && last_type != Some(SegmentKind::Index) {
			while final_pattern.ends_with('/') {
				final_pattern.pop();
			}
		}

		Ok(Pattern::new(
			original.to_string(),
			final_pattern,
			segments,
			last_type,
			num_dynamic,
		))
	}

	pub fn register_pattern(&mut self, original: &str) -> Result<Pattern, String> {
		let rp = self.normalize_pattern(original)?;

		for store in [&self.static_patterns, &self.dynamic_patterns] {
			if let Some(existing) = store.get(rp.normalized_pattern()) {
				if existing.original_pattern() == original {
					return Ok(existing.clone());
				}
				return Err(format!(
					"normalized pattern collision: \"{}\" and \"{}\" both normalize to \"{}\"",
					original,
					existing.original_pattern(),
					rp.normalized_pattern()
				));
			}
			if !is_static(&rp.normalized_segments) {
				let shape_key = rp.shape_key();
				for existing in store.values() {
					if existing.shape_key() == shape_key {
						return Err(format!(
							"route shape collision: \"{}\" and \"{}\" both match the same paths",
							original,
							existing.original_pattern()
						));
					}
				}
			}
		}

		if is_static(&rp.normalized_segments) {
			self.static_patterns
				.insert(rp.normalized_pattern.clone(), rp.clone());
			return Ok(rp);
		}

		self.dynamic_patterns
			.insert(rp.normalized_pattern.clone(), rp.clone());

		let mut current = &mut self.root_node;
		let mut node_score = 0i32;
		for (i, seg) in rp.normalized_segments.iter().enumerate() {
			let child = current.find_or_create_child(&seg.normalized_value);
			match seg.kind {
				SegmentKind::Dynamic => node_score += SCORE_DYNAMIC,
				SegmentKind::Splat => {}
				_ => node_score += SCORE_STATIC,
			}
			if i == rp.normalized_segments.len() - 1 {
				child.pattern = rp.normalized_pattern.clone();
				child.final_score = node_score;
			}
			current = child;
		}

		Ok(rp)
	}

	pub fn finish(self) -> Matcher {
		Matcher::from_parts(
			self.static_patterns,
			self.dynamic_patterns,
			self.root_node,
			self.explicit_index_segment,
			self.dynamic_param_prefix,
			self.splat_segment_id,
		)
	}

	fn validate_pattern_text(&self, pattern: &str) -> Result<(), String> {
		if pattern.is_empty() {
			return Ok(());
		}
		if !pattern.starts_with('/') {
			return Err(format!(
				"invalid route pattern \"{pattern}\": pattern must be absolute"
			));
		}
		if pattern != "/" && pattern.contains("//") {
			return Err(format!(
				"invalid route pattern \"{pattern}\": empty middle segments are not permitted"
			));
		}
		Ok(())
	}

	fn classify_segment(&self, seg: &str) -> SegmentKind {
		if seg.is_empty() {
			SegmentKind::Index
		} else if seg.chars().count() == 1 && seg.starts_with(self.splat_segment_id) {
			SegmentKind::Splat
		} else if seg.starts_with(self.dynamic_param_prefix) {
			SegmentKind::Dynamic
		} else {
			SegmentKind::Static
		}
	}
}

fn validate_options(opts: &Options) -> Result<(), String> {
	if opts.dynamic_param_prefix == '/' {
		return Err("dynamic param prefix cannot be a slash".to_string());
	}
	if opts.splat_segment_identifier == '/' {
		return Err("splat segment identifier cannot be a slash".to_string());
	}
	if opts.dynamic_param_prefix == opts.splat_segment_identifier {
		return Err("dynamic param prefix and splat segment identifier must differ".to_string());
	}
	if opts.explicit_index_segment_identifier.contains('/') {
		return Err("explicit index segment cannot contain a slash".to_string());
	}
	if !opts.explicit_index_segment_identifier.is_empty()
		&& (opts
			.explicit_index_segment_identifier
			.starts_with(opts.dynamic_param_prefix)
			|| opts.explicit_index_segment_identifier == opts.splat_segment_identifier.to_string())
	{
		return Err("explicit index segment cannot be classified as dynamic or splat".to_string());
	}

	Ok(())
}

fn validate_param_name(pattern: &str, name: &str) -> Result<(), String> {
	if name.is_empty() {
		return Err(format!(
			"invalid route pattern \"{pattern}\": param name cannot be empty"
		));
	}

	let mut chars = name.chars();
	let Some(first) = chars.next() else {
		return Err(format!(
			"invalid route pattern \"{pattern}\": param name cannot be empty"
		));
	};
	if !(first == '_' || first.is_ascii_alphabetic()) {
		return Err(format!(
			"invalid route pattern \"{pattern}\": param name \"{name}\" must start with an ASCII letter or underscore"
		));
	}
	if !chars.all(|ch| ch == '_' || ch.is_ascii_alphanumeric()) {
		return Err(format!(
			"invalid route pattern \"{pattern}\": param name \"{name}\" must contain only ASCII letters, digits, or underscores"
		));
	}

	Ok(())
}
