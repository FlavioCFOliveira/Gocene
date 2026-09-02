// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildDocMaps computes, for every sub-reader, the mapping from its local
// docID to the merged segment's docID. Deleted documents map to -1.
//
// When the merged segment carries an index sort, the documents are renumbered
// into the merged sort order via a MultiSorter merge-sort across the (already
// per-segment-sorted) leaves (rmp #115); otherwise live documents are
// renumbered densely in (reader, docID) order — the exact order in which
// mergeFields writes the stored fields, so all per-format merges agree on the
// new doc numbering. Mirrors the net effect of MergeState.DocMap (Lucene
// 10.4.0) for both the index-sort and no-index-sort cases.
func (sm *SegmentMerger) buildDocMaps() error {
	if sm.MergeState.DocMaps != nil {
		return nil // already computed
	}

	if sort := sm.MergeState.SegmentInfo.IndexSort(); sort != nil && len(sort.Fields()) > 0 {
		maps, err := multiSorterSort(sort, sm.MergeState.Readers)
		if err != nil {
			return fmt.Errorf("index: index-sorted merge: %w", err)
		}
		if maps != nil {
			// A reorder is required: the segments are not already globally
			// sorted relative to one another.
			sm.MergeState.DocMaps = maps
			sm.MergeState.NeedsIndexSort = true
			sm.buildInverseDocMap()
			return nil
		}
		// maps == nil: the leaves are already in global sort order, so the
		// plain concatenation map below preserves the sort.
	}

	docMaps := make([]DocMap, len(sm.MergeState.Readers))
	newDocID := 0
	for i := range sm.MergeState.Readers {
		maxDoc := sm.MergeState.MaxDocs[i]
		live := sm.MergeState.LiveDocs[i]
		m := make([]int, maxDoc)
		for d := 0; d < maxDoc; d++ {
			if live != nil && !live.Get(d) {
				m[d] = -1
				continue
			}
			m[d] = newDocID
			newDocID++
		}
		docMaps[i] = sliceDocMap(m)
	}
	sm.MergeState.DocMaps = docMaps
	return nil
}

// buildInverseDocMap fills sm.inverseDocMap with one (readerIndex, localDocID)
// entry per merged docID, derived from the per-reader DocMaps. It is used by
// the per-document merge steps to iterate in merged (sorted) docID order.
func (sm *SegmentMerger) buildInverseDocMap() {
	total := 0
	for i := range sm.MergeState.Readers {
		maxDoc := sm.MergeState.MaxDocs[i]
		dm := sm.MergeState.DocMaps[i]
		for d := 0; d < maxDoc; d++ {
			if dm.Get(d) >= 0 {
				total++
			}
		}
	}
	inv := make([][2]int, total)
	for i := range sm.MergeState.Readers {
		maxDoc := sm.MergeState.MaxDocs[i]
		dm := sm.MergeState.DocMaps[i]
		for d := 0; d < maxDoc; d++ {
			if nd := dm.Get(d); nd >= 0 && nd < total {
				inv[nd] = [2]int{i, d}
			}
		}
	}
	sm.inverseDocMap = inv
}

// sliceDocMap is a DocMap backed by a precomputed old→new slice.
type sliceDocMap []int

func (m sliceDocMap) Get(oldDocID int) int {
	if oldDocID < 0 || oldDocID >= len(m) {
		return -1
	}
	return m[oldDocID]
}

// mergeTerms merges the term dictionaries and postings of every indexed field
// across the source segments into the new segment, remapping each posting's
// docID through the merge DocMaps. It mirrors the net effect of Lucene's
// FieldsConsumer.merge(MergeState) (rmp #14/#114).
func (sm *SegmentMerger) mergeTerms() error {
	if sm.codec == nil || sm.codec.PostingsFormat() == nil {
		return nil
	}
	if sm.MergeState.DocMaps == nil {
		if err := sm.buildDocMaps(); err != nil {
			return err
		}
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix:  "",
			NeedsIndexSort: sm.MergeState.NeedsIndexSort,
			IsMerge:        true,
	}
	consumer, err := sm.codec.PostingsFormat().FieldsConsumer(state)
	if err != nil {
		return fmt.Errorf("index: merge postings: open consumer: %w", err)
	}
	defer consumer.Close()

	iter := sm.MergeState.MergeFieldInfos.Iterator()
	for iter.HasNext() {
		info := iter.Next()
		if info.IndexOptions() == IndexOptionsNone {
			continue // field is not indexed; no postings to merge
		}
		field := info.Name()

		var subs []Terms
		var subMaps []DocMap
		for i, reader := range sm.MergeState.Readers {
			if reader == nil {
				continue
			}
			terms, err := reader.Terms(field)
			if err != nil {
				return fmt.Errorf("index: merge postings: terms %q of reader %d: %w", field, i, err)
			}
			if terms == nil {
				continue // field absent in this sub
			}
			subs = append(subs, terms)
			subMaps = append(subMaps, sm.MergeState.DocMaps[i])
		}
		if len(subs) == 0 {
			continue
		}
		merged := &mergeFieldTerms{subs: subs, docMaps: subMaps, fieldInfo: info}
		if err := consumer.Write(field, merged); err != nil {
			return fmt.Errorf("index: merge postings: write field %q: %w", field, err)
		}
	}
	return nil
}

// mergeFieldTerms is a Terms view over one field's per-segment Terms whose
// postings are remapped to the merged doc space. The block-tree terms writer
// only calls GetIterator and reads the per-field flags from FieldInfo, so the
// statistical accessors return best-effort values.
type mergeFieldTerms struct {
	subs      []Terms
	docMaps   []DocMap
	fieldInfo *FieldInfo
}

func (t *mergeFieldTerms) GetIterator() (TermsEnum, error) {
	enums := make([]TermsEnum, len(t.subs))
	curr := make([]*Term, len(t.subs))
	for i, s := range t.subs {
		te, err := s.GetIterator()
		if err != nil {
			return nil, err
		}
		enums[i] = te
	}
	return &mergeTermsEnum{enums: enums, curr: curr, docMaps: t.docMaps}, nil
}

func (t *mergeFieldTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, err := te.SeekCeil(seekTerm); err != nil {
			return nil, err
		}
	}
	return te, nil
}

func (t *mergeFieldTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(NewTerm(t.fieldInfo.Name(), termText))
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

func (t *mergeFieldTerms) Size() int64                   { return -1 }
func (t *mergeFieldTerms) GetDocCount() (int, error)     { return -1, nil }
func (t *mergeFieldTerms) GetSumDocFreq() (int64, error) { return -1, nil }
func (t *mergeFieldTerms) GetSumTotalTermFreq() (int64, error) {
	return -1, nil
}
func (t *mergeFieldTerms) HasFreqs() bool {
	return t.fieldInfo.IndexOptions() >= IndexOptionsDocsAndFreqs
}
func (t *mergeFieldTerms) HasOffsets() bool {
	return t.fieldInfo.IndexOptions() >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}
func (t *mergeFieldTerms) HasPositions() bool {
	return t.fieldInfo.IndexOptions() >= IndexOptionsDocsAndFreqsAndPositions
}
func (t *mergeFieldTerms) HasPayloads() bool { return t.fieldInfo.HasPayloads() }
func (t *mergeFieldTerms) GetMin() (*Term, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	return te.Next()
}
func (t *mergeFieldTerms) GetMax() (*Term, error) { return nil, nil }

// mergeTermsEnum performs a k-way merge of the per-segment term streams. The
// term order is identical across segments, so a simple smallest-term selection
// yields the sorted union; for each emitted term, Postings concatenates the
// postings of every sub positioned on that term, remapped via the DocMaps.
type mergeTermsEnum struct {
	enums   []TermsEnum
	curr    []*Term // current term of each sub (nil if exhausted)
	docMaps []DocMap
	primed  bool
	current *Term
}

func (e *mergeTermsEnum) Next() (*Term, error) {
	if !e.primed {
		for i := range e.enums {
			t, err := e.enums[i].Next()
			if err != nil {
				return nil, err
			}
			e.curr[i] = t
		}
		e.primed = true
	} else if e.current != nil {
		// Advance every sub that was positioned on the just-emitted term.
		cb := termBytesOf(e.current)
		for i := range e.enums {
			if e.curr[i] != nil && bytes.Equal(termBytesOf(e.curr[i]), cb) {
				t, err := e.enums[i].Next()
				if err != nil {
					return nil, err
				}
				e.curr[i] = t
			}
		}
	}

	var min *Term
	var minBytes []byte
	for i := range e.curr {
		if e.curr[i] == nil {
			continue
		}
		b := termBytesOf(e.curr[i])
		if min == nil || bytes.Compare(b, minBytes) < 0 {
			min, minBytes = e.curr[i], b
		}
	}
	e.current = min
	return min, nil
}

func (e *mergeTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if e.current == nil {
		return nil, nil
	}
	cb := termBytesOf(e.current)
	var parts []mergePostingsPart
	for i := range e.enums {
		if e.curr[i] != nil && bytes.Equal(termBytesOf(e.curr[i]), cb) {
			// Always request the full postings payload so the writer can read
			// positions/offsets/payloads regardless of the flags it passes.
			pe, err := e.enums[i].Postings(PostingsFlagAll)
			if err != nil {
				return nil, err
			}
			if pe != nil {
				parts = append(parts, mergePostingsPart{pe: pe, docMap: e.docMaps[i]})
			}
		}
	}
	return &mergeMappingPostings{parts: parts, idx: -1}, nil
}

func (e *mergeTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (PostingsEnum, error) {
	// Source readers already exclude deleted docs via the DocMaps (deleted ->
	// -1), so live-docs filtering is folded into the mapping.
	return e.Postings(flags)
}

func (e *mergeTermsEnum) SeekCeil(target *Term) (*Term, error) {
	tb := termBytesOf(target)
	for i := range e.enums {
		t, err := e.enums[i].SeekCeil(target)
		if err != nil {
			return nil, err
		}
		e.curr[i] = t
	}
	e.primed = true
	var min *Term
	var minBytes []byte
	for i := range e.curr {
		if e.curr[i] == nil {
			continue
		}
		b := termBytesOf(e.curr[i])
		if min == nil || bytes.Compare(b, minBytes) < 0 {
			min, minBytes = e.curr[i], b
		}
	}
	e.current = min
	_ = tb
	return min, nil
}

func (e *mergeTermsEnum) SeekExact(target *Term) (bool, error) {
	t, err := e.SeekCeil(target)
	if err != nil {
		return false, err
	}
	return t != nil && bytes.Equal(termBytesOf(t), termBytesOf(target)), nil
}

func (e *mergeTermsEnum) Term() *Term { return e.current }

func (e *mergeTermsEnum) DocFreq() (int, error) {
	if e.current == nil {
		return 0, nil
	}
	cb := termBytesOf(e.current)
	total := 0
	for i := range e.enums {
		if e.curr[i] != nil && bytes.Equal(termBytesOf(e.curr[i]), cb) {
			df, err := e.enums[i].DocFreq()
			if err != nil {
				return 0, err
			}
			total += df
		}
	}
	return total, nil
}

func (e *mergeTermsEnum) TotalTermFreq() (int64, error) {
	if e.current == nil {
		return 0, nil
	}
	cb := termBytesOf(e.current)
	var total int64
	for i := range e.enums {
		if e.curr[i] != nil && bytes.Equal(termBytesOf(e.curr[i]), cb) {
			ttf, err := e.enums[i].TotalTermFreq()
			if err != nil {
				return 0, err
			}
			if ttf > 0 {
				total += ttf
			}
		}
	}
	return total, nil
}

// mergePostingsPart is one source segment's postings for the current term,
// together with the DocMap that rebases its docIDs into the merged space.
type mergePostingsPart struct {
	pe     PostingsEnum
	docMap DocMap
}

// mergeMappingPostings concatenates the per-segment postings of one term into
// the merged doc space. Because each segment occupies a contiguous, ordered,
// disjoint range of merged docIDs, iterating the parts in order yields a
// strictly increasing docID stream. Deleted source docs (DocMap -> -1) are
// skipped.
type mergeMappingPostings struct {
	parts []mergePostingsPart
	idx   int // index of the part currently being consumed
	doc   int
}

func (p *mergeMappingPostings) NextDoc() (int, error) {
	if p.idx == -1 {
		p.idx = 0
	}
	for p.idx < len(p.parts) {
		part := p.parts[p.idx]
		for {
			d, err := part.pe.NextDoc()
			if err != nil {
				return 0, err
			}
			if d == NO_MORE_DOCS {
				break
			}
			mapped := part.docMap.Get(d)
			if mapped < 0 {
				continue // deleted in the source segment
			}
			p.doc = mapped
			return mapped, nil
		}
		p.idx++
	}
	p.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (p *mergeMappingPostings) Advance(target int) (int, error) {
	for {
		d, err := p.NextDoc()
		if err != nil {
			return 0, err
		}
		if d == NO_MORE_DOCS || d >= target {
			return d, nil
		}
	}
}

func (p *mergeMappingPostings) DocID() int { return p.doc }

func (p *mergeMappingPostings) current() PostingsEnum {
	if p.idx < 0 || p.idx >= len(p.parts) {
		return nil
	}
	return p.parts[p.idx].pe
}

func (p *mergeMappingPostings) Freq() (int, error) {
	if c := p.current(); c != nil {
		return c.Freq()
	}
	return 0, nil
}

func (p *mergeMappingPostings) NextPosition() (int, error) {
	if c := p.current(); c != nil {
		return c.NextPosition()
	}
	return -1, nil
}

func (p *mergeMappingPostings) StartOffset() (int, error) {
	if c := p.current(); c != nil {
		return c.StartOffset()
	}
	return -1, nil
}

func (p *mergeMappingPostings) EndOffset() (int, error) {
	if c := p.current(); c != nil {
		return c.EndOffset()
	}
	return -1, nil
}

func (p *mergeMappingPostings) GetPayload() ([]byte, error) {
	if c := p.current(); c != nil {
		return c.GetPayload()
	}
	return nil, nil
}

func (p *mergeMappingPostings) Cost() int64 {
	var total int64
	for _, part := range p.parts {
		total += part.pe.Cost()
	}
	return total
}

// termBytesOf returns the term's bytes for ordering/equality, falling back to
// the UTF-8 of its text when no BytesRef is attached.
func termBytesOf(t *Term) []byte {
	if t == nil {
		return nil
	}
	if bv := t.BytesValue(); bv != nil {
		return bv.ValidBytes()
	}
	return []byte(t.Text())
}
