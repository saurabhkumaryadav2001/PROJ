package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// waitFor simulates cancellable I/O or CPU-adjacent waiting. Real database and
// HTTP calls should use their context-aware APIs instead of sleeping.
func waitFor(ctx context.Context, duration time.Duration) error {
	if duration < 0 {
		return errors.New("duration cannot be negative")
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Task struct {
	ID       string
	Duration time.Duration
}

type TaskResult struct {
	ID          string    `json:"id"`
	CompletedAt time.Time `json:"completed_at"`
}

// RunTasks executes tasks with a bounded number of workers. Cancellation is
// cooperative: every worker observes workCtx.Done() and exits.
func RunTasks(ctx context.Context, tasks []Task, workerCount int) ([]TaskResult, error) {
	if workerCount < 1 {
		return nil, errors.New("worker count must be at least one")
	}
	for _, task := range tasks {
		if task.ID == "" {
			return nil, errors.New("task ID is required")
		}
		if task.Duration < 0 {
			return nil, fmt.Errorf("task %q has a negative duration", task.ID)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return []TaskResult{}, nil
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Task)
	results := make(chan TaskResult, len(tasks))
	workerCount = min(workerCount, len(tasks))

	var workers sync.WaitGroup
	workers.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-workCtx.Done():
					return
				case task, ok := <-jobs:
					if !ok {
						return
					}
					if err := waitFor(workCtx, task.Duration); err != nil {
						return
					}
					result := TaskResult{ID: task.ID, CompletedAt: time.Now().UTC()}
					select {
					case results <- result:
					case <-workCtx.Done():
						return
					}
				}
			}
		}()
	}

sendLoop:
	for _, task := range tasks {
		select {
		case <-workCtx.Done():
			break sendLoop
		case jobs <- task:
		}
	}
	close(jobs)

	workers.Wait()
	close(results)

	completed := make([]TaskResult, 0, len(tasks))
	for result := range results {
		completed = append(completed, result)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := workCtx.Err(); err != nil {
		return nil, err
	}
	return completed, nil
}
