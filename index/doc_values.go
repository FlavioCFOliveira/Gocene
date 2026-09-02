// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file ports the static helper methods of
// org.apache.lucene.index.DocValues from Apache Lucene 10.4.0:
//
//   - EmptyBinary / EmptyNumeric / EmptySorted / EmptySortedNumeric /
//     EmptySortedSet — empty iterators (always at NO_MORE_DOCS).
//   - Singleton — wraps a NumericDocValues / SortedDocValues so it can be
//     used in a multi-valued context.
//   - UnwrapSingleton — returns the wrapped value if the iterator was
//     produced by Singleton, otherwise nil.
//
// The doc-values interfaces themselves live in doc_values_interfaces.go.

// EmptyBinary returns an empty BinaryDocValues iterator.
func EmptyBinary() BinaryDocValues {
	return &emptyBinaryDV{}
}

// EmptyNumeric returns an empty NumericDocValues iterator.
func EmptyNumeric() NumericDocValues {
	return &emptyNumericDV{}
}

// EmptySorted returns an empty SortedDocValues iterator.
func EmptySorted() SortedDocValues {
	return &emptySortedDV{}
}

// EmptySortedNumeric returns an empty SortedNumericDocValues iterator.
func EmptySortedNumeric() SortedNumericDocValues {
	return &emptySortedNumericDV{}
}

// EmptySortedSet returns an empty SortedSetDocValues iterator.
func EmptySortedSet() SortedSetDocValues {
	return &emptySortedSetDV{}
}

// Singleton wraps a NumericDocValues as a SortedNumericDocValues that yields
// exactly one value per document. Mirrors DocValues.singleton(NumericDocValues).
func Singleton(dv NumericDocValues) SortedNumericDocValues {
	if dv == nil {
		return EmptySortedNumeric()
	}
	return &singletonSortedNumeric{wrapped: dv}
}

// SingletonSortedSet wraps a SortedDocValues as a SortedSetDocValues that
// yields exactly one ordinal per document. Mirrors
// DocValues.singleton(SortedDocValues).
func SingletonSortedSet(dv SortedDocValues) SortedSetDocValues {
	if dv == nil {
		return EmptySortedSet()
	}
	return &singletonSortedSet{wrapped: dv}
}

// UnwrapSingletonSortedNumeric returns the underlying NumericDocValues if
// dv was produced by Singleton(NumericDocValues), otherwise nil.
func UnwrapSingletonSortedNumeric(dv SortedNumericDocValues) NumericDocValues {
	if s, ok := dv.(*singletonSortedNumeric); ok {
		return s.wrapped
	}
	return nil
}

// UnwrapSingletonSortedSet returns the underlying SortedDocValues if dv was
// produced by SingletonSortedSet(SortedDocValues), otherwise nil.
func UnwrapSingletonSortedSet(dv SortedSetDocValues) SortedDocValues {
	if s, ok := dv.(*singletonSortedSet); ok {
		return s.wrapped
	}
	return nil
}

// --- empty iterators ---------------------------------------------------------

type emptyBinaryDV struct{ docID int }

func (e *emptyBinaryDV) Advance(int) (int, error)       { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) AdvanceExact(int) (bool, error) { e.docID = NO_MORE_DOCS; return false, nil }
func (e *emptyBinaryDV) BinaryValue() ([]byte, error)   { return nil, nil }
func (e *emptyBinaryDV) NextDoc() (int, error)          { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) DocID() int                     { return e.docID }
func (e *emptyBinaryDV) Cost() int64                    { return 0 }

type emptyNumericDV struct{ docID int }

func (e *emptyNumericDV) Advance(int) (int, error)       { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) AdvanceExact(int) (bool, error) { e.docID = NO_MORE_DOCS; return false, nil }
func (e *emptyNumericDV) LongValue() (int64, error)      { return 0, nil }
func (e *emptyNumericDV) NextDoc() (int, error)          { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) DocID() int                     { return e.docID }
func (e *emptyNumericDV) Cost() int64                    { return 0 }

type emptySortedDV struct{ docID int }

func (e *emptySortedDV) Advance(int) (int, error)       { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) AdvanceExact(int) (bool, error) { e.docID = NO_MORE_DOCS; return false, nil }
func (e *emptySortedDV) NextDoc() (int, error)          { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) DocID() int                     { return e.docID }
func (e *emptySortedDV) OrdValue() (int, error)         { return -1, nil }
func (e *emptySortedDV) LongValue() (int64, error)      { return -1, nil }
func (e *emptySortedDV) LookupOrd(int) ([]byte, error)  { return nil, nil }
func (e *emptySortedDV) GetValueCount() int             { return 0 }
func (e *emptySortedDV) Cost() int64                    { return 0 }

type emptySortedNumericDV struct{ docID int }

func (e *emptySortedNumericDV) Advance(int) (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedNumericDV) AdvanceExact(int) (bool, error) {
	e.docID = NO_MORE_DOCS
	return false, nil
}
func (e *emptySortedNumericDV) NextValue() (int64, error)   { return 0, nil }
func (e *emptySortedNumericDV) DocValueCount() (int, error) { return 0, nil }
func (e *emptySortedNumericDV) LongValue() (int64, error)   { return 0, nil }
func (e *emptySortedNumericDV) NextDoc() (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedNumericDV) DocID() int   { return e.docID }
func (e *emptySortedNumericDV) Cost() int64  { return 0 }

type emptySortedSetDV struct{ docID int }

func (e *emptySortedSetDV) Advance(int) (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedSetDV) AdvanceExact(int) (bool, error) {
	e.docID = NO_MORE_DOCS
	return false, nil
}
func (e *emptySortedSetDV) NextOrd() (int, error)         { return -1, nil }
func (e *emptySortedSetDV) NextDoc() (int, error)         { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedSetDV) DocID() int                    { return e.docID }
func (e *emptySortedSetDV) LookupOrd(int) ([]byte, error) { return nil, nil }
func (e *emptySortedSetDV) GetValueCount() int            { return 0 }
func (e *emptySortedSetDV) Cost() int64                   { return 0 }

// --- singleton wrappers ------------------------------------------------------
//
// singletonSortedNumeric lives in singleton_sorted_numeric_doc_values.go;
// singletonSortedSet lives in singleton_sorted_set_doc_values.go.
// See those files for the dedicated ports of
// org.apache.lucene.index.SingletonSortedNumericDocValues and
// org.apache.lucene.index.SingletonSortedSetDocValues respectively.

// NumericValueAt positions values on docID via AdvanceExact and returns
// the numeric value, or (0, false, nil) when the document has no value.
//
// Helper introduced by rmp #4710 (Sprint 118 phase 2f) as a small,
// allocation-free replacement for the legacy random-access
// NumericDocValues.Get(docID) accessor. Callers MUST advance
// monotonically (docID >= previous docID) per AdvanceExact's contract.
func NumericValueAt(values NumericDocValues, docID int) (int64, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	ok, err := values.AdvanceExact(docID)
	if err != nil || !ok {
		return 0, false, err
	}
	v, err := values.LongValue()
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

// BinaryValueAt positions values on docID via AdvanceExact and returns
// the binary value, or (nil, nil) when the document has no value.
//
// Helper introduced by rmp #4710 (Sprint 118 phase 2f) as the
// iterator-shaped replacement for the legacy
// BinaryDocValues.Get(docID) accessor. Callers MUST advance
// monotonically.
func BinaryValueAt(values BinaryDocValues, docID int) ([]byte, error) {
	if values == nil {
		return nil, nil
	}
	ok, err := values.AdvanceExact(docID)
	if err != nil || !ok {
		return nil, err
	}
	return values.BinaryValue()
}

// SortedOrdAt positions values on docID via AdvanceExact and returns
// the ordinal, or -1 when the document has no value. Helper introduced
// by rmp #4710 as the iterator-shaped replacement for the legacy
// SortedDocValues.GetOrd(docID) accessor.
func SortedOrdAt(values SortedDocValues, docID int) (int, error) {
	if values == nil {
		return -1, nil
	}
	ok, err := values.AdvanceExact(docID)
	if err != nil {
		return -1, err
	}
	if !ok {
		return -1, nil
	}
	return values.OrdValue()
}

// CollectSortedSetOrds materialises every ordinal bound to the currently
// positioned document of values into a fresh slice (empty when the
// document has no values). Callers MUST have already positioned the
// iterator via NextDoc / Advance / AdvanceExact before invoking this
// helper.
//
// Helper introduced by rmp #4710 alongside CollectSortedNumericValues
// for sites where the iterator is already positioned and a second
// AdvanceExact would be wasteful or unsupported (writer-side buffered
// views forbid AdvanceExact, matching the Java reference).
func CollectSortedSetOrds(values SortedSetDocValues) ([]int, error) {
	if values == nil {
		return nil, nil
	}
	var out []int
	for {
		ord, err := values.NextOrd()
		if err != nil {
			return nil, err
		}
		if ord == -1 {
			break
		}
		out = append(out, ord)
	}
	return out, nil
}

// CollectSortedNumericValues materialises every value bound to the
// currently positioned document of values into a fresh slice (empty
// when the document has no values). Callers MUST have already
// positioned the iterator via NextDoc / Advance / AdvanceExact before
// invoking this helper.
//
// Helper introduced by rmp #4710 to replace inline DocValueCount +
// NextValue drains at call sites where the iterator is already
// positioned and a second AdvanceExact would be wasteful.
func CollectSortedNumericValues(values SortedNumericDocValues) ([]int64, error) {
	if values == nil {
		return nil, nil
	}
	count, err := values.DocValueCount()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	out := make([]int64, count)
	for i := 0; i < count; i++ {
		v, err := values.NextValue()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// DrainSortedNumeric positions values on docID via AdvanceExact and
// materialises every value bound to that document into a fresh slice
// (empty when the doc has no values). Helper introduced by rmp #4710 as
// the iterator-shaped replacement for the legacy
// SortedNumericDocValues.Get(docID) accessor.
//
// The returned slice is owned by the caller; the iterator is left
// positioned on docID with its value-cursor exhausted.
func DrainSortedNumeric(values SortedNumericDocValues, docID int) ([]int64, error) {
	if values == nil {
		return nil, nil
	}
	ok, err := values.AdvanceExact(docID)
	if err != nil || !ok {
		return nil, err
	}
	count, err := values.DocValueCount()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	out := make([]int64, count)
	for i := 0; i < count; i++ {
		v, err := values.NextValue()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// DrainSortedSet positions values on docID via AdvanceExact and
// materialises every ordinal bound to that document into a fresh slice
// (empty when the doc has no values). Helper introduced by rmp #4710
// as the iterator-shaped replacement for the legacy
// SortedSetDocValues.Get(docID) accessor.
func DrainSortedSet(values SortedSetDocValues, docID int) ([]int, error) {
	if values == nil {
		return nil, nil
	}
	ok, err := values.AdvanceExact(docID)
	if err != nil || !ok {
		return nil, err
	}
	var out []int
	for {
		ord, err := values.NextOrd()
		if err != nil {
			return nil, err
		}
		if ord == -1 {
			break
		}
		out = append(out, ord)
	}
	return out, nil
}

// IsDocValuesCacheable mirrors the static helper
// org.apache.lucene.index.DocValues#isCacheable(LeafReaderContext, String...).
//
// A query that consumes doc values for the supplied fields is cacheable on a
// given leaf only when none of those fields have an associated doc-values
// update generation. A non-negative DocValuesGen means the field's doc
// values have been overwritten by an in-place update, so the segment-level
// values can change underfoot and caching the matching doc set would be
// stale.
//
// The reader exposed by LeafReaderContext is the generic
// IndexReaderInterface, which does not declare GetFieldInfos directly. The
// helper unwraps the concrete reader through a narrow type assertion that
// every production leaf (LeafReader, SegmentReader, FilterLeafReader,
// CodecReader, ...) already satisfies; readers without a FieldInfos surface
// default to cacheable, matching the Java reference's behaviour when
// fieldInfo lookup returns null.
func IsDocValuesCacheable(ctx *LeafReaderContext, fields ...string) bool {
	if ctx == nil {
		return true
	}
	type fieldInfosReader interface {
		GetFieldInfos() *FieldInfos
	}
	reader, ok := ctx.LeafReader().(fieldInfosReader)
	if !ok || reader == nil {
		return true
	}
	infos := reader.GetFieldInfos()
	if infos == nil {
		return true
	}
	for _, name := range fields {
		fi := infos.GetByName(name)
		if fi != nil && fi.DocValuesGen() > -1 {
			return false
		}
	}
	return true
}
