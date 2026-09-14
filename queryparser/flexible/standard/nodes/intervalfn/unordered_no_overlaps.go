// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// UnorderedNoOverlaps is a node that represents Intervals#unorderedNoOverlaps.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.UnorderedNoOverlaps.
type UnorderedNoOverlaps struct {
	a IntervalFunction
	b IntervalFunction
}

// NewUnorderedNoOverlaps creates a new UnorderedNoOverlaps node.
func NewUnorderedNoOverlaps(a, b IntervalFunction) *UnorderedNoOverlaps {
	if a == nil {
		panic("a must not be nil")
	}
	if b == nil {
		panic("b must not be nil")
	}
	return &UnorderedNoOverlaps{a: a, b: b}
}

// ToIntervalSource converts the UnorderedNoOverlaps node into an IntervalsSource.
func (n *UnorderedNoOverlaps) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.UnorderedNoOverlaps(
		n.a.ToIntervalSource(field, analyzer),
		n.b.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:unorderedNoOverlaps(<a> <b>)".
func (n *UnorderedNoOverlaps) String() string {
	return fmt.Sprintf("fn:unorderedNoOverlaps(%s %s)", n.a, n.b)
}
