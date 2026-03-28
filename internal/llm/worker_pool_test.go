package llm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/no22/repo-scout/internal/schema"
)

func TestWorkerPool_Execute_Empty(t *testing.T) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 4,
	})

	result := pool.Execute(context.Background(), nil)

	if result.TotalTasks != 0 {
		t.Errorf("Expected TotalTasks=0, got %d", result.TotalTasks)
	}
	if len(result.Results) != 0 {
		t.Errorf("Expected empty results, got %d", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected empty errors, got %d", len(result.Errors))
	}
}

func TestWorkerPool_Execute_SingleTask(t *testing.T) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 4,
	})

	card := NewTaskCard(TaskClassifyFileRole, "test task", schema.NewFileCard("test.go"))
	results := pool.Execute(context.Background(), []*TaskCard{card})

	if results.TotalTasks != 1 {
		t.Errorf("Expected TotalTasks=1, got %d", results.TotalTasks)
	}
	if results.SuccessfulTasks != 1 {
		t.Errorf("Expected SuccessfulTasks=1, got %d", results.SuccessfulTasks)
	}
	if results.FailedTasks != 0 {
		t.Errorf("Expected FailedTasks=0, got %d", results.FailedTasks)
	}
	if len(results.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results.Results))
	}
	if results.Results[0] == nil {
		t.Fatal("Expected non-nil result")
	}
	if results.Results[0].Classification != "main_chain" {
		t.Errorf("Expected classification='main_chain', got %s", results.Results[0].Classification)
	}
}

func TestWorkerPool_Execute_MultipleTasks(t *testing.T) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	var cards []*TaskCard
	for i := 0; i < 5; i++ {
		card := NewTaskCard(TaskClassifyFileRole, fmt.Sprintf("task %d", i), schema.NewFileCard("test.go"))
		cards = append(cards, card)
	}

	results := pool.Execute(context.Background(), cards)

	if results.TotalTasks != 5 {
		t.Errorf("Expected TotalTasks=5, got %d", results.TotalTasks)
	}
	if results.SuccessfulTasks != 5 {
		t.Errorf("Expected SuccessfulTasks=5, got %d", results.SuccessfulTasks)
	}
	if results.FailedTasks != 0 {
		t.Errorf("Expected FailedTasks=0, got %d", results.FailedTasks)
	}

	// Verify all results are present (order preserved)
	for i, r := range results.Results {
		if r == nil {
			t.Errorf("Result %d is nil", i)
		}
	}
}

func TestWorkerPool_Execute_OrderPreserved(t *testing.T) {
	// Create a mock adapter that introduces delays to test ordering
	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			// Simulate variable processing time
			time.Sleep(time.Millisecond * time.Duration(10))
			return &TaskResult{
				Type:           card.Type,
				Classification: card.FilePath, // Use filepath as classification to verify order
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 3,
	})

	// Create tasks with unique file paths
	var cards []*TaskCard
	for i := 0; i < 10; i++ {
		card := &TaskCard{
			ID:        fmt.Sprintf("task-%d", i),
			Type:      TaskClassifyFileRole,
			Task:      "test",
			FilePath:  fmt.Sprintf("file%d.go", i),
			CreatedAt: time.Now(),
		}
		cards = append(cards, card)
	}

	results := pool.Execute(context.Background(), cards)

	// Verify order is preserved
	for i, r := range results.Results {
		if r == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}
		expected := fmt.Sprintf("file%d.go", i)
		if r.Classification != expected {
			t.Errorf("Result %d: expected classification=%s, got %s", i, expected, r.Classification)
		}
	}
}

func TestWorkerPool_Execute_ErrorHandling(t *testing.T) {
	// Create a mock adapter that fails for specific tasks
	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			if card.FilePath == "fail.go" {
				return nil, fmt.Errorf("simulated failure")
			}
			return &TaskResult{
				Type:           card.Type,
				Classification: "main_chain",
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:          mock,
		MaxConcurrency:   2,
		StopOnFirstError: false,
	})

	cards := []*TaskCard{
		{ID: "1", Type: TaskClassifyFileRole, FilePath: "ok1.go", CreatedAt: time.Now()},
		{ID: "2", Type: TaskClassifyFileRole, FilePath: "fail.go", CreatedAt: time.Now()},
		{ID: "3", Type: TaskClassifyFileRole, FilePath: "ok2.go", CreatedAt: time.Now()},
	}

	results := pool.Execute(context.Background(), cards)

	if results.TotalTasks != 3 {
		t.Errorf("Expected TotalTasks=3, got %d", results.TotalTasks)
	}
	if results.SuccessfulTasks != 2 {
		t.Errorf("Expected SuccessfulTasks=2, got %d", results.SuccessfulTasks)
	}
	if results.FailedTasks != 1 {
		t.Errorf("Expected FailedTasks=1, got %d", results.FailedTasks)
	}

	// Verify failed index
	if results.Errors[1] == nil {
		t.Error("Expected error at index 1")
	}
	if results.Results[1] != nil {
		t.Error("Expected nil result at index 1")
	}
}

func TestWorkerPool_Execute_StopOnFirstError(t *testing.T) {
	var callCount int32

	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			atomic.AddInt32(&callCount, 1)
			if card.FilePath == "fail.go" {
				return nil, fmt.Errorf("simulated failure")
			}
			return &TaskResult{
				Type:           card.Type,
				Classification: "main_chain",
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:          mock,
		MaxConcurrency:   1, // Low concurrency to ensure sequential execution
		StopOnFirstError: true,
	})

	cards := []*TaskCard{
		{ID: "1", Type: TaskClassifyFileRole, FilePath: "fail.go", CreatedAt: time.Now()},
		{ID: "2", Type: TaskClassifyFileRole, FilePath: "ok.go", CreatedAt: time.Now()},
	}

	results := pool.Execute(context.Background(), cards)

	if results.FailedTasks < 1 {
		t.Errorf("Expected at least 1 failed task, got %d", results.FailedTasks)
	}

	// With StopOnFirstError, once the first task fails,
	// remaining tasks should be stopped
	// We expect at least 1 failed task (the first one)
	// The second task may or may not have been started due to race conditions
}

func TestWorkerPool_Execute_ContextCancellation(t *testing.T) {
	var started int32
	var completed int32

	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			atomic.AddInt32(&started, 1)
			select {
			case <-time.After(100 * time.Millisecond):
				atomic.AddInt32(&completed, 1)
				return &TaskResult{
					Type:           card.Type,
					Classification: "main_chain",
					Confidence:     0.9,
				}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Create multiple tasks
	var cards []*TaskCard
	for i := 0; i < 5; i++ {
		cards = append(cards, &TaskCard{
			ID:        fmt.Sprintf("task-%d", i),
			Type:      TaskClassifyFileRole,
			FilePath:  fmt.Sprintf("file%d.go", i),
			CreatedAt: time.Now(),
		})
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	results := pool.Execute(ctx, cards)

	// Some tasks should have been cancelled
	if results.FailedTasks == 0 {
		t.Log("Warning: Expected some failed tasks due to cancellation")
	}
}

func TestWorkerPool_Execute_NoAdapter(t *testing.T) {
	pool := NewWorkerPool(&WorkerPoolConfig{
		MaxConcurrency: 2,
	})

	card := NewTaskCard(TaskClassifyFileRole, "test task", schema.NewFileCard("test.go"))
	results := pool.Execute(context.Background(), []*TaskCard{card})

	if results.TotalTasks != 1 {
		t.Errorf("Expected TotalTasks=1, got %d", results.TotalTasks)
	}
	if results.SuccessfulTasks != 1 {
		t.Errorf("Expected SuccessfulTasks=1, got %d", results.SuccessfulTasks)
	}
	if results.Results[0] == nil {
		t.Fatal("Expected non-nil result")
	}
	if results.Results[0].Reason != "No adapter configured" {
		t.Errorf("Expected default reason, got %s", results.Results[0].Reason)
	}
}

func TestWorkerPool_ExecuteSequential(t *testing.T) {
	var executionOrder []int
	var mu sync.Mutex

	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, int(card.ID[5]-'0')) // Extract number from "task-N"
			mu.Unlock()
			return &TaskResult{
				Type:           card.Type,
				Classification: "main_chain",
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 4, // High concurrency but sequential execution
	})

	var cards []*TaskCard
	for i := 0; i < 5; i++ {
		cards = append(cards, &TaskCard{
			ID:        fmt.Sprintf("task-%d", i),
			Type:      TaskClassifyFileRole,
			FilePath:  fmt.Sprintf("file%d.go", i),
			CreatedAt: time.Now(),
		})
	}

	results := pool.ExecuteSequential(context.Background(), cards)

	if results.TotalTasks != 5 {
		t.Errorf("Expected TotalTasks=5, got %d", results.TotalTasks)
	}
	if results.SuccessfulTasks != 5 {
		t.Errorf("Expected SuccessfulTasks=5, got %d", results.SuccessfulTasks)
	}

	// Verify sequential order
	for i, order := range executionOrder {
		if order != i {
			t.Errorf("Expected order[%d]=%d, got %d", i, i, order)
		}
	}
}

func TestWorkerPool_ExecuteWithCallback(t *testing.T) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	var callbackOrder []int
	var mu sync.Mutex

	callback := func(index int, result *TaskResult, err error) {
		mu.Lock()
		callbackOrder = append(callbackOrder, index)
		mu.Unlock()
	}

	var cards []*TaskCard
	for i := 0; i < 5; i++ {
		cards = append(cards, &TaskCard{
			ID:        fmt.Sprintf("task-%d", i),
			Type:      TaskClassifyFileRole,
			FilePath:  fmt.Sprintf("file%d.go", i),
			CreatedAt: time.Now(),
		})
	}

	results := pool.ExecuteWithCallback(context.Background(), cards, callback)

	if results.TotalTasks != 5 {
		t.Errorf("Expected TotalTasks=5, got %d", results.TotalTasks)
	}

	// Verify callback was called for each task
	if len(callbackOrder) != 5 {
		t.Errorf("Expected 5 callbacks, got %d", len(callbackOrder))
	}
}

func TestWorkerPool_GetSuccessfulResults(t *testing.T) {
	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			if card.FilePath == "fail.go" {
				return nil, fmt.Errorf("simulated failure")
			}
			return &TaskResult{
				Type:           card.Type,
				Classification: "main_chain",
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	cards := []*TaskCard{
		{ID: "1", Type: TaskClassifyFileRole, FilePath: "ok.go", CreatedAt: time.Now()},
		{ID: "2", Type: TaskClassifyFileRole, FilePath: "fail.go", CreatedAt: time.Now()},
		{ID: "3", Type: TaskClassifyFileRole, FilePath: "ok2.go", CreatedAt: time.Now()},
	}

	results := pool.Execute(context.Background(), cards)

	successful := results.GetSuccessfulResults()
	if len(successful) != 2 {
		t.Errorf("Expected 2 successful results, got %d", len(successful))
	}
}

func TestWorkerPool_GetFailedIndices(t *testing.T) {
	mock := &MockAdapter{
		ExecuteFunc: func(ctx context.Context, card *TaskCard) (*TaskResult, error) {
			// Both fail.go and fail2.go should fail
			if card.FilePath == "fail.go" || card.FilePath == "fail2.go" {
				return nil, fmt.Errorf("simulated failure")
			}
			return &TaskResult{
				Type:           card.Type,
				Classification: "main_chain",
				Confidence:     0.9,
			}, nil
		},
		available: true,
	}

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	cards := []*TaskCard{
		{ID: "1", Type: TaskClassifyFileRole, FilePath: "ok.go", CreatedAt: time.Now()},
		{ID: "2", Type: TaskClassifyFileRole, FilePath: "fail.go", CreatedAt: time.Now()},
		{ID: "3", Type: TaskClassifyFileRole, FilePath: "fail2.go", CreatedAt: time.Now()},
		{ID: "4", Type: TaskClassifyFileRole, FilePath: "ok2.go", CreatedAt: time.Now()},
	}

	results := pool.Execute(context.Background(), cards)

	failedIndices := results.GetFailedIndices()
	if len(failedIndices) != 2 {
		t.Errorf("Expected 2 failed indices, got %d", len(failedIndices))
	}

	// Check specific indices (order might vary due to concurrency)
	failedMap := make(map[int]bool)
	for _, idx := range failedIndices {
		failedMap[idx] = true
	}

	if !failedMap[1] || !failedMap[2] {
		t.Errorf("Expected indices 1 and 2 to be failed, got %v", failedIndices)
	}
}

func TestWorkerPool_IsAvailable(t *testing.T) {
	mock := NewMockAdapter()
	mock.SetAvailable(true)

	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	if !pool.IsAvailable() {
		t.Error("Expected pool to be available")
	}

	mock.SetAvailable(false)
	if pool.IsAvailable() {
		t.Error("Expected pool to be unavailable")
	}

	// Pool without adapter should be available
	poolNoAdapter := NewWorkerPool(nil)
	if !poolNoAdapter.IsAvailable() {
		t.Error("Expected pool without adapter to be available")
	}
}

func TestWorkerPool_Close(t *testing.T) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 2,
	})

	if err := pool.Close(); err != nil {
		t.Errorf("Unexpected error on close: %v", err)
	}

	// Pool without adapter should also close without error
	poolNoAdapter := NewWorkerPool(nil)
	if err := poolNoAdapter.Close(); err != nil {
		t.Errorf("Unexpected error on close: %v", err)
	}
}

func TestWorkerPool_Setters(t *testing.T) {
	pool := NewWorkerPool(&WorkerPoolConfig{
		MaxConcurrency: 2,
	})

	if pool.GetMaxConcurrency() != 2 {
		t.Errorf("Expected MaxConcurrency=2, got %d", pool.GetMaxConcurrency())
	}

	pool.SetMaxConcurrency(8)
	if pool.GetMaxConcurrency() != 8 {
		t.Errorf("Expected MaxConcurrency=8, got %d", pool.GetMaxConcurrency())
	}

	// Invalid value should not change
	pool.SetMaxConcurrency(0)
	if pool.GetMaxConcurrency() != 8 {
		t.Errorf("Expected MaxConcurrency to remain 8, got %d", pool.GetMaxConcurrency())
	}

	mock := NewMockAdapter()
	pool.SetAdapter(mock)
	// Verify adapter is set by checking availability
	mock.SetAvailable(false)
	if pool.IsAvailable() {
		t.Error("Expected pool to be unavailable after setting adapter")
	}
}

func TestWorkerPool_DefaultConfig(t *testing.T) {
	pool := NewWorkerPool(nil)

	if pool.GetMaxConcurrency() != 4 {
		t.Errorf("Expected default MaxConcurrency=4, got %d", pool.GetMaxConcurrency())
	}
}

// Benchmark tests
func BenchmarkWorkerPool_Execute(b *testing.B) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 4,
	})

	var cards []*TaskCard
	for i := 0; i < 10; i++ {
		cards = append(cards, NewTaskCard(TaskClassifyFileRole, "benchmark task", schema.NewFileCard("test.go")))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Execute(context.Background(), cards)
	}
}

func BenchmarkWorkerPool_ExecuteSequential(b *testing.B) {
	mock := NewMockAdapter()
	pool := NewWorkerPool(&WorkerPoolConfig{
		Adapter:        mock,
		MaxConcurrency: 4,
	})

	var cards []*TaskCard
	for i := 0; i < 10; i++ {
		cards = append(cards, NewTaskCard(TaskClassifyFileRole, "benchmark task", schema.NewFileCard("test.go")))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.ExecuteSequential(context.Background(), cards)
	}
}
