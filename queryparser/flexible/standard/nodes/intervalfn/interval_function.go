package intervalfn

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// IntervalFunction is a representation of an interval function that can be converted to IntervalsSource.
type IntervalFunction interface {
	ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource
	String() string
}
