pub(crate) fn strip_trailing_slash(path: &str) -> &str {
	path.strip_suffix('/').unwrap_or(path)
}

pub fn parse_segments(path: &str) -> Vec<String> {
	if path.is_empty() {
		return Vec::new();
	}
	if path == "/" {
		return vec![String::new()];
	}

	let bytes = path.as_bytes();
	let start_idx = if bytes[0] == b'/' { 1 } else { 0 };
	let mut segs = Vec::new();
	let mut start = start_idx;
	for i in start_idx..bytes.len() {
		if bytes[i] == b'/' {
			if i > start {
				segs.push(path[start..i].to_string());
			}
			start = i + 1;
		}
	}
	if start < path.len() {
		segs.push(path[start..].to_string());
	}
	if path.ends_with('/') {
		segs.push(String::new());
	}
	segs
}
