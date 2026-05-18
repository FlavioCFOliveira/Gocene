// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

// PositionSpan is an inclusive [Start, End] interval over term positions.
// Mirrors org.apache.lucene.search.highlight.PositionSpan.
type PositionSpan struct {
	Start int
	End   int
}

// NewPositionSpan builds a PositionSpan.
func NewPositionSpan(start, end int) PositionSpan {
	if start > end {
		start, end = end, start
	}
	return PositionSpan{Start: start, End: end}
}

// Contains reports whether position lies inside the span.
func (p PositionSpan) Contains(position int) bool {
	return position >= p.Start && position <= p.End
}
