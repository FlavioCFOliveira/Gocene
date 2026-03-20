package index

import (
	"testing"
	"time"
)

// mockSegmentReader is a mock implementation for testing
type mockSegmentReader struct {
	closed bool
}

func (m *mockSegmentReader) Close() error {
	m.closed = true
	return nil
}

func TestNewReaderPool(t *testing.T) {
	rp := NewReaderPool(5)
	if rp == nil {
		t.Fatal("expected ReaderPool to not be nil")
	}
	if rp.GetMaxSize() != 5 {
		t.Errorf("expected max size 5, got %d", rp.GetMaxSize())
	}
	if rp.GetSize() != 0 {
		t.Errorf("expected size 0, got %d", rp.GetSize())
	}
}

func TestNewReaderPool_DefaultSize(t *testing.T) {
	rp := NewReaderPool(0)
	if rp.GetMaxSize() != 10 {
		t.Errorf("expected default max size 10, got %d", rp.GetMaxSize())
	}
}

func TestReaderPool_GetAndPut(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	segmentName := "test_segment"

	// Get from empty pool should return nil
	reader := rp.Get(segmentName)
	if reader != nil {
		t.Error("expected nil from empty pool")
	}

	// Put a reader
	rp.pool[segmentName] = append(rp.pool[segmentName], &SegmentReader{})

	// Get should return the reader
	reader = rp.Get(segmentName)
	if reader == nil {
		t.Error("expected reader from pool")
	}

	// Pool should be empty again
	if rp.GetSegmentSize(segmentName) != 0 {
		t.Error("expected empty pool after get")
	}
}

func TestReaderPool_Put_Nil(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	err := rp.Put("segment", nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestReaderPool_Put_Closed(t *testing.T) {
	rp := NewReaderPool(5)
	rp.Close()

	err := rp.Put("segment", &SegmentReader{})
	if err == nil {
		t.Error("expected error when putting to closed pool")
	}
}

func TestReaderPool_Get_Closed(t *testing.T) {
	rp := NewReaderPool(5)
	rp.Close()

	reader := rp.Get("segment")
	if reader != nil {
		t.Error("expected nil from closed pool")
	}
}

func TestReaderPool_Acquire(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	segmentName := "test_segment"

	// Acquire with empty pool should use factory
	reader, err := rp.Acquire(segmentName, func() (*SegmentReader, error) {
		return &SegmentReader{}, nil
	})
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}
	if reader == nil {
		t.Error("expected reader from factory")
	}

	// Release and re-acquire
	rp.Release(segmentName, reader)

	reader2, err := rp.Acquire(segmentName, nil)
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}
	if reader2 == nil {
		t.Error("expected reader from pool")
	}
}

func TestReaderPool_Acquire_NoFactory(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	_, err := rp.Acquire("segment", nil)
	if err == nil {
		t.Error("expected error when no factory provided and pool is empty")
	}
}

func TestReaderPool_Clear(t *testing.T) {
	rp := NewReaderPool(5)

	// Add readers to pool
	rp.pool["seg1"] = []*SegmentReader{{}, {}}
	rp.pool["seg2"] = []*SegmentReader{{}}

	if rp.GetSize() != 3 {
		t.Errorf("expected size 3, got %d", rp.GetSize())
	}

	err := rp.Clear()
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	if rp.GetSize() != 0 {
		t.Errorf("expected size 0 after clear, got %d", rp.GetSize())
	}
}

func TestReaderPool_ClearSegment(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	// Add readers to different segments
	rp.pool["seg1"] = []*SegmentReader{{}, {}}
	rp.pool["seg2"] = []*SegmentReader{{}}

	err := rp.ClearSegment("seg1")
	if err != nil {
		t.Fatalf("failed to clear segment: %v", err)
	}

	if rp.GetSegmentSize("seg1") != 0 {
		t.Error("expected seg1 to be empty")
	}
	if rp.GetSegmentSize("seg2") != 1 {
		t.Error("expected seg2 to still have 1 reader")
	}
}

func TestReaderPool_Close(t *testing.T) {
	rp := NewReaderPool(5)

	// Add readers to pool
	rp.pool["seg1"] = []*SegmentReader{{}, {}}

	err := rp.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !rp.IsClosed() {
		t.Error("expected pool to be closed")
	}

	// Close again should not error
	err = rp.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestReaderPool_SetMaxSize(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	rp.SetMaxSize(10)
	if rp.GetMaxSize() != 10 {
		t.Errorf("expected max size 10, got %d", rp.GetMaxSize())
	}

	// Setting 0 should default to 10
	rp.SetMaxSize(0)
	if rp.GetMaxSize() != 10 {
		t.Errorf("expected default max size 10, got %d", rp.GetMaxSize())
	}
}

func TestReaderPool_ActiveTracking(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	segmentName := "test_segment"
	reader := &SegmentReader{}

	// Initially not active
	if rp.IsActive(reader) {
		t.Error("expected reader to not be active")
	}

	// Manually add to pool and get
	rp.pool[segmentName] = append(rp.pool[segmentName], reader)
	rp.Get(segmentName)

	// Should be active now
	if !rp.IsActive(reader) {
		t.Error("expected reader to be active after get")
	}

	// Release
	rp.Put(segmentName, reader)

	// Should not be active anymore
	if rp.IsActive(reader) {
		t.Error("expected reader to not be active after put")
	}
}

func TestReaderPool_GetActiveCount(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	if rp.GetActiveCount() != 0 {
		t.Errorf("expected 0 active, got %d", rp.GetActiveCount())
	}

	// Manually add active readers
	rp.active[&SegmentReader{}] = true
	rp.active[&SegmentReader{}] = true

	if rp.GetActiveCount() != 2 {
		t.Errorf("expected 2 active, got %d", rp.GetActiveCount())
	}
}

func TestReaderPool_PoolSizeLimit(t *testing.T) {
	rp := NewReaderPool(2)
	defer rp.Close()

	segmentName := "test_segment"

	// Add readers up to limit
	reader1 := &SegmentReader{}
	reader2 := &SegmentReader{}
	reader3 := &SegmentReader{}

	rp.pool[segmentName] = []*SegmentReader{reader1, reader2}

	// Adding another should cause reader3 to be closed
	rp.active[reader3] = true
	err := rp.Put(segmentName, reader3)
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Pool should still be at max size
	if rp.GetSegmentSize(segmentName) != 2 {
		t.Errorf("expected segment size 2, got %d", rp.GetSegmentSize(segmentName))
	}
}

func TestReaderPool_ConcurrentAccess(t *testing.T) {
	rp := NewReaderPool(10)
	defer rp.Close()

	segmentName := "test_segment"
	done := make(chan bool)

	// Concurrent gets
	for i := 0; i < 100; i++ {
		go func() {
			rp.Get(segmentName)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestReaderPool_Cleanup(t *testing.T) {
	rp := NewReaderPool(5)
	defer rp.Close()

	segmentName := "test_segment"

	// Add a reader with old last access time
	rp.pool[segmentName] = []*SegmentReader{{}}
	rp.lastAccess[segmentName] = time.Now().Add(-2 * time.Hour)

	// Manually trigger cleanup by setting short cleanup interval
	// This is tested through the cleanupLoop, but we can't easily test it directly
	// So we'll just verify the pool is cleared after manual cleanup

	// Simulate cleanup logic
	cutoff := time.Now().Add(-1 * time.Hour)
	if rp.lastAccess[segmentName].Before(cutoff) {
		rp.ClearSegment(segmentName)
	}

	if rp.GetSegmentSize(segmentName) != 0 {
		t.Error("expected segment to be cleared after cleanup")
	}
}
