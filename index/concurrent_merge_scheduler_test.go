// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for concurrent merge scheduler.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestConcurrentMergeScheduler
// and related test files:
//   - TestConcurrentMergeScheduler.java
//
// GC-117: Index Tests - Concurrent Operations
package index_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestConcurrentMergeScheduler tests the ConcurrentMergeScheduler implementation.
// Ported from: TestConcurrentMergeScheduler.java
func TestConcurrentMergeScheduler(t *testing.T) {
	t.Run("new concurrent merge scheduler", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		if scheduler == nil {
			t.Fatal("NewConcurrentMergeScheduler() returned nil")
		}
		defer scheduler.Close()

		// Check defaults
		if scheduler.MaxThreadCount() != 1 {
			t.Errorf("MaxThreadCount() = %d, want 1", scheduler.MaxThreadCount())
		}
		if scheduler.MaxMergeCount() != 5 {
			t.Errorf("MaxMergeCount() = %d, want 5", scheduler.MaxMergeCount())
		}
	})

	t.Run("set and get max thread count", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		scheduler.SetMaxThreadCount(4)
		if scheduler.MaxThreadCount() != 4 {
			t.Errorf("MaxThreadCount() = %d, want 4", scheduler.MaxThreadCount())
		}
	})

	t.Run("set max thread count to auto", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		// Set to 0 for auto
		scheduler.SetMaxThreadCount(0)
		if scheduler.MaxThreadCount() != 0 {
			t.Errorf("MaxThreadCount() = %d, want 0", scheduler.MaxThreadCount())
		}
	})

	t.Run("set and get max merge count", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		scheduler.SetMaxMergeCount(10)
		if scheduler.MaxMergeCount() != 10 {
			t.Errorf("MaxMergeCount() = %d, want 10", scheduler.MaxMergeCount())
		}
	})

	t.Run("max merge count minimum", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		// Setting below 1 should clamp to 1
		scheduler.SetMaxMergeCount(0)
		if scheduler.MaxMergeCount() != 1 {
			t.Errorf("MaxMergeCount() = %d, want 1 (minimum)", scheduler.MaxMergeCount())
		}
	})

	t.Run("string representation", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		s := scheduler.String()
		if s == "" {
			t.Error("String() returned empty string")
		}
		t.Logf("ConcurrentMergeScheduler string: %s", s)
	})
}

// TestConcurrentMergeSchedulerClose tests scheduler shutdown.
func TestConcurrentMergeSchedulerClose(t *testing.T) {
	t.Run("close without merges", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()

		err := scheduler.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("close twice", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()

		scheduler.Close()
		err := scheduler.Close()
		if err != nil {
			t.Errorf("Close() second time error = %v", err)
		}
	})

	t.Run("close with timeout", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := scheduler.CloseWithContext(ctx)
		if err != nil {
			t.Errorf("CloseWithContext() error = %v", err)
		}
	})
}

// TestConcurrentMergeSchedulerAsync tests asynchronous merge operations.
func TestConcurrentMergeSchedulerAsync(t *testing.T) {
	t.Run("merge async", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		// Create a test merge
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		// Try async merge (will fail with nil writer but tests the path)
		// In a full implementation, this would queue and execute the merge
		_ = merge
		t.Skip("Full merge async test requires complete IndexWriter implementation")
	})

	t.Run("merge async when closed", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		scheduler.Close()

		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		// Should return error when scheduler is closed
		_ = merge
		t.Skip("Full merge async test requires complete IndexWriter implementation")
	})
}

// TestConcurrentMergeSchedulerConcurrency tests concurrent access.
func TestConcurrentMergeSchedulerConcurrency(t *testing.T) {
	t.Run("concurrent configuration access", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		var wg sync.WaitGroup
		numGoroutines := 10

		// Concurrent readers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = scheduler.MaxThreadCount()
				_ = scheduler.MaxMergeCount()
				_ = scheduler.GetRunningMergeCount()
				_ = scheduler.GetPendingMergeCount()
			}()
		}

		// Concurrent writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				scheduler.SetMaxThreadCount(id % 5)
				scheduler.SetMaxMergeCount((id % 5) + 1)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent string calls", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		var wg sync.WaitGroup
		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = scheduler.String()
			}()
		}

		wg.Wait()
	})
}

// TestMergeProgress tests merge progress tracking.
func TestMergeProgress(t *testing.T) {
	t.Run("new merge progress", func(t *testing.T) {
		progress := index.NewMergeProgress(100)
		if progress == nil {
			t.Fatal("NewMergeProgress() returned nil")
		}

		if progress.TotalDocs != 100 {
			t.Errorf("TotalDocs = %d, want 100", progress.TotalDocs)
		}
		if progress.MergedDocs != 0 {
			t.Errorf("MergedDocs = %d, want 0", progress.MergedDocs)
		}
		if progress.IsAborted {
			t.Error("IsAborted should be false")
		}
	})

	t.Run("get progress percentage", func(t *testing.T) {
		progress := index.NewMergeProgress(100)

		// Initially 0%
		pct := progress.GetProgress()
		if pct != 0.0 {
			t.Errorf("GetProgress() = %f, want 0.0", pct)
		}

		// Set to 50
		progress.SetProgress(50)
		pct = progress.GetProgress()
		if pct != 50.0 {
			t.Errorf("GetProgress() = %f, want 50.0", pct)
		}

		// Complete
		progress.SetProgress(100)
		pct = progress.GetProgress()
		if pct != 100.0 {
			t.Errorf("GetProgress() = %f, want 100.0", pct)
		}
	})

	t.Run("increment progress", func(t *testing.T) {
		progress := index.NewMergeProgress(100)

		progress.IncrementProgress(25)
		if progress.MergedDocs != 25 {
			t.Errorf("MergedDocs = %d, want 25", progress.MergedDocs)
		}

		progress.IncrementProgress(25)
		if progress.MergedDocs != 50 {
			t.Errorf("MergedDocs = %d, want 50", progress.MergedDocs)
		}
	})

	t.Run("progress with zero total docs", func(t *testing.T) {
		progress := index.NewMergeProgress(0)

		// With 0 total docs, progress should be 100%
		pct := progress.GetProgress()
		if pct != 100.0 {
			t.Errorf("GetProgress() = %f, want 100.0", pct)
		}
	})

	t.Run("abort merge", func(t *testing.T) {
		progress := index.NewMergeProgress(100)

		progress.Abort()
		if !progress.IsAborted {
			t.Error("IsAborted should be true after Abort()")
		}
	})

	t.Run("concurrent progress access", func(t *testing.T) {
		progress := index.NewMergeProgress(1000)

		var wg sync.WaitGroup
		numGoroutines := 10

		// Concurrent readers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = progress.GetProgress()
			}()
		}

		// Concurrent writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				progress.IncrementProgress(id)
			}(i)
		}

		wg.Wait()

		// Verify total progress
		pct := progress.GetProgress()
		if pct < 0 || pct > 100 {
			t.Errorf("GetProgress() = %f, out of range [0, 100]", pct)
		}
	})
}

// TestMergeSchedulerWithIndexWriter tests integration with IndexWriter.
func TestMergeSchedulerWithIndexWriter(t *testing.T) {
	t.Run("set concurrent merge scheduler in config", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		// In a full implementation, this would be:
		// config := index.NewIndexWriterConfig(createTestAnalyzer())
		// config.SetMergeScheduler(scheduler)
		// writer, _ := index.NewIndexWriter(dir, config)
		t.Skip("Full integration test requires IndexWriter to use MergeScheduler")
	})
}

// TestConcurrentMergeSchedulerStress tests scheduler under load.
func TestConcurrentMergeSchedulerStress(t *testing.T) {
	t.Run("rapid open close cycles", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			scheduler := index.NewConcurrentMergeScheduler()
			err := scheduler.Close()
			if err != nil {
				t.Fatalf("Close() error at iteration %d: %v", i, err)
			}
		}
	})

	t.Run("configuration changes under load", func(t *testing.T) {
		scheduler := index.NewConcurrentMergeScheduler()
		defer scheduler.Close()

		var wg sync.WaitGroup
		numIterations := 50
		numGoroutines := 5

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					scheduler.SetMaxThreadCount((j % 4) + 1)
					_ = scheduler.MaxThreadCount()
					scheduler.SetMaxMergeCount((j % 5) + 1)
					_ = scheduler.MaxMergeCount()
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestConcurrentMergeScheduler_DynamicLimits tests changing limits at runtime.
func TestConcurrentMergeScheduler_DynamicLimits(t *testing.T) {
	scheduler := index.NewConcurrentMergeScheduler()
	defer scheduler.Close()

	// Initial limits
	scheduler.SetMaxThreadCount(2)
	scheduler.SetMaxMergeCount(4)

	if scheduler.MaxThreadCount() != 2 {
		t.Errorf("MaxThreadCount() = %d, want 2", scheduler.MaxThreadCount())
	}
	if scheduler.MaxMergeCount() != 4 {
		t.Errorf("MaxMergeCount() = %d, want 4", scheduler.MaxMergeCount())
	}

	// Change limits
	scheduler.SetMaxThreadCount(1)
	scheduler.SetMaxMergeCount(2)

	if scheduler.MaxThreadCount() != 1 {
		t.Errorf("MaxThreadCount() = %d, want 1", scheduler.MaxThreadCount())
	}
	if scheduler.MaxMergeCount() != 2 {
		t.Errorf("MaxMergeCount() = %d, want 2", scheduler.MaxMergeCount())
	}
}

// TestConcurrentMergeScheduler_MergeStalling tests that stalling logic is triggered.
func TestConcurrentMergeScheduler_MergeStalling(t *testing.T) {
	// This would require a way to simulate many merges and check if indexing stalls
	t.Skip("Merge stalling test requires more complete implementation of CMS and IndexWriter")
}
