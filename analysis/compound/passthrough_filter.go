// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compound

import "github.com/FlavioCFOliveira/Gocene/analysis"

// passThroughFilter is a no-op TokenFilter that delegates everything to its
// input unchanged. Used when a factory is configured with an empty dictionary.
type passThroughFilter struct {
	*analysis.BaseTokenFilter
}

func (f *passThroughFilter) IncrementToken() (bool, error) {
	return f.GetInput().IncrementToken()
}

var _ analysis.TokenFilter = (*passThroughFilter)(nil)
