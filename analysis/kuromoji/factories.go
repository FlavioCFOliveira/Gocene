// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// JapaneseKatakanaStemFilterFactory creates JapaneseKatakanaStemFilter
// instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseKatakanaStemFilterFactory from Apache
// Lucene 10.4.0.
type JapaneseKatakanaStemFilterFactory struct {
	minimumLength int
}

// NewJapaneseKatakanaStemFilterFactory creates a factory with the given
// minimum length.
func NewJapaneseKatakanaStemFilterFactory(minimumLength int) *JapaneseKatakanaStemFilterFactory {
	if minimumLength < 2 {
		panic("kuromoji: JapaneseKatakanaStemFilterFactory minimumLength must be >= 2")
	}
	return &JapaneseKatakanaStemFilterFactory{minimumLength: minimumLength}
}

// NewJapaneseKatakanaStemFilterFactoryDefault creates a factory with the
// default minimum length.
func NewJapaneseKatakanaStemFilterFactoryDefault() *JapaneseKatakanaStemFilterFactory {
	return NewJapaneseKatakanaStemFilterFactory(DefaultMinimumKatakanaLength)
}

// Create returns a new JapaneseKatakanaStemFilter wrapping input.
func (f *JapaneseKatakanaStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseKatakanaStemFilter(input, f.minimumLength)
}

// Ensure JapaneseKatakanaStemFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseKatakanaStemFilterFactory)(nil)

// JapaneseHiraganaUppercaseFilterFactory creates
// JapaneseHiraganaUppercaseFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseHiraganaUppercaseFilterFactory from
// Apache Lucene 10.4.0.
type JapaneseHiraganaUppercaseFilterFactory struct{}

// NewJapaneseHiraganaUppercaseFilterFactory creates a
// JapaneseHiraganaUppercaseFilterFactory.
func NewJapaneseHiraganaUppercaseFilterFactory() *JapaneseHiraganaUppercaseFilterFactory {
	return &JapaneseHiraganaUppercaseFilterFactory{}
}

// Create returns a new JapaneseHiraganaUppercaseFilter wrapping input.
func (f *JapaneseHiraganaUppercaseFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseHiraganaUppercaseFilter(input)
}

// Ensure JapaneseHiraganaUppercaseFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseHiraganaUppercaseFilterFactory)(nil)

// JapaneseKatakanaUppercaseFilterFactory creates
// JapaneseKatakanaUppercaseFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseKatakanaUppercaseFilterFactory from
// Apache Lucene 10.4.0.
type JapaneseKatakanaUppercaseFilterFactory struct{}

// NewJapaneseKatakanaUppercaseFilterFactory creates a
// JapaneseKatakanaUppercaseFilterFactory.
func NewJapaneseKatakanaUppercaseFilterFactory() *JapaneseKatakanaUppercaseFilterFactory {
	return &JapaneseKatakanaUppercaseFilterFactory{}
}

// Create returns a new JapaneseKatakanaUppercaseFilter wrapping input.
func (f *JapaneseKatakanaUppercaseFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseKatakanaUppercaseFilter(input)
}

// Ensure JapaneseKatakanaUppercaseFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseKatakanaUppercaseFilterFactory)(nil)

// JapaneseNumberFilterFactory creates JapaneseNumberFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseNumberFilterFactory from Apache
// Lucene 10.4.0.
type JapaneseNumberFilterFactory struct{}

// NewJapaneseNumberFilterFactory creates a JapaneseNumberFilterFactory.
func NewJapaneseNumberFilterFactory() *JapaneseNumberFilterFactory {
	return &JapaneseNumberFilterFactory{}
}

// Create returns a new JapaneseNumberFilter wrapping input.
func (f *JapaneseNumberFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseNumberFilter(input)
}

// Ensure JapaneseNumberFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseNumberFilterFactory)(nil)

// JapaneseBaseFormFilterFactory creates JapaneseBaseFormFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseBaseFormFilterFactory from Apache
// Lucene 10.4.0.
type JapaneseBaseFormFilterFactory struct{}

// NewJapaneseBaseFormFilterFactory creates a JapaneseBaseFormFilterFactory.
func NewJapaneseBaseFormFilterFactory() *JapaneseBaseFormFilterFactory {
	return &JapaneseBaseFormFilterFactory{}
}

// Create returns a new JapaneseBaseFormFilter wrapping input.
func (f *JapaneseBaseFormFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseBaseFormFilter(input)
}

// Ensure JapaneseBaseFormFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseBaseFormFilterFactory)(nil)

// JapanesePartOfSpeechStopFilterFactory creates
// JapanesePartOfSpeechStopFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapanesePartOfSpeechStopFilterFactory from
// Apache Lucene 10.4.0.
type JapanesePartOfSpeechStopFilterFactory struct {
	stopTags map[string]struct{}
}

// NewJapanesePartOfSpeechStopFilterFactory creates a factory with the given
// stop tags. If stopTags is nil, a non-filtering factory is produced (the
// filter will pass all tokens through).
func NewJapanesePartOfSpeechStopFilterFactory(stopTags map[string]struct{}) *JapanesePartOfSpeechStopFilterFactory {
	return &JapanesePartOfSpeechStopFilterFactory{stopTags: stopTags}
}

// Create returns a new JapanesePartOfSpeechStopFilter wrapping stream, or
// stream unchanged if no stop tags are configured.
func (f *JapanesePartOfSpeechStopFilterFactory) Create(stream analysis.TokenStream) analysis.TokenFilter {
	if f.stopTags == nil {
		// No stop tags: wrap with a pass-through filter.
		return analysis.NewBaseTokenFilter(stream)
	}
	return NewJapanesePartOfSpeechStopFilter(stream, f.stopTags)
}

// Ensure JapanesePartOfSpeechStopFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapanesePartOfSpeechStopFilterFactory)(nil)

// JapaneseReadingFormFilterFactory creates JapaneseReadingFormFilter
// instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseReadingFormFilterFactory from Apache
// Lucene 10.4.0.
type JapaneseReadingFormFilterFactory struct {
	useRomaji bool
}

// NewJapaneseReadingFormFilterFactory creates a factory that produces
// katakana-reading or romaji-reading filters.
func NewJapaneseReadingFormFilterFactory(useRomaji bool) *JapaneseReadingFormFilterFactory {
	return &JapaneseReadingFormFilterFactory{useRomaji: useRomaji}
}

// Create returns a new JapaneseReadingFormFilter wrapping input.
func (f *JapaneseReadingFormFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseReadingFormFilter(input, f.useRomaji)
}

// Ensure JapaneseReadingFormFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseReadingFormFilterFactory)(nil)

// JapaneseCompletionFilterFactory creates JapaneseCompletionFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseCompletionFilterFactory from Apache
// Lucene 10.4.0.
type JapaneseCompletionFilterFactory struct {
	mode CompletionMode
}

// NewJapaneseCompletionFilterFactory creates a factory with the given mode.
func NewJapaneseCompletionFilterFactory(mode CompletionMode) *JapaneseCompletionFilterFactory {
	return &JapaneseCompletionFilterFactory{mode: mode}
}

// NewJapaneseCompletionFilterFactoryDefault creates a factory with index mode.
func NewJapaneseCompletionFilterFactoryDefault() *JapaneseCompletionFilterFactory {
	return NewJapaneseCompletionFilterFactory(DefaultCompletionMode)
}

// Create returns a new JapaneseCompletionFilter wrapping input.
func (f *JapaneseCompletionFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewJapaneseCompletionFilter(input, f.mode)
}

// Ensure JapaneseCompletionFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*JapaneseCompletionFilterFactory)(nil)

// JapaneseIterationMarkCharFilterFactory creates
// JapaneseIterationMarkCharFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseIterationMarkCharFilterFactory from
// Apache Lucene 10.4.0.
type JapaneseIterationMarkCharFilterFactory struct {
	normalizeKanji bool
	normalizeKana  bool
}

// NewJapaneseIterationMarkCharFilterFactory creates a factory with the given
// normalization flags.
func NewJapaneseIterationMarkCharFilterFactory(normalizeKanji, normalizeKana bool) *JapaneseIterationMarkCharFilterFactory {
	return &JapaneseIterationMarkCharFilterFactory{
		normalizeKanji: normalizeKanji,
		normalizeKana:  normalizeKana,
	}
}

// Create returns a new JapaneseIterationMarkCharFilter wrapping r.
func (f *JapaneseIterationMarkCharFilterFactory) Create(r io.Reader) *analysis.CharFilter {
	filt := NewJapaneseIterationMarkCharFilter(r, f.normalizeKanji, f.normalizeKana)
	return filt.CharFilter
}
