// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/PayloadFilteredTermIntervalsSource.java

package intervals

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// PayloadFilteredTermIntervalsSource is an IntervalsSource for a term that filters
// positions by payload predicate.
//
// Mirrors org.apache.lucene.queries.intervals.PayloadFilteredTermIntervalsSource.
type PayloadFilteredTermIntervalsSource struct {
	term   []byte
	filter func([]byte) bool
}

// NewPayloadFilteredTermIntervalsSource creates a PayloadFilteredTermIntervalsSource.
func NewPayloadFilteredTermIntervalsSource(term []byte, filter func([]byte) bool) *PayloadFilteredTermIntervalsSource {
	cp := make([]byte, len(term))
	copy(cp, term)
	return &PayloadFilteredTermIntervalsSource{term: cp, filter: filter}
}

// Intervals creates an IntervalIterator filtered by the payload predicate.
func (s *PayloadFilteredTermIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	terms, err := ctx.LeafReader().Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	if !terms.HasPositions() {
		return nil, fmt.Errorf("cannot create an IntervalIterator over field %s because it has no indexed positions", field)
	}
	if !terms.HasPayloads() {
		return nil, fmt.Errorf("cannot create a payload-filtered iterator over field %s because it has no indexed payloads", field)
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(index.NewTerm(field, string(s.term)))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	pe, err := te.Postings(payloadsFlag)
	if err != nil {
		return nil, err
	}
	cost, err := termPositionsCost(te)
	if err != nil {
		return nil, err
	}
	filter := s.filter
	return &payloadFilteredIntervalIterator{pe: pe, term: s.term, matchCostVal: cost, filter: filter}, nil
}

// payloadFilteredIntervalIterator skips positions where the payload fails the filter.
type payloadFilteredIntervalIterator struct {
	pe           index.PostingsEnum
	term         []byte
	matchCostVal float32
	filter       func([]byte) bool
	pos          int
	upto         int
}

func (t *payloadFilteredIntervalIterator) DocID() int        { return t.pe.DocID() }
func (t *payloadFilteredIntervalIterator) DocIDRunEnd() int   { return t.DocID() + 1 }
func (t *payloadFilteredIntervalIterator) Cost() int64       { return t.pe.Cost() }
func (t *payloadFilteredIntervalIterator) MatchCost() float32 { return t.matchCostVal }
func (t *payloadFilteredIntervalIterator) Start() int        { return t.pos }
func (t *payloadFilteredIntervalIterator) End() int          { return t.pos }
func (t *payloadFilteredIntervalIterator) Gaps() int         { return 0 }
func (t *payloadFilteredIntervalIterator) Width() int        { return 1 }

func (t *payloadFilteredIntervalIterator) NextDoc() (int, error) {
	doc, err := t.pe.NextDoc()
	if err != nil {
		return 0, err
	}
	if err := t.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (t *payloadFilteredIntervalIterator) Advance(target int) (int, error) {
	doc, err := t.pe.Advance(target)
	if err != nil {
		return 0, err
	}
	if err := t.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (t *payloadFilteredIntervalIterator) NextInterval() (int, error) {
	for {
		if t.upto <= 0 {
			t.pos = NoMoreIntervals
			return NoMoreIntervals, nil
		}
		t.upto--
		pos, err := t.pe.NextPosition()
		if err != nil {
			return 0, err
		}
		t.pos = pos
		payload, err := t.pe.GetPayload()
		if err != nil {
			return 0, err
		}
		if t.filter(payload) {
			return t.pos, nil
		}
	}
}

func (t *payloadFilteredIntervalIterator) reset() error {
	if t.pe.DocID() == search.NO_MORE_DOCS {
		t.upto = -1
		t.pos = NoMoreIntervals
	} else {
		freq, err := t.pe.Freq()
		if err != nil {
			return err
		}
		t.upto = freq
		t.pos = -1
	}
	return nil
}

var _ search.DocIdSetIterator = (*payloadFilteredIntervalIterator)(nil)

// Matches creates an IntervalMatchesIterator filtered by the payload predicate.
func (s *PayloadFilteredTermIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	terms, err := ctx.LeafReader().Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	if !terms.HasPositions() {
		return nil, fmt.Errorf("cannot create an IntervalIterator over field %s because it has no indexed positions", field)
	}
	if !terms.HasPayloads() {
		return nil, fmt.Errorf("cannot create a payload-filtered iterator over field %s because it has no indexed payloads", field)
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(index.NewTerm(field, string(s.term)))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	pe, err := te.Postings(allPostingsFlag)
	if err != nil {
		return nil, err
	}
	advanced, err := pe.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return nil, nil
	}
	freq, err := pe.Freq()
	if err != nil {
		return nil, err
	}
	filter := s.filter
	return &payloadFilteredMatchesIterator{pe: pe, upto: freq, pos: -1, filter: filter}, nil
}

// payloadFilteredMatchesIterator skips matches where the payload fails the filter.
type payloadFilteredMatchesIterator struct {
	pe     index.PostingsEnum
	upto   int
	pos    int
	filter func([]byte) bool
}

func (m *payloadFilteredMatchesIterator) Gaps() int  { return 0 }
func (m *payloadFilteredMatchesIterator) Width() int { return 1 }

func (m *payloadFilteredMatchesIterator) Next() (bool, error) {
	for {
		if m.upto <= 0 {
			m.pos = NoMoreIntervals
			return false, nil
		}
		m.upto--
		pos, err := m.pe.NextPosition()
		if err != nil {
			return false, err
		}
		m.pos = pos
		payload, err := m.pe.GetPayload()
		if err != nil {
			return false, err
		}
		if m.filter(payload) {
			return true, nil
		}
	}
}

func (m *payloadFilteredMatchesIterator) StartPosition() int { return m.pos }
func (m *payloadFilteredMatchesIterator) EndPosition() int   { return m.pos }
func (m *payloadFilteredMatchesIterator) StartOffset() (int, error) {
	return m.pe.StartOffset()
}
func (m *payloadFilteredMatchesIterator) EndOffset() (int, error) {
	return m.pe.EndOffset()
}
func (m *payloadFilteredMatchesIterator) GetSubMatches() (search.MatchesIterator, error) { return nil, nil }
func (m *payloadFilteredMatchesIterator) GetQuery() search.Query                          { panic("unsupported") }

// Visit visits with the query visitor.
func (s *PayloadFilteredTermIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	visitor.ConsumeTerms(NewIntervalQuery(field, s), index.NewTerm(field, string(s.term)))
}

// MinExtent returns 1.
func (s *PayloadFilteredTermIntervalsSource) MinExtent() int { return 1 }

// PullUpDisjunctions returns a singleton list.
func (s *PayloadFilteredTermIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports equality based on term bytes only (filter is not comparable).
func (s *PayloadFilteredTermIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*PayloadFilteredTermIntervalsSource)
	if !ok || len(s.term) != len(o.term) {
		return false
	}
	for i, b := range s.term {
		if b != o.term[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code based on term bytes.
func (s *PayloadFilteredTermIntervalsSource) HashCode() int {
	return hashBytes(s.term)
}

// String returns a string representation.
func (s *PayloadFilteredTermIntervalsSource) String() string {
	return "PAYLOAD_FILTERED(" + string(s.term) + ")"
}
