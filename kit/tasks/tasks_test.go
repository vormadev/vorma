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
		task1 := NewTask(func(c *Ctx, input int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return input * 2, nil
		})

		task2 := NewTask(func(c *Ctx, input string) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return input + "3", nil
		})

		ctx := NewCtx(context.Background())
		start := time.Now()

		var result1 int
		var result2 string
		err := ctx.RunParallel(
			task1.Bind(5, &result1),
			task2.Bind("3", &result2),
		)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected no errors, got %v", err)
		}
		if result1 != 10 || result2 != "33" {
			t.Errorf("Expected 10 and 15, got %d and %s", result1, result2)
		}
		if duration > 150*time.Millisecond {
			t.Errorf("Expected parallel execution (<150ms), took %v", duration)
		}
	})

	t.Run("TaskDependencies", func(t *testing.T) {
		authTask := NewTask(func(c *Ctx, input string) (string, error) {
			return "token-" + input, nil
		})

		userTask := NewTask(func(c *Ctx, input string) (string, error) {
			token, err := authTask.Run(c, input)
			if err != nil {
				return "", err
			}
			return "user-" + token, nil
		})

		ctx := NewCtx(context.Background())
		result, err := userTask.Run(ctx, "123")

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

		parentCtx, cancel := context.WithCancel(context.Background())
		ctx := NewCtx(parentCtx)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := runTask(ctx, task, "test")
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
		result, err := runTask(ctx, task, "test")

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
				_, err := runTask(ctx, task, "test")
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
		var authCounter int32
		authTask := NewTask(func(c *Ctx, _ struct{}) (int, error) {
			atomic.AddInt32(&authCounter, 1)
			time.Sleep(100 * time.Millisecond)
			return 123, nil
		})

		userTask := NewTask(func(c *Ctx, _ string) (string, error) {
			token, err := runTask(c, authTask, struct{}{})
			if err != nil {
				return "", err
			}
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("user-%d", token), nil
		})

		user2Task := NewTask(func(c *Ctx, _ string) (string, error) {
			token, err := runTask(c, authTask, struct{}{})
			if err != nil {
				return "", err
			}
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("user2-%d", token), nil
		})

		ctx := NewCtx(context.Background())
		var userData, user2Data string
		err := ctx.RunParallel(
			userTask.Bind("test", &userData),
			user2Task.Bind("test", &user2Data),
		)

		if err != nil {
			t.Fatal("runTasks failed with an unexpected error:", err)
		}

		expectedUserData := "user-123"
		expectedUser2Data := "user2-123"
		if userData != expectedUserData {
			t.Errorf("Expected userTask to return '%s', got '%s'", expectedUserData, userData)
		}
		if user2Data != expectedUser2Data {
			t.Errorf("Expected user2Task to return '%s', got '%s'", expectedUser2Data, user2Data)
		}

		if authCounter != 1 {
			t.Errorf("Expected authTask to run exactly once, ran %d times", authCounter)
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

		gotParent, err := task.Run(parent, "a")
		if err != nil {
			t.Fatalf("parent run error = %v, want nil", err)
		}
		if gotParent != "ok-a" {
			t.Fatalf("parent run value = %q, want %q", gotParent, "ok-a")
		}

		gotChild, err := task.Run(child, "a")
		if err != nil {
			t.Fatalf("child run error = %v, want nil", err)
		}
		if gotChild != "ok-a" {
			t.Fatalf("child run value = %q, want %q", gotChild, "ok-a")
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

	t.Run(
		"AcquireSharedStateChildContextWithNativeContext_SharesCacheAndReleases",
		func(t *testing.T) {
			var runs atomic.Int32
			task := NewTask(func(c *Ctx, input string) (string, error) {
				runs.Add(1)
				return "ok-" + input, nil
			})

			parent := NewCtx(context.Background())
			child := parent.AcquireSharedStateChildContextWithNativeContext(
				context.Background(),
			)

			parentValue, err := task.Run(parent, "cache-key")
			if err != nil {
				t.Fatalf("parent run error = %v, want nil", err)
			}
			if parentValue != "ok-cache-key" {
				t.Fatalf("parent run value = %q, want %q", parentValue, "ok-cache-key")
			}

			childValue, err := task.Run(child, "cache-key")
			if err != nil {
				t.Fatalf("child run error = %v, want nil", err)
			}
			if childValue != "ok-cache-key" {
				t.Fatalf("child run value = %q, want %q", childValue, "ok-cache-key")
			}
			if runs.Load() != 1 {
				t.Fatalf("task runs = %d, want 1", runs.Load())
			}

			ReleaseSharedStateChildContext(child)

			if child.isBorrowedSharedStateChildContext {
				t.Fatal(
					"expected released child context to clear borrowed-child marker",
				)
			}
			if child.mu != nil ||
				child.results != nil ||
				child.ctx != nil ||
				child.lastCleanup != nil {
				t.Fatal("expected released child context internals to be cleared")
			}
		},
	)
}

func TestComprehensiveSharedDependencies(t *testing.T) {
	// State tracking variables for the test
	var executionOrder []string
	var executionMu sync.Mutex
	recordExecution := func(name string) {
		executionMu.Lock()
		executionOrder = append(executionOrder, name)
		executionMu.Unlock()
	}

	var authCounter, userCounter, user2Counter, profileCounter int32
	var userInputs, user2Inputs []string
	var userTokens, user2Tokens []int
	var stateMu sync.Mutex // A single mutex to protect all test state slices/maps

	// Define Tasks using the new API
	authTask := NewTask(func(c *Ctx, _ struct{}) (int, error) {
		recordExecution("auth-start")
		atomic.AddInt32(&authCounter, 1)
		time.Sleep(50 * time.Millisecond)
		recordExecution("auth-end")
		return 123, nil
	})

	userTask := NewTask(func(c *Ctx, input string) (string, error) {
		recordExecution("user-start")
		atomic.AddInt32(&userCounter, 1)
		if input == "" {
			t.Error("Expected non-empty input in userTask")
		}

		token, err := runTask(c, authTask, struct{}{})
		if err != nil {
			return "", err
		}

		stateMu.Lock()
		userInputs = append(userInputs, input)
		userTokens = append(userTokens, token)
		stateMu.Unlock()

		time.Sleep(25 * time.Millisecond)
		recordExecution("user-end")
		return fmt.Sprintf("user-%s-%d", input, token), nil
	})

	user2Task := NewTask(func(c *Ctx, input string) (string, error) {
		recordExecution("user2-start")
		atomic.AddInt32(&user2Counter, 1)
		if input == "" {
			t.Error("Expected non-empty input in user2Task")
		}

		token, err := runTask(c, authTask, struct{}{})
		if err != nil {
			return "", err
		}

		stateMu.Lock()
		user2Inputs = append(user2Inputs, input)
		user2Tokens = append(user2Tokens, token)
		stateMu.Unlock()

		time.Sleep(25 * time.Millisecond)
		recordExecution("user2-end")
		return fmt.Sprintf("user2-%s-%d", input, token), nil
	})

	profileTask := NewTask(func(ctx *Ctx, input string) (map[string]string, error) {
		recordExecution("profile-start")
		atomic.AddInt32(&profileCounter, 1)

		var userData, user2Data string
		err := ctx.RunParallel(
			userTask.Bind(input, &userData),
			user2Task.Bind(input+"_alt", &user2Data),
		)
		if err != nil {
			return nil, err
		}

		time.Sleep(25 * time.Millisecond)
		recordExecution("profile-end")

		return map[string]string{
			"user":   userData,
			"user2":  user2Data,
			"status": "complete",
		}, nil
	})

	// --- Execute Test Cases ---
	const testInput1 = "test_input_1"
	const testInput2 = "test_input_2"

	// Execution for first context
	ctx1 := NewCtx(context.Background())
	profileResult1, profileErr1 := runTask(ctx1, profileTask, testInput1)

	// Execution for second context
	ctx2 := NewCtx(context.Background())
	profileResult2, profileErr2 := runTask(ctx2, profileTask, testInput2)

	// --- VERIFICATIONS ---
	if profileErr1 != nil {
		t.Errorf("Expected no error from first profile, got %v", profileErr1)
	}
	if profileErr2 != nil {
		t.Errorf("Expected no error from second profile, got %v", profileErr2)
	}

	expectedUserData1 := "user-test_input_1-123"
	expectedUser2Data1 := "user2-test_input_1_alt-123"
	if profileResult1["user"] != expectedUserData1 {
		t.Errorf("Expected profile1.user to be '%s', got '%s'", expectedUserData1, profileResult1["user"])
	}
	if profileResult1["user2"] != expectedUser2Data1 {
		t.Errorf("Expected profile1.user2 to be '%s', got '%s'", expectedUser2Data1, profileResult1["user2"])
	}

	expectedUserData2 := "user-test_input_2-123"
	expectedUser2Data2 := "user2-test_input_2_alt-123"
	if profileResult2["user"] != expectedUserData2 {
		t.Errorf("Expected profile2.user to be '%s', got '%s'", expectedUserData2, profileResult2["user"])
	}
	if profileResult2["user2"] != expectedUser2Data2 {
		t.Errorf("Expected profile2.user2 to be '%s', got '%s'", expectedUser2Data2, profileResult2["user2"])
	}

	if atomic.LoadInt32(&authCounter) != 2 {
		t.Errorf("Expected authTask to run twice (once per context), ran %d times", authCounter)
	}
	if atomic.LoadInt32(&userCounter) != 2 {
		t.Errorf("Expected userTask to run twice, ran %d times", userCounter)
	}
	if atomic.LoadInt32(&user2Counter) != 2 {
		t.Errorf("Expected user2Task to run twice, ran %d times", user2Counter)
	}
	if atomic.LoadInt32(&profileCounter) != 2 {
		t.Errorf("Expected profileTask to run twice, ran %d times", profileCounter)
	}

	// Verify execution order for key events
	verifyExecutionOrder := func(events [2]string, message string) {
		executionMu.Lock()
		defer executionMu.Unlock()
		firstIndex := -1
		// Find the first event
		for i, event := range executionOrder {
			if event == events[0] {
				firstIndex = i
				break
			}
		}
		if firstIndex == -1 {
			t.Errorf("Execution order event not found: %s. Order: %v", events[0], executionOrder)
			return
		}
		// Search for the second event *after* the first one
		for i := firstIndex + 1; i < len(executionOrder); i++ {
			if executionOrder[i] == events[1] {
				return // Found in correct order
			}
		}
		t.Errorf("Expected '%s' to appear after '%s', but it didn't. Order: %v. Message: %s", events[1], events[0], executionOrder, message)
	}

	// Check that auth completes before its dependents can finish
	verifyExecutionOrder([2]string{"auth-end", "user-end"}, "userTask should finish after authTask")
	verifyExecutionOrder([2]string{"auth-end", "user2-end"}, "user2Task should finish after authTask")

	// Check that the top-level profile task finishes after its own dependents
	verifyExecutionOrder([2]string{"user-end", "profile-end"}, "profileTask should finish after userTask")
	verifyExecutionOrder([2]string{"user2-end", "profile-end"}, "profileTask should finish after user2Task")

	// Log for diagnostics if needed
	t.Logf("Execution order: %v", executionOrder)
}

func TestTasksWithDifferentInputs(t *testing.T) {
	t.Run("Same_Input_Uses_Cache", func(t *testing.T) {
		var execCount int32
		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&execCount, 1)
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())

		// Call 3 times with same input
		r1, _ := runTask(ctx, task, "foo")
		r2, _ := runTask(ctx, task, "foo")
		r3, _ := runTask(ctx, task, "foo")

		if r1 != "result-foo" || r2 != "result-foo" || r3 != "result-foo" {
			t.Error("Expected same result for same input")
		}

		if execCount != 1 {
			t.Errorf("Expected task to execute once, executed %d times", execCount)
		}
	})

	t.Run("Different_Inputs_Execute_Separately", func(t *testing.T) {
		var execCount int32
		execInputs := make([]string, 0)
		var mu sync.Mutex

		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&execCount, 1)
			mu.Lock()
			execInputs = append(execInputs, input)
			mu.Unlock()
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())

		// Call with different inputs
		r1, _ := runTask(ctx, task, "foo")
		r2, _ := runTask(ctx, task, "bar")
		r3, _ := runTask(ctx, task, "baz")

		// Call again with same inputs (should use cache)
		r1b, _ := runTask(ctx, task, "foo")
		r2b, _ := runTask(ctx, task, "bar")

		// Verify results
		if r1 != "result-foo" || r1b != "result-foo" {
			t.Error("Expected consistent results for 'foo'")
		}
		if r2 != "result-bar" || r2b != "result-bar" {
			t.Error("Expected consistent results for 'bar'")
		}
		if r3 != "result-baz" {
			t.Error("Expected correct result for 'baz'")
		}

		// Should execute exactly 3 times (once per unique input)
		if execCount != 3 {
			t.Errorf("Expected 3 executions, got %d", execCount)
		}

		// Verify it saw all 3 inputs
		if len(execInputs) != 3 {
			t.Errorf("Expected 3 inputs recorded, got %d", len(execInputs))
		}
	})

	t.Run("Different_Input_Types", func(t *testing.T) {
		// Test with int inputs
		intTask := NewTask(func(ctx *Ctx, input int) (int, error) {
			return input * 2, nil
		})

		ctx := NewCtx(context.Background())

		r1, _ := runTask(ctx, intTask, 5)
		r2, _ := runTask(ctx, intTask, 10)
		r3, _ := runTask(ctx, intTask, 5) // Same as r1

		if r1 != 10 || r3 != 10 {
			t.Error("Expected same result for same int input")
		}
		if r2 != 20 {
			t.Error("Expected different result for different int input")
		}
	})

	t.Run("Struct_Inputs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		var execCount int32
		task := NewTask(func(ctx *Ctx, p Person) (string, error) {
			atomic.AddInt32(&execCount, 1)
			return fmt.Sprintf("%s is %d", p.Name, p.Age), nil
		})

		ctx := NewCtx(context.Background())

		p1 := Person{Name: "Alice", Age: 30}
		p2 := Person{Name: "Bob", Age: 25}

		r1, _ := runTask(ctx, task, p1)
		r2, _ := runTask(ctx, task, p2)
		r3, _ := runTask(ctx, task, p1) // Same as first

		if r1 != "Alice is 30" || r3 != "Alice is 30" {
			t.Error("Expected same result for same struct")
		}
		if r2 != "Bob is 25" {
			t.Error("Expected different result for different struct")
		}

		if execCount != 2 {
			t.Errorf("Expected 2 executions, got %d", execCount)
		}
	})

	t.Run("Parallel_Different_Inputs", func(t *testing.T) {
		var execCount int32
		task := NewTask(func(ctx *Ctx, input string) (string, error) {
			atomic.AddInt32(&execCount, 1)
			time.Sleep(50 * time.Millisecond)
			return "result-" + input, nil
		})

		ctx := NewCtx(context.Background())

		var result1, result2, result3 string
		err := ctx.RunParallel(
			task.Bind("alpha", &result1),
			task.Bind("beta", &result2),
			task.Bind("alpha", &result3), // Duplicate
		)

		if err != nil {
			t.Fatal(err)
		}

		if result1 != "result-alpha" || result3 != "result-alpha" {
			t.Error("Expected same result for 'alpha'")
		}
		if result2 != "result-beta" {
			t.Error("Expected different result for 'beta'")
		}

		// Should only execute twice (alpha and beta)
		if execCount != 2 {
			t.Errorf("Expected 2 executions, got %d", execCount)
		}
	})
}

func TestTTL_BasicExpiration(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&execCount, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// First execution
	result1, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("First execution failed: %v", err)
	}
	if result1 != "test-1" {
		t.Errorf("Expected 'test-1', got '%s'", result1)
	}

	// Second execution (within TTL, should use cache)
	time.Sleep(50 * time.Millisecond)
	result2, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Second execution failed: %v", err)
	}
	if result2 != "test-1" {
		t.Errorf("Expected cached 'test-1', got '%s'", result2)
	}

	// Third execution (after TTL, should re-execute)
	time.Sleep(60 * time.Millisecond)
	result3, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Third execution failed: %v", err)
	}
	if result3 != "test-2" {
		t.Errorf("Expected new 'test-2', got '%s'", result3)
	}

	if atomic.LoadInt32(&execCount) != 2 {
		t.Errorf("Expected 2 executions, got %d", execCount)
	}
}

func TestTTL_NoTTL_NeverExpires(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		atomic.AddInt32(&execCount, 1)
		return "result", nil
	})

	// Using NewCtx (no TTL)
	ctx := NewCtx(context.Background())

	// Execute multiple times with delays
	for i := 0; i < 5; i++ {
		_, err := task.Run(ctx, "test")
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if atomic.LoadInt32(&execCount) != 1 {
		t.Errorf("Expected 1 execution (cache should never expire), got %d", execCount)
	}
}

func TestTTL_ZeroTTL_NeverExpires(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		atomic.AddInt32(&execCount, 1)
		return "result", nil
	})

	// Explicitly set TTL to 0
	ctx := NewCtxWithTTL(context.Background(), 0)

	// Execute multiple times with delays
	for i := 0; i < 5; i++ {
		_, err := task.Run(ctx, "test")
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if atomic.LoadInt32(&execCount) != 1 {
		t.Errorf("Expected 1 execution (zero TTL should never expire), got %d", execCount)
	}
}

func TestTTL_DifferentInputs_SeparateExpiration(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&execCount, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// Execute with input "a"
	result1, _ := task.Run(ctx, "a")
	if result1 != "a-1" {
		t.Errorf("Expected 'a-1', got '%s'", result1)
	}

	// Wait 50ms, execute with input "b"
	time.Sleep(50 * time.Millisecond)
	result2, _ := task.Run(ctx, "b")
	if result2 != "b-2" {
		t.Errorf("Expected 'b-2', got '%s'", result2)
	}

	// Wait another 60ms (110ms total, "a" expired but "b" still valid)
	time.Sleep(60 * time.Millisecond)

	// "a" should be expired and re-execute
	result3, _ := task.Run(ctx, "a")
	if result3 != "a-3" {
		t.Errorf("Expected 'a-3' (expired), got '%s'", result3)
	}

	// "b" should still be cached
	result4, _ := task.Run(ctx, "b")
	if result4 != "b-2" {
		t.Errorf("Expected 'b-2' (cached), got '%s'", result4)
	}

	if atomic.LoadInt32(&execCount) != 3 {
		t.Errorf("Expected 3 executions, got %d", execCount)
	}
}

func TestTTL_Cleanup_RemovesExpiredEntries(t *testing.T) {
	task := NewTask(func(ctx *Ctx, input int) (int, error) {
		return input * 2, nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// Create many entries
	for i := 0; i < 10; i++ {
		_, err := task.Run(ctx, i)
		if err != nil {
			t.Fatalf("Failed to execute task with input %d: %v", i, err)
		}
	}

	// Verify all entries are cached
	initialSize := len(ctx.results)
	if initialSize != 10 {
		t.Errorf("Expected 10 cache entries, got %d", initialSize)
	}

	// Wait for entries to expire and trigger cleanup
	time.Sleep(ttl + 10*time.Millisecond)

	// Access any task to trigger cleanup
	_, _ = task.Run(ctx, 100)

	// Check that old entries were cleaned up
	ctx.mu.RLock()
	finalSize := len(ctx.results)
	ctx.mu.RUnlock()

	// Should have 1 entry (the new one with input 100)
	if finalSize != 1 {
		t.Errorf("Expected cleanup to reduce entries to 1, got %d", finalSize)
	}
}

func TestTTL_Cleanup_OnlyRunsOncePerTTLPeriod(t *testing.T) {
	task := NewTask(func(ctx *Ctx, input int) (int, error) {
		return input, nil
	})

	ttl := 200 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// Create initial entry
	_, _ = task.Run(ctx, 1)

	// Record initial cleanup time
	initialCleanup := ctx.lastCleanup.Load()

	// Wait less than TTL
	time.Sleep(50 * time.Millisecond)

	// Try to trigger cleanup by accessing an expired entry
	// (won't actually expire yet, but tests the cleanup timing logic)
	_, _ = task.Run(ctx, 2)

	// Cleanup timestamp should not have changed
	if ctx.lastCleanup.Load() != initialCleanup {
		t.Error("Cleanup ran too early (before TTL period elapsed)")
	}

	// Wait for TTL to elapse
	time.Sleep(160 * time.Millisecond)

	// Now trigger cleanup
	_, _ = task.Run(ctx, 3)

	// Cleanup timestamp should have updated
	if ctx.lastCleanup.Load() == initialCleanup {
		t.Error("Cleanup did not run after TTL period elapsed")
	}
}

func TestTTL_ConcurrentAccess_WithExpiration(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&execCount, 1)
		time.Sleep(10 * time.Millisecond)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// Phase 1: Concurrent access within TTL (should use cache)
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

	// All should have same result
	for i, result := range results {
		if result != "test-1" {
			t.Errorf("Goroutine %d got '%s', expected 'test-1'", i, result)
		}
	}

	// Wait for expiration
	time.Sleep(110 * time.Millisecond)

	// Phase 2: Concurrent access after expiration (should re-execute once)
	results2 := make([]string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := task.Run(ctx, "test")
			if err != nil {
				t.Errorf("Goroutine %d error: %v", idx, err)
			}
			results2[idx] = result
		}(i)
	}
	wg.Wait()

	// All should have same new result
	for i, result := range results2 {
		if result != "test-2" {
			t.Errorf("Goroutine %d got '%s', expected 'test-2'", i, result)
		}
	}

	if atomic.LoadInt32(&execCount) != 2 {
		t.Errorf("Expected 2 executions total, got %d", execCount)
	}
}

func TestTTL_ParallelExecution_WithSharedDependency(t *testing.T) {
	var authCount, task1Count, task2Count int32

	authTask := NewTask(func(ctx *Ctx, _ struct{}) (int, error) {
		count := atomic.AddInt32(&authCount, 1)
		time.Sleep(20 * time.Millisecond)
		return int(count), nil
	})

	task1 := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&task1Count, 1)
		token, err := authTask.Run(ctx, struct{}{})
		if err != nil {
			return "", err
		}
		return "task1-" + string(rune('0'+token)), nil
	})

	task2 := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&task2Count, 1)
		token, err := authTask.Run(ctx, struct{}{})
		if err != nil {
			return "", err
		}
		return "task2-" + string(rune('0'+token)), nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// First parallel execution
	var r1, r2 string
	err := ctx.RunParallel(
		task1.Bind("input", &r1),
		task2.Bind("input", &r2),
	)
	if err != nil {
		t.Fatalf("First parallel execution failed: %v", err)
	}

	if r1 != "task1-1" || r2 != "task2-1" {
		t.Errorf("Expected 'task1-1' and 'task2-1', got '%s' and '%s'", r1, r2)
	}

	// Second parallel execution (within TTL, should use cache)
	time.Sleep(50 * time.Millisecond)
	var r3, r4 string
	err = ctx.RunParallel(
		task1.Bind("input", &r3),
		task2.Bind("input", &r4),
	)
	if err != nil {
		t.Fatalf("Second parallel execution failed: %v", err)
	}

	if r3 != "task1-1" || r4 != "task2-1" {
		t.Errorf("Expected cached values, got '%s' and '%s'", r3, r4)
	}

	// Third parallel execution (after TTL expiration)
	time.Sleep(60 * time.Millisecond)
	var r5, r6 string
	err = ctx.RunParallel(
		task1.Bind("input", &r5),
		task2.Bind("input", &r6),
	)
	if err != nil {
		t.Fatalf("Third parallel execution failed: %v", err)
	}

	if r5 != "task1-2" || r6 != "task2-2" {
		t.Errorf("Expected new values after expiration, got '%s' and '%s'", r5, r6)
	}

	// Verify execution counts
	if atomic.LoadInt32(&authCount) != 2 {
		t.Errorf("Expected authTask to execute 2 times, got %d", authCount)
	}
	if atomic.LoadInt32(&task1Count) != 2 {
		t.Errorf("Expected task1 to execute 2 times, got %d", task1Count)
	}
	if atomic.LoadInt32(&task2Count) != 2 {
		t.Errorf("Expected task2 to execute 2 times, got %d", task2Count)
	}
}

func TestTTL_ExpiredResultsAllowRetry(t *testing.T) {
	var execCount int32
	shouldFail := true

	task := NewTask(func(ctx *Ctx, _ string) (string, error) {
		atomic.AddInt32(&execCount, 1)
		if shouldFail {
			return "", errors.New("intentional error")
		}
		return "success", nil
	})

	ttl := 100 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	// First call should fail
	_, err := task.Run(ctx, "test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Immediate retry should still fail (errors are cached by sync.Once)
	_, err = task.Run(ctx, "test")
	if err == nil {
		t.Fatal("Expected error on retry, got nil")
	}

	// Wait for TTL and change behavior
	time.Sleep(110 * time.Millisecond)
	shouldFail = false

	// After TTL, should create new result and succeed
	result, err := task.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Expected success after TTL, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}

	// Note: Because of sync.Once, the first execution (error) is permanently
	// cached in that TaskResult. The TTL causes a NEW TaskResult to be created.
	// So we expect 2 executions total.
	if atomic.LoadInt32(&execCount) != 2 {
		t.Errorf("Expected 2 executions, got %d", execCount)
	}
}

func TestTTL_VeryShortTTL(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, _ string) (int, error) {
		return int(atomic.AddInt32(&execCount, 1)), nil
	})

	// Very short TTL
	ttl := 10 * time.Millisecond
	ctx := NewCtxWithTTL(context.Background(), ttl)

	results := make([]int, 0)

	// Execute multiple times with delays
	for i := 0; i < 5; i++ {
		result, err := task.Run(ctx, "test")
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}
		results = append(results, result)
		time.Sleep(15 * time.Millisecond) // Longer than TTL
	}

	// Each execution should have been fresh (not cached)
	for i, result := range results {
		expected := i + 1
		if result != expected {
			t.Errorf("Iteration %d: expected %d, got %d", i, expected, result)
		}
	}

	if atomic.LoadInt32(&execCount) != 5 {
		t.Errorf("Expected 5 executions, got %d", execCount)
	}
}

func TestTTL_MultipleContexts_IndependentCaches(t *testing.T) {
	var execCount int32
	task := NewTask(func(ctx *Ctx, input string) (string, error) {
		count := atomic.AddInt32(&execCount, 1)
		return input + "-" + string(rune('0'+count)), nil
	})

	ttl := 100 * time.Millisecond
	ctx1 := NewCtxWithTTL(context.Background(), ttl)
	ctx2 := NewCtxWithTTL(context.Background(), ttl)

	// Execute in both contexts
	result1, _ := task.Run(ctx1, "test")
	result2, _ := task.Run(ctx2, "test")

	// Should execute twice (once per context)
	if result1 != "test-1" {
		t.Errorf("Context 1: expected 'test-1', got '%s'", result1)
	}
	if result2 != "test-2" {
		t.Errorf("Context 2: expected 'test-2', got '%s'", result2)
	}

	// Within TTL, both contexts should cache independently
	result1b, _ := task.Run(ctx1, "test")
	result2b, _ := task.Run(ctx2, "test")

	if result1b != "test-1" {
		t.Errorf("Context 1 cache: expected 'test-1', got '%s'", result1b)
	}
	if result2b != "test-2" {
		t.Errorf("Context 2 cache: expected 'test-2', got '%s'", result2b)
	}

	if atomic.LoadInt32(&execCount) != 2 {
		t.Errorf("Expected 2 executions, got %d", execCount)
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
		t.Fatalf("expected cached result with ttl disabled, got %q and %q", first, second)
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

	var typedNil *int
	got, err := task.RunWithAnyInput(ctx, typedNil)
	if err != nil {
		t.Fatalf("expected no error for typed nil pointer, got %v", err)
	}
	if got != true {
		t.Fatalf("expected true, got %v", got)
	}
}
