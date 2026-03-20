package index

import (
	"testing"
	"time"
)

func TestNewNRTReaderPool(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, err := NewNRTReaderPool(10, factory)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected pool to not be nil")
	}

	if pool.GetMaxSize() != 10 {
		t.Errorf("expected max size 10, got %d", pool.GetMaxSize())
	}

	if pool.GetSize() != 0 {
		t.Errorf("expected size 0, got %d", pool.GetSize())
	}

	if !pool.IsOpen() {
		t.Error("expected pool to be open")
	}
}

func TestNewNRTReaderPool_InvalidMaxSize(t *testing.T) {
	_, err := NewNRTReaderPool(0, func() (*NRTReader, error) {
		return &NRTReader{}, nil
	})
	if err == nil {
		t.Error("expected error for zero max size")
	}

	_, err = NewNRTReaderPool(-1, func() (*NRTReader, error) {
		return &NRTReader{}, nil
	})
	if err == nil {
		t.Error("expected error for negative max size")
	}
}

func TestNewNRTReaderPool_NilFactory(t *testing.T) {
	_, err := NewNRTReaderPool(10, nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
}

func TestNRTReaderPool_BorrowReturn(t *testing.T) {
	factoryCalled := 0
	factory := func() (*NRTReader, error) {
		factoryCalled++
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	// First borrow - should call factory
	reader1, err := pool.Borrow()
	if err != nil {
		t.Fatalf("failed to borrow: %v", err)
	}

	if factoryCalled != 1 {
		t.Errorf("expected factory called 1 time, got %d", factoryCalled)
	}

	if pool.GetHitCount() != 0 {
		t.Errorf("expected 0 hits, got %d", pool.GetHitCount())
	}

	if pool.GetMissCount() != 1 {
		t.Errorf("expected 1 miss, got %d", pool.GetMissCount())
	}

	// Return the reader
	err = pool.Return(reader1)
	if err != nil {
		t.Fatalf("failed to return: %v", err)
	}

	if pool.GetSize() != 1 {
		t.Errorf("expected size 1, got %d", pool.GetSize())
	}

	// Second borrow - should use pooled reader
	reader2, err := pool.Borrow()
	if err != nil {
		t.Fatalf("failed to borrow: %v", err)
	}

	if factoryCalled != 1 {
		t.Errorf("expected factory still called 1 time, got %d", factoryCalled)
	}

	if pool.GetHitCount() != 1 {
		t.Errorf("expected 1 hit, got %d", pool.GetHitCount())
	}

	if pool.GetSize() != 0 {
		t.Errorf("expected size 0, got %d", pool.GetSize())
	}

	pool.Return(reader2)
}

func TestNRTReaderPool_Borrow_Closed(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	pool.Close()

	_, err := pool.Borrow()
	if err == nil {
		t.Error("expected error when borrowing from closed pool")
	}
}

func TestNRTReaderPool_Return_Nil(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	err := pool.Return(nil)
	if err != nil {
		t.Errorf("expected no error for nil return: %v", err)
	}
}

func TestNRTReaderPool_Return_PoolFull(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(2, factory)
	defer pool.Close()

	// Add readers to fill pool
	reader1 := &NRTReader{}
	reader2 := &NRTReader{}
	reader3 := &NRTReader{}

	pool.Return(reader1)
	pool.Return(reader2)

	if pool.GetSize() != 2 {
		t.Errorf("expected size 2, got %d", pool.GetSize())
	}

	// Return another reader - should be closed/discarded
	pool.Return(reader3)

	// Size should still be 2
	if pool.GetSize() != 2 {
		t.Errorf("expected size 2, got %d", pool.GetSize())
	}
}

func TestNRTReaderPool_Clear(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)

	// Add some readers
	pool.Return(&NRTReader{})
	pool.Return(&NRTReader{})

	if pool.GetSize() != 2 {
		t.Errorf("expected size 2, got %d", pool.GetSize())
	}

	pool.Clear()

	if pool.GetSize() != 0 {
		t.Errorf("expected size 0 after clear, got %d", pool.GetSize())
	}
}

func TestNRTReaderPool_CleanupIdle(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	// Set short idle time
	pool.SetMaxIdleTime(50 * time.Millisecond)

	// Add readers
	pool.Return(&NRTReader{})
	pool.Return(&NRTReader{})

	if pool.GetSize() != 2 {
		t.Errorf("expected size 2, got %d", pool.GetSize())
	}

	// Wait for idle timeout
	time.Sleep(100 * time.Millisecond)

	// Cleanup idle readers
	removed := pool.CleanupIdle()

	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	if pool.GetSize() != 0 {
		t.Errorf("expected size 0, got %d", pool.GetSize())
	}
}

func TestNRTReaderPool_SetMaxSize(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	pool.SetMaxSize(20)

	if pool.GetMaxSize() != 20 {
		t.Errorf("expected max size 20, got %d", pool.GetMaxSize())
	}
}

func TestNRTReaderPool_HitRatio(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	// Initially no ratio
	if pool.GetHitRatio() != 0 {
		t.Errorf("expected hit ratio 0, got %f", pool.GetHitRatio())
	}

	// Borrow (miss)
	reader, _ := pool.Borrow()

	// Return
	pool.Return(reader)

	// Borrow again (hit)
	pool.Borrow()

	// Expected: 1 hit, 1 miss = 0.5 ratio
	ratio := pool.GetHitRatio()
	if ratio != 0.5 {
		t.Errorf("expected hit ratio 0.5, got %f", ratio)
	}
}

func TestNRTReaderPool_Close(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)

	// Add some readers
	pool.Return(&NRTReader{})
	pool.Return(&NRTReader{})

	err := pool.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if pool.IsOpen() {
		t.Error("expected pool to be closed")
	}

	// Close again should not error
	err = pool.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestNRTReaderPool_String(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(10, factory)
	defer pool.Close()

	str := pool.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestNRTReaderPool_ConcurrentOperations(t *testing.T) {
	factory := func() (*NRTReader, error) {
		return &NRTReader{}, nil
	}

	pool, _ := NewNRTReaderPool(20, factory)
	defer pool.Close()

	done := make(chan bool, 4)

	// Borrow goroutine
	go func() {
		for i := 0; i < 20; i++ {
			reader, _ := pool.Borrow()
			time.Sleep(5 * time.Millisecond)
			pool.Return(reader)
		}
		done <- true
	}()

	// Get stats goroutine
	go func() {
		for i := 0; i < 20; i++ {
			pool.GetSize()
			pool.GetHitCount()
			pool.GetMissCount()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get ratio goroutine
	go func() {
		for i := 0; i < 20; i++ {
			pool.GetHitRatio()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Get return count goroutine
	go func() {
		for i := 0; i < 20; i++ {
			pool.GetReturnCount()
			time.Sleep(5 * time.Millisecond)
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
