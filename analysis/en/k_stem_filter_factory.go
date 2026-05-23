// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package en

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// KStemFilterFactory creates KStemFilter instances.
//
// Go port of org.apache.lucene.analysis.en.KStemFilterFactory
// (Apache Lucene 10.4.0).
//
// SPI name: "kStem"
type KStemFilterFactory struct{}

// NewKStemFilterFactory creates a new KStemFilterFactory.
func NewKStemFilterFactory() *KStemFilterFactory {
	return &KStemFilterFactory{}
}

// Create wraps the given input stream with a KStemFilter.
func (f *KStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewKStemFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*KStemFilterFactory)(nil)
