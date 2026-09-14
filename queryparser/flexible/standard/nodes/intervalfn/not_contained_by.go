// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// NotContainedBy is a node that represents Intervals#notContainedBy.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotContainedBy.
type NotContainedBy struct {
	small IntervalFunction
	big   IntervalFunction
}

// NewNotContainedBy creates a new NotContainedBy node.
func NewNotContainedBy(small, big IntervalFunction) *NotContainedBy {
	if small == nil {
		panic("small must not be nil")
	}
	if big == nil {
		panic("big must not be nil")
	}
	return &NotContainedBy{small: small, big: big}
}

// ToIntervalSource converts the NotContainedBy node into an IntervalsSource.
func (n *NotContainedBy) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.NotContainedBy(
		n.small.ToIntervalSource(field, analyzer),
		n.big.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:notContainedBy(<small> <big>)".
func (n *NotContainedBy) String() string {
	return fmt.Sprintf("fn:notContainedBy(%s %s)", n.small, n.big)
}
