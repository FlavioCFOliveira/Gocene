// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// NotContaining is a node that represents Intervals#notContaining.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotContaining.
type NotContaining struct {
	minuend    IntervalFunction
	subtrahend IntervalFunction
}

// NewNotContaining creates a new NotContaining node.
func NewNotContaining(minuend, subtrahend IntervalFunction) *NotContaining {
	if minuend == nil {
		panic("minuend must not be nil")
	}
	if subtrahend == nil {
		panic("subtrahend must not be nil")
	}
	return &NotContaining{minuend: minuend, subtrahend: subtrahend}
}

// ToIntervalSource converts the NotContaining node into an IntervalsSource.
func (n *NotContaining) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.NotContaining(
		n.minuend.ToIntervalSource(field, analyzer),
		n.subtrahend.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:notContaining(<minuend> <subtrahend>)".
func (n *NotContaining) String() string {
	return fmt.Sprintf("fn:notContaining(%s %s)", n.minuend, n.subtrahend)
}
