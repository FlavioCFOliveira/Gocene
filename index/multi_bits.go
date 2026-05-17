// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// MultiBits concatenates several Bits objects into a single virtual Bits
// over a wider docID space. Mirrors org.apache.lucene.index.MultiBits from
// Apache Lucene 10.4.0.
type MultiBits struct {
	subs   []util.Bits
	starts []int // start[i] is the docBase of subs[i]; length = len(subs)+1
}

// NewMultiBits builds a MultiBits from per-sub-reader Bits and matching
// starts (the docBase per sub). starts must have length len(subs)+1, with
// the last entry equal to the total maxDoc.
func NewMultiBits(subs []util.Bits, starts []int) *MultiBits {
	if len(starts) != len(subs)+1 {
		panic("MultiBits: starts length must be len(subs)+1")
	}
	return &MultiBits{subs: subs, starts: starts}
}

// Get reports whether docID is set in the underlying sub-Bits. If the sub at
// the corresponding slot is nil, all docs in that slot are considered live
// (matches Lucene's null = "no deletions" convention).
func (m *MultiBits) Get(docID int) bool {
	// Binary search for the sub that contains docID.
	lo, hi := 0, len(m.starts)-1
	for lo < hi {
		mid := (lo + hi) >> 1
		if docID < m.starts[mid+1] {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo >= len(m.subs) {
		return false
	}
	sub := m.subs[lo]
	if sub == nil {
		return true
	}
	return sub.Get(docID - m.starts[lo])
}

// Length returns the total docID space (matches Lucene's length()).
func (m *MultiBits) Length() int {
	return m.starts[len(m.starts)-1]
}
