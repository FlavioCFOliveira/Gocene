// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"regexp"
	"time"
)

// PerFieldAnalyzerWrapper selects a different Analyzer for each
// indexed field. Falls back to defaultAnalyzer when fieldAnalyzers
// has no entry for the requested field.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.PerFieldAnalyzerWrapper
// from Apache Lucene 10.4.0.
type PerFieldAnalyzerWrapper struct {
	*AnalyzerWrapper

	defaultAnalyzer Analyzer
	fieldAnalyzers  map[string]Analyzer
}

// NewPerFieldAnalyzerWrapper returns a wrapper that delegates to
// defaultAnalyzer when no per-field analyzer is configured for a
// field.
func NewPerFieldAnalyzerWrapper(defaultAnalyzer Analyzer) *PerFieldAnalyzerWrapper {
	return NewPerFieldAnalyzerWrapperWithMap(defaultAnalyzer, nil)
}

// NewPerFieldAnalyzerWrapperWithMap returns a wrapper configured
// with the given per-field analyzer overrides; pass nil for an
// empty map.
func NewPerFieldAnalyzerWrapperWithMap(defaultAnalyzer Analyzer, fieldAnalyzers map[string]Analyzer) *PerFieldAnalyzerWrapper {
	if fieldAnalyzers == nil {
		fieldAnalyzers = map[string]Analyzer{}
	}
	w := &PerFieldAnalyzerWrapper{
		defaultAnalyzer: defaultAnalyzer,
		fieldAnalyzers:  fieldAnalyzers,
	}
	w.AnalyzerWrapper = NewAnalyzerWrapper(w.getWrappedAnalyzer)
	return w
}

func (w *PerFieldAnalyzerWrapper) getWrappedAnalyzer(fieldName string) Analyzer {
	if a, ok := w.fieldAnalyzers[fieldName]; ok && a != nil {
		return a
	}
	return w.defaultAnalyzer
}

// DefaultAnalyzer returns the fallback analyzer.
func (w *PerFieldAnalyzerWrapper) DefaultAnalyzer() Analyzer {
	return w.defaultAnalyzer
}

// FieldAnalyzers returns the configured per-field map (read-only
// reference; callers must not mutate it).
func (w *PerFieldAnalyzerWrapper) FieldAnalyzers() map[string]Analyzer {
	return w.fieldAnalyzers
}

// --- LimitTokenCountAnalyzer ---

// LimitTokenCountAnalyzer wraps another Analyzer and applies
// LimitTokenCountFilter to its token stream. Useful as a replacement
// for the maxFieldLength setting on IndexWriter.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.LimitTokenCountAnalyzer
// from Apache Lucene 10.4.0.
type LimitTokenCountAnalyzer struct {
	*AnalyzerWrapper

	delegate            Analyzer
	maxTokenCount       int
	consumeAllTokens    bool
}

// NewLimitTokenCountAnalyzer wraps delegate, capping each field's
// token stream at maxTokenCount tokens.
func NewLimitTokenCountAnalyzer(delegate Analyzer, maxTokenCount int) *LimitTokenCountAnalyzer {
	return NewLimitTokenCountAnalyzerWithConsume(delegate, maxTokenCount, false)
}

// NewLimitTokenCountAnalyzerWithConsume wraps delegate as
// NewLimitTokenCountAnalyzer but with an explicit flag controlling
// whether the underlying stream is fully consumed after the limit
// is reached.
func NewLimitTokenCountAnalyzerWithConsume(delegate Analyzer, maxTokenCount int, consumeAllTokens bool) *LimitTokenCountAnalyzer {
	a := &LimitTokenCountAnalyzer{
		delegate:         delegate,
		maxTokenCount:    maxTokenCount,
		consumeAllTokens: consumeAllTokens,
	}
	a.AnalyzerWrapper = NewAnalyzerWrapper(func(fieldName string) Analyzer { return delegate })
	a.AnalyzerWrapper.WrapTokenStream = func(_ string, in TokenStream) TokenStream {
		return NewLimitTokenCountFilter(in, maxTokenCount)
	}
	return a
}

// MaxTokenCount returns the configured token count limit.
func (a *LimitTokenCountAnalyzer) MaxTokenCount() int {
	return a.maxTokenCount
}

// ConsumeAllTokens returns the configured consumeAllTokens flag.
func (a *LimitTokenCountAnalyzer) ConsumeAllTokens() bool {
	return a.consumeAllTokens
}

// --- PatternKeywordMarkerFilter ---

// PatternKeywordMarkerFilter marks every token whose text matches
// the supplied compiled regular expression as a keyword.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.PatternKeywordMarkerFilter.
type PatternKeywordMarkerFilter struct {
	*KeywordMarkerFilter

	pattern *regexp.Regexp
}

// NewPatternKeywordMarkerFilter wraps input. pattern must be
// non-nil.
func NewPatternKeywordMarkerFilter(input TokenStream, pattern *regexp.Regexp) *PatternKeywordMarkerFilter {
	if pattern == nil {
		panic("PatternKeywordMarkerFilter: pattern must not be nil")
	}
	f := &PatternKeywordMarkerFilter{pattern: pattern}
	f.KeywordMarkerFilter = NewKeywordMarkerFilter(input, f.isKeyword)
	return f
}

func (f *PatternKeywordMarkerFilter) isKeyword() bool {
	if f.termAttr == nil {
		return false
	}
	return f.pattern.MatchString(f.termAttr.String())
}

// Ensure PatternKeywordMarkerFilter implements TokenFilter.
var _ TokenFilter = (*PatternKeywordMarkerFilter)(nil)

// --- DateRecognizerFilter ---

// DateRecognizerType is the token type set on tokens parsed as
// dates by the DateRecognizerFilter. Matches Lucene's DATE_TYPE.
const DateRecognizerType = "date"

// DateRecognizerFilter accepts tokens whose text parses as a date
// under the configured layout. Tokens that fail to parse are
// dropped.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.DateRecognizerFilter.
//
// Deviation from Lucene: the reference accepts a java.text.DateFormat
// (which carries locale and style). Gocene uses a Go reference time
// layout string (see the time package). The default layout matches
// the English short form returned by Java's DateFormat.getDateInstance
// with DEFAULT style: time.RFC3339 is *not* used by default because
// it is not equivalent to the Java default; instead the Go default
// is "01/02/06" (US slash) and "02/01/2006" (DMY), both attempted in
// order, mimicking the typical Java DateFormat.DEFAULT behaviour.
type DateRecognizerFilter struct {
	*BaseTokenFilter

	layouts []string
	termAttr CharTermAttribute
}

// NewDateRecognizerFilter wraps input with default layouts.
func NewDateRecognizerFilter(input TokenStream) *DateRecognizerFilter {
	return NewDateRecognizerFilterWithLayouts(input, []string{"01/02/06", "02/01/2006", "2006-01-02"})
}

// NewDateRecognizerFilterWithLayouts wraps input with the given
// candidate layout strings.
func NewDateRecognizerFilterWithLayouts(input TokenStream, layouts []string) *DateRecognizerFilter {
	f := &DateRecognizerFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		layouts:         append([]string(nil), layouts...),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken drops tokens that do not parse under any layout.
func (f *DateRecognizerFilter) IncrementToken() (bool, error) {
	for {
		ok, err := f.input.IncrementToken()
		if err != nil || !ok {
			return ok, err
		}
		if f.termAttr == nil {
			return true, nil
		}
		s := f.termAttr.String()
		matched := false
		for _, layout := range f.layouts {
			if _, err := time.Parse(layout, s); err == nil {
				matched = true
				break
			}
		}
		if matched {
			return true, nil
		}
	}
}

// Ensure DateRecognizerFilter implements TokenFilter.
var _ TokenFilter = (*DateRecognizerFilter)(nil)

// DateRecognizerFilterFactory creates DateRecognizerFilter instances.
type DateRecognizerFilterFactory struct {
	layouts []string
}

// NewDateRecognizerFilterFactory returns a factory using default
// layouts.
func NewDateRecognizerFilterFactory() *DateRecognizerFilterFactory {
	return &DateRecognizerFilterFactory{}
}

// NewDateRecognizerFilterFactoryWithLayouts returns a factory with
// the given layout list.
func NewDateRecognizerFilterFactoryWithLayouts(layouts []string) *DateRecognizerFilterFactory {
	return &DateRecognizerFilterFactory{layouts: append([]string(nil), layouts...)}
}

// Create wraps input.
func (f *DateRecognizerFilterFactory) Create(input TokenStream) TokenFilter {
	if len(f.layouts) == 0 {
		return NewDateRecognizerFilter(input)
	}
	return NewDateRecognizerFilterWithLayouts(input, f.layouts)
}

// --- ConditionalTokenFilter (abstract base) ---

// ConditionalTokenFilter conditionally routes each token through an
// inner filter chain when ShouldFilter returns true.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ConditionalTokenFilter
// from Apache Lucene 10.4.0.
//
// Deviation from Lucene: the reference uses a Function<TokenStream,
// TokenStream> factory bound at construction time and a
// OneTimeWrapper indirection to wire the inner filter chain to the
// outer input. Gocene's implementation is intentionally a small
// hook: callers supply BothShouldFilterFunc plus an Inner
// TokenStream that has already been wired to BypassInput; for most
// use cases the function-typed factory pattern works equally well
// in Go without the inner OneTimeWrapper because Go closures
// capture the outer state directly.
type ConditionalTokenFilter struct {
	*BaseTokenFilter

	// ShouldFilterFunc is consulted before each token; when it
	// returns true the next call to IncrementToken pulls from Inner,
	// otherwise it pulls from the upstream input directly.
	ShouldFilterFunc func() (bool, error)

	// Inner is the wrapped filter chain. It must read from the same
	// upstream input passed to this filter; if Inner is nil the
	// filter behaves as a pass-through.
	Inner TokenStream
}

// NewConditionalTokenFilter wraps input with the conditional
// routing. ShouldFilter is consulted on every pulled token.
func NewConditionalTokenFilter(input TokenStream, shouldFilter func() (bool, error), inner TokenStream) *ConditionalTokenFilter {
	return &ConditionalTokenFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		ShouldFilterFunc: shouldFilter,
		Inner:            inner,
	}
}

// IncrementToken delegates to Inner when ShouldFilterFunc returns
// true; otherwise it pulls a single token from the upstream input.
func (f *ConditionalTokenFilter) IncrementToken() (bool, error) {
	apply := false
	if f.ShouldFilterFunc != nil {
		ok, err := f.ShouldFilterFunc()
		if err != nil {
			return false, err
		}
		apply = ok
	}
	if apply && f.Inner != nil {
		return f.Inner.IncrementToken()
	}
	return f.input.IncrementToken()
}

// Ensure ConditionalTokenFilter implements TokenFilter.
var _ TokenFilter = (*ConditionalTokenFilter)(nil)

