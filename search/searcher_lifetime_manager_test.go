package search

import (
	"context"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewSearcherLifetimeManager(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create minimal SearcherManager
	sm, _ := NewSearcherManager(nil, nil)

	lm, err := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("failed to create SearcherLifetimeManager: %v", err)
	}
	defer lm.Close()

	if lm == nil {
		t.Fatal("expected SearcherLifetimeManager to not be nil")
	}

	if !lm.IsOpen() {
		t.Error("expected manager to be open")
	}

	if lm.GetMaxSearcherAge() != 5*time.Minute {
		t.Errorf("expected max age 5m, got %v", lm.GetMaxSearcherAge())
	}

	if lm.GetMaxSearchers() != 10 {
		t.Errorf("expected max searchers 10, got %d", lm.GetMaxSearchers())
	}
}

func TestNewSearcherLifetimeManager_NilManager(t *testing.T) {
	_, err := NewSearcherLifetimeManager(nil, 5*time.Minute, 10)
	if err == nil {
		t.Error("expected error for nil manager")
	}
}

func TestNewSearcherLifetimeManager_InvalidMaxAge(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	_, err := NewSearcherLifetimeManager(sm, 0, 10)
	if err == nil {
		t.Error("expected error for zero max age")
	}

	_, err = NewSearcherLifetimeManager(sm, -1*time.Second, 10)
	if err == nil {
		t.Error("expected error for negative max age")
	}
}

func TestNewSearcherLifetimeManager_InvalidMaxSearchers(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	_, err := NewSearcherLifetimeManager(sm, 5*time.Minute, 0)
	if err == nil {
		t.Error("expected error for zero max searchers")
	}

	_, err = NewSearcherLifetimeManager(sm, 5*time.Minute, -1)
	if err == nil {
		t.Error("expected error for negative max searchers")
	}
}

func TestSearcherLifetimeManager_AcquireRelease(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create SearcherManager
	sm, _ := NewSearcherManager(dir, nil)

	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	// Acquire
	ctx := context.Background()
	searcher, err := lm.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}

	if searcher == nil {
		t.Error("expected searcher to not be nil")
	}

	// Release
	err = lm.Release(searcher)
	if err != nil {
		t.Fatalf("failed to release: %v", err)
	}
}

func TestSearcherLifetimeManager_Acquire_Closed(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	lm.Close()

	_, err := lm.Acquire(context.Background())
	if err == nil {
		t.Error("expected error when acquiring from closed manager")
	}
}

func TestSearcherLifetimeManager_SetWarmer(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	// Initially no warmer
	if lm.GetWarmer() != nil {
		t.Error("expected no warmer initially")
	}

	// Set warmer
	warmer := WarmerFunc(func(ctx context.Context, searcher *IndexSearcher) error {
		return nil
	})
	lm.SetWarmer(warmer)

	if lm.GetWarmer() == nil {
		t.Error("expected warmer to be set")
	}
}

func TestSearcherLifetimeManager_WarmSearcher(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	// Without warmer, should succeed
	ctx := context.Background()
	err := lm.WarmSearcher(ctx, nil)
	if err != nil {
		t.Errorf("expected no error without warmer: %v", err)
	}

	// With warmer
	warmCalled := false
	warmer := WarmerFunc(func(ctx context.Context, searcher *IndexSearcher) error {
		warmCalled = true
		return nil
	})
	lm.SetWarmer(warmer)

	err = lm.WarmSearcher(ctx, nil)
	if err != nil {
		t.Errorf("warm failed: %v", err)
	}

	if !warmCalled {
		t.Error("expected warmer to be called")
	}
}

func TestSearcherLifetimeManager_GetSearcherCount(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	// Initially 0
	if lm.GetSearcherCount() != 0 {
		t.Errorf("expected 0 searchers, got %d", lm.GetSearcherCount())
	}
}

func TestSearcherLifetimeManager_SetMaxSearcherAge(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	lm.SetMaxSearcherAge(10 * time.Minute)

	if lm.GetMaxSearcherAge() != 10*time.Minute {
		t.Errorf("expected 10m, got %v", lm.GetMaxSearcherAge())
	}
}

func TestSearcherLifetimeManager_SetMaxSearchers(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	lm.SetMaxSearchers(20)

	if lm.GetMaxSearchers() != 20 {
		t.Errorf("expected 20, got %d", lm.GetMaxSearchers())
	}
}

func TestSearcherLifetimeManager_Close(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)

	err := lm.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if lm.IsOpen() {
		t.Error("expected manager to be closed")
	}

	// Close again should not error
	err = lm.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestSearcherLifetimeManager_String(t *testing.T) {
	sm, _ := NewSearcherManager(nil, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	str := lm.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestSearcherLifetimeManager_Cleanup(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create SearcherManager
	sm, _ := NewSearcherManager(dir, nil)

	// Short max age for testing
	lm, _ := NewSearcherLifetimeManager(sm, 100*time.Millisecond, 10)
	defer lm.Close()

	// Acquire and release
	ctx := context.Background()
	searcher, _ := lm.Acquire(ctx)
	lm.Release(searcher)

	// Wait for cleanup
	time.Sleep(300 * time.Millisecond)

	// Cleanup should have run
	// Note: Actual cleanup depends on implementation details
}

func TestSearcherLifetimeManager_ConcurrentOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	lm, _ := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	defer lm.Close()

	// Run concurrent operations
	done := make(chan bool, 3)

	// Acquire/Release goroutine
	go func() {
		for i := 0; i < 10; i++ {
			ctx := context.Background()
			searcher, _ := lm.Acquire(ctx)
			time.Sleep(10 * time.Millisecond)
			lm.Release(searcher)
		}
		done <- true
	}()

	// Get count goroutine
	go func() {
		for i := 0; i < 10; i++ {
			_ = lm.GetSearcherCount()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Get config goroutine
	go func() {
		for i := 0; i < 10; i++ {
			_ = lm.GetMaxSearcherAge()
			_ = lm.GetMaxSearchers()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
