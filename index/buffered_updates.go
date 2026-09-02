//go:build ignore

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
	bytesUsed atomic.Int64

	// fieldUpdatesBytesUsed tracks the total bytes used for field updates
	fieldUpdatesBytesUsed atomic.Int64
}

type queryDelete struct {
	query    Query
	docUpTo int
}

const bytesPerDelQuery = 64 // Rough estimate mirroring Lucene's BYTES_PER_DEL_QUERY

func NewBufferedUpdates(segmentName string) *BufferedUpdates {
	return &BufferedUpdates{
		segmentName:   segmentName,
		deleteTerms:    newDeletedTerms(),
		deleteQueries:  make([]queryDelete, 0),
		fieldUpdates:    make(map[string]*FieldUpdatesBuffer),
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
	if b.bytesUsed.Load() != 0 {
		s += fmt.Sprintf(" bytesUsed=%d", b.bytesUsed.Load())
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
	b.bytesUsed.Add(bytesPerDelQuery)
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
		buffer, err = NewFieldUpdatesBufferBinary(b.bytesUsed, update.Term(), docIDUpTo, update.value, update.HasValue())
		if err != nil {
			panic(err)
		}
		b.fieldUpdates[update.Field()] = buffer
	}
	if update.HasValue() {
		buffer.AddBinaryUpdate(update.Term(), update.value, docIDUpTo)
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
	b.bytesUsed.Store(0)
	b.fieldUpdatesBytesUsed.Store(0)
}

func (b *BufferedUpdates) Any() bool {
	return !b.deleteTerms.isEmpty() || len(b.deleteQueries) > 0 || b.numFieldUpdates.Load() > 0
}

func (b *BufferedUpdates) RamBytesUsed() int64 {
	return b.bytesUsed.Load() + b.fieldUpdatesBytesUsed.Load() + b.deleteTerms.ramBytesUsed()
}

type deletedTerms struct {
	bytesUsed atomic.Int64
	pool      *util.ByteBlockPool
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
	hash, ok := dt.deleteTerms[term.Field()]
	if !ok {
		return -1
	}
	return hash.get(term.Bytes())
}

func (dt *deletedTerms) put(term Term, value int) {
	hash, ok := dt.deleteTerms[term.Field()]
	if !ok {
		hash = newBytesRefIntMap(dt.pool, &dt.bytesUsed)
		dt.deleteTerms[term.Field()] = hash
	}
	if hash.put(term.Bytes(), value) {
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

type bytesRefIntMap struct {
	pool       *util.ByteBlockPool
	bytesRefHash *util.BytesRefHash
	values     []int
	counter    *atomic.Int64
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
	e := m.bytesRefHash.Add(ref)
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