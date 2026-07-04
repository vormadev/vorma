use smallvec::SmallVec;

// Inline capacity for per-request segment lists: paths beyond eight
// segments are rare, and the spill to the heap is correct, just slower.
pub(crate) type SegmentList<'p> = SmallVec<[&'p str; 8]>;

pub(crate) fn strip_trailing_slash(path: &str) -> &str {
	path.strip_suffix('/').unwrap_or(path)
}

// The uniform dirty-path rule: at most one trailing slash is tolerated
// as noise, and an empty segment never matches anything. Any doubled
// slash — mid-path or trailing — spells an empty segment, so such paths
// match nothing in either matcher.
pub(crate) fn has_doubled_slash(path: &str) -> bool {
	path.contains("//")
}

// The one segment-splitting loop; both collectors below share it so the
// two can never drift.
fn for_each_segment<'p>(path: &'p str, mut push: impl FnMut(&'p str)) {
	if path.is_empty() {
		return;
	}
	if path == "/" {
		push("");
		return;
	}

	// A manual byte scan: paths are short, so a tight loop beats the
	// per-part overhead of the std split machinery here (measured).
	let bytes = path.as_bytes();
	let start_idx = if bytes[0] == b'/' { 1 } else { 0 };
	let mut start = start_idx;
	for i in start_idx..bytes.len() {
		if bytes[i] == b'/' {
			if i > start {
				push(&path[start..i]);
			}
			start = i + 1;
		}
	}
	if start < path.len() {
		push(&path[start..]);
	}
	if path.ends_with('/') {
		push("");
	}
}

/// Split a path or pattern string into its slash-separated segments.
///
/// An empty string yields no segments; a bare `/` yields one empty
/// segment (the root); at most one trailing empty segment is ever
/// produced no matter how many trailing slashes the input has (`"/a/"`
/// and `"/a///"` both yield `["a", ""]`); and an empty segment produced
/// anywhere *other* than a lone trailing position is silently dropped
/// rather than represented (`"/a//b"` yields `["a", "b"]`, not
/// `["a", "", "b"]`). This function does not itself decide whether a
/// path is well-formed — matching's own doubled-slash rejection runs
/// as a separate check before splitting, which is why a caller working
/// directly with `parse_segments` output cannot recover "was there a
/// doubled slash here" from the segment list alone.
///
/// This is the same segment splitter matching and pattern registration
/// use internally; it is exposed for applications and framework layers
/// that need to reason about a path's segment shape directly — for
/// example rendering breadcrumbs, or deriving a nested-layout hierarchy
/// from a request path outside the matcher itself. Segments borrow
/// from `path`, so splitting costs no per-segment allocation.
///
/// ```
/// use vorma_matcher::parse_segments;
///
/// assert_eq!(parse_segments("/users/42"), vec!["users", "42"]);
/// assert_eq!(parse_segments("/"), vec![""]);
/// assert_eq!(parse_segments(""), Vec::<&str>::new());
/// assert_eq!(parse_segments("/a///"), vec!["a", ""]);
/// ```
pub fn parse_segments(path: &str) -> Vec<&str> {
	let mut segs = Vec::with_capacity(8);
	for_each_segment(path, |seg| segs.push(seg));
	segs
}

// Allocation-free collector for the matching hot paths: segments live
// inline on the stack for typical path lengths.
pub(crate) fn parse_segments_inline(path: &str) -> SegmentList<'_> {
	let mut segs = SegmentList::new();
	for_each_segment(path, |seg| segs.push(seg));
	segs
}
