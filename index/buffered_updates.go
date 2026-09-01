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
		buffer = NewFieldUpdatesBuffer(b, update, docIDUpTo)
		b.fieldUpdates[update.Field()] = buffer
	}
	if update.HasValue() {
		buffer.addUpdate(update.Term(), update.value, docIDUpTo)
	} else {
		buffer.addNoValue(update.Term(), docIDUpTo)
	}
	b.numFieldUpdates.Add(1)
}

func (b *BufferedUpdates) AddBinaryUpdate(update *BinaryDocValuesUpdate, docIDUpTo int) {
	buffer, ok := b.fieldUpdates[update.Field()]
	if !ok {
		buffer = NewFieldUpdatesBuffer(b, update, docIDUpTo)
		b.fieldUpdates[update.Field()] = buffer
	}
	if update.HasValue() {
		buffer.addUpdate(update.Term(), update.value, docIDUpTo)
	} else {
		buffer.addNoValue(update.Term(), docIDUpTo)
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
type FieldUpdatesBuffer struct {
	bytesUsed    *atomic.Int64
	numUpdates   int
	termValues   *util.BytesRefArray
	termSortState *util.BytesRefArraySortState
	byteValues   *util.BytesRefArray
	docsUpTo     []int
	numericValues []longs // custom type or slice
	hasValues    *util.FixedBitSet
	maxNumeric   int64
	minNumeric   int64
	fields       []string
	isNumeric    bool
	finished     bool
	parent       *BufferedUpdates
}

type longs []int64

func NewFieldUpdatesBuffer(parent *BufferedUpdates, initialValue DocValuesUpdate, docUpTo int) *FieldUpdatesBuffer {
	isNumeric := initialValue.Type() == DocValuesTypeNumeric

	buf := &FieldUpdatesBuffer{
		parent:    parent,
		bytesUsed: &parent.fieldUpdatesBytesUsed,
		numUpdates: 1,
		termValues: util.NewBytesRefArray(0),
		fields:     []string{initialValue.Field()},
		docsUpTo:    []int{docUpTo},
		isNumeric:   isNumeric,
	}

	// Initial value
	if term, ok := initialValue.(*NumericDocValuesUpdate); ok {
		buf.termValues.AppendBytes(term.Term().Bytes())
		if term.HasValue() {
			buf.numericValues = []int64{term.value}
			buf.maxNumeric, buf.minNumeric = term.value, term.value
		} else {
			buf.numericValues = []int64{0}
		}
		if !term.HasValue() {
			buf.hasValues, _ = util.NewFixedBitSet(1)
		}
	} else if term, ok := initialValue.(*BinaryDocValuesUpdate); ok {
		buf.termValues.AppendBytes(term.Term().Bytes())
		buf.byteValues = util.NewBytesRefArray(0)
		if term.HasValue() {
			buf.byteValues.AppendBytes(term.value)
		}
	}

	return buf
}

func (b *FieldUpdatesBuffer) addUpdate(term Term, value interface{}, docUpTo int) {
	ord := b.append(term)
	b.add(term.Field(), docUpTo, ord, true)
	if b.isNumeric {
		val := value.(int64)
		if b.numericValues == nil {
			b.numericValues = make([]int64, 0)
		}
		b.numericValues = append(b.numericValues, val)
		if val > b.maxNumeric { b.maxNumeric = val }
		if val < b.minNumeric { b.minNumeric = val }
	} else {
		val := value.([]byte)
		b.byteValues.AppendBytes(val)
	}
}

func (b *FieldUpdatesBuffer) addNoValue(term Term, docUpTo int) {
	ord := b.append(term)
	b.add(term.Field(), docUpTo, ord, false)
}

func (b *FieldUpdatesBuffer) append(term Term) int {
	b.termValues.AppendBytes(term.Bytes())
	return b.numUpdates
}

func (b *FieldUpdatesBuffer) add(field string, docUpTo, ord int, hasValue bool) {
	// simplified: assume fields[0] is the only field for now as per Lucene's common case
	if b.fields[0] != field {
		// handle multiple fields if necessary
	}

	if len(b.docsUpTo) <= ord {
		b.docsUpTo = append(b.docsUpTo, docUpTo)
	} else {
		b.docsUpTo[ord] = docUpTo
	}

	if !hasValue || b.hasValues != nil {
		if b.hasValues == nil {
			b.hasValues, _ = util.NewFixedBitSet(ord + 1)
		}
		if hasValue {
			b.hasValues.Set(ord)
		}
	}
}

func (b *FieldUpdatesBuffer) Finish() {
	b.finished = true
	if b.hasSingleValue() && b.hasValues == nil && len(b.fields) == 1 {
		b.termSortState = b.termValues.SortByBytes()
	}
}

func (b *FieldUpdatesBuffer) hasSingleValue() bool {
	return b.isNumeric && len(b.numericValues) == 1
}

func (b *FieldUpdatesBuffer) Iterator() *BufferedUpdateIterator {
	return &BufferedUpdateIterator{
		buffer: b,
	}
}

type BufferedUpdate struct {
	DocUpTo      int
	NumericValue int64
	BinaryValue  []byte
	HasValue     bool
	TermField    string
	TermValue    []byte
}

type BufferedUpdateIterator struct {
	buffer *FieldUpdatesBuffer
	index  int
}

func (it *BufferedUpdateIterator) Next() (*BufferedUpdate, bool) {
	if it.index >= it.buffer.numUpdates {
		return nil, false
	}

	idx := it.index
	it.index++

	update := &BufferedUpdate{
		TermValue: it.buffer.termValues.GetBytes(idx),
		TermField: it.buffer.fields[0],
		DocUpTo:   it.buffer.docsUpTo[idx],
	}

	// check hasValue
	hasVal := true
	if it.buffer.hasValues != nil {
		hasVal = it.buffer.hasValues.Get(idx)
	}
	update.HasValue = hasVal

	if hasVal {
		if it.buffer.isNumeric {
			update.NumericValue = it.buffer.numericValues[idx]
		} else {
			update.BinaryValue = it.buffer.byteValues.GetBytes(idx)
		}
	}

	return update, true
}
