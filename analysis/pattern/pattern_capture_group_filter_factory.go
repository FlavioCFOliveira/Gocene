// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pattern

import (
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// PatternCaptureGroupFilterFactory creates [PatternCaptureGroupTokenFilter]
// instances from a pre-compiled regular expression.
//
// This is the Go port of
// org.apache.lucene.analysis.pattern.PatternCaptureGroupFilterFactory from
// Apache Lucene 10.4.0.
//
// Deviation: Java reads pattern and preserve_original from a Map<String,String>
// at Solr SPI initialisation time. Go callers supply values directly.
type PatternCaptureGroupFilterFactory struct {
	pattern          *regexp.Regexp
	preserveOriginal bool
}

// NewPatternCaptureGroupFilterFactory builds a factory.
//
//   - pattern: the compiled regular expression
//   - preserveOriginal: if true the original token is emitted first (default true
//     in the Java implementation)
func NewPatternCaptureGroupFilterFactory(pattern *regexp.Regexp, preserveOriginal bool) *PatternCaptureGroupFilterFactory {
	return &PatternCaptureGroupFilterFactory{
		pattern:          pattern,
		preserveOriginal: preserveOriginal,
	}
}

// Create returns a new [PatternCaptureGroupTokenFilter] wrapping input.
func (f *PatternCaptureGroupFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewPatternCaptureGroupTokenFilter(input, f.preserveOriginal, f.pattern)
}

// Ensure factory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*PatternCaptureGroupFilterFactory)(nil)
