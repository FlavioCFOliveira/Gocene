// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// ContainedBy is a node that represents Intervals#containedBy.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.ContainedBy.
type ContainedBy struct {
	small IntervalFunction
	big   IntervalFunction
}

// NewContainedBy creates a new ContainedBy node.
func NewContainedBy(small, big IntervalFunction) *ContainedBy {
	if small == nil {
		panic("small must not be nil")
	}
	if big == nil {
		panic("big must not be nil")
	}
	return &ContainedBy{small: small, big: big}
}

// ToIntervalSource converts the ContainedBy node into an IntervalsSource.
func (n *ContainedBy) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.ContainedBy(
		n.small.ToIntervalSource(field, analyzer),
		n.big.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:containedBy(<small> <big>)".
func (n *ContainedBy) String() string {
	return fmt.Sprintf("fn:containedBy(%s %s)", n.small, n.big)
}
