use std::cmp::Ordering;
use std::collections::HashMap;

use crate::builder::MatcherBuilder;
use crate::match_result::{Match, NestedMatch, NestedMatches, Params};
use crate::options::Options;
use crate::parse::{parse_segments, strip_trailing_slash};
use crate::pattern::Pattern;
use crate::segment::{SCORE_DYNAMIC, SCORE_STATIC, SegmentKind};
use crate::tree::{NodeType, SegmentNode};

/// Route matcher for exact, dynamic, splat, and nested view matching.
#[derive(Clone, Debug)]
pub struct Matcher {
	static_patterns: HashMap<String, Pattern>,
	dynamic_patterns: HashMap<String, Pattern>,
	root_node: SegmentNode,

	explicit_index_segment: String,
	dynamic_param_prefix: char,
	splat_segment_id: char,
}

impl Matcher {
	/// Create a builder that validates and registers patterns before matching.
	pub fn builder(opts: Options) -> Result<MatcherBuilder, String> {
		MatcherBuilder::new(opts)
	}

	pub(crate) fn from_parts(
		static_patterns: HashMap<String, Pattern>,
		dynamic_patterns: HashMap<String, Pattern>,
		root_node: SegmentNode,
		explicit_index_segment: String,
		dynamic_param_prefix: char,
		splat_segment_id: char,
	) -> Self {
		Self {
			static_patterns,
			dynamic_patterns,
			root_node,
			explicit_index_segment,
			dynamic_param_prefix,
			splat_segment_id,
		}
	}

	/// Configured explicit index segment identifier.
	pub fn explicit_index_segment_identifier(&self) -> &str {
		&self.explicit_index_segment
	}

	/// Configured dynamic parameter prefix.
	pub fn dynamic_param_prefix(&self) -> char {
		self.dynamic_param_prefix
	}

	/// Configured splat segment identifier.
	pub fn splat_segment_identifier(&self) -> char {
		self.splat_segment_id
	}

	/// Find the best single pattern match for a concrete path.
	pub fn find_best_match(&self, real_path: &str) -> Option<Match> {
		if let Some(rr) = self.static_patterns.get(real_path) {
			return Some(Match::from_registered(rr.clone(), 0));
		}

		let segments = parse_segments(real_path);
		let has_trailing = !real_path.is_empty() && real_path.ends_with('/');

		if has_trailing {
			let without = &real_path[..real_path.len() - 1];
			if let Some(rr) = self.static_patterns.get(without) {
				return Some(Match::from_registered(rr.clone(), 0));
			}
		}

		let mut best = self.find_best_dynamic_match(&segments, has_trailing)?;

		if best.pattern.num_dynamic_param_segs > 0 {
			let mut params = Params::with_capacity(best.pattern.num_dynamic_param_segs);
			for (i, seg) in best.pattern.normalized_segments.iter().enumerate() {
				if seg.kind == SegmentKind::Dynamic
					&& let Some(value) = segments.get(i)
				{
					params.insert(seg.normalized_value[1..].to_string(), value.clone());
				}
			}
			best.params = params;
		}

		if best.pattern.normalized_pattern == "/*" || best.pattern.last_seg_is_non_root_splat {
			let start = best.pattern.normalized_segments.len() - 1;
			best.splat_values = segments[start..].to_vec();
		}

		Some(best)
	}

	/// Find the ordered nested pattern chain for a concrete path.
	pub fn find_nested_matches(&self, real_path: &str) -> Option<NestedMatches> {
		let real_path = strip_trailing_slash(real_path).to_string();
		let real_segs = parse_segments(&real_path);
		let real_segs_len = real_segs.len();
		let mut matches: HashMap<String, NestedMatch> = HashMap::new();

		let has_empty = if let Some(empty_rr) = self.static_patterns.get("") {
			matches.insert(
				empty_rr.normalized_pattern.clone(),
				NestedMatch::from_registered(empty_rr.clone()),
			);
			true
		} else {
			false
		};

		if real_path.is_empty() {
			if let Some(rr) = self.static_patterns.get("/") {
				matches.insert(
					rr.normalized_pattern.clone(),
					NestedMatch::from_registered(rr.clone()),
				);
			} else if let Some(rr) = self.dynamic_patterns.get("/*") {
				matches.insert(
					"/*".to_string(),
					NestedMatch {
						pattern: rr.clone(),
						params: Params::new(),
						splat_values: Vec::new(),
					},
				);
			}
			return flatten_and_sort(matches, &real_path, real_segs_len);
		}

		let mut pb = String::with_capacity(real_path.len() + 1);
		let mut found_full_static = false;
		for (i, seg) in real_segs.iter().enumerate() {
			pb.push('/');
			pb.push_str(seg);
			if let Some(rr) = self.static_patterns.get(&pb) {
				matches.insert(
					rr.normalized_pattern.clone(),
					NestedMatch::from_registered(rr.clone()),
				);
				if i == real_segs_len - 1 {
					found_full_static = true;
				}
			}
			if i == real_segs_len - 1 {
				pb.push('/');
				if let Some(rr) = self.static_patterns.get(&pb) {
					matches.insert(
						rr.normalized_pattern.clone(),
						NestedMatch::from_registered(rr.clone()),
					);
				}
			}
		}

		if !found_full_static {
			if let Some(rr) = self.dynamic_patterns.get("/*") {
				matches.insert(
					"/*".to_string(),
					NestedMatch {
						pattern: rr.clone(),
						params: Params::new(),
						splat_values: real_segs.clone(),
					},
				);
			}
			self.collect_nested_dynamic_matches(&real_segs, &mut matches);
		}

		if matches.contains_key("/*") {
			if has_empty {
				if matches.len() > 2 {
					matches.remove("/*");
				}
			} else if matches.len() > 1 {
				matches.remove("/*");
			}
		}

		if matches.len() < 2 {
			return flatten_and_sort(matches, &real_path, real_segs_len);
		}

		prune_nested_matches(&mut matches, real_segs_len);
		flatten_and_sort(matches, &real_path, real_segs_len)
	}

	fn find_best_dynamic_match(&self, segments: &[String], check_trailing: bool) -> Option<Match> {
		let mut best = None;
		let mut stack = vec![(&self.root_node, 0usize, 0u32)];
		while let Some((node, depth, score)) = stack.pop() {
			let at_normal_end = check_trailing && depth == segments.len().saturating_sub(1);

			if !node.pattern.is_empty()
				&& let Some(rp) = self.dynamic_patterns.get(&node.pattern)
				&& (depth == segments.len() || node.node_type == NodeType::Splat || at_normal_end)
			{
				let candidate = Match::from_registered(rp.clone(), score);
				if best
					.as_ref()
					.is_none_or(|current| candidate.better_than(current))
				{
					best = Some(candidate);
				}
			}

			if depth >= segments.len() {
				continue;
			}

			if let Some(child) = node.children.get(&segments[depth]) {
				stack.push((child, depth + 1, score + SCORE_STATIC as u32));
			}

			for child in &node.dyn_children {
				match child.node_type {
					NodeType::Dynamic => {
						if !segments[depth].is_empty() {
							stack.push((child, depth + 1, score + SCORE_DYNAMIC as u32));
						}
					}
					NodeType::Splat => {
						if !child.pattern.is_empty()
							&& let Some(rp) = self.dynamic_patterns.get(&child.pattern)
						{
							let candidate = Match::from_registered(rp.clone(), score);
							if best
								.as_ref()
								.is_none_or(|current| candidate.better_than(current))
							{
								best = Some(candidate);
							}
						}
					}
					NodeType::Static => {}
				}
			}
		}
		best
	}

	fn collect_nested_dynamic_matches(
		&self,
		segments: &[String],
		matches: &mut HashMap<String, NestedMatch>,
	) {
		let mut stack = vec![(&self.root_node, 0usize, Params::new())];
		while let Some((node, depth, params)) = stack.pop() {
			if !node.pattern.is_empty()
				&& let Some(rp) = self.dynamic_patterns.get(&node.pattern)
				&& node.pattern != "/*"
			{
				let splat_values = if node.node_type == NodeType::Splat && depth < segments.len() {
					segments[depth..].to_vec()
				} else {
					Vec::new()
				};

				matches.insert(
					node.pattern.clone(),
					NestedMatch {
						pattern: rp.clone(),
						params: params.clone(),
						splat_values,
					},
				);

				if depth == segments.len() {
					let idx_pattern = format!("{}/", node.pattern);
					if let Some(irp) = self.dynamic_patterns.get(&idx_pattern) {
						matches.insert(
							idx_pattern,
							NestedMatch {
								pattern: irp.clone(),
								params: params.clone(),
								splat_values: Vec::new(),
							},
						);
					}
				}
			}

			if depth >= segments.len() {
				continue;
			}

			let seg = &segments[depth];
			if let Some(child) = node.children.get(seg) {
				stack.push((child, depth + 1, params.clone()));
			}

			for child in &node.dyn_children {
				match child.node_type {
					NodeType::Dynamic => {
						let mut child_params = params.clone();
						child_params.insert(child.param_name.clone(), seg.clone());
						stack.push((child, depth + 1, child_params));
					}
					NodeType::Splat => {
						stack.push((child, depth, params.clone()));
					}
					NodeType::Static => {}
				}
			}
		}
	}
}

fn prune_nested_matches(matches: &mut HashMap<String, NestedMatch>, real_segs_len: usize) {
	let mut longest_len = 0usize;
	let mut longest_index: Option<String> = None;
	let mut longest_dynamic: Option<String> = None;
	let mut longest_splat: Option<String> = None;
	for (pat, m) in matches.iter() {
		let seg_len = m.pattern.normalized_segments.len();
		if seg_len > longest_len {
			longest_len = seg_len;
			longest_index = None;
			longest_dynamic = None;
			longest_splat = None;
		}
		if seg_len == longest_len {
			match m.pattern.last_seg_type {
				Some(SegmentKind::Index) => longest_index = Some(pat.clone()),
				Some(SegmentKind::Dynamic) => longest_dynamic = Some(pat.clone()),
				Some(SegmentKind::Splat) => longest_splat = Some(pat.clone()),
				_ => {}
			}
		}
	}

	let shorter_to_remove: Vec<String> = matches
		.iter()
		.filter_map(|(pat, m)| {
			if m.pattern.normalized_segments.len() < longest_len
				&& (m.pattern.last_seg_is_non_root_splat || m.pattern.last_seg_is_index)
			{
				Some(pat.clone())
			} else {
				None
			}
		})
		.collect();
	for pat in shorter_to_remove {
		matches.remove(&pat);
	}

	if matches.len() < 2 {
		return;
	}

	let type_count = longest_index.is_some() as usize
		+ longest_dynamic.is_some() as usize
		+ longest_splat.is_some() as usize;
	if type_count <= 1 {
		return;
	}

	if let Some(pat) = longest_index {
		matches.remove(&pat);
	}
	let has_dyn = longest_dynamic.is_some();
	let has_spl = longest_splat.is_some();
	if real_segs_len == longest_len && has_dyn && has_spl {
		let to_remove: Vec<String> = matches
			.iter()
			.filter_map(|(pat, m)| {
				if m.pattern.normalized_segments.len() == longest_len
					&& m.pattern.last_seg_type == Some(SegmentKind::Splat)
				{
					Some(pat.clone())
				} else {
					None
				}
			})
			.collect();
		for pat in to_remove {
			matches.remove(&pat);
		}
	}
	if real_segs_len > longest_len && has_spl && has_dyn {
		let to_remove: Vec<String> = matches
			.iter()
			.filter_map(|(pat, m)| {
				if m.pattern.normalized_segments.len() == longest_len
					&& m.pattern.last_seg_type == Some(SegmentKind::Dynamic)
				{
					Some(pat.clone())
				} else {
					None
				}
			})
			.collect();
		for pat in to_remove {
			matches.remove(&pat);
		}
	}
}

fn flatten_and_sort(
	matches: HashMap<String, NestedMatch>,
	real_path: &str,
	real_seg_len: usize,
) -> Option<NestedMatches> {
	let count = matches.len();
	if count == 0 {
		return None;
	}

	let mut results: Vec<NestedMatch> = matches.into_values().collect();

	if count > 1 {
		results.sort_by(|a, b| {
			if a.pattern.last_seg_is_index != b.pattern.last_seg_is_index {
				if a.pattern.last_seg_is_index {
					return Ordering::Greater;
				}
				return Ordering::Less;
			}
			match a
				.pattern
				.normalized_segments
				.len()
				.cmp(&b.pattern.normalized_segments.len())
			{
				Ordering::Equal => a
					.pattern
					.normalized_pattern
					.cmp(&b.pattern.normalized_pattern),
				other => other,
			}
		});
	}

	let is_not_slash = !real_path.is_empty() && real_path != "/";
	if is_not_slash && results.len() == 1 && results[0].pattern.normalized_pattern.is_empty() {
		return None;
	}

	let last = results.last()?;

	if !last.pattern.last_seg_is_non_root_splat && last.pattern.normalized_pattern != "/*" {
		let pat_seg_len = last.pattern.normalized_segments.len();
		if pat_seg_len < real_seg_len {
			return None;
		}
		if pat_seg_len == real_seg_len
			&& last.pattern.num_dynamic_param_segs > 0
			&& last.params.is_empty()
		{
			return None;
		}
	}

	Some(NestedMatches {
		params: last.params.clone(),
		splat_values: last.splat_values.clone(),
		matches: results,
	})
}
