// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestSerialMergeScheduler tests the SerialMergeScheduler implementation.
func TestSerialMergeScheduler(t *testing.T) {
	t.Run("new serial merge scheduler", func(t *testing.T) {
		scheduler := index.NewSerialMergeScheduler()
		if scheduler == nil {
			t.Fatal("NewSerialMergeScheduler() returned nil")
		}
		defer scheduler.Close()

		// SerialMergeScheduler runs merges synchronously, so running count should always be 0
		if scheduler.GetRunningMergeCount() != 0 {
			t.Errorf("GetRunningMergeCount() = %d, want 0", scheduler.GetRunningMergeCount())
		}
	})

	t.Run("close", func(t *testing.T) {
		scheduler := index.NewSerialMergeScheduler()
		err := scheduler.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Close again should be safe
		err = scheduler.Close()
		if err != nil {
			t.Errorf("Close() second time error = %v", err)
		}
	})
}

// TestOneMergeProgress tests the OneMergeProgress implementation.
func TestOneMergeProgress(t *testing.T) {
	t.Run("new progress", func(t *testing.T) {
		progress := index.NewOneMergeProgress()
		if progress == nil {
			t.Fatal("NewOneMergeProgress() returned nil")
		}

		if progress.IsAborted() {
			t.Error("IsAborted() = true, want false")
		}
	})

	t.Run("abort", func(t *testing.T) {
		progress := index.NewOneMergeProgress()

		if progress.IsAborted() {
			t.Error("IsAborted() = true before abort")
		}

		progress.Abort()

		if !progress.IsAborted() {
			t.Error("IsAborted() = false after abort")
		}
	})

	t.Run("check aborted", func(t *testing.T) {
		progress := index.NewOneMergeProgress()

		// Not aborted, should return nil
		err := progress.CheckAborted()
		if err != nil {
			t.Errorf("CheckAborted() error = %v, want nil", err)
		}

		// Abort and check again
		progress.Abort()
		err = progress.CheckAborted()
		if err == nil {
			t.Error("CheckAborted() error = nil, want error after abort")
		}
	})

	t.Run("wakeup", func(t *testing.T) {
		progress := index.NewOneMergeProgress()

		// Wakeup should be safe to call
		progress.Wakeup()
		progress.Wakeup()
	})

	t.Run("pause times", func(t *testing.T) {
		progress := index.NewOneMergeProgress()

		pauseTimes := progress.GetPauseTimes()
		if pauseTimes == nil {
			t.Error("GetPauseTimes() = nil, want map")
		}
	})
}

// TestOneMergeWithProgress tests the OneMerge with progress.
func TestOneMergeWithProgress(t *testing.T) {
	t.Run("new merge with progress", func(t *testing.T) {
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		if merge == nil {
			t.Fatal("NewOneMerge() returned nil")
		}

		if merge.Progress == nil {
			t.Error("Progress = nil, want non-nil")
		}
	})

	t.Run("abort merge", func(t *testing.T) {
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		if merge.IsAborted() {
			t.Error("IsAborted() = true before abort")
		}

		merge.Abort()

		if !merge.IsAborted() {
			t.Error("IsAborted() = false after abort")
		}
	})

	t.Run("check aborted", func(t *testing.T) {
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		// Not aborted, should return nil
		err := merge.CheckAborted()
		if err != nil {
			t.Errorf("CheckAborted() error = %v, want nil", err)
		}

		// Abort and check again
		merge.Abort()
		err = merge.CheckAborted()
		if err == nil {
			t.Error("CheckAborted() error = nil, want error after abort")
		}
	})

	t.Run("get progress", func(t *testing.T) {
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		progress := merge.GetProgress()
		if progress == nil {
			t.Error("GetProgress() = nil, want non-nil")
		}
	})
}

// TestNewMergeTriggers tests the new MergeTrigger enum values.
func TestNewMergeTriggers(t *testing.T) {
	t.Run("all trigger values", func(t *testing.T) {
		triggers := []index.MergeTrigger{
			index.SEGMENT_FLUSH,
			index.FULL_FLUSH,
			index.EXPLICIT,
			index.MERGE_FINISHED,
			index.CLOSING,
			index.COMMIT,
			index.GET_READER,
			index.ADD_INDEXES,
			index.CLOSED_WRITER,
		}

		for _, trigger := range triggers {
			s := trigger.String()
			if s == "" {
				t.Errorf("String() for trigger %d returned empty string", trigger)
			}
			t.Logf("MergeTrigger: %s", s)
		}
	})

	t.Run("new triggers", func(t *testing.T) {
		// Test the new triggers
		if index.ADD_INDEXES.String() != "ADD_INDEXES" {
			t.Errorf("ADD_INDEXES.String() = %s, want ADD_INDEXES", index.ADD_INDEXES.String())
		}
		if index.CLOSING.String() != "CLOSING" {
			t.Errorf("CLOSING.String() = %s, want CLOSING", index.CLOSING.String())
		}
	})
}

// TestMergeSource tests the MergeSource interface.
func TestMergeSource(t *testing.T) {
	// This test verifies that the interface is correctly defined
	// A mock implementation would be needed for full testing
	t.Run("interface exists", func(t *testing.T) {
		// The interface is defined in merge_scheduler.go
		// This is a compile-time check that the interface exists
		var _ index.MergeSource = (*mockMergeSource)(nil)
	})
}

// mockMergeSource is a mock implementation of MergeSource for testing.
type mockMergeSource struct {
	merges     []*index.OneMerge
	finished   []*index.OneMerge
	mergeError error
	mergeCount int
}

func newMockMergeSource(merges []*index.OneMerge) *mockMergeSource {
	return &mockMergeSource{
		merges: merges,
	}
}

func (m *mockMergeSource) GetNextMerge() *index.OneMerge {
	if len(m.merges) == 0 {
		return nil
	}
	merge := m.merges[0]
	m.merges = m.merges[1:]
	return merge
}

func (m *mockMergeSource) OnMergeFinished(merge *index.OneMerge) {
	m.finished = append(m.finished, merge)
}

func (m *mockMergeSource) HasPendingMerges() bool {
	return len(m.merges) > 0
}

func (m *mockMergeSource) Merge(merge *index.OneMerge) error {
	m.mergeCount++
	return m.mergeError
}

// TestSerialMergeSchedulerWithMockSource tests SerialMergeScheduler with a mock source.
func TestSerialMergeSchedulerWithMockSource(t *testing.T) {
	t.Run("merge with empty source", func(t *testing.T) {
		scheduler := index.NewSerialMergeScheduler()
		defer scheduler.Close()

		source := newMockMergeSource(nil)
		err := scheduler.Merge(source, index.EXPLICIT)
		if err != nil {
			t.Errorf("Merge() error = %v, want nil", err)
		}
	})

	t.Run("merge with multiple merges", func(t *testing.T) {
		scheduler := index.NewSerialMergeScheduler()
		defer scheduler.Close()

		// Create multiple merges
		merge1 := index.NewOneMerge([]*index.SegmentCommitInfo{})
		merge2 := index.NewOneMerge([]*index.SegmentCommitInfo{})
		merge3 := index.NewOneMerge([]*index.SegmentCommitInfo{})

		source := newMockMergeSource([]*index.OneMerge{merge1, merge2, merge3})
		err := scheduler.Merge(source, index.EXPLICIT)
		if err != nil {
			t.Errorf("Merge() error = %v, want nil", err)
		}

		// Verify all merges were processed
		if source.mergeCount != 3 {
			t.Errorf("source.mergeCount = %d, want 3", source.mergeCount)
		}
		if len(source.finished) != 3 {
			t.Errorf("len(source.finished) = %d, want 3", len(source.finished))
		}
	})
}

// TestMergeThread tests the MergeThread implementation.
func TestMergeThread(t *testing.T) {
	t.Run("new merge thread", func(t *testing.T) {
		merge := index.NewOneMerge([]*index.SegmentCommitInfo{})
		thread := index.NewMergeThread("test-thread", merge)

		if thread == nil {
			t.Fatal("NewMergeThread() returned nil")
		}

		if thread.Name != "test-thread" {
			t.Errorf("Name = %s, want test-thread", thread.Name)
		}

		if thread.IsRunning() {
			t.Error("IsRunning() = true, want false")
		}
	})

	t.Run("set running", func(t *testing.T) {
		merge := index.NewOneMerge([]*index.SegmentCommitInfo{})
		thread := index.NewMergeThread("test-thread", merge)

		thread.SetRunning(true)
		if !thread.IsRunning() {
			t.Error("IsRunning() = false, want true")
		}

		thread.SetRunning(false)
		if thread.IsRunning() {
			t.Error("IsRunning() = true, want false")
		}
	})

	t.Run("done channel", func(t *testing.T) {
		merge := index.NewOneMerge([]*index.SegmentCommitInfo{})
		thread := index.NewMergeThread("test-thread", merge)

		// Done channel should be created
		if thread.Done() == nil {
			t.Error("Done() = nil, want channel")
		}
	})
}

// TestMergeRateLimiter tests the MergeRateLimiter implementation.
func TestMergeRateLimiter(t *testing.T) {
	t.Run("new rate limiter", func(t *testing.T) {
		limiter := index.NewMergeRateLimiter()
		if limiter == nil {
			t.Fatal("NewMergeRateLimiter() returned nil")
		}

		// Default rate should be 20 MB/s
		if limiter.GetMBPerSec() != 20.0 {
			t.Errorf("GetMBPerSec() = %f, want 20.0", limiter.GetMBPerSec())
		}
	})

	t.Run("set rate", func(t *testing.T) {
		limiter := index.NewMergeRateLimiter()

		limiter.SetMBPerSec(50.0)
		if limiter.GetMBPerSec() != 50.0 {
			t.Errorf("GetMBPerSec() = %f, want 50.0", limiter.GetMBPerSec())
		}
	})

	t.Run("pause with zero rate", func(t *testing.T) {
		limiter := index.NewMergeRateLimiter()
		limiter.SetMBPerSec(0) // No throttling

		progress := index.NewOneMergeProgress()
		err := limiter.Pause(1024*1024, progress) // 1MB
		if err != nil {
			t.Errorf("Pause() error = %v, want nil", err)
		}
	})

	t.Run("total bytes written", func(t *testing.T) {
		limiter := index.NewMergeRateLimiter()

		if limiter.TotalBytesWritten() != 0 {
			t.Errorf("TotalBytesWritten() = %d, want 0", limiter.TotalBytesWritten())
		}
	})
}

// TestPauseReason tests the PauseReason enum.
func TestPauseReason(t *testing.T) {
	// Test that the constants exist
	_ = index.STOPPED
	_ = index.PAUSED
	_ = index.OTHER
}