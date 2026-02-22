package watch_test

import (
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

func TestDebouncer_BatchesRapidEventsIntoOneCallback(t *testing.T) {
	var mu sync.Mutex
	var batches [][]fsnotify.Event
	done := make(chan struct{}, 1)

	debouncer := watch.NewDebouncer(20*time.Millisecond, func(events []fsnotify.Event) {
		copyOfEvents := append([]fsnotify.Event(nil), events...)
		mu.Lock()
		batches = append(batches, copyOfEvents)
		mu.Unlock()
		done <- struct{}{}
	})
	defer debouncer.Stop()

	debouncer.Add(fsnotify.Event{Name: "a.txt", Op: fsnotify.Write})
	debouncer.Add(fsnotify.Event{Name: "b.txt", Op: fsnotify.Write})

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for debounced callback")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(batches) != 1 {
		t.Fatalf("expected 1 callback batch, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Fatalf("expected 2 events in callback batch, got %d", len(batches[0]))
	}
}

func TestDebouncer_QueuesPendingEventsWhileCallbackIsInFlight(t *testing.T) {
	started := make(chan []fsnotify.Event, 2)
	release := make(chan struct{}, 2)

	debouncer := watch.NewDebouncer(10*time.Millisecond, func(events []fsnotify.Event) {
		started <- append([]fsnotify.Event(nil), events...)
		<-release
	})
	defer debouncer.Stop()

	firstEvent := fsnotify.Event{Name: "first.txt", Op: fsnotify.Write}
	secondEvent := fsnotify.Event{Name: "second.txt", Op: fsnotify.Write}

	debouncer.Add(firstEvent)

	var firstBatch []fsnotify.Event
	select {
	case firstBatch = <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first callback")
	}

	if len(firstBatch) != 1 || firstBatch[0].Name != firstEvent.Name {
		t.Fatalf("unexpected first batch: %#v", firstBatch)
	}

	debouncer.Add(secondEvent)
	time.Sleep(30 * time.Millisecond) // longer than debounce duration

	select {
	case batch := <-started:
		t.Fatalf("expected no second callback while first is in-flight, got %#v", batch)
	default:
	}

	release <- struct{}{}

	var secondBatch []fsnotify.Event
	select {
	case secondBatch = <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for queued second callback")
	}

	if len(secondBatch) != 1 || secondBatch[0].Name != secondEvent.Name {
		t.Fatalf("unexpected second batch: %#v", secondBatch)
	}

	release <- struct{}{}
}

func TestDebouncer_StopPreventsPendingAndFutureCallbacks(t *testing.T) {
	called := make(chan struct{}, 1)

	debouncer := watch.NewDebouncer(20*time.Millisecond, func(_ []fsnotify.Event) {
		called <- struct{}{}
	})

	debouncer.Add(fsnotify.Event{Name: "before-stop.txt", Op: fsnotify.Write})
	debouncer.Stop()

	select {
	case <-called:
		t.Fatal("callback should not run after Stop")
	case <-time.After(80 * time.Millisecond):
	}

	debouncer.Add(fsnotify.Event{Name: "after-stop.txt", Op: fsnotify.Write})

	select {
	case <-called:
		t.Fatal("callback should not run for events added after Stop")
	case <-time.After(80 * time.Millisecond):
	}
}
