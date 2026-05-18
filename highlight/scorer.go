// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import "github.com/FlavioCFOliveira/Gocene/analysis"

// Scorer is the contract every text-fragment scorer must satisfy. Mirrors
// org.apache.lucene.search.highlight.Scorer.
//
// The highlighter calls Init once per text input, GetTokenScore once per
// token to fold in its contribution, and GetFragmentScore once per fragment
// to retrieve the final score. StartFragment is invoked between fragments.
type Scorer interface {
	// Init prepares the scorer for a new text input.
	Init(tokenStream analysis.TokenStream) (analysis.TokenStream, error)

	// StartFragment is called at the start of every fragment.
	StartFragment(newFragment TextFragment)

	// GetTokenScore returns the score contribution of the current token.
	GetTokenScore() float32

	// GetFragmentScore returns the score of the current fragment.
	GetFragmentScore() float32

	// AllFragments returns the global score across every fragment.
	AllFragments() float32
}
