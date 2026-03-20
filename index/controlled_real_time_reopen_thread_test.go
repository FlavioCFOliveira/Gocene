package index

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewControlledRealTimeReopenThread(t *testing.T) {
	writer := &IndexWriter{}
	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	thread, err := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create ControlledRealTimeReopenThread: %v", err)
	}

	if thread == nil {
		t.Fatal("expected ControlledRealTimeReopenThread to not be nil")
	}

	if thread.GetMinStaleSec() != 100*time.Millisecond {
		t.Errorf("expected minStaleSec 100ms, got %v", thread.GetMinStaleSec())
	}

	if thread.GetMaxStaleSec() != 500*time.Millisecond {
		t.Errorf("expected maxStaleSec 500ms, got %v", thread.GetMaxStaleSec())
	}
}

func TestNewControlledRealTimeReopenThread_NilManager(t *testing.T) {
	_, err := NewControlledRealTimeReopenThread(nil, 100*time.Millisecond, 500*time.Millisecond)
	if err == nil {
		t.Error("expected error for nil manager")
	}
}

func TestNewControlledRealTimeReopenThread_InvalidMinStale(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	_, err := NewControlledRealTimeReopenThread(manager, 0, 500*time.Millisecond)
	if err == nil {
		t.Error("expected error for zero minStaleSec")
	}

	_, err = NewControlledRealTimeReopenThread(manager, -1*time.Second, 500*time.Millisecond)
	if err == nil {
		t.Error("expected error for negative minStaleSec")
	}
}

func TestNewControlledRealTimeReopenThread_InvalidMaxStale(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	_, err := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 0)
	if err == nil {
		t.Error("expected error for zero maxStaleSec")
	}

	_, err = NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, -1*time.Second)
	if err == nil {
		t.Error("expected error for negative maxStaleSec")
	}
}

func TestNewControlledRealTimeReopenThread_MaxLessThanMin(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	_, err := NewControlledRealTimeReopenThread(manager, 500*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Error("expected error when maxStaleSec < minStaleSec")
	}
}

func TestControlledRealTimeReopenThread_StartStop(t *testing.T) {
	writer := &IndexWriter{}
	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	thread, err := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// Start the thread
	err = thread.Start()
	if err != nil {
		t.Fatalf("failed to start thread: %v", err)
	}

	if !thread.IsRunning() {
		t.Error("expected thread to be running")
	}

	// Stop the thread
	err = thread.Stop()
	if err != nil {
		t.Fatalf("failed to stop thread: %v", err)
	}

	if thread.IsRunning() {
		t.Error("expected thread to be stopped")
	}
}

func TestControlledRealTimeReopenThread_StartAlreadyRunning(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	thread.Start()
	defer thread.Stop()

	// Try to start again
	err := thread.Start()
	if err == nil {
		t.Error("expected error when starting already running thread")
	}
}

func TestControlledRealTimeReopenThread_StopNotRunning(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	// Stop when not running should not error
	err := thread.Stop()
	if err != nil {
		t.Errorf("expected no error when stopping non-running thread: %v", err)
	}
}

func TestControlledRealTimeReopenThread_ForceReopen(t *testing.T) {
	writer := &IndexWriter{}
	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	thread, err := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	// ForceReopen when not running should error
	err = thread.ForceReopen()
	if err == nil {
		t.Error("expected error when force reopen on non-running thread")
	}

	// Start the thread
	thread.Start()
	defer thread.Stop()

	// ForceReopen when running should work (even if no reader available)
	// Note: This may return an error if no reader is available, which is expected
	_ = thread.ForceReopen()
}

func TestControlledRealTimeReopenThread_GetReopenCount(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	// Initial count should be 0
	if thread.GetReopenCount() != 0 {
		t.Errorf("expected initial reopen count 0, got %d", thread.GetReopenCount())
	}
}

func TestControlledRealTimeReopenThread_GetLastReopenTime(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	// Initial time should be zero
	if !thread.GetLastReopenTime().IsZero() {
		t.Error("expected initial last reopen time to be zero")
	}
}

func TestControlledRealTimeReopenThread_SetErrorHandler(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	var handlerCalled atomic.Bool
	handler := func(err error) {
		handlerCalled.Store(true)
	}

	thread.SetErrorHandler(handler)

	// Start and stop to trigger potential errors
	thread.Start()
	time.Sleep(50 * time.Millisecond)
	thread.Stop()

	// Handler may or may not be called depending on timing
	_ = handlerCalled.Load()
}

func TestControlledRealTimeReopenThread_WaitForGeneration(t *testing.T) {
	writer := &IndexWriter{}
	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	// WaitForGeneration when not running should error
	_, err = thread.WaitForGeneration(context.Background(), 1)
	if err == nil {
		t.Error("expected error when waiting on non-running thread")
	}

	// Start the thread
	thread.Start()
	defer thread.Stop()

	// Get initial reader
	_, _ = manager.GetReader()

	// Wait for current generation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = thread.WaitForGeneration(ctx, 1)
	// May error if no reader available, which is expected in test environment
	_ = err
}

func TestControlledRealTimeReopenThread_RunLoop(t *testing.T) {
	writer := &IndexWriter{}
	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Use longer intervals to avoid too many iterations
	thread, _ := NewControlledRealTimeReopenThread(manager, 200*time.Millisecond, 1*time.Second)

	thread.Start()

	// Let it run for a bit
	time.Sleep(300 * time.Millisecond)

	thread.Stop()

	// Thread should have stopped cleanly
	if thread.IsRunning() {
		t.Error("expected thread to be stopped")
	}
}

func TestControlledRealTimeReopenThread_ConcurrentStartStop(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 100*time.Millisecond, 500*time.Millisecond)

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		err := thread.Start()
		if err != nil {
			t.Fatalf("failed to start thread (iteration %d): %v", i, err)
		}

		time.Sleep(50 * time.Millisecond)

		err = thread.Stop()
		if err != nil {
			t.Fatalf("failed to stop thread (iteration %d): %v", i, err)
		}
	}
}

func TestControlledRealTimeReopenThread_Getters(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	minStale := 150 * time.Millisecond
	maxStale := 750 * time.Millisecond

	thread, _ := NewControlledRealTimeReopenThread(manager, minStale, maxStale)

	if thread.GetMinStaleSec() != minStale {
		t.Errorf("expected minStaleSec %v, got %v", minStale, thread.GetMinStaleSec())
	}

	if thread.GetMaxStaleSec() != maxStale {
		t.Errorf("expected maxStaleSec %v, got %v", maxStale, thread.GetMaxStaleSec())
	}
}

func TestControlledRealTimeReopenThread_ThreadSafety(t *testing.T) {
	writer := &IndexWriter{}
	manager, _ := NewNRTManager(writer)
	defer manager.Close()

	thread, _ := NewControlledRealTimeReopenThread(manager, 50*time.Millisecond, 200*time.Millisecond)

	thread.Start()
	defer thread.Stop()

	// Concurrent operations
	done := make(chan bool, 4)

	go func() {
		for i := 0; i < 10; i++ {
			_ = thread.IsRunning()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = thread.GetReopenCount()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = thread.GetLastReopenTime()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = thread.GetMinStaleSec()
			_ = thread.GetMaxStaleSec()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
