// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// NotWithin is a node that represents Intervals#notWithin.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotWithin.
type NotWithin struct {
	positions  int
	minuend    IntervalFunction
	subtrahend IntervalFunction
}

// NewNotWithin creates a new NotWithin node.
func NewNotWithin(minuend IntervalFunction, positions int, subtrahend IntervalFunction) *NotWithin {
	if minuend == nil {
		panic("minuend must not be nil")
	}
	if subtrahend == nil {
		panic("subtrahend must not be nil")
	}
	return &NotWithin{positions: positions, minuend: minuend, subtrahend: subtrahend}
}

// ToIntervalSource converts the NotWithin node into an IntervalsSource.
func (n *NotWithin) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.NotWithin(
		n.minuend.ToIntervalSource(field, analyzer),
		n.positions,
		n.subtrahend.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:notWithin(<minuend> <positions> <subtrahend>)".
func (n *NotWithin) String() string {
	return fmt.Sprintf("fn:notWithin(%s %d %s)", n.minuend, n.positions, n.subtrahend)
}
