// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package commongrams

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/en"
)

// CommonGramsFilterFactory constructs CommonGramsFilter instances.
//
// Go port of org.apache.lucene.analysis.commongrams.CommonGramsFilterFactory
// (Apache Lucene 10.4.0).
//
// SPI name: "commonGrams"
type CommonGramsFilterFactory struct {
	en.AbstractWordsFileFilterFactory
}

// NewCommonGramsFilterFactory creates a new CommonGramsFilterFactory. The
// args map may contain "words", "format", and "ignoreCase" keys. When no
// word list is configured, English stop words are used as the default
// common-word set, mirroring the Java createDefaultWords().
func NewCommonGramsFilterFactory(args map[string]string) *CommonGramsFilterFactory {
	f := &CommonGramsFilterFactory{}
	f.Init(args)
	if f.Words() == nil {
		f.SetWords(analysis.GetWordSetFromStrings(analysis.EnglishStopWords, f.IgnoreCase()))
	}
	return f
}

// NewCommonGramsFilterFactoryWithWords creates a factory using the supplied
// word set directly.
func NewCommonGramsFilterFactoryWithWords(commonWords *analysis.CharArraySet) *CommonGramsFilterFactory {
	f := &CommonGramsFilterFactory{}
	f.SetWords(commonWords)
	return f
}

// GetCommonWords returns the set of common words used to create bigrams.
func (f *CommonGramsFilterFactory) GetCommonWords() *analysis.CharArraySet {
	return f.Words()
}

// Create wraps input with a CommonGramsFilter using the configured word set.
func (f *CommonGramsFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewCommonGramsFilter(input, f.Words())
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*CommonGramsFilterFactory)(nil)

// CommonGramsQueryFilterFactory creates a CommonGramsFilter wrapped in a
// CommonGramsQueryFilter, optimising phrase queries.
//
// Go port of
// org.apache.lucene.analysis.commongrams.CommonGramsQueryFilterFactory
// (Apache Lucene 10.4.0).
//
// SPI name: "commonGramsQuery"
type CommonGramsQueryFilterFactory struct {
	CommonGramsFilterFactory
}

// NewCommonGramsQueryFilterFactory creates a new
// CommonGramsQueryFilterFactory with optional args.
func NewCommonGramsQueryFilterFactory(args map[string]string) *CommonGramsQueryFilterFactory {
	f := &CommonGramsQueryFilterFactory{}
	f.Init(args)
	if f.Words() == nil {
		f.SetWords(analysis.GetWordSetFromStrings(analysis.EnglishStopWords, f.IgnoreCase()))
	}
	return f
}

// Create wraps input with CommonGramsFilter then CommonGramsQueryFilter.
func (f *CommonGramsQueryFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	cgf := NewCommonGramsFilter(input, f.Words())
	return NewCommonGramsQueryFilter(cgf)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*CommonGramsQueryFilterFactory)(nil)
