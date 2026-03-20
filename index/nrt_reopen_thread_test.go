package index

import (
	"errors"
	"testing"
	"time"
)

func TestNewNRTReopenThread(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, err := NewNRTReopenThread("test-thread", 1*time.Second, reopenFunc)
	if err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	if thread == nil {
		t.Fatal("expected thread to not be nil")
	}

	if thread.GetName() != "test-thread" {
		t.Errorf("expected name 'test-thread', got %s", thread.GetName())
	}

	if thread.GetInterval() != 1*time.Second {
		t.Errorf("expected interval 1s, got %v", thread.GetInterval())
	}

	if thread.IsRunning() {
		t.Error("expected thread to not be running initially")
	}

	if thread.GetReopenCount() != 0 {
		t.Errorf("expected 0 reopens, got %d", thread.GetReopenCount())
	}
}

func TestNewNRTReopenThread_EmptyName(t *testing.T) {
	_, err := NewNRTReopenThread("", 1*time.Second, func() error { return nil })
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestNewNRTReopenThread_InvalidInterval(t *testing.T) {
	_, err := NewNRTReopenThread("test", 0, func() error { return nil })
	if err == nil {
		t.Error("expected error for zero interval")
	}

	_, err = NewNRTReopenThread("test", -1*time.Second, func() error { return nil })
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestNewNRTReopenThread_NilFunc(t *testing.T) {
	_, err := NewNRTReopenThread("test", 1*time.Second, nil)
	if err == nil {
		t.Error("expected error for nil reopenFunc")
	}
}

func TestNRTReopenThread_StartStop(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 100*time.Millisecond, reopenFunc)

	// Start
	err := thread.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !thread.IsRunning() {
		t.Error("expected thread to be running")
	}

	// Start again should fail
	err = thread.Start()
	if err == nil {
		t.Error("expected error when starting already running thread")
	}

	// Wait for some reopens
	time.Sleep(250 * time.Millisecond)

	if thread.GetReopenCount() < 1 {
		t.Error("expected at least 1 reopen")
	}

	// Stop
	err = thread.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if thread.IsRunning() {
		t.Error("expected thread to be stopped")
	}

	// Stop again should not error
	err = thread.Stop()
	if err != nil {
		t.Errorf("second stop failed: %v", err)
	}
}

func TestNRTReopenThread_Trigger(t *testing.T) {
	reopenCalled := false
	reopenFunc := func() error {
		reopenCalled = true
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Hour, reopenFunc) // Long interval

	thread.Start()
	defer thread.Stop()

	err := thread.Trigger()
	if err != nil {
		t.Fatalf("trigger failed: %v", err)
	}

	if !reopenCalled {
		t.Error("expected reopenFunc to be called")
	}

	if thread.GetReopenCount() != 1 {
		t.Errorf("expected 1 reopen, got %d", thread.GetReopenCount())
	}
}

func TestNRTReopenThread_Trigger_NotRunning(t *testing.T) {
	thread, _ := NewNRTReopenThread("test-thread", 1*time.Second, func() error { return nil })

	err := thread.Trigger()
	if err == nil {
		t.Error("expected error when triggering stopped thread")
	}
}

func TestNRTReopenThread_ReopenError(t *testing.T) {
	expectedErr := errors.New("reopen failed")
	reopenFunc := func() error {
		return expectedErr
	}

	thread, _ := NewNRTReopenThread("test-thread", 100*time.Millisecond, reopenFunc)
	thread.Start()
	defer thread.Stop()

	// Wait for reopen
	time.Sleep(150 * time.Millisecond)

	if thread.GetLastException() != expectedErr {
		t.Error("expected last exception to be set")
	}

	if thread.GetReopenCount() != 0 {
		t.Error("expected 0 reopens on error")
	}
}

func TestNRTReopenThread_GetLastReopenTime(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Hour, reopenFunc)

	initialTime := thread.GetLastReopenTime()
	if initialTime.IsZero() {
		t.Error("expected initial time to be set")
	}

	thread.Start()
	defer thread.Stop()

	time.Sleep(10 * time.Millisecond)
	thread.Trigger()

	newTime := thread.GetLastReopenTime()
	if !newTime.After(initialTime) {
		t.Error("expected new time to be after initial")
	}
}

func TestNRTReopenThread_SetInterval(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Second, reopenFunc)

	err := thread.SetInterval(2 * time.Second)
	if err != nil {
		t.Fatalf("failed to set interval: %v", err)
	}

	if thread.GetInterval() != 2*time.Second {
		t.Errorf("expected interval 2s, got %v", thread.GetInterval())
	}

	err = thread.SetInterval(0)
	if err == nil {
		t.Error("expected error for zero interval")
	}
}

func TestNRTReopenThread_ApplyAllDeletes(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Second, reopenFunc)

	if !thread.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be true by default")
	}

	thread.SetApplyAllDeletes(false)

	if thread.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be false")
	}
}

func TestNRTReopenThread_GetTimeSinceLastReopen(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Second, reopenFunc)

	// Initially should be small
	duration := thread.GetTimeSinceLastReopen()
	if duration < 0 {
		t.Error("expected non-negative duration")
	}

	thread.Start()
	defer thread.Stop()

	time.Sleep(10 * time.Millisecond)
	thread.Trigger()

	duration = thread.GetTimeSinceLastReopen()
	if duration < 0 {
		t.Error("expected non-negative duration after reopen")
	}
}

func TestNRTReopenThread_String(t *testing.T) {
	reopenFunc := func() error {
		return nil
	}

	thread, _ := NewNRTReopenThread("test-thread", 1*time.Second, reopenFunc)

	str := thread.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}
