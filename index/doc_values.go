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

func (e *emptyBinaryDV) Get(int) ([]byte, error)  { return nil, nil }
func (e *emptyBinaryDV) Advance(int) (int, error) { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) NextDoc() (int, error)    { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) DocID() int               { return e.docID }

type emptyNumericDV struct{ docID int }

func (e *emptyNumericDV) Get(int) (int64, error)   { return 0, nil }
func (e *emptyNumericDV) Advance(int) (int, error) { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) NextDoc() (int, error)    { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) DocID() int               { return e.docID }

type emptySortedDV struct{ docID int }

func (e *emptySortedDV) Get(int) ([]byte, error)       { return nil, nil }
func (e *emptySortedDV) Advance(int) (int, error)      { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) NextDoc() (int, error)         { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) DocID() int                    { return e.docID }
func (e *emptySortedDV) GetOrd(int) (int, error)       { return -1, nil }
func (e *emptySortedDV) LookupOrd(int) ([]byte, error) { return nil, nil }
func (e *emptySortedDV) GetValueCount() int            { return 0 }

type emptySortedNumericDV struct{ docID int }

func (e *emptySortedNumericDV) Get(int) ([]int64, error) { return nil, nil }
func (e *emptySortedNumericDV) Advance(int) (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedNumericDV) NextDoc() (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedNumericDV) DocID() int { return e.docID }

type emptySortedSetDV struct{ docID int }

func (e *emptySortedSetDV) Get(int) ([]int, error) { return nil, nil }
func (e *emptySortedSetDV) Advance(int) (int, error) {
	e.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}
func (e *emptySortedSetDV) NextDoc() (int, error)         { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedSetDV) DocID() int                    { return e.docID }
func (e *emptySortedSetDV) LookupOrd(int) ([]byte, error) { return nil, nil }
func (e *emptySortedSetDV) GetValueCount() int            { return 0 }

// --- singleton wrappers ------------------------------------------------------
//
// singletonSortedNumeric lives in singleton_sorted_numeric_doc_values.go;
// see that file for the dedicated port of
// org.apache.lucene.index.SingletonSortedNumericDocValues.

type singletonSortedSet struct {
	wrapped SortedDocValues
}

func (s *singletonSortedSet) Get(docID int) ([]int, error) {
	ord, err := s.wrapped.GetOrd(docID)
	if err != nil {
		return nil, err
	}
	if ord < 0 {
		return nil, nil
	}
	return []int{ord}, nil
}

func (s *singletonSortedSet) Advance(target int) (int, error) {
	return s.wrapped.Advance(target)
}

func (s *singletonSortedSet) NextDoc() (int, error) {
	return s.wrapped.NextDoc()
}

func (s *singletonSortedSet) DocID() int {
	return s.wrapped.DocID()
}

func (s *singletonSortedSet) LookupOrd(ord int) ([]byte, error) {
	return s.wrapped.LookupOrd(ord)
}

func (s *singletonSortedSet) GetValueCount() int {
	return s.wrapped.GetValueCount()
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
