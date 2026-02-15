package tooling

import (
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debouncer batches rapid file events and ensures callbacks don't overlap.
type debouncer struct {
	duration time.Duration
	callback func([]fsnotify.Event)
	mu       sync.Mutex
	timer    *time.Timer
	events   []fsnotify.Event
	stopped  bool
	inFlight bool
	pending  []fsnotify.Event
}

func newDebouncer(duration time.Duration, callback func([]fsnotify.Event)) *debouncer {
	return &debouncer{duration: duration, callback: callback}
}

func (debouncer *debouncer) Add(event fsnotify.Event) {
	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()

	if debouncer.stopped {
		return
	}

	debouncer.events = append(debouncer.events, event)

	if debouncer.timer != nil {
		debouncer.timer.Stop()
	}

	debouncer.timer = time.AfterFunc(debouncer.duration, debouncer.flush)
}

// flush is called by the timer. It checks if a callback is in-flight and either
// runs the callback or queues events for later.
func (debouncer *debouncer) flush() {
	debouncer.mu.Lock()

	if debouncer.stopped {
		debouncer.mu.Unlock()
		return
	}

	events := debouncer.events
	debouncer.events = nil

	if len(events) == 0 {
		debouncer.mu.Unlock()
		return
	}

	// If a callback is already running, queue these events for when it finishes
	if debouncer.inFlight {
		debouncer.pending = append(debouncer.pending, events...)
		debouncer.mu.Unlock()
		return
	}

	// Mark as in-flight and release lock before callback
	debouncer.inFlight = true
	debouncer.mu.Unlock()

	// Run callback outside of lock
	debouncer.callback(events)

	// After callback completes, check for pending events
	debouncer.mu.Lock()
	debouncer.inFlight = false

	if len(debouncer.pending) > 0 && !debouncer.stopped {
		// Move pending to events and schedule another flush
		debouncer.events = debouncer.pending
		debouncer.pending = nil
		debouncer.timer = time.AfterFunc(debouncer.duration, debouncer.flush)
	}
	debouncer.mu.Unlock()
}

// Stop cancels any pending debounced callback and prevents future events.
// This should be called when the watcher is being closed to prevent
// callbacks from firing during or after cleanup.
func (debouncer *debouncer) Stop() {
	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()

	debouncer.stopped = true
	if debouncer.timer != nil {
		debouncer.timer.Stop()
		debouncer.timer = nil
	}
	debouncer.events = nil
	debouncer.pending = nil
}

// isNonEmptyChmodOnly checks if an event is only a chmod operation on a non-empty file.
// We skip these because they're likely just permission changes, not content changes.
// However, chmod on an empty file might be part of a file creation sequence
// (some editors: create empty -> chmod -> write), so we don't skip those.
func isNonEmptyChmodOnly(event fsnotify.Event) bool {
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) ||
		event.Has(fsnotify.Rename) {
		return false
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		return false
	}

	return info.Size() > 0
}
