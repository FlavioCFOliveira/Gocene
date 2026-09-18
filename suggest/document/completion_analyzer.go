// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/miscellaneous"
)

// HOLE_CHARACTER represents a hole character, inserted by TokenStreamToAutomaton.
// It mirrors Lucene's TokenStreamToAutomaton.HOLE (0x001E).
const HOLE_CHARACTER = '\x1e'

// CompletionAnalyzer wraps an Analyzer to provide additional completion-only tuning.
//
// This is the Go port of org.apache.lucene.search.suggest.document.CompletionAnalyzer from Apache Lucene 10.5.0.
type CompletionAnalyzer struct {
	*analysis.AnalyzerWrapper

	analyzer                   analysis.Analyzer
	preserveSep                bool
	preservePositionIncrements bool
	maxGraphExpansions         int
}

// NewCompletionAnalyzer creates a CompletionAnalyzer wrapping the given analyzer.
func NewCompletionAnalyzer(analyzer analysis.Analyzer) *CompletionAnalyzer {
	return NewCompletionAnalyzerFull(
		analyzer,
		miscellaneous.DefaultSepLabel,
		true,
		miscellaneous.DefaultMaxGraphExpansions,
	)
}

// NewCompletionAnalyzerFull creates a CompletionAnalyzer wrapping the given analyzer with explicit settings.
func NewCompletionAnalyzerFull(
	analyzer analysis.Analyzer,
	preserveSep bool,
	preservePositionIncrements bool,
	maxGraphExpansions int,
) *CompletionAnalyzer {
	ca := &CompletionAnalyzer{
		analyzer:                   analyzer,
		preserveSep:                preserveSep,
		preservePositionIncrements: preservePositionIncrements,
		maxGraphExpansions:         maxGraphExpansions,
	}

	// Initialize AnalyzerWrapper
	ca.AnalyzerWrapper = analysis.NewAnalyzerWrapper(func(fieldName string) analysis.Analyzer {
		return analyzer
	})

	// Set the WrapTokenStream hook
	ca.AnalyzerWrapper.WrapTokenStream = func(fieldName string, in analysis.TokenStream) analysis.TokenStream {
		return NewCompletionTokenStreamFull(
			in,
			ca.preserveSep,
			ca.preservePositionIncrements,
			ca.maxGraphExpansions,
		)
	}

	return ca
}

// PreserveSep returns true if separation between tokens are preserved.
func (ca *CompletionAnalyzer) PreserveSep() bool {
	return ca.preserveSep
}

// PreservePositionIncrements returns true if position increments are preserved.
func (ca *CompletionAnalyzer) PreservePositionIncrements() bool {
	return ca.preservePositionIncrements
}

// Ensure CompletionAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*CompletionAnalyzer)(nil)
