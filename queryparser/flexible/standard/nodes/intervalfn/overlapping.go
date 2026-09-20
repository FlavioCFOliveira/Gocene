// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Overlapping is a node that represents Intervals#overlapping.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Overlapping.
type Overlapping struct {
	source    IntervalFunction
	reference IntervalFunction
}

// NewOverlapping creates a new Overlapping node.
func NewOverlapping(source, reference IntervalFunction) *Overlapping {
	if source == nil {
		panic("source must not be nil")
	}
	if reference == nil {
		panic("reference must not be nil")
	}
	return &Overlapping{source: source, reference: reference}
}

// ToIntervalSource converts the Overlapping node into an IntervalsSource.
func (n *Overlapping) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.Overlapping(
		n.source.ToIntervalSource(field, analyzer),
		n.reference.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:overlapping(<source> <reference>)".
func (n *Overlapping) String() string {
	return fmt.Sprintf("fn:overlapping(%s %s)", n.source, n.reference)
}
