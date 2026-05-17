// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"context"
	"sync"
)

// TaskExecutor is the executor wrapper used by IndexSearcher to parallelize
// search across segments.
//
// Mirrors org.apache.lucene.search.TaskExecutor.
type TaskExecutor struct {
	dispatch func(func()) // submit a task for asynchronous execution; nil means caller-goroutine.
}

// NewTaskExecutor creates a TaskExecutor backed by a dispatcher function.
// Pass nil to run all tasks on the caller goroutine.
func NewTaskExecutor(dispatch func(func())) *TaskExecutor {
	return &TaskExecutor{dispatch: dispatch}
}

// Callable is a unit of work that produces a value of type R.
type Callable[R any] func(ctx context.Context) (R, error)

// InvokeAll runs all callables. If a dispatcher is configured, count-1 tasks
// are dispatched asynchronously and the last one runs on the caller goroutine.
// On the first error, all in-flight tasks observe a cancelled context and
// InvokeAll returns the first error.
func InvokeAll[R any](te *TaskExecutor, ctx context.Context, callables []Callable[R]) ([]R, error) {
	n := len(callables)
	out := make([]R, n)
	if n == 0 {
		return out, nil
	}
	if te == nil || te.dispatch == nil || n == 1 {
		for i, c := range callables {
			v, err := c(ctx)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	report := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	wg.Add(n - 1)
	for i := 0; i < n-1; i++ {
		i, c := i, callables[i]
		te.dispatch(func() {
			defer wg.Done()
			if subCtx.Err() != nil {
				return
			}
			v, err := c(subCtx)
			if err != nil {
				report(err)
				return
			}
			out[i] = v
		})
	}

	// Run the last task on the caller goroutine to keep it busy.
	if subCtx.Err() == nil {
		v, err := callables[n-1](subCtx)
		if err != nil {
			report(err)
		} else {
			out[n-1] = v
		}
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}
