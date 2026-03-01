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
	"fmt"
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
	id uint64
	fn func(ctx *Ctx, input I) (O, error)
}

var globalTaskIDCounter atomic.Uint64

func NewTask[I comparable, O any](fn func(ctx *Ctx, input I) (O, error)) *Task[I, O] {
	if fn == nil {
		return nil
	}
	return &Task[I, O]{
		id: globalTaskIDCounter.Add(1),
		fn: fn,
	}
}

func (t *Task[I, O]) RunWithAnyInput(ctx *Ctx, input any) (any, error) {
	typedInput, ok := input.(I)
	if !ok {
		return nil, fmt.Errorf(
			"tasks: input type mismatch: expected %s, got %T",
			reflect.TypeOf((*I)(nil)).Elem(),
			input,
		)
	}
	return runTask(ctx, t, typedInput)
}

func (t *Task[I, O]) Run(ctx *Ctx, input I) (O, error) {
	return runTask(ctx, t, input)
}

func (t *Task[I, O]) Bind(input I, dest *O) BoundTask {
	return bindTask(t, input, dest)
}

// taskKey is used for map lookups to avoid allocating anonymous structs
type taskKey struct {
	taskID uint64
	input  any
}

type Ctx struct {
	mu          *sync.RWMutex
	results     map[taskKey]cacheEntry
	ctx         context.Context
	ttl         time.Duration
	lastCleanup *atomic.Int64 // Unix timestamp in nanoseconds (nil when TTL disabled)
	// true only for contexts leased from sharedStateChildCtxPool.
	isBorrowedSharedStateChildContext bool
}

var sharedStateChildCtxPool = sync.Pool{
	New: func() any {
		return &Ctx{}
	},
}

type cacheEntry struct {
	result    *taskResult
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
	if ttl < 0 {
		ttl = 0
	}

	c := &Ctx{
		mu:      &sync.RWMutex{},
		results: make(map[taskKey]cacheEntry, 4),
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

// WithNativeContext returns a child Ctx that shares the same task cache as c
// but uses the provided native context for cancellation/deadline semantics.
func (c *Ctx) WithNativeContext(native context.Context) *Ctx {
	if c == nil {
		return NewCtx(native)
	}
	if native == nil {
		native = context.Background()
	}
	return &Ctx{
		mu:          c.mu,
		results:     c.results,
		ctx:         native,
		ttl:         c.ttl,
		lastCleanup: c.lastCleanup,
	}
}

// AcquireSharedStateChildContextWithNativeContext returns a child Ctx that
// shares task cache state with c and uses native for cancellation/deadline
// behavior. The returned context must be returned via
// ReleaseSharedStateChildContext once the caller is done with it.
func (c *Ctx) AcquireSharedStateChildContextWithNativeContext(
	native context.Context,
) *Ctx {
	if c == nil {
		return NewCtx(native)
	}
	if native == nil {
		native = context.Background()
	}
	child := sharedStateChildCtxPool.Get().(*Ctx)
	child.mu = c.mu
	child.results = c.results
	child.ctx = native
	child.ttl = c.ttl
	child.lastCleanup = c.lastCleanup
	child.isBorrowedSharedStateChildContext = true
	return child
}

// ReleaseSharedStateChildContext returns a context leased via
// AcquireSharedStateChildContextWithNativeContext back to the internal pool.
func ReleaseSharedStateChildContext(child *Ctx) {
	if child == nil || !child.isBorrowedSharedStateChildContext {
		return
	}
	child.mu = nil
	child.results = nil
	child.ctx = nil
	child.ttl = 0
	child.lastCleanup = nil
	child.isBorrowedSharedStateChildContext = false
	sharedStateChildCtxPool.Put(child)
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

	r := c.getOrCreateResult(
		task.id,
		input,
	)
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
		r.err = nil
	})

	if r.err != nil {
		return result, r.err
	}
	if r.data == nil {
		return result, nil
	}
	return genericsutil.AssertOrZero[O](r.data), nil
}

func (c *Ctx) getOrCreateResult(taskID uint64, input any) *taskResult {
	key := taskKey{
		taskID: taskID,
		input:  input,
	}

	if c.ttl == 0 {
		return c.getOrCreateResultWithoutTTL(key)
	}
	return c.getOrCreateResultWithTTL(key)
}

func (c *Ctx) getOrCreateResultWithoutTTL(key taskKey) *taskResult {
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

	r := newTaskResult()
	c.results[key] = cacheEntry{result: r}
	return r
}

func (c *Ctx) getOrCreateResultWithTTL(key taskKey) *taskResult {
	now := time.Now()
	nowUnixNano := now.UnixNano()

	// Lazy cleanup: remove expired entries at most once per TTL period.
	if nowUnixNano-c.lastCleanup.Load() >= int64(c.ttl) {
		c.cleanupExpired(now, nowUnixNano)
	}

	c.mu.RLock()
	if entry, ok := c.results[key]; ok {
		if now.Before(entry.expiresAt) {
			c.mu.RUnlock()
			return entry.result
		}
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.results[key]; ok {
		if now.Before(entry.expiresAt) {
			return entry.result
		}
	}

	r := newTaskResult()
	c.results[key] = cacheEntry{
		result:    r,
		expiresAt: now.Add(c.ttl),
	}
	return r
}

// cleanupExpired removes all expired entries from the cache.
// This is called lazily during getOrCreateResult, at most once per TTL period.
func (c *Ctx) cleanupExpired(
	now time.Time,
	nowUnixNano int64,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if nowUnixNano-c.lastCleanup.Load() < int64(c.ttl) {
		return
	}

	// Remove all expired entries
	for key, entry := range c.results {
		if now.After(entry.expiresAt) {
			delete(c.results, key)
		}
	}

	c.lastCleanup.Store(nowUnixNano)
}

type taskResult struct {
	data any
	err  error
	once sync.Once
}

func newTaskResult() *taskResult {
	return &taskResult{}
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
