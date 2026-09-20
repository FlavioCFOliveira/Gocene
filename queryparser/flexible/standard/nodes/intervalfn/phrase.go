// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

// Phrase is a node that represents Intervals#phrase.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Phrase.
type Phrase struct {
	sources []IntervalFunction
}

// NewPhrase creates a new Phrase node.
func NewPhrase(sources []IntervalFunction) *Phrase {
	if sources == nil {
		panic("sources must not be nil")
	}
	return &Phrase{sources: sources}
}

// ToIntervalSource converts the Phrase node into an IntervalsSource.
func (n *Phrase) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	sources := make([]intervals.IntervalsSource, len(n.sources))
	for i, src := range n.sources {
		sources[i] = src.ToIntervalSource(field, analyzer)
	}
	return intervals.PhraseOf(sources...)
}

// String returns "fn:phrase(<s1> <s2> ...)".
func (n *Phrase) String() string {
	return fmt.Sprintf("fn:phrase(%s)", joinFunctions(n.sources))
}
