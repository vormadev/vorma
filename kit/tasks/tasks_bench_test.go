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
	task := NewTask(func(c *Cache, input int) (int, error) {
		return input * 2, nil
	})

	for i := 0; b.Loop(); i++ {
		ctx := NewCache(context.Background())
		_, err := task.Run(ctx, i)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelIndependentTasks(b *testing.B) {
	task_1 := NewTask(
		func(c *Cache, input int) (int, error) { return input * 2, nil },
	)
	task_2 := NewTask(
		func(c *Cache, input int) (int, error) { return input * 3, nil },
	)
	task_3 := NewTask(
		func(c *Cache, input int) (int, error) { return input * 4, nil },
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCache(context.Background())
		var r1, r2, r3 int
		_ = ctx.RunParallel(
			task_1.BindInput(i, &r1),
			task_2.BindInput(i, &r2),
			task_3.BindInput(i, &r3),
		)
	}
}

func BenchmarkHighContention(b *testing.B) {
	shared := NewTask(func(c *Cache, _ struct{}) (string, error) {
		time.Sleep(1 * time.Microsecond)
		return "result", nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCache(context.Background())
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				_, err := shared.Run(ctx, struct{}{})
				if err != nil {
					b.Error(err)
				}
			})
		}
		wg.Wait()
	}
}

func BenchmarkTaskWithDependencies(b *testing.B) {
	base := NewTask(func(c *Cache, input int) (int, error) {
		return input * 2, nil
	})
	dependent := NewTask(func(c *Cache, input int) (int, error) {
		val, err := base.Run(c, input)
		if err != nil {
			return 0, err
		}
		return val + 10, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := NewCache(context.Background())
		_, err := dependent.Run(ctx, i)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocations(b *testing.B) {
	task := NewTask(func(c *Cache, input string) (string, error) {
		return "Hello, " + input, nil
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := NewCache(context.Background())
		_, err := task.Run(ctx, "World")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelScaling(b *testing.B) {
	for _, num := range []int{1, 2, 5, 10, 20, 50} {
		b.Run(fmt.Sprintf("tasks-%d", num), func(b *testing.B) {
			task_list := make([]*Task[int, int], num)
			for i := range num {
				id := i
				task_list[i] = NewTask(func(c *Cache, input int) (int, error) {
					return input + id, nil
				})
			}

			bound := make([]Prepared, num)
			results := make([]int, num)

			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				ctx := NewCache(context.Background())
				for j := range num {
					bound[j] = task_list[j].BindInput(i, &results[j])
				}
				_ = ctx.RunParallel(bound...)
			}
		})
	}
}

func BenchmarkContextCancellation(b *testing.B) {
	task := NewTask(func(c *Cache, input int) (int, error) {
		time.Sleep(10 * time.Millisecond)
		return input * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parent, cancel := context.WithCancel(context.Background())
		ctx := NewCache(parent)
		time.AfterFunc(1*time.Microsecond, cancel)
		_, _ = task.Run(ctx, i)
	}
}

func BenchmarkRepeatedTaskCalls(b *testing.B) {
	var counter int64
	task := NewTask(func(c *Cache, input int) (int, error) {
		atomic.AddInt64(&counter, 1)
		return input * 2, nil
	})

	ctx := NewCache(context.Background())
	if _, err := task.Run(ctx, 42); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := task.Run(ctx, 42)
		if err != nil {
			b.Fatal(err)
		}
	}

	if atomic.LoadInt64(&counter) != 1 {
		b.Fatalf("Expected func to run once, ran %d times", counter)
	}
}
