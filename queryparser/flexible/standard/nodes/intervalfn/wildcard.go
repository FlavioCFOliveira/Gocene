// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Wildcard is a node that represents Intervals#wildcard.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Wildcard.
type Wildcard struct {
	wildcard      string
	maxExpansions int
}

// NewWildcard creates a new Wildcard node. A maxExpansions of 0 selects
// Intervals.DEFAULT_MAX_EXPANSIONS, matching the Java toIntervalSource branch.
func NewWildcard(wildcard string, maxExpansions int) *Wildcard {
	return &Wildcard{wildcard: wildcard, maxExpansions: maxExpansions}
}

// ToIntervalSource converts the Wildcard node into an IntervalsSource.
//
// Java calls Intervals.wildcard(new BytesRef(wildcard)) when maxExpansions is 0
// and Intervals.wildcard(new BytesRef(wildcard), maxExpansions) otherwise. The
// body of Intervals.wildcard is
//
//	CompiledAutomaton ca = new CompiledAutomaton(
//	    WildcardQuery.toAutomaton(new Term("", wildcard), DEFAULT_DETERMINIZE_WORK_LIMIT));
//	return new MultiTermIntervalsSource(ca, maxExpansions, wildcard.utf8ToString());
//
// Gocene's intervals package does not ship the wildcard convenience factory and
// documents that callers build the CompiledAutomaton and call Multiterm
// directly, so that body is reproduced here verbatim.
func (n *Wildcard) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	maxExpansions := n.maxExpansions
	if maxExpansions == 0 {
		maxExpansions = intervals.DefaultMaxExpansions
	}
	a := search.WildcardQueryToAutomaton(
		index.NewTerm("", n.wildcard), automaton.DefaultDeterminizeWorkLimit)
	return intervals.MultitermWithMaxExpansions(
		automaton.NewCompiledAutomatonSimplified(a), maxExpansions, n.wildcard)
}

// String returns "fn:wildcard(<pattern>)" or
// "fn:wildcard(<pattern> maxExpansions:<n>)".
func (n *Wildcard) String() string {
	suffix := ""
	if n.maxExpansions != 0 {
		suffix = fmt.Sprintf(" maxExpansions:%d", n.maxExpansions)
	}
	return fmt.Sprintf("fn:wildcard(%s%s)", n.wildcard, suffix)
}
