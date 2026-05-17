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

func (e *emptyBinaryDV) Get(int) ([]byte, error)    { return nil, nil }
func (e *emptyBinaryDV) Advance(int) (int, error)   { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) NextDoc() (int, error)      { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyBinaryDV) DocID() int                 { return e.docID }

type emptyNumericDV struct{ docID int }

func (e *emptyNumericDV) Get(int) (int64, error)     { return 0, nil }
func (e *emptyNumericDV) Advance(int) (int, error)   { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) NextDoc() (int, error)      { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptyNumericDV) DocID() int                 { return e.docID }

type emptySortedDV struct{ docID int }

func (e *emptySortedDV) Get(int) ([]byte, error)    { return nil, nil }
func (e *emptySortedDV) Advance(int) (int, error)   { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) NextDoc() (int, error)      { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedDV) DocID() int                 { return e.docID }
func (e *emptySortedDV) GetOrd(int) (int, error)    { return -1, nil }
func (e *emptySortedDV) LookupOrd(int) ([]byte, error) { return nil, nil }
func (e *emptySortedDV) GetValueCount() int         { return 0 }

type emptySortedNumericDV struct{ docID int }

func (e *emptySortedNumericDV) Get(int) ([]int64, error) { return nil, nil }
func (e *emptySortedNumericDV) Advance(int) (int, error) { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedNumericDV) NextDoc() (int, error)    { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedNumericDV) DocID() int               { return e.docID }

type emptySortedSetDV struct{ docID int }

func (e *emptySortedSetDV) Get(int) ([]int, error)     { return nil, nil }
func (e *emptySortedSetDV) Advance(int) (int, error)   { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedSetDV) NextDoc() (int, error)      { e.docID = NO_MORE_DOCS; return NO_MORE_DOCS, nil }
func (e *emptySortedSetDV) DocID() int                 { return e.docID }
func (e *emptySortedSetDV) LookupOrd(int) ([]byte, error) { return nil, nil }
func (e *emptySortedSetDV) GetValueCount() int         { return 0 }

// --- singleton wrappers ------------------------------------------------------

type singletonSortedNumeric struct {
	wrapped NumericDocValues
}

func (s *singletonSortedNumeric) Get(docID int) ([]int64, error) {
	v, err := s.wrapped.Get(docID)
	if err != nil {
		return nil, err
	}
	return []int64{v}, nil
}

func (s *singletonSortedNumeric) Advance(target int) (int, error) {
	return s.wrapped.Advance(target)
}

func (s *singletonSortedNumeric) NextDoc() (int, error) {
	return s.wrapped.NextDoc()
}

func (s *singletonSortedNumeric) DocID() int {
	return s.wrapped.DocID()
}

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
