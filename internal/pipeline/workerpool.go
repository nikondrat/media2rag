package pipeline

import (
	"context"
	"sync"
)

type WorkerPool[T any] struct {
	NumWorkers int
	ProcessFn  func(ctx context.Context, item T) error
}

func (wp *WorkerPool[T]) Run(ctx context.Context, items []T) error {
	if wp.NumWorkers <= 0 {
		wp.NumWorkers = 3
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	jobs := make(chan T, len(items))

	for w := 0; w < wp.NumWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := wp.ProcessFn(ctx, item); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					mu.Unlock()
					return
				}
			}
		}()
	}

	for _, item := range items {
		jobs <- item
	}
	close(jobs)
	wg.Wait()

	return firstErr
}
