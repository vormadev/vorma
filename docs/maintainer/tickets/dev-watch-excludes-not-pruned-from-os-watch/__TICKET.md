# Dev-watch exclusions are classification-only; the OS watch still descends into them

Discovered during P001b (2026-07-01, fixing the Linux dev-rebuild bug). This is NOT that
bug's root cause -- it is a separate wastefulness/robustness issue found alongside it.

## Current behavior

`crates/vorma-build/src/dev_watcher.rs`:

- `DevFileWatchPlan::compile` turns watch patterns into `watch_roots` (directories) and,
  separately, into `exclusions` (negated `!...` globs). `watch_root_for_source` computes a
  directory root per non-exclusion entry.
- `start_dev_file_watcher` registers each watch root with
  `watcher.watch(root, RecursiveMode::Recursive)`.
- Exclusions (`exclusions.iter().any(...)` in `classify_path`) are applied ONLY at
  classification time, to decide whether an already-delivered event is framework-relevant.

Consequence: the recommended app config watches the whole app root (pattern `.`), so the
OS watch descends recursively into everything under it -- including generated trees that
are explicitly excluded, e.g. the framework test fixture's `!.bombadil/**` (cargo target +
Vite cache, hundreds-to-thousands of directories) and `.dist.*`. `notify`'s
`RecursiveMode::Recursive` walks the tree with `WalkDir` and registers one inotify watch
per directory (see `~/.cargo/.../notify-8.2.0/src/inotify.rs::add_watch`), with no
exclusion filter. So the process pays to watch and receive events for large generated
trees whose events are then always discarded by classification.

## Why it matters

- Violates the repo's least-work invariant (`docs/maintainer/REMINDERS.md`: "Everything
  must do the LEAST amount of necessary work ... This is true for the dev server").
- Latent inotify-watch exhaustion risk on large projects: `max_user_watches` is finite
  (65536 on the P001b machine); a big cargo target under a watched root can consume a
  large share for no benefit. If registration hits `ENOSPC`, `notify` returns
  `MaxFilesWatch` and the dev session fails to start.
- Extra event-processing load in the hot callback for events that will always be dropped.

## Desired outcome

Make dev-watch exclusions prune the OS-level watch, not just classification. Registration
should not descend into excluded subtrees. Likely shape: replace the single recursive
`watcher.watch(root, Recursive)` per root with an exclusion-aware recursive registration
that walks the root, skips any directory matching an exclusion (both the directory itself
and its `**` contents), and registers watches only on surviving directories -- mirroring
what mature watchers (watchman, chokidar) do. Newly created directories under a watched
root must also be pruned if they match an exclusion (notify auto-adds watches for created
subdirs in recursive mode; that path must respect exclusions too, or the design must add
subdir watches manually).

Keep macOS (FSEvents) behavior correct: FSEvents uses one kernel recursive stream per
registered path and cannot cheaply prune subtrees, so a per-directory registration model
may need a platform split, or classification-time exclusion must remain the macOS story
while Linux prunes at registration. Decide deliberately and document the reasoning.

## Constraints

- Dev events stay event-driven; no polling (REMINDERS.md).
- Do not change which paths are considered framework-relevant (classification semantics
  are frozen unless a packet grants otherwise) -- this is purely about which directories
  the OS watch covers. A pruned OS watch must still deliver every event that
  classification would have accepted.
- Scope is `crates/vorma-build`.

## Verification expectations

- A unit/integration test that registers a watch root containing an excluded subtree and
  asserts that writes inside the excluded subtree deliver no events (not merely that they
  are classified empty), while writes to non-excluded paths still deliver.
- `make e2e` / `make e2e-smoke` remain green on Linux (the P001b fix already makes them
  green; this must not regress them).
- Dev watcher unit suite green.
