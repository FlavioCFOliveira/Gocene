// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/TermIntervalsSource.java

package intervals

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const (
	// termPositionsSeekOpsPerDoc is an estimate of operations per document for seek/refill.
	termPositionsSeekOpsPerDoc = 256
	// termOpsPerPos is the number of operations per position call.
	termOpsPerPos = 7
)

// TermIntervalsSource is an IntervalsSource exposing intervals for a single term.
//
// Mirrors org.apache.lucene.queries.intervals.TermIntervalsSource.
type TermIntervalsSource struct {
	term []byte // raw bytes of the term
}

// NewTermIntervalsSource creates a TermIntervalsSource for the given term bytes.
func NewTermIntervalsSource(term []byte) *TermIntervalsSource {
	cp := make([]byte, len(term))
	copy(cp, term)
	return &TermIntervalsSource{term: cp}
}

// TermIntervalsSourceFromString creates a TermIntervalsSource for the given term string.
func TermIntervalsSourceFromString(term string) *TermIntervalsSource {
	return &TermIntervalsSource{term: []byte(term)}
}

// positionsFlag is the PostingsEnum flag for position data.
// Mirrors PostingsEnum.POSITIONS in Lucene.
const positionsFlag = 2

// offsetsFlag is the PostingsEnum flag for offset data (includes positions).
// Mirrors PostingsEnum.OFFSETS in Lucene.
const offsetsFlag = 6

// payloadsFlag is the PostingsEnum flag for payload data (includes positions).
// Mirrors PostingsEnum.PAYLOADS in Lucene.
const payloadsFlag = 18

// allPostingsFlag includes positions, offsets, and payloads.
const allPostingsFlag = 22

// Intervals creates an IntervalIterator for the given field and context.
func (s *TermIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
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
	return termIntervals(s.term, te)
}

// termIntervals creates an IntervalIterator from a positioned TermsEnum.
func termIntervals(term []byte, te index.TermsEnum) (IntervalIterator, error) {
	pe, err := te.Postings(positionsFlag)
	if err != nil {
		return nil, err
	}
	cost, err := termPositionsCost(te)
	if err != nil {
		return nil, err
	}
	return &termIntervalIterator{pe: pe, term: term, matchCostVal: cost}, nil
}

// termIntervalIterator iterates over position-level intervals for a single term.
type termIntervalIterator struct {
	pe           index.PostingsEnum
	term         []byte
	matchCostVal float32
	pos          int
	upto         int
}

func (t *termIntervalIterator) DocID() int        { return t.pe.DocID() }
func (t *termIntervalIterator) DocIDRunEnd() int   { return t.DocID() + 1 }
func (t *termIntervalIterator) Cost() int64       { return t.pe.Cost() }
func (t *termIntervalIterator) MatchCost() float32 { return t.matchCostVal }
func (t *termIntervalIterator) Start() int        { return t.pos }
func (t *termIntervalIterator) End() int          { return t.pos }
func (t *termIntervalIterator) Gaps() int         { return 0 }
func (t *termIntervalIterator) Width() int        { return 1 }

func (t *termIntervalIterator) NextDoc() (int, error) {
	doc, err := t.pe.NextDoc()
	if err != nil {
		return 0, err
	}
	if err := t.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (t *termIntervalIterator) Advance(target int) (int, error) {
	doc, err := t.pe.Advance(target)
	if err != nil {
		return 0, err
	}
	if err := t.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (t *termIntervalIterator) NextInterval() (int, error) {
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
	return pos, nil
}

func (t *termIntervalIterator) reset() error {
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

var _ search.DocIdSetIterator = (*termIntervalIterator)(nil)

// Matches returns an IntervalMatchesIterator for the given field, context and doc.
func (s *TermIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
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
	return termMatches(te, doc, field)
}

// termMatches creates an IntervalMatchesIterator from a positioned TermsEnum for a given doc.
func termMatches(te index.TermsEnum, doc int, field string) (IntervalMatchesIterator, error) {
	pe, err := te.Postings(offsetsFlag)
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
	query := search.NewTermQuery(index.NewTerm(field, te.Term().Text()))
	return &termMatchesIterator{pe: pe, upto: freq, pos: -1, query: query}, nil
}

// termMatchesIterator is the matches iterator for a single term.
type termMatchesIterator struct {
	pe    index.PostingsEnum
	upto  int
	pos   int
	query search.Query
}

func (m *termMatchesIterator) Gaps() int  { return 0 }
func (m *termMatchesIterator) Width() int { return 1 }

func (m *termMatchesIterator) Next() (bool, error) {
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
	return true, nil
}

func (m *termMatchesIterator) StartPosition() int { return m.pos }
func (m *termMatchesIterator) EndPosition() int   { return m.pos }
func (m *termMatchesIterator) StartOffset() (int, error) {
	return m.pe.StartOffset()
}
func (m *termMatchesIterator) EndOffset() (int, error) {
	return m.pe.EndOffset()
}
func (m *termMatchesIterator) GetSubMatches() (search.MatchesIterator, error) { return nil, nil }
func (m *termMatchesIterator) GetQuery() search.Query                          { return m.query }

// Visit visits with the given QueryVisitor.
func (s *TermIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	visitor.ConsumeTerms(NewIntervalQuery(field, s), index.NewTerm(field, string(s.term)))
}

// MinExtent returns 1 (a single term spans exactly one position).
func (s *TermIntervalsSource) MinExtent() int { return 1 }

// PullUpDisjunctions returns a singleton list.
func (s *TermIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports structural equality.
func (s *TermIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*TermIntervalsSource)
	if !ok {
		return false
	}
	if len(s.term) != len(o.term) {
		return false
	}
	for i, b := range s.term {
		if b != o.term[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code based on the term bytes.
func (s *TermIntervalsSource) HashCode() int {
	return hashBytes(s.term)
}

// String returns the term as a UTF-8 string.
func (s *TermIntervalsSource) String() string { return string(s.term) }

// termPositionsCost estimates the per-document cost of processing term positions.
func termPositionsCost(te index.TermsEnum) (float32, error) {
	docFreq, err := te.DocFreq()
	if err != nil {
		return 0, err
	}
	if docFreq == 0 {
		return float32(termPositionsSeekOpsPerDoc), nil
	}
	totalTermFreq, err := te.TotalTermFreq()
	if err != nil {
		return 0, err
	}
	expOccurrences := float32(totalTermFreq) / float32(docFreq)
	return float32(termPositionsSeekOpsPerDoc) + expOccurrences*float32(termOpsPerPos), nil
}

// hashBytes computes a hash over a byte slice.
func hashBytes(b []byte) int {
	h := 31
	for _, c := range b {
		h = h*31 + int(c)
	}
	return h
}
