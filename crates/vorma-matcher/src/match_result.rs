use std::fmt;
use std::ops::Index;
use std::sync::Arc;

use rustc_hash::FxHashMap;
use smallvec::SmallVec;

use crate::pattern::Pattern;

/// Captured dynamic route parameters.
///
/// Presents captured names and values as plain string slices; the
/// storage behind them is an implementation detail.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct Params {
	// Keys are shared from the matched pattern's own segment names, so
	// capturing a param never allocates a key.
	inner: FxHashMap<Arc<str>, String>,
}

impl Params {
	/// Value for a named parameter.
	pub fn get(&self, key: &str) -> Option<&str> {
		self.inner.get(key).map(String::as_str)
	}

	/// Whether a named parameter was captured.
	pub fn contains_key(&self, key: &str) -> bool {
		self.inner.contains_key(key)
	}

	/// Iterate over parameter name/value pairs.
	pub fn iter(&self) -> impl Iterator<Item = (&str, &str)> {
		self.inner
			.iter()
			.map(|(key, value)| (&**key, value.as_str()))
	}

	/// Number of captured parameters.
	pub fn len(&self) -> usize {
		self.inner.len()
	}

	/// Whether no parameters were captured.
	pub fn is_empty(&self) -> bool {
		self.inner.is_empty()
	}

	/// Insert one parameter value under a name.
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

	fn index(&self, key: &str) -> &String {
		&self.inner[key]
	}
}

impl<K: AsRef<str>, V: Into<String>> FromIterator<(K, V)> for Params {
	fn from_iter<T: IntoIterator<Item = (K, V)>>(entries: T) -> Self {
		let mut params = Params::default();
		for (key, value) in entries {
			params.insert(key.as_ref(), value);
		}
		params
	}
}

/// Captured splat segment values.
///
/// Presents captured segments as plain string slices; all segments of
/// one capture share a single backing buffer.
#[derive(Clone, Default, Eq, PartialEq)]
pub struct SplatValues {
	// Segments joined by '/', with per-segment byte bounds. One capture
	// costs one allocation regardless of segment count.
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

	/// Captured segment at an index.
	pub fn get(&self, index: usize) -> Option<&str> {
		self.bounds
			.get(index)
			.map(|(start, end)| &self.buffer[*start as usize..*end as usize])
	}

	/// Iterate over captured segments.
	pub fn iter(&self) -> impl Iterator<Item = &str> {
		self.bounds
			.iter()
			.map(|(start, end)| &self.buffer[*start as usize..*end as usize])
	}

	/// Number of captured segments.
	pub fn len(&self) -> usize {
		self.bounds.len()
	}

	/// Whether no segments were captured.
	pub fn is_empty(&self) -> bool {
		self.bounds.is_empty()
	}

	/// Captured segments joined by a separator.
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

	/// Captured segments as owned strings.
	pub fn to_vec(&self) -> Vec<String> {
		self.iter().map(str::to_owned).collect()
	}
}

impl fmt::Debug for SplatValues {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.debug_list().entries(self.iter()).finish()
	}
}

impl PartialEq<Vec<String>> for SplatValues {
	fn eq(&self, other: &Vec<String>) -> bool {
		self.len() == other.len() && self.iter().zip(other).all(|(a, b)| a == b)
	}
}

impl PartialEq<Vec<&str>> for SplatValues {
	fn eq(&self, other: &Vec<&str>) -> bool {
		self.len() == other.len() && self.iter().zip(other).all(|(a, b)| a == *b)
	}
}

/// Best single route match.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Match {
	/// Registered pattern that matched the path.
	pub pattern: Arc<Pattern>,
	/// Captured dynamic parameter values.
	pub params: Params,
	/// Captured splat segment values.
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

/// One matched ancestor or leaf pattern in a nested match chain.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatch {
	/// Registered pattern that matched this nested position.
	pub pattern: Arc<Pattern>,
	pub(crate) params: Params,
	pub(crate) splat_values: SplatValues,
}

impl NestedMatch {
	/// Captured dynamic parameter values for this nested match.
	pub fn params(&self) -> &Params {
		&self.params
	}

	/// Captured splat segment values for this nested match.
	pub fn splat_values(&self) -> &SplatValues {
		&self.splat_values
	}
}

/// Ordered nested match chain plus shared captures.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NestedMatches {
	/// Captured dynamic parameter values shared across the nested chain.
	pub params: Params,
	/// Captured splat segment values shared across the nested chain.
	pub splat_values: SplatValues,
	/// Ordered nested matches from outermost to innermost.
	pub matches: Vec<NestedMatch>,
}
