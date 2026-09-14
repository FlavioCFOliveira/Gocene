// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Containing is a node that represents Intervals#containing.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Containing.
type Containing struct {
	big   IntervalFunction
	small IntervalFunction
}

// NewContaining creates a new Containing node.
func NewContaining(big, small IntervalFunction) *Containing {
	if big == nil {
		panic("big must not be nil")
	}
	if small == nil {
		panic("small must not be nil")
	}
	return &Containing{big: big, small: small}
}

// ToIntervalSource converts the Containing node into an IntervalsSource.
func (n *Containing) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.Containing(
		n.big.ToIntervalSource(field, analyzer),
		n.small.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:containing(<big> <small>)".
func (n *Containing) String() string {
	return fmt.Sprintf("fn:containing(%s %s)", n.big, n.small)
}
