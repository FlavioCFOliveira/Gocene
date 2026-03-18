// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SpanScorer scores span queries.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanScorer.
type SpanScorer struct {
	spans *Spans
	score float32
	doc   int
	freq  float32
}

// NewSpanScorer creates a new SpanScorer.
func NewSpanScorer(spans *Spans, score float32) *SpanScorer {
	return &SpanScorer{
		spans: spans,
		score: score,
		doc:   -1,
	}
}

// Score returns the score for the current document.
func (s *SpanScorer) Score() float32 {
	return s.score * s.freq
}

// DocID returns the current document ID.
func (s *SpanScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
func (s *SpanScorer) NextDoc() (int, error) {
	doc, err := s.spans.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	if doc != NO_MORE_DOCS {
		s.freq = float32(s.spans.Freq())
	}
	return doc, nil
}

// Advance advances to the specified document.
func (s *SpanScorer) Advance(target int) (int, error) {
	doc, err := s.spans.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	if doc != NO_MORE_DOCS {
		s.freq = float32(s.spans.Freq())
	}
	return doc, nil
}

// Spans returns the underlying Spans.
func (s *SpanScorer) Spans() *Spans {
	return s.spans
}

// Cost returns the estimated cost of iterating through all documents.
func (s *SpanScorer) Cost() int64 {
	return s.spans.Cost()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *SpanScorer) DocIDRunEnd() int {
	return s.spans.DocIDRunEnd()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *SpanScorer) GetMaxScore(upTo int) float32 {
	return s.score * s.freq
}

// Ensure SpanScorer implements Scorer
var _ Scorer = (*SpanScorer)(nil)
