// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port note:
//
// This file is the Go port of
// lucene/core/src/java/org/apache/lucene/index/FieldUpdatesBuffer.java
// (Apache Lucene 10.4.0).
//
// Divergences from the Java source:
//
//   - The Java constructors accept a {@code DocValuesUpdate.NumericDocValuesUpdate}
//     or {@code DocValuesUpdate.BinaryDocValuesUpdate}. Gocene's pre-existing
//     [NumericDocValuesUpdate] / [BinaryDocValuesUpdate] do not yet model the
//     "no value" (reset) case Lucene encodes via {@code hasValue()}. To stay
//     faithful to the Lucene buffer semantics without forcing a refactor of
//     the existing update types, the constructors here accept primitives:
//     a term, the initial doc-up-to, a value, and a {@code hasValue} flag.
//     The buffer itself never sees the wrapper types.
//
//   - Lucene relies on {@code BytesRefArray.IndexedBytesRefIterator}, which
//     exposes a sorted-iteration cursor alongside the original insertion
//     ordinal. Gocene's [util.BytesRefArraySortState] already exposes
//     {@code Ord()}; for unsorted (insertion-order) iteration we wrap a
//     [util.BytesRefArrayIterator] and track the ordinal locally.
//
//   - Lucene uses {@code FixedBitSet.ensureCapacity} to grow the
//     {@code hasValues} bitset in place. Gocene's [util.FixedBitSet] does not
//     expose an equivalent helper, so we allocate a fresh, larger bitset and
//     copy the set bits explicitly via {@code NextSetBit}.
//
//   - Memory accounting uses [util.Counter] (atomic int64) and approximates
//     Lucene's RamUsageEstimator constants. Exact byte parity with Java is
//     not a goal: the counter is informational, used by callers to decide
//     when to flush, and the relative growth pattern is preserved.

// selfShallowSize approximates the shallow size of a FieldUpdatesBuffer.
// Lucene uses RamUsageEstimator.shallowSizeOfInstance(FieldUpdatesBuffer.class);
// the Gocene constant is a deliberate approximation, see the port note above.
const selfShallowSize int64 = 96

// stringShallowSize approximates the shallow size of a String header.
// Lucene uses RamUsageEstimator.shallowSizeOfInstance(String.class); in Go
// a string is a (ptr, len) tuple, 16 bytes on 64-bit platforms.
const stringShallowSize int64 = 16

// sizeOfString approximates the RAM cost of a string with the given content.
// Mirrors Lucene's sizeOfString which adds Character.BYTES (2) per char; Go
// strings are byte-counted internally but we keep the *2 multiplier so the
// growth pattern matches Lucene's expectations.
func sizeOfString(s string) int64 {
	return stringShallowSize + int64(len(s))*2
}

// FieldUpdatesBuffer efficiently buffers numeric and binary field updates
// and stores terms, values and metadata in a memory efficient way without
// creating large amounts of objects. Update terms are stored without
// de-duplicating the update term. In general we try to optimize for several
// use-cases. For instance we try to use constant space for the update terms
// field since the common case always updates on the same field. Also for
// docUpTo we try to optimize for the case when updates should be applied to
// all docs ie. docUpTo=math.MaxInt32. In other cases each update will likely
// have a different docUpTo. Along the same lines this implementation
// optimizes the case when all updates have a value. Lastly, if all updates
// share the same value for a numeric field we only store the value once.
//
// This is the Go port of Lucene's
// {@code org.apache.lucene.index.FieldUpdatesBuffer}.
//
// Not safe for concurrent use.
type FieldUpdatesBuffer struct {
	bytesUsed *util.Counter

	numUpdates int

	// termValues stores the update term bytes without de-duplication. The
	// stable sort applied in Finish preserves insertion order for equal
	// terms so they are applied in the order requested.
	termValues    *util.BytesRefArray
	termSortState *util.BytesRefArraySortState

	// byteValues holds binary values; nil for numeric buffers.
	byteValues *util.BytesRefArray

	docsUpTo []int

	// numericValues holds numeric values; nil for binary buffers.
	numericValues []int64

	// hasValues is lazily allocated: nil when every update so far has a
	// value (the common case), otherwise tracks the per-update flag.
	hasValues *util.FixedBitSet

	maxNumeric int64
	minNumeric int64

	// fields holds the per-update field name. Length is 1 while every
	// update targets the same field; grown to numUpdates once a divergent
	// field appears.
	fields []string

	isNumeric bool
	finished  bool
}

// newFieldUpdatesBuffer is the shared body of the public constructors.
// initialTerm and initialField cannot be nil/empty respectively; the
// caller guarantees validity.
func newFieldUpdatesBuffer(
	bytesUsed *util.Counter,
	initialTerm *Term,
	docUpTo int,
	hasValue bool,
	isNumeric bool,
) (*FieldUpdatesBuffer, error) {
	if bytesUsed == nil {
		return nil, errors.New("FieldUpdatesBuffer: bytesUsed must not be nil")
	}
	if initialTerm == nil {
		return nil, errors.New("FieldUpdatesBuffer: initialTerm must not be nil")
	}

	bytesUsed.AddAndGet(selfShallowSize)

	termValues := util.NewBytesRefArray(0)
	termValues.Append(initialTerm.Bytes)

	buf := &FieldUpdatesBuffer{
		bytesUsed:  bytesUsed,
		numUpdates: 1,
		termValues: termValues,
		fields:     []string{initialTerm.Field},
		docsUpTo:   []int{docUpTo},
		isNumeric:  isNumeric,
		maxNumeric: math.MinInt64,
		minNumeric: math.MaxInt64,
	}
	bytesUsed.AddAndGet(sizeOfString(initialTerm.Field))

	if !hasValue {
		bs, err := util.NewFixedBitSet(1)
		if err != nil {
			return nil, fmt.Errorf("FieldUpdatesBuffer: hasValues bitset: %w", err)
		}
		buf.hasValues = bs
		bytesUsed.AddAndGet(bs.RamBytesUsed())
	}

	if !isNumeric {
		buf.byteValues = util.NewBytesRefArray(0)
	}

	return buf, nil
}

// NewFieldUpdatesBufferNumeric constructs a numeric buffer seeded with a
// single update. If hasValue is false, value is ignored and the update is
// recorded as a reset.
//
// Mirrors {@code FieldUpdatesBuffer(Counter, NumericDocValuesUpdate, int)}.
func NewFieldUpdatesBufferNumeric(
	bytesUsed *util.Counter,
	initialTerm *Term,
	docUpTo int,
	initialValue int64,
	hasValue bool,
) (*FieldUpdatesBuffer, error) {
	buf, err := newFieldUpdatesBuffer(bytesUsed, initialTerm, docUpTo, hasValue, true)
	if err != nil {
		return nil, err
	}
	if hasValue {
		buf.numericValues = []int64{initialValue}
		buf.maxNumeric = initialValue
		buf.minNumeric = initialValue
	} else {
		buf.numericValues = []int64{0}
	}
	bytesUsed.AddAndGet(8) // Long.BYTES
	return buf, nil
}

// NewFieldUpdatesBufferBinary constructs a binary buffer seeded with a
// single update. If hasValue is false, value is ignored and the update is
// recorded as a reset.
//
// Mirrors {@code FieldUpdatesBuffer(Counter, BinaryDocValuesUpdate, int)}.
func NewFieldUpdatesBufferBinary(
	bytesUsed *util.Counter,
	initialTerm *Term,
	docUpTo int,
	initialValue *util.BytesRef,
	hasValue bool,
) (*FieldUpdatesBuffer, error) {
	buf, err := newFieldUpdatesBuffer(bytesUsed, initialTerm, docUpTo, hasValue, false)
	if err != nil {
		return nil, err
	}
	if hasValue {
		buf.byteValues.Append(initialValue)
	}
	return buf, nil
}

// GetMaxNumeric returns the maximum numeric value observed across all
// updates carrying a value, or 0 if no update carries a value.
func (b *FieldUpdatesBuffer) GetMaxNumeric() int64 {
	if !b.isNumeric {
		panic("FieldUpdatesBuffer: GetMaxNumeric called on binary buffer")
	}
	if b.minNumeric == math.MaxInt64 && b.maxNumeric == math.MinInt64 {
		return 0
	}
	return b.maxNumeric
}

// GetMinNumeric returns the minimum numeric value observed across all
// updates carrying a value, or 0 if no update carries a value.
func (b *FieldUpdatesBuffer) GetMinNumeric() int64 {
	if !b.isNumeric {
		panic("FieldUpdatesBuffer: GetMinNumeric called on binary buffer")
	}
	if b.minNumeric == math.MaxInt64 && b.maxNumeric == math.MinInt64 {
		return 0
	}
	return b.minNumeric
}

// add records the per-update field/docUpTo/hasValue tuple at the given ord.
// It mirrors Lucene's package-private {@code add(String, int, int, boolean)}.
func (b *FieldUpdatesBuffer) add(field string, docUpTo, ord int, hasValue bool) error {
	if b.finished {
		return errors.New("FieldUpdatesBuffer: buffer was finished already")
	}

	if b.fields[0] != field || len(b.fields) != 1 {
		if len(b.fields) <= ord {
			oldLen := len(b.fields)
			newLen := util.Oversize(ord+1, util.NumBytesObjectRef)
			grown := make([]string, newLen)
			copy(grown, b.fields)
			if oldLen == 1 {
				// Fill positions 1..ord-1 with the original sole field, so
				// later iteration sees consistent values for the entries
				// inserted before fields became per-ord.
				for i := 1; i < ord; i++ {
					grown[i] = b.fields[0]
				}
			}
			b.bytesUsed.AddAndGet(int64(newLen-oldLen) * util.NumBytesObjectRef)
			b.fields = grown
		}
		if field != b.fields[0] {
			b.bytesUsed.AddAndGet(sizeOfString(field))
		}
		b.fields[ord] = field
	}

	if b.docsUpTo[0] != docUpTo || len(b.docsUpTo) != 1 {
		if len(b.docsUpTo) <= ord {
			oldLen := len(b.docsUpTo)
			newLen := util.Oversize(ord+1, 4) // Integer.BYTES
			grown := make([]int, newLen)
			copy(grown, b.docsUpTo)
			if oldLen == 1 {
				for i := 1; i < ord; i++ {
					grown[i] = b.docsUpTo[0]
				}
			}
			b.bytesUsed.AddAndGet(int64(newLen-oldLen) * 4)
			b.docsUpTo = grown
		}
		b.docsUpTo[ord] = docUpTo
	}

	if !hasValue || b.hasValues != nil {
		if b.hasValues == nil {
			bs, err := util.NewFixedBitSet(ord + 1)
			if err != nil {
				return fmt.Errorf("FieldUpdatesBuffer: hasValues bitset: %w", err)
			}
			// All previous updates carried a value: set bits [0, ord).
			setRange(bs, 0, ord)
			b.hasValues = bs
			b.bytesUsed.AddAndGet(bs.RamBytesUsed())
		} else if b.hasValues.Length() <= ord {
			oldBytes := b.hasValues.RamBytesUsed()
			grown, err := ensureBitSetCapacity(b.hasValues, util.Oversize(ord+1, 1))
			if err != nil {
				return err
			}
			b.bytesUsed.AddAndGet(grown.RamBytesUsed() - oldBytes)
			b.hasValues = grown
		}
		if hasValue {
			b.hasValues.Set(ord)
		}
	}

	return nil
}

// AddUpdate buffers a numeric update for the given term and value.
//
// Mirrors {@code addUpdate(Term, long, int)}. Returns an error if the
// buffer has already been finished.
func (b *FieldUpdatesBuffer) AddUpdate(term *Term, value int64, docUpTo int) error {
	if !b.isNumeric {
		return errors.New("FieldUpdatesBuffer: AddUpdate(numeric) called on binary buffer")
	}
	ord := b.append(term)
	if err := b.add(term.Field, docUpTo, ord, true); err != nil {
		return err
	}

	if value < b.minNumeric {
		b.minNumeric = value
	}
	if value > b.maxNumeric {
		b.maxNumeric = value
	}

	if b.numericValues[0] != value || len(b.numericValues) != 1 {
		if len(b.numericValues) <= ord {
			oldLen := len(b.numericValues)
			newLen := util.Oversize(ord+1, 8) // Long.BYTES
			grown := make([]int64, newLen)
			copy(grown, b.numericValues)
			if oldLen == 1 {
				for i := 1; i < ord; i++ {
					grown[i] = b.numericValues[0]
				}
			}
			b.bytesUsed.AddAndGet(int64(newLen-oldLen) * 8)
			b.numericValues = grown
		}
		b.numericValues[ord] = value
	}
	return nil
}

// AddNoValue buffers a reset (a "no value" update) for the given term.
//
// Mirrors {@code addNoValue(Term, int)}.
func (b *FieldUpdatesBuffer) AddNoValue(term *Term, docUpTo int) error {
	ord := b.append(term)
	return b.add(term.Field, docUpTo, ord, false)
}

// AddBinaryUpdate buffers a binary update for the given term and value.
//
// Mirrors {@code addUpdate(Term, BytesRef, int)}.
func (b *FieldUpdatesBuffer) AddBinaryUpdate(term *Term, value *util.BytesRef, docUpTo int) error {
	if b.isNumeric {
		return errors.New("FieldUpdatesBuffer: AddBinaryUpdate called on numeric buffer")
	}
	ord := b.append(term)
	b.byteValues.Append(value)
	return b.add(term.Field, docUpTo, ord, true)
}

// append records a term in the term store and returns the new ordinal.
// Note: numUpdates pre-increments to match Lucene's {@code numUpdates++}
// post-increment combined with the initial value of 1 (so the first
// returned ord here is 1, matching Java).
func (b *FieldUpdatesBuffer) append(term *Term) int {
	b.termValues.Append(term.Bytes)
	ord := b.numUpdates
	b.numUpdates++
	return ord
}

// Finish freezes the buffer. After Finish returns, no further updates may
// be added but Iterator may be called. Calling Finish more than once is an
// error, mirroring Lucene's IllegalStateException.
func (b *FieldUpdatesBuffer) Finish() error {
	if b.finished {
		return errors.New("FieldUpdatesBuffer: buffer was finished already")
	}
	b.finished = true

	// If every update sets the same field to the same value, we can apply
	// updates in term-sorted order. This permits the iterator's lookahead
	// dedup optimization.
	if b.HasSingleValue() && b.hasValues == nil && len(b.fields) == 1 {
		b.termSortState = b.termValues.SortStable(func(a, c *util.BytesRef) bool {
			return bytes.Compare(a.ValidBytes(), c.ValidBytes()) < 0
		})
		// Lucene adds termSortState.ramBytesUsed() to the counter; we
		// approximate as 4 bytes per ordinal entry plus a fixed header.
		b.bytesUsed.AddAndGet(int64(b.termSortState.Size()*4) + 16)
	}
	return nil
}

// Iterator returns a new iterator over the buffered updates. The buffer
// must have been finished. Returns an error otherwise.
func (b *FieldUpdatesBuffer) Iterator() (*BufferedUpdateIterator, error) {
	if !b.finished {
		return nil, errors.New("FieldUpdatesBuffer: buffer is not finished yet")
	}
	return newBufferedUpdateIterator(b), nil
}

// IsNumeric reports whether this buffer holds numeric (true) or binary
// (false) updates.
func (b *FieldUpdatesBuffer) IsNumeric() bool {
	return b.isNumeric
}

// HasSingleValue reports whether every numeric update in this buffer shares
// the same value. Always false for binary buffers, matching Lucene.
func (b *FieldUpdatesBuffer) HasSingleValue() bool {
	return b.isNumeric && len(b.numericValues) == 1
}

// GetNumericValue returns the buffered numeric value at the given update
// index, or 0 if the update at idx has no value.
func (b *FieldUpdatesBuffer) GetNumericValue(idx int) int64 {
	if b.hasValues != nil && !b.hasValues.Get(idx) {
		return 0
	}
	return b.numericValues[getArrayIndex(len(b.numericValues), idx)]
}

// BufferedUpdate is a struct-like view returned by the iterator's Next.
// The same instance is reused across calls; callers must consume each
// snapshot fully before advancing.
//
// Mirrors {@code FieldUpdatesBuffer.BufferedUpdate}.
type BufferedUpdate struct {
	// DocUpTo is the max document id this update should be applied to.
	DocUpTo int
	// NumericValue is set for numeric buffers; 0 otherwise.
	NumericValue int64
	// BinaryValue is set for binary buffers; nil otherwise.
	BinaryValue *util.BytesRef
	// HasValue is false when the update is a reset.
	HasValue bool
	// TermField is the field name; never empty.
	TermField string
	// TermValue is the term bytes; never nil.
	TermValue *util.BytesRef
}

// BufferedUpdateIterator walks the buffered updates either in insertion
// order or, when possible, in sorted term order (see
// [FieldUpdatesBuffer.Finish] for the conditions that enable sorted
// iteration).
type BufferedUpdateIterator struct {
	owner *FieldUpdatesBuffer

	// Insertion-order cursor; used when termSortState is nil.
	insertionIter *util.BytesRefArrayIterator
	insertionOrd  int

	// Sorted-order cursors; used when termSortState is not nil. We need a
	// pair because the next() method below performs a one-step lookahead
	// to skip duplicate terms (the Lucene dedup optimization).
	sortedIter       *util.BytesRefArraySortState
	lookAheadIter    *util.BytesRefArraySortState
	lookAheadPrimed  bool
	firstCallPending bool

	// Binary-value cursor; nil for numeric buffers.
	byteValuesIter *util.BytesRefArrayIterator

	// Bits view used to mask updates without a value. Wraps either a real
	// FixedBitSet or a MatchAllBits when every update carries a value.
	updatesWithValue util.Bits

	shared BufferedUpdate
}

func newBufferedUpdateIterator(owner *FieldUpdatesBuffer) *BufferedUpdateIterator {
	it := &BufferedUpdateIterator{owner: owner}

	if owner.termSortState != nil {
		// Two independent positional cursors over the same sorted view of
		// termValues. Re-running SortStable with an identical comparator
		// produces a state with the same indices slice but its own pos
		// cursor; the underlying termValues is unchanged.
		owner.termSortState.Reset()
		it.sortedIter = owner.termSortState
		it.lookAheadIter = owner.termValues.SortStable(func(a, c *util.BytesRef) bool {
			return bytes.Compare(a.ValidBytes(), c.ValidBytes()) < 0
		})
		it.firstCallPending = true
	} else {
		it.insertionIter = owner.termValues.Iterator()
		it.insertionOrd = -1
	}

	if !owner.isNumeric {
		it.byteValuesIter = owner.byteValues.Iterator()
	}

	if owner.hasValues == nil {
		it.updatesWithValue = util.NewMatchAllBits(owner.numUpdates)
	} else {
		it.updatesWithValue = owner.hasValues.AsReadOnlyBits()
	}

	return it
}

// IsSortedTerms reports whether iteration yields terms in sorted order.
// When true, equal terms are de-duplicated on the fly.
func (it *BufferedUpdateIterator) IsSortedTerms() bool {
	return it.sortedIter != nil
}

// Next advances to the next buffered update and returns it, or nil when
// the buffer is exhausted. The returned pointer is reused across calls.
func (it *BufferedUpdateIterator) Next() (*BufferedUpdate, error) {
	term, ord, ok := it.nextTerm()
	if !ok {
		return nil, nil
	}

	it.shared.TermValue = term
	it.shared.HasValue = it.updatesWithValue.Get(ord)
	it.shared.TermField = it.owner.fields[getArrayIndex(len(it.owner.fields), ord)]
	it.shared.DocUpTo = it.owner.docsUpTo[getArrayIndex(len(it.owner.docsUpTo), ord)]

	if it.shared.HasValue {
		if it.owner.isNumeric {
			it.shared.NumericValue = it.owner.numericValues[getArrayIndex(len(it.owner.numericValues), ord)]
			it.shared.BinaryValue = nil
		} else {
			next, hasNext := it.byteValuesIter.Next()
			if !hasNext {
				return nil, errors.New("FieldUpdatesBuffer: binary value iterator exhausted before terms")
			}
			it.shared.BinaryValue = next
			it.shared.NumericValue = 0
		}
	} else {
		it.shared.BinaryValue = nil
		it.shared.NumericValue = 0
	}

	return &it.shared, nil
}

// nextTerm returns the next term, its ordinal, and a bool indicating
// whether a term was produced. Implements the lookahead dedup in the
// sorted-iteration branch.
func (it *BufferedUpdateIterator) nextTerm() (*util.BytesRef, int, bool) {
	if it.sortedIter == nil {
		next, ok := it.insertionIter.Next()
		if !ok {
			return nil, 0, false
		}
		it.insertionOrd++
		return next, it.insertionOrd, true
	}

	// Sorted branch with one-step look-ahead. On the first call we prime
	// the look-ahead cursor so it sits exactly one step ahead of the main
	// cursor. On every subsequent call both advance in lockstep inside
	// the loop, and we skip the main entry whenever its successor is
	// equal but originally had a larger insertion ord (which the stable
	// sort guarantees).
	if it.firstCallPending {
		var primeSpare util.BytesRef
		_ = it.lookAheadIter.Next(&primeSpare)
		it.firstCallPending = false
	}

	var lastTerm *util.BytesRef
	var lastOrd int

	for {
		var aheadSpare util.BytesRef
		aheadOK := it.lookAheadIter.Next(&aheadSpare)
		var aheadTerm *util.BytesRef
		if aheadOK {
			aheadTerm = aheadSpare.Clone()
		}

		var lastSpare util.BytesRef
		lastOK := it.sortedIter.Next(&lastSpare)
		if !lastOK {
			return nil, 0, false
		}
		lastTerm = lastSpare.Clone()
		lastOrd = it.sortedIter.Ord()

		if !aheadOK {
			break
		}
		if it.lookAheadIter.Ord() <= it.sortedIter.Ord() {
			break
		}
		if !bytes.Equal(aheadTerm.ValidBytes(), lastTerm.ValidBytes()) {
			break
		}
		// Equal aheadTerm with larger ord: drop lastTerm and continue.
	}

	return lastTerm, lastOrd, true
}

// getArrayIndex mirrors Lucene's static helper of the same name. It clamps
// idx into [0, arrayLength) when the underlying array has been collapsed
// to length 1 (the all-same-value optimization), and returns idx otherwise.
func getArrayIndex(arrayLength, idx int) int {
	if arrayLength == 1 {
		return 0
	}
	if arrayLength <= idx {
		// Defensive: matches Lucene's assert that arrayLength > idx; we
		// fall back to the last element rather than panicking so the
		// iterator never crashes on malformed buffers.
		return arrayLength - 1
	}
	return idx
}

// setRange sets bits in [from, to) on the given bitset. Provided here so
// the rest of the file doesn't depend on a util-level range helper.
func setRange(bs *util.FixedBitSet, from, to int) {
	for i := from; i < to; i++ {
		bs.Set(i)
	}
}

// ensureBitSetCapacity returns a FixedBitSet at least minSize bits long,
// reusing the contents of the input by walking set bits with NextSetBit.
// This mirrors Lucene's FixedBitSet.ensureCapacity, which grows by
// allocating a fresh underlying word array and copying.
func ensureBitSetCapacity(src *util.FixedBitSet, minSize int) (*util.FixedBitSet, error) {
	if src.Length() >= minSize {
		return src, nil
	}
	dst, err := util.NewFixedBitSet(minSize)
	if err != nil {
		return nil, fmt.Errorf("FieldUpdatesBuffer: grow bitset: %w", err)
	}
	for i := src.NextSetBit(0); i != -1 && i < src.Length(); i = src.NextSetBit(i + 1) {
		dst.Set(i)
	}
	return dst, nil
}
