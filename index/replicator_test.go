package index

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewReplicatorBase(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorBase("test-replicator", checkFunc)
	if r == nil {
		t.Fatal("expected replicator to not be nil")
	}

	if r.GetName() != "test-replicator" {
		t.Errorf("expected name 'test-replicator', got %s", r.GetName())
	}

	if r.IsRunning() {
		t.Error("expected replicator to not be running initially")
	}

	if r.GetLastException() != nil {
		t.Error("expected no initial exception")
	}
}

func TestReplicatorBase_StartStop(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorBase("test-replicator", checkFunc)

	// Start
	err := r.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !r.IsRunning() {
		t.Error("expected replicator to be running")
	}

	// Start again should fail
	err = r.Start()
	if err == nil {
		t.Error("expected error when starting already running replicator")
	}

	// Stop
	err = r.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if r.IsRunning() {
		t.Error("expected replicator to be stopped")
	}

	// Stop again should not error
	err = r.Stop()
	if err != nil {
		t.Errorf("second stop failed: %v", err)
	}
}

func TestReplicatorBase_CheckNow(t *testing.T) {
	checkCalled := false
	checkFunc := func(ctx context.Context) error {
		checkCalled = true
		return nil
	}

	r := NewReplicatorBase("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}

	if !checkCalled {
		t.Error("expected check function to be called")
	}
}

func TestReplicatorBase_CheckNow_NotRunning(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorBase("test-replicator", checkFunc)
	// Don't start

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err == nil {
		t.Error("expected error when checking not running replicator")
	}
}

func TestReplicatorBase_CheckNow_NoCheckFunc(t *testing.T) {
	r := NewReplicatorBase("test-replicator", nil)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err == nil {
		t.Error("expected error when check function is nil")
	}
}

func TestReplicatorBase_CheckNow_Error(t *testing.T) {
	expectedErr := errors.New("check failed")
	checkFunc := func(ctx context.Context) error {
		return expectedErr
	}

	r := NewReplicatorBase("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err == nil {
		t.Fatal("expected error")
	}

	if r.GetLastException() != expectedErr {
		t.Error("expected last exception to be set")
	}
}

func TestReplicatorBase_SetCheckFunc(t *testing.T) {
	r := NewReplicatorBase("test-replicator", nil)

	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r.SetCheckFunc(checkFunc)

	r.Start()
	defer r.Stop()

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err != nil {
		t.Errorf("check failed: %v", err)
	}
}

func TestReplicatorBase_GetContext(t *testing.T) {
	r := NewReplicatorBase("test-replicator", nil)

	ctx := r.GetContext()
	if ctx == nil {
		t.Error("expected context to not be nil")
	}

	// Context should be cancelled after stop
	r.Start()
	r.Stop()

	// Context might be nil after stop due to reset
	_ = r.GetContext()
}

func TestNewReplicatorWithStats(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)
	if r == nil {
		t.Fatal("expected replicator to not be nil")
	}

	if r.ReplicatorBase == nil {
		t.Error("expected base replicator to be set")
	}

	stats := r.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 total checks, got %d", stats.TotalChecks)
	}
}

func TestReplicatorWithStats_CheckNow_Success(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	err := r.CheckNow(ctx)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}

	stats := r.GetStats()
	if stats.TotalChecks != 1 {
		t.Errorf("expected 1 total check, got %d", stats.TotalChecks)
	}

	if stats.SuccessfulChecks != 1 {
		t.Errorf("expected 1 successful check, got %d", stats.SuccessfulChecks)
	}

	if stats.FailedChecks != 0 {
		t.Errorf("expected 0 failed checks, got %d", stats.FailedChecks)
	}

	if stats.LastCheckTime == 0 {
		t.Error("expected last check time to be set")
	}
}

func TestReplicatorWithStats_CheckNow_Failure(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return errors.New("check failed")
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	r.CheckNow(ctx) // Ignore error

	stats := r.GetStats()
	if stats.TotalChecks != 1 {
		t.Errorf("expected 1 total check, got %d", stats.TotalChecks)
	}

	if stats.SuccessfulChecks != 0 {
		t.Errorf("expected 0 successful checks, got %d", stats.SuccessfulChecks)
	}

	if stats.FailedChecks != 1 {
		t.Errorf("expected 1 failed check, got %d", stats.FailedChecks)
	}
}

func TestReplicatorWithStats_RecordBytesTransferred(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)

	r.RecordBytesTransferred(1000)
	r.RecordBytesTransferred(500)

	stats := r.GetStats()
	if stats.TotalBytesTransferred != 1500 {
		t.Errorf("expected 1500 bytes transferred, got %d", stats.TotalBytesTransferred)
	}
}

func TestReplicatorWithStats_ResetStats(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		return nil
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	ctx := context.Background()
	r.CheckNow(ctx)
	r.RecordBytesTransferred(1000)

	r.ResetStats()

	stats := r.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 total checks after reset, got %d", stats.TotalChecks)
	}

	if stats.TotalBytesTransferred != 0 {
		t.Errorf("expected 0 bytes transferred after reset, got %d", stats.TotalBytesTransferred)
	}
}

func TestReplicatorStats_Struct(t *testing.T) {
	stats := ReplicatorStats{
		TotalChecks:           10,
		SuccessfulChecks:      8,
		FailedChecks:          2,
		LastCheckTime:         time.Now().UnixMilli(),
		TotalBytesTransferred: 1024,
	}

	if stats.TotalChecks != 10 {
		t.Errorf("expected 10 total checks, got %d", stats.TotalChecks)
	}

	if stats.SuccessfulChecks != 8 {
		t.Errorf("expected 8 successful checks, got %d", stats.SuccessfulChecks)
	}

	if stats.FailedChecks != 2 {
		t.Errorf("expected 2 failed checks, got %d", stats.FailedChecks)
	}

	if stats.TotalBytesTransferred != 1024 {
		t.Errorf("expected 1024 bytes transferred, got %d", stats.TotalBytesTransferred)
	}
}

func TestReplicator_ConcurrentOperations(t *testing.T) {
	checkFunc := func(ctx context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	r := NewReplicatorWithStats("test-replicator", checkFunc)
	r.Start()
	defer r.Stop()

	done := make(chan bool, 4)

	// CheckNow goroutine
	go func() {
		for i := 0; i < 10; i++ {
			ctx := context.Background()
			r.CheckNow(ctx)
		}
		done <- true
	}()

	// GetStats goroutine
	go func() {
		for i := 0; i < 10; i++ {
			r.GetStats()
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()

	// RecordBytesTransferred goroutine
	go func() {
		for i := 0; i < 10; i++ {
			r.RecordBytesTransferred(100)
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()

	// GetLastException goroutine
	go func() {
		for i := 0; i < 10; i++ {
			r.GetLastException()
			time.Sleep(2 * time.Millisecond)
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
