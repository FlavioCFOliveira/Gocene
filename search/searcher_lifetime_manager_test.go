package search

import (
	"context"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newRealSearcherManagerForTest creates a SearcherManager backed by a small
// in-memory index with two documents.
func newRealSearcherManagerForTest(t *testing.T) *SearcherManager {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, text := range []string{"hello world", "foo bar"} {
		doc := document.NewDocument()
		f, _ := document.NewTextField("text", text, true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	_ = w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	searcher := NewIndexSearcher(reader)
	sm, err := NewSearcherManager(searcher, nil, nil)
	if err != nil {
		t.Fatalf("NewSearcherManager: %v", err)
	}
	t.Cleanup(func() { _ = sm.Close() })
	return sm
}

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
	sm := newRealSearcherManagerForTest(t)
	lm, err := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("NewSearcherLifetimeManager: %v", err)
	}
	defer lm.Close()

	ctx := context.Background()
	searcher, err := lm.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if searcher == nil {
		t.Fatal("expected non-nil searcher")
	}

	if err := lm.Release(searcher); err != nil {
		t.Fatalf("Release: %v", err)
	}
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
	sm := newRealSearcherManagerForTest(t)
	lm, err := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("NewSearcherLifetimeManager: %v", err)
	}
	defer lm.Close()

	ctx := context.Background()
	searcher, err := lm.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lm.Release(searcher); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// Cleanup should not error
	lm.cleanup()
}

func TestSearcherLifetimeManager_ConcurrentOperations(t *testing.T) {
	sm := newRealSearcherManagerForTest(t)
	lm, err := NewSearcherLifetimeManager(sm, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("NewSearcherLifetimeManager: %v", err)
	}
	defer lm.Close()

	ctx := context.Background()
	// Simple sequential acquire/release to verify no data race
	for i := 0; i < 3; i++ {
		s, err := lm.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
		if err := lm.Release(s); err != nil {
			t.Fatalf("Release %d: %v", i, err)
		}
}	}
