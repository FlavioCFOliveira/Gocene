// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// ExceptionalCallback is a callback that can return an error.
// Mirrors SuggestRebuildTestUtil.ExceptionalCallback.
type ExceptionalCallback func(suggester suggest.Lookup) error

// CheckLookupsDuringReBuild confirms that a Lookup can serve suggestions
// during a slow rebuild by interleaving reads with each step of an
// iterator-driven build. Mirrors
// SuggestRebuildTestUtil.testLookupsDuringReBuild.
//
// The function:
//  1. Builds the suggester from initialData and runs initialChecks.
//  2. Starts a background goroutine that rebuilds from initialData+extraData
//     but pauses before each iterator step.
//  3. At each pause point it calls initialChecks to confirm old results
//     are still served.
//  4. After the rebuild completes it calls finalChecks.
func CheckLookupsDuringReBuild(
	t *testing.T,
	suggester suggest.Lookup,
	initialData []*Input,
	initialChecks ExceptionalCallback,
	extraData []*Input,
	finalChecks ExceptionalCallback,
) {
	t.Helper()

	// build initial state
	if err := suggester.Build(NewInputArrayIterator(initialData)); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if err := initialChecks(suggester); err != nil {
		t.Fatalf("initial checks: %v", err)
	}

	// combine initial + extra data for the rebuild
	allData := make([]*Input, 0, len(initialData)+len(extraData))
	allData = append(allData, initialData...)
	allData = append(allData, extraData...)

	readyCh := make(chan struct{}, len(allData)+1)
	advanceCh := make(chan struct{}, len(allData)+1)
	var buildErr atomic.Value

	// delayed iterator: pauses before returning each entry
	delayed := &delayedInputIterator{
		inner:     NewInputArrayIterator(allData),
		readyCh:   readyCh,
		advanceCh: advanceCh,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := suggester.Build(delayed); err != nil {
			buildErr.Store(err)
			// unblock waiting goroutines on error
			for range allData {
				readyCh <- struct{}{}
			}
		}
	}()

	// +1 iteration for the final nil from the iterator
	for i := 0; i < len(allData)+1; i++ {
		<-readyCh
		if v := buildErr.Load(); v != nil {
			t.Fatalf("rebuild error: %v", v)
		}
		if err := initialChecks(suggester); err != nil {
			t.Errorf("check during rebuild (step %d): %v", i, err)
		}
		advanceCh <- struct{}{}
	}

	wg.Wait()
	if v := buildErr.Load(); v != nil {
		t.Fatalf("rebuild error after join: %v", v)
	}
	if err := finalChecks(suggester); err != nil {
		t.Errorf("final checks: %v", err)
	}
}

// delayedInputIterator wraps an InputIterator and signals on readyCh before
// each call to Next, waiting for advanceCh before proceeding. Mirrors
// SuggestRebuildTestUtil.DelayedInputIterator.
type delayedInputIterator struct {
	inner     suggest.InputIterator
	readyCh   chan<- struct{}
	advanceCh <-chan struct{}
}

func (d *delayedInputIterator) Next() ([]byte, int64, []byte, [][]byte, bool, error) {
	d.readyCh <- struct{}{}
	<-d.advanceCh
	return d.inner.Next()
}

func (d *delayedInputIterator) HasPayloads() bool { return d.inner.HasPayloads() }
func (d *delayedInputIterator) HasContexts() bool { return d.inner.HasContexts() }

var _ suggest.InputIterator = (*delayedInputIterator)(nil)
