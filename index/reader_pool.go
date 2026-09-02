package index

import (
	"fmt"
	"sync"
)

// ReaderPool manages a pool of ReadersAndUpdates instances.
// This is the Go port of Lucene's org.apache.lucene.index.ReaderPool.
type ReaderPool struct {
	mu sync.Mutex

	// pool holds the ReadersAndUpdates instances keyed by SegmentCommitInfo.
	pool map[*SegmentCommitInfo]*ReadersAndUpdates
}

// NewReaderPool creates a new ReaderPool.
func NewReaderPool() *ReaderPool {
	return &ReaderPool{
		pool: make(map[*SegmentCommitInfo]*ReadersAndUpdates),
	}
}

// Get retrieves a ReadersAndUpdates instance from the pool.
// If create is true and no instance exists, it creates a new one.
func (rp *ReaderPool) Get(info *SegmentCommitInfo, create bool, factory func(*SegmentCommitInfo) *ReadersAndUpdates) *ReadersAndUpdates {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rau, ok := rp.pool[info]; ok {
		rau.IncRef()
		return rau
	}

	if !create || factory == nil {
		return nil
	}

	rau := factory(info)
	rp.pool[info] = rau
	// NewReadersAndUpdates already starts with refCount 1.
	// We need an extra ref for the caller.
	rau.IncRef()
	return rau
}

// Release decrements the ref count of the ReadersAndUpdates instance.
// If the ref count reaches 1 (only the pool holds it), it may be removed from the pool.
func (rp *ReaderPool) Release(rau *ReadersAndUpdates, assertInfoLive bool) bool {
	if rau == nil {
		return false
	}

	rau.DecRef()

	// In Lucene, if the segment is no longer live, it's removed from the pool.
	// For now, we only remove if it's specifically requested or the ref count is low.
	// Simplified: we keep it in the pool until a explicit cleanup or segment removal.
	return true
}

// Clear removes all instances from the pool.
func (rp *ReaderPool) Clear() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	for info, rau := range rp.pool {
		rau.DecRef()
		delete(rp.pool, info)
	}
}
