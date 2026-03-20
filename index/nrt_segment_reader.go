package index

import (
	"fmt"
	"sync"
)

// NRTSegmentReader is a SegmentReader variant that supports Near Real-Time (NRT) updates.
// It extends SegmentReader with the ability to handle live document changes
// and provides thread-safe access to the underlying segment data during indexing.
// This is the Go port of Lucene's org.apache.lucene.index.NRTSegmentReader.
type NRTSegmentReader struct {
	*SegmentReader

	// liveDocs tracks which documents are currently live (not deleted)
	liveDocs *LiveDocs

	// pendingDeletes tracks documents that are pending deletion
	pendingDeletes *PendingDeletes

	// mu protects liveDocs and pendingDeletes
	mu sync.RWMutex

	// isNRT indicates if this reader supports NRT operations
	isNRT bool

	// version tracks the NRT version of this reader
	version int64

	// writer is the IndexWriter for NRT operations (may be nil)
	writer *IndexWriter
}

// LiveDocs tracks live documents in an NRT segment.
type LiveDocs struct {
	// bits is a bitset where 1 means the document is live
	bits []uint64
	// numDocs is the count of live documents
	numDocs int
	// totalDocs is the total document count (including deleted)
	totalDocs int
}

// PendingDeletes tracks documents pending deletion.
type PendingDeletes struct {
	// docIDs is the set of document IDs pending deletion
	docIDs map[int]bool
	// mu protects docIDs
	mu sync.Mutex
}

// NewNRTSegmentReader creates a new NRTSegmentReader from an existing SegmentReader.
func NewNRTSegmentReader(segmentReader *SegmentReader, writer *IndexWriter) (*NRTSegmentReader, error) {
	if segmentReader == nil {
		return nil, fmt.Errorf("segment reader cannot be nil")
	}

	totalDocs := segmentReader.NumDocs()
	nrtr := &NRTSegmentReader{
		SegmentReader:  segmentReader,
		isNRT:          true,
		version:        1,
		writer:         writer,
		liveDocs:       newLiveDocs(totalDocs),
		pendingDeletes: newPendingDeletes(),
	}

	return nrtr, nil
}

// newLiveDocs creates a new LiveDocs with all documents marked as live.
func newLiveDocs(totalDocs int) *LiveDocs {
	// Calculate number of uint64 needed for bitset
	numWords := (totalDocs + 63) / 64
	bits := make([]uint64, numWords)

	// Mark all documents as live (set all bits to 1)
	for i := range bits {
		bits[i] = ^uint64(0)
	}

	// Clear excess bits in the last word
	if totalDocs%64 != 0 {
		excessBits := 64 - (totalDocs % 64)
		bits[len(bits)-1] &= ^uint64(0) >> excessBits
	}

	return &LiveDocs{
		bits:      bits,
		numDocs:   totalDocs,
		totalDocs: totalDocs,
	}
}

// newPendingDeletes creates a new PendingDeletes.
func newPendingDeletes() *PendingDeletes {
	return &PendingDeletes{
		docIDs: make(map[int]bool),
	}
}

// IsLive returns true if the given document ID is live (not deleted).
func (r *NRTSegmentReader) IsLive(docID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return true // All docs are live if no live docs tracking
	}

	if docID < 0 || docID >= r.liveDocs.totalDocs {
		return false
	}

	wordIndex := docID / 64
	bitIndex := uint(docID % 64)

	return r.liveDocs.bits[wordIndex]&(1<<bitIndex) != 0
}

// MarkDeleted marks a document as deleted.
// Returns true if the document was previously live.
func (r *NRTSegmentReader) MarkDeleted(docID int) (bool, error) {
	if docID < 0 || docID >= r.liveDocs.totalDocs {
		return false, fmt.Errorf("docID %d out of range [0, %d)", docID, r.liveDocs.totalDocs)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.liveDocs == nil {
		return false, fmt.Errorf("live docs not initialized")
	}

	wordIndex := docID / 64
	bitIndex := uint(docID % 64)
	bitMask := uint64(1 << bitIndex)

	// Check if already deleted
	if r.liveDocs.bits[wordIndex]&bitMask == 0 {
		return false, nil // Already deleted
	}

	// Mark as deleted
	r.liveDocs.bits[wordIndex] &^= bitMask
	r.liveDocs.numDocs--

	// Track in pending deletes
	r.pendingDeletes.mu.Lock()
	r.pendingDeletes.docIDs[docID] = true
	r.pendingDeletes.mu.Unlock()

	return true, nil
}

// NumDocs returns the number of live documents.
func (r *NRTSegmentReader) NumDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return r.SegmentReader.NumDocs()
	}

	return r.liveDocs.numDocs
}

// HasDeletions returns true if this reader has deleted documents.
func (r *NRTSegmentReader) HasDeletions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return false
	}

	return r.liveDocs.numDocs < r.liveDocs.totalDocs
}

// GetLiveDocs returns a copy of the live documents bitset.
func (r *NRTSegmentReader) GetLiveDocs() []uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return nil
	}

	result := make([]uint64, len(r.liveDocs.bits))
	copy(result, r.liveDocs.bits)
	return result
}

// GetPendingDeletes returns the document IDs pending deletion.
func (r *NRTSegmentReader) GetPendingDeletes() []int {
	r.pendingDeletes.mu.Lock()
	defer r.pendingDeletes.mu.Unlock()

	result := make([]int, 0, len(r.pendingDeletes.docIDs))
	for docID := range r.pendingDeletes.docIDs {
		result = append(result, docID)
	}
	return result
}

// ClearPendingDeletes clears the pending deletes.
func (r *NRTSegmentReader) ClearPendingDeletes() {
	r.pendingDeletes.mu.Lock()
	defer r.pendingDeletes.mu.Unlock()

	r.pendingDeletes.docIDs = make(map[int]bool)
}

// IsNRT returns true if this is an NRT reader.
func (r *NRTSegmentReader) IsNRT() bool {
	return r.isNRT
}

// GetVersion returns the NRT version of this reader.
func (r *NRTSegmentReader) GetVersion() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// IncrementVersion increments the NRT version.
func (r *NRTSegmentReader) IncrementVersion() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.version++
}

// GetWriter returns the IndexWriter for NRT operations, or nil if not in NRT mode.
func (r *NRTSegmentReader) GetWriter() *IndexWriter {
	return r.writer
}

// Clone creates a shallow copy of this NRTSegmentReader for use in another thread.
// The cloned reader shares the underlying data but has its own live docs reference.
func (r *NRTSegmentReader) Clone() (*NRTSegmentReader, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &NRTSegmentReader{
		SegmentReader:  r.SegmentReader,
		liveDocs:       r.liveDocs,
		pendingDeletes: newPendingDeletes(), // Fresh pending deletes
		isNRT:          r.isNRT,
		version:        r.version,
		writer:         r.writer,
	}, nil
}

// RefreshLiveDocs refreshes the live docs from the underlying reader.
// This is called when the segment has been updated.
func (r *NRTSegmentReader) RefreshLiveDocs() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalDocs := r.SegmentReader.NumDocs()
	r.liveDocs = newLiveDocs(totalDocs)
	r.version++

	return nil
}

// Close closes this NRTSegmentReader.
// Note: This does not close the underlying SegmentReader.
func (r *NRTSegmentReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isNRT = false
	r.liveDocs = nil
	r.pendingDeletes = nil
	r.writer = nil

	return nil
}

// GetTotalDocs returns the total number of documents (including deleted).
func (r *NRTSegmentReader) GetTotalDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return r.SegmentReader.NumDocs()
	}

	return r.liveDocs.totalDocs
}

// GetNumDeleted returns the number of deleted documents.
func (r *NRTSegmentReader) GetNumDeleted() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return 0
	}

	return r.liveDocs.totalDocs - r.liveDocs.numDocs
}

// ForEachLiveDoc iterates over all live documents and calls the provided function.
func (r *NRTSegmentReader) ForEachLiveDoc(fn func(docID int) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return nil
	}

	for docID := 0; docID < r.liveDocs.totalDocs; docID++ {
		if r.isLiveUnlocked(docID) {
			if err := fn(docID); err != nil {
				return err
			}
		}
	}

	return nil
}

// isLiveUnlocked checks if a document is live without locking (caller must hold lock).
func (r *NRTSegmentReader) isLiveUnlocked(docID int) bool {
	if r.liveDocs == nil || docID < 0 || docID >= r.liveDocs.totalDocs {
		return false
	}

	wordIndex := docID / 64
	bitIndex := uint(docID % 64)

	return r.liveDocs.bits[wordIndex]&(1<<bitIndex) != 0
}

// GetLiveDocCount returns the number of live documents.
// This is an alias for NumDocs().
func (r *NRTSegmentReader) GetLiveDocCount() int {
	return r.NumDocs()
}
