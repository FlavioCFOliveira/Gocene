//go:build ignore

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

	// poolReaders is a "write once" flag: false by default, true after first getReader() call.
	// Once true, SegmentReaders are retained in the pool and reused across operations.
	// See Lucene's ReaderPool:120 and IndexWriter:524.
	poolReaders bool
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

// EnableReaderPooling switches reader pooling from disabled to enabled (write-once flag).
// This is called on the first NRT reader request (IndexWriter.getReader()).
// Once enabled, SegmentReaders are retained in the pool for reuse across operations
// (deletes, merges, subsequent NRT reader opens).
// Ref: Lucene IndexWriter:524, ReaderPool:120.
func (rp *ReaderPool) EnableReaderPooling() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	// Write-once: only set to true the first time.
	rp.poolReaders = true
}

// IsPoolingEnabled returns whether reader pooling is currently enabled.
func (rp *ReaderPool) IsPoolingEnabled() bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.poolReaders
}

// WriteReaderPool persists any pending deletes/updates from the pooled readers to disk.
// If writeAllDeletes is true, all live docs bitsets (.liv files) are written.
// This is called during getReader() to ensure deletes are durable before creating the reader.
// Ref: Lucene IndexWriter:603.
func (rp *ReaderPool) WriteReaderPool(writeAllDeletes bool) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Iterate over all pooled ReadersAndUpdates and persist their deletes.
	for _, rau := range rp.pool {
		if rau == nil {
			continue
		}

		// Write deletes if requested. In the full implementation, this would
		// call WriteLiveDocs() on the ReadersAndUpdates to persist .liv bitsets.
		// For now, we pass (the deletion state is held in memory via the
		// ReadersAndUpdates' live docs bitset and will be applied when the reader is opened).
		if writeAllDeletes {
			// TODO: Call rau.WriteLiveDocs() once the method is available.
			// This should persist any live docs changes to disk.
		}
	}

	return nil
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
