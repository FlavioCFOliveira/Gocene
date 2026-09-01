package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Before is a node that represents intervals.Before.
type Before struct {
	source    IntervalFunction
	reference IntervalFunction
}

// NewBefore creates a new Before node.
func NewBefore(source, reference IntervalFunction) *Before {
	if source == nil || reference == nil {
		panic("source and reference must not be nil")
	}
	return &Before{
		source:    source,
		reference: reference,
	}
}

// ToIntervalSource converts the Before node to an IntervalsSource.
func (b *Before) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.Before(
		b.source.ToIntervalSource(field, analyzer),
		b.reference.ToIntervalSource(field, analyzer),
	)
}

// String returns the string representation of the Before node.
func (b *Before) String() string {
	return fmt.Sprintf("fn:before(%s %s)", b.source, b.reference)
}
