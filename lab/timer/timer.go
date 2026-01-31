// Package timer provides a simple timer for measuring the duration of code execution.
package timer

import (
	"log"
	"time"
)

// Timer is a simple timer for measuring the duration of code execution.
type Timer struct {
	start time.Time
	on    bool
}

// New returns a new timer.
func New() *Timer {
	return &Timer{start: time.Now(), on: true}
}

// Conditional returns a new timer that is only active if the condition is true.
func Conditional(condition bool) *Timer {
	return &Timer{start: time.Now(), on: condition}
}

// Checkpoint prints the duration since the last checkpoint and resets the timer.
func (t *Timer) Checkpoint(label string) {
	if !t.on {
		return
	}
	log.Println("duration for", label, ":", time.Since(t.start))
	t.Reset()
}

// Reset resets the timer.
func (t *Timer) Reset() {
	if !t.on {
		return
	}
	t.start = time.Now()
}

// Example usage: `defer timer.Duration("some label", time.Now())`
func Duration(label string, t time.Time) {
	log.Printf(` %s completed in %s`, label, time.Since(t))
}
