// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

// AdvanceExact shims for the SimpleText doc-values iterators, required
// by the iterator surface unified onto spi.X by rmp #4709 (Sprint 118
// phase 2e additive).
//
// Semantics mirror the Lucene contract: Advance to target; the iterator
// has a value for target iff the resulting docID equals target. The
// SimpleText format is a debugging format and does not have a sparse
// skip index, so AdvanceExact runs the same line-by-line scan as
// Advance.

func (it *dvNumericIter) AdvanceExact(target int) (bool, error) {
	got, err := it.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (it *dvBinaryIter) AdvanceExact(target int) (bool, error) {
	got, err := it.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (it *dvSortedIter) AdvanceExact(target int) (bool, error) {
	got, err := it.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (it *dvSortedNumericIter) AdvanceExact(target int) (bool, error) {
	got, err := it.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (it *dvSortedSetIter) AdvanceExact(target int) (bool, error) {
	got, err := it.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}
