// Package tasks provides memoized, concurrency-safe task execution.
//
// A Task takes comparable input and returns data or an error. Calls to the same
// Task with the same input share one execution within a Cache. The Cache is the
// sharing boundary: calls using different Cache values do not share results.
//
// Task expiration is configured when the Task is created. With no expiration
// argument, completed results are retained for the lifetime of the Cache. A
// positive expiration retains completed results for that duration, measured from
// when the task function returns. A zero expiration coalesces only concurrent
// in-flight calls and does not retain completed results.
//
// Completed results include successful values and non-cancellation errors.
// Errors caused by context cancellation or deadline expiration are not retained.
//
// Tasks are automatically protected from circular dependencies by Go's
// compile-time "initialization cycle" errors when defined as package-level
// variables.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

// AnyTask is the type-erased interface implemented by all Task instances.
type AnyTask interface {
	RunWithAnyInput(ctx *Cache, input any) (any, error)
}

// Task is a typed, memoized unit of work.
type Task[I comparable, O any] struct {
	id              uint64
	fn              func(ctx *Cache, input I) (O, error)
	cache_completed bool
	expiration      time.Duration
}

// NewTask creates a Task from fn.
//
// NewTask(fn) caches completed results for the lifetime of the Cache used to run
// it. NewTask(fn, d) with d > 0 caches completed results for d, measured from
// when fn returns. NewTask(fn, 0) only coalesces concurrent in-flight calls and
// does not retain completed results.
//
// NewTask returns nil if fn is nil. It panics if more than one expiration is
// provided or if the expiration is negative.
func NewTask[I comparable, O any](
	fn func(ctx *Cache, input I) (O, error),
	expiration ...time.Duration,
) *Task[I, O] {
	if len(expiration) > 1 {
		panic("tasks: NewTask accepts at most one expiration")
	}
	cache_completed := true
	var expires_after time.Duration
	if len(expiration) == 1 {
		expires_after = expiration[0]
		if expires_after < 0 {
			panic("tasks: NewTask expiration must be non-negative")
		}
		cache_completed = expires_after != 0
	}
	if fn == nil {
		return nil
	}
	return &Task[I, O]{
		id:              global_task_id.Add(1),
		fn:              fn,
		cache_completed: cache_completed,
		expiration:      expires_after,
	}
}

// RunWithAnyInput executes the Task with a type-erased input.
//
// RunWithAnyInput returns an error if input does not have the Task's input type.
func (t *Task[I, O]) RunWithAnyInput(ctx *Cache, input any) (any, error) {
	typed, ok := input.(I)
	if !ok {
		return nil, fmt.Errorf(
			"tasks: input type mismatch: expected %s, got %T",
			reflect.TypeFor[I](), input,
		)
	}
	return run_task(ctx, t, typed)
}

// Run executes the Task with input using ctx as the sharing boundary.
func (t *Task[I, O]) Run(ctx *Cache, input I) (O, error) {
	return run_task(ctx, t, input)
}

// BindInput creates a Prepared that captures the input and an optional
// destination pointer for the result.
//
// If a destination is provided, running the Prepared stores the completed result
// there after the Task succeeds.
func (t *Task[I, O]) BindInput(input I, dest ...*O) Prepared {
	var dest_ptr *O
	if len(dest) > 0 {
		dest_ptr = dest[0]
	}
	return prepare_task(t, input, dest_ptr)
}

// Cache stores memoized task results by task/input pair.
//
// A Cache defines which Task calls share in-flight work and completed results.
// Its context supplies cancellation and deadline semantics.
type Cache struct {
	store *cache_store
	ctx   context.Context
}

// NewCache creates a Cache for memoized task results.
//
// NewCache panics if parent is nil.
func NewCache(parent context.Context) *Cache {
	if parent == nil {
		panic("tasks: nil context")
	}
	c := &Cache{
		store: &cache_store{
			results: make(map[task_key]cache_entry, 4),
		},
		ctx: parent,
	}
	return c
}

// Context returns the underlying context.Context.
func (c *Cache) Context() context.Context {
	return c.ctx
}

// WithContext returns a child Cache that shares the same task cache
// but uses the provided context for cancellation/deadline semantics.
//
// WithContext panics if c or native is nil.
func (c *Cache) WithContext(native context.Context) *Cache {
	if c == nil {
		panic("tasks: nil Cache")
	}
	if native == nil {
		panic("tasks: nil context")
	}
	return &Cache{
		store: c.store,
		ctx:   native,
	}
}

// RunParallel executes all provided prepared tasks concurrently, returning
// the first non-nil error (if any). All tasks share this Cache.
//
// Nil Prepared values are ignored.
func (c *Cache) RunParallel(tasks ...Prepared) error {
	return run_tasks(c, tasks...)
}

// Prepared is a Task with pre-bound input, ready for RunParallel.
type Prepared interface {
	Run(ctx *Cache) error
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE
/////////////////////////////////////////////////////////////////////

var global_task_id atomic.Uint64

type task_key struct {
	task_id uint64
	input   any
}

type cache_entry struct {
	result     *task_result
	expires_at time.Time
}

type cache_store struct {
	mu           sync.RWMutex
	results      map[task_key]cache_entry
	last_cleanup atomic.Int64 // unix nanos
}

type task_result struct {
	data any
	err  error
	mu   sync.Mutex
	wait chan struct{}
	run  bool
	done bool
}

/////// Core Execution

func run_task[I comparable, O any](
	c *Cache,
	task *Task[I, O],
	input I,
) (result O, err error) {
	if c == nil {
		return result, errors.New("tasks: nil Cache")
	}
	if task == nil || task.fn == nil {
		return result, errors.New("tasks: invalid task")
	}
	if err := c.ctx.Err(); err != nil {
		return result, err
	}

	cache_completed := task.cache_completed
	expiration := task.expiration
	finish_on_completion := !cache_completed || expiration > 0
	key := task_key{task_id: task.id, input: input}
	r := c.get_or_create_no_ttl(key)
	if expiration > 0 {
		r = c.get_or_create_with_expiration(key, expiration)
	}
	for {
		if err := c.ctx.Err(); err != nil {
			return result, err
		}

		r.mu.Lock()
		if r.done {
			data := r.data
			err := r.err
			r.mu.Unlock()

			if err != nil {
				return result, err
			}
			if data == nil {
				return result, nil
			}
			if typed, ok := data.(O); ok {
				return typed, nil
			}
			return result, nil
		}

		if !r.run {
			r.run = true
			r.mu.Unlock()

			val, task_err := task.fn(c, input)

			r.mu.Lock()
			wait := r.wait
			r.wait = nil
			r.run = false
			switch {
			case task_err != nil &&
				!errors.Is(task_err, context.Canceled) &&
				!errors.Is(task_err, context.DeadlineExceeded):
				r.err = task_err
				r.done = true
				r.mu.Unlock()
				if finish_on_completion {
					c.finish_cache_entry(key, r, cache_completed, expiration)
				}
				if wait != nil {
					close(wait)
				}
				continue

			case task_err != nil:
				r.mu.Unlock()
				if wait != nil {
					close(wait)
				}
				return result, task_err

			case c.ctx.Err() != nil:
				cancel_err := c.ctx.Err()
				r.mu.Unlock()
				if wait != nil {
					close(wait)
				}
				return result, cancel_err

			default:
				r.data = val
				r.done = true
				r.mu.Unlock()
				if finish_on_completion {
					c.finish_cache_entry(key, r, cache_completed, expiration)
				}
				if wait != nil {
					close(wait)
				}
				continue
			}
		}

		if r.wait == nil {
			r.wait = make(chan struct{})
		}
		wait := r.wait
		r.mu.Unlock()

		select {
		case <-wait:
			continue
		case <-c.ctx.Done():
			return result, c.ctx.Err()
		}
	}
}

/////// Cache Access

func (c *Cache) get_or_create_no_ttl(key task_key) *task_result {
	c.store.mu.RLock()
	if entry, ok := c.store.results[key]; ok {
		c.store.mu.RUnlock()
		return entry.result
	}
	c.store.mu.RUnlock()

	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if entry, ok := c.store.results[key]; ok {
		return entry.result
	}
	r := &task_result{}
	c.store.results[key] = cache_entry{result: r}
	return r
}

func (c *Cache) get_or_create_with_expiration(
	key task_key,
	expiration time.Duration,
) *task_result {
	now := time.Now()
	now_nanos := now.UnixNano()

	last_cleanup := c.store.last_cleanup.Load()
	if last_cleanup == 0 {
		if c.store.last_cleanup.CompareAndSwap(0, now_nanos) {
			last_cleanup = now_nanos
		} else {
			last_cleanup = c.store.last_cleanup.Load()
		}
	}

	if now_nanos-last_cleanup >= int64(expiration) {
		c.cleanup_expired(now, now_nanos, expiration)
	}

	c.store.mu.RLock()
	if entry, ok := c.store.results[key]; ok {
		if entry.expires_at.IsZero() || now.Before(entry.expires_at) {
			c.store.mu.RUnlock()
			return entry.result
		}
	}
	c.store.mu.RUnlock()

	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if entry, ok := c.store.results[key]; ok {
		if entry.expires_at.IsZero() || now.Before(entry.expires_at) {
			return entry.result
		}
	}
	r := &task_result{}
	c.store.results[key] = cache_entry{result: r}
	return r
}

func (c *Cache) cleanup_expired(
	now time.Time,
	now_nanos int64,
	expiration time.Duration,
) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	if now_nanos-c.store.last_cleanup.Load() < int64(expiration) {
		return
	}
	for k, entry := range c.store.results {
		if !entry.expires_at.IsZero() && now.After(entry.expires_at) {
			delete(c.store.results, k)
		}
	}
	c.store.last_cleanup.Store(now_nanos)
}

func (c *Cache) finish_cache_entry(
	key task_key,
	result *task_result,
	cache_completed bool,
	expiration time.Duration,
) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	entry, ok := c.store.results[key]
	if !ok || entry.result != result {
		return
	}
	if !cache_completed {
		delete(c.store.results, key)
		return
	}
	if expiration > 0 {
		entry.expires_at = time.Now().Add(expiration)
	}
	c.store.results[key] = entry
}

/////// Prepared Tasks

type prepared_task[O any] struct {
	runner func(ctx *Cache) (O, error)
	dest   *O
}

func prepare_task[I comparable, O any](
	task *Task[I, O],
	input I,
	dest *O,
) Prepared {
	if task == nil || task.fn == nil {
		return &prepared_task[O]{
			runner: func(_ *Cache) (O, error) {
				var zero O
				return zero, errors.New(
					"tasks: BindInput called with nil or invalid task",
				)
			},
			dest: dest,
		}
	}
	return &prepared_task[O]{
		runner: func(ctx *Cache) (O, error) {
			return run_task(ctx, task, input)
		},
		dest: dest,
	}
}

func (bt *prepared_task[O]) Run(ctx *Cache) error {
	if ctx == nil {
		return errors.New("tasks: Run called with nil Cache")
	}
	if bt.runner == nil {
		return errors.New("tasks: runner is nil")
	}
	res, err := bt.runner(ctx)
	if err != nil {
		return err
	}
	if bt.dest != nil {
		*bt.dest = res
	}
	return nil
}

/////// Parallel Execution

func run_tasks(ctx *Cache, calls ...Prepared) error {
	if ctx == nil {
		return errors.New("tasks: RunParallel called with nil Cache")
	}
	if err := ctx.ctx.Err(); err != nil {
		return err
	}
	valid := calls[:0]
	for _, c := range calls {
		if c != nil {
			valid = append(valid, c)
		}
	}
	switch len(valid) {
	case 0:
		return nil
	case 1:
		return valid[0].Run(ctx)
	}
	g, g_ctx := errgroup.WithContext(ctx.ctx)
	shared := ctx.WithContext(g_ctx)
	for _, call := range valid {
		c := call
		g.Go(func() error {
			if err := c.Run(shared); err != nil {
				return err
			}
			return shared.ctx.Err()
		})
	}
	return g.Wait()
}
