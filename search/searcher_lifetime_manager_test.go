package search

import (
	"context"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newSearcherManagerForTest creates a minimal SearcherManager for unit testing
// of SearcherLifetimeManager. It bypasses the nil-initial-searcher guard so
// tests that never call Acquire() can construct a manager without real index
// infrastructure.
func newSearcherManagerForTest() *SearcherManager {
	return &SearcherManager{
		refCount: make(map[*IndexSearcher]int),
	}
}

// mustLifetimeManager creates a SearcherLifetimeManager or fails the test.
func mustLifetimeManager(t *testing.T, maxAge time.Duration, maxSearchers int) *SearcherLifetimeManager {
	t.Helper()
	sm := newSearcherManagerForTest()
	lm, err := NewSearcherLifetimeManager(sm, maxAge, maxSearchers)
	if err != nil {
		t.Fatalf("NewSearcherLifetimeManager: %v", err)
	}
	return lm
}

func TestNewSearcherLifetimeManager(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	lm, err := NewSearcherLifetimeManager(newSearcherManagerForTest(), 5*time.Minute, 10)
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
	sm := newSearcherManagerForTest()
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
	sm := newSearcherManagerForTest()
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
	// Requires a real IndexSearcher backed by a live DirectoryReader.
	// Skipped until NRT infrastructure is wired into SearcherLifetimeManager tests.
	t.Skip("requires real index infrastructure")
}

func TestSearcherLifetimeManager_Acquire_Closed(t *testing.T) {
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
	lm.Close()

	_, err := lm.Acquire(context.Background())
	if err == nil {
		t.Error("expected error when acquiring from closed manager")
	}
}

func TestSearcherLifetimeManager_SetWarmer(t *testing.T) {
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
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
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
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
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
	defer lm.Close()

	// Initially 0
	if lm.GetSearcherCount() != 0 {
		t.Errorf("expected 0 searchers, got %d", lm.GetSearcherCount())
	}
}

func TestSearcherLifetimeManager_SetMaxSearcherAge(t *testing.T) {
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
	defer lm.Close()

	lm.SetMaxSearcherAge(10 * time.Minute)

	if lm.GetMaxSearcherAge() != 10*time.Minute {
		t.Errorf("expected 10m, got %v", lm.GetMaxSearcherAge())
	}
}

func TestSearcherLifetimeManager_SetMaxSearchers(t *testing.T) {
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
	defer lm.Close()

	lm.SetMaxSearchers(20)

	if lm.GetMaxSearchers() != 20 {
		t.Errorf("expected 20, got %d", lm.GetMaxSearchers())
	}
}

func TestSearcherLifetimeManager_Close(t *testing.T) {
	lm := mustLifetimeManager(t, 5*time.Minute, 10)

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
	lm := mustLifetimeManager(t, 5*time.Minute, 10)
	defer lm.Close()

	str := lm.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestSearcherLifetimeManager_Cleanup(t *testing.T) {
	// Requires a real IndexSearcher to acquire from the manager.
	// Skipped until NRT infrastructure is wired into SearcherLifetimeManager tests.
	t.Skip("requires real index infrastructure")
}

func TestSearcherLifetimeManager_ConcurrentOperations(t *testing.T) {
	// Requires a real IndexSearcher to exercise concurrent Acquire/Release.
	// Skipped until NRT infrastructure is wired into SearcherLifetimeManager tests.
	t.Skip("requires real index infrastructure")
}
