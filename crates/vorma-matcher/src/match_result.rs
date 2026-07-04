use std::fmt;
use std::ops::Index;
use std::sync::Arc;

use rustc_hash::FxHashMap;
use smallvec::SmallVec;

use crate::pattern::Pattern;

/// Dynamic parameter values captured by one match, keyed by parameter
/// name.
///
/// A pattern like `/users/:user_id` captures one entry, `"user_id"` to
/// the matched path's corresponding segment, whenever it matches; a
/// pattern with no dynamic segments always produces an empty `Params`.
/// Reached via [`Match::params`], [`NestedMatch::params`], or
/// [`NestedMatches::params`] — applications never construct a `Params`
/// themselves except in tests (via [`Params::from_iter`]/[`FromIterator`]
/// or [`Params::insert`]) to build an expected value to compare
/// against. Presents captured names and values as plain string slices;
/// the storage behind them (a hash map keyed by names shared, not
/// copied, from the matched pattern) is an implementation detail.
///
/// Captured values are always non-empty strings — the crate's
/// dirty-path rule guarantees an empty path segment never matches, so
/// there is no such thing as a captured empty parameter.
///
/// ```
/// use vorma_matcher::{MatcherBuilder, Options};
///
/// let mut builder = MatcherBuilder::new(Options::default())?;
/// builder.register_pattern("/users/:user_id")?;
/// let matcher = builder.finish_flat();
///
/// let found = matcher.find_best_match("/users/42").unwrap();
/// assert_eq!(found.params.get("user_id"), Some("42"));
/// assert_eq!(&found.params["user_id"], "42"); // indexing panics if absent
/// assert_eq!(found.params.get("missing"), None);
/// # Ok::<(), String>(())
/// ```
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct Params {
	// Keys are shared from the matched pattern's own segment names, so
	// capturing a param never allocates a key.
	inner: FxHashMap<Arc<str>, String>,
}

impl Params {
	/// Captured value for `key`, or `None` if no parameter of that name
	/// was captured (either because the matched pattern has no such
	/// dynamic segment, or none matched).
	pub fn get(&self, key: &str) -> Option<&str> {
		self.inner.get(key).map(String::as_str)
	}

	/// Whether a parameter named `key` was captured.
	pub fn contains_key(&self, key: &str) -> bool {
		self.inner.contains_key(key)
	}

	/// Iterate over captured `(name, value)` pairs. Iteration order is
	/// not the pattern's declared parameter order — treat it as
	/// unordered and look values up by name.
	pub fn iter(&self) -> impl Iterator<Item = (&str, &str)> {
		self.inner
			.iter()
			.map(|(key, value)| (&**key, value.as_str()))
	}

	/// Number of captured parameters.
	pub fn len(&self) -> usize {
		self.inner.len()
	}

	/// Whether no parameters were captured — true whenever the matched
	/// pattern had no dynamic segments.
	pub fn is_empty(&self) -> bool {
		self.inner.is_empty()
	}

	/// Insert one parameter value under `key`, overwriting any existing
	/// value for that name.
	///
	/// Matching never calls this — matched results arrive fully built.
	/// It exists for tests that need to construct an expected `Params`
	/// value to compare a match result against.
	pub fn insert(&mut self, key: &str, value: impl Into<String>) {
		self.inner.insert(Arc::from(key), value.into());
	}

	pub(crate) fn reserve(&mut self, additional: usize) {
		self.inner.reserve(additional);
	}

	pub(crate) fn insert_shared(&mut self, key: &Arc<str>, value: String) {
		self.inner.insert(Arc::clone(key), value);
	}
}

impl Index<&str> for Params {
	type Output = String;

	/// Captured value for a parameter name. Panics if no parameter of
	/// that name was captured — prefer [`Params::get`] when the
	/// parameter's presence is not already guaranteed by the matched
	/// pattern's own shape.
	fn index(&self, key: &str) -> &String {
		&self.inner[key]
	}
}

impl<K: AsRef<str>, V: Into<String>> FromIterator<(K, V)> for Params {
	/// Build a `Params` from `(name, value)` pairs. Exists for tests
	/// that need to construct an expected value to compare a match
	/// result against; matching itself never uses this.
	fn from_iter<T: IntoIterator<Item = (K, V)>>(entries: T) -> Self {
		let mut params = Params::default();
		for (key, value) in entries {
			params.insert(key.as_ref(), value);
		}
		params
	}
}

/// Segment values captured by a splat (`*`) at the end of a matched
/// pattern, in path order.
///
/// A pattern like `/files/*` matching `/files/a/b/c` captures three
/// segments, `["a", "b", "c"]`, in that order. Reached via
/// [`Match::splat_values`], [`NestedMatch::splat_values`], or
/// [`NestedMatches::splat_values`]; a matched pattern with no splat
/// segment always produces an empty `SplatValues`. Presents captured
/// segments as plain string slices; the storage behind them (every
/// segment of one capture packed into a single shared buffer, rather
/// than one small allocation per segment) is an implementation detail.
///
/// Captured segments are always non-empty strings, for the same reason
/// captured [`Params`] values are: the dirty-path rule guarantees an
/// empty path segment never matches.
///
/// ```
/// use vorma_matcher::{MatcherBuilder, Options};
///
/// let mut builder = MatcherBuilder::new(Options::default())?;
/// builder.register_pattern("/files/*")?;
/// let matcher = builder.finish_flat();
///
/// let found = matcher.find_best_match("/files/a/b/c").unwrap();
/// assert_eq!(found.splat_values.len(), 3);
/// assert_eq!(found.splat_values.get(1), Some("b"));
/// assert_eq!(found.splat_values.join("/"), "a/b/c");
/// # Ok::<(), String>(())
/// ```
#[derive(Clone, Default, Eq, PartialEq)]
pub struct SplatValues {
	// Segments joined by '/', with per-segment byte bounds. One capture
	// costs one allocation for the buffer; per-segment bounds stay
	// inline up to 8 segments, spilling to the heap only beyond that.
	buffer: String,
	bounds: SmallVec<[(u32, u32); 8]>,
}

impl SplatValues {
	pub(crate) fn from_segments(segments: &[&str]) -> Self {
		if segments.is_empty() {
			return Self::default();
		}
		let total: usize = segments.iter().map(|seg| seg.len()).sum::<usize>() + segments.len() - 1;
		let mut buffer = String::with_capacity(total);
		let mut bounds = SmallVec::with_capacity(segments.len());
		for (i, seg) in segments.iter().enumerate() {
			if i > 0 {
				buffer.push('/');
			}
			let start = buffer.len() as u32;
			buffer.push_str(seg);
			bounds.push((start, buffer.len() as u32));
		}
		Self { buffer, bounds }
	}

	/// Captured segment at `index` (0 is the first segment after the
	/// splat), or `None` if `index` is out of bounds.
	pub fn get(&self, index: usize) -> Option<&str> {
		self.bounds
			.get(index)
			.map(|(start, end)| &self.buffer[*start as usize..*end as usize])
	}

	/// Iterate over captured segments, in path order.
	pub fn iter(&self) -> impl Iterator<Item = &str> {
		self.bounds
			.iter()
			.map(|(start, end)| &self.buffer[*start as usize..*end as usize])
	}

	/// Number of captured segments.
	pub fn len(&self) -> usize {
		self.bounds.len()
	}

	/// Whether no segments were captured — true whenever the matched
	/// pattern had no splat segment.
	pub fn is_empty(&self) -> bool {
		self.bounds.is_empty()
	}

	/// Captured segments rejoined with `separator` between them (for
	/// example to rebuild a filesystem path from a `/files/*` capture).
	pub fn join(&self, separator: &str) -> String {
		if separator == "/" {
			return self.buffer.clone();
		}
		let mut out = String::new();
		for (i, segment) in self.iter().enumerate() {
			if i > 0 {
				out.push_str(separator);
			}
			out.push_str(segment);
		}
		out
	}

	/// Captured segments as a freshly allocated `Vec<String>`, one
	/// element per segment. Prefer [`SplatValues::iter`] or
	/// [`SplatValues::get`] when an owned `Vec` is not specifically
	/// needed — they read straight from the shared backing buffer.
	pub fn to_vec(&self) -> Vec<String> {
		self.iter().map(str::to_owned).collect()
	}
}

impl fmt::Debug for SplatValues {
	/// Renders as a list of the captured segments, matching how a
	/// `Vec<&str>` of the same segments would render.
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_list().entries(self.iter()).finish()
	}
}

impl PartialEq<Vec<String>> for SplatValues {
	/// Compares element-for-element against an owned `Vec<String>`, so
	/// test assertions can write `splat_values == vec!["a".to_owned()]`
	/// directly, without an intermediate [`SplatValues::to_vec`] call.
	fn eq(&self, other: &Vec<String>) -> bool {
		self.len() == other.len() && self.iter().zip(other).all(|(a, b)| a == b)
	}
}

impl PartialEq<Vec<&str>> for SplatValues {
	/// Compares element-for-element against a `Vec<&str>`, so test
	/// assertions can write `splat_values == vec!["a", "b"]` directly.
	fn eq(&self, other: &Vec<&str>) -> bool {
		self.len() == other.len() && self.iter().zip(other).all(|(a, b)| a == *b)
	}
}

/// The single best-matching pattern for one request path, returned by
/// [`FlatMatcher::find_best_match`], with everything it captured.
///
/// [`FlatMatcher::find_best_match`]: crate::FlatMatcher::find_best_match
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Match {
	/// The registered pattern that matched.
	pub pattern: Arc<Pattern>,
	/// Dynamic parameter values captured from the matched path.
	pub params: Params,
	/// Splat segment values captured from the matched path.
	pub splat_values: SplatValues,
}

impl Match {
	pub(crate) fn from_registered(pattern: Arc<Pattern>) -> Self {
		Self {
			pattern,
			params: Params::default(),
			splat_values: SplatValues::default(),
		}
	}
}

/// One matched pattern at one position — ancestor layout or innermost
/// leaf — in a [`NestedMatcher`]'s resolved chain.
///
/// Every entry in [`NestedMatches::matches`] is a `NestedMatch`. Its own
/// [`NestedMatch::params`] and [`NestedMatch::splat_values`] hold
/// exactly *this pattern's own* declared captures — a `:tenant_id`
/// dynamic segment on an outer layout appears in that layout's own
/// `NestedMatch`, not in a descendant's, and a leaf pattern with no
/// dynamic segments of its own has empty captures here even when an
/// ancestor captured something. Reach for the per-position view when a
/// specific pattern's own parameter matters in isolation; reach for
/// [`NestedMatches::params`]/[`NestedMatches::splat_values`] for the
/// convenience view that already has every captured name merged
/// together, as most call sites want.
///
/// [`NestedMatcher`]: crate::NestedMatcher
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatch {
	/// The registered pattern that matched at this position.
	pub pattern: Arc<Pattern>,
	pub(crate) params: Params,
	pub(crate) splat_values: SplatValues,
}

impl NestedMatch {
	/// Dynamic parameter values declared by this position's own
	/// pattern — not merged with any other position in the chain.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Splat segment values declared by this position's own pattern —
	/// not merged with any other position in the chain. Empty unless
	/// this exact pattern ends in a splat segment (only the chain's
	/// last entry ever can, since a splat is required to be a
	/// pattern's final segment).
	pub fn splat_values(&self) -> &SplatValues {
		&self.splat_values
	}
}

/// The ordered nested match chain for one request path, returned by
/// [`NestedMatcher::find_nested_matches`], plus a convenience view of
/// every parameter captured along the way.
///
/// [`NestedMatches::matches`] is the chain itself, outermost first,
/// innermost (the leaf that actually renders) last.
/// [`NestedMatches::params`] and [`NestedMatches::splat_values`] are
/// exactly the innermost entry's own
/// [`NestedMatch::params`]/[`NestedMatch::splat_values`]. Because a
/// nested route's own pattern text already contains every ancestor
/// segment as a literal prefix (a leaf registered as
/// `/:tenant_id/posts/:post_id` declares `tenant_id` itself, not just
/// its ancestor `/:tenant_id` layout), the leaf's own captures are
/// already every parameter captured anywhere in the chain — an
/// application that just wants "all the parameters this request
/// captured" reads them here without walking `matches` itself.
///
/// [`NestedMatcher::find_nested_matches`]: crate::NestedMatcher::find_nested_matches
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatches {
	/// Convenience alias for the innermost entry's own captured
	/// parameters. See the type-level docs.
	pub params: Params,
	/// Convenience alias for the innermost entry's own captured splat
	/// values. See the type-level docs.
	pub splat_values: SplatValues,
	/// The matched chain, ordered from the outermost ancestor to the
	/// innermost leaf.
	pub matches: Vec<NestedMatch>,
}
