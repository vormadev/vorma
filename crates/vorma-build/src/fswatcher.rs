use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::mpsc::{self, Receiver, RecvTimeoutError};
use std::thread;
use std::time::{Duration, SystemTime};

use notify::Watcher as NotifyWatcher;
use walkdir::WalkDir;

use crate::config::sys_norm;
use crate::watch_plan::{WatchPlan, split_watch_path};

/////////////////////////////////////////////////////////////////////
/////// EVENTS
/////////////////////////////////////////////////////////////////////

#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub(crate) enum Op {
	Create,
	Edit,
	Delete,
}

#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub(crate) struct Evt {
	pub(crate) path: String,
	pub(crate) op: Op,
	pub(crate) is_known_dir: bool,
}

/////////////////////////////////////////////////////////////////////
/////// WATCHER
/////////////////////////////////////////////////////////////////////

pub(crate) struct WatcherOptions {
	pub(crate) root_dir: PathBuf,
	pub(crate) watch_patterns: Vec<String>,
	pub(crate) debounce_duration: Duration,
	pub(crate) on_add_path: Option<PathHook>,
	pub(crate) on_remove_path: Option<PathHook>,
}

impl Default for WatcherOptions {
	fn default() -> Self {
		Self {
			root_dir: PathBuf::from("."),
			watch_patterns: Vec::new(),
			debounce_duration: Duration::from_millis(30),
			on_add_path: None,
			on_remove_path: None,
		}
	}
}

pub(crate) struct Watcher {
	root_dir: String,
	plan: WatchPlan,
	debounce_duration: Duration,
	on_add_path: Option<PathHook>,
	on_remove_path: Option<PathHook>,
	known_paths: BTreeMap<String, PathEntry>,
	watched_dirs: BTreeSet<String>,
}

type PathHook = Box<dyn FnMut(&str) + Send>;

enum WatchMsg {
	Raw(notify::Result<notify::Event>),
	Stop,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct PathEntry {
	mtime: Option<SystemTime>,
	is_dir: bool,
}

impl Watcher {
	pub(crate) fn new(opts: WatcherOptions) -> Result<Self, String> {
		let debounce_duration = if opts.debounce_duration.is_zero() {
			Duration::from_millis(30)
		} else {
			opts.debounce_duration
		};
		Ok(Self {
			root_dir: sys_norm(opts.root_dir.to_string_lossy().as_ref()),
			plan: WatchPlan::new(&opts.watch_patterns)?,
			debounce_duration,
			on_add_path: opts.on_add_path,
			on_remove_path: opts.on_remove_path,
			known_paths: BTreeMap::new(),
			watched_dirs: BTreeSet::new(),
		})
	}

	pub(crate) fn watch<F>(
		&mut self,
		stop_rx: Receiver<()>,
		ready_tx: mpsc::Sender<Result<(), String>>,
		mut on_evt_batch: F,
	) -> Result<(), String>
	where
		F: FnMut(Vec<Evt>) -> Result<(), String>,
	{
		let (msg_tx, msg_rx) = mpsc::channel();
		let raw_tx = msg_tx.clone();
		let mut fw = notify::recommended_watcher(move |result| {
			let _ = raw_tx.send(WatchMsg::Raw(result));
		})
		.map_err(|err| format!("create filesystem watcher: {err}"))?;
		let stop_tx = msg_tx.clone();
		thread::spawn(move || {
			let _ = stop_rx.recv();
			let _ = stop_tx.send(WatchMsg::Stop);
		});
		match self.reconcile(&mut fw) {
			Ok(()) => {
				let _ = ready_tx.send(Ok(()));
			}
			Err(err) => {
				let _ = ready_tx.send(Err(err.clone()));
				return Err(err);
			}
		}

		let mut batch = Vec::new();
		loop {
			let msg = if batch.is_empty() {
				match msg_rx.recv() {
					Ok(msg) => Ok(msg),
					Err(_) => return Ok(()),
				}
			} else {
				msg_rx.recv_timeout(self.debounce_duration)
			};

			match msg {
				Ok(WatchMsg::Raw(Ok(raw))) => {
					batch.extend(raw.paths);
				}
				Ok(WatchMsg::Raw(Err(err))) => {
					return Err(format!("[Watcher.Watch]: fsnotify error: {err}"));
				}
				Ok(WatchMsg::Stop) => {
					return Ok(());
				}
				Err(RecvTimeoutError::Timeout) if !batch.is_empty() => {
					self.process_batch(&mut fw, std::mem::take(&mut batch), &mut on_evt_batch)?;
				}
				Err(RecvTimeoutError::Timeout) => {}
				Err(RecvTimeoutError::Disconnected) => return Ok(()),
			}
		}
	}

	fn process_batch<F>(
		&mut self,
		fw: &mut notify::RecommendedWatcher,
		batch: Vec<PathBuf>,
		on_evt_batch: &mut F,
	) -> Result<(), String>
	where
		F: FnMut(Vec<Evt>) -> Result<(), String>,
	{
		let mut deduped = BTreeSet::new();
		let mut reconcile_needed = false;
		for raw in batch {
			let (evt, should_reconcile) = self.reduce_path(raw)?;
			reconcile_needed = reconcile_needed || should_reconcile;
			if let Some(evt) = evt {
				deduped.insert(evt);
			}
		}

		if !deduped.is_empty()
			&& let Err(err) = on_evt_batch(deduped.into_iter().collect())
		{
			return Err(format!("watch batch handler error: {err}"));
		}
		if reconcile_needed && let Err(err) = self.reconcile(fw) {
			return Err(format!("watch reconcile error: {err}"));
		}
		Ok(())
	}

	fn reduce_path(&mut self, raw_path: impl AsRef<Path>) -> Result<(Option<Evt>, bool), String> {
		let raw_path = sys_norm(raw_path.as_ref().to_string_lossy().as_ref());
		let path = self.physical_path(&raw_path);
		let logical_path = self.logical_path(&path);
		let stat = match fs::metadata(&path) {
			Ok(stat) => stat,
			Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
				if let Some(prev) = self.known_paths.remove(&path) {
					if self.plan.should_emit(&logical_path, prev.is_dir) {
						return Ok((
							Some(Evt {
								path: logical_path,
								op: Op::Delete,
								is_known_dir: prev.is_dir,
							}),
							true,
						));
					}
					return Ok((None, prev.is_dir));
				}
				return Ok((None, true));
			}
			Err(err) => {
				return Err(format!("error statting watched path {path:?}: {err}"));
			}
		};

		let is_dir = stat.is_dir();
		let mtime = stat.modified().ok();
		let should_reconcile = is_dir;
		let prev = self
			.known_paths
			.insert(path.clone(), PathEntry { mtime, is_dir });

		if !self.plan.should_emit(&logical_path, is_dir) {
			return Ok((None, should_reconcile));
		}

		let Some(prev) = prev else {
			return Ok((
				Some(Evt {
					path: logical_path,
					op: Op::Create,
					is_known_dir: is_dir,
				}),
				should_reconcile,
			));
		};

		if !is_dir && mtime != prev.mtime {
			return Ok((
				Some(Evt {
					path: logical_path,
					op: Op::Edit,
					is_known_dir: false,
				}),
				should_reconcile,
			));
		}

		Ok((None, should_reconcile))
	}
	fn reconcile(&mut self, fw: &mut notify::RecommendedWatcher) -> Result<(), String> {
		let desired = self.desired_watch_dirs()?;
		let currently_watched = self.watched_dirs.clone();

		for path in desired.difference(&currently_watched) {
			if let Some(on_add_path) = &mut self.on_add_path {
				on_add_path(path);
			}
			fw.watch(Path::new(path), notify::RecursiveMode::NonRecursive)
				.map_err(|err| format!("error adding watch for path '{path}': {err}"))?;
			self.watched_dirs.insert(path.clone());
		}

		for path in currently_watched.difference(&desired) {
			if let Some(on_remove_path) = &mut self.on_remove_path {
				on_remove_path(path);
			}
			let _ = fw.unwatch(Path::new(path));
			self.watched_dirs.remove(path);
		}

		Ok(())
	}

	fn desired_watch_dirs(&mut self) -> Result<BTreeSet<String>, String> {
		let mut desired = BTreeSet::new();

		for root in self.plan.roots.clone() {
			self.add_watch_parent_dirs(&mut desired, &root.path)?;

			let physical_root = self.physical_path(&root.path);
			let info = match fs::metadata(&physical_root) {
				Ok(info) => info,
				Err(err) if err.kind() == std::io::ErrorKind::NotFound => continue,
				Err(err) => {
					return Err(format!(
						"error statting watch root {:?}: {err}",
						physical_root
					));
				}
			};
			if !info.is_dir() {
				continue;
			}
			if root.dynamic && !self.plan.should_emit(&root.path, true) {
				continue;
			}
			self.walk_watch_root(&mut desired, &physical_root)?;
		}

		Ok(desired)
	}

	fn add_watch_parent_dirs(
		&mut self,
		desired: &mut BTreeSet<String>,
		watch_path: &str,
	) -> Result<(), String> {
		let dir = if watch_path == "." {
			".".to_owned()
		} else {
			Path::new(watch_path)
				.parent()
				.map(|path| path.to_string_lossy().into_owned())
				.unwrap_or_else(|| ".".to_owned())
		};

		let mut current = self.root_dir.clone();
		self.add_watch_dir(desired, &current)?;
		for part in split_watch_path(&dir) {
			current = Path::new(&current)
				.join(part)
				.to_string_lossy()
				.into_owned();
			self.add_watch_dir(desired, &current)?;
		}
		Ok(())
	}

	fn add_watch_dir(&mut self, desired: &mut BTreeSet<String>, dir: &str) -> Result<(), String> {
		let info = match fs::metadata(dir) {
			Ok(info) => info,
			Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(()),
			Err(err) => return Err(format!("error statting watch directory {dir:?}: {err}")),
		};
		if !info.is_dir() {
			return Ok(());
		}

		let norm_path = sys_norm(dir);
		desired.insert(norm_path.clone());
		self.known_paths.insert(
			norm_path,
			PathEntry {
				mtime: None,
				is_dir: true,
			},
		);
		Ok(())
	}

	fn walk_watch_root(
		&mut self,
		desired: &mut BTreeSet<String>,
		root: &str,
	) -> Result<(), String> {
		let mut entries = WalkDir::new(root).into_iter();
		while let Some(entry) = entries.next() {
			let entry = entry.map_err(|err| format!("error walking watch root {root:?}: {err}"))?;
			let norm_path = sys_norm(entry.path().to_string_lossy().as_ref());
			let logical_path = self.logical_path(&norm_path);
			if entry.file_type().is_dir() {
				if !self.plan.should_watch_dir(&logical_path) {
					entries.skip_current_dir();
					continue;
				}
				desired.insert(norm_path.clone());
				self.known_paths.insert(
					norm_path,
					PathEntry {
						mtime: None,
						is_dir: true,
					},
				);
				continue;
			}
			if !self.plan.should_emit(&logical_path, false) {
				continue;
			}
			self.known_paths.entry(norm_path).or_insert_with(|| {
				let mtime = entry
					.metadata()
					.ok()
					.and_then(|metadata| metadata.modified().ok());
				PathEntry {
					mtime,
					is_dir: false,
				}
			});
		}
		Ok(())
	}

	fn physical_path(&self, path: &str) -> String {
		let path = Path::new(path);
		if path.is_absolute() {
			return sys_norm(path.to_string_lossy().as_ref());
		}
		sys_norm(
			Path::new(&self.root_dir)
				.join(path)
				.to_string_lossy()
				.as_ref(),
		)
	}

	fn logical_path(&self, path: &str) -> String {
		crate::config::relative_path(&self.root_dir, path)
			.map(|path| {
				let path = sys_norm(path.to_string_lossy().as_ref());
				if path.is_empty() {
					".".to_owned()
				} else {
					path
				}
			})
			.unwrap_or_else(|| sys_norm(path))
	}
}

#[cfg(test)]
mod tests {
	use std::time::{SystemTime, UNIX_EPOCH};

	use super::*;

	#[test]
	fn reduce_path_emits_create_edit_and_delete() {
		let root = temp_abs_dir("reduce-path");
		let path = root.join("main.rs");
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root.clone(),
			watch_patterns: vec![".".to_owned()],
			..WatcherOptions::default()
		})
		.unwrap();

		fs::write(&path, "one").unwrap();
		let (evt, _) = watcher.reduce_path(&path).unwrap();
		let evt = evt.unwrap();
		assert_eq!(evt.path, "main.rs");
		assert_eq!(evt.op, Op::Create);

		std::thread::sleep(Duration::from_millis(2));
		fs::write(&path, "two").unwrap();
		let (evt, _) = watcher.reduce_path(&path).unwrap();
		assert_eq!(evt.unwrap().op, Op::Edit);

		fs::remove_file(&path).unwrap();
		let (evt, _) = watcher.reduce_path(&path).unwrap();
		assert_eq!(evt.unwrap().op, Op::Delete);

		let _ = fs::remove_dir_all(root);
	}

	#[test]
	fn reduce_path_reports_non_not_found_stat_errors() {
		let root = temp_abs_dir("reduce-path-stat-error");
		let not_dir = root.join("not-dir");
		fs::write(&not_dir, "file").unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root.clone(),
			watch_patterns: vec![".".to_owned()],
			..WatcherOptions::default()
		})
		.unwrap();

		let error = watcher.reduce_path(not_dir.join("child.rs")).unwrap_err();

		assert!(error.contains("error statting watched path"));
		let _ = fs::remove_dir_all(root);
	}

	#[test]
	fn reduce_path_roots_relative_event_paths_after_snapshot() {
		let root = temp_abs_dir("reduce-path-relative-event");
		let src = root.join("src");
		let path = src.join("main.rs");
		fs::create_dir_all(&src).unwrap();
		fs::write(&path, "one").unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root.clone(),
			watch_patterns: vec![".".to_owned()],
			..WatcherOptions::default()
		})
		.unwrap();
		watcher.desired_watch_dirs().unwrap();

		std::thread::sleep(Duration::from_millis(2));
		fs::write(&path, "two").unwrap();
		let (evt, _) = watcher.reduce_path("src/main.rs").unwrap();
		let evt = evt.unwrap();
		assert_eq!(evt.path, "src/main.rs");
		assert_eq!(evt.op, Op::Edit);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn desired_watch_dirs_for_dot_pattern_watches_recursively_with_exclusions() {
		let root_dir = test_root_dir();
		let root = temp_rel_dir("dot-pattern");
		fs::create_dir_all(path(&root, "src/nested")).unwrap();
		fs::create_dir_all(path(&root, "node_modules/pkg")).unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec![root.clone(), format!("!{}", path(&root, "node_modules"))],
			..WatcherOptions::default()
		})
		.unwrap();

		assert_watch_dirs(
			&mut watcher,
			&root_dir,
			&[
				".",
				"target",
				&root,
				&path(&root, "src"),
				&path(&root, "src/nested"),
			],
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn desired_watch_dirs_for_file_pattern_watches_parent_only() {
		let root_dir = test_root_dir();
		let root = temp_rel_dir("file-pattern");
		fs::write(path(&root, "server.rs"), "fn main() {}\n").unwrap();
		fs::create_dir_all(path(&root, "nested")).unwrap();
		fs::write(path(&root, "nested/server.rs"), "fn main() {}\n").unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec![format!("./{}", path(&root, "server.rs"))],
			..WatcherOptions::default()
		})
		.unwrap();

		assert_watch_dirs(&mut watcher, &root_dir, &[".", "target", &root]);
		assert!(watcher.plan.should_emit(&path(&root, "server.rs"), false));
		assert!(
			!watcher
				.plan
				.should_emit(&path(&root, "nested/server.rs"), false)
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn desired_watch_dirs_for_recursive_file_pattern_watches_matching_subtree() {
		let root_dir = test_root_dir();
		let root = temp_rel_dir("recursive-file-pattern");
		fs::create_dir_all(path(&root, "app/content/deep")).unwrap();
		fs::create_dir_all(path(&root, "other")).unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec![format!("{}/**/*.md", path(&root, "app"))],
			..WatcherOptions::default()
		})
		.unwrap();

		assert_watch_dirs(
			&mut watcher,
			&root_dir,
			&[
				".",
				"target",
				&root,
				&path(&root, "app"),
				&path(&root, "app/content"),
				&path(&root, "app/content/deep"),
			],
		);
		assert!(
			watcher
				.plan
				.should_emit(&path(&root, "app/content/post.md"), false)
		);
		assert!(
			!watcher
				.plan
				.should_emit(&path(&root, "app/content/post.txt"), false)
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn desired_watch_dirs_for_single_level_pattern_prunes_deeper_dirs() {
		let root_dir = test_root_dir();
		let root = temp_rel_dir("single-level-pattern");
		fs::create_dir_all(path(&root, "app/nested")).unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec![format!("{}/*.md", path(&root, "app"))],
			..WatcherOptions::default()
		})
		.unwrap();

		assert_watch_dirs(
			&mut watcher,
			&root_dir,
			&[".", "target", &root, &path(&root, "app")],
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn desired_watch_dirs_keeps_reincluded_path_under_excluded_subtree() {
		let root_dir = test_root_dir();
		let root = temp_rel_dir("reincluded-path");
		fs::create_dir_all(path(&root, "tmp/other/deep")).unwrap();
		fs::write(path(&root, "tmp/keep.rs"), "fn main() {}\n").unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec![
				root.clone(),
				format!("!{}/**", path(&root, "tmp")),
				path(&root, "tmp/keep.rs"),
			],
			..WatcherOptions::default()
		})
		.unwrap();

		assert_watch_dirs(
			&mut watcher,
			&root_dir,
			&[".", "target", &root, &path(&root, "tmp")],
		);
		assert!(watcher.plan.should_emit(&path(&root, "tmp/keep.rs"), false));
		assert!(
			!watcher
				.plan
				.should_emit(&path(&root, "tmp/other/drop.rs"), false)
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn escaped_watch_patterns_watch_and_emit_sibling_paths() {
		let parent = temp_abs_dir("escaped-watch-parent");
		let root_dir = parent.join("app");
		let sibling = parent.join("shared_package");
		fs::create_dir_all(root_dir.join("src")).unwrap();
		fs::create_dir_all(sibling.join("src/nested")).unwrap();
		let file = sibling.join("src/nested/button.tsx");
		fs::write(&file, "export const Button = () => null;\n").unwrap();
		let mut watcher = Watcher::new(WatcherOptions {
			root_dir: root_dir.clone(),
			watch_patterns: vec!["../shared_package/src/**/*.tsx".to_owned()],
			..WatcherOptions::default()
		})
		.unwrap();

		let desired = watcher.desired_watch_dirs().unwrap();

		assert!(desired.contains(&sys_norm(parent.to_string_lossy().as_ref())));
		assert!(desired.contains(&sys_norm(sibling.join("src").to_string_lossy().as_ref())));
		assert!(desired.contains(&sys_norm(
			sibling.join("src/nested").to_string_lossy().as_ref()
		)));
		std::thread::sleep(Duration::from_millis(2));
		fs::write(&file, "export const Button = () => 'updated';\n").unwrap();
		let (evt, _) = watcher.reduce_path(&file).unwrap();
		let evt = evt.unwrap();
		assert_eq!(evt.path, "../shared_package/src/nested/button.tsx");
		assert_eq!(evt.op, Op::Edit);
		fs::remove_dir_all(parent).unwrap();
	}

	fn assert_watch_dirs(watcher: &mut Watcher, root_dir: &Path, expected: &[&str]) {
		let actual = watcher.desired_watch_dirs().unwrap();
		let expected = expected
			.iter()
			.map(|path| sys_norm(root_dir.join(path).to_string_lossy().as_ref()))
			.collect::<BTreeSet<_>>();
		assert_eq!(actual, expected);
	}

	fn path(root: &str, child: &str) -> String {
		Path::new(root).join(child).to_string_lossy().into_owned()
	}

	fn temp_abs_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-fswatcher-{name}-{nonce}"));
		fs::create_dir_all(&root).unwrap();
		root
	}

	fn test_root_dir() -> PathBuf {
		std::env::current_dir().unwrap()
	}

	fn temp_rel_dir(name: &str) -> String {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = format!("target/vorma-fswatcher-{name}-{nonce}");
		fs::create_dir_all(&root).unwrap();
		root
	}
}
