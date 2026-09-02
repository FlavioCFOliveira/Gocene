package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTReaderPool manages a pool of NRT readers for reuse.
// It helps reduce the overhead of creating and closing readers frequently.
type NRTReaderPool struct {
	mu sync.RWMutex

	// readers holds available readers in the pool
	readers []*NRTReader

	// maxSize is the maximum number of readers to keep in the pool
	maxSize int

	// currentSize is the current number of readers in the pool
	currentSize int

	// isOpen indicates if the pool is open
	isOpen atomic.Bool

	// hitCount tracks successful borrows
	hitCount int64

	// missCount tracks failed borrows (had to create new)
	missCount int64

	// returnCount tracks returns to the pool
	returnCount int64

	// factory is called to create new readers when pool is empty
	factory func() (*NRTReader, error)

	// closeFunc is called when a reader is removed from the pool
	closeFunc func(*NRTReader)

	// maxIdleTime is the maximum time a reader can be idle before removal
	maxIdleTime time.Duration

	// lastUsed tracks when each reader was last used
	lastUsed map[*NRTReader]time.Time
}

// NewNRTReaderPool creates a new NRTReaderPool.
func NewNRTReaderPool(maxSize int, factory func() (*NRTReader, error)) (*NRTReaderPool, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("maxSize must be positive")
	}

	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	pool := &NRTReaderPool{
		maxSize:     maxSize,
		readers:     make([]*NRTReader, 0, maxSize),
		factory:     factory,
		lastUsed:    make(map[*NRTReader]time.Time),
		maxIdleTime: 5 * time.Minute,
	}

	pool.isOpen.Store(true)

	return pool, nil
}

// SetCloseFunc sets the function called when a reader is removed from the pool.
func (p *NRTReaderPool) SetCloseFunc(closeFunc func(*NRTReader)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closeFunc = closeFunc
}

// Borrow gets a reader from the pool or creates a new one.
func (p *NRTReaderPool) Borrow() (*NRTReader, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isOpen.Load() {
		return nil, fmt.Errorf("pool is closed")
	}

	// Try to get an available reader
	if len(p.readers) > 0 {
		reader := p.readers[len(p.readers)-1]
		p.readers = p.readers[:len(p.readers)-1]
		delete(p.lastUsed, reader)
		p.currentSize--
		p.hitCount++
		return reader, nil
	}

	p.missCount++

	// Create new reader using factory
	p.mu.Unlock()
	reader, err := p.factory()
	p.mu.Lock()

	if err != nil {
		return nil, fmt.Errorf("factory error: %w", err)
	}

	return reader, nil
}

// Return returns a reader to the pool.
// If the pool is full or closed, the reader is closed.
func (p *NRTReaderPool) Return(reader *NRTReader) error {
	if reader == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isOpen.Load() {
		// Pool is closed, close the reader
		if p.closeFunc != nil {
			p.closeFunc(reader)
		} else {
			reader.Close()
		}
		return nil
	}

	// Check if pool is at capacity
	if p.currentSize >= p.maxSize {
		// Pool is full, close the reader
		if p.closeFunc != nil {
			p.closeFunc(reader)
		} else {
			reader.Close()
		}
		return nil
	}

	// Add reader back to pool
	p.readers = append(p.readers, reader)
	p.lastUsed[reader] = time.Now()
	p.currentSize++
	p.returnCount++

	return nil
}

// GetSize returns the current number of readers in the pool.
func (p *NRTReaderPool) GetSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.currentSize
}

// GetMaxSize returns the maximum pool size.
func (p *NRTReaderPool) GetMaxSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.maxSize
}

// SetMaxSize sets the maximum pool size.
func (p *NRTReaderPool) SetMaxSize(maxSize int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxSize = maxSize
}

// GetHitCount returns the number of successful borrows.
func (p *NRTReaderPool) GetHitCount() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.hitCount
}

// GetMissCount returns the number of failed borrows.
func (p *NRTReaderPool) GetMissCount() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.missCount
}

// GetHitRatio returns the hit ratio (0-1).
func (p *NRTReaderPool) GetHitRatio() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := p.hitCount + p.missCount
	if total == 0 {
		return 0
	}

	return float64(p.hitCount) / float64(total)
}

// GetReturnCount returns the number of returns.
func (p *NRTReaderPool) GetReturnCount() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.returnCount
}

// Clear removes all readers from the pool.
func (p *NRTReaderPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, reader := range p.readers {
		if p.closeFunc != nil {
			p.closeFunc(reader)
		} else {
			reader.Close()
		}
	}

	p.readers = p.readers[:0]
	p.currentSize = 0
	p.lastUsed = make(map[*NRTReader]time.Time)
}

// CleanupIdle removes readers that have been idle for longer than maxIdleTime.
func (p *NRTReaderPool) CleanupIdle() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	removed := 0
	active := make([]*NRTReader, 0, len(p.readers))

	for _, reader := range p.readers {
		if lastUsed, ok := p.lastUsed[reader]; ok {
			if now.Sub(lastUsed) > p.maxIdleTime {
				// Reader is idle, close it
				if p.closeFunc != nil {
					p.closeFunc(reader)
				} else {
					reader.Close()
				}
				delete(p.lastUsed, reader)
				removed++
				p.currentSize--
				continue
			}
		}
		active = append(active, reader)
	}

	p.readers = active
	return removed
}

// SetMaxIdleTime sets the maximum idle time.
func (p *NRTReaderPool) SetMaxIdleTime(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxIdleTime = duration
}

// GetMaxIdleTime returns the maximum idle time.
func (p *NRTReaderPool) GetMaxIdleTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.maxIdleTime
}

// IsOpen returns true if the pool is open.
func (p *NRTReaderPool) IsOpen() bool {
	return p.isOpen.Load()
}

// Close closes the pool and all readers in it.
func (p *NRTReaderPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isOpen.Load() {
		return nil
	}

	p.isOpen.Store(false)

	// Close all readers in the pool
	for _, reader := range p.readers {
		if p.closeFunc != nil {
			p.closeFunc(reader)
		} else {
			reader.Close()
		}
	}

	p.readers = nil
	p.currentSize = 0
	p.lastUsed = nil

	return nil
}

// String returns a string representation of the NRTReaderPool.
func (p *NRTReaderPool) String() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return fmt.Sprintf("NRTReaderPool{open=%v, size=%d/%d, hits=%d, misses=%d, ratio=%.2f}",
		p.isOpen.Load(), p.currentSize, p.maxSize, p.hitCount, p.missCount, p.GetHitRatio())
}
