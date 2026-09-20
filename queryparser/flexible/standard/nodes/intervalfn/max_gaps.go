// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// MaxGaps is a node that represents Intervals#maxgaps.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.MaxGaps.
type MaxGaps struct {
	maxGaps int
	source  IntervalFunction
}

// NewMaxGaps creates a new MaxGaps node.
func NewMaxGaps(maxGaps int, source IntervalFunction) *MaxGaps {
	if source == nil {
		panic("source must not be nil")
	}
	return &MaxGaps{maxGaps: maxGaps, source: source}
}

// ToIntervalSource converts the MaxGaps node into an IntervalsSource.
func (n *MaxGaps) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.MaxGaps(n.maxGaps, n.source.ToIntervalSource(field, analyzer))
}

// String returns "fn:maxgaps(<maxGaps> <source>)".
func (n *MaxGaps) String() string {
	return fmt.Sprintf("fn:maxgaps(%d %s)", n.maxGaps, n.source)
}
