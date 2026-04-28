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
	"github.com/vormadev/vorma/kit/globset"
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
	WatchRoot        string
	IgnorePatterns   []string
	DebounceDuration time.Duration
	Logger           *slog.Logger
	OnAddPath        func(path string)
	OnRemovePath     func(path string)
}

type Watcher struct {
	mu                sync.Mutex
	watcher           *fsnotify.Watcher
	watch_root        string
	ignore_set        *globset.Set
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

	ignore_set, err := globset.Compile(o.IgnorePatterns)
	if err != nil {
		panic("[Watcher.NewWatcher]: invalid ignore pattern: " + err.Error())
	}

	return &Watcher{
		watch_root:        o.WatchRoot,
		ignore_set:        ignore_set,
		debounce_duration: o.DebounceDuration,
		logger:            o.Logger,
		on_add_path:       o.OnAddPath,
		on_remove_path:    o.OnRemovePath,
		known_paths:       make(map[string]path_entry),
	}
}

// SetWatchRoot updates the watch root and immediately reconciles.
func (w *Watcher) SetWatchRoot(new_root string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.watch_root = new_root

	if w.watcher != nil {
		if err := w.reconcile(w.watcher); err != nil {
			w.logger.Error(
				"[Watcher.SetWatchRoot]: Failed to reconcile after watch root change",
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
				evt := w.reduce_evt(raw)
				if evt == nil {
					continue
				}
				deduped.Add(*evt)
				reconcile_needed = reconcile_needed || evt.IsKnownDir ||
					evt.IsDelete()
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
func (w *Watcher) reduce_evt(raw fsnotify.Event) *Evt {
	path := raw.Name

	if w.is_ignored(path) {
		return nil
	}

	stat, stat_err := os.Stat(path)

	// Path no longer exists: it was deleted.
	if stat_err != nil {
		prev, was_tracked := w.known_paths[path]
		if was_tracked {
			delete(w.known_paths, path)
			return &Evt{Path: path, Op: OpDelete, IsKnownDir: prev.is_dir}
		}
		return nil
	}

	is_dir := stat.IsDir()
	mtime := stat.ModTime()

	prev, existed := w.known_paths[path]
	w.known_paths[path] = path_entry{mtime: mtime, is_dir: is_dir}

	if !existed {
		return &Evt{Path: path, Op: OpCreate, IsKnownDir: is_dir}
	}
	if !is_dir && !mtime.Equal(prev.mtime) {
		return &Evt{Path: path, Op: OpEdit}
	}
	return nil
}

// reconcile synchronises the set of watched directories with what
// actually exists on disk under watch_root, and seeds mtimes for
// any files not yet tracked.
// Must be called with w.mu held.
func (w *Watcher) reconcile(fw *fsnotify.Watcher) error {
	currently_watched := &set.Set[string]{}
	for _, p := range fw.WatchList() {
		currently_watched.Add(fsutil.SysNorm(p))
	}

	desired := &set.Set[string]{}
	if err := filepath.WalkDir(
		w.watch_root,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("error walking watch root: %w", err)
			}
			if w.is_ignored(path) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				norm_path := fsutil.SysNorm(path)
				desired.Add(norm_path)
				w.known_paths[norm_path] = path_entry{is_dir: true}
				return nil
			}
			// Seed entry for files we haven't seen yet.
			norm_path := fsutil.SysNorm(path)
			if _, tracked := w.known_paths[norm_path]; !tracked {
				if info, err := entry.Info(); err == nil {
					w.known_paths[norm_path] = path_entry{mtime: info.ModTime()}
				}
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("error walking watch root: %w", err)
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

func (w *Watcher) is_ignored(path string) bool {
	if w.ignore_set == nil {
		return false
	}

	rel_path, err := filepath.Rel(w.watch_root, path)
	if err != nil {
		rel_path = path
	}

	return w.ignore_set.Match(rel_path)
}
