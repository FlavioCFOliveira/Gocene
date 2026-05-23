// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pattern

import (
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SimplePatternSplitTokenizerFactory creates [analysis.SimplePatternSplitTokenizer]
// instances from a pre-compiled regular expression.
//
// The pattern matches the characters that act as token separators (like
// [strings.Split] or Java's String.split). The matching is greedy: the longest
// separator at any given point wins. Empty tokens are never produced.
//
// This is the Go port of
// org.apache.lucene.analysis.pattern.SimplePatternSplitTokenizerFactory from
// Apache Lucene 10.4.0.
//
// Deviation: Java uses Lucene RegExp + Operations.determinize to build a
// finite-automaton tokenizer. Go uses standard library regexp because Lucene's
// automaton-based tokenizer is not yet ported. The public API is identical.
type SimplePatternSplitTokenizerFactory struct {
	pattern *regexp.Regexp
}

// NewSimplePatternSplitTokenizerFactory builds a factory from a pre-compiled
// regular expression.
func NewSimplePatternSplitTokenizerFactory(pattern *regexp.Regexp) *SimplePatternSplitTokenizerFactory {
	return &SimplePatternSplitTokenizerFactory{pattern: pattern}
}

// NewSimplePatternSplitTokenizerFactoryFromString compiles pattern and builds
// the factory. Returns an error if the pattern is invalid.
func NewSimplePatternSplitTokenizerFactoryFromString(pattern string) (*SimplePatternSplitTokenizerFactory, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return NewSimplePatternSplitTokenizerFactory(re), nil
}

// Create returns a new [analysis.SimplePatternSplitTokenizer].
func (f *SimplePatternSplitTokenizerFactory) Create() analysis.Tokenizer {
	t, err := analysis.NewSimplePatternSplitTokenizer(f.pattern)
	if err != nil {
		// pattern was validated at construction; this cannot happen.
		panic("simplePatternSplitTokenizerFactory: unexpected error: " + err.Error())
	}
	return t
}

// Ensure factory implements analysis.TokenizerFactory.
var _ analysis.TokenizerFactory = (*SimplePatternSplitTokenizerFactory)(nil)
