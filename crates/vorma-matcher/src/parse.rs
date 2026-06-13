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

/// Split a path or pattern into slash-separated segments.
///
/// Segments borrow from the input; matching pays no per-segment
/// allocation.
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
