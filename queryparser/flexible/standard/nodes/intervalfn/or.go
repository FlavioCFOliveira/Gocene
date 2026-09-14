// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Or is a node that represents Intervals#or.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Or.
type Or struct {
	sources []IntervalFunction
}

// NewOr creates a new Or node.
func NewOr(sources []IntervalFunction) *Or {
	if sources == nil {
		panic("sources must not be nil")
	}
	return &Or{sources: sources}
}

// ToIntervalSource converts the Or node into an IntervalsSource.
func (n *Or) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	sources := make([]intervals.IntervalsSource, len(n.sources))
	for i, src := range n.sources {
		sources[i] = src.ToIntervalSource(field, analyzer)
	}
	return intervals.Or(sources...)
}

// String returns "fn:or(<s1> <s2> ...)".
func (n *Or) String() string {
	return fmt.Sprintf("fn:or(%s)", joinFunctions(n.sources))
}
