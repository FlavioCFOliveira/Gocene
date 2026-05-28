// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "github.com/FlavioCFOliveira/Gocene/index"

// AdvanceExact methods for the in-memory MemoryDocValuesProducer test
// helper types. Required by the iterator surface unified onto spi.X by
// rmp #4709 (Sprint 118 phase 2e additive). The implementations linear
// scan dv.docIDs for target and update the cursor; the helpers exist
// for tests and do not need to be hot-path-optimised.

func (dv *memoryNumericDocValues) AdvanceExact(target int) (bool, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] == target {
			dv.currDoc = target
			return true, nil
		}
		if dv.docIDs[dv.pos] > target {
			// Step back so the next NextDoc/Advance returns this docID.
			dv.pos--
			dv.currDoc = target
			return false, nil
		}
	}
	dv.currDoc = target
	return false, nil
}

func (dv *memoryBinaryDocValues) AdvanceExact(target int) (bool, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] == target {
			dv.currDoc = target
			return true, nil
		}
		if dv.docIDs[dv.pos] > target {
			dv.pos--
			dv.currDoc = target
			return false, nil
		}
	}
	dv.currDoc = target
	return false, nil
}

// memorySortedDocValues embeds memoryBinaryDocValues, so it inherits
// AdvanceExact above automatically. No explicit method needed.

func (dv *memorySortedSetDocValues) AdvanceExact(target int) (bool, error) {
	for dv.pos+1 < len(dv.docIDs) {
		dv.pos++
		if dv.docIDs[dv.pos] == target {
			dv.currDoc = target
			dv.currOrd = -1
			return true, nil
		}
		if dv.docIDs[dv.pos] > target {
			dv.pos--
			dv.currDoc = target
			dv.currOrd = -1
			return false, nil
		}
	}
	dv.currDoc = index.NO_MORE_DOCS
	return false, nil
}

// memorySortedNumericDocValues embeds memoryNumericDocValues, so it
// inherits AdvanceExact above automatically. No explicit method needed.
