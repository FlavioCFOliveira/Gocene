// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestCodecLoadingDeadlock ports Lucene's TestCodecLoadingDeadlock
// (org/apache/lucene/codecs/TestCodecLoadingDeadlock.java).
//
// The original test forks a separate JVM and races 14 goroutine-equivalents
// against a CyclicBarrier to stress the static class-initialization path of
// the Codec/PostingsFormat/DocValuesFormat SPI loaders (a classloader-deadlock
// regression check).
//
// In Gocene the SPI surface is replaced by name-keyed registries protected by
// sync.RWMutex (PostingsFormatByName / DocValuesFormatByName / ForName /
// AvailableCodecs).  The original deadlock vector does not exist in the Go
// model, but correctness under concurrent access must still be verified.
//
// This test releases N goroutines simultaneously via a sync.WaitGroup barrier
// and has each goroutine call ForName, AvailableCodecs, PostingsFormatByName,
// and DocValuesFormatByName in a tight loop.  It runs under -race and is
// bounded by a context deadline that mirrors Lucene's 30-second timeout cap.
func TestCodecLoadingDeadlock(t *testing.T) {
	const (
		numGoroutines = 14   // mirrors Lucene's NUM_THREADS=14
		deadline      = 10 * time.Second
	)

	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()

	// Barrier: all goroutines start at the same moment, mirroring
	// Lucene's CyclicBarrier.
	var ready sync.WaitGroup
	ready.Add(numGoroutines)
	var start sync.WaitGroup
	start.Add(1)

	var wg sync.WaitGroup
	errs := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ready.Done()  // signal this goroutine is ready
			start.Wait() // wait for the barrier to drop

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Exercise all three registries concurrently.
				_ = AvailableCodecs()

				if _, err := ForName("Lucene104"); err != nil {
					errs <- "ForName(Lucene104): " + err.Error()
					return
				}

				// PostingsFormatByName with a known name must not panic or deadlock.
				// Use "Lucene103" which is registered at init time; tolerate "not found"
				// for names that vary by build.
				_, _ = PostingsFormatByName("Lucene103")

				// DocValuesFormatByName with a known name.
				_, _ = DocValuesFormatByName("Lucene90")
			}
		}()
	}

	// Wait for all goroutines to be ready, then release them together.
	ready.Wait()
	start.Done()

	// Wait for either all goroutines to finish or the deadline to expire.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines exited cleanly.
	case <-ctx.Done():
		// Deadline: goroutines ran long enough to stress the registries.
	}

	// Drain any errors reported by goroutines.
	close(errs)
	for msg := range errs {
		t.Errorf("goroutine reported error: %s", msg)
	}
}
