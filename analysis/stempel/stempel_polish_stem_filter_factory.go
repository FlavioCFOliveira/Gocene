// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package stempel

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// StempelPolishStemFilterFactory is a TokenFilterFactory that creates a
// StempelFilter using the built-in Polish stemming table.
//
// SPI name: "stempelPolishStem"
//
// This is the Go port of
// org.apache.lucene.analysis.stempel.StempelPolishStemFilterFactory
// (Lucene 10.4.0).
type StempelPolishStemFilterFactory struct{}

// NewStempelPolishStemFilterFactory creates a new factory from a parameter map.
// Rejects unknown parameters, mirroring the Lucene reference.
func NewStempelPolishStemFilterFactory(args map[string]string) (*StempelPolishStemFilterFactory, error) {
	if len(args) > 0 {
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("unknown parameters: %v", keys)
	}
	return &StempelPolishStemFilterFactory{}, nil
}

// Create returns a StempelFilter that applies the Polish stemmer to input.
func (f *StempelPolishStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewStempelFilter(input, NewStempelStemmer(GetDefaultTable()))
}

// Ensure StempelPolishStemFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*StempelPolishStemFilterFactory)(nil)
