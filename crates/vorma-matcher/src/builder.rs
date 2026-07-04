use std::collections::HashSet;
use std::sync::Arc;

use rustc_hash::FxHashMap;

use crate::matcher::{FlatMatcher, MatcherEngine, NestedMatcher};
use crate::options::Options;
use crate::parse::parse_segments;
use crate::pattern::{InternalSegment, Pattern, is_static};
use crate::segment::SegmentKind;
use crate::tree::SegmentNode;

/// Accumulates and validates route patterns, then finishes into an
/// immutable [`FlatMatcher`] or [`NestedMatcher`].
///
/// This is the crate's entry point: construct one with [`MatcherBuilder::new`],
/// call [`MatcherBuilder::register_pattern`] once per route pattern the
/// application owns, then call [`MatcherBuilder::finish_flat`] or
/// [`MatcherBuilder::finish_nested`] to produce the matcher that will
/// actually serve requests. Registration is where mistakes in pattern
/// text are caught — a malformed pattern, a duplicate parameter name, or
/// two patterns that collide (normalize to the same text, or share an
/// unresolvable specificity tie over some path) all fail here, at
/// startup, rather than surfacing as a request-time surprise.
///
/// A builder is one-shot by design: [`MatcherBuilder::finish_flat`] and
/// [`MatcherBuilder::finish_nested`] both consume `self`. There is no
/// mutation after finishing — a live matcher is immutable, so its
/// matching algorithms never need to account for a route table changing
/// underneath them.
#[derive(Clone, Debug)]
pub struct MatcherBuilder {
	static_patterns: FxHashMap<String, Arc<Pattern>>,
	dynamic_patterns: FxHashMap<String, Arc<Pattern>>,
	root_node: SegmentNode,

	explicit_index_segment: String,
	dynamic_param_prefix: char,
	splat_segment_id: char,
	slash_index_segment: String,
	using_explicit_index_segment: bool,
}

impl MatcherBuilder {
	/// Create an empty builder configured with `opts`.
	///
	/// Fails if `opts` is internally inconsistent: the dynamic prefix or
	/// splat identifier is itself a slash, the two are set to the same
	/// character, or a non-empty explicit index segment contains a
	/// slash or would itself be classified as dynamic or splat under
	/// the other two settings. None of these depend on any pattern, so
	/// they are the only mistakes this constructor itself can catch —
	/// [`Options::default`] is always internally consistent.
	///
	/// ```
	/// use vorma_matcher::{MatcherBuilder, Options};
	///
	/// let builder = MatcherBuilder::new(Options::default())?;
	/// # Ok::<(), String>(())
	/// ```
	pub fn new(opts: Options) -> Result<Self, String> {
		validate_options(&opts)?;
		let using_explicit_index_segment = !opts.explicit_index_segment_identifier.is_empty();
		let slash_index_segment = format!("/{}", opts.explicit_index_segment_identifier);
		Ok(Self {
			static_patterns: FxHashMap::default(),
			dynamic_patterns: FxHashMap::default(),
			root_node: SegmentNode::default(),
			explicit_index_segment: opts.explicit_index_segment_identifier,
			dynamic_param_prefix: opts.dynamic_param_prefix,
			splat_segment_id: opts.splat_segment_identifier,
			slash_index_segment,
			using_explicit_index_segment,
		})
	}

	/// This builder's configured [`Options::explicit_index_segment_identifier`].
	///
	/// [`Options::explicit_index_segment_identifier`]: crate::Options::explicit_index_segment_identifier
	pub fn explicit_index_segment_identifier(&self) -> &str {
		&self.explicit_index_segment
	}

	/// This builder's configured [`Options::dynamic_param_prefix`].
	///
	/// [`Options::dynamic_param_prefix`]: crate::Options::dynamic_param_prefix
	pub fn dynamic_param_prefix(&self) -> char {
		self.dynamic_param_prefix
	}

	/// This builder's configured [`Options::splat_segment_identifier`].
	///
	/// [`Options::splat_segment_identifier`]: crate::Options::splat_segment_identifier
	pub fn splat_segment_identifier(&self) -> char {
		self.splat_segment_id
	}

	/// Validate and normalize one pattern's text without registering it
	/// on this builder.
	///
	/// Reach for this over [`MatcherBuilder::register_pattern`] when an
	/// application needs a [`Pattern`]'s normalized shape (to display,
	/// to key some external structure by, to feed into
	/// [`compare_specificity`]) without the collision checks or side
	/// effects of actually adding it to the matcher being built —
	/// [`normalize_pattern`] never fails on a duplicate or a colliding
	/// shape, only on malformed pattern text itself.
	///
	/// [`normalize_pattern`]: MatcherBuilder::normalize_pattern
	/// [`compare_specificity`]: crate::compare_specificity
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

		let mut seen_params = HashSet::new();
		let raw_segment_count = raw_segments.len();

		for (i, segment) in raw_segments.into_iter().enumerate() {
			let segment_kind = self.classify_segment(segment);
			let normalized_value = match segment_kind {
				SegmentKind::Dynamic => {
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
				_ => segment.to_owned(),
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
		))
	}

	/// Validate, normalize, and register one route pattern.
	///
	/// Registering the same pattern text (byte-for-byte, as originally
	/// written) twice is a no-op that returns the already-registered
	/// [`Pattern`]. Registering two *different* pattern texts that
	/// normalize to the same [`Pattern::normalized_pattern`] fails —
	/// applications should register each logical route exactly once.
	///
	/// Registering two dynamic-or-splat patterns of identical shape
	/// (same [`compare_specificity`] tie, over the paths they could
	/// share) also fails, as a route shape collision: the crate-level
	/// docs on specificity explain why such a tie has no principled
	/// winner. Static-vs-static and shape-distinguishable
	/// dynamic-vs-dynamic overlaps are always allowed — overlap between
	/// registered patterns is the normal condition; only an
	/// unresolvable tie is rejected. (An index pattern also claims its
	/// parent path with no distinct sibling required — this crate does
	/// not reject a GET/HEAD-style resource-versus-view tie itself;
	/// that additional rejection, where it applies, belongs to a layer
	/// built on top of this crate that knows about HTTP methods.)
	///
	/// ```
	/// use vorma_matcher::{MatcherBuilder, Options};
	///
	/// let mut builder = MatcherBuilder::new(Options::default())?;
	/// builder.register_pattern("/users/:user_id")?;
	/// // Two different spellings that normalize to the same text collide.
	/// assert!(builder.register_pattern("/users/:id").is_err());
	/// # Ok::<(), String>(())
	/// ```
	///
	/// [`compare_specificity`]: crate::compare_specificity
	pub fn register_pattern(&mut self, original: &str) -> Result<Pattern, String> {
		let rp = self.normalize_pattern(original)?;

		for store in [&self.static_patterns, &self.dynamic_patterns] {
			if let Some(existing) = store.get(rp.normalized_pattern()) {
				if existing.original_pattern() == original {
					return Ok(Pattern::clone(existing));
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

		let arc = Arc::new(rp.clone());
		let store = if rp.is_static {
			&mut self.static_patterns
		} else {
			&mut self.dynamic_patterns
		};
		store.insert(rp.normalized_pattern.clone(), Arc::clone(&arc));

		// Every pattern enters the tree — static chains included — so
		// both matchers walk one structure and read candidates straight
		// off the nodes.
		let mut current = &mut self.root_node;
		for (i, seg) in rp.normalized_segments.iter().enumerate() {
			let child = current.find_or_create_child(&seg.normalized_value);
			if i == rp.normalized_segments.len() - 1 {
				child.registered = Some(Arc::clone(&arc));
			}
			current = child;
		}
		if rp.normalized_segments.is_empty() {
			self.root_node.registered = Some(arc);
		}

		Ok(rp)
	}

	/// Consume this builder and produce an immutable [`FlatMatcher`]: one
	/// request path resolves to the single best-matching registered
	/// pattern. This is the shape a plain HTTP router wants.
	pub fn finish_flat(self) -> FlatMatcher {
		FlatMatcher::from_engine(self.into_engine())
	}

	/// Consume this builder and produce an immutable [`NestedMatcher`]:
	/// one request path resolves to an ordered chain of registered
	/// patterns, from outermost to innermost. This is the shape a
	/// file-system-routed, layout-nesting framework wants.
	pub fn finish_nested(self) -> NestedMatcher {
		NestedMatcher::from_engine(self.into_engine())
	}

	pub(crate) fn into_engine(self) -> MatcherEngine {
		MatcherEngine::from_parts(self.static_patterns, self.dynamic_patterns, self.root_node)
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

	// `name` is non-empty past the guard above, so a first char always
	// exists.
	let mut chars = name.chars();
	let first = chars.next().expect("non-empty name has a first char");
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
