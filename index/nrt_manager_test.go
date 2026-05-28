package index

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewNRTManager(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	if manager == nil {
		t.Fatal("expected NRTManager to not be nil")
	}

	if !manager.IsOpen() {
		t.Error("expected manager to be open")
	}

	if manager.GetGeneration() != 1 {
		t.Errorf("expected generation 1, got %d", manager.GetGeneration())
	}

	if manager.GetWriter() != writer {
		t.Error("expected writer to be set")
	}
}

func TestNewNRTManager_Nil(t *testing.T) {
	_, err := NewNRTManager(nil)
	if err == nil {
		t.Error("expected error for nil writer")
	}
}

func TestNRTManager_GetReader(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	reader, err := manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	if reader == nil {
		t.Error("expected reader to not be nil")
	}

	// Getting reader again should return same instance
	reader2, err := manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	if reader != reader2 {
		t.Error("expected same reader instance")
	}
}

func TestNRTManager_GetReader_Closed(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	manager.Close()

	_, err = manager.GetReader()
	if err == nil {
		t.Error("expected error when manager is closed")
	}
}

func TestNRTManager_MaybeRefresh(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Try refresh
	refreshed, err := manager.MaybeRefresh(context.Background())
	if err != nil {
		t.Fatalf("maybe refresh failed: %v", err)
	}

	// Without actual changes, should not refresh
	if refreshed {
		t.Error("expected no refresh without changes")
	}
}

func TestNRTManager_MaybeRefresh_Closed(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	manager.Close()

	_, err = manager.MaybeRefresh(context.Background())
	if err == nil {
		t.Error("expected error when manager is closed")
	}
}

func TestNRTManager_Close(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}

	err = manager.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if manager.IsOpen() {
		t.Error("expected manager to be closed")
	}

	// Close again should not error
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestNRTManager_SetMaxRefreshSec(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Default should be 60 seconds
	if manager.GetMaxRefreshSec() != 60*time.Second {
		t.Errorf("expected default 60s, got %v", manager.GetMaxRefreshSec())
	}

	// Set to 30 seconds
	manager.SetMaxRefreshSec(30 * time.Second)
	if manager.GetMaxRefreshSec() != 30*time.Second {
		t.Errorf("expected 30s, got %v", manager.GetMaxRefreshSec())
	}
}

func TestNRTManager_ApplyAllDeletes(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Default should be true
	if !manager.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be true by default")
	}

	// Set to false
	manager.SetApplyAllDeletes(false)
	if manager.GetApplyAllDeletes() {
		t.Error("expected applyAllDeletes to be false")
	}
}

// mockNRTReopenListener is a mock implementation of NRTReopenListener for testing
type mockNRTReopenListener struct {
	beforeCalled bool
	afterCalled  bool
	generation   int64
}

func (m *mockNRTReopenListener) BeforeReopen() {
	m.beforeCalled = true
}

func (m *mockNRTReopenListener) AfterReopen(generation int64) {
	m.afterCalled = true
	m.generation = generation
}

func TestNRTManager_ReopenListener(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	listener := &mockNRTReopenListener{}
	manager.AddReopenListener(listener)

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Trigger refresh
	manager.MaybeRefresh(context.Background())

	// Note: Listeners are only called when an actual refresh happens
	// Without actual changes, they won't be called
}

func TestNRTManager_RemoveReopenListener(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	listener := &mockNRTReopenListener{}
	manager.AddReopenListener(listener)
	manager.RemoveReopenListener(listener)

	// Should not panic
}

func TestNRTManager_WaitForGeneration(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Wait for current generation should return immediately
	reader, err := manager.WaitForGeneration(context.Background(), 1)
	if err != nil {
		t.Fatalf("wait for generation failed: %v", err)
	}

	if reader == nil {
		t.Error("expected reader")
	}
}

func TestNRTManager_WaitForGeneration_Timeout(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Set short timeout
	manager.SetMaxRefreshSec(100 * time.Millisecond)

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Wait for future generation should timeout
	_, err = manager.WaitForGeneration(context.Background(), 100)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestNRTManager_Refresh(t *testing.T) {
	writer := &IndexWriter{}

	manager, err := NewNRTManager(writer)
	if err != nil {
		t.Fatalf("failed to create NRTManager: %v", err)
	}
	defer manager.Close()

	// Get initial reader
	_, err = manager.GetReader()
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}

	// Refresh
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = manager.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
}

// TestNRTManager_Refresh_NoWaitChanLeakOnMaybeRefreshError verifies that when
// MaybeRefresh fails after Refresh has registered its waitChan, the waitChan is
// removed from waitingReaders instead of lingering until Close.
//
// The MaybeRefresh error branch sits between Refresh registering its waitChan and
// the select that consumes it, and is only reachable while the manager is
// concurrently transitioning to closed. This is a white-box test: it drives the
// reader stale so Refresh registers a waitChan, then flips the manager closed
// under m.mu. MaybeRefresh re-acquires m.mu and re-checks isOpen, so it observes
// the closed state and returns "manager is closed" before the select.
//
// Two invariants are asserted: (1) on every Refresh return — error, success, or
// the short fallback timeout — waitingReaders must be empty (no channel lingers);
// (2) across the run, the MaybeRefresh error path must be exercised at least once,
// otherwise the test would silently fail to cover the fix.
func TestNRTManager_Refresh_NoWaitChanLeakOnMaybeRefreshError(t *testing.T) {
	const attempts = 2000

	sawClosedError := false

	for i := 0; i < attempts && !sawClosedError; i++ {
		writer := &IndexWriter{}
		// One buffered document makes the freshly-created NRT reader non-current,
		// so Refresh proceeds past the fast path and registers a waitChan.
		writer.docCount.Store(1)

		manager, err := NewNRTManager(writer)
		if err != nil {
			t.Fatalf("failed to create NRTManager: %v", err)
		}
		// Keep the select's fallback timeout short so an iteration that misses the
		// injection window returns in milliseconds instead of the default 60s.
		manager.SetMaxRefreshSec(2 * time.Millisecond)

		// Materialise the current (stale) reader up front so Refresh's GetReader
		// returns it while the manager is still open.
		if _, err := manager.GetReader(); err != nil {
			manager.Close()
			t.Fatalf("failed to get reader: %v", err)
		}

		refreshErr := make(chan error, 1)
		go func() {
			_, err := manager.Refresh(context.Background())
			refreshErr <- err
		}()

		// Injector: wait until Refresh has registered its waitChan, then flip the
		// manager closed under m.mu. We only flip AFTER registration so that any
		// resulting "manager is closed" error originates from MaybeRefresh (after
		// the waitChan is in waitingReaders), never from Refresh's earlier
		// GetReader call — otherwise no waitChan would have been registered and the
		// fix would go untested. registered reports whether the waitChan was seen.
		registeredCh := make(chan bool, 1)
		go func() {
			deadline := time.Now().Add(50 * time.Millisecond)
			for time.Now().Before(deadline) {
				// Tight spin (no yield) to catch the brief window in which the
				// waitChan is registered but MaybeRefresh has not yet drained it.
				manager.mu.RLock()
				n := len(manager.waitingReaders)
				manager.mu.RUnlock()
				if n > 0 {
					manager.mu.Lock()
					manager.isOpen.Store(false)
					manager.mu.Unlock()
					registeredCh <- true
					return
				}
			}
			registeredCh <- false
		}()

		var got error
		select {
		case got = <-refreshErr:
		case <-time.After(2 * time.Second):
			manager.Close()
			t.Fatal("Refresh did not return in time")
		}
		registered := <-registeredCh

		// Invariant 1: no waitChan may linger, on any return path.
		manager.mu.Lock()
		leaked := len(manager.waitingReaders)
		manager.mu.Unlock()
		if leaked != 0 {
			manager.Close()
			t.Fatalf("attempt %d: waitingReaders leaked %d channels after Refresh returned (err=%v)",
				i, leaked, got)
		}

		// Detect the specific MaybeRefresh error path: a waitChan was registered and
		// Refresh returned "manager is closed". This proves the fix's branch ran.
		if registered && got != nil && strings.Contains(got.Error(), "manager is closed") {
			sawClosedError = true
		}

		manager.Close()
	}

	// Invariant 2: the MaybeRefresh error branch must have been exercised.
	if !sawClosedError {
		t.Fatal("MaybeRefresh \"manager is closed\" error path was never exercised; " +
			"the leak fix went unverified")
	}
}
