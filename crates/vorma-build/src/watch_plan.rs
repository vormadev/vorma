use std::collections::BTreeSet;
use std::path::Path;

use path_clean::PathClean;
use path_slash::PathBufExt;

use crate::config::{relative_path, sys_norm};
use crate::globset;

#[derive(Clone, Debug)]
pub(crate) struct WatchPlan {
	match_set: globset::Set,
	rules: Vec<globset::Rule>,
	pub(crate) roots: Vec<WatchRoot>,
}

#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub(crate) struct WatchRoot {
	pub(crate) path: String,
	pub(crate) dynamic: bool,
}

impl WatchPlan {
	pub(crate) fn new(raw_patterns: &[String]) -> Result<Self, String> {
		let mut patterns = raw_patterns
			.iter()
			.map(|pattern| pattern.trim().to_owned())
			.collect::<Vec<_>>();
		if patterns.is_empty() {
			patterns.push(".".to_owned());
		}

		let match_set = globset::compile(&patterns)?;
		let rules = match_set.rules().to_vec();
		let mut roots = BTreeSet::new();
		for rule in &rules {
			if !rule.excluded {
				roots.insert(watch_root_for_rule(rule));
			}
		}

		Ok(Self {
			match_set,
			rules,
			roots: roots.into_iter().collect(),
		})
	}

	pub(crate) fn should_emit(&self, path: &str, is_dir: bool) -> bool {
		let mut rel_path = watch_rel_path(path);
		if is_dir {
			rel_path.push('/');
		}
		self.match_set.is_match(&rel_path)
	}

	pub(crate) fn should_watch_dir(&self, path: &str) -> bool {
		self.could_match_in_subtree(&watch_rel_path(path))
	}

	fn could_match_in_subtree(&self, dir: &str) -> bool {
		let dir = normalize_watch_path(dir);
		let mut last_cover_index = None;
		let mut last_cover_excluded = false;
		for (i, rule) in self.rules.iter().enumerate() {
			if rule_covers_subtree(rule, &dir) {
				last_cover_index = Some(i);
				last_cover_excluded = rule.excluded;
			}
		}

		if let Some(last_cover_index) = last_cover_index
			&& last_cover_excluded
		{
			return self.rules[last_cover_index + 1..]
				.iter()
				.any(|rule| !rule.excluded && rule_can_match_in_subtree(rule, &dir));
		}

		self.rules
			.iter()
			.any(|rule| !rule.excluded && rule_can_match_in_subtree(rule, &dir))
	}
}

fn watch_root_for_rule(rule: &globset::Rule) -> WatchRoot {
	if rule.pattern == "." {
		return WatchRoot {
			path: ".".to_owned(),
			dynamic: false,
		};
	}

	let mut literal_segments = Vec::new();
	let pattern_segments = split_watch_path(&rule.pattern);
	for segment in &pattern_segments {
		if segment_has_glob(segment) {
			break;
		}
		literal_segments.push(segment.clone());
	}

	if literal_segments.is_empty() {
		return WatchRoot {
			path: ".".to_owned(),
			dynamic: false,
		};
	}

	let root_path = literal_segments.join("/");
	WatchRoot {
		dynamic: literal_segments.len() == pattern_segments.len(),
		path: root_path,
	}
}

fn watch_rel_path(path: &str) -> String {
	let path = sys_norm(path);
	relative_path(".", &path)
		.map(|path| normalize_watch_path(path.to_string_lossy()))
		.unwrap_or_else(|| normalize_watch_path(path))
}

fn normalize_watch_path(raw_path: impl AsRef<str>) -> String {
	let raw_path = raw_path.as_ref().trim();
	let mut path = std::path::PathBuf::from_slash(raw_path)
		.clean()
		.to_slash_lossy()
		.trim_end_matches('/')
		.trim_start_matches("./")
		.to_owned();
	if path.is_empty() {
		path = ".".to_owned();
	}
	path
}

pub(crate) fn split_watch_path(path: &str) -> Vec<String> {
	let path = normalize_watch_path(path);
	if path == "." {
		return Vec::new();
	}
	path.split('/').map(str::to_owned).collect()
}

fn segment_has_glob(segment: &str) -> bool {
	segment.contains(['*', '?', '[', '{'])
}

fn rule_can_match_in_subtree(rule: &globset::Rule, dir: &str) -> bool {
	if rule.pattern == "." {
		return true;
	}

	let dir_segments = split_watch_path(dir);
	if pattern_can_match_prefix(&rule.pattern, &dir_segments) {
		return true;
	}
	if !rule.child_pattern.is_empty()
		&& pattern_can_match_prefix(&rule.child_pattern, &dir_segments)
	{
		return true;
	}
	if rule.dir_only {
		let child_pattern = format!("{}/**", rule.pattern);
		return pattern_can_match_prefix(&child_pattern, &dir_segments);
	}
	false
}

fn rule_covers_subtree(rule: &globset::Rule, dir: &str) -> bool {
	if rule.pattern == "." {
		return true;
	}

	for ancestor in watch_path_ancestors(dir) {
		if pattern_matches_path(&rule.pattern, &ancestor)
			&& (!rule.child_pattern.is_empty() || rule.dir_only)
		{
			return true;
		}
		if pattern_suffix_covers_subtree(&rule.pattern, &ancestor) {
			return true;
		}
	}
	false
}

fn watch_path_ancestors(path: &str) -> Vec<String> {
	let mut path = normalize_watch_path(path);
	let mut ancestors = vec![path.clone()];
	while path != "." {
		path = normalize_watch_path(
			Path::new(&path)
				.parent()
				.map(|path| path.to_string_lossy().into_owned())
				.unwrap_or_else(|| ".".to_owned()),
		);
		ancestors.push(path.clone());
	}
	ancestors
}

fn pattern_suffix_covers_subtree(pattern: &str, dir: &str) -> bool {
	let Some(base) = pattern.strip_suffix("/**") else {
		return false;
	};
	base.is_empty() || pattern_matches_path(base, dir)
}

fn pattern_matches_path(pattern: &str, path: &str) -> bool {
	let pattern = normalize_watch_path(pattern);
	let path = normalize_watch_path(path);
	pattern == "." || globset::path_match_unvalidated(&pattern, &path)
}

fn pattern_can_match_prefix(pattern: &str, prefix_segments: &[String]) -> bool {
	let pattern_segments = split_watch_path(pattern);
	let mut seen = BTreeSet::new();

	fn walk(
		pattern_segments: &[String],
		prefix_segments: &[String],
		pattern_index: usize,
		prefix_index: usize,
		seen: &mut BTreeSet<(usize, usize)>,
	) -> bool {
		if prefix_index == prefix_segments.len() {
			return true;
		}
		if pattern_index == pattern_segments.len() {
			return false;
		}
		if !seen.insert((pattern_index, prefix_index)) {
			return false;
		}

		let pattern_segment = &pattern_segments[pattern_index];
		if pattern_segment == "**" {
			if walk(
				pattern_segments,
				prefix_segments,
				pattern_index + 1,
				prefix_index,
				seen,
			) {
				return true;
			}
			return walk(
				pattern_segments,
				prefix_segments,
				pattern_index,
				prefix_index + 1,
				seen,
			);
		}

		if !globset::path_match_unvalidated(pattern_segment, &prefix_segments[prefix_index]) {
			return false;
		}
		walk(
			pattern_segments,
			prefix_segments,
			pattern_index + 1,
			prefix_index + 1,
			seen,
		)
	}

	walk(&pattern_segments, prefix_segments, 0, 0, &mut seen)
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn watch_plan_uses_dot_root_for_unanchored_globs_and_literal_roots_for_subtrees() {
		let plan = WatchPlan::new(&[
			"src/**/*.rs".to_owned(),
			"public".to_owned(),
			"!target/**".to_owned(),
		])
		.unwrap();

		assert_eq!(
			plan.roots,
			vec![
				WatchRoot {
					path: ".".to_owned(),
					dynamic: false,
				},
				WatchRoot {
					path: "src".to_owned(),
					dynamic: false,
				},
			],
		);
	}

	#[test]
	fn watch_plan_understands_excluded_subtrees_with_later_includes() {
		let plan = WatchPlan::new(&[
			".".to_owned(),
			"!src/generated/**".to_owned(),
			"src/generated/keep.rs".to_owned(),
		])
		.unwrap();

		assert!(plan.should_watch_dir("src"));
		assert!(plan.should_watch_dir("src/generated"));
		assert!(plan.should_emit("src/generated/keep.rs", false));
		assert!(!plan.should_emit("src/generated/drop.rs", false));
	}
}
