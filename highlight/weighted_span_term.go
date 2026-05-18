// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

// WeightedSpanTerm extends WeightedTerm with the list of position spans where
// the term matches and a flag indicating whether the span was discovered
// inside a SpanQuery (positionSensitive). Mirrors
// org.apache.lucene.search.highlight.WeightedSpanTerm.
type WeightedSpanTerm struct {
	*WeightedTerm
	PositionSensitive bool
	Spans             []PositionSpan
}

// NewWeightedSpanTerm builds a WeightedSpanTerm.
func NewWeightedSpanTerm(weight float32, term string, positionSensitive bool) *WeightedSpanTerm {
	return &WeightedSpanTerm{
		WeightedTerm:      NewWeightedTerm(weight, term),
		PositionSensitive: positionSensitive,
	}
}

// AddPositionSpan registers a match span.
func (w *WeightedSpanTerm) AddPositionSpan(start, end int) {
	w.Spans = append(w.Spans, PositionSpan{Start: start, End: end})
}

// CheckPosition reports whether position falls inside any registered span.
// When the term is not position-sensitive every position matches.
func (w *WeightedSpanTerm) CheckPosition(position int) bool {
	if !w.PositionSensitive {
		return true
	}
	for _, s := range w.Spans {
		if position >= s.Start && position <= s.End {
			return true
		}
	}
	return false
}
