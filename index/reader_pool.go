package index

import (
	"fmt"
	"sync"
	"time"
)

// ReaderPool manages a pool of SegmentReader instances for efficient reuse
// during NRT (Near Real-Time) updates. It reduces allocation overhead
// by reusing readers instead of creating new ones.
// This is the Go port of Lucene's org.apache.lucene.index.ReaderPool.
type ReaderPool struct {
	mu sync.RWMutex

	// pool holds the available readers keyed by segment name
	pool map[string][]*SegmentReader

	// active tracks readers currently in use
	active map[*SegmentReader]bool

	// maxSize is the maximum number of readers to keep per segment
	maxSize int

	// isClosed indicates if the pool has been closed
	isClosed bool

	// lastAccess tracks when each segment pool was last accessed
	lastAccess map[string]time.Time

	// cleanupInterval is the interval for cleanup goroutine
	cleanupInterval time.Duration
}

// NewReaderPool creates a new ReaderPool with the given maximum size per segment.
// If maxSize is 0 or negative, a default value of 10 is used.
func NewReaderPool(maxSize int) *ReaderPool {
	if maxSize <= 0 {
		maxSize = 10
	}

	rp := &ReaderPool{
		pool:            make(map[string][]*SegmentReader),
		active:          make(map[*SegmentReader]bool),
		maxSize:         maxSize,
		lastAccess:      make(map[string]time.Time),
		cleanupInterval: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go rp.cleanupLoop()

	return rp
}

// Get retrieves a SegmentReader from the pool for the given segment.
// If no reader is available, it returns nil and the caller should create a new one.
func (rp *ReaderPool) Get(segmentName string) *SegmentReader {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isClosed {
		return nil
	}

	// Update last access time
	rp.lastAccess[segmentName] = time.Now()

	// Try to get a reader from the pool
	readers := rp.pool[segmentName]
	if len(readers) > 0 {
		// Get the last reader from the slice (most efficient)
		reader := readers[len(readers)-1]
		rp.pool[segmentName] = readers[:len(readers)-1]
		rp.active[reader] = true
		return reader
	}

	return nil
}

// Put returns a SegmentReader to the pool for reuse.
// If the pool is full for this segment, the reader is discarded.
func (rp *ReaderPool) Put(segmentName string, reader *SegmentReader) error {
	if reader == nil {
		return fmt.Errorf("cannot put nil reader")
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isClosed {
		return fmt.Errorf("reader pool is closed")
	}

	// Remove from active map
	delete(rp.active, reader)

	// Check if we should keep this reader
	readers := rp.pool[segmentName]
	if len(readers) >= rp.maxSize {
		// Pool is full, discard this reader
		reader.Close()
		return nil
	}

	// Add to pool
	rp.pool[segmentName] = append(readers, reader)
	rp.lastAccess[segmentName] = time.Now()

	return nil
}

// Acquire gets a reader from the pool or creates a new one using the provided factory.
// The factory is called when no reader is available in the pool.
func (rp *ReaderPool) Acquire(segmentName string, factory func() (*SegmentReader, error)) (*SegmentReader, error) {
	// Try to get from pool first
	reader := rp.Get(segmentName)
	if reader != nil {
		return reader, nil
	}

	// Create new reader using factory
	if factory == nil {
		return nil, fmt.Errorf("factory is required when pool is empty")
	}

	reader, err := factory()
	if err != nil {
		return nil, fmt.Errorf("factory failed to create reader: %w", err)
	}

	rp.mu.Lock()
	rp.active[reader] = true
	rp.mu.Unlock()

	return reader, nil
}

// Release returns a reader to the pool.
// This is a convenience method that calls Put.
func (rp *ReaderPool) Release(segmentName string, reader *SegmentReader) error {
	return rp.Put(segmentName, reader)
}

// Clear removes all readers from the pool and closes them.
func (rp *ReaderPool) Clear() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isClosed {
		return fmt.Errorf("reader pool is closed")
	}

	// Close all pooled readers
	for segmentName, readers := range rp.pool {
		for _, reader := range readers {
			if reader != nil {
				reader.Close()
			}
		}
		delete(rp.pool, segmentName)
	}

	// Clear last access times
	rp.lastAccess = make(map[string]time.Time)

	return nil
}

// ClearSegment removes all readers for a specific segment from the pool.
func (rp *ReaderPool) ClearSegment(segmentName string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isClosed {
		return fmt.Errorf("reader pool is closed")
	}

	readers := rp.pool[segmentName]
	for _, reader := range readers {
		if reader != nil {
			reader.Close()
		}
	}

	delete(rp.pool, segmentName)
	delete(rp.lastAccess, segmentName)

	return nil
}

// Close closes the reader pool and all its readers.
func (rp *ReaderPool) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isClosed {
		return nil // Already closed
	}

	rp.isClosed = true

	// Close all pooled readers
	for segmentName, readers := range rp.pool {
		for _, reader := range readers {
			if reader != nil {
				reader.Close()
			}
		}
		delete(rp.pool, segmentName)
	}

	// Note: We don't close active readers as they are still in use
	// The caller is responsible for releasing active readers

	return nil
}

// IsClosed returns true if the pool has been closed.
func (rp *ReaderPool) IsClosed() bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.isClosed
}

// GetSize returns the total number of readers in the pool.
func (rp *ReaderPool) GetSize() int {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	size := 0
	for _, readers := range rp.pool {
		size += len(readers)
	}
	return size
}

// GetSegmentSize returns the number of readers in the pool for a specific segment.
func (rp *ReaderPool) GetSegmentSize(segmentName string) int {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return len(rp.pool[segmentName])
}

// GetActiveCount returns the number of readers currently in use.
func (rp *ReaderPool) GetActiveCount() int {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return len(rp.active)
}

// GetMaxSize returns the maximum number of readers per segment.
func (rp *ReaderPool) GetMaxSize() int {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.maxSize
}

// SetMaxSize sets the maximum number of readers per segment.
// If the new size is smaller than the current pool size, excess readers are not removed immediately.
func (rp *ReaderPool) SetMaxSize(maxSize int) {
	if maxSize <= 0 {
		maxSize = 10
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.maxSize = maxSize
}

// cleanupLoop runs periodically to remove stale readers from the pool.
func (rp *ReaderPool) cleanupLoop() {
	ticker := time.NewTicker(rp.cleanupInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		rp.mu.Lock()
		if rp.isClosed {
			rp.mu.Unlock()
			return
		}

		// Remove stale segment pools (not accessed in the last hour)
		cutoff := time.Now().Add(-1 * time.Hour)
		for segmentName, lastAccess := range rp.lastAccess {
			if lastAccess.Before(cutoff) {
				// Close and remove stale readers
				readers := rp.pool[segmentName]
				for _, reader := range readers {
					if reader != nil {
						reader.Close()
					}
				}
				delete(rp.pool, segmentName)
				delete(rp.lastAccess, segmentName)
			}
		}
		rp.mu.Unlock()
	}
}

// IsActive returns true if the given reader is currently in use.
func (rp *ReaderPool) IsActive(reader *SegmentReader) bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.active[reader]
}
