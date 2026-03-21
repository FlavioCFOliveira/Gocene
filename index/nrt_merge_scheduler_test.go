// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
	"time"
)

// mockMergeSource is a mock implementation of MergeSource for testing
type mockMergeSource struct {
	merges       []*OneMerge
	mergeIndex   int
	mergeCalled  bool
	finishCalled bool
	hasPending   bool
}

func (m *mockMergeSource) GetNextMerge() *OneMerge {
	if m.mergeIndex < len(m.merges) {
		merge := m.merges[m.mergeIndex]
		m.mergeIndex++
		return merge
	}
	return nil
}

func (m *mockMergeSource) OnMergeFinished(merge *OneMerge) {
	m.finishCalled = true
}

func (m *mockMergeSource) HasPendingMerges() bool {
	return m.hasPending || m.mergeIndex < len(m.merges)
}

func (m *mockMergeSource) Merge(merge *OneMerge) error {
	m.mergeCalled = true
	return nil
}

func TestNewNRTMergeScheduler(t *testing.T) {
	tests := []struct {
		name      string
		wrapped   MergeScheduler
		wantErr   bool
		wantMaxMB float64
		wantPause bool
	}{
		{
			name:      "with nil wrapped scheduler",
			wrapped:   nil,
			wantErr:   false,
			wantMaxMB: 100.0,
			wantPause: true,
		},
		{
			name:      "with ConcurrentMergeScheduler",
			wrapped:   NewConcurrentMergeScheduler(),
			wantErr:   false,
			wantMaxMB: 100.0,
			wantPause: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler, err := NewNRTMergeScheduler(tt.wrapped)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNRTMergeScheduler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if scheduler == nil {
					t.Error("expected non-nil scheduler")
					return
				}
				if scheduler.GetMaxMergeMBDuringNRT() != tt.wantMaxMB {
					t.Errorf("expected maxMergeMBDuringNRT %f, got %f", tt.wantMaxMB, scheduler.GetMaxMergeMBDuringNRT())
				}
				if scheduler.GetPauseDuringReopen() != tt.wantPause {
					t.Errorf("expected pauseDuringReopen %v, got %v", tt.wantPause, scheduler.GetPauseDuringReopen())
				}
				if !scheduler.isOpen.Load() {
					t.Error("expected scheduler to be open")
				}
			}
		})
	}
}

func TestNRTMergeScheduler_NRTManager(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Initially should be nil
	if scheduler.GetNRTManager() != nil {
		t.Error("expected nil NRTManager initially")
	}

	// Create a mock NRTManager (we can't create real one without IndexWriter)
	// Just test the getter/setter
	scheduler.SetNRTManager(nil)
	if scheduler.GetNRTManager() != nil {
		t.Error("expected nil after setting nil")
	}
}

func TestNRTMergeScheduler_MaxMergeMBDuringNRT(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 50 MB",
			mb:   50.0,
			want: 50.0,
		},
		{
			name: "set to 200 MB",
			mb:   200.0,
			want: 200.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -10.0,
			want: 0.0,
		},
		{
			name: "set to zero",
			mb:   0.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler.SetMaxMergeMBDuringNRT(tt.mb)
			got := scheduler.GetMaxMergeMBDuringNRT()
			if got != tt.want {
				t.Errorf("GetMaxMergeMBDuringNRT() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergeScheduler_PauseDuringReopen(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Default should be true
	if !scheduler.GetPauseDuringReopen() {
		t.Error("expected pauseDuringReopen to be true by default")
	}

	scheduler.SetPauseDuringReopen(false)
	if scheduler.GetPauseDuringReopen() {
		t.Error("expected pauseDuringReopen to be false after setting")
	}

	scheduler.SetPauseDuringReopen(true)
	if !scheduler.GetPauseDuringReopen() {
		t.Error("expected pauseDuringReopen to be true after setting")
	}
}

func TestNRTMergeScheduler_MergeThrottlePercent(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	tests := []struct {
		name    string
		percent int
		want    int
	}{
		{
			name:    "set to 100%",
			percent: 100,
			want:    100,
		},
		{
			name:    "set to 50%",
			percent: 50,
			want:    50,
		},
		{
			name:    "set to 0%",
			percent: 0,
			want:    0,
		},
		{
			name:    "set to negative (should become 0)",
			percent: -10,
			want:    0,
		},
		{
			name:    "set to over 100 (should become 100)",
			percent: 150,
			want:    100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler.SetMergeThrottlePercent(tt.percent)
			got := scheduler.GetMergeThrottlePercent()
			if got != tt.want {
				t.Errorf("GetMergeThrottlePercent() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNRTMergeScheduler_ReopenNotifications(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Initially no reopen in progress
	if scheduler.IsReopenInProgress() {
		t.Error("expected no reopen in progress initially")
	}

	// Notify reopen started
	scheduler.NotifyReopenStarted()
	if !scheduler.IsReopenInProgress() {
		t.Error("expected reopen in progress after NotifyReopenStarted")
	}

	// Notify reopen finished
	scheduler.NotifyReopenFinished()
	if scheduler.IsReopenInProgress() {
		t.Error("expected no reopen in progress after NotifyReopenFinished")
	}
}

func TestNRTMergeScheduler_Close(t *testing.T) {
	wrapped := NewConcurrentMergeScheduler()
	scheduler, _ := NewNRTMergeScheduler(wrapped)

	// Close the scheduler
	err := scheduler.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Should be closed
	if scheduler.isOpen.Load() {
		t.Error("expected scheduler to be closed")
	}

	// Close again should be no-op
	err = scheduler.Close()
	if err != nil {
		t.Errorf("second Close() error = %v", err)
	}

	// Merge should fail on closed scheduler
	source := &mockMergeSource{}
	err = scheduler.Merge(source, FULL_FLUSH)
	if err == nil {
		t.Error("expected error when calling Merge on closed scheduler")
	}
}

func TestNRTMergeScheduler_Stats(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Get initial stats
	stats := scheduler.GetStats()
	if stats.TotalMerges != 0 {
		t.Errorf("expected 0 total merges initially, got %d", stats.TotalMerges)
	}
	if stats.DeferredMerges != 0 {
		t.Errorf("expected 0 deferred merges initially, got %d", stats.DeferredMerges)
	}
	if stats.PausedMerges != 0 {
		t.Errorf("expected 0 paused merges initially, got %d", stats.PausedMerges)
	}

	// Reset stats
	scheduler.ResetStats()
	stats = scheduler.GetStats()
	if stats.TotalMerges != 0 {
		t.Errorf("expected 0 total merges after reset, got %d", stats.TotalMerges)
	}
}

func TestNRTMergeScheduler_String(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	str := scheduler.String()
	if str == "" {
		t.Error("String() should not return empty string")
	}

	// Should contain expected components
	if str == "" {
		t.Error("String() should return non-empty string")
	}
}

func TestNRTMergeScheduler_MaxMerges(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Set max merges
	scheduler.SetMaxMerges(5)
	if scheduler.GetMaxMerges() != 5 {
		t.Errorf("expected maxMerges 5, got %d", scheduler.GetMaxMerges())
	}
}

func TestNRTMergeScheduler_waitIfReopenInProgress(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	// Disable pause during reopen for this test
	scheduler.SetPauseDuringReopen(false)

	// Should return immediately when pauseDuringReopen is false
	err := scheduler.waitIfReopenInProgress()
	if err != nil {
		t.Errorf("waitIfReopenInProgress() error = %v", err)
	}

	// Enable pause during reopen
	scheduler.SetPauseDuringReopen(true)

	// Should return immediately when no reopen in progress
	err = scheduler.waitIfReopenInProgress()
	if err != nil {
		t.Errorf("waitIfReopenInProgress() error = %v", err)
	}
}

func TestNRTMergeScheduler_shouldDeferMerge(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)
	source := &mockMergeSource{}

	// By default should not defer
	if scheduler.shouldDeferMerge(source) {
		t.Error("expected not to defer merge by default")
	}

	// Set max merge MB to 0 (no limit)
	scheduler.SetMaxMergeMBDuringNRT(0)
	if scheduler.shouldDeferMerge(source) {
		t.Error("expected not to defer when maxMergeMB is 0")
	}
}

func TestNRTMergeScheduler_ConcurrentAccess(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	done := make(chan bool, 100)

	// Concurrent getters
	for i := 0; i < 50; i++ {
		go func() {
			_ = scheduler.GetMaxMergeMBDuringNRT()
			_ = scheduler.GetPauseDuringReopen()
			_ = scheduler.GetMergeThrottlePercent()
			_ = scheduler.IsReopenInProgress()
			_ = scheduler.GetRunningMergeCount()
			done <- true
		}()
	}

	// Concurrent setters
	for i := 0; i < 50; i++ {
		go func(idx int) {
			scheduler.SetMaxMergeMBDuringNRT(float64(idx))
			scheduler.SetMergeThrottlePercent(idx % 100)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Scheduler should still be valid
	if !scheduler.isOpen.Load() {
		t.Error("scheduler should be open after concurrent access")
	}
}

func TestNRTMergeStats_ConcurrentAccess(t *testing.T) {
	var stats NRTMergeStats

	done := make(chan bool, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		go func() {
			stats.mu.RLock()
			_ = stats.TotalMerges
			_ = stats.DeferredMerges
			_ = stats.PausedMerges
			_ = stats.TotalMergeTime
			stats.mu.RUnlock()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 50; i++ {
		go func() {
			stats.mu.Lock()
			stats.TotalMerges++
			stats.DeferredMerges++
			stats.PausedMerges++
			stats.TotalMergeTime += time.Millisecond
			stats.mu.Unlock()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify stats
	stats.mu.RLock()
	if stats.TotalMerges != 50 {
		t.Errorf("expected TotalMerges 50, got %d", stats.TotalMerges)
	}
	if stats.DeferredMerges != 50 {
		t.Errorf("expected DeferredMerges 50, got %d", stats.DeferredMerges)
	}
	if stats.PausedMerges != 50 {
		t.Errorf("expected PausedMerges 50, got %d", stats.PausedMerges)
	}
	stats.mu.RUnlock()
}

func TestNRTMergeScheduler_NotifyReopenConcurrent(t *testing.T) {
	scheduler, _ := NewNRTMergeScheduler(nil)

	done := make(chan bool, 200)

	// Concurrent start/finish notifications
	for i := 0; i < 100; i++ {
		go func() {
			scheduler.NotifyReopenStarted()
			done <- true
		}()
		go func() {
			scheduler.NotifyReopenFinished()
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 200; i++ {
		<-done
	}

	// Final state could be either, but should not panic
}
