// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FuzzyTerm is a node that represents Intervals#fuzzyTerm.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.FuzzyTerm.
type FuzzyTerm struct {
	term          string
	maxEdits      int
	maxExpansions int
}

// NewFuzzyTerm creates a new FuzzyTerm node. Passing nil for maxEdits or
// maxExpansions selects the Lucene default, mirroring the Integer parameters of
// the Java constructor.
func NewFuzzyTerm(term string, maxEdits, maxExpansions *int) *FuzzyTerm {
	edits := search.DefaultMaxEdits
	if maxEdits != nil {
		edits = *maxEdits
	}
	expansions := intervals.DefaultMaxExpansions
	if maxExpansions != nil {
		expansions = *maxExpansions
	}
	return &FuzzyTerm{term: term, maxEdits: edits, maxExpansions: expansions}
}

// ToIntervalSource converts the FuzzyTerm node into an IntervalsSource.
//
// Java calls Intervals.fuzzyTerm(term, maxEdits, FuzzyQuery.defaultPrefixLength,
// FuzzyQuery.defaultTranspositions, maxExpansions), whose body is
// Intervals.multiterm(FuzzyQuery.getFuzzyAutomaton(...), maxExpansions,
// term + "~" + maxEdits). Gocene's intervals package does not ship the
// fuzzyTerm convenience factory and documents that callers build the
// CompiledAutomaton and call Multiterm directly, so that body is reproduced
// here verbatim.
func (n *FuzzyTerm) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	ca := search.FuzzyQueryGetFuzzyAutomaton(
		n.term, n.maxEdits, search.DefaultPrefixLength, search.DefaultTranspositions)
	return intervals.MultitermWithMaxExpansions(
		ca, n.maxExpansions, fmt.Sprintf("%s~%d", n.term, n.maxEdits))
}

// String returns "fn:fuzzyTerm(<term> <maxEdits><maxExpansions>)".
//
// The missing separator between maxEdits and maxExpansions is Lucene's own
// format string, reproduced as-is.
func (n *FuzzyTerm) String() string {
	term := n.term
	if requiresQuotes(term) {
		term = `"` + term + `"`
	}
	return fmt.Sprintf("fn:fuzzyTerm(%s %d%d)", term, n.maxEdits, n.maxExpansions)
}
