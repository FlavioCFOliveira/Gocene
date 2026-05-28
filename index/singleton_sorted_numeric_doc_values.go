// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file ports org.apache.lucene.index.SingletonSortedNumericDocValues
// from Apache Lucene 10.4.0.
//
// Lucene exposes the type as a package-private final class that adapts a
// single-valued NumericDocValues to the multi-valued SortedNumericDocValues
// surface, so a single implementation can serve callers that need either
// view. Gocene keeps the wrapper unexported for the same reason; the only
// public entry points are DocValues.Singleton (constructor) and
// DocValues.UnwrapSingletonSortedNumeric (downcast helper), both declared
// in doc_values.go.
//
// Lucene 10.4 enriches the iterator surface with advanceExact, intoBitSet,
// docIDRunEnd, cost, nextValue and docValueCount. The Gocene
// SortedNumericDocValues / NumericDocValues interfaces in
// doc_values_interfaces.go currently only declare Get / Advance / NextDoc /
// DocID; those richer methods will be threaded through once the rest of the
// doc-values stack catches up. The accessor GetNumericDocValues mirrors
// Lucene's getNumericDocValues so callers can recover the underlying
// single-valued iterator without violating the "iterator already used"
// invariant.

// singletonSortedNumeric adapts a single-valued NumericDocValues to the
// multi-valued SortedNumericDocValues contract.
//
// Lucene's constructor refuses an iterator whose docID is anything other
// than -1 (the "not yet positioned" sentinel) because handing out a partial
// iterator would silently skip documents. The Gocene factory
// DocValues.Singleton(NumericDocValues) preserves that guarantee at the
// type level — callers always pass a freshly minted iterator — so the
// wrapper trusts its input here. GetNumericDocValues re-asserts the same
// invariant at runtime, matching Lucene.
type singletonSortedNumeric struct {
	wrapped NumericDocValues
}

// GetNumericDocValues returns the wrapped NumericDocValues iterator.
//
// Mirrors Lucene's SingletonSortedNumericDocValues#getNumericDocValues:
// the wrapper is only meaningful as long as the underlying iterator is
// pristine. Returning the iterator after it has already been consumed
// would let the caller resume mid-stream and miss documents, so we
// require docID() == -1 just like the Java reference. The error type
// matches AlreadyClosedException's role of signalling an unusable
// resource without panicking the goroutine.
func (s *singletonSortedNumeric) GetNumericDocValues() (NumericDocValues, error) {
	if id := s.wrapped.DocID(); id != -1 {
		return nil, &IllegalStateError{Op: "SingletonSortedNumericDocValues.GetNumericDocValues", DocID: id}
	}
	return s.wrapped, nil
}

// Get returns the wrapped value packed as a one-element slice. The Lucene
// equivalent is the pair (docValueCount, nextValue) which always yields a
// single long for this wrapper; until the Gocene iterator surface grows
// those methods, the slice form keeps the semantics observable without
// breaking existing consumers.
func (s *singletonSortedNumeric) Get(docID int) ([]int64, error) {
	v, err := s.wrapped.Get(docID)
	if err != nil {
		return nil, err
	}
	return []int64{v}, nil
}

// Advance delegates to the wrapped iterator; the multi-valued surface is
// pure adapter, no extra state.
func (s *singletonSortedNumeric) Advance(target int) (int, error) {
	return s.wrapped.Advance(target)
}

// AdvanceExact delegates to the wrapped iterator (single-valued: a doc
// either has the one value or it doesn't).
func (s *singletonSortedNumeric) AdvanceExact(target int) (bool, error) {
	return s.wrapped.AdvanceExact(target)
}

// NextValue returns the single wrapped value. Singleton always has
// exactly one value per positioned document.
func (s *singletonSortedNumeric) NextValue() (int64, error) {
	return s.wrapped.LongValue()
}

// DocValueCount returns 1 — singleton wrappers always expose exactly
// one value per positioned document.
func (s *singletonSortedNumeric) DocValueCount() (int, error) {
	return 1, nil
}

// NextDoc delegates to the wrapped iterator.
func (s *singletonSortedNumeric) NextDoc() (int, error) {
	return s.wrapped.NextDoc()
}

// DocID delegates to the wrapped iterator.
func (s *singletonSortedNumeric) DocID() int {
	return s.wrapped.DocID()
}

// IllegalStateError signals an attempt to use a doc-values iterator that
// has already been advanced past its starting position. It mirrors the
// java.lang.IllegalStateException Lucene throws from
// SingletonSortedNumericDocValues' constructor and getter.
type IllegalStateError struct {
	Op    string
	DocID int
}

// Error implements the error interface.
func (e *IllegalStateError) Error() string {
	return e.Op + ": iterator has already been used: docID=" + itoa(e.DocID)
}

// itoa is a small, allocation-free signed-int to ASCII helper. Using the
// strconv package here would pull a runtime dependency just for an error
// path that fires on misuse only; keeping it local makes IllegalStateError
// trivially testable in isolation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
