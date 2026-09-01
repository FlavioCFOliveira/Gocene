package intervalfn

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// AtLeast is a node that represents intervals.AtLeast.
type AtLeast struct {
	minShouldMatch int
	sources        []IntervalFunction
}

// NewAtLeast creates a new AtLeast interval function.
func NewAtLeast(minShouldMatch int, sources []IntervalFunction) *AtLeast {
	return &AtLeast{
		minShouldMatch: minShouldMatch,
		sources:        sources,
	}
}

// ToIntervalSource converts the AtLeast node into an IntervalsSource.
func (a *AtLeast) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	sources := make([]intervals.IntervalsSource, len(a.sources))
	for i, src := range a.sources {
		sources[i] = src.ToIntervalSource(field, analyzer)
	}
	return intervals.AtLeast(a.minShouldMatch, sources...)
}

// String returns the string representation of the AtLeast node.
func (a *AtLeast) String() string {
	var sb strings.Builder
	for i, src := range a.sources {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(src.String())
	}
	return fmt.Sprintf("fn:atLeast(%d %s)", a.minShouldMatch, sb.String())
}
