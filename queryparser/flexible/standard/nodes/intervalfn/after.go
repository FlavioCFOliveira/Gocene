// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// After is a node that represents Intervals#after.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.After.
type After struct {
	source    IntervalFunction
	reference IntervalFunction
}

// NewAfter creates a new After node.
func NewAfter(source, reference IntervalFunction) *After {
	if source == nil {
		panic("source must not be nil")
	}
	if reference == nil {
		panic("reference must not be nil")
	}
	return &After{source: source, reference: reference}
}

// ToIntervalSource converts the After node into an IntervalsSource.
func (n *After) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return intervals.After(
		n.source.ToIntervalSource(field, analyzer),
		n.reference.ToIntervalSource(field, analyzer),
	)
}

// String returns "fn:after(<source> <reference>)".
func (n *After) String() string {
	return fmt.Sprintf("fn:after(%s %s)", n.source, n.reference)
}
