// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import "github.com/FlavioCFOliveira/Gocene/search"

// WeightedSpanTermExtractor walks a Query tree and produces a map of
// WeightedSpanTerm keyed by term text. Mirrors
// org.apache.lucene.search.highlight.WeightedSpanTermExtractor.
//
// The position-sensitive flag is set whenever a Span* query is encountered,
// matching the Lucene behaviour: SpanTermQuery / SpanNearQuery etc produce
// position-sensitive terms while plain TermQuery / PhraseQuery do not.
type WeightedSpanTermExtractor struct {
	fieldName       string
	expandMultiTerm bool
}

// NewWeightedSpanTermExtractor builds an extractor for the supplied field
// (empty = any field).
func NewWeightedSpanTermExtractor(fieldName string) *WeightedSpanTermExtractor {
	return &WeightedSpanTermExtractor{fieldName: fieldName, expandMultiTerm: true}
}

// SetExpandMultiTerm toggles wildcard expansion (no-op for the Go port that
// only operates on the lowered TermQuery shape).
func (e *WeightedSpanTermExtractor) SetExpandMultiTerm(expand bool) { e.expandMultiTerm = expand }

// Extract returns the WeightedSpanTerm map for query.
func (e *WeightedSpanTermExtractor) Extract(query search.Query) map[string]*WeightedSpanTerm {
	terms := make(map[string]*WeightedSpanTerm)
	e.extract(query, 1.0, false, terms)
	return terms
}

func (e *WeightedSpanTermExtractor) extract(query search.Query, weight float32, positionSensitive bool, out map[string]*WeightedSpanTerm) {
	switch q := query.(type) {
	case *search.TermQuery:
		term := q.Term()
		if e.fieldName != "" && term.Field != e.fieldName {
			return
		}
		e.add(term.Text(), weight, positionSensitive, out)
	case *search.PhraseQuery:
		for _, t := range q.Terms() {
			if e.fieldName != "" && t.Field != e.fieldName {
				continue
			}
			e.add(t.Text(), weight, true, out)
		}
	case *search.BooleanQuery:
		for _, c := range q.Clauses() {
			if c.Occur == search.MUST_NOT {
				continue
			}
			e.extract(c.Query, weight, positionSensitive, out)
		}
	case *search.BoostQuery:
		e.extract(q.Query(), weight*q.Boost(), positionSensitive, out)
	case *search.SpanTermQuery:
		t := q.Term()
		if e.fieldName != "" && t.Field != e.fieldName {
			return
		}
		e.add(t.Text(), weight, true, out)
	case *search.SpanNearQuery:
		for _, c := range q.Clauses() {
			e.extract(c, weight, true, out)
		}
	case *search.SpanOrQuery:
		for _, c := range q.Clauses() {
			e.extract(c, weight, true, out)
		}
	}
}

func (e *WeightedSpanTermExtractor) add(text string, weight float32, positionSensitive bool, out map[string]*WeightedSpanTerm) {
	if existing, ok := out[text]; ok {
		if weight > existing.GetWeight() {
			existing.SetWeight(weight)
		}
		if positionSensitive {
			existing.PositionSensitive = true
		}
		return
	}
	out[text] = NewWeightedSpanTerm(weight, text, positionSensitive)
}
