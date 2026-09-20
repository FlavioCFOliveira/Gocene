// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Ordered is a node that represents Intervals#ordered.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Ordered.
type Ordered struct {
	sources []IntervalFunction
}

// NewOrdered creates a new Ordered node.
func NewOrdered(sources []IntervalFunction) *Ordered {
	if sources == nil {
		panic("sources must not be nil")
	}
	return &Ordered{sources: sources}
}

// ToIntervalSource converts the Ordered node into an IntervalsSource.
func (n *Ordered) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	sources := make([]intervals.IntervalsSource, len(n.sources))
	for i, src := range n.sources {
		sources[i] = src.ToIntervalSource(field, analyzer)
	}
	return intervals.Ordered(sources...)
}

// String returns "fn:ordered(<s1> <s2> ...)".
func (n *Ordered) String() string {
	return fmt.Sprintf("fn:ordered(%s)", joinFunctions(n.sources))
}
