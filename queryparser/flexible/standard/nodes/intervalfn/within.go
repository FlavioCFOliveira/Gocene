// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Within is a node that represents Intervals#within.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Within.
type Within struct {
	positions int
	source    IntervalFunction
	reference IntervalFunction
}

// NewWithin creates a new Within node.
func NewWithin(source IntervalFunction, positions int, reference IntervalFunction) *Within {
	if source == nil {
		panic("source must not be nil")
	}
	if reference == nil {
		panic("reference must not be nil")
	}
	return &Within{positions: positions, source: source, reference: reference}
}

// ToIntervalSource converts the Within node into an IntervalsSource.
func (n *Within) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.Within(
		n.source.ToIntervalSource(field, analyzer),
		n.positions,
		n.reference.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:within(<source> <positions> <reference>)".
func (n *Within) String() string {
	return fmt.Sprintf("fn:within(%s %d %s)", n.source, n.positions, n.reference)
}
