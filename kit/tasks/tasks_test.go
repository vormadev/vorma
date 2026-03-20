package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTasks(t *testing.T) {
	t.Run("BasicTaskExecution", func(t *testing.T) {
		task := NewTask(func(c *Ctx, input string) (string, error) {
			return "Hello, " + input, nil
		})

		ctx := NewCtx(context.Background())
		result, err := task.Run(ctx, "World")

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "Hello, World" {
			t.Errorf("Expected 'Hello, World', got '%s'", result)
		}
	})

	t.Run("ParallelExecution", func(t *testing.T) {
		task_1 := NewTask(func(c *Ctx, input int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return input * 2, nil
		})
		task_2 := NewTask(func(c *Ctx, input string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return input + "3", nil
		})

		ctx := NewCtx(context.Background())
		start := time.Now()

		var r1 int
		var r2 string
		err := ctx.RunParallel(
			task_1.Bind(5, &r1),
			task_2.Bind("3", &r2),
		)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected no errors, got %v", err)
		}
		if r1 != 10 || r2 != "33" {
			t.Errorf("Expected 10 and '33', got %d and %s", r1, r2)
		}
		if duration > 150*time.Millisecond {
			t.Errorf("Expected parallel execution (<150ms), took %v", duration)
		}
	})

	t.Run("TaskDependencies", func(t *testing.T) {
		auth_task := NewTask(func(c *Ctx, input string) (string, error) {
			return "token-" + input, nil
		})
		user_task := NewTask(func(c *Ctx, input string) (string, error) {
			token, err := auth_task.Run(c, input)
			if err != nil {
				return "", err
			}
			return "user-" + token, nil
		})

		ctx := NewCtx(context.Background())
		result, err := user_task.Run(ctx, "123")

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "user-token-123" {
			t.Errorf("Expected 'user-token-123', got '%s'", result)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		task := NewTask(func(c *Ctx, _ string) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return "done", nil
		})

		parent, cancel := context.WithCancel(context.Background())
		ctx := NewCtx(parent)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := run_task(ctx, task, "test")
		if err == nil {
			t.Error("Expected context cancellation error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		task := NewTask(func(c *Ctx, _ string) (string, error) {
			return "", errors.New("task failed")
		})

		ctx := NewCtx(context.Background())
		result, err := run_task(ctx, task, "test")

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "task failed" {
			t.Errorf("Expected 'task failed' error, got '%v'", err)
		}
		if result != "" {
			t.Errorf("Expected empty string, got '%s'", result)
		}
	})

	t.Run("OnceExecution", func(t *testing.T) {
		var counter int32
		task := NewTask(func(c *Ctx, _ string) (string, error) {
			atomic.AddInt32(&counter, 1)
			time.Sleep(50 * time.Millisecond)
			return "done", nil
		})

		ctx := NewCtx(context.Background())
		var wg sync.WaitGroup
		wg.Add(3)

		for range 3 {
			go func() {
				defer wg.Done()
				_, err := run_task(ctx, task, "test")
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()

		if counter != 1 {
			t.Errorf("Expected task to run once, ran %d times", counter)
		}
	})
}

func TestTasksWithSharedDependencies(t *testing.T) {
	t.Run("ParallelTasksWithSharedDependencies", func(t *testing.T) {
		var auth_counter int32
		auth_task := NewTask(func(c *Ctx, _ struct{}) (int, error) {
			atomic.AddInt32(&auth_counter, 1)
			time.Sleep(100 * time.Millisecond)
			return 123, nil
		})

		user_task := NewTask(func(c *Ctx, _ string) (string, error) {
			token, err := run_task(c, auth_task, struct{}{})
			if err != nil {
				return "", err
			}
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("user-%d", token), nil
		})

		user2_task := NewTask(func(c *Ctx, _ string) (string, error) {
			token, err := run_task(c, auth_task, struct{}{})
			if err != nil {
				return "", err
			}
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("user2-%d", token), nil
		})

		ctx := NewCtx(context.Background())
		var user_data, user2_data string
		err := ctx.RunParallel(
			user_task.Bind("test", &user_data),
			user2_task.Bind("test", &user2_data),
		)

		if err != nil {
			t.Fatal("RunParallel failed:", err)
		}
		if user_data != "user-123" {
			t.Errorf("Expected 'user-123', got '%s'", user_data)
		}
		if user2_data != "user2-123" {
			t.Errorf("Expected 'user2-123', got '%s'", user2_data)
		}
		if auth_counter != 1 {
			t.Errorf(
				"Expected authTask to run once, ran %d times",
				auth_counter,
			)
		}
	})
}

func TestCtxWithNativeContext(t *testing.T) {
	t.Run("SharesTaskCacheWithParent", func(t *testing.T) {
		var runs atomic.Int32
		task := NewTask(func(c *Ctx, input string) (string, error) {
			runs.Add(1)
			return "ok-" + input, nil
		})

		parent := NewCtx(context.Background())
		child := parent.WithNativeContext(context.Background())

		got_parent, err := task.Run(parent, "a")
		if err != nil {
			t.Fatalf("parent run error = %v", err)
		}
		if got_parent != "ok-a" {
			t.Fatalf("parent value = %q, want %q", got_parent, "ok-a")
		}

		got_child, err := task.Run(child, "a")
		if err != nil {
			t.Fatalf("child run error = %v", err)
		}
		if got_child != "ok-a" {
			t.Fatalf("child value = %q, want %q", got_child, "ok-a")
		}

		if runs.Load() != 1 {
			t.Fatalf("task runs = %d, want 1 (shared cache)", runs.Load())
		}
	})

	t.Run("UsesProvidedNativeContext", func(t *testing.T) {
		parent := NewCtx(context.Background())
		native, cancel := context.WithCancel(context.Background())
		cancel()

		child := parent.WithNativeContext(native)
		task := NewTask(func(c *Ctx, _ string) (string, error) {
			return "ok", nil
		})

		_, err := task.Run(child, "ignored")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})
}

func TestComprehensiveSharedDependencies(t *testing.T) {
	var execution_order []string
	var execution_mu sync.Mutex
	record := func(name string) {
		execution_mu.Lock()
		execution_order = append(execution_order, name)
		execution_mu.Unlock()
	}

	var auth_counter, user_counter, user2_counter, profile_counter int32
	var user_inputs, user2_inputs []string
	var user_tokens, user2_tokens []int
	var state_mu sync.Mutex

	auth_task := NewTask(func(c *Ctx, _ struct{}) (int, error) {
		record("auth-start")
		atomic.AddInt32(&auth_counter, 1)
		time.Sleep(50 * time.Millisecond)
		record("auth-end")
		return 123, nil
	})

	user_task := NewTask(func(c *Ctx, input string) (string, error) {
		record("user-start")
		atomic.AddInt32(&user_counter, 1)
		if input == "" {
			t.Error("Expected non-empty input in user_task")
		}

		token, err := run_task(c, auth_task, struct{}{})
		if err != nil {
			return "", err
		}

		state_mu.Lock()
		user_inputs = append(user_inputs, input)
		user_tokens = append(user_tokens, token)
		state_mu.Unlock()

		time.Sleep(25 * time.Millisecond)
		record("user-end")
		return fmt.Sprintf("user-%s-%d", input, token), nil
	})

	user2_task := NewTask(func(c *Ctx, input string) (string, error) {
		record("user2-start")
		atomic.AddInt32(&user2_counter, 1)
		if input == "" {
			t.Error("Expected non-empty input in user2_task")
		}

		token, err := run_task(c, auth_task, struct{}{})
		if err != nil {
			return "", err
		}

		state_mu.Lock()
		user2_inputs = append(user2_inputs, input)
		user2_tokens = append(user2_tokens, token)
		state_mu.Unlock()

		time.Sleep(25 * time.Millisecond)
		record("user2-end")
		return fmt.Sprintf("user2-%s-%d", input, token), nil
	})

	profile_task := NewTask(
		func(ctx *Ctx, input string) (map[string]string, error) {
			record("profile-start")
			atomic.AddInt32(&profile_counter, 1)

			var user_data, user2_data string
			err := ctx.RunParallel(
				user_task.Bind(input, &user_data),
				user2_task.Bind(input+"_alt", &user2_data),
			)
			if err != nil {
				return nil, err
			}

			time.Sleep(25 * time.Millisecond)
			record("profile-end")

			return map[string]string{
				"user":   user_data,
				"user2":  user2_data,
				"status": "complete",
			}, nil
		},
	)

	const input_1 = "test_input_1"
	const input_2 = "test_input_2"

	ctx1 := NewCtx(context.Background())
	result_1, err_1 := run_task(ctx1, profile_task, input_1)

	ctx2 := NewCtx(context.Background())
	result_2, err_2 := run_task(ctx2, profile_task, input_2)

	if err_1 != nil {
		t.Errorf("Expected no error from first profile, got %v", err_1)
	}
	if err_2 != nil {
		t.Errorf("Expected no error from second profile, got %v", err_2)
	}

	if result_1["user"] != "user-test_input_1-123" {
		t.Errorf("Expected 'user-test_input_1-123', got '%s'", result_1["user"])
	}
	if result_1["user2"] != "user2-test_input_1_alt-123" {
		t.Errorf(
			"Expected 'user2-test_input_1_alt-123', got '%s'",
			result_1["user2"],
		)
	}
	if result_2["user"] != "user-test_input_2-123" {
		t.Errorf("Expected 'user-test_input_2-123', got '%s'", result_2["user"])
	}
	if result_2["user2"] != "user2-test_input_2_alt-123" {
		t.Errorf(
			"Expected 'user2-test_input_2_alt-123', got '%s'",
			result_2["user2"],
		)
	}

	if atomic.LoadInt32(&auth_counter) != 2 {
		t.Errorf("Expected auth to run twice, ran %d times", auth_counter)
	}
	if atomic.LoadInt32(&user_counter) != 2 {
		t.Errorf("Expected user to run twice, ran %d times", user_counter)
	}
	if atomic.LoadInt32(&user2_counter) != 2 {
		t.Errorf("Expected user2 to run twice, ran %d times", user2_counter)
	}
	if atomic.LoadInt32(&profile_counter) != 2 {
		t.Errorf("Expected profile to run twice, ran %d times", profile_counter)
	}

	verify_order := func(events [2]string, msg string) {
		execution_mu.Lock()
		defer execution_mu.Unlock()
		first_idx := -1
		for i, e := range execution_order {
			if e == events[0] {
				first_idx = i
				break
			}
		}
		if first_idx == -1 {
			t.Errorf(
				"Event not found: %s. Order: %v",
				events[0],
				execution_order,
			)
			return
		}
		for i := first_idx + 1; i < len(execution_order); i++ {
			if execution_order[i] == events[1] {
				return
			}
		}
		t.Errorf(
			"Expected '%s' after '%s'. Order: %v. %s",
			events[1],
			events[0],
			execution_order,
			msg,
		)
	}

	verify_order(
		[2]string{"auth-end", "user-end"},
		"user should finish after auth",
	)
	verify_order(
		[2]string{"auth-end", "user2-end"},
		"user2 should finish after auth",
	)
	verify_order(
		[2]string{"user-end", "profile-end"},
		"profile should finish after user",
	)
	verify_order(
		[2]string{"user2-end", "profile-end"},
		"profile should finish after user2",
	)

	t.Logf("Execution order: %v", execution_order)
}

func TestTasksWithDifferentInputs(t *testing.T) {
	t.Run("Same_Input_Uses_Cache", func(t *testing.T) {
		var exec_count int32
		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&exec_count, 1)
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())
		r1, _ := run_task(ctx, task, "foo")
		r2, _ := run_task(ctx, task, "foo")
		r3, _ := run_task(ctx, task, "foo")

		if r1 != "result-foo" || r2 != "result-foo" || r3 != "result-foo" {
			t.Error("Expected same result for same input")
		}
		if exec_count != 1 {
			t.Errorf("Expected 1 execution, got %d", exec_count)
		}
	})

	t.Run("Different_Inputs_Execute_Separately", func(t *testing.T) {
		var exec_count int32
		var exec_inputs []string
		var mu sync.Mutex

		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&exec_count, 1)
			mu.Lock()
			exec_inputs = append(exec_inputs, input)
			mu.Unlock()
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())
		r1, _ := run_task(ctx, task, "foo")
		r2, _ := run_task(ctx, task, "bar")
		r3, _ := run_task(ctx, task, "baz")
		r1b, _ := run_task(ctx, task, "foo")
		r2b, _ := run_task(ctx, task, "bar")

		if r1 != "result-foo" || r1b != "result-foo" {
			t.Error("Expected consistent results for 'foo'")
		}
		if r2 != "result-bar" || r2b != "result-bar" {
			t.Error("Expected consistent results for 'bar'")
		}
		if r3 != "result-baz" {
			t.Error("Expected correct result for 'baz'")
		}
		if exec_count != 3 {
			t.Errorf("Expected 3 executions, got %d", exec_count)
		}
		if len(exec_inputs) != 3 {
			t.Errorf("Expected 3 inputs recorded, got %d", len(exec_inputs))
		}
	})

	t.Run("Different_Input_Types", func(t *testing.T) {
		int_task := NewTask(func(ctx *Ctx, input int) (int, error) {
			return input * 2, nil
		})

		ctx := NewCtx(context.Background())
		r1, _ := run_task(ctx, int_task, 5)
		r2, _ := run_task(ctx, int_task, 10)
		r3, _ := run_task(ctx, int_task, 5)

		if r1 != 10 || r3 != 10 {
			t.Error("Expected same result for same int input")
		}
		if r2 != 20 {
			t.Error("Expected different result for different int input")
		}
	})

	t.Run("Struct_Inputs", func(t *testing.T) {
		type person struct {
			Name string
			Age  int
		}

		var exec_count int32
		task := NewTask(func(ctx *Ctx, p person) (string, error) {
			atomic.AddInt32(&exec_count, 1)
			return fmt.Sprintf("%s is %d", p.Name, p.Age), nil
		})

		ctx := NewCtx(context.Background())
		p1 := person{Name: "Alice", Age: 30}
		p2 := person{Name: "Bob", Age: 25}

		r1, _ := run_task(ctx, task, p1)
		r2, _ := run_task(ctx, task, p2)
		r3, _ := run_task(ctx, task, p1)

		if r1 != "Alice is 30" || r3 != "Alice is 30" {
			t.Error("Expected same result for same struct")
		}
		if r2 != "Bob is 25" {
			t.Error("Expected different result for different struct")
		}
		if exec_count != 2 {
			t.Errorf("Expected 2 executions, got %d", exec_count)
		}
	})

	t.Run("Parallel_Different_Inputs", func(t *testing.T) {
		var exec_count int32
		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&exec_count, 1)
			time.Sleep(50 * time.Millisecond)
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())
		var r1, r2, r3 string
		err := ctx.RunParallel(
			task.Bind("alpha", &r1),
			task.Bind("beta", &r2),
			task.Bind("alpha", &r3),
		)
		if err != nil {
			t.Fatal(err)
		}

		if r1 != "result-alpha" || r3 != "result-alpha" {
			t.Error("Expected same result for 'alpha'")
		}
		if r2 != "result-beta" {
			t.Error("Expected different result for 'beta'")
		}
		if exec_count != 2 {
			t.Errorf("Expected 2 executions, got %d", exec_count)
		}
	})
}

func TestTTL_BasicExpiration(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&exec_count, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	r1, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("First execution failed: %v", err)
	}
	if r1 != "test-1" {
		t.Errorf("Expected 'test-1', got '%s'", r1)
	}

	time.Sleep(50 * time.Millisecond)
	r2, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Second execution failed: %v", err)
	}
	if r2 != "test-1" {
		t.Errorf("Expected cached 'test-1', got '%s'", r2)
	}

	time.Sleep(60 * time.Millisecond)
	r3, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Third execution failed: %v", err)
	}
	if r3 != "test-2" {
		t.Errorf("Expected new 'test-2', got '%s'", r3)
	}

	if atomic.LoadInt32(&exec_count) != 2 {
		t.Errorf("Expected 2 executions, got %d", exec_count)
	}
}

func TestTTL_NoTTL_NeverExpires(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		atomic.AddInt32(&exec_count, 1)
		return "result", nil
	})

	ctx := NewCtx(context.Background())
	for i := 0; i < 5; i++ {
		if _, err := task.Run(ctx, "test"); err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if atomic.LoadInt32(&exec_count) != 1 {
		t.Errorf("Expected 1 execution, got %d", exec_count)
	}
}

func TestTTL_ZeroTTL_NeverExpires(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		atomic.AddInt32(&exec_count, 1)
		return "result", nil
	})

	ctx := NewCtxWithTTL(context.Background(), 0)
	for i := 0; i < 5; i++ {
		if _, err := task.Run(ctx, "test"); err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if atomic.LoadInt32(&exec_count) != 1 {
		t.Errorf("Expected 1 execution, got %d", exec_count)
	}
}

func TestTTL_DifferentInputs_SeparateExpiration(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&exec_count, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	r1, _ := task.Run(ctx, "a")
	if r1 != "a-1" {
		t.Errorf("Expected 'a-1', got '%s'", r1)
	}

	time.Sleep(50 * time.Millisecond)
	r2, _ := task.Run(ctx, "b")
	if r2 != "b-2" {
		t.Errorf("Expected 'b-2', got '%s'", r2)
	}

	time.Sleep(60 * time.Millisecond)

	r3, _ := task.Run(ctx, "a")
	if r3 != "a-3" {
		t.Errorf("Expected 'a-3' (expired), got '%s'", r3)
	}

	r4, _ := task.Run(ctx, "b")
	if r4 != "b-2" {
		t.Errorf("Expected 'b-2' (cached), got '%s'", r4)
	}

	if atomic.LoadInt32(&exec_count) != 3 {
		t.Errorf("Expected 3 executions, got %d", exec_count)
	}
}

func TestTTL_Cleanup_RemovesExpiredEntries(t *testing.T) {
	task := NewTask(func(ctx *Ctx, input int) (int, error) {
		return input * 2, nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	for i := 0; i < 10; i++ {
		if _, err := task.Run(ctx, i); err != nil {
			t.Fatalf("Failed with input %d: %v", i, err)
		}
	}

	initial_size := len(ctx.results)
	if initial_size != 10 {
		t.Errorf("Expected 10 cache entries, got %d", initial_size)
	}

	time.Sleep(ttl + 10*time.Millisecond)
	_, _ = task.Run(ctx, 100)

	ctx.mu.RLock()
	final_size := len(ctx.results)
	ctx.mu.RUnlock()

	if final_size != 1 {
		t.Errorf("Expected cleanup to reduce to 1, got %d", final_size)
	}
}

func TestTTL_Cleanup_OnlyRunsOncePerTTLPeriod(t *testing.T) {
	task := NewTask(func(ctx *Ctx, input int) (int, error) {
		return input, nil
	})

	ttl := 200 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	_, _ = task.Run(ctx, 1)
	initial_cleanup := ctx.last_cleanup.Load()

	time.Sleep(50 * time.Millisecond)
	_, _ = task.Run(ctx, 2)

	if ctx.last_cleanup.Load() != initial_cleanup {
		t.Error("Cleanup ran too early")
	}

	time.Sleep(160 * time.Millisecond)
	_, _ = task.Run(ctx, 3)

	if ctx.last_cleanup.Load() == initial_cleanup {
		t.Error("Cleanup did not run after TTL period elapsed")
	}
}

func TestTTL_ConcurrentAccess_WithExpiration(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&exec_count, 1)
		time.Sleep(10 * time.Millisecond)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	var wg sync.WaitGroup
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := task.Run(ctx, "test")
			if err != nil {
				t.Errorf("Goroutine %d error: %v", idx, err)
			}
			results[idx] = result
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r != "test-1" {
			t.Errorf("Goroutine %d got '%s', expected 'test-1'", i, r)
		}
	}

	time.Sleep(110 * time.Millisecond)

	results_2 := make([]string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := task.Run(ctx, "test")
			if err != nil {
				t.Errorf("Goroutine %d error: %v", idx, err)
			}
			results_2[idx] = result
		}(i)
	}
	wg.Wait()

	for i, r := range results_2 {
		if r != "test-2" {
			t.Errorf("Goroutine %d got '%s', expected 'test-2'", i, r)
		}
	}

	if atomic.LoadInt32(&exec_count) != 2 {
		t.Errorf("Expected 2 executions total, got %d", exec_count)
	}
}

func TestTTL_ParallelExecution_WithSharedDependency(t *testing.T) {
	var auth_count, task1_count, task2_count int32

	auth_task := NewTask(func(ctx *Ctx, _ struct{}) (int, error) {
		count := atomic.AddInt32(&auth_count, 1)
		time.Sleep(20 * time.Millisecond)
		return int(count), nil
	})

	task_1 := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&task1_count, 1)
		token, err := auth_task.Run(ctx, struct{}{})
		if err != nil {
			return "", err
		}
		return "task1-" + string(rune('0'+token)), nil
	})

	task_2 := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&task2_count, 1)
		token, err := auth_task.Run(ctx, struct{}{})
		if err != nil {
			return "", err
		}
		return "task2-" + string(rune('0'+token)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	var r1, r2 string
	if err := ctx.RunParallel(task_1.Bind("input", &r1), task_2.Bind("input", &r2)); err != nil {
		t.Fatalf("First parallel failed: %v", err)
	}
	if r1 != "task1-1" || r2 != "task2-1" {
		t.Errorf("Expected 'task1-1' and 'task2-1', got '%s' and '%s'", r1, r2)
	}

	time.Sleep(50 * time.Millisecond)
	var r3, r4 string
	if err := ctx.RunParallel(task_1.Bind("input", &r3), task_2.Bind("input", &r4)); err != nil {
		t.Fatalf("Second parallel failed: %v", err)
	}
	if r3 != "task1-1" || r4 != "task2-1" {
		t.Errorf("Expected cached values, got '%s' and '%s'", r3, r4)
	}

	time.Sleep(60 * time.Millisecond)
	var r5, r6 string
	if err := ctx.RunParallel(task_1.Bind("input", &r5), task_2.Bind("input", &r6)); err != nil {
		t.Fatalf("Third parallel failed: %v", err)
	}
	if r5 != "task1-2" || r6 != "task2-2" {
		t.Errorf("Expected new values, got '%s' and '%s'", r5, r6)
	}

	if atomic.LoadInt32(&auth_count) != 2 {
		t.Errorf("Expected auth to run 2 times, got %d", auth_count)
	}
	if atomic.LoadInt32(&task1_count) != 2 {
		t.Errorf("Expected task1 to run 2 times, got %d", task1_count)
	}
	if atomic.LoadInt32(&task2_count) != 2 {
		t.Errorf("Expected task2 to run 2 times, got %d", task2_count)
	}
}

func TestTTL_ExpiredResultsAllowRetry(t *testing.T) {
	var exec_count int32
	should_fail := true

	task := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&exec_count, 1)
		if should_fail {
			return "", errors.New("intentional error")
		}
		return "success", nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	_, err := task.Run(ctx, "test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	_, err = task.Run(ctx, "test")
	if err == nil {
		t.Fatal("Expected error on retry, got nil")
	}

	time.Sleep(110 * time.Millisecond)
	should_fail = false

	result, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Expected success after TTL, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}

	if atomic.LoadInt32(&exec_count) != 2 {
		t.Errorf("Expected 2 executions, got %d", exec_count)
	}
}

func TestTTL_VeryShortTTL(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, _ string) (int, error) {
		return int(atomic.AddInt32(&exec_count, 1)), nil
	})

	ttl := 10 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	var results []int
	for i := 0; i < 5; i++ {
		result, err := task.Run(ctx, "test")
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		results = append(results, result)
		time.Sleep(15 * time.Millisecond)
	}

	for i, r := range results {
		if r != i+1 {
			t.Errorf("Iteration %d: expected %d, got %d", i, i+1, r)
		}
	}

	if atomic.LoadInt32(&exec_count) != 5 {
		t.Errorf("Expected 5 executions, got %d", exec_count)
	}
}

func TestTTL_MultipleContexts_IndependentCaches(t *testing.T) {
	var exec_count int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&exec_count, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx1 := NewCtxWithTTL(context.Background(), ttl)
	ctx2 := NewCtxWithTTL(context.Background(), ttl)

	r1, _ := task.Run(ctx1, "test")
	r2, _ := task.Run(ctx2, "test")

	if r1 != "test-1" {
		t.Errorf("ctx1: expected 'test-1', got '%s'", r1)
	}
	if r2 != "test-2" {
		t.Errorf("ctx2: expected 'test-2', got '%s'", r2)
	}

	r1b, _ := task.Run(ctx1, "test")
	r2b, _ := task.Run(ctx2, "test")

	if r1b != "test-1" {
		t.Errorf("ctx1 cache: expected 'test-1', got '%s'", r1b)
	}
	if r2b != "test-2" {
		t.Errorf("ctx2 cache: expected 'test-2', got '%s'", r2b)
	}

	if atomic.LoadInt32(&exec_count) != 2 {
		t.Errorf("Expected 2 executions, got %d", exec_count)
	}
}

func TestNewCtxWithTTLNegativeDisablesTTL(t *testing.T) {
	ctx := NewCtxWithTTL(context.Background(), -1*time.Second)
	if ctx.ttl != 0 {
		t.Fatalf("expected ttl=0 for negative input, got %v", ctx.ttl)
	}

	var count int32
	task := NewTask(func(_ *Ctx, input string) (string, error) {
		n := atomic.AddInt32(&count, 1)
		return input + "-" + string(rune('0'+n)), nil
	})

	first, err := task.Run(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	second, err := task.Run(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatalf("expected cached result, got %q and %q", first, second)
	}
	if count != 1 {
		t.Fatalf("expected single execution, got %d", count)
	}
}

func TestRunWithAnyInputTypeChecks(t *testing.T) {
	ctx := NewCtx(context.Background())
	task := NewTask(func(_ *Ctx, input string) (string, error) {
		return "ok:" + input, nil
	})

	got, err := task.RunWithAnyInput(ctx, "x")
	if err != nil {
		t.Fatalf("expected no error for matching type, got %v", err)
	}
	if got != "ok:x" {
		t.Fatalf("unexpected result: %v", got)
	}

	_, err = task.RunWithAnyInput(ctx, 123)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}

	_, err = task.RunWithAnyInput(ctx, nil)
	if err == nil {
		t.Fatal("expected type mismatch error for untyped nil")
	}
}

func TestRunWithAnyInputTypedNilPointer(t *testing.T) {
	ctx := NewCtx(context.Background())
	task := NewTask(func(_ *Ctx, input *int) (bool, error) {
		return input == nil, nil
	})

	var typed_nil *int
	got, err := task.RunWithAnyInput(ctx, typed_nil)
	if err != nil {
		t.Fatalf("expected no error for typed nil pointer, got %v", err)
	}
	if got != true {
		t.Fatalf("expected true, got %v", got)
	}
}
