// Package tasks provides memoized, concurrency-safe task execution.
//
// A Task is a function that takes input, returns data (or an error), and
// runs at most once per Ctx/input pairing, even if invoked repeatedly.
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
	RunWithAnyInput(ctx *Ctx, input any) (any, error)
}

// Task is a typed, memoized unit of work.
type Task[I comparable, O any] struct {
	id uint64
	fn func(ctx *Ctx, input I) (O, error)
}

// NewTask creates a Task from the provided function. Returns nil if fn is nil.
func NewTask[I comparable, O any](
	fn func(ctx *Ctx, input I) (O, error),
) *Task[I, O] {
	if fn == nil {
		return nil
	}
	return &Task[I, O]{
		id: global_task_id.Add(1),
		fn: fn,
	}
}

// RunWithAnyInput executes the task with a type-erased input.
func (t *Task[I, O]) RunWithAnyInput(ctx *Ctx, input any) (any, error) {
	typed, ok := input.(I)
	if !ok {
		return nil, fmt.Errorf(
			"tasks: input type mismatch: expected %s, got %T",
			reflect.TypeOf((*I)(nil)).Elem(), input,
		)
	}
	return run_task(ctx, t, typed)
}

// Run executes the task with a typed input.
func (t *Task[I, O]) Run(ctx *Ctx, input I) (O, error) {
	return run_task(ctx, t, input)
}

// Bind creates a BoundTask that captures the input and an optional
// destination pointer for the result.
func (t *Task[I, O]) Bind(input I, dest *O) BoundTask {
	return bind_task(t, input, dest)
}

// Ctx is a task execution context that memoizes results by task/input pair.
type Ctx struct {
	mu           *sync.RWMutex
	results      map[task_key]cache_entry
	ctx          context.Context
	ttl          time.Duration
	last_cleanup *atomic.Int64 // unix nanos; nil when TTL disabled
}

// NewCtx creates a Ctx with no TTL (results cached indefinitely).
func NewCtx(parent context.Context) *Ctx {
	return NewCtxWithTTL(parent, 0)
}

// NewCtxWithTTL creates a Ctx whose cached results expire after ttl.
// Expired entries are lazily cleaned up during cache access.
func NewCtxWithTTL(parent context.Context, ttl time.Duration) *Ctx {
	if parent == nil {
		parent = context.Background()
	}
	if ttl < 0 {
		ttl = 0
	}
	c := &Ctx{
		mu:      &sync.RWMutex{},
		results: make(map[task_key]cache_entry, 4),
		ctx:     parent,
		ttl:     ttl,
	}
	if ttl > 0 {
		c.last_cleanup = &atomic.Int64{}
		c.last_cleanup.Store(time.Now().UnixNano())
	}
	return c
}

// NativeContext returns the underlying context.Context.
func (c *Ctx) NativeContext() context.Context {
	return c.ctx
}

// WithNativeContext returns a child Ctx that shares the same task cache
// but uses the provided context for cancellation/deadline semantics.
func (c *Ctx) WithNativeContext(native context.Context) *Ctx {
	if c == nil {
		return NewCtx(native)
	}
	if native == nil {
		native = context.Background()
	}
	return &Ctx{
		mu:           c.mu,
		results:      c.results,
		ctx:          native,
		ttl:          c.ttl,
		last_cleanup: c.last_cleanup,
	}
}

// RunParallel executes all provided BoundTasks concurrently, returning
// the first non-nil error (if any). All tasks share this Ctx's cache.
func (c *Ctx) RunParallel(tasks ...BoundTask) error {
	return run_tasks(c, tasks...)
}

// BoundTask is a task with pre-bound input, ready for execution.
type BoundTask interface {
	Run(ctx *Ctx) error
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

type task_result struct {
	data any
	err  error
	once sync.Once
}

// --- core execution ---

func run_task[I comparable, O any](
	c *Ctx,
	task *Task[I, O],
	input I,
) (result O, err error) {
	if c == nil {
		return result, errors.New("tasks: nil Ctx")
	}
	if task == nil || task.fn == nil {
		return result, errors.New("tasks: invalid task")
	}
	if err := c.ctx.Err(); err != nil {
		return result, err
	}

	r := c.get_or_create_result(task.id, input)
	r.once.Do(func() {
		val, err := task.fn(c, input)
		if err != nil {
			r.err = err
			return
		}
		if cerr := c.ctx.Err(); cerr != nil {
			r.err = cerr
			return
		}
		r.data = val
	})

	if r.err != nil {
		return result, r.err
	}
	if r.data == nil {
		return result, nil
	}
	if typed, ok := r.data.(O); ok {
		return typed, nil
	}
	return result, nil
}

// --- cache access ---

func (c *Ctx) get_or_create_result(task_id uint64, input any) *task_result {
	key := task_key{task_id: task_id, input: input}
	if c.ttl == 0 {
		return c.get_or_create_no_ttl(key)
	}
	return c.get_or_create_with_ttl(key)
}

func (c *Ctx) get_or_create_no_ttl(key task_key) *task_result {
	c.mu.RLock()
	if entry, ok := c.results[key]; ok {
		c.mu.RUnlock()
		return entry.result
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.results[key]; ok {
		return entry.result
	}
	r := &task_result{}
	c.results[key] = cache_entry{result: r}
	return r
}

func (c *Ctx) get_or_create_with_ttl(key task_key) *task_result {
	now := time.Now()
	now_nanos := now.UnixNano()

	if now_nanos-c.last_cleanup.Load() >= int64(c.ttl) {
		c.cleanup_expired(now, now_nanos)
	}

	c.mu.RLock()
	if entry, ok := c.results[key]; ok && now.Before(entry.expires_at) {
		c.mu.RUnlock()
		return entry.result
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.results[key]; ok && now.Before(entry.expires_at) {
		return entry.result
	}
	r := &task_result{}
	c.results[key] = cache_entry{result: r, expires_at: now.Add(c.ttl)}
	return r
}

func (c *Ctx) cleanup_expired(now time.Time, now_nanos int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now_nanos-c.last_cleanup.Load() < int64(c.ttl) {
		return
	}
	for k, entry := range c.results {
		if now.After(entry.expires_at) {
			delete(c.results, k)
		}
	}
	c.last_cleanup.Store(now_nanos)
}

// --- bound tasks ---

type bound_task[O any] struct {
	runner func(ctx *Ctx) (O, error)
	dest   *O
}

func bind_task[I comparable, O any](
	task *Task[I, O],
	input I,
	dest *O,
) BoundTask {
	if task == nil || task.fn == nil {
		return &bound_task[O]{
			runner: func(_ *Ctx) (O, error) {
				var zero O
				return zero, errors.New(
					"tasks: Bind called with nil or invalid task",
				)
			},
			dest: dest,
		}
	}
	return &bound_task[O]{
		runner: func(ctx *Ctx) (O, error) {
			return run_task(ctx, task, input)
		},
		dest: dest,
	}
}

func (bt *bound_task[O]) Run(ctx *Ctx) error {
	if ctx == nil {
		return errors.New("tasks: Run called with nil Ctx")
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

// --- parallel execution ---

func run_tasks(ctx *Ctx, calls ...BoundTask) error {
	if ctx == nil {
		return errors.New("tasks: RunParallel called with nil Ctx")
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
	shared := ctx.WithNativeContext(g_ctx)
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
