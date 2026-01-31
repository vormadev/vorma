// A "Task", as used in this package, is simply a function that takes in input,
// returns data (or an error), and runs a maximum of one time per execution
// context / input value pairing, even if invoked repeatedly during the lifetime
// of the execution context.
//
// Tasks are automatically protected from circular deps by Go's compile-time
// "initialization cycle" errors (assuming they are defined as package-level
// variables).
package tasks

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vormadev/vorma/kit/genericsutil"
	"golang.org/x/sync/errgroup"
)

type AnyTask interface {
	RunWithAnyInput(ctx *Ctx, input any) (any, error)
}

type Task[I comparable, O any] struct {
	fn func(ctx *Ctx, input I) (O, error)
}

func NewTask[I comparable, O any](fn func(ctx *Ctx, input I) (O, error)) *Task[I, O] {
	if fn == nil {
		return nil
	}
	return &Task[I, O]{fn: fn}
}

func (t *Task[I, O]) RunWithAnyInput(ctx *Ctx, input any) (any, error) {
	return runTask(ctx, t, genericsutil.AssertOrZero[I](input))
}

func (t *Task[I, O]) Run(ctx *Ctx, input I) (O, error) {
	return runTask(ctx, t, input)
}

func (t *Task[I, O]) Bind(input I, dest *O) BoundTask {
	return bindTask(t, input, dest)
}

// taskKey is used for map lookups to avoid allocating anonymous structs
type taskKey struct {
	taskPtr uintptr
	input   any
}

type Ctx struct {
	mu          *sync.RWMutex
	results     map[taskKey]*cacheEntry
	ctx         context.Context
	ttl         time.Duration
	lastCleanup *atomic.Int64 // Unix timestamp in nanoseconds (nil when TTL disabled)
}

type cacheEntry struct {
	result    *TaskResult
	expiresAt time.Time
}

// NewCtx creates a new task execution context with no TTL.
// The context will cache task results indefinitely until the Ctx is discarded.
func NewCtx(parent context.Context) *Ctx {
	return NewCtxWithTTL(parent, 0)
}

// NewCtxWithTTL creates a new task execution context with a TTL for cached results.
// When ttl > 0, cached results expire after the specified duration and will be
// re-executed on subsequent access. Expired entries are lazily removed from memory
// during cache access, at most once per TTL period.
func NewCtxWithTTL(parent context.Context, ttl time.Duration) *Ctx {
	if parent == nil {
		parent = context.Background()
	}

	c := &Ctx{
		mu:      &sync.RWMutex{},
		results: make(map[taskKey]*cacheEntry, 4),
		ctx:     parent,
		ttl:     ttl,
	}

	// Only initialize lastCleanup if TTL is enabled
	if ttl > 0 {
		c.lastCleanup = &atomic.Int64{}
		c.lastCleanup.Store(time.Now().UnixNano())
	}

	return c
}

func (c *Ctx) NativeContext() context.Context {
	return c.ctx
}

func (c *Ctx) RunParallel(tasks ...BoundTask) error {
	return runTasks(c, tasks...)
}

func runTask[I comparable, O any](c *Ctx, task *Task[I, O], input I) (result O, err error) {
	if c == nil {
		return result, errors.New("tasks: nil TasksCtx")
	}
	if task == nil || task.fn == nil {
		return result, errors.New("tasks: invalid task")
	}

	// Check context only once at the beginning
	if err := c.ctx.Err(); err != nil {
		return result, err
	}

	r := c.getOrCreateResult(task, input)
	r.once.Do(func() {
		val, err := task.fn(c, input)
		if err != nil {
			r.Err = err
			return
		}
		if cerr := c.ctx.Err(); cerr != nil {
			r.Err = cerr
			return
		}
		r.Data = val
		r.Err = nil
	})

	if r.Err != nil {
		return result, r.Err
	}
	if r.Data == nil {
		return result, nil
	}
	return genericsutil.AssertOrZero[O](r.Data), nil
}

func (c *Ctx) getOrCreateResult(taskPtr any, input any) *TaskResult {
	// Use uintptr for task pointer to avoid allocation
	key := taskKey{
		taskPtr: reflect.ValueOf(taskPtr).Pointer(),
		input:   input,
	}

	// Only do time operations if TTL is enabled
	if c.ttl > 0 {
		now := time.Now()

		// Lazy cleanup: remove expired entries at most once per TTL period
		lastCleanupNano := c.lastCleanup.Load()
		lastCleanupTime := time.Unix(0, lastCleanupNano)
		if now.Sub(lastCleanupTime) >= c.ttl {
			c.cleanupExpired(now)
		}
	}

	// Fast path: check if valid cached result exists
	c.mu.RLock()
	if entry, ok := c.results[key]; ok {
		// Check if entry is still valid (not expired)
		if c.ttl == 0 || time.Now().Before(entry.expiresAt) {
			c.mu.RUnlock()
			return entry.result
		}
		// Entry expired, fall through to recreate
	}
	c.mu.RUnlock()

	// Slow path: need to create new result
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Double-check after acquiring write lock
	if entry, ok := c.results[key]; ok {
		// Check again if still valid (another goroutine may have refreshed it)
		if c.ttl == 0 || now.Before(entry.expiresAt) {
			return entry.result
		}
		// Still expired, will overwrite below
	}

	// Create new result and cache entry
	r := newTaskResult()
	c.results[key] = &cacheEntry{result: r}
	if c.ttl > 0 {
		c.results[key].expiresAt = now.Add(c.ttl)
	}
	return r
}

// cleanupExpired removes all expired entries from the cache.
// This is called lazily during getOrCreateResult, at most once per TTL period.
func (c *Ctx) cleanupExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	lastCleanupNano := c.lastCleanup.Load()
	lastCleanupTime := time.Unix(0, lastCleanupNano)
	if now.Sub(lastCleanupTime) < c.ttl {
		return
	}

	// Remove all expired entries
	for key, entry := range c.results {
		if now.After(entry.expiresAt) {
			delete(c.results, key)
		}
	}

	c.lastCleanup.Store(now.UnixNano())
}

type TaskResult struct {
	Data any
	Err  error
	once *sync.Once
}

func newTaskResult() *TaskResult {
	return &TaskResult{once: &sync.Once{}}
}

func (r *TaskResult) OK() bool {
	return r.Err == nil
}

type BoundTask interface {
	Run(ctx *Ctx) error
}

type boundTask[O any] struct {
	runner func(ctx *Ctx) (O, error)
	dest   *O
}

func bindTask[I comparable, O any](task *Task[I, O], input I, dest *O) BoundTask {
	if task == nil || task.fn == nil {
		return &boundTask[O]{
			runner: func(ctx *Ctx) (O, error) {
				var zero O
				return zero, errors.New("tasks: bindTask called with a nil or invalid task")
			},
			dest: dest,
		}
	}
	return &boundTask[O]{
		runner: func(ctx *Ctx) (O, error) {
			return runTask(ctx, task, input)
		},
		dest: dest,
	}
}

func (bc *boundTask[O]) Run(ctx *Ctx) error {
	if ctx == nil {
		return errors.New("tasks: boundTask.Run called with nil TasksCtx")
	}
	if bc.runner == nil {
		return errors.New("tasks: boundTask runner is nil (task may have been invalid at Bind)")
	}
	res, err := bc.runner(ctx)
	if err != nil {
		return err
	}
	if bc.dest != nil {
		*bc.dest = res
	}
	return nil
}

func runTasks(ctx *Ctx, calls ...BoundTask) error {
	if ctx == nil {
		return errors.New("tasks: runTasks called with nil TasksCtx")
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
	g, gCtx := errgroup.WithContext(ctx.ctx)
	shared := &Ctx{
		mu:          ctx.mu,
		results:     ctx.results,
		ctx:         gCtx,
		ttl:         ctx.ttl,
		lastCleanup: ctx.lastCleanup,
	}
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
