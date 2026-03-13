package fswatcher

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

/////////////////////////////////////////////////////////////////////
/////// WATCHER
/////////////////////////////////////////////////////////////////////

type WatcherOptions struct {
	WatchRoot        strict.CWDRelPath
	DebounceDuration time.Duration
	Logger           *slog.Logger
	OnAddPath        func(path strict.CWDRelPath)
	OnRemovePath     func(path strict.CWDRelPath)
}

func NewWatcher(opts ...WatcherOptions) *Watcher {
	var opts_to_use WatcherOptions
	if len(opts) == 0 {
		opts_to_use = WatcherOptions{}
	} else {
		opts_to_use = opts[0]
	}

	if filepath.IsAbs(string(opts_to_use.WatchRoot)) {
		panic("[NewWatcher]: watch root must be a CWD-relative path")
	}
	if opts_to_use.DebounceDuration <= 0 {
		opts_to_use.DebounceDuration = 30 * time.Millisecond
	}
	if opts_to_use.Logger == nil {
		opts_to_use.Logger = colorlog.New("Watcher")
	}

	return &Watcher{
		watch_root:        strict.MustNormalize(opts_to_use.WatchRoot),
		debounce_duration: opts_to_use.DebounceDuration,
		logger:            opts_to_use.Logger,
		on_add_path:       opts_to_use.OnAddPath,
		on_remove_path:    opts_to_use.OnRemovePath,
	}
}

type Watcher struct {
	mu                sync.Mutex
	watcher           *fsnotify.Watcher
	watch_root        strict.CWDRelPath
	debounce_duration time.Duration
	logger            *slog.Logger
	on_add_path       func(path strict.CWDRelPath)
	on_remove_path    func(path strict.CWDRelPath)
}

// SetWatchRoot updates the watch root and immediately reconciles.
func (watcher *Watcher) SetWatchRoot(new_root strict.CWDRelPath) {
	if filepath.IsAbs(string(new_root)) {
		panic(
			"[Watcher.SetWatchRoot]: watch root must be a CWD-relative path",
		)
	}

	watcher.mu.Lock()
	defer watcher.mu.Unlock()

	watcher.watch_root = strict.MustNormalize(new_root)

	if watcher.watcher != nil {
		if err := watcher.reconcile_watch_dirs(watcher.watcher); err != nil {
			watcher.logger.Error(
				"[Watcher.SetWatchRoot]: failed to reconcile after watch root change",
				"error",
				err,
			)
		}
	}
}

func (watcher *Watcher) Watch(
	native_ctx context.Context,
	on_evt_batch func([]Evt) error,
) {
	watcher.mu.Lock()
	_watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(
			"[Watcher.Watch]: failed to create fsnotify watcher: " + err.Error(),
		)
	}
	defer _watcher.Close()
	watcher.watcher = _watcher

	if err := watcher.reconcile_watch_dirs(_watcher); err != nil {
		panic(
			"[Watcher.Watch]: failed to reconcile watch directories: " + err.Error(),
		)
	}
	watcher.mu.Unlock()

	var batch []fsnotify.Event
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-native_ctx.Done():
			return

		case _evt, ok := <-_watcher.Events:
			if !ok {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(watcher.debounce_duration)
			batch = append(batch, _evt)

		case err, ok := <-_watcher.Errors:
			if !ok {
				return
			}
			watcher.logger.Error(
				"[Watcher.Watch]: fsnotify error",
				"error",
				err,
			)

		case <-timer.C:
			if len(batch) > 0 {
				deduped_batch := &set.Set[Evt]{}
				reconcile_needed := false

				for _, _evt := range batch {
					evt := ReduceEvt(_evt)
					if evt == nil {
						continue
					}
					deduped_batch.Add(*evt)
					reconcile_needed = reconcile_needed || evt.IsKnownDir ||
						evt.IsDelete()
				}

				if err := on_evt_batch(deduped_batch.Slice()); err != nil {
					watcher.logger.Error(
						"[Watcher.Watch]: batch handler error",
						"error",
						err,
					)
				}

				if reconcile_needed {
					watcher.mu.Lock()
					if err := watcher.reconcile_watch_dirs(_watcher); err != nil {
						watcher.logger.Error(
							"[Watcher.Watch]: failed to reconcile watch directories",
							"error",
							err,
						)
					}
					watcher.mu.Unlock()
				}

				batch = nil
			}
		}
	}
}

// must be called with Watcher.mu held.
func (watcher *Watcher) reconcile_watch_dirs(
	_watcher *fsnotify.Watcher,
) error {
	currently_watched_dirs := &set.Set[strict.CWDRelPath]{}
	for _, path := range _watcher.WatchList() {
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		currently_watched_dirs.Add(strict.MustNormalize(rel))
	}

	desired_watched_dirs := &set.Set[strict.CWDRelPath]{}
	filepath.WalkDir(
		string(watcher.watch_root),
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if slices.Contains(
					[]string{".git", "node_modules", ".waveout"},
					entry.Name(),
				) {
					return filepath.SkipDir
				}
				desired_watched_dirs.Add(strict.MustNormalize(path))
			}
			return nil
		},
	)

	should_watch := desired_watched_dirs.Diff(currently_watched_dirs)
	for path := range should_watch.Range() {
		if watcher.on_add_path != nil {
			watcher.on_add_path(path)
		}
		if err := _watcher.Add(string(path)); err != nil {
			return err
		}
	}

	should_unwatch := currently_watched_dirs.Diff(desired_watched_dirs)
	for path := range should_unwatch.Range() {
		if watcher.on_remove_path != nil {
			watcher.on_remove_path(path)
		}
		_ = _watcher.Remove(string(path))
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// EVENTS
/////////////////////////////////////////////////////////////////////

type Evt struct {
	Path       strict.CWDRelPath
	Op         Op
	IsKnownDir bool
}

func (e *Evt) IsCreate() bool { return e.Op == OpCreate }
func (e *Evt) IsEdit() bool   { return e.Op == OpEdit }
func (e *Evt) IsDelete() bool { return e.Op == OpDelete }

type Op string

const (
	OpCreate Op = "create"
	OpEdit   Op = "edit"
	OpDelete Op = "delete"
)

// Reduces a raw fsnotify event to a simpler representation
// with a create, edit, or delete op. Returns nil for events
// that should be ignored in a typical file watching setup.
// Treats CHMOD events for empty files as writes, as sometimes
// editors will effectuate full content deletions via CHMOD
// truncations.
func ReduceEvt(_evt fsnotify.Event) *Evt {
	path := strict.MustNormalize(_evt.Name)

	stat, stat_err := os.Stat(string(path))
	is_non_empty_file := stat_err == nil && !stat.IsDir() && stat.Size() > 0
	is_dir := stat_err == nil && stat.IsDir()

	has_chmod := _evt.Has(fsnotify.Chmod)
	has_create := _evt.Has(fsnotify.Create)
	has_write := _evt.Has(fsnotify.Write)
	has_remove := _evt.Has(fsnotify.Remove)
	has_rename := _evt.Has(fsnotify.Rename)

	is_non_empty_chmod_only := has_chmod &&
		is_non_empty_file &&
		!has_create &&
		!has_write &&
		!has_remove &&
		!has_rename
	if is_non_empty_chmod_only {
		return nil
	}

	var op Op
	switch {
	case has_remove, has_rename:
		op = OpDelete
	case has_create:
		op = OpCreate
	case has_write, has_chmod:
		op = OpEdit
	default:
		panic("unrecognized fsnotify event: " + _evt.String())
	}

	return &Evt{Path: path, Op: op, IsKnownDir: is_dir}
}
