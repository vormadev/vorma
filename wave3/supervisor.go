package wave3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type supercycle_run_result struct {
	batch_id                   uint64
	current_build_id           string
	current_config_fingerprint string
	err                        error
}

func run_supervisor(
	parent_ctx context.Context,
	watch_root string,
) error {
	if parent_ctx == nil {
		return errors.New("wave3: parent_ctx is nil")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("wave3: create watcher: %w", err)
	}
	defer watcher.Close()

	if err := add_watch_dirs_recursive(
		watcher,
		watch_root,
	); err != nil {
		return err
	}

	var pending_batch []fsnotify.Event
	var batch_flush_timer *time.Timer
	var batch_flush_timer_ch <-chan time.Time
	var next_batch_id uint64
	var previous_build_id string
	var previous_config_fingerprint string
	var current_supercycle_cancel context.CancelFunc
	run_result_ch := make(chan supercycle_run_result)

	flush_batch_and_start_supercycle := func() error {
		if len(pending_batch) == 0 {
			return nil
		}

		current_config_fingerprint, err := read_current_user_config_fingerprint_for_supercycle(
			watch_root,
		)
		if err != nil {
			return err
		}

		next_batch_id++
		batch_id := next_batch_id
		current_build_id := fmt.Sprintf("%d", batch_id)
		batch_evts := append(
			make([]fsnotify.Event, 0, len(pending_batch)),
			pending_batch...,
		)
		pending_batch = pending_batch[:0]

		if current_supercycle_cancel != nil {
			current_supercycle_cancel()
		}

		supercycle_ctx, supercycle_cancel := context.WithCancel(parent_ctx)
		current_supercycle_cancel = supercycle_cancel

		go func(
			batch_id uint64,
			current_build_id string,
			current_config_fingerprint string,
			batch_evts []fsnotify.Event,
		) {
			err := run_supercycle(
				supercycle_ctx,
				supercycle_input{
					batch_id: batch_id,
					batch_data: &supercycle_batch_data{
						evts: batch_evts,
					},
					previous_build_id:           previous_build_id,
					previous_config_fingerprint: previous_config_fingerprint,
					current_config_fingerprint:  current_config_fingerprint,
				},
			)
			select {
			case run_result_ch <- supercycle_run_result{
				batch_id:                   batch_id,
				current_build_id:           current_build_id,
				current_config_fingerprint: current_config_fingerprint,
				err:                        err,
			}:
			case <-parent_ctx.Done():
			}
		}(
			batch_id,
			current_build_id,
			current_config_fingerprint,
			batch_evts,
		)

		return nil
	}

	for {
		select {
		case <-parent_ctx.Done():
			if current_supercycle_cancel != nil {
				current_supercycle_cancel()
			}
			stop_timer_and_drain(batch_flush_timer)
			return parent_ctx.Err()

		case watcher_err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("wave3: watcher errors channel closed")
			}
			return fmt.Errorf("wave3: watcher error: %w", watcher_err)

		case evt, ok := <-watcher.Events:
			if !ok {
				return errors.New("wave3: watcher events channel closed")
			}

			if evt.Has(fsnotify.Create) {
				info, stat_err := os.Stat(evt.Name)
				if stat_err == nil && info.IsDir() {
					if err := add_watch_dirs_recursive(
						watcher,
						evt.Name,
					); err != nil {
						return err
					}
				}
			}

			pending_batch = append(pending_batch, evt)
			if batch_flush_timer == nil {
				batch_flush_timer = time.NewTimer(20 * time.Millisecond)
				batch_flush_timer_ch = batch_flush_timer.C
			} else {
				stop_timer_and_drain(batch_flush_timer)
				batch_flush_timer.Reset(20 * time.Millisecond)
			}

		case <-batch_flush_timer_ch:
			stop_timer_and_drain(batch_flush_timer)
			batch_flush_timer = nil
			batch_flush_timer_ch = nil
			if err := flush_batch_and_start_supercycle(); err != nil {
				return err
			}

		case run_result := <-run_result_ch:
			if run_result.err == nil {
				previous_build_id = run_result.current_build_id
				previous_config_fingerprint = run_result.current_config_fingerprint
				continue
			}
			if errors.Is(run_result.err, context.Canceled) {
				continue
			}
			return fmt.Errorf(
				"wave3: supercycle %d failed: %w",
				run_result.batch_id,
				run_result.err,
			)
		}
	}
}

func read_current_user_config_fingerprint_for_supercycle(
	watch_root string,
) (string, error) {
	if watch_root == "" {
		return "", errors.New("wave3: watch_root is empty")
	}
	return filepath.Clean(watch_root), nil
}

func add_watch_dirs_recursive(
	watcher *fsnotify.Watcher,
	root string,
) error {
	return filepath.WalkDir(root, func(
		path string,
		entry os.DirEntry,
		err error,
	) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if add_err := watcher.Add(path); add_err != nil {
			return fmt.Errorf(
				"wave3: add watcher for %q: %w",
				path,
				add_err,
			)
		}
		return nil
	})
}

func stop_timer_and_drain(timer *time.Timer) {
	if timer == nil {
		return
	}
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
