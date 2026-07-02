//! Development filesystem watcher projected from the build watch plan.

use std::collections::BTreeSet;
use std::path::{Component, Path, PathBuf};
use std::sync::mpsc::{self, Receiver};
use std::sync::{Arc, RwLock};
use std::time::Duration;

use globset::{Glob, GlobMatcher};
use notify::Watcher;
use path_slash::PathExt;

use crate::build_plan::{BuildDevWatchIntent, BuildDevWatchPlan};

/// Compiled dev filesystem watch plan.
#[derive(Clone, Debug)]
pub struct DevFileWatchPlan {
	root_dir: PathBuf,
	entries: Vec<CompiledDevWatchEntry>,
	exclusions: Vec<CompiledDevWatchExclusion>,
	watch_roots: Vec<PathBuf>,
}

impl DevFileWatchPlan {
	/// Compile a dev file watch plan from graph-derived watch entries.
	pub fn compile(
		root_dir: impl Into<PathBuf>,
		watch_plan: &BuildDevWatchPlan,
	) -> Result<Self, DevWatcherError> {
		let root_dir = root_dir.into();
		if root_dir.as_os_str().is_empty() {
			return Err(DevWatcherError::EmptyRootDir);
		}
		let mut entries = Vec::new();
		let mut exclusions = Vec::new();
		let mut watch_roots = BTreeSet::new();
		for entry in watch_plan.entries() {
			let Some(pattern) = parse_watch_pattern(entry.source_path())? else {
				continue;
			};
			let normalized = normalize_watch_source(&root_dir, pattern.source_path)?;
			if pattern.is_exclusion {
				exclusions.push(CompiledDevWatchExclusion::new(normalized)?);
			} else {
				let root = watch_root_for_source(&root_dir, &normalized);
				watch_roots.insert(root);
				entries.push(CompiledDevWatchEntry::new(normalized, entry.intent())?);
			}
		}
		Ok(Self {
			root_dir,
			entries,
			exclusions,
			watch_roots: watch_roots.into_iter().collect(),
		})
	}

	/// Directories watched by the filesystem watcher.
	pub fn watch_roots(&self) -> &[PathBuf] {
		&self.watch_roots
	}

	/// Classify one changed path into framework-level watch intents.
	pub fn classify_path(&self, path: impl AsRef<Path>) -> DevFileChange {
		let Some(relative_path) = relative_event_path(&self.root_dir, path.as_ref()) else {
			return DevFileChange::default();
		};
		if self
			.exclusions
			.iter()
			.any(|exclusion| exclusion.matches(&relative_path))
		{
			return DevFileChange::default();
		}
		let mut intents = BTreeSet::new();
		for entry in &self.entries {
			if entry.matches(&relative_path) {
				intents.insert(entry.intent);
			}
		}
		if intents.is_empty() {
			DevFileChange::default()
		} else {
			DevFileChange {
				paths: vec![self.root_dir.join(relative_path)],
				intents,
			}
		}
	}

	fn classify_event(&self, event: &notify::Event) -> DevFileChange {
		if !event_kind_mutates_source(&event.kind) {
			return DevFileChange::default();
		}
		let mut change = DevFileChange::default();
		for path in &event.paths {
			change.extend(self.classify_path(path));
		}
		change
	}
}

/*
A build watcher reacts to source *mutations*, never to bare reads. inotify (Linux)
emits IN_OPEN and IN_CLOSE_NOWRITE — surfaced as `Access(Open(_))` and
`Access(Close(Read/Any/Execute))` — every time any process merely opens a watched
file for reading. During a rebuild the spawned cargo/rustc reads the very
`src/**/*.rs` sources it is compiling, so those read events would classify as
`ServerRecompile` and cancel-and-restart the in-flight rebuild forever (the
compiler never finishes because its own file reads keep retiring it). Dropping
non-mutating access events breaks that feedback loop at the source. FSEvents
(macOS) never emits open/read events at all, so this gate is a no-op there and
cannot change the currently-working macOS behavior. Every genuine write is still
seen: `IN_MODIFY` -> `Modify(Data(_))`, `IN_ATTRIB` -> `Modify(Metadata(_))`,
plus `Create`/`Remove`/rename and the `IN_CLOSE_WRITE` -> `Access(Close(Write))`
write-completion signal, all of which are retained below.
*/
fn event_kind_mutates_source(kind: &notify::EventKind) -> bool {
	use notify::EventKind;
	use notify::event::{AccessKind, AccessMode};
	match kind {
		EventKind::Access(access) => matches!(access, AccessKind::Close(AccessMode::Write)),
		EventKind::Create(_) | EventKind::Modify(_) | EventKind::Remove(_) => true,
		/*
		`Any`/`Other` are the imprecise catch-alls a backend emits when it cannot
		classify a native event; treat them as potential mutations rather than
		risk dropping a real change.
		*/
		EventKind::Any | EventKind::Other => true,
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct ParsedWatchPattern<'a> {
	source_path: &'a str,
	is_exclusion: bool,
}

#[derive(Clone, Debug)]
struct CompiledDevWatchEntry {
	source_path: PathBuf,
	source_path_slash: String,
	intent: BuildDevWatchIntent,
	matcher: Option<GlobMatcher>,
}

impl CompiledDevWatchEntry {
	fn new(source_path: PathBuf, intent: BuildDevWatchIntent) -> Result<Self, DevWatcherError> {
		let source_path_slash = path_to_slash(&source_path)?;
		let matcher = if contains_glob_meta(&source_path_slash) {
			Some(
				Glob::new(&source_path_slash)
					.map_err(|source| DevWatcherError::InvalidGlob {
						pattern: source_path_slash.clone(),
						message: source.to_string(),
					})?
					.compile_matcher(),
			)
		} else {
			None
		};
		Ok(Self {
			source_path,
			source_path_slash,
			intent,
			matcher,
		})
	}

	fn matches(&self, relative_path: &Path) -> bool {
		let Ok(relative_path_slash) = path_to_slash(relative_path) else {
			return false;
		};
		match &self.matcher {
			Some(matcher) => matcher.is_match(&relative_path_slash),
			None => {
				relative_path == self.source_path
					|| relative_path.starts_with(&self.source_path)
					|| relative_path_slash == self.source_path_slash
			}
		}
	}
}

#[derive(Clone, Debug)]
struct CompiledDevWatchExclusion {
	source_path: PathBuf,
	source_path_slash: String,
	matcher: Option<GlobMatcher>,
}

impl CompiledDevWatchExclusion {
	fn new(source_path: PathBuf) -> Result<Self, DevWatcherError> {
		let source_path_slash = path_to_slash(&source_path)?;
		let matcher = if contains_glob_meta(&source_path_slash) {
			Some(
				Glob::new(&source_path_slash)
					.map_err(|source| DevWatcherError::InvalidGlob {
						pattern: source_path_slash.clone(),
						message: source.to_string(),
					})?
					.compile_matcher(),
			)
		} else {
			None
		};
		Ok(Self {
			source_path,
			source_path_slash,
			matcher,
		})
	}

	fn matches(&self, relative_path: &Path) -> bool {
		let Ok(relative_path_slash) = path_to_slash(relative_path) else {
			return false;
		};
		match &self.matcher {
			Some(matcher) => matcher.is_match(&relative_path_slash),
			None => {
				relative_path == self.source_path
					|| relative_path.starts_with(&self.source_path)
					|| relative_path_slash == self.source_path_slash
			}
		}
	}
}

/// One framework-relevant filesystem change.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct DevFileChange {
	paths: Vec<PathBuf>,
	intents: BTreeSet<BuildDevWatchIntent>,
}

impl DevFileChange {
	/// Framework-level intents associated with this change.
	pub fn intents(&self) -> &BTreeSet<BuildDevWatchIntent> {
		&self.intents
	}

	/// Whether the change matched no watch-plan entries.
	pub fn is_empty(&self) -> bool {
		self.intents.is_empty()
	}

	/// Whether this change requires a Vorma dev generation update.
	pub fn requires_vorma_generation(&self) -> bool {
		self.requires_app_server_generation()
			|| self.requires_static_output_generation()
			|| self.requires_client_revalidation()
	}

	/// Whether this change requires app-server-backed generation activation.
	pub fn requires_app_server_generation(&self) -> bool {
		self.intents
			.iter()
			.any(|intent| matches!(intent, BuildDevWatchIntent::ServerRecompile))
	}

	/// Whether this change requires active browser route data revalidation.
	pub fn requires_client_revalidation(&self) -> bool {
		self.intents
			.contains(&BuildDevWatchIntent::ClientRevalidate)
	}

	/// Whether this change requires public static output or critical CSS work.
	pub fn requires_static_output_generation(&self) -> bool {
		self.intents.iter().any(|intent| {
			matches!(
				intent,
				BuildDevWatchIntent::PublicStaticInput | BuildDevWatchIntent::CriticalCssInput
			)
		})
	}

	/// Whether this change only needs browser route data revalidation.
	#[cfg(test)]
	pub fn requires_only_client_revalidation(&self) -> bool {
		self.requires_client_revalidation()
			&& !self.requires_app_server_generation()
			&& !self.requires_static_output_generation()
	}

	pub(crate) fn extend(&mut self, next: Self) {
		self.paths.extend(next.paths);
		self.intents.extend(next.intents);
	}

	#[cfg(test)]
	pub(crate) fn from_intents_for_test(
		intents: impl IntoIterator<Item = BuildDevWatchIntent>,
	) -> Self {
		Self {
			paths: Vec::new(),
			intents: intents.into_iter().collect(),
		}
	}
}

/// Started dev file watcher.
pub struct StartedDevFileWatcher {
	_watcher: notify::RecommendedWatcher,
	rx: Receiver<Result<DevFileChange, DevWatcherError>>,
	plan: Arc<RwLock<Arc<DevFileWatchPlan>>>,
}

/*
Notify roots are bound when the watcher starts; replacing the plan swaps
only event classification (entries/intents). Per-generation facts — view
modules, declared assets, critical-CSS imports — stay current without
restarting the OS watcher.
*/
/// Handle for replacing the classification plan of a running watcher.
#[derive(Clone)]
pub struct DevFileWatchPlanHandle {
	plan: Arc<RwLock<Arc<DevFileWatchPlan>>>,
}

impl DevFileWatchPlanHandle {
	/// Replace the classification plan for all future events.
	pub fn replace(&self, plan: DevFileWatchPlan) {
		*self.plan.write().expect("dev watch plan lock poisoned") = Arc::new(plan);
	}
}

impl StartedDevFileWatcher {
	/// Handle for replacing the classification plan of this watcher.
	pub fn plan_handle(&self) -> DevFileWatchPlanHandle {
		DevFileWatchPlanHandle {
			plan: Arc::clone(&self.plan),
		}
	}

	/// Classify a path with the watcher's current plan (the callback's read path).
	#[cfg(test)]
	pub(crate) fn classify_with_current_plan(&self, path: impl AsRef<Path>) -> DevFileChange {
		let plan = Arc::clone(&self.plan.read().expect("dev watch plan lock poisoned"));
		plan.classify_path(path)
	}

	/// Bounded receive of the framework-relevant change within `budget` (test only).
	///
	/// Drains raw events until the whole budget elapses, merging every non-empty
	/// change seen into one. Returns `None` if no relevant change arrives in time.
	/// Unlike `recv_settled`, this never blocks past the deadline, so a "no
	/// change" outcome is directly assertable without a helper thread.
	#[cfg(test)]
	pub(crate) fn recv_relevant_within(&self, budget: Duration) -> Option<DevFileChange> {
		use std::time::Instant;
		let deadline = Instant::now() + budget;
		let mut collected: Option<DevFileChange> = None;
		while let Some(remaining) = deadline.checked_duration_since(Instant::now()) {
			match self.rx.recv_timeout(remaining) {
				Ok(Ok(change)) if !change.is_empty() => match &mut collected {
					Some(existing) => existing.extend(change),
					None => collected = Some(change),
				},
				Ok(_) => {}
				Err(mpsc::RecvTimeoutError::Timeout | mpsc::RecvTimeoutError::Disconnected) => {
					break;
				}
			}
		}
		collected
	}

	/// Wait for the next framework-relevant filesystem change.
	pub fn recv(&self) -> Result<DevFileChange, DevWatcherError> {
		loop {
			let message = self.rx.recv().map_err(|_| DevWatcherError::Stopped)?;
			let change = message?;
			if !change.is_empty() {
				return Ok(change);
			}
		}
	}

	/// Wait for the next framework-relevant filesystem change, then keep collecting
	/// additional relevant changes until the filesystem has been quiet for the settle
	/// interval.
	pub fn recv_settled(
		&self,
		settle_interval: Duration,
	) -> Result<DevFileChange, DevWatcherError> {
		let mut change = self.recv()?;
		loop {
			let message = match self.rx.recv_timeout(settle_interval) {
				Ok(message) => message,
				Err(mpsc::RecvTimeoutError::Timeout) => return Ok(change),
				Err(mpsc::RecvTimeoutError::Disconnected) => return Err(DevWatcherError::Stopped),
			};
			let next = message?;
			if next.is_empty() {
				continue;
			}
			change.extend(next);
		}
	}
}

/// Start the dev filesystem watcher.
pub fn start_dev_file_watcher(
	plan: DevFileWatchPlan,
) -> Result<StartedDevFileWatcher, DevWatcherError> {
	if plan.watch_roots().is_empty() {
		return Err(DevWatcherError::EmptyWatchPlan);
	}
	let (tx, rx) = mpsc::channel();
	let watch_roots = plan.watch_roots().to_vec();
	let plan = Arc::new(RwLock::new(Arc::new(plan)));
	let callback_plan = Arc::clone(&plan);
	let mut watcher = notify::recommended_watcher(move |result: notify::Result<notify::Event>| {
		let message = match result {
			Ok(event) => {
				let plan = Arc::clone(&callback_plan.read().expect("dev watch plan lock poisoned"));
				Ok(plan.classify_event(&event))
			}
			Err(source) => Err(DevWatcherError::Notify {
				message: source.to_string(),
			}),
		};
		let _ = tx.send(message);
	})
	.map_err(|source| DevWatcherError::Notify {
		message: source.to_string(),
	})?;
	for root in &watch_roots {
		watcher
			.watch(root, notify::RecursiveMode::Recursive)
			.map_err(|source| DevWatcherError::WatchRoot {
				path: root.display().to_string(),
				message: source.to_string(),
			})?;
	}
	Ok(StartedDevFileWatcher {
		_watcher: watcher,
		rx,
		plan,
	})
}

/// Dev filesystem watcher error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevWatcherError {
	/// Build root directory was empty.
	EmptyRootDir,
	/// No paths were available to watch.
	EmptyWatchPlan,
	/// A configured glob was invalid.
	InvalidGlob {
		/// Rejected glob pattern.
		pattern: String,
		/// Glob parser message.
		message: String,
	},
	/// A configured watch pattern was invalid.
	InvalidWatchPattern {
		/// Rejected watch pattern.
		source_path: String,
		/// Validation message.
		message: String,
	},
	/// A path could not be represented as slash-separated UTF-8.
	InvalidPath {
		/// Rejected path.
		path: String,
	},
	/// Filesystem watcher failed.
	Notify {
		/// Watcher error message.
		message: String,
	},
	/// A watch root could not be registered.
	WatchRoot {
		/// Watch root path.
		path: String,
		/// Watcher error message.
		message: String,
	},
	/// Filesystem watcher stopped.
	Stopped,
}

impl std::fmt::Display for DevWatcherError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::EmptyWatchPlan => f.write_str("dev watch plan cannot be empty"),
			Self::InvalidGlob { pattern, message } => {
				write!(f, "invalid dev watch glob {pattern:?}: {message}")
			}
			Self::InvalidWatchPattern {
				source_path,
				message,
			} => {
				write!(f, "invalid dev watch pattern {source_path:?}: {message}")
			}
			Self::InvalidPath { path } => write!(f, "invalid dev watch path {path:?}"),
			Self::Notify { message } => write!(f, "dev filesystem watcher failed: {message}"),
			Self::WatchRoot { path, message } => {
				write!(f, "watch dev root {path:?}: {message}")
			}
			Self::Stopped => f.write_str("dev filesystem watcher stopped"),
		}
	}
}

impl std::error::Error for DevWatcherError {}

fn parse_watch_pattern(
	source_path: &str,
) -> Result<Option<ParsedWatchPattern<'_>>, DevWatcherError> {
	let source_path = source_path.trim();
	if source_path.is_empty() {
		return Ok(None);
	}
	let is_exclusion = source_path.starts_with('!');
	let parsed_source_path = if is_exclusion {
		source_path.trim_start_matches('!').trim()
	} else {
		source_path
	};
	if parsed_source_path.is_empty() {
		return Err(DevWatcherError::InvalidWatchPattern {
			source_path: source_path.to_owned(),
			message: "exclusion pattern cannot be empty".to_owned(),
		});
	}
	Ok(Some(ParsedWatchPattern {
		source_path: parsed_source_path,
		is_exclusion,
	}))
}

/*
The root dir is only the anchor relative watch sources resolve against, not a
containment boundary: explicit parent escapes and absolute paths outside the
root are supported so any monorepo layout can watch sibling trees.
*/
fn normalize_watch_source(root_dir: &Path, source_path: &str) -> Result<PathBuf, DevWatcherError> {
	let source = Path::new(source_path);
	if source.is_absolute() {
		return pathdiff::diff_paths(source, root_dir).ok_or_else(|| {
			DevWatcherError::InvalidWatchPattern {
				source_path: source_path.to_owned(),
				message: "absolute watch path has no relative form from the root dir".to_owned(),
			}
		});
	}
	let mut normalized = PathBuf::new();
	for component in source.components() {
		match component {
			Component::CurDir => {}
			Component::Normal(part) => normalized.push(part),
			Component::ParentDir => {
				let popped = matches!(
					normalized.components().next_back(),
					Some(Component::Normal(_))
				) && normalized.pop();
				if !popped {
					normalized.push(Component::ParentDir.as_os_str());
				}
			}
			Component::RootDir | Component::Prefix(_) => {
				return Err(DevWatcherError::InvalidWatchPattern {
					source_path: source_path.to_owned(),
					message: "relative watch path cannot contain absolute components".to_owned(),
				});
			}
		}
	}
	if normalized.as_os_str().is_empty() {
		return Ok(PathBuf::from("."));
	}
	Ok(normalized)
}

fn watch_root_for_source(root_dir: &Path, source_path: &Path) -> PathBuf {
	let glob_root = prefix_before_glob(source_path);
	let candidate = lexically_cleaned(&root_dir.join(glob_root));
	if candidate.is_dir() {
		return candidate;
	}
	candidate
		.parent()
		.map(Path::to_path_buf)
		.unwrap_or_else(|| root_dir.to_path_buf())
}

/*
Escaped watch sources join onto the root as `root/../sibling` forms; folding
them lexically keeps watch roots and pinned expectations in real-path shape
without requiring the directories to exist.
*/
fn lexically_cleaned(path: &Path) -> PathBuf {
	let mut cleaned = PathBuf::new();
	for component in path.components() {
		match component {
			Component::CurDir => {}
			Component::ParentDir => {
				let popped = matches!(cleaned.components().next_back(), Some(Component::Normal(_)))
					&& cleaned.pop();
				if !popped {
					cleaned.push(Component::ParentDir.as_os_str());
				}
			}
			other => cleaned.push(other.as_os_str()),
		}
	}
	cleaned
}

fn prefix_before_glob(source_path: &Path) -> PathBuf {
	let mut prefix = PathBuf::new();
	for component in source_path.components() {
		let component_text = component.as_os_str().to_string_lossy();
		if contains_glob_meta(&component_text) {
			break;
		}
		prefix.push(component.as_os_str());
	}
	if prefix.as_os_str().is_empty() {
		PathBuf::from(".")
	} else {
		prefix
	}
}

fn relative_event_path(root_dir: &Path, path: &Path) -> Option<PathBuf> {
	if path.is_absolute() {
		pathdiff::diff_paths(path, root_dir)
	} else {
		Some(path.to_path_buf())
	}
}

fn contains_glob_meta(value: &str) -> bool {
	value.contains('*') || value.contains('?') || value.contains('[') || value.contains('{')
}

fn path_to_slash(path: &Path) -> Result<String, DevWatcherError> {
	path.to_slash()
		.map(|value| {
			if value == "." {
				"**".to_owned()
			} else {
				value.to_string()
			}
		})
		.ok_or_else(|| DevWatcherError::InvalidPath {
			path: path.display().to_string(),
		})
}

#[cfg(test)]
mod tests {
	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};

	use super::*;
	use crate::build_plan::BuildProjectionPlan;
	use crate::projection_compiler::ProjectionBundle;
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn watch_plan() -> BuildDevWatchPlan {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			"/workspace/app",
			"dist",
			FrontendBuildInputs::new(
				"react",
				"pnpm exec",
				"web",
				"vite.config.ts",
				"src/entry.tsx",
				"public",
				"src/critical.css",
			),
			"src/vorma.gen.ts",
			DevWatchConfig::new(
				vec!["content/**/*.md".to_owned()],
				vec!["src/**/*.rs".to_owned()],
				vec!["data/routes.json".to_owned()],
			),
		));
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"src/views/root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		BuildProjectionPlan::compile(&ProjectionBundle::compile(&graph))
			.unwrap()
			.dev_watch()
			.clone()
	}

	#[test]
	fn dev_watch_plan_classifies_configured_and_projected_sources() {
		let root_dir = PathBuf::from("/workspace/app");
		let plan = DevFileWatchPlan::compile(&root_dir, &watch_plan()).unwrap();

		let rust = plan.classify_path("/workspace/app/src/main.rs");
		let content = plan.classify_path("/workspace/app/content/post.md");
		let revalidate = plan.classify_path("/workspace/app/data/routes.json");
		let view = plan.classify_path("/workspace/app/src/views/root.tsx");
		let ignored = plan.classify_path("/workspace/app/README.md");

		assert!(
			rust.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)
		);
		assert!(
			content
				.intents()
				.contains(&BuildDevWatchIntent::GeneralWatch)
		);
		assert!(
			revalidate
				.intents()
				.contains(&BuildDevWatchIntent::ClientRevalidate)
		);
		assert!(
			view.intents()
				.contains(&BuildDevWatchIntent::FrontendBuildInput)
		);
		assert!(ignored.is_empty());
	}

	#[test]
	fn dev_watch_plan_excludes_negated_watch_patterns() {
		let root_dir = PathBuf::from("/workspace/app");
		let plan = DevFileWatchPlan::compile(
			&root_dir,
			&BuildDevWatchPlan::from_entries_for_test(vec![
				(".".to_owned(), BuildDevWatchIntent::GeneralWatch),
				(
					"!.bombadil/**".to_owned(),
					BuildDevWatchIntent::GeneralWatch,
				),
				(
					"src/**/*.rs".to_owned(),
					BuildDevWatchIntent::ServerRecompile,
				),
			]),
		)
		.unwrap();

		let ignored = plan.classify_path("/workspace/app/.bombadil/vite-cache/react/chunk.js");
		let rust = plan.classify_path("/workspace/app/src/main.rs");

		assert!(ignored.is_empty());
		assert!(rust.intents().contains(&BuildDevWatchIntent::GeneralWatch));
		assert!(
			rust.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)
		);
	}

	#[test]
	fn dev_file_change_distinguishes_app_server_generation_from_client_revalidation() {
		let root_dir = PathBuf::from("/workspace/app");
		let plan = DevFileWatchPlan::compile(
			&root_dir,
			&BuildDevWatchPlan::from_entries_for_test(vec![
				(".".to_owned(), BuildDevWatchIntent::GeneralWatch),
				(
					"src/views/root.tsx".to_owned(),
					BuildDevWatchIntent::FrontendBuildInput,
				),
				(
					"src/main.rs".to_owned(),
					BuildDevWatchIntent::ServerRecompile,
				),
				("public".to_owned(), BuildDevWatchIntent::PublicStaticInput),
				(
					"src/critical.css".to_owned(),
					BuildDevWatchIntent::CriticalCssInput,
				),
				(
					"data/routes.json".to_owned(),
					BuildDevWatchIntent::ClientRevalidate,
				),
			]),
		)
		.unwrap();

		assert!(
			!plan
				.classify_path("/workspace/app/runtime/react_hmr_probe.tsx")
				.requires_vorma_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/runtime/react_hmr_probe.tsx")
				.requires_app_server_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/src/views/root.tsx")
				.requires_vorma_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/src/views/root.tsx")
				.requires_app_server_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/src/main.rs")
				.requires_vorma_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/src/main.rs")
				.requires_app_server_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/public/logo.svg")
				.requires_vorma_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/public/logo.svg")
				.requires_app_server_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/public/logo.svg")
				.requires_static_output_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/src/critical.css")
				.requires_vorma_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/src/critical.css")
				.requires_app_server_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/src/critical.css")
				.requires_static_output_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/data/routes.json")
				.requires_vorma_generation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/data/routes.json")
				.requires_app_server_generation()
		);
		assert!(
			plan.classify_path("/workspace/app/data/routes.json")
				.requires_only_client_revalidation()
		);
		assert!(
			!plan
				.classify_path("/workspace/app/src/main.rs")
				.requires_static_output_generation()
		);
	}

	#[test]
	fn dev_watch_plan_resolves_sources_that_explicitly_escape_root() {
		let plan = DevFileWatchPlan::compile(
			"/workspace/app",
			&BuildDevWatchPlan::from_entries_for_test(vec![(
				"../shared/src/**/*.rs".to_owned(),
				BuildDevWatchIntent::ServerRecompile,
			)]),
		)
		.unwrap();

		assert!(
			plan.watch_roots()
				.contains(&PathBuf::from("/workspace/shared"))
		);
		assert!(
			plan.classify_path("/workspace/shared/src/lib.rs")
				.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)
		);
		assert!(plan.classify_path("/workspace/other/src/lib.rs").is_empty());
	}

	#[test]
	fn dev_watch_plan_resolves_absolute_sources_outside_root() {
		let plan = DevFileWatchPlan::compile(
			"/workspace/app",
			&BuildDevWatchPlan::from_entries_for_test(vec![(
				"/workspace/shared/styles.css".to_owned(),
				BuildDevWatchIntent::CriticalCssInput,
			)]),
		)
		.unwrap();

		assert!(
			plan.classify_path("/workspace/shared/styles.css")
				.intents()
				.contains(&BuildDevWatchIntent::CriticalCssInput)
		);
	}

	#[test]
	fn replaced_plan_reclassifies_future_events_without_new_watch_roots() {
		let root = std::env::temp_dir().join(format!(
			"vorma-watch-swap-{}-{}",
			std::process::id(),
			std::time::SystemTime::now()
				.duration_since(std::time::UNIX_EPOCH)
				.unwrap()
				.as_nanos()
		));
		std::fs::create_dir_all(root.join("src")).unwrap();
		let base_plan = BuildDevWatchPlan::from_entries_for_test(vec![(
			"src/app.rs".to_owned(),
			BuildDevWatchIntent::GeneralWatch,
		)]);
		let compiled = DevFileWatchPlan::compile(&root, &base_plan).unwrap();
		/*
		The import lives under the already-watched root but matches no base
		entry, so only the swapped plan can classify it.
		*/
		let import_path = root.join("src/base.css");
		assert!(compiled.classify_path(&import_path).is_empty());

		let watcher = start_dev_file_watcher(compiled).unwrap();
		assert!(watcher.classify_with_current_plan(&import_path).is_empty());

		let handle = watcher.plan_handle();
		let extended = base_plan
			.extended_with_critical_css_imports(&[import_path.to_string_lossy().into_owned()]);
		handle.replace(DevFileWatchPlan::compile(&root, &extended).unwrap());

		let change = watcher.classify_with_current_plan(&import_path);
		assert!(
			change
				.intents()
				.contains(&BuildDevWatchIntent::CriticalCssInput)
		);
		drop(watcher);
		std::fs::remove_dir_all(&root).unwrap();
	}

	fn server_recompile_plan(root: &Path) -> DevFileWatchPlan {
		DevFileWatchPlan::compile(
			root,
			&BuildDevWatchPlan::from_entries_for_test(vec![(
				"src/**/*.rs".to_owned(),
				BuildDevWatchIntent::ServerRecompile,
			)]),
		)
		.unwrap()
	}

	fn event_for(kind: notify::EventKind, path: &Path) -> notify::Event {
		notify::Event::new(kind).add_path(path.to_path_buf())
	}

	/*
	Pins the Linux dev-rebuild bug: a watched source file merely being *opened
	for reading* (inotify IN_OPEN / IN_CLOSE_NOWRITE) must never classify as a
	change. The rebuild's own cargo/rustc reads the sources it compiles; without
	this gate those reads classify as ServerRecompile and cancel-restart the
	in-flight rebuild forever. Real writes must still classify.
	*/
	#[test]
	fn read_only_access_events_never_classify_as_source_change() {
		use notify::EventKind;
		use notify::event::{AccessKind, AccessMode, DataChange, ModifyKind, RemoveKind};

		let root = PathBuf::from("/workspace/app");
		let plan = server_recompile_plan(&root);
		let source = root.join("src/main.rs");

		// Pure non-mutating access events — every one is compiler read noise.
		for kind in [
			EventKind::Access(AccessKind::Open(AccessMode::Any)),
			EventKind::Access(AccessKind::Open(AccessMode::Read)),
			EventKind::Access(AccessKind::Open(AccessMode::Execute)),
			EventKind::Access(AccessKind::Read),
			EventKind::Access(AccessKind::Close(AccessMode::Read)),
			EventKind::Access(AccessKind::Close(AccessMode::Any)),
			EventKind::Access(AccessKind::Any),
		] {
			let change = plan.classify_event(&event_for(kind, &source));
			assert!(
				change.is_empty(),
				"read/open access event {kind:?} must not classify as a source change",
			);
		}

		// Genuine mutations must still classify as ServerRecompile.
		for kind in [
			EventKind::Modify(ModifyKind::Data(DataChange::Any)),
			EventKind::Create(notify::event::CreateKind::File),
			EventKind::Remove(RemoveKind::File),
			// IN_CLOSE_WRITE: a write handle closing is a completed write.
			EventKind::Access(AccessKind::Close(AccessMode::Write)),
		] {
			let change = plan.classify_event(&event_for(kind, &source));
			assert!(
				change
					.intents()
					.contains(&BuildDevWatchIntent::ServerRecompile),
				"mutation event {kind:?} must classify as ServerRecompile",
			);
		}
	}

	/*
	End-to-end pin over a real notify watcher on a temp tree: reproduces the
	inotify open-vs-write divergence. Opening the watched `.rs` file for reading
	(no write) must surface no framework-relevant change; a subsequent write
	must. Fails on the pre-fix code, where the compiler's own reads were
	classified as rebuild-triggering ServerRecompile changes.
	*/
	#[test]
	fn real_watcher_ignores_reads_but_sees_writes() {
		use std::io::Read as _;

		let root = std::env::temp_dir().join(format!(
			"vorma-watch-reads-{}-{}",
			std::process::id(),
			std::time::SystemTime::now()
				.duration_since(std::time::UNIX_EPOCH)
				.unwrap()
				.as_nanos()
		));
		std::fs::create_dir_all(root.join("src")).unwrap();
		let source = root.join("src/main.rs");
		std::fs::write(&source, "fn main() {}\n").unwrap();

		let watcher = start_dev_file_watcher(server_recompile_plan(&root)).unwrap();
		// Let the OS watch registration settle so the reads below are observed.
		std::thread::sleep(Duration::from_millis(200));

		// Open the source for reading many times, exactly like a compiler pass.
		for _ in 0..25 {
			let mut file = std::fs::File::open(&source).unwrap();
			let mut buf = String::new();
			file.read_to_string(&mut buf).unwrap();
		}
		// A read-only burst must surface nothing framework-relevant. If reads
		// leaked through as ServerRecompile, this would return a change.
		let after_reads = watcher.recv_relevant_within(Duration::from_millis(400));
		assert!(
			after_reads.is_none(),
			"reads produced a framework-relevant change: {after_reads:?}",
		);

		// A real write must be seen.
		std::fs::write(&source, "fn main() { let _ = 1; }\n").unwrap();
		let after_write = watcher.recv_relevant_within(Duration::from_secs(5));
		assert!(
			after_write.as_ref().is_some_and(|change| change
				.intents()
				.contains(&BuildDevWatchIntent::ServerRecompile)),
			"write was not observed as a ServerRecompile change: {after_write:?}",
		);

		drop(watcher);
		std::fs::remove_dir_all(&root).unwrap();
	}
}
