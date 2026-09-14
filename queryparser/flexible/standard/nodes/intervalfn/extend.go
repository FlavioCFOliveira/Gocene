// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Extend is a node that represents Intervals#extend.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Extend.
type Extend struct {
	before int
	after  int
	source IntervalFunction
}

// NewExtend creates a new Extend node.
func NewExtend(source IntervalFunction, before, after int) *Extend {
	if source == nil {
		panic("source must not be nil")
	}
	return &Extend{source: source, before: before, after: after}
}

// ToIntervalSource converts the Extend node into an IntervalsSource.
func (n *Extend) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.Extend(n.source.ToIntervalSource(field, analyzer), n.before, n.after)
}

// String returns "fn:extend(<source> <before> <after>)".
func (n *Extend) String() string {
	return fmt.Sprintf("fn:extend(%s %d %d)", n.source, n.before, n.after)
}
