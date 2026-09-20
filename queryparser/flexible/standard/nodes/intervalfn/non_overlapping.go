// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// NonOverlapping is a node that represents Intervals#nonOverlapping.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NonOverlapping.
type NonOverlapping struct {
	minuend    IntervalFunction
	subtrahend IntervalFunction
}

// NewNonOverlapping creates a new NonOverlapping node.
func NewNonOverlapping(minuend, subtrahend IntervalFunction) *NonOverlapping {
	if minuend == nil {
		panic("minuend must not be nil")
	}
	if subtrahend == nil {
		panic("subtrahend must not be nil")
	}
	return &NonOverlapping{minuend: minuend, subtrahend: subtrahend}
}

// ToIntervalSource converts the NonOverlapping node into an IntervalsSource.
func (n *NonOverlapping) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.NonOverlapping(
		n.minuend.ToIntervalSource(field, analyzer),
		n.subtrahend.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:nonOverlapping(<minuend> <subtrahend>)".
func (n *NonOverlapping) String() string {
	return fmt.Sprintf("fn:nonOverlapping(%s %s)", n.minuend, n.subtrahend)
}
