package tasks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkSingleTask(b *testing.B) {
	task := NewTask(func(c *Ctx, input int) (int, error) {
		return input * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCtx(context.Background())
		_, err := run_task(ctx, task, i)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelIndependentTasks(b *testing.B) {
	task_1 := NewTask(
		func(c *Ctx, input int) (int, error) { return input * 2, nil },
	)
	task_2 := NewTask(
		func(c *Ctx, input int) (int, error) { return input * 3, nil },
	)
	task_3 := NewTask(
		func(c *Ctx, input int) (int, error) { return input * 4, nil },
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCtx(context.Background())
		var r1, r2, r3 int
		_ = run_tasks(ctx,
			task_1.Bind(i, &r1),
			task_2.Bind(i, &r2),
			task_3.Bind(i, &r3),
		)
	}
}

func BenchmarkHighContention(b *testing.B) {
	shared := NewTask(func(c *Ctx, _ struct{}) (string, error) {
		time.Sleep(1 * time.Microsecond)
		return "result", nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCtx(context.Background())
		var wg sync.WaitGroup
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := run_task(ctx, shared, struct{}{})
				if err != nil {
					b.Error(err)
				}
			}()
		}
		wg.Wait()
	}
}

func BenchmarkTaskWithDependencies(b *testing.B) {
	base := NewTask(func(c *Ctx, input int) (int, error) {
		return input * 2, nil
	})
	dependent := NewTask(func(c *Ctx, input int) (int, error) {
		val, err := run_task(c, base, input)
		if err != nil {
			return 0, err
		}
		return val + 10, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCtx(context.Background())
		_, err := run_task(ctx, dependent, i)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocations(b *testing.B) {
	task := NewTask(func(c *Ctx, input string) (string, error) {
		return "Hello, " + input, nil
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := NewCtx(context.Background())
		_, err := run_task(ctx, task, "World")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelScaling(b *testing.B) {
	for _, num := range []int{1, 2, 5, 10, 20, 50} {
		b.Run(fmt.Sprintf("tasks-%d", num), func(b *testing.B) {
			task_list := make([]*Task[int, int], num)
			for i := 0; i < num; i++ {
				id := i
				task_list[i] = NewTask(func(c *Ctx, input int) (int, error) {
					return input + id, nil
				})
			}

			bound := make([]BoundTask, num)
			results := make([]int, num)

			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				ctx := NewCtx(context.Background())
				for j := range num {
					bound[j] = task_list[j].Bind(i, &results[j])
				}
				_ = run_tasks(ctx, bound...)
			}
		})
	}
}

func BenchmarkContextCancellation(b *testing.B) {
	task := NewTask(func(c *Ctx, input int) (int, error) {
		time.Sleep(10 * time.Millisecond)
		return input * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parent, cancel := context.WithCancel(context.Background())
		ctx := NewCtx(parent)
		time.AfterFunc(1*time.Microsecond, cancel)
		_, _ = run_task(ctx, task, i)
	}
}

func BenchmarkRepeatedTaskCalls(b *testing.B) {
	var counter int64
	task := NewTask(func(c *Ctx, input int) (int, error) {
		atomic.AddInt64(&counter, 1)
		return input * 2, nil
	})

	ctx := NewCtx(context.Background())
	if _, err := run_task(ctx, task, 42); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := run_task(ctx, task, 42)
		if err != nil {
			b.Fatal(err)
		}
	}

	if atomic.LoadInt64(&counter) != 1 {
		b.Fatalf("Expected task to run once, ran %d times", counter)
	}
}
