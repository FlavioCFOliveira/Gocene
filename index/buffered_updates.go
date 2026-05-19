// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// BufferedUpdates
// Source: lucene/core/src/java/org/apache/lucene/index/BufferedUpdates.java
// Purpose: Holds buffered deletes and updates, keyed by docID, term, or query,
// for a single segment. Used by DocumentsWriter to accumulate pending deletes
// against the to-be-flushed segment; once the deletes and updates are pushed
// on flush they are converted to a FrozenBufferedUpdates instance and pushed
// to the BufferedUpdatesStream.

package index

import (
	"sort"
)

// BytesPerDelQuery is the estimated bytes used per buffered delete query.
//
// Rough logic mirrors Lucene's BufferedUpdates.BYTES_PER_DEL_QUERY:
// HashMap entry overhead (5 references), 2 object headers, 2 int fields,
// plus a constant 24 bytes accounting for the typical Query object footprint.
const BytesPerDelQuery = 5*8 + 2*16 + 2*4 + 24

// BufferedUpdates holds buffered deletes and updates for a single segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.BufferedUpdates.
type BufferedUpdates struct {
	segmentName     string
	deleteTerms     *DeletedTerms
	deleteQueries   map[Query]int
	bytesUsed       int64
	numFieldUpdates int
}

// NewBufferedUpdates creates a new BufferedUpdates instance bound to the
// given segment name.
func NewBufferedUpdates(segmentName string) *BufferedUpdates {
	return &BufferedUpdates{
		segmentName:   segmentName,
		deleteTerms:   NewDeletedTerms(),
		deleteQueries: make(map[Query]int),
	}
}

// SegmentName returns the segment name this BufferedUpdates is bound to.
func (bu *BufferedUpdates) SegmentName() string {
	return bu.segmentName
}

// RamBytesUsed returns the estimated RAM usage of this BufferedUpdates,
// including its DeletedTerms.
func (bu *BufferedUpdates) RamBytesUsed() int64 {
	return bu.bytesUsed + bu.deleteTerms.RamBytesUsed()
}

// Any reports whether any deletes or field updates have been buffered.
func (bu *BufferedUpdates) Any() bool {
	return bu.deleteTerms.Size() > 0 || len(bu.deleteQueries) > 0 || bu.numFieldUpdates > 0
}

// AddQuery records a query-based delete against the given upper-bound docID.
// Bytes accounting is incremented only the first time the query is seen.
func (bu *BufferedUpdates) AddQuery(query Query, docIDUpto int) {
	if _, exists := bu.deleteQueries[query]; !exists {
		bu.bytesUsed += BytesPerDelQuery
	}
	bu.deleteQueries[query] = docIDUpto
}

// AddTerm records a term-based delete against the given upper-bound docID.
//
// If the term has already been recorded with a higher docID upper bound the
// new value is dropped, matching Lucene's monotonic-replace guard that
// prevents lower-docID inserts from racing past higher-docID ones.
func (bu *BufferedUpdates) AddTerm(term *Term, docIDUpto int) {
	current := bu.deleteTerms.Get(term)
	if current != -1 && docIDUpto < current {
		// Only record the new number if it's greater than the current one.
		// This matches Lucene's guard against multi-threaded out-of-order
		// replacement of the same document.
		return
	}
	bu.deleteTerms.Put(term, docIDUpto)
}

// ClearDeleteTerms clears all term deletes and reclaims their RAM accounting.
// Buffered queries and field updates are left untouched.
func (bu *BufferedUpdates) ClearDeleteTerms() {
	bu.bytesUsed -= bu.deleteTerms.RamBytesUsed()
	bu.deleteTerms.Clear()
}

// Clear discards every buffered delete and field update and resets accounting.
func (bu *BufferedUpdates) Clear() {
	bu.deleteTerms.Clear()
	bu.deleteQueries = make(map[Query]int)
	bu.numFieldUpdates = 0
	bu.bytesUsed = 0
}

// DeletedTerms holds the deleted-term -> docID-upper-bound mapping for a
// single segment, partitioned by field name. This is the Go port of
// Lucene's BufferedUpdates.DeletedTerms inner class.
type DeletedTerms struct {
	terms     map[string]map[string]int // field -> term bytes -> docID
	bytesUsed int64
}

// NewDeletedTerms creates an empty DeletedTerms.
func NewDeletedTerms() *DeletedTerms {
	return &DeletedTerms{
		terms: make(map[string]map[string]int),
	}
}

// Get returns the most recent docID upper bound recorded for the given term,
// or -1 if the term is not present.
func (dt *DeletedTerms) Get(term *Term) int {
	fieldMap, exists := dt.terms[term.Field]
	if !exists {
		return -1
	}
	docID, exists := fieldMap[string(term.Bytes.ValidBytes())]
	if !exists {
		return -1
	}
	return docID
}

// Put records the given docID upper bound for the term, replacing any
// previously stored value. RAM accounting is updated for each newly
// observed field and term.
func (dt *DeletedTerms) Put(term *Term, docID int) {
	fieldMap, exists := dt.terms[term.Field]
	if !exists {
		fieldMap = make(map[string]int)
		dt.terms[term.Field] = fieldMap
		dt.bytesUsed += int64(len(term.Field)) + 16 // String overhead estimate
	}
	termBytes := string(term.Bytes.ValidBytes())
	if _, exists := fieldMap[termBytes]; !exists {
		dt.bytesUsed += int64(len(termBytes)) + 4 + 16 // bytes + int + map overhead
	}
	fieldMap[termBytes] = docID
}

// Size returns the total number of unique deleted terms across all fields.
func (dt *DeletedTerms) Size() int {
	count := 0
	for _, fieldMap := range dt.terms {
		count += len(fieldMap)
	}
	return count
}

// IsEmpty reports whether no deleted terms are currently held.
func (dt *DeletedTerms) IsEmpty() bool {
	return dt.Size() == 0
}

// Clear discards all deleted terms and resets RAM accounting to zero.
func (dt *DeletedTerms) Clear() {
	dt.terms = make(map[string]map[string]int)
	dt.bytesUsed = 0
}

// RamBytesUsed returns the estimated RAM usage of this DeletedTerms.
func (dt *DeletedTerms) RamBytesUsed() int64 {
	return dt.bytesUsed
}

// ForEachOrdered returns every stored entry sorted by field name and then by
// term bytes, matching the iteration contract of Lucene's forEachOrdered.
func (dt *DeletedTerms) ForEachOrdered() []TermEntry {
	fields := make([]string, 0, len(dt.terms))
	for field := range dt.terms {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	var entries []TermEntry
	for _, field := range fields {
		fieldMap := dt.terms[field]
		termBytes := make([]string, 0, len(fieldMap))
		for tb := range fieldMap {
			termBytes = append(termBytes, tb)
		}
		sort.Strings(termBytes)

		for _, tb := range termBytes {
			entries = append(entries, TermEntry{
				Field: field,
				Bytes: []byte(tb),
				Value: fieldMap[tb],
			})
		}
	}
	return entries
}

// GetPool returns the backing byte pool. The current Gocene port stores term
// bytes inside the field-keyed map and does not allocate an internal pool, so
// this method returns nil. It is kept for parity with Lucene's
// DeletedTerms.getPool, which is documented as visible-for-testing.
func (dt *DeletedTerms) GetPool() *ByteBlockPool {
	return nil
}

// ByteBlockPool is a minimal placeholder mirroring Lucene's
// util.ByteBlockPool surface area used by DeletedTerms.GetPool. The full
// util.ByteBlockPool is not exposed here to avoid forcing callers of
// BufferedUpdates into the util package's pool lifecycle.
type ByteBlockPool struct {
	buffer []byte
}

// Buffer returns the underlying byte buffer, or nil when the pool is unset.
func (bp *ByteBlockPool) Buffer() []byte {
	if bp == nil {
		return nil
	}
	return bp.buffer
}

// TermEntry is an ordered entry yielded by DeletedTerms.ForEachOrdered.
type TermEntry struct {
	Field string
	Bytes []byte
	Value int
}
