// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Unordered is a node that represents Intervals#unordered.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Unordered.
type Unordered struct {
	sources []IntervalFunction
}

// NewUnordered creates a new Unordered node.
func NewUnordered(sources []IntervalFunction) *Unordered {
	if sources == nil {
		panic("sources must not be nil")
	}
	return &Unordered{sources: sources}
}

// ToIntervalSource converts the Unordered node into an IntervalsSource.
func (n *Unordered) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	sources := make([]intervals.IntervalsSource, len(n.sources))
	for i, src := range n.sources {
		sources[i] = src.ToIntervalSource(field, analyzer)
	}
	return intervals.Unordered(sources...)
}

// String returns "fn:unordered(<s1> <s2> ...)".
func (n *Unordered) String() string {
	return fmt.Sprintf("fn:unordered(%s)", joinFunctions(n.sources))
}
