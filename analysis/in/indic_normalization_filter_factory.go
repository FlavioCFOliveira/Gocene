// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package in

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// IndicNormalizationFilterFactory creates IndicNormalizationFilter instances.
//
// Go port of org.apache.lucene.analysis.in.IndicNormalizationFilterFactory
// (Apache Lucene 10.4.0).
//
// SPI name: "indicNormalization"
type IndicNormalizationFilterFactory struct{}

// NewIndicNormalizationFilterFactory creates a new IndicNormalizationFilterFactory.
func NewIndicNormalizationFilterFactory() *IndicNormalizationFilterFactory {
	return &IndicNormalizationFilterFactory{}
}

// Create wraps the given input stream with an IndicNormalizationFilter.
func (f *IndicNormalizationFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewIndicNormalizationFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*IndicNormalizationFilterFactory)(nil)
