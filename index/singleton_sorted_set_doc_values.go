// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file ports org.apache.lucene.index.SingletonSortedSetDocValues from
// Apache Lucene 10.4.0.
//
// Lucene exposes the type as a package-private final class that adapts a
// single-valued SortedDocValues to the multi-valued SortedSetDocValues
// surface, so a single implementation can serve callers that need either
// view. Gocene keeps the wrapper unexported for the same reason; the only
// public entry points are DocValues.SingletonSortedSet (constructor) and
// DocValues.UnwrapSingletonSortedSet (downcast helper), both declared in
// doc_values.go.
//
// Lucene 10.4 enriches the iterator surface with nextOrd / docValueCount /
// advanceExact / intoBitSet / docIDRunEnd / lookupTerm / termsEnum / cost.
// The Gocene SortedSetDocValues / SortedDocValues interfaces in
// doc_values_interfaces.go currently only declare Get / Advance / NextDoc /
// DocID / LookupOrd / GetValueCount; those richer methods will be threaded
// through once the rest of the doc-values stack catches up. The accessor
// GetSortedDocValues mirrors Lucene's getSortedDocValues so callers can
// recover the underlying single-valued iterator without violating the
// "iterator already used" invariant.

// singletonSortedSet adapts a single-valued SortedDocValues to the
// multi-valued SortedSetDocValues contract.
//
// Lucene's constructor refuses an iterator whose docID is anything other
// than -1 (the "not yet positioned" sentinel) because handing out a partial
// iterator would silently skip documents. The Gocene factory
// DocValues.SingletonSortedSet(SortedDocValues) preserves that guarantee at
// the type level — callers always pass a freshly minted iterator — so the
// wrapper trusts its input here. GetSortedDocValues re-asserts the same
// invariant at runtime, matching Lucene.
type singletonSortedSet struct {
	wrapped SortedDocValues
}

// GetSortedDocValues returns the wrapped SortedDocValues iterator.
//
// Mirrors Lucene's SingletonSortedSetDocValues#getSortedDocValues: the
// wrapper is only meaningful as long as the underlying iterator is pristine.
// Returning the iterator after it has already been consumed would let the
// caller resume mid-stream and miss documents, so we require docID() == -1
// just like the Java reference. The error type reuses [IllegalStateError]
// declared alongside the SingletonSortedNumericDocValues port.
func (s *singletonSortedSet) GetSortedDocValues() (SortedDocValues, error) {
	if id := s.wrapped.DocID(); id != -1 {
		return nil, &IllegalStateError{Op: "SingletonSortedSetDocValues.GetSortedDocValues", DocID: id}
	}
	return s.wrapped, nil
}

// Get returns the wrapped ordinal packed as a one-element slice. The Lucene
// equivalent is the pair (docValueCount, nextOrd) which always yields a
// single ord for this wrapper; until the Gocene iterator surface grows those
// methods, the slice form keeps the semantics observable without breaking
// existing consumers. A negative ord (Lucene's "no value" sentinel returned
// by SortedDocValues#ordValue when the document has no term) collapses to a
// nil slice, matching the empty-multi-value contract of SortedSetDocValues.
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

// Advance delegates to the wrapped iterator; the multi-valued surface is
// pure adapter, no extra state.
func (s *singletonSortedSet) Advance(target int) (int, error) {
	return s.wrapped.Advance(target)
}

// NextDoc delegates to the wrapped iterator.
func (s *singletonSortedSet) NextDoc() (int, error) {
	return s.wrapped.NextDoc()
}

// DocID delegates to the wrapped iterator.
func (s *singletonSortedSet) DocID() int {
	return s.wrapped.DocID()
}

// LookupOrd delegates to the wrapped iterator. Lucene casts the long ord
// down to int before delegating; Gocene's ord type is already int, so the
// downcast is implicit.
func (s *singletonSortedSet) LookupOrd(ord int) ([]byte, error) {
	return s.wrapped.LookupOrd(ord)
}

// GetValueCount delegates to the wrapped iterator. The total ord cardinality
// is identical between the single-valued and multi-valued views because the
// wrapper never invents new ords — it only exposes the same ord as a
// length-1 multi-value.
func (s *singletonSortedSet) GetValueCount() int {
	return s.wrapped.GetValueCount()
}
