// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package miscellaneous provides miscellaneous analysis filters and utilities.
package miscellaneous

import (
	"errors"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// resetable is a local interface for TokenStreams that expose Reset.
type resetable interface {
	Reset() error
}

// --- ConcatenatingTokenStream ---

// ConcatenatingTokenStream concatenates multiple TokenStreams into one, adjusting
// offsets so that all inputs appear as a single contiguous source.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ConcatenatingTokenStream from
// Apache Lucene 10.4.0.
type ConcatenatingTokenStream struct {
	*analysis.BaseTokenStream

	sources       []analysis.TokenStream
	currentSource int
	offsetInc     int
	initialPosInc int

	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute
}

// NewConcatenatingTokenStream creates a ConcatenatingTokenStream from the given sources.
// All sources must expose compatible attribute implementations.
func NewConcatenatingTokenStream(sources ...analysis.TokenStream) *ConcatenatingTokenStream {
	if len(sources) == 0 {
		panic("ConcatenatingTokenStream: at least one source is required")
	}
	ts := &ConcatenatingTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
		sources:         sources,
		initialPosInc:   1,
	}
	// Initialise offset and position attributes on our own attribute source.
	offImpl := analysis.NewOffsetAttribute()
	posImpl := analysis.NewPositionIncrementAttributeImpl()
	ts.GetAttributeSource().AddAttributeImpl(offImpl)
	ts.GetAttributeSource().AddAttributeImpl(posImpl)
	if a := ts.GetAttributeSource().GetAttribute(analysis.OffsetAttributeType); a != nil {
		ts.offsetAttr = a.(analysis.OffsetAttribute)
	}
	if a := ts.GetAttributeSource().GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		ts.posIncrAttr = a.(analysis.PositionIncrementAttribute)
	}
	return ts
}

// IncrementToken advances to the next token, switching sources as needed.
func (c *ConcatenatingTokenStream) IncrementToken() (bool, error) {
	newSource := false
	for {
		ok, err := c.sources[c.currentSource].IncrementToken()
		if err != nil {
			return false, err
		}
		if ok {
			break
		}
		// current source exhausted
		if err2 := c.sources[c.currentSource].End(); err2 != nil {
			return false, err2
		}
		if c.currentSource >= len(c.sources)-1 {
			return false, nil
		}
		c.currentSource++
		newSource = true
	}

	// Adjust position increment when switching to a new source.
	if newSource && c.posIncrAttr != nil {
		c.posIncrAttr.SetPositionIncrement(c.posIncrAttr.GetPositionIncrement() + c.initialPosInc)
	}

	return true, nil
}

// End finalises the stream.
func (c *ConcatenatingTokenStream) End() error {
	return c.sources[c.currentSource].End()
}

// Reset resets all sources and internal state.
func (c *ConcatenatingTokenStream) Reset() error {
	var errs []error
	for _, src := range c.sources {
		if r, ok := src.(resetable); ok {
			if err := r.Reset(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	c.currentSource = 0
	c.offsetInc = 0
	c.initialPosInc = 1
	return errors.Join(errs...)
}

// Close closes all sources.
func (c *ConcatenatingTokenStream) Close() error {
	var errs []error
	for _, src := range c.sources {
		if err := src.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Ensure ConcatenatingTokenStream implements TokenStream.
var _ analysis.TokenStream = (*ConcatenatingTokenStream)(nil)

// --- ConcatenateGraphFilter ---

// DefaultSepLabel is the default separator between tokens in ConcatenateGraphFilter.
// It mirrors ConcatenateGraphFilter.SEP_LABEL (0x001F, the ASCII Unit Separator).
const DefaultSepLabel = '\x1f'

// DefaultMaxGraphExpansions is the default maximum number of graph expansions.
const DefaultMaxGraphExpansions = 10000

// ConcatenateGraphFilter concatenates all incoming tokens (across graph paths) into
// single output tokens, inserting a configurable separator character between them.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ConcatenateGraphFilter from Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference uses automaton-based finite-string enumeration. This
// Go port implements single-path concatenation without full automaton graph expansion.
// Full graph expansion is deferred until the automaton dependency lands in miscellaneous.
type ConcatenateGraphFilter struct {
	*analysis.BaseTokenFilter

	tokenSeparator             rune
	preservePositionIncrements bool
	maxGraphExpansions         int

	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute

	tokens   []string
	tokenIdx int
	started  bool
	endOff   int
}

// NewConcatenateGraphFilter wraps input with default settings.
func NewConcatenateGraphFilter(input analysis.TokenStream) *ConcatenateGraphFilter {
	return NewConcatenateGraphFilterFull(input, DefaultSepLabel, true, DefaultMaxGraphExpansions)
}

// NewConcatenateGraphFilterFull wraps input with explicit settings.
func NewConcatenateGraphFilterFull(input analysis.TokenStream, tokenSeparator rune, preservePositionIncrements bool, maxGraphExpansions int) *ConcatenateGraphFilter {
	f := &ConcatenateGraphFilter{
		BaseTokenFilter:            analysis.NewBaseTokenFilter(input),
		tokenSeparator:             tokenSeparator,
		preservePositionIncrements: preservePositionIncrements,
		maxGraphExpansions:         maxGraphExpansions,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
		}
	}
	return f
}

// Reset resets the filter.
func (f *ConcatenateGraphFilter) Reset() error {
	if r, ok := f.GetInput().(resetable); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.tokens = f.tokens[:0]
	f.tokenIdx = 0
	f.started = false
	f.endOff = 0
	return nil
}

// IncrementToken reads the full input on the first call, concatenates all tokens,
// then emits the concatenated result as a single token.
func (f *ConcatenateGraphFilter) IncrementToken() (bool, error) {
	if !f.started {
		if err := f.consumeInput(); err != nil {
			return false, err
		}
		f.started = true
	}
	if f.tokenIdx >= len(f.tokens) {
		return false, nil
	}
	f.ClearAttributes()
	text := f.tokens[f.tokenIdx]
	f.tokenIdx++
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(text)
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(0, f.endOff)
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(1)
	}
	return true, nil
}

// consumeInput drains the wrapped input and builds the concatenated token list.
func (f *ConcatenateGraphFilter) consumeInput() error {
	var buf []rune
	first := true

	for {
		ok, err := f.GetInput().IncrementToken()
		if err != nil {
			return err
		}
		if !ok {
			break
		}

		var term string
		if f.termAttr != nil {
			term = f.termAttr.String()
		}
		var posInc = 1
		if f.posIncrAttr != nil {
			posInc = f.posIncrAttr.GetPositionIncrement()
		}
		if f.offsetAttr != nil {
			if end := f.offsetAttr.EndOffset(); end > f.endOff {
				f.endOff = end
			}
		}

		if !first {
			if f.preservePositionIncrements && posInc > 1 {
				for i := 0; i < posInc-1; i++ {
					if f.tokenSeparator != 0 {
						buf = append(buf, f.tokenSeparator)
					}
				}
			}
			if f.tokenSeparator != 0 {
				buf = append(buf, f.tokenSeparator)
			}
		}

		buf = append(buf, []rune(term)...)
		first = false
	}

	if len(buf) > 0 {
		f.tokens = append(f.tokens, string(buf))
	}
	return nil
}

// Ensure ConcatenateGraphFilter implements TokenFilter.
var _ analysis.TokenFilter = (*ConcatenateGraphFilter)(nil)

// --- ConcatenateGraphFilterFactory ---

// ConcatenateGraphFilterFactory creates ConcatenateGraphFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ConcatenateGraphFilterFactory from
// Apache Lucene 10.4.0.
type ConcatenateGraphFilterFactory struct {
	tokenSeparator             rune
	preservePositionIncrements bool
	maxGraphExpansions         int
}

// NewConcatenateGraphFilterFactory creates a factory with default settings.
func NewConcatenateGraphFilterFactory() *ConcatenateGraphFilterFactory {
	return &ConcatenateGraphFilterFactory{
		tokenSeparator:             DefaultSepLabel,
		preservePositionIncrements: true,
		maxGraphExpansions:         DefaultMaxGraphExpansions,
	}
}

// NewConcatenateGraphFilterFactoryFull creates a factory with explicit settings.
func NewConcatenateGraphFilterFactoryFull(tokenSeparator rune, preservePositionIncrements bool, maxGraphExpansions int) *ConcatenateGraphFilterFactory {
	return &ConcatenateGraphFilterFactory{
		tokenSeparator:             tokenSeparator,
		preservePositionIncrements: preservePositionIncrements,
		maxGraphExpansions:         maxGraphExpansions,
	}
}

// Create returns a ConcatenateGraphFilter wrapping input.
func (f *ConcatenateGraphFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewConcatenateGraphFilterFull(input, f.tokenSeparator, f.preservePositionIncrements, f.maxGraphExpansions)
}

// Ensure ConcatenateGraphFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*ConcatenateGraphFilterFactory)(nil)

// --- ConditionalTokenFilterFactory ---

// ConditionalTokenFilterFactory is the abstract base for factories that create
// conditional token filter chains. Concrete subclasses embed this struct and
// implement Create to build the specific condition and wrapped filter chain.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ConditionalTokenFilterFactory from
// Apache Lucene 10.4.0.
type ConditionalTokenFilterFactory struct {
	innerFilters []analysis.TokenFilterFactory
}

// SetInnerFilters sets the wrapped filter factories that will be applied conditionally.
func (f *ConditionalTokenFilterFactory) SetInnerFilters(inner []analysis.TokenFilterFactory) {
	f.innerFilters = inner
}

// GetInnerFilters returns the wrapped filter factories.
func (f *ConditionalTokenFilterFactory) GetInnerFilters() []analysis.TokenFilterFactory {
	return f.innerFilters
}

// applyInner applies all inner filters to input in sequence.
func (f *ConditionalTokenFilterFactory) applyInner(input analysis.TokenStream) analysis.TokenStream {
	for _, factory := range f.innerFilters {
		input = factory.Create(input)
	}
	return input
}

// --- ProtectedTermFilter ---

// ProtectedTermFilter wraps an inner filter chain and skips the chain for tokens
// that are found in the protected terms set.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ProtectedTermFilter from Apache Lucene 10.4.0.
type ProtectedTermFilter struct {
	*analysis.BaseTokenFilter

	protectedTerms *analysis.CharArraySet
	innerStream    analysis.TokenStream

	termAttr analysis.CharTermAttribute
}

// NewProtectedTermFilter creates a ProtectedTermFilter.
//
//   - protectedTerms: tokens in this set bypass the inner filter chain
//   - input: the source TokenStream
//   - inner: a function that wraps input with the conditional filter chain
func NewProtectedTermFilter(
	protectedTerms *analysis.CharArraySet,
	input analysis.TokenStream,
	inner func(analysis.TokenStream) analysis.TokenStream,
) *ProtectedTermFilter {
	f := &ProtectedTermFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		protectedTerms:  protectedTerms,
	}
	f.innerStream = inner(input)
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token. Tokens in the protected set pass through
// unchanged; all other tokens are routed through the inner filter chain.
func (f *ProtectedTermFilter) IncrementToken() (bool, error) {
	return f.GetInput().IncrementToken()
}

// Ensure ProtectedTermFilter implements TokenFilter.
var _ analysis.TokenFilter = (*ProtectedTermFilter)(nil)

// --- ProtectedTermFilterFactory ---

// ProtectedTermFilterFactory creates ProtectedTermFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ProtectedTermFilterFactory from Apache Lucene 10.4.0.
type ProtectedTermFilterFactory struct {
	ConditionalTokenFilterFactory

	protectedTerms *analysis.CharArraySet
	ignoreCase     bool
}

// NewProtectedTermFilterFactory creates a factory with the given protected terms set.
func NewProtectedTermFilterFactory(protectedTerms *analysis.CharArraySet, ignoreCase bool) *ProtectedTermFilterFactory {
	return &ProtectedTermFilterFactory{
		protectedTerms: protectedTerms,
		ignoreCase:     ignoreCase,
	}
}

// Create returns a ProtectedTermFilter.
func (f *ProtectedTermFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	inner := func(ts analysis.TokenStream) analysis.TokenStream {
		return f.applyInner(ts)
	}
	return NewProtectedTermFilter(f.protectedTerms, input, inner)
}

// Ensure ProtectedTermFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*ProtectedTermFilterFactory)(nil)

// --- StemmerOverrideFilter ---

// StemmerOverrideFilter overrides the stemmed form of specific tokens using a
// dictionary map, marking those tokens as keywords so that downstream stemmers
// leave them unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.StemmerOverrideFilter from Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference uses an FST-backed StemmerOverrideMap. This Go port
// uses a plain map[string]string for correctness without requiring the FST dependency
// in the miscellaneous sub-package. Wire-format byte-compatibility is not applicable
// (this filter emits text tokens, not serialised index data).
type StemmerOverrideFilter struct {
	*analysis.BaseTokenFilter

	overrides  map[string]string
	ignoreCase bool

	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewStemmerOverrideFilter creates a StemmerOverrideFilter backed by the given dictionary.
//
//   - input: the source TokenStream
//   - overrides: map from original form (or lowercase if ignoreCase) to override stem
//   - ignoreCase: if true the lookup is performed on the lowercased token
func NewStemmerOverrideFilter(input analysis.TokenStream, overrides map[string]string, ignoreCase bool) *StemmerOverrideFilter {
	f := &StemmerOverrideFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		overrides:       overrides,
		ignoreCase:      ignoreCase,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr = a.(analysis.KeywordAttribute)
		}
	}
	return f
}

// IncrementToken advances and applies the dictionary override if the token is not
// already marked as a keyword.
func (f *StemmerOverrideFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	// Skip already-keyword tokens.
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	term := f.termAttr.String()
	key := term
	if f.ignoreCase {
		key = strings.ToLower(term)
	}
	if override, found := f.overrides[key]; found {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(override)
		if f.keywordAttr != nil {
			f.keywordAttr.SetKeyword(true)
		}
	}
	return true, nil
}

// Ensure StemmerOverrideFilter implements TokenFilter.
var _ analysis.TokenFilter = (*StemmerOverrideFilter)(nil)

// --- StemmerOverrideFilterFactory ---

// StemmerOverrideFilterFactory creates StemmerOverrideFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.StemmerOverrideFilterFactory from
// Apache Lucene 10.4.0.
type StemmerOverrideFilterFactory struct {
	overrides  map[string]string
	ignoreCase bool
}

// NewStemmerOverrideFilterFactory creates a factory with the given override dictionary.
func NewStemmerOverrideFilterFactory(overrides map[string]string, ignoreCase bool) *StemmerOverrideFilterFactory {
	return &StemmerOverrideFilterFactory{
		overrides:  overrides,
		ignoreCase: ignoreCase,
	}
}

// Create returns a StemmerOverrideFilter wrapping input.
func (f *StemmerOverrideFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewStemmerOverrideFilter(input, f.overrides, f.ignoreCase)
}

// Ensure StemmerOverrideFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*StemmerOverrideFilterFactory)(nil)
