// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// EnumWithSlice pairs a PostingsEnum with the ReaderSlice that describes how
// the sub-reader fits into the composite reader's doc-ID space. Mirrors
// org.apache.lucene.index.MultiPostingsEnum.EnumWithSlice from Apache Lucene
// 10.4.0.
type EnumWithSlice struct {
	// PostingsEnum is the per-sub-reader postings iterator.
	PostingsEnum PostingsEnum

	// Slice describes the sub-reader's position in the composite doc-ID space.
	Slice ReaderSlice
}

// String returns a human-readable description of the pair.
func (e *EnumWithSlice) String() string {
	return fmt.Sprintf("%v:%v", e.Slice, e.PostingsEnum)
}

// MultiPostingsEnum exposes a merged PostingsEnum over the postings lists of
// several sub-readers, translating local sub-reader doc IDs to global composite
// doc IDs by adding the sub-reader's base offset. Mirrors
// org.apache.lucene.index.MultiPostingsEnum from Apache Lucene 10.4.0.
//
// Callers build a MultiPostingsEnum from a MultiTermsEnum via Reset, then use
// it exactly as any other PostingsEnum. The enum is reusable: call Reset again
// with new subs to avoid allocation on repeated term lookups.
type MultiPostingsEnum struct {
	parent *MultiTermsEnum

	// subPostingsEnums is the reusable backing array for per-sub PostingsEnums.
	subPostingsEnums []PostingsEnum

	// subs is the active slice of (postingsEnum, slice) pairs for the current term.
	subs []EnumWithSlice

	// numSubs is the number of active sub-enumerators.
	numSubs int

	// upto is the index into subs of the currently active sub-enum.
	upto int

	// current is the currently active sub-enum (nil if exhausted or before first call).
	current PostingsEnum

	// currentBase is the doc-ID base offset of the current sub-enum's slice.
	currentBase int

	// doc is the current composite doc ID (-1 before first call).
	doc int
}

// NewMultiPostingsEnum constructs a MultiPostingsEnum owned by the given
// MultiTermsEnum, with capacity for subReaderCount sub-enumerators.
func NewMultiPostingsEnum(parent *MultiTermsEnum, subReaderCount int) *MultiPostingsEnum {
	subs := make([]EnumWithSlice, subReaderCount)
	return &MultiPostingsEnum{
		parent:           parent,
		subPostingsEnums: make([]PostingsEnum, subReaderCount),
		subs:             subs,
		doc:              -1,
	}
}

// CanReuse reports whether this instance was created by the given
// MultiTermsEnum and can therefore be recycled for a new term.
func (m *MultiPostingsEnum) CanReuse(parent *MultiTermsEnum) bool {
	return m.parent == parent
}

// Reset rebinds this instance to a fresh set of (postingsEnum, slice) pairs
// and resets iteration state. Mirrors MultiPostingsEnum.reset() in Lucene.
func (m *MultiPostingsEnum) Reset(subs []EnumWithSlice, numSubs int) *MultiPostingsEnum {
	m.numSubs = numSubs
	for i := 0; i < numSubs; i++ {
		m.subs[i].PostingsEnum = subs[i].PostingsEnum
		m.subs[i].Slice = subs[i].Slice
	}
	m.upto = -1
	m.doc = -1
	m.current = nil
	return m
}

// GetNumSubs returns the number of active sub-enumerators for the current term.
func (m *MultiPostingsEnum) GetNumSubs() int { return m.numSubs }

// GetSubs returns the active (PostingsEnum, ReaderSlice) pairs.
func (m *MultiPostingsEnum) GetSubs() []EnumWithSlice { return m.subs[:m.numSubs] }

// Freq returns the term frequency of the current document.
// Panics if no sub-enum is active (caller contract violation).
func (m *MultiPostingsEnum) Freq() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiPostingsEnum.Freq: no active sub-enum")
	}
	return m.current.Freq()
}

// DocID returns the current composite doc ID, or -1 before the first call.
func (m *MultiPostingsEnum) DocID() int { return m.doc }

// Advance advances to the first document at or beyond target. Mirrors the
// Lucene advance() implementation: if the current sub-enum has the target in
// its window it delegates to sub.advance, otherwise it falls through to the
// next sub-enum. Callers must ensure target > DocID().
func (m *MultiPostingsEnum) Advance(target int) (int, error) {
	for {
		if m.current != nil {
			var localDoc int
			var err error
			if target < m.currentBase {
				// target lies before this sub's window; the sub has already
				// passed target — just call nextDoc() to advance within the sub.
				localDoc, err = m.current.NextDoc()
			} else {
				localDoc, err = m.current.Advance(target - m.currentBase)
			}
			if err != nil {
				return 0, fmt.Errorf("MultiPostingsEnum.Advance: sub advance: %w", err)
			}
			if localDoc == NO_MORE_DOCS {
				m.current = nil
			} else {
				m.doc = localDoc + m.currentBase
				return m.doc, nil
			}
		} else if m.upto == m.numSubs-1 {
			m.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		} else {
			m.upto++
			m.current = m.subs[m.upto].PostingsEnum
			m.currentBase = m.subs[m.upto].Slice.Start
		}
	}
}

// NextDoc advances to the next document in the merged sequence. Mirrors
// MultiPostingsEnum.nextDoc() in Lucene: iterates sub-enums in order,
// translating sub-local doc IDs to composite doc IDs via currentBase.
func (m *MultiPostingsEnum) NextDoc() (int, error) {
	for {
		if m.current == nil {
			if m.upto == m.numSubs-1 {
				m.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, nil
			}
			m.upto++
			m.current = m.subs[m.upto].PostingsEnum
			m.currentBase = m.subs[m.upto].Slice.Start
		}

		localDoc, err := m.current.NextDoc()
		if err != nil {
			return 0, fmt.Errorf("MultiPostingsEnum.NextDoc: sub nextDoc: %w", err)
		}
		if localDoc != NO_MORE_DOCS {
			m.doc = m.currentBase + localDoc
			return m.doc, nil
		}
		m.current = nil
	}
}

// NextPosition returns the next position within the current document.
func (m *MultiPostingsEnum) NextPosition() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiPostingsEnum.NextPosition: no active sub-enum")
	}
	return m.current.NextPosition()
}

// StartOffset returns the start offset of the current occurrence.
func (m *MultiPostingsEnum) StartOffset() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiPostingsEnum.StartOffset: no active sub-enum")
	}
	return m.current.StartOffset()
}

// EndOffset returns the end offset of the current occurrence.
func (m *MultiPostingsEnum) EndOffset() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiPostingsEnum.EndOffset: no active sub-enum")
	}
	return m.current.EndOffset()
}

// GetPayload returns the payload of the current occurrence.
func (m *MultiPostingsEnum) GetPayload() ([]byte, error) {
	if m.current == nil {
		return nil, fmt.Errorf("MultiPostingsEnum.GetPayload: no active sub-enum")
	}
	return m.current.GetPayload()
}

// Cost returns the aggregate cost across all active sub-enumerators.
func (m *MultiPostingsEnum) Cost() int64 {
	var total int64
	for i := 0; i < m.numSubs; i++ {
		if m.subs[i].PostingsEnum != nil {
			total += m.subs[i].PostingsEnum.Cost()
		}
	}
	return total
}

// Compile-time assertion that MultiPostingsEnum satisfies PostingsEnum.
var _ PostingsEnum = (*MultiPostingsEnum)(nil)
