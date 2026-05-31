use std::path::{self, PathBuf};

use globset::{Glob, GlobMatcher};
use path_clean::PathClean;
use path_slash::{PathBufExt, PathExt};

#[derive(Clone, Debug)]
pub(crate) struct Set {
	rules: Vec<Rule>,
}

#[derive(Clone, Debug)]
pub(crate) struct Rule {
	pub(crate) pattern: String,
	pub(crate) child_pattern: String,
	pub(crate) excluded: bool,
	pub(crate) dir_only: bool,
	pub(crate) anchored: bool,
	matcher: GlobMatcher,
	child_matcher: Option<GlobMatcher>,
}

pub(crate) fn parse(raw_rule: &str) -> Result<Option<Rule>, String> {
	let raw_rule = raw_rule.trim();
	if raw_rule.is_empty() {
		return Err("glob rule cannot be empty".to_owned());
	}

	let excluded = raw_rule.starts_with('!');
	let mut pattern = to_slash_path(raw_rule.trim_start_matches('!').trim());
	if pattern.is_empty() {
		return Err("glob rule pattern cannot be empty".to_owned());
	}

	let dir_only = pattern.ends_with('/');
	if dir_only {
		pattern.truncate(pattern.len() - 1);
	}

	if let Some(after) = pattern.strip_prefix("./") {
		pattern = format!("/{after}");
	}

	let anchored = pattern.starts_with('/');
	if pattern != "." && !pattern.starts_with("**") {
		if anchored {
			pattern = pattern.trim_start_matches('/').to_owned();
		} else if !pattern.contains('/') {
			pattern = format!("**/{pattern}");
		}
	}

	if pattern.is_empty() {
		return Err("glob rule pattern cannot be empty".to_owned());
	}

	let matcher = compile_matcher(&pattern)?;
	let child_pattern = if !dir_only && pattern != "." && !pattern.ends_with("**") {
		format!("{pattern}/**")
	} else {
		String::new()
	};
	let child_matcher = if child_pattern.is_empty() {
		None
	} else {
		Some(compile_matcher(&child_pattern)?)
	};
	Ok(Some(Rule {
		pattern,
		child_pattern,
		excluded,
		dir_only,
		anchored,
		matcher,
		child_matcher,
	}))
}

pub(crate) fn compile(raw_rules: &[String]) -> Result<Set, String> {
	let mut rules = Vec::with_capacity(raw_rules.len());
	for (i, raw_rule) in raw_rules.iter().enumerate() {
		if let Some(rule) =
			parse(raw_rule).map_err(|err| format!("invalid rule {i} ({raw_rule:?}): {err}"))?
		{
			rules.push(rule);
		}
	}
	Ok(Set { rules })
}

pub(crate) fn path_match_unvalidated(pattern: &str, path: &str) -> bool {
	compile_matcher(pattern)
		.map(|matcher| matcher.is_match(path))
		.unwrap_or_else(|err| panic!("invalid internal glob pattern {pattern:?}: {err}"))
}

impl Rule {
	#[cfg(test)]
	pub(crate) fn has_segment(&self, name: &str) -> bool {
		self.pattern
			.split('/')
			.filter(|segment| !segment.is_empty() && *segment != "**")
			.any(|segment| path_match_unvalidated(segment, name))
	}
}

impl Set {
	pub(crate) fn rules(&self) -> &[Rule] {
		&self.rules
	}

	pub(crate) fn is_match(&self, path: &str) -> bool {
		let (path, is_dir) = normalize_path(path);
		let mut matched = false;

		for rule in &self.rules {
			if rule.pattern == "." {
				matched = !rule.excluded;
				continue;
			}

			if matches_rule(rule, &path, is_dir) {
				matched = !rule.excluded;
			}
		}

		matched
	}
}

fn compile_matcher(pattern: &str) -> Result<GlobMatcher, String> {
	Glob::new(pattern)
		.map(|glob| glob.compile_matcher())
		.map_err(|err| err.to_string())
}

fn normalize_path(path: &str) -> (String, bool) {
	let mut path = to_slash_path(path.trim());
	let is_dir = path.ends_with('/');
	path = path.trim_end_matches('/').to_owned();
	path = clean_slash_path(&path);
	if let Some(after) = path.strip_prefix("./") {
		path = after.to_owned();
	}
	if path == "." || path.is_empty() {
		return (".".to_owned(), is_dir);
	}
	(path, is_dir)
}

fn clean_slash_path(path: &str) -> String {
	PathBuf::from_slash(path)
		.clean()
		.to_slash_lossy()
		.into_owned()
}

fn matches_rule(rule: &Rule, path: &str, is_dir: bool) -> bool {
	if rule.dir_only {
		if is_dir && matches_exact(rule, path) {
			return true;
		}
		return matches_parent(rule, path);
	}

	if matches_exact(rule, path) {
		return true;
	}

	matches_child(rule, path)
}

fn matches_exact(rule: &Rule, path: &str) -> bool {
	if rule.anchored && !rule.pattern.contains('/') && path.contains('/') {
		return false;
	}
	rule.matcher.is_match(path)
}

fn matches_child(rule: &Rule, path: &str) -> bool {
	if rule.child_pattern.is_empty() {
		return false;
	}

	if rule.anchored && !rule.pattern.contains('/') {
		let Some((first, _)) = path.split_once('/') else {
			return false;
		};
		return rule.matcher.is_match(first);
	}

	rule.child_matcher
		.as_ref()
		.is_some_and(|matcher| matcher.is_match(path))
}

fn matches_parent(rule: &Rule, path: &str) -> bool {
	let mut dir = path.to_owned();
	loop {
		dir = parent_dir(&dir);
		if dir == "." {
			return false;
		}
		if matches_exact(rule, &dir) {
			return true;
		}
	}
}

fn parent_dir(path: &str) -> String {
	let path = PathBuf::from_slash(path);
	path.parent()
		.map(|parent| {
			let value = parent.to_slash_lossy();
			if value.is_empty() {
				".".to_owned()
			} else {
				value.into_owned()
			}
		})
		.unwrap_or_else(|| ".".to_owned())
}

fn to_slash_path(path: &str) -> String {
	path::Path::new(path).to_slash_lossy().into_owned()
}

#[cfg(test)]
mod tests {
	use super::*;

	fn must_compile(patterns: &[&str]) -> Set {
		compile(
			&patterns
				.iter()
				.map(|pattern| pattern.to_string())
				.collect::<Vec<_>>(),
		)
		.unwrap()
	}

	#[test]
	fn match_includes_simple_subtree() {
		let set = must_compile(&["src/**"]);

		assert!(set.is_match("src/main.rs"));
		assert!(!set.is_match("lib/main.rs"));
	}

	#[test]
	fn match_pattern_without_slash_matches_anywhere() {
		let set = must_compile(&["*.rs"]);

		assert!(set.is_match("main.rs"));
		assert!(set.is_match("src/main.rs"));
		assert!(!set.is_match("src/main.ts"));
	}

	#[test]
	fn match_leading_slash_anchors_to_root() {
		let set = must_compile(&["/*.rs"]);

		assert!(set.is_match("main.rs"));
		assert!(!set.is_match("src/main.rs"));
	}

	#[test]
	fn match_trailing_slash_only_matches_directories() {
		let set = must_compile(&[".", "!build/"]);

		assert!(!set.is_match("build/"));
		assert!(!set.is_match("build/out.rs"));
		assert!(set.is_match("build"));
	}

	#[test]
	fn match_last_rule_wins() {
		let set = must_compile(&["src/**", "!src/vendor/**", "src/vendor/keep.rs"]);

		assert!(set.is_match("src/main.rs"));
		assert!(!set.is_match("src/vendor/drop.rs"));
		assert!(set.is_match("src/vendor/keep.rs"));
	}

	#[test]
	fn match_dot_slash_anchors_to_root() {
		let set = must_compile(&["./build"]);

		assert!(set.is_match("build"));
		assert!(!set.is_match("src/build"));
	}

	#[test]
	fn rule_has_segment_matches_literal_and_glob_segments() {
		let rule = parse("**/node_modules/**").unwrap().unwrap();
		assert!(rule.has_segment("node_modules"));

		let rule = parse("*modules/**").unwrap().unwrap();
		assert!(rule.has_segment("node_modules"));
	}

	#[test]
	fn match_normalizes_candidate_paths() {
		let set = must_compile(&["src/**", "!src/vendor/**"]);

		assert!(set.is_match("./src//main.rs"));
		assert!(!set.is_match("./src/vendor/./dep.rs"));
	}
}
