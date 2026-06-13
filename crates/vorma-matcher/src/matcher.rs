use std::cmp::Ordering;
use std::sync::Arc;

use rustc_hash::FxHashMap;
use smallvec::SmallVec;

use crate::match_result::{Match, NestedMatch, NestedMatches, Params, SplatValues};
use crate::parse::{has_doubled_slash, parse_segments_inline, strip_trailing_slash};
use crate::pattern::{Pattern, compare_specificity};
use crate::segment::SegmentKind;
use crate::tree::{NodeType, SegmentNode};

/// Route matcher for whole-path matching: one request path resolves to
/// the single best registered pattern.
///
/// Flat and nested matching are distinct semantics (they treat trailing
/// slashes and pattern coverage differently), so each lives on its own
/// matcher type; a builder finishes into exactly one of them.
#[derive(Clone, Debug)]
pub struct FlatMatcher {
	engine: MatcherEngine,
}

impl FlatMatcher {
	pub(crate) fn from_engine(engine: MatcherEngine) -> Self {
		Self { engine }
	}

	pub(crate) fn engine(&self) -> &MatcherEngine {
		&self.engine
	}

	/// Find the best single pattern match for a concrete path.
	pub fn find_best_match(&self, real_path: &str) -> Option<Match> {
		self.engine.find_best_match(real_path)
	}
}

/// Route matcher for nested chain matching: one request path resolves to
/// an ordered chain of registered patterns from outermost to innermost.
///
/// Flat and nested matching are distinct semantics (they treat trailing
/// slashes and pattern coverage differently), so each lives on its own
/// matcher type; a builder finishes into exactly one of them.
#[derive(Clone, Debug)]
pub struct NestedMatcher {
	engine: MatcherEngine,
}

impl NestedMatcher {
	pub(crate) fn from_engine(engine: MatcherEngine) -> Self {
		Self { engine }
	}

	pub(crate) fn engine(&self) -> &MatcherEngine {
		&self.engine
	}

	/// Find the ordered nested pattern chain for a concrete path.
	pub fn find_nested_matches(&self, real_path: &str) -> Option<NestedMatches> {
		self.engine.find_nested_matches(real_path)
	}
}

// Inline capacity for the depth-first walk stacks: route trees rarely
// branch more than a few alternatives deep, and the spill to the heap
// is correct, just slower.
type WalkStack<'m> = SmallVec<[(&'m SegmentNode, usize); 16]>;

// In-flight nested candidates: registered patterns by reference, in
// discovery order. Chains are short, so the list lives inline and
// membership runs linear — cheaper than hashing at this size.
type CandidateList<'m> = SmallVec<[&'m Arc<Pattern>; 8]>;

// The walks reach each tree position at most once per request, so a
// duplicate push is structurally impossible; the membership check keeps
// that invariant load-bearing rather than assumed.
fn push_candidate<'m>(matches: &mut CandidateList<'m>, rp: &'m Arc<Pattern>) {
	if !matches.iter().any(|existing| Arc::ptr_eq(existing, rp)) {
		matches.push(rp);
	}
}

/// Shared pattern store and matching algorithms behind the typed
/// matchers. The semantics split is enforced at the public type level;
/// the engine itself is mode-neutral storage plus both algorithms.
#[derive(Clone, Debug)]
pub(crate) struct MatcherEngine {
	static_patterns: FxHashMap<String, Arc<Pattern>>,
	dynamic_patterns: FxHashMap<String, Arc<Pattern>>,
	root_node: SegmentNode,
}

impl MatcherEngine {
	pub(crate) fn from_parts(
		static_patterns: FxHashMap<String, Arc<Pattern>>,
		dynamic_patterns: FxHashMap<String, Arc<Pattern>>,
		root_node: SegmentNode,
	) -> Self {
		Self {
			static_patterns,
			dynamic_patterns,
			root_node,
		}
	}

	pub(crate) fn registered_patterns(&self) -> Vec<&Pattern> {
		let mut patterns: Vec<&Pattern> = self
			.static_patterns
			.values()
			.chain(self.dynamic_patterns.values())
			.map(Arc::as_ref)
			.collect();
		patterns.sort_by(|left, right| left.normalized_pattern.cmp(&right.normalized_pattern));
		patterns
	}

	/// Find the best single pattern match for a concrete path.
	/*
	Dirty-path rule: at most one trailing slash is tolerated as noise,
	and an empty segment never matches anything — a doubled slash
	anywhere means no match, and params and splat values never contain
	empty strings.
	*/
	pub fn find_best_match(&self, real_path: &str) -> Option<Match> {
		if let Some(rr) = self.static_patterns.get(real_path) {
			return Some(Match::from_registered(Arc::clone(rr)));
		}
		/*
		The dirty-path check runs after the raw lookup: registered keys
		never contain doubled slashes, so a raw hit is always clean. It
		must run before the stripped lookup — stripping one slash from a
		doubled trailing slash would otherwise fabricate a clean path.
		*/
		if has_doubled_slash(real_path) {
			return None;
		}

		let effective = strip_trailing_slash(real_path);
		if effective.len() != real_path.len()
			&& let Some(rr) = self.static_patterns.get(effective)
		{
			return Some(Match::from_registered(Arc::clone(rr)));
		}

		// Root family: the root catch-all takes the root path itself,
		// with no splat values (there is no segment to capture).
		if effective.is_empty() {
			if let Some(rr) = self.dynamic_patterns.get("/*") {
				return Some(Match::from_registered(Arc::clone(rr)));
			}
			return None;
		}

		if self.dynamic_patterns.is_empty() {
			return None;
		}

		let segments = parse_segments_inline(effective);
		let had_trailing = effective.len() != real_path.len();
		let best = self.find_best_dynamic_match(&segments, had_trailing)?;
		let (params, splat_values) = build_captures(best, &segments);
		Some(Match {
			pattern: Arc::clone(best),
			params,
			splat_values,
		})
	}

	/// Find the ordered nested pattern chain for a concrete path.
	/*
	Dirty-path rule: at most one trailing slash is tolerated as noise,
	and an empty segment never matches anything — a doubled slash
	anywhere means no match.
	*/
	pub fn find_nested_matches(&self, real_path: &str) -> Option<NestedMatches> {
		if has_doubled_slash(real_path) {
			return None;
		}
		let real_path = strip_trailing_slash(real_path);
		let real_segs = parse_segments_inline(real_path);
		let real_segs_len = real_segs.len();
		let mut matches: CandidateList<'_> = SmallVec::new();

		if let Some(empty_rr) = self.static_patterns.get("") {
			matches.push(empty_rr);
		}

		if real_path.is_empty() {
			if let Some(rr) = self.static_patterns.get("/") {
				push_candidate(&mut matches, rr);
			} else if let Some(rr) = self.dynamic_patterns.get("/*") {
				push_candidate(&mut matches, rr);
			}
			return flatten_and_sort(matches, real_path, &real_segs);
		}

		// Static candidates ride the tree: one descent along the path's
		// own segments visits every static prefix, hashing each short
		// segment once instead of re-hashing growing prefix strings.
		let mut found_full_static = false;
		let mut node = &self.root_node;
		for (i, seg) in real_segs.iter().enumerate() {
			let Some(child) = node.children.get(*seg) else {
				break;
			};
			if let Some(rp) = &child.registered
				&& rp.is_static
			{
				push_candidate(&mut matches, rp);
				if i == real_segs_len - 1 {
					found_full_static = true;
				}
			}
			if i == real_segs_len - 1
				&& let Some(index_child) = child.children.get("")
				&& let Some(irp) = &index_child.registered
				&& irp.is_static
			{
				push_candidate(&mut matches, irp);
			}
			node = child;
		}

		if !found_full_static {
			if let Some(rr) = self.dynamic_patterns.get("/*") {
				push_candidate(&mut matches, rr);
			}
			self.collect_nested_dynamic_matches(&real_segs, &mut matches);
		}

		/*
		A prefix hit is not a match: a path matches a pattern only when a
		chain completes through it. The catch-all therefore yields only
		to an entry that actually covers the path; when nothing covers,
		the dead prefixes drop out and the catch-all chain is the layout
		plus the catch-all itself.
		*/
		if matches.iter().any(|pattern| pattern.is_root_catch_all) {
			let any_other_covers = matches.iter().any(|pattern| {
				!pattern.is_root_catch_all
					&& (pattern.last_seg_is_non_root_splat
						|| pattern.normalized_segments.len() >= real_segs_len)
			});
			if any_other_covers {
				matches.retain(|pattern| !pattern.is_root_catch_all);
			} else {
				matches.retain(|pattern| {
					pattern.normalized_pattern.is_empty() || pattern.is_root_catch_all
				});
			}
		}

		if matches.len() < 2 {
			return flatten_and_sort(matches, real_path, &real_segs);
		}

		prune_nested_matches(&mut matches, real_segs_len);
		flatten_and_sort(matches, real_path, &real_segs)
	}

	fn find_best_dynamic_match(
		&self,
		segments: &[&str],
		had_trailing: bool,
	) -> Option<&Arc<Pattern>> {
		let mut best: Option<&Arc<Pattern>> = None;
		let mut stack: WalkStack<'_> = SmallVec::new();
		stack.push((&self.root_node, 0usize));
		while let Some((node, depth)) = stack.pop() {
			if let Some(rp) = &node.registered
				&& !rp.is_static
				&& (depth == segments.len() || node.node_type == NodeType::Splat)
				&& best.is_none_or(|current| compare_specificity(rp, current) == Ordering::Greater)
			{
				best = Some(rp);
			}

			/*
			An index pattern's canonical spelling carries the trailing
			slash, so its terminal empty-segment child is accepted
			exactly when the path supplied that one slash.
			*/
			if had_trailing
				&& depth == segments.len()
				&& let Some(index_child) = node.children.get("")
				&& let Some(rp) = &index_child.registered
				&& !rp.is_static
				&& best.is_none_or(|current| compare_specificity(rp, current) == Ordering::Greater)
			{
				best = Some(rp);
			}

			if depth >= segments.len() {
				continue;
			}

			if let Some(child) = node.children.get(segments[depth]) {
				stack.push((child, depth + 1));
			}

			for child in &node.dyn_children {
				match child.node_type {
					NodeType::Dynamic => {
						if !segments[depth].is_empty() {
							stack.push((child, depth + 1));
						}
					}
					NodeType::Splat => {
						if let Some(rp) = &child.registered
							&& best.is_none_or(|current| {
								compare_specificity(rp, current) == Ordering::Greater
							}) {
							best = Some(rp);
						}
					}
					NodeType::Static => {}
				}
			}
		}
		best
	}

	// The nested walk tracks reachable pattern nodes only; captures are
	// rebuilt positionally at emit time, so losing branches never
	// allocate.
	fn collect_nested_dynamic_matches<'m>(
		&'m self,
		segments: &[&str],
		matches: &mut CandidateList<'m>,
	) {
		let mut stack: WalkStack<'m> = SmallVec::new();
		stack.push((&self.root_node, 0usize));
		while let Some((node, depth)) = stack.pop() {
			if let Some(rp) = &node.registered
				&& !rp.is_static
				&& !rp.is_root_catch_all
			{
				push_candidate(matches, rp);
			}

			/*
			An index pattern's canonical spelling carries the trailing
			slash; in nested matching it claims its parent path, so its
			terminal empty-segment child is accepted whenever the walk
			consumed the whole path — whether or not the parent path is
			itself a registered pattern.
			*/
			if depth == segments.len()
				&& let Some(index_child) = node.children.get("")
				&& let Some(irp) = &index_child.registered
				&& !irp.is_static
			{
				push_candidate(matches, irp);
			}

			if depth >= segments.len() {
				continue;
			}

			let seg = segments[depth];
			if let Some(child) = node.children.get(seg) {
				stack.push((child, depth + 1));
			}

			for child in &node.dyn_children {
				match child.node_type {
					NodeType::Dynamic => {
						stack.push((child, depth + 1));
					}
					NodeType::Splat => {
						stack.push((child, depth));
					}
					NodeType::Static => {}
				}
			}
		}
	}
}

// Build the captured params and splat values for one matched pattern
// against the real path segments. Matching is positional, so captures
// reconstruct exactly: a dynamic segment at position i captured
// segments[i], and a splat tail captured everything from its own
// position on. Called once per emitted match — the walks themselves
// never allocate captures.
fn build_captures(pattern: &Pattern, segments: &[&str]) -> (Params, SplatValues) {
	let mut params = Params::default();
	if !pattern.param_positions.is_empty() {
		params.reserve(pattern.param_positions.len());
		for (i, name) in &pattern.param_positions {
			if let Some(value) = segments.get(*i) {
				params.insert_shared(name, (*value).to_owned());
			}
		}
	}

	let splat_values = match pattern.splat_start {
		Some(start) => SplatValues::from_segments(segments.get(start..).unwrap_or_default()),
		None => SplatValues::default(),
	};

	(params, splat_values)
}

fn prune_nested_matches(matches: &mut CandidateList<'_>, real_segs_len: usize) {
	let mut longest_len = 0usize;
	for m in matches.iter() {
		longest_len = longest_len.max(m.normalized_segments.len());
	}
	let mut has_index = false;
	let mut has_dynamic = false;
	let mut has_splat = false;
	for m in matches.iter() {
		if m.normalized_segments.len() == longest_len {
			match m.last_seg_type {
				Some(SegmentKind::Index) => has_index = true,
				Some(SegmentKind::Dynamic) => has_dynamic = true,
				Some(SegmentKind::Splat) => has_splat = true,
				_ => {}
			}
		}
	}

	matches.retain(|m| {
		!(m.normalized_segments.len() < longest_len
			&& (m.last_seg_is_non_root_splat || m.last_seg_is_index))
	});

	if matches.len() < 2 {
		return;
	}

	let type_count = has_index as usize + has_dynamic as usize + has_splat as usize;
	if type_count <= 1 {
		return;
	}

	if has_index {
		matches.retain(|m| {
			!(m.normalized_segments.len() == longest_len
				&& m.last_seg_type == Some(SegmentKind::Index))
		});
	}
	if real_segs_len == longest_len && has_dynamic && has_splat {
		matches.retain(|m| {
			!(m.normalized_segments.len() == longest_len
				&& m.last_seg_type == Some(SegmentKind::Splat))
		});
	}
	if real_segs_len > longest_len && has_splat && has_dynamic {
		matches.retain(|m| {
			!(m.normalized_segments.len() == longest_len
				&& m.last_seg_type == Some(SegmentKind::Dynamic))
		});
	}
}

fn flatten_and_sort(
	mut results: CandidateList<'_>,
	real_path: &str,
	real_segs: &[&str],
) -> Option<NestedMatches> {
	if results.is_empty() {
		return None;
	}

	if results.len() > 1 {
		results.sort_by(|a, b| {
			if a.last_seg_is_index != b.last_seg_is_index {
				if a.last_seg_is_index {
					return Ordering::Greater;
				}
				return Ordering::Less;
			}
			match a
				.normalized_segments
				.len()
				.cmp(&b.normalized_segments.len())
			{
				Ordering::Equal => a.normalized_pattern.cmp(&b.normalized_pattern),
				other => other,
			}
		});
	}

	let is_not_slash = !real_path.is_empty() && real_path != "/";
	if is_not_slash && results.len() == 1 && results[0].normalized_pattern.is_empty() {
		return None;
	}

	let last = results.last()?;

	if !last.last_seg_is_non_root_splat
		&& !last.is_root_catch_all
		&& last.normalized_segments.len() < real_segs.len()
	{
		return None;
	}

	let matches: Vec<NestedMatch> = results
		.into_iter()
		.map(|pattern| {
			let (params, splat_values) = build_captures(pattern, real_segs);
			NestedMatch {
				pattern: Arc::clone(pattern),
				params,
				splat_values,
			}
		})
		.collect();
	let last = matches
		.last()
		.expect("a non-empty candidate set produces a non-empty chain");

	Some(NestedMatches {
		params: last.params.clone(),
		splat_values: last.splat_values.clone(),
		matches,
	})
}
