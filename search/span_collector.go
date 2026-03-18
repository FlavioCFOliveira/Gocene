// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SpanCollector collects span matches during search.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanCollector.
type SpanCollector struct {
	spans []*SpanMatch
}

// SpanMatch represents a single span match.
type SpanMatch struct {
	DocID int
	Start int
	End   int
}

// NewSpanCollector creates a new SpanCollector.
func NewSpanCollector() *SpanCollector {
	return &SpanCollector{
		spans: make([]*SpanMatch, 0),
	}
}

// Collect collects a span match.
func (c *SpanCollector) Collect(docID, start, end int) {
	c.spans = append(c.spans, &SpanMatch{
		DocID: docID,
		Start: start,
		End:   end,
	})
}

// Spans returns all collected span matches.
func (c *SpanCollector) Spans() []*SpanMatch {
	return c.spans
}

// Reset resets the collector.
func (c *SpanCollector) Reset() {
	c.spans = c.spans[:0]
}

// Size returns the number of collected spans.
func (c *SpanCollector) Size() int {
	return len(c.spans)
}
