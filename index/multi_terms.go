// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
)

// ReaderSlice records the (start, length, readerIndex) coordinates of a slice
// of a parent IndexReader's doc-ID space. Mirrors
// org.apache.lucene.index.ReaderSlice (Apache Lucene 10.4.0). Lucene places
// this type in the same package; Gocene keeps it next to its sole user.
type ReaderSlice struct {
	Start       int
	Length      int
	ReaderIndex int
}

// MultiTerms aggregates Terms from several sub-segments into one virtual
// Terms instance. Mirrors org.apache.lucene.index.MultiTerms (Apache Lucene
// 10.4.0).
//
// Gocene skeleton: aggregation of TermsEnum across leaves is deferred to a
// follow-up sprint that lands MultiTermsEnum (see backlog #2706). The
// constructor, accessors and trivial aggregate stats are in place.
type MultiTerms struct {
	field     string
	subs      []Terms
	subSlices []ReaderSlice
}

// NewMultiTerms builds a MultiTerms over the supplied sub-Terms and matching
// slices. Returns an error when the slice lists differ in length.
func NewMultiTerms(subs []Terms, slices []ReaderSlice) (*MultiTerms, error) {
	return NewMultiTermsForField("", subs, slices)
}

// NewMultiTermsForField builds a MultiTerms that remembers the field name it
// aggregates, which GetPostingsReader needs to construct seek terms.
func NewMultiTermsForField(field string, subs []Terms, slices []ReaderSlice) (*MultiTerms, error) {
	if len(subs) != len(slices) {
		return nil, errors.New("MultiTerms: subs and slices must have the same length")
	}
	return &MultiTerms{field: field, subs: subs, subSlices: slices}, nil
}

// GetSubTerms returns the underlying sub-Terms.
func (m *MultiTerms) GetSubTerms() []Terms { return m.subs }

// GetSubSlices returns the underlying reader slices.
func (m *MultiTerms) GetSubSlices() []ReaderSlice { return m.subSlices }

// Size sums Size() across sub-Terms when known, returning -1 otherwise.
// Lucene returns -1 by contract because the union may contain duplicates.
func (m *MultiTerms) Size() int64 { return -1 }

// Iterator returns a MultiTermsEnum that merges, by term text in byte order,
// the TermsEnum of every sub-Terms. Mirrors MultiTerms.iterator (Apache Lucene
// 10.4.0): each sub-iterator is bound to its ReaderSlice and the merge is
// primed via Reset.
//
// When no sub-Terms has any term the returned TermsEnum is an empty enum whose
// Next immediately yields nil (mirroring Lucene's TermsEnum.EMPTY).
func (m *MultiTerms) Iterator() (TermsEnum, error) {
	enum := NewMultiTermsEnum(m.subSlices)
	subEnums := make([]TermsEnum, len(m.subs))
	for i, sub := range m.subs {
		te, err := sub.GetIterator()
		if err != nil {
			return nil, fmt.Errorf("MultiTerms.Iterator: sub %d: %w", i, err)
		}
		subEnums[i] = te
	}
	bound, err := enum.Reset(subEnums)
	if err != nil {
		return nil, fmt.Errorf("MultiTerms.Iterator: reset: %w", err)
	}
	if bound == nil {
		// No sub had any term: present an empty enum (TermsEnum.EMPTY).
		return &EmptyTermsEnum{}, nil
	}
	return bound, nil
}

// GetIterator returns the merged TermsEnum (Terms-interface alias for
// Iterator), so a MultiTerms can stand in for any single-segment Terms.
func (m *MultiTerms) GetIterator() (TermsEnum, error) { return m.Iterator() }

// GetIteratorWithSeek returns the merged TermsEnum positioned on the smallest
// term >= seekTerm, or nil when no such term exists.
func (m *MultiTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	it, err := m.Iterator()
	if err != nil {
		return nil, err
	}
	landed, err := it.SeekCeil(seekTerm)
	if err != nil {
		return nil, err
	}
	if landed == nil {
		return nil, nil
	}
	return it, nil
}

// GetPostingsReader merges the postings of termText across every sub-segment,
// remapping each sub's docIDs into the composite doc-ID space. Returns nil when
// the term is absent from all subs.
func (m *MultiTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	it, err := m.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := it.SeekExact(NewTerm(m.field, termText))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return it.Postings(flags)
}

// GetDocCount sums the per-sub document counts. Sub doc-ID spaces are disjoint,
// so the sum is the field's composite document count; a -1 from any sub (count
// unknown) makes the aggregate unknown.
func (m *MultiTerms) GetDocCount() (int, error) {
	total := 0
	for _, s := range m.subs {
		c, err := s.GetDocCount()
		if err != nil {
			return 0, err
		}
		if c < 0 {
			return -1, nil
		}
		total += c
	}
	return total, nil
}

// GetSumDocFreq sums the per-sub posting counts, propagating -1 (unknown).
func (m *MultiTerms) GetSumDocFreq() (int64, error) {
	var total int64
	for _, s := range m.subs {
		v, err := s.GetSumDocFreq()
		if err != nil {
			return 0, err
		}
		if v < 0 {
			return -1, nil
		}
		total += v
	}
	return total, nil
}

// GetSumTotalTermFreq sums the per-sub total term frequencies, propagating -1.
func (m *MultiTerms) GetSumTotalTermFreq() (int64, error) {
	var total int64
	for _, s := range m.subs {
		v, err := s.GetSumTotalTermFreq()
		if err != nil {
			return 0, err
		}
		if v < 0 {
			return -1, nil
		}
		total += v
	}
	return total, nil
}

// GetMin returns the smallest term across all subs.
func (m *MultiTerms) GetMin() (*Term, error) {
	var min *Term
	for _, s := range m.subs {
		t, err := s.GetMin()
		if err != nil {
			return nil, err
		}
		if t == nil {
			continue
		}
		if min == nil || t.CompareTo(min) < 0 {
			min = t
		}
	}
	return min, nil
}

// GetMax returns the largest term across all subs.
func (m *MultiTerms) GetMax() (*Term, error) {
	var max *Term
	for _, s := range m.subs {
		t, err := s.GetMax()
		if err != nil {
			return nil, err
		}
		if t == nil {
			continue
		}
		if max == nil || t.CompareTo(max) > 0 {
			max = t
		}
	}
	return max, nil
}

// HasFreqs returns true if every sub-Terms reports frequencies.
func (m *MultiTerms) HasFreqs() bool {
	for _, s := range m.subs {
		if !s.HasFreqs() {
			return false
		}
	}
	return true
}

// HasOffsets returns true if every sub-Terms reports offsets.
func (m *MultiTerms) HasOffsets() bool {
	for _, s := range m.subs {
		if !s.HasOffsets() {
			return false
		}
	}
	return true
}

// HasPositions returns true if every sub-Terms reports positions.
func (m *MultiTerms) HasPositions() bool {
	for _, s := range m.subs {
		if !s.HasPositions() {
			return false
		}
	}
	return true
}

// HasPayloads returns true if every sub-Terms reports payloads.
func (m *MultiTerms) HasPayloads() bool {
	for _, s := range m.subs {
		if !s.HasPayloads() {
			return false
		}
	}
	return true
}
