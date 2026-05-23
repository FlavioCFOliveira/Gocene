// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package icu provides ICU-based analysis components for Unicode text.
//
// Go port of org.apache.lucene.analysis.icu (Apache Lucene 10.4.0).
//
// Deviation: The Java package depends on ICU4J (com.ibm.icu). Go has no
// equivalent CGO-free library with full ICU4J API parity. This package
// provides structural equivalents using golang.org/x/text and the Go
// standard library where possible. The Transliterator, Normalizer2 (nfkc_cf),
// RuleBasedBreakIterator with dictionary segmentation, and Collator types are
// represented as Go interfaces so callers can provide pluggable implementations.
package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Transliterator transforms text according to a set of rules.
//
// This is the Go equivalent of com.ibm.icu.text.Transliterator.
// Callers must supply a concrete implementation; the icu package does not
// bundle a default ICU rule engine.
type Transliterator interface {
	// Transliterate applies the transform to src and returns the result.
	Transliterate(src string) string
}

// ICUTransformFilter is a TokenFilter that transforms token text using a
// Transliterator.
//
// Go port of org.apache.lucene.analysis.icu.ICUTransformFilter
// (Apache Lucene 10.4.0).
//
// Deviation: The Java implementation uses com.ibm.icu.text.Transliterator
// directly, including a ReplaceableTermAttribute Replaceable adapter and
// a source-set optimiser that automatically installs the transliterator's
// source-set as its filter to skip characters outside the source set. In Go
// the Transliterator is an interface; callers that need the source-set
// optimisation must configure their implementation themselves.
type ICUTransformFilter struct {
	*analysis.BaseTokenFilter
	transform Transliterator
	termAttr  analysis.CharTermAttribute
}

// NewICUTransformFilter creates a new ICUTransformFilter that applies
// transform to every token produced by input.
func NewICUTransformFilter(input analysis.TokenStream, transform Transliterator) *ICUTransformFilter {
	f := &ICUTransformFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		transform:       transform,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if attr := src.GetAttribute(analysis.CharTermAttributeType); attr != nil {
			f.termAttr = attr.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token and applies the transliteration.
func (f *ICUTransformFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().(analysis.TokenStream).IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr != nil {
		result := f.transform.Transliterate(f.termAttr.String())
		f.termAttr.SetValue(result)
	}
	return true, nil
}

// GetInput returns the wrapped input TokenStream.
func (f *ICUTransformFilter) GetInput() analysis.TokenStream {
	return f.BaseTokenFilter.GetInput()
}

// ICUTransformFilterFactory creates ICUTransformFilter instances.
//
// Go port of org.apache.lucene.analysis.icu.ICUTransformFilterFactory
// (Apache Lucene 10.4.0).
type ICUTransformFilterFactory struct {
	transform Transliterator
}

// NewICUTransformFilterFactory creates a factory that wraps each input
// TokenStream with an ICUTransformFilter applying the given Transliterator.
func NewICUTransformFilterFactory(transform Transliterator) *ICUTransformFilterFactory {
	return &ICUTransformFilterFactory{transform: transform}
}

// Create wraps input with an ICUTransformFilter.
func (f *ICUTransformFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewICUTransformFilter(input, f.transform)
}

// Ensure compile-time interface satisfaction.
var _ analysis.TokenFilter = (*ICUTransformFilter)(nil)
