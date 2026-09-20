// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// MaxWidth is a node that represents Intervals#maxwidth.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.MaxWidth.
type MaxWidth struct {
	width  int
	source IntervalFunction
}

// NewMaxWidth creates a new MaxWidth node.
func NewMaxWidth(width int, source IntervalFunction) *MaxWidth {
	if source == nil {
		panic("source must not be nil")
	}
	return &MaxWidth{width: width, source: source}
}

// ToIntervalSource converts the MaxWidth node into an IntervalsSource.
func (n *MaxWidth) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.MaxWidth(n.width, n.source.ToIntervalSource(field, analyzer))
}

// String returns "fn:maxwidth(<width> <source>)".
func (n *MaxWidth) String() string {
	return fmt.Sprintf("fn:maxwidth(%d %s)", n.width, n.source)
}
