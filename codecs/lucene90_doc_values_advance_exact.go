// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// This file gathers the AdvanceExact methods required by the iterator
// surface unified onto spi.NumericDocValues / BinaryDocValues /
// SortedDocValues / SortedSetDocValues / SortedNumericDocValues by
// rmp #4709 (Sprint 118 phase 2e additive). Keeping these in a single
// companion file makes the additive surface easy to audit and easy to
// remove if the iterator contract ever sheds AdvanceExact.
//
// Semantics:
//   - Dense iterators (value-for-every-doc): AdvanceExact(target)
//     repositions doc to target and returns (true, nil) when target <
//     maxDoc, otherwise (false, nil) and parks at NO_MORE_DOCS.
//   - Sparse iterators (DISI-backed): delegate to dvIndexedDISI.
//   - Empty iterators: always return false.
//   - Composite iterators: delegate to the underlying value-bearing
//     iterator they wrap.

// --- NumericDocValues family -------------------------------------------------

func (emptyNumericDV) AdvanceExact(int) (bool, error) { return false, nil }

func (d *denseConstNumericDV) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return false, nil
	}
	d.doc = target
	return true, nil
}

func (d *denseNumericDV) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return false, nil
	}
	d.doc = target
	return true, nil
}

func (s *sparseConstNumericDV) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

func (s *sparseNumericDV) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

// --- BinaryDocValues family --------------------------------------------------

func (emptyBinaryDV) AdvanceExact(int) (bool, error) { return false, nil }

func (d *denseFixedBinaryDV) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return false, nil
	}
	d.doc = target
	return true, nil
}

func (d *denseVarBinaryDV) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return false, nil
	}
	d.doc = target
	return true, nil
}

func (s *sparseFixedBinaryDV) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

func (s *sparseVarBinaryDV) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

// --- SortedDocValues family --------------------------------------------------

func (s *sortedDVDense) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= s.maxDoc {
		s.doc = dvNoMoreDocs
		return false, nil
	}
	s.doc = target
	return true, nil
}

func (s *sortedDVSparse) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

func (s *sortedDVGeneral) AdvanceExact(target int) (bool, error) {
	return s.ndv.AdvanceExact(target)
}

// --- SortedSetDocValues family ----------------------------------------------

func (s *sortedSetDVDense) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= s.maxDoc {
		s.doc = dvNoMoreDocs
		return false, nil
	}
	s.doc = target
	// Mirror the eager reload Advance/NextDoc do on this dense path so
	// NextOrd / DocValueCount return the values bound to target.
	s.curr = s.addrs.Get(int64(s.doc))
	s.count = int(s.addrs.Get(int64(s.doc)+1) - s.curr)
	return true, nil
}

func (s *sortedSetDVSparse) AdvanceExact(target int) (bool, error) {
	ok, err := s.disi.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	// Force the count/ord reload on next NextOrd; matches the lazy
	// setIfNeeded() pattern used elsewhere on this type.
	s.set = false
	return ok, nil
}

func (s *sortedSetDVGeneral) AdvanceExact(target int) (bool, error) {
	return s.sndv.AdvanceExact(target)
}

// --- SortedNumericDocValues family ------------------------------------------

func (d *sortedNumericDVDense) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return false, nil
	}
	d.doc = target
	// Recompute the value window for the target document so DocValueCount and
	// NextValue read the correct slice, mirroring Advance/NextDoc. Every doc in
	// a dense SortedNumeric carries at least one value.
	d.start = d.addrs.Get(int64(d.doc))
	d.end = d.addrs.Get(int64(d.doc) + 1)
	d.count = int(d.end - d.start)
	return true, nil
}

func (s *sortedNumericDVSparse) AdvanceExact(target int) (bool, error) {
	// Reset the cached value window so DocValueCount/NextValue recompute it for
	// the target document, mirroring NextDoc/Advance.
	s.set = false
	return s.disi.AdvanceExact(target)
}

// --- singleton wrappers in the producer -------------------------------------

func (s *singletonSS) AdvanceExact(target int) (bool, error) {
	s.ordConsumed = false
	return s.sdv.AdvanceExact(target)
}

func (s *singletonSN) AdvanceExact(target int) (bool, error) {
	s.valConsumed = false
	return s.ndv.AdvanceExact(target)
}
