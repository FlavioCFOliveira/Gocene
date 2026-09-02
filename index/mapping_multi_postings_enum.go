// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// MaxPosition is the largest position that can be stored in a posting list.
// Mirrors org.apache.lucene.index.IndexWriter.MAX_POSITION from Apache
// Lucene 10.4.0 (Integer.MAX_VALUE - 128). Defined here because IndexWriter
// in Gocene does not yet expose this constant.
const MaxPosition = int(^uint32(0)>>1) - 128

// MappingMultiPostingsEnum exposes the flex API merged from the flex APIs of
// sub-segments, remapping docIDs (this is used for segment merging). Mirrors
// org.apache.lucene.index.MappingMultiPostingsEnum from Apache Lucene 10.4.0.
//
// Lucene marks this type as @lucene.experimental and package-private; Gocene
// follows the same intent — the type lives in package index and is intended
// for codec-merge consumers.
type MappingMultiPostingsEnum struct {
	multiDocsAndPositionsEnum *MultiPostingsEnum
	field                     string
	docIDMerger               DocIDMerger
	current                   *mappingPostingsSub
	allSubs                   []*mappingPostingsSub
	subs                      []*mappingPostingsSub
}

// mappingPostingsSub adapts a per-segment PostingsEnum to the DocIDMerger.Sub
// contract, remapping docIDs through the merge DocMap.
type mappingPostingsSub struct {
	postings    PostingsEnum
	docMap      DocMap
	mappedDocID int
}

func newMappingPostingsSub(docMap DocMap) *mappingPostingsSub {
	return &mappingPostingsSub{docMap: docMap, mappedDocID: -1}
}

// MappedDocID returns the mapped docID for the current postings position.
func (s *mappingPostingsSub) MappedDocID() int { return s.mappedDocID }

// NextDoc advances the underlying PostingsEnum and returns the raw docID
// (pre-mapping). NO_MORE_DOCS signals exhaustion.
func (s *mappingPostingsSub) NextDoc() (int, error) {
	return s.postings.NextDoc()
}

// NextMappedDoc walks the postings until it lands on a docID the DocMap
// keeps (DocMap.Get != -1) or the postings are exhausted, returning the
// mapped docID (or NO_MORE_DOCS).
func (s *mappingPostingsSub) NextMappedDoc() (int, error) {
	for {
		raw, err := s.postings.NextDoc()
		if err != nil {
			return 0, err
		}
		if raw == NO_MORE_DOCS {
			s.mappedDocID = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(raw)
		if mapped != -1 {
			s.mappedDocID = mapped
			return mapped, nil
		}
	}
}

// NewMappingMultiPostingsEnum builds a MappingMultiPostingsEnum for the
// supplied field over the per-sub-reader DocMaps carried by mergeState.
// Mirrors the sole Lucene constructor.
func NewMappingMultiPostingsEnum(field string, mergeState *MergeState) (*MappingMultiPostingsEnum, error) {
	if mergeState == nil {
		return nil, fmt.Errorf("MappingMultiPostingsEnum: mergeState is nil")
	}
	n := len(mergeState.DocMaps)
	allSubs := make([]*mappingPostingsSub, n)
	for i := 0; i < n; i++ {
		allSubs[i] = newMappingPostingsSub(mergeState.DocMaps[i])
	}
	m := &MappingMultiPostingsEnum{
		field:   field,
		allSubs: allSubs,
		subs:    make([]*mappingPostingsSub, 0, n),
	}
	merger, err := NewDocIDMerger(toDocIDMergerSubs(m.subs), n, mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}
	m.docIDMerger = merger
	return m, nil
}

// Reset attaches a new MultiPostingsEnum, rebuilding the active sub list
// from its (postingsEnum, slice) pairs. Mirrors reset() in Lucene.
func (m *MappingMultiPostingsEnum) Reset(postingsEnum *MultiPostingsEnum) (*MappingMultiPostingsEnum, error) {
	if postingsEnum == nil {
		return nil, fmt.Errorf("MappingMultiPostingsEnum.Reset: postingsEnum is nil")
	}
	m.multiDocsAndPositionsEnum = postingsEnum
	subsArray := postingsEnum.GetSubs()
	count := postingsEnum.GetNumSubs()
	if count > len(subsArray) {
		return nil, fmt.Errorf("MappingMultiPostingsEnum.Reset: numSubs (%d) exceeds subs length (%d)", count, len(subsArray))
	}
	m.subs = m.subs[:0]
	for i := 0; i < count; i++ {
		readerIndex := subsArray[i].Slice.ReaderIndex
		if readerIndex < 0 || readerIndex >= len(m.allSubs) {
			return nil, fmt.Errorf("MappingMultiPostingsEnum.Reset: readerIndex %d out of range [0,%d)", readerIndex, len(m.allSubs))
		}
		sub := m.allSubs[readerIndex]
		sub.postings = subsArray[i].PostingsEnum
		m.subs = append(m.subs, sub)
	}
	// Rebuild merger over the new active sub list — Lucene relies on the
	// stable ArrayList reference; in Go we hand a fresh snapshot to the
	// merger, which is semantically equivalent.
	merger, err := newDocIDMergerFromSubs(m.subs, len(m.allSubs), m.docIDMergerIsSorted())
	if err != nil {
		return nil, err
	}
	m.docIDMerger = merger
	if err := m.docIDMerger.Reset(); err != nil {
		return nil, err
	}
	m.current = nil
	return m, nil
}

// docIDMergerIsSorted reports whether the current merger is the sorted
// variant. It is used to preserve the indexIsSorted hint across Reset
// calls.
func (m *MappingMultiPostingsEnum) docIDMergerIsSorted() bool {
	_, ok := m.docIDMerger.(*sortedDocIDMerger)
	return ok
}

func toDocIDMergerSubs(subs []*mappingPostingsSub) []DocIDMergerSub {
	out := make([]DocIDMergerSub, len(subs))
	for i, s := range subs {
		out[i] = s
	}
	return out
}

func newDocIDMergerFromSubs(subs []*mappingPostingsSub, maxCount int, indexIsSorted bool) (DocIDMerger, error) {
	return NewDocIDMerger(toDocIDMergerSubs(subs), maxCount, indexIsSorted)
}

// Freq returns the term frequency in the current document.
func (m *MappingMultiPostingsEnum) Freq() (int, error) {
	return m.current.postings.Freq()
}

// DocID returns the current mapped docID, or -1 before the first NextDoc.
func (m *MappingMultiPostingsEnum) DocID() int {
	if m.current == nil {
		return -1
	}
	return m.current.mappedDocID
}

// Advance is unsupported — mirrors Lucene which throws
// UnsupportedOperationException.
func (m *MappingMultiPostingsEnum) Advance(target int) (int, error) {
	return 0, fmt.Errorf("MappingMultiPostingsEnum.Advance: unsupported operation")
}

// NextDoc returns the next mapped docID, or NO_MORE_DOCS when exhausted.
func (m *MappingMultiPostingsEnum) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.current = nil
		return NO_MORE_DOCS, nil
	}
	m.current = sub.(*mappingPostingsSub)
	return m.current.mappedDocID, nil
}

// NextPosition returns the next position within the current document, or
// NO_MORE_POSITIONS when exhausted. Wraps positions with a CorruptIndex
// guard mirroring Lucene's bounds check against MaxPosition.
func (m *MappingMultiPostingsEnum) NextPosition() (int, error) {
	pos, err := m.current.postings.NextPosition()
	if err != nil {
		return 0, err
	}
	if pos < 0 {
		return 0, NewCorruptIndexException(
			fmt.Sprintf("position=%d is negative, field=\"%s doc=%d", pos, m.field, m.current.mappedDocID),
			fmt.Sprintf("%v", m.current.postings),
		)
	}
	if pos > MaxPosition {
		return 0, NewCorruptIndexException(
			fmt.Sprintf("position=%d is too large (> IndexWriter.MAX_POSITION=%d), field=\"%s\" doc=%d",
				pos, MaxPosition, m.field, m.current.mappedDocID),
			fmt.Sprintf("%v", m.current.postings),
		)
	}
	return pos, nil
}

// StartOffset returns the start character offset of the current occurrence,
// or -1 if offsets were not indexed.
func (m *MappingMultiPostingsEnum) StartOffset() (int, error) {
	return m.current.postings.StartOffset()
}

// EndOffset returns the end character offset of the current occurrence, or
// -1 if offsets were not indexed.
func (m *MappingMultiPostingsEnum) EndOffset() (int, error) {
	return m.current.postings.EndOffset()
}

// GetPayload returns the payload bytes for the current occurrence, or nil
// if there is no payload.
func (m *MappingMultiPostingsEnum) GetPayload() ([]byte, error) {
	return m.current.postings.GetPayload()
}

// Cost returns the aggregate cost across the active sub-enumerators.
func (m *MappingMultiPostingsEnum) Cost() int64 {
	var total int64
	for _, sub := range m.subs {
		total += sub.postings.Cost()
	}
	return total
}

// Compile-time assertion that MappingMultiPostingsEnum satisfies the
// PostingsEnum surface. Kept at file scope so an accidental signature drift
// breaks the build rather than a runtime call site.
var _ PostingsEnum = (*MappingMultiPostingsEnum)(nil)
