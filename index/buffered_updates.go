// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BufferedUpdates holds buffered deletes and updates, by docID, term or query for a single segment.
// This is used to hold buffered pending deletes and updates against the to-be-flushed segment.
// Once the deletes and updates are pushed (on flush in DocumentsWriter), they are converted to a
// FrozenBufferedUpdates instance and pushed to the BufferedUpdatesStream.
type BufferedUpdates struct {
	// numFieldUpdates is the number of field updates
	numFieldUpdates atomic.Int32

	// deleteTerms holds deleted terms
	deleteTerms *deletedTerms

	// deleteQueries holds deleted queries. Using a slice of pairs to avoid issues with
	// interface comparability as map keys.
	deleteQueries []queryDelete

	// fieldUpdates holds updates for each field
	fieldUpdates map[string]*FieldUpdatesBuffer

	// gen is the generation of the segment
	gen int64

	// segmentName is the name of the segment
	segmentName string

	// bytesUsed tracks the total bytes used for deletes
	bytesUsed *util.Counter

	// fieldUpdatesBytesUsed tracks the total bytes used for field updates
	fieldUpdatesBytesUsed *util.Counter
}

type queryDelete struct {
	query   Query
	docUpTo int
}

const bytesPerDelQuery = 64 // Rough estimate mirroring Lucene's BYTES_PER_DEL_QUERY

func NewBufferedUpdates(segmentName string) *BufferedUpdates {
	return &BufferedUpdates{
		segmentName:           segmentName,
		deleteTerms:           newDeletedTerms(),
		deleteQueries:         make([]queryDelete, 0),
		fieldUpdates:          make(map[string]*FieldUpdatesBuffer),
		bytesUsed:             util.NewCounter(),
		fieldUpdatesBytesUsed: util.NewCounter(),
	}
}

func (b *BufferedUpdates) String() string {
	s := fmt.Sprintf("gen=%d", b.gen)
	if !b.deleteTerms.isEmpty() {
		s += fmt.Sprintf(" %d unique deleted terms ", b.deleteTerms.size())
	}
	if len(b.deleteQueries) != 0 {
		s += fmt.Sprintf(" %d deleted queries", len(b.deleteQueries))
	}
	if b.numFieldUpdates.Load() != 0 {
		s += fmt.Sprintf(" %d field updates", b.numFieldUpdates.Load())
	}
	if b.bytesUsed.Get() != 0 {
		s += fmt.Sprintf(" bytesUsed=%d", b.bytesUsed.Get())
	}
	return s
}

func (b *BufferedUpdates) AddQuery(query Query, docIDUpTo int) {
	// Check if query already exists
	for i, qd := range b.deleteQueries {
		if qd.query.Equals(query) {
			b.deleteQueries[i].docUpTo = docIDUpTo
			return
		}
	}
	// New query
	b.deleteQueries = append(b.deleteQueries, queryDelete{query: query, docUpTo: docIDUpTo})
	b.bytesUsed.AddAndGet(bytesPerDelQuery)
}

func (b *BufferedUpdates) AddTerm(term Term, docIDUpTo int) {
	current := b.deleteTerms.get(term)
	if current != -1 && docIDUpTo < current {
		return
	}
	b.deleteTerms.put(term, docIDUpTo)
}

func (b *BufferedUpdates) AddNumericUpdate(update *NumericDocValuesUpdate, docIDUpTo int) {
	buffer, ok := b.fieldUpdates[update.Field()]
	if !ok {
		var err error
		buffer, err = NewFieldUpdatesBufferNumeric(b.bytesUsed, update.Term(), docIDUpTo, update.value, update.HasValue())
		if err != nil {
			panic(err)
		}
		b.fieldUpdates[update.Field()] = buffer
	}
	if update.HasValue() {
		buffer.AddUpdate(update.Term(), update.value, docIDUpTo)
	} else {
		buffer.AddNoValue(update.Term(), docIDUpTo)
	}
	b.numFieldUpdates.Add(1)
}

func (b *BufferedUpdates) AddBinaryUpdate(update *BinaryDocValuesUpdate, docIDUpTo int) {
	buffer, ok := b.fieldUpdates[update.Field()]
	if !ok {
		var err error
		buffer, err = NewFieldUpdatesBufferBinary(b.bytesUsed, update.Term(), docIDUpTo, util.NewBytesRef(update.value), update.HasValue())
		if err != nil {
			panic(err)
		}
		b.fieldUpdates[update.Field()] = buffer
	}
	if update.HasValue() {
		buffer.AddBinaryUpdate(update.Term(), util.NewBytesRef(update.value), docIDUpTo)
	} else {
		buffer.AddNoValue(update.Term(), docIDUpTo)
	}
	b.numFieldUpdates.Add(1)
}

func (b *BufferedUpdates) ClearDeleteTerms() {
	b.deleteTerms.clear()
}

func (b *BufferedUpdates) Clear() {
	b.deleteTerms.clear()
	b.deleteQueries = b.deleteQueries[:0]
	b.numFieldUpdates.Store(0)
	b.fieldUpdates = make(map[string]*FieldUpdatesBuffer)
	b.bytesUsed = util.NewCounter()
	b.fieldUpdatesBytesUsed = util.NewCounter()
}

func (b *BufferedUpdates) Any() bool {
	return !b.deleteTerms.isEmpty() || len(b.deleteQueries) > 0 || b.numFieldUpdates.Load() > 0
}

func (b *BufferedUpdates) RamBytesUsed() int64 {
	return b.bytesUsed.Get() + b.fieldUpdatesBytesUsed.Get() + b.deleteTerms.ramBytesUsed()
}

type deletedTerms struct {
	bytesUsed   atomic.Int64
	pool        *util.ByteBlockPool
	deleteTerms map[string]*bytesRefIntMap
	termsSize   int
}

func newDeletedTerms() *deletedTerms {
	return &deletedTerms{
		pool:        util.NewByteBlockPool(util.NewDirectTrackingAllocator(&atomic.Int64{})), // Use a dummy counter or the one from BufferedUpdates
		deleteTerms: make(map[string]*bytesRefIntMap),
	}
}

func (dt *deletedTerms) get(term Term) int {
	hash, ok := dt.deleteTerms[term.Field]
	if !ok {
		return -1
	}
	return hash.get(term.Bytes.ValidBytes())
}

func (dt *deletedTerms) put(term Term, value int) {
	hash, ok := dt.deleteTerms[term.Field]
	if !ok {
		hash = newBytesRefIntMap(dt.pool, &dt.bytesUsed)
		dt.deleteTerms[term.Field] = hash
	}
	if hash.put(term.Bytes.ValidBytes(), value) {
		dt.termsSize++
	}
}

func (dt *deletedTerms) clear() {
	if dt.pool != nil {
		dt.pool.Reset(false, false)
	}
	dt.bytesUsed.Store(0)
	dt.deleteTerms = make(map[string]*bytesRefIntMap)
	dt.termsSize = 0
}

func (dt *deletedTerms) size() int {
	return dt.termsSize
}

func (dt *deletedTerms) isEmpty() bool {
	return dt.termsSize == 0
}

func (dt *deletedTerms) ramBytesUsed() int64 {
	return dt.bytesUsed.Load()
}

// deletedTermEntry is one buffered delete term together with the exclusive
// upper-bound doc id it deletes up to. It is the Go rendering of the
// (Term term, int docId) pair Lucene's DeletedTerms.DeletedTermConsumer
// accepts.
type deletedTermEntry struct {
	// Field is the term's field name.
	Field string
	// Bytes is the term's encoded bytes.
	Bytes []byte
	// Value is the newest doc id buffered for this term; documents with a
	// lower doc id are deleted.
	Value int
}

// ForEachOrdered returns every buffered delete term in sorted order: by field
// name, then by term bytes within each field.
//
// Mirrors DeletedTerms.forEachOrdered, which feeds an ordered stream to a
// consumer; Go materialises the same ordered projection so callers can range
// over it. Like the Java original this is a destructive operation: it calls
// BytesRefHash.Sort() on every per-field hash.
func (dt *deletedTerms) ForEachOrdered() []deletedTermEntry {
	if dt.termsSize == 0 {
		return nil
	}
	fields := make([]string, 0, len(dt.deleteTerms))
	for field := range dt.deleteTerms {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	out := make([]deletedTermEntry, 0, dt.termsSize)
	for _, field := range fields {
		terms := dt.deleteTerms[field]
		indices := terms.bytesRefHash.Sort()
		for i := 0; i < terms.bytesRefHash.Size(); i++ {
			index := indices[i]
			var scratch util.BytesRef
			terms.bytesRefHash.Get(index, &scratch)
			// Copy: the scratch view points into the shared byte pool, which
			// the next Get call overwrites.
			bytes := make([]byte, len(scratch.ValidBytes()))
			copy(bytes, scratch.ValidBytes())
			out = append(out, deletedTermEntry{
				Field: field,
				Bytes: bytes,
				Value: terms.values[index],
			})
		}
	}
	return out
}

type bytesRefIntMap struct {
	pool         *util.ByteBlockPool
	bytesRefHash *util.BytesRefHash
	values       []int
	counter      *atomic.Int64
}

func newBytesRefIntMap(pool *util.ByteBlockPool, counter *atomic.Int64) *bytesRefIntMap {
	hash := util.NewBytesRefHashWithPool(pool)
	return &bytesRefIntMap{
		pool:         pool,
		bytesRefHash: hash,
		values:       make([]int, 0, util.DefaultCapacity),
		counter:      counter,
	}
}

func (m *bytesRefIntMap) put(key []byte, value int) bool {
	ref := &util.BytesRef{Bytes: key}
	e, err := m.bytesRefHash.Add(ref)
	if err != nil {
		panic(err)
	}
	if e < 0 {
		idx := -e - 1
		m.values[idx] = value
		return false
	}
	m.values = append(m.values, value)
	return true
}

func (m *bytesRefIntMap) get(key []byte) int {
	ref := &util.BytesRef{Bytes: key}
	e := m.bytesRefHash.Find(ref)
	if e == -1 {
		return -1
	}
	return m.values[e]
}

// FieldUpdatesBuffer buffers numeric and binary field updates.
// IsBufferedUpdates satisfies the spi.BufferedUpdatesRef marker interface,
// letting a *BufferedUpdates flow through SegmentWriteState.SegUpdates
// without spi/ importing index/ (see spi.BufferedUpdatesRef).
func (b *BufferedUpdates) IsBufferedUpdates() {}
