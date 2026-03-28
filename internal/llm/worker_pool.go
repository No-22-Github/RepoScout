// Package llm provides LLM integration for RepoScout.
package llm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// WorkerPool manages concurrent execution of TaskCards.
// It provides controlled concurrency with error isolation and result tracking.
type WorkerPool struct {
	// adapter is the LLM provider used to execute tasks.
	adapter ProviderAdapter

	// maxConcurrency limits the number of concurrent executions.
	maxConcurrency int

	// stopOnFirstError controls whether to stop on first error.
	// When false, errors are collected but execution continues.
	stopOnFirstError bool
}

// WorkerPoolConfig holds configuration for creating a WorkerPool.
type WorkerPoolConfig struct {
	// Adapter is the LLM provider adapter.
	Adapter ProviderAdapter

	// MaxConcurrency is the maximum number of concurrent executions.
	MaxConcurrency int

	// StopOnFirstError controls whether to stop on first error.
	StopOnFirstError bool
}

// NewWorkerPool creates a new WorkerPool with the given configuration.
func NewWorkerPool(cfg *WorkerPoolConfig) *WorkerPool {
	if cfg == nil {
		cfg = &WorkerPoolConfig{
			MaxConcurrency: 4,
		}
	}

	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 4
	}

	return &WorkerPool{
		adapter:          cfg.Adapter,
		maxConcurrency:   cfg.MaxConcurrency,
		stopOnFirstError: cfg.StopOnFirstError,
	}
}

// PoolResult represents the result of executing a batch of tasks.
type PoolResult struct {
	// Results contains the task results, indexed by input order.
	// If a task failed, its corresponding entry will be nil.
	Results []*TaskResult

	// Errors contains any errors encountered, indexed by input order.
	// If a task succeeded, its corresponding entry will be nil.
	Errors []error

	// TotalTasks is the total number of tasks submitted.
	TotalTasks int

	// SuccessfulTasks is the number of tasks that completed successfully.
	SuccessfulTasks int

	// FailedTasks is the number of tasks that failed.
	FailedTasks int
}

// GetSuccessfulResults returns only the successful results.
func (pr *PoolResult) GetSuccessfulResults() []*TaskResult {
	var results []*TaskResult
	for _, r := range pr.Results {
		if r != nil {
			results = append(results, r)
		}
	}
	return results
}

// GetFailedIndices returns the indices of failed tasks.
func (pr *PoolResult) GetFailedIndices() []int {
	var indices []int
	for i, err := range pr.Errors {
		if err != nil {
			indices = append(indices, i)
		}
	}
	return indices
}

// Execute executes a batch of TaskCards concurrently.
// The results are returned in the same order as the input tasks.
// Individual task failures do not affect other tasks unless StopOnFirstError is set.
func (wp *WorkerPool) Execute(ctx context.Context, cards []*TaskCard) *PoolResult {
	if len(cards) == 0 {
		return &PoolResult{
			Results:    []*TaskResult{},
			Errors:     []error{},
			TotalTasks: 0,
		}
	}

	results := make([]*TaskResult, len(cards))
	errors := make([]error, len(cards))

	// Use a semaphore to limit concurrency
	sem := make(chan struct{}, wp.maxConcurrency)

	// Track success/failure counts atomically
	var successfulCount int32
	var failedCount int32

	// Context cancellation for early termination
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to signal early termination
	stopCh := make(chan struct{})
	var stopped atomic.Bool

	// WaitGroup for coordinating goroutines
	var wg sync.WaitGroup

	for i, card := range cards {
		// Check if we should stop early
		if stopped.Load() {
			errors[i] = fmt.Errorf("execution stopped due to previous error")
			failedCount++
			continue
		}

		wg.Add(1)

		go func(index int, task *TaskCard) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errors[index] = ctx.Err()
				atomic.AddInt32(&failedCount, 1)
				return
			case <-stopCh:
				errors[index] = fmt.Errorf("execution stopped")
				atomic.AddInt32(&failedCount, 1)
				return
			}

			// Check again after acquiring semaphore
			if stopped.Load() {
				errors[index] = fmt.Errorf("execution stopped due to previous error")
				atomic.AddInt32(&failedCount, 1)
				return
			}

			// Execute the task
			result, err := wp.executeTask(ctx, task)

			if err != nil {
				errors[index] = err
				atomic.AddInt32(&failedCount, 1)

				// Signal early termination if configured
				if wp.stopOnFirstError {
					if stopped.CompareAndSwap(false, true) {
						close(stopCh)
						cancel()
					}
				}
				return
			}

			results[index] = result
			atomic.AddInt32(&successfulCount, 1)
		}(i, card)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return &PoolResult{
		Results:         results,
		Errors:          errors,
		TotalTasks:      len(cards),
		SuccessfulTasks: int(successfulCount),
		FailedTasks:     int(failedCount),
	}
}

// ExecuteWithCallback executes tasks and calls the callback for each completed task.
// This is useful for progress reporting or streaming results.
func (wp *WorkerPool) ExecuteWithCallback(
	ctx context.Context,
	cards []*TaskCard,
	callback func(index int, result *TaskResult, err error),
) *PoolResult {
	if len(cards) == 0 {
		return &PoolResult{
			Results:    []*TaskResult{},
			Errors:     []error{},
			TotalTasks: 0,
		}
	}

	results := make([]*TaskResult, len(cards))
	errors := make([]error, len(cards))

	sem := make(chan struct{}, wp.maxConcurrency)
	var successfulCount int32
	var failedCount int32

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stopCh := make(chan struct{})
	var stopped atomic.Bool

	var wg sync.WaitGroup

	for i, card := range cards {
		if stopped.Load() {
			errors[i] = fmt.Errorf("execution stopped due to previous error")
			failedCount++
			if callback != nil {
				callback(i, nil, errors[i])
			}
			continue
		}

		wg.Add(1)

		go func(index int, task *TaskCard) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errors[index] = ctx.Err()
				atomic.AddInt32(&failedCount, 1)
				if callback != nil {
					callback(index, nil, errors[index])
				}
				return
			case <-stopCh:
				errors[index] = fmt.Errorf("execution stopped")
				atomic.AddInt32(&failedCount, 1)
				if callback != nil {
					callback(index, nil, errors[index])
				}
				return
			}

			if stopped.Load() {
				errors[index] = fmt.Errorf("execution stopped due to previous error")
				atomic.AddInt32(&failedCount, 1)
				if callback != nil {
					callback(index, nil, errors[index])
				}
				return
			}

			result, err := wp.executeTask(ctx, task)

			if err != nil {
				errors[index] = err
				atomic.AddInt32(&failedCount, 1)

				if wp.stopOnFirstError {
					if stopped.CompareAndSwap(false, true) {
						close(stopCh)
						cancel()
					}
				}

				if callback != nil {
					callback(index, nil, err)
				}
				return
			}

			results[index] = result
			atomic.AddInt32(&successfulCount, 1)

			if callback != nil {
				callback(index, result, nil)
			}
		}(i, card)
	}

	wg.Wait()

	return &PoolResult{
		Results:         results,
		Errors:          errors,
		TotalTasks:      len(cards),
		SuccessfulTasks: int(successfulCount),
		FailedTasks:     int(failedCount),
	}
}

// executeTask executes a single task with the adapter.
// If no adapter is configured, it returns a mock result.
func (wp *WorkerPool) executeTask(ctx context.Context, card *TaskCard) (*TaskResult, error) {
	if wp.adapter == nil {
		// Return a default result if no adapter is configured
		return &TaskResult{
			Type:       card.Type,
			Confidence: 0.5,
			Reason:     "No adapter configured",
		}, nil
	}

	return wp.adapter.Execute(ctx, card)
}

// SetAdapter sets the provider adapter.
func (wp *WorkerPool) SetAdapter(adapter ProviderAdapter) {
	wp.adapter = adapter
}

// GetMaxConcurrency returns the maximum concurrency.
func (wp *WorkerPool) GetMaxConcurrency() int {
	return wp.maxConcurrency
}

// SetMaxConcurrency sets the maximum concurrency.
func (wp *WorkerPool) SetMaxConcurrency(n int) {
	if n > 0 {
		wp.maxConcurrency = n
	}
}

// IsAvailable returns true if the worker pool can accept tasks.
func (wp *WorkerPool) IsAvailable() bool {
	return wp.adapter == nil || wp.adapter.IsAvailable()
}

// Close closes the underlying adapter if it exists.
func (wp *WorkerPool) Close() error {
	if wp.adapter != nil {
		return wp.adapter.Close()
	}
	return nil
}

// ExecuteSequential executes tasks one at a time without concurrency.
// This is useful for debugging or when concurrency is not desired.
func (wp *WorkerPool) ExecuteSequential(ctx context.Context, cards []*TaskCard) *PoolResult {
	if len(cards) == 0 {
		return &PoolResult{
			Results:    []*TaskResult{},
			Errors:     []error{},
			TotalTasks: 0,
		}
	}

	results := make([]*TaskResult, len(cards))
	errors := make([]error, len(cards))

	var successfulCount, failedCount int

	for i, card := range cards {
		select {
		case <-ctx.Done():
			errors[i] = ctx.Err()
			failedCount++
			continue
		default:
		}

		result, err := wp.executeTask(ctx, card)
		if err != nil {
			errors[i] = err
			failedCount++

			if wp.stopOnFirstError {
				// Fill remaining errors
				for j := i + 1; j < len(cards); j++ {
					errors[j] = fmt.Errorf("execution stopped due to previous error")
					failedCount++
				}
				break
			}
			continue
		}

		results[i] = result
		successfulCount++
	}

	return &PoolResult{
		Results:         results,
		Errors:          errors,
		TotalTasks:      len(cards),
		SuccessfulTasks: successfulCount,
		FailedTasks:     failedCount,
	}
}
