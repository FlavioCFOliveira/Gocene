// Package spans provides the Go port of org.apache.lucene.queries.spans.
//
// This package mirrors the Apache Lucene 10.4.0 spans query framework.
// The types defined here correspond to the Java classes in
// org.apache.lucene.queries.spans.
package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// NoMorePositions is the sentinel value returned by NextStartPosition when
// there are no more positions in the current document.
const NoMorePositions = int(^uint(0) >> 1) // math.MaxInt32 == Integer.MAX_VALUE in Java

// Spans is the interface for span iterators.
// It iterates through combinations of start/end positions per-doc.
// Each start/end position represents a range of term positions within the current document.
// These are enumerated in order, by increasing document number, within that by increasing
// start position and finally by increasing end position.
//
// Mirrors org.apache.lucene.queries.spans.Spans (abstract class).
type Spans interface {
	search.DocIdSetIterator

	// NextStartPosition returns the next start position for the current doc.
	// After the last start/end position at the current doc this returns NoMorePositions.
	NextStartPosition() (int, error)

	// StartPosition returns the start position in the current doc, or -1 when
	// NextStartPosition was not yet called on the current doc.
	StartPosition() int

	// EndPosition returns the end position for the current start position, or -1 when
	// NextStartPosition was not yet called on the current doc.
	EndPosition() int

	// Width returns the width of the match (used for sloppy freq).
	Width() int

	// Collect collects postings data from the leaves of the current Spans.
	Collect(collector SpanCollector) error

	// PositionsCost returns an estimation of the cost of using positions of this Spans
	// for any single document. Only valid when AsTwoPhaseIterator returns nil.
	PositionsCost() float32

	// AsTwoPhaseIterator returns an optional TwoPhaseIterator view of this Spans,
	// or nil if two-phase iteration is not supported.
	AsTwoPhaseIterator() *search.TwoPhaseIterator

	// DoStartCurrentDoc is called before the current doc's frequency is calculated.
	// Default is a no-op.
	DoStartCurrentDoc() error

	// DoCurrentSpans is called each time the scorer's Spans is advanced during
	// frequency calculation. Default is a no-op.
	DoCurrentSpans() error
}

// SpanCollector is the interface defining collection of postings information
// from the leaves of a Spans.
//
// Mirrors org.apache.lucene.queries.spans.SpanCollector.
type SpanCollector interface {
	// CollectLeaf collects information from postings.
	CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error

	// Reset indicates that the driving Spans has moved to a new position.
	Reset()
}

// BaseSpans provides default no-op implementations of DoStartCurrentDoc and DoCurrentSpans.
// Embed this in concrete Spans implementations.
type BaseSpans struct{}

// DoStartCurrentDoc is a no-op.
func (*BaseSpans) DoStartCurrentDoc() error { return nil }

// DoCurrentSpans is a no-op.
func (*BaseSpans) DoCurrentSpans() error { return nil }

// Stubs retained for backward-compat while full ports land progressively.

// FieldMaskingSpanQuery mirrors org.apache.lucene.queries.spans.FieldMaskingSpanQuery.
type FieldMaskingSpanQuery struct{}

// NewFieldMaskingSpanQuery builds a FieldMaskingSpanQuery.
func NewFieldMaskingSpanQuery() *FieldMaskingSpanQuery { return &FieldMaskingSpanQuery{} }

// FilterSpans mirrors org.apache.lucene.queries.spans.FilterSpans.
type FilterSpans struct{}

// NewFilterSpans builds a FilterSpans.
func NewFilterSpans() *FilterSpans { return &FilterSpans{} }

// SpanMultiTermQueryWrapper mirrors org.apache.lucene.queries.spans.SpanMultiTermQueryWrapper.
type SpanMultiTermQueryWrapper struct{}

// NewSpanMultiTermQueryWrapper builds a SpanMultiTermQueryWrapper.
func NewSpanMultiTermQueryWrapper() *SpanMultiTermQueryWrapper { return &SpanMultiTermQueryWrapper{} }

// SpanNotQuery mirrors org.apache.lucene.queries.spans.SpanNotQuery.
type SpanNotQuery struct{}

// NewSpanNotQuery builds a SpanNotQuery.
func NewSpanNotQuery() *SpanNotQuery { return &SpanNotQuery{} }

// SpanOrQuery mirrors org.apache.lucene.queries.spans.SpanOrQuery.
type SpanOrQuery struct{}

// NewSpanOrQuery builds a SpanOrQuery.
func NewSpanOrQuery() *SpanOrQuery { return &SpanOrQuery{} }

// SpanWithinQuery mirrors org.apache.lucene.queries.spans.SpanWithinQuery.
type SpanWithinQuery struct{}

// NewSpanWithinQuery builds a SpanWithinQuery.
func NewSpanWithinQuery() *SpanWithinQuery { return &SpanWithinQuery{} }

// SpanFirstQuery mirrors org.apache.lucene.queries.spans.SpanFirstQuery.
type SpanFirstQuery struct{}

// NewSpanFirstQuery builds a SpanFirstQuery.
func NewSpanFirstQuery() *SpanFirstQuery { return &SpanFirstQuery{} }

// SpanPositionCheckQuery mirrors org.apache.lucene.queries.spans.SpanPositionCheckQuery.
type SpanPositionCheckQuery struct{}

// NewSpanPositionCheckQuery builds a SpanPositionCheckQuery.
func NewSpanPositionCheckQuery() *SpanPositionCheckQuery { return &SpanPositionCheckQuery{} }

// SpanPositionRangeQuery mirrors org.apache.lucene.queries.spans.SpanPositionRangeQuery.
type SpanPositionRangeQuery struct{}

// NewSpanPositionRangeQuery builds a SpanPositionRangeQuery.
func NewSpanPositionRangeQuery() *SpanPositionRangeQuery { return &SpanPositionRangeQuery{} }
