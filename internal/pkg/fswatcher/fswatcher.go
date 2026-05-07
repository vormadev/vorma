package fswatcher

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/set"
)

/////////////////////////////////////////////////////////////////////
/////// EVENTS
/////////////////////////////////////////////////////////////////////

type Op string

const (
	OpCreate Op = "create"
	OpEdit   Op = "edit"
	OpDelete Op = "delete"
)

type Evt struct {
	Path       string
	Op         Op
	IsKnownDir bool
}

func (e *Evt) IsCreate() bool { return e.Op == OpCreate }
func (e *Evt) IsEdit() bool   { return e.Op == OpEdit }
func (e *Evt) IsDelete() bool { return e.Op == OpDelete }

/////////////////////////////////////////////////////////////////////
/////// WATCHER
/////////////////////////////////////////////////////////////////////

type WatcherOptions struct {
	WatchPatterns    []string
	DebounceDuration time.Duration
	Logger           *slog.Logger
	OnAddPath        func(path string)
	OnRemovePath     func(path string)
}

type Watcher struct {
	mu                sync.Mutex
	watcher           *fsnotify.Watcher
	plan              *watch_plan
	debounce_duration time.Duration
	logger            *slog.Logger
	on_add_path       func(path string)
	on_remove_path    func(path string)
	known_paths       map[string]path_entry
}

type path_entry struct {
	mtime  time.Time
	is_dir bool
}

func NewWatcher(opts ...WatcherOptions) *Watcher {
	var o WatcherOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.DebounceDuration <= 0 {
		o.DebounceDuration = 30 * time.Millisecond
	}
	if o.Logger == nil {
		o.Logger = colorlog.New("Watcher")
	}

	plan, err := new_watch_plan(o.WatchPatterns)
	if err != nil {
		panic("[Watcher.NewWatcher]: invalid watch pattern: " + err.Error())
	}

	return &Watcher{
		plan:              plan,
		debounce_duration: o.DebounceDuration,
		logger:            o.Logger,
		on_add_path:       o.OnAddPath,
		on_remove_path:    o.OnRemovePath,
		known_paths:       make(map[string]path_entry),
	}
}

// SetWatchPatterns updates the watch patterns and immediately reconciles.
func (w *Watcher) SetWatchPatterns(patterns []string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	plan, err := new_watch_plan(patterns)
	if err != nil {
		w.logger.Error(
			"[Watcher.SetWatchPatterns]: Failed to compile watch patterns",
			"error",
			err,
		)
		return
	}
	w.plan = plan

	if w.watcher != nil {
		if err := w.reconcile(w.watcher); err != nil {
			w.logger.Error(
				"[Watcher.SetWatchPatterns]: Failed to reconcile after watch pattern change",
				"error",
				err,
			)
		}
	}
}

func (w *Watcher) Watch(ctx context.Context, on_evt_batch func([]Evt) error) {
	w.mu.Lock()
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		panic(
			"[Watcher.Watch]: Failed to create fsnotify watcher: " + err.Error(),
		)
	}
	defer fw.Close()
	w.watcher = fw

	if err := w.reconcile(fw); err != nil {
		panic(
			"[Watcher.Watch]: Failed to reconcile watch directories: " + err.Error(),
		)
	}
	w.mu.Unlock()

	var batch []fsnotify.Event
	timer := time.NewTimer(0)
	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case raw, ok := <-fw.Events:
			if !ok {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.debounce_duration)
			batch = append(batch, raw)

		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			w.logger.Error("[Watcher.Watch]: fsnotify error", "error", err)

		case <-timer.C:
			if len(batch) == 0 {
				continue
			}

			w.mu.Lock()

			deduped := &set.Set[Evt]{}
			reconcile_needed := false
			for _, raw := range batch {
				evt, should_reconcile := w.reduce_evt(raw)
				reconcile_needed = reconcile_needed || should_reconcile
				if evt != nil {
					deduped.Add(*evt)
				}
			}

			if s := deduped.Slice(); len(s) > 0 {
				if err := on_evt_batch(s); err != nil {
					w.logger.Error(
						"[Watcher.Watch]: batch handler error",
						"error",
						err,
					)
				}
			}

			if reconcile_needed {
				if err := w.reconcile(fw); err != nil {
					w.logger.Error(
						"[Watcher.Watch]: failed to reconcile",
						"error",
						err,
					)
				}
			}

			batch = nil
			w.mu.Unlock()
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNALS
/////////////////////////////////////////////////////////////////////

// reduce_evt collapses a raw fsnotify event into a simpler Evt
// using mtime comparison rather than interpreting op flags.
// Must be called with w.mu held.
func (w *Watcher) reduce_evt(raw fsnotify.Event) (*Evt, bool) {
	path := fsutil.SysNorm(raw.Name)

	stat, stat_err := os.Stat(path)

	// Path no longer exists: it was deleted.
	if stat_err != nil {
		prev, was_tracked := w.known_paths[path]
		if was_tracked {
			delete(w.known_paths, path)
			if w.plan.should_emit(path, prev.is_dir) {
				return &Evt{Path: path, Op: OpDelete, IsKnownDir: prev.is_dir}, true
			}
			return nil, prev.is_dir
		}
		return nil, true
	}

	is_dir := stat.IsDir()
	mtime := stat.ModTime()
	should_reconcile := is_dir

	prev, existed := w.known_paths[path]
	w.known_paths[path] = path_entry{mtime: mtime, is_dir: is_dir}

	if !w.plan.should_emit(path, is_dir) {
		return nil, should_reconcile
	}

	if !existed {
		return &Evt{Path: path, Op: OpCreate, IsKnownDir: is_dir}, should_reconcile
	}
	if !is_dir && !mtime.Equal(prev.mtime) {
		return &Evt{Path: path, Op: OpEdit}, should_reconcile
	}
	return nil, should_reconcile
}

// reconcile synchronises the set of watched directories with the watch plan,
// and seeds mtimes for any matching files not yet tracked.
// Must be called with w.mu held.
func (w *Watcher) reconcile(fw *fsnotify.Watcher) error {
	currently_watched := &set.Set[string]{}
	for _, p := range fw.WatchList() {
		currently_watched.Add(fsutil.SysNorm(p))
	}

	desired, err := w.desired_watch_dirs()
	if err != nil {
		return err
	}

	for path := range desired.Diff(currently_watched).Range() {
		if w.on_add_path != nil {
			w.on_add_path(path)
		}
		if err := fw.Add(path); err != nil {
			return fmt.Errorf("error adding watch for path '%s': %w", path, err)
		}
	}

	for path := range currently_watched.Diff(desired).Range() {
		if w.on_remove_path != nil {
			w.on_remove_path(path)
		}
		_ = fw.Remove(path)
	}

	return nil
}

func (w *Watcher) desired_watch_dirs() (*set.Set[string], error) {
	desired := set.New[string]()

	for _, root := range w.plan.roots {
		if err := w.add_watch_parent_dirs(desired, root.path); err != nil {
			return nil, err
		}

		info, err := os.Stat(root.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("error statting watch root %q: %w", root.path, err)
		}
		if !info.IsDir() {
			continue
		}
		if root.dynamic && !w.plan.should_emit(root.path, true) {
			continue
		}
		if err := w.walk_watch_root(desired, root.path); err != nil {
			return nil, err
		}
	}

	return desired, nil
}

func (w *Watcher) add_watch_parent_dirs(
	desired *set.Set[string],
	watch_path string,
) error {
	dir := filepath.Dir(watch_path)
	if watch_path == "." {
		dir = "."
	}

	parts := split_watch_path(dir)
	current := "."
	if err := w.add_watch_dir(desired, current); err != nil {
		return err
	}
	for _, part := range parts {
		current = filepath.Join(current, part)
		if err := w.add_watch_dir(desired, current); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) add_watch_dir(desired *set.Set[string], dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error statting watch directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil
	}
	norm_path := fsutil.SysNorm(dir)
	desired.Add(norm_path)
	w.known_paths[norm_path] = path_entry{is_dir: true}
	return nil
}

func (w *Watcher) walk_watch_root(
	desired *set.Set[string],
	root string,
) error {
	if err := filepath.WalkDir(
		root,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("error walking watch root %q: %w", root, err)
			}
			norm_path := fsutil.SysNorm(path)
			if entry.IsDir() {
				if !w.plan.should_watch_dir(norm_path) {
					return filepath.SkipDir
				}
				desired.Add(norm_path)
				w.known_paths[norm_path] = path_entry{is_dir: true}
				return nil
			}
			if !w.plan.should_emit(norm_path, false) {
				return nil
			}
			if _, tracked := w.known_paths[norm_path]; !tracked {
				if info, err := entry.Info(); err == nil {
					w.known_paths[norm_path] = path_entry{mtime: info.ModTime()}
				}
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("error walking watch root %q: %w", root, err)
	}
	return nil
}
