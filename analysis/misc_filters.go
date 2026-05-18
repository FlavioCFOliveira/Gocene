// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"
)

// EmptyTokenStream is a TokenStream that produces no tokens. Useful
// as a placeholder in pipelines where a token source is required but
// the upstream caller has no input.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.EmptyTokenStream from
// Apache Lucene 10.4.0.
type EmptyTokenStream struct {
	*BaseTokenStream
}

// NewEmptyTokenStream returns an empty token stream.
func NewEmptyTokenStream() *EmptyTokenStream {
	return &EmptyTokenStream{
		BaseTokenStream: NewBaseTokenStream(),
	}
}

// IncrementToken always returns false.
func (s *EmptyTokenStream) IncrementToken() (bool, error) {
	return false, nil
}

// End performs end-of-stream operations.
func (s *EmptyTokenStream) End() error {
	return nil
}

// Close releases resources held by this TokenStream.
func (s *EmptyTokenStream) Close() error {
	return nil
}

// Ensure EmptyTokenStream implements TokenStream.
var _ TokenStream = (*EmptyTokenStream)(nil)

// --- CodepointCountFilter ---

// CodepointCountFilter removes tokens whose codepoint count falls
// outside [min, max] inclusive.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.CodepointCountFilter from
// Apache Lucene 10.4.0.
type CodepointCountFilter struct {
	*BaseTokenFilter

	min, max int
	termAttr CharTermAttribute
}

// NewCodepointCountFilter wraps input with a min/max codepoint count
// filter. Returns an error if min < 0 or min > max.
func NewCodepointCountFilter(input TokenStream, min, max int) (*CodepointCountFilter, error) {
	if min < 0 {
		return nil, errors.New("CodepointCountFilter: min must be non-negative")
	}
	if min > max {
		return nil, errors.New("CodepointCountFilter: min must be <= max")
	}
	f := &CodepointCountFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		min:             min,
		max:             max,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f, nil
}

// IncrementToken drops tokens whose codepoint count is outside the
// configured range.
func (f *CodepointCountFilter) IncrementToken() (bool, error) {
	for {
		ok, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		if f.termAttr == nil {
			return true, nil
		}
		// Fast bound: UTF-8 byte length is between 1 and 4 times the
		// codepoint count. If max <= byte_len/4 - 1 or min >= byte_len -
		// 1 we can short-circuit by counting runes only when needed.
		count := utf8.RuneCountInString(f.termAttr.String())
		if count >= f.min && count <= f.max {
			return true, nil
		}
	}
}

// Ensure CodepointCountFilter implements TokenFilter.
var _ TokenFilter = (*CodepointCountFilter)(nil)

// CodepointCountFilterFactory creates CodepointCountFilter instances.
type CodepointCountFilterFactory struct {
	min, max int
}

// NewCodepointCountFilterFactory returns a factory.
func NewCodepointCountFilterFactory(min, max int) *CodepointCountFilterFactory {
	return &CodepointCountFilterFactory{min: min, max: max}
}

// Create wraps input; invalid configuration is reported by panicking,
// since the factory contract has no error channel and the validation
// duplicates the filter constructor.
func (f *CodepointCountFilterFactory) Create(input TokenStream) TokenFilter {
	c, err := NewCodepointCountFilter(input, f.min, f.max)
	if err != nil {
		panic(err)
	}
	return c
}

// --- FixBrokenOffsetsFilter (deprecated upstream, ported for parity) ---

// FixBrokenOffsetsFilter corrects offsets that illegally go
// backwards. Lucene marks this @Deprecated; we port it for parity
// with the upstream class set.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.FixBrokenOffsetsFilter.
type FixBrokenOffsetsFilter struct {
	*BaseTokenFilter

	lastStartOffset int
	offsetAttr      OffsetAttribute
}

// NewFixBrokenOffsetsFilter wraps input.
func NewFixBrokenOffsetsFilter(input TokenStream) *FixBrokenOffsetsFilter {
	f := &FixBrokenOffsetsFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); a != nil {
			f.offsetAttr = a.(OffsetAttribute)
		}
	}
	return f
}

// IncrementToken propagates the input token and rewrites broken
// offsets in place.
func (f *FixBrokenOffsetsFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	f.fixOffsets()
	return true, nil
}

// End forwards the End() call and re-applies the offset correction.
func (f *FixBrokenOffsetsFilter) End() error {
	if err := f.input.End(); err != nil {
		return err
	}
	f.fixOffsets()
	return nil
}

func (f *FixBrokenOffsetsFilter) fixOffsets() {
	if f.offsetAttr == nil {
		return
	}
	start := f.offsetAttr.StartOffset()
	end := f.offsetAttr.EndOffset()
	if start < f.lastStartOffset {
		start = f.lastStartOffset
	}
	if end < start {
		end = start
	}
	f.offsetAttr.SetOffset(start, end)
	f.lastStartOffset = start
}

// Ensure FixBrokenOffsetsFilter implements TokenFilter.
var _ TokenFilter = (*FixBrokenOffsetsFilter)(nil)

// FixBrokenOffsetsFilterFactory creates instances.
type FixBrokenOffsetsFilterFactory struct{}

// NewFixBrokenOffsetsFilterFactory returns a fresh factory.
func NewFixBrokenOffsetsFilterFactory() *FixBrokenOffsetsFilterFactory {
	return &FixBrokenOffsetsFilterFactory{}
}

// Create wraps input.
func (f *FixBrokenOffsetsFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFixBrokenOffsetsFilter(input)
}

// --- KeywordMarkerFilter (abstract base) and SetKeywordMarkerFilter ---

// KeywordMarkerFilter is the base struct embedded by every keyword
// marker filter. Subclasses provide an IsKeywordFunc that decides
// whether the current token should be marked as a keyword.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.KeywordMarkerFilter from
// Apache Lucene 10.4.0.
type KeywordMarkerFilter struct {
	*BaseTokenFilter

	IsKeywordFunc func() bool
	termAttr      CharTermAttribute
	keywordAttr   KeywordAttribute
}

// NewKeywordMarkerFilter wraps input with a marker filter that calls
// isKeyword on every token; when isKeyword returns true the
// KeywordAttribute is set.
func NewKeywordMarkerFilter(input TokenStream, isKeyword func() bool) *KeywordMarkerFilter {
	f := &KeywordMarkerFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		IsKeywordFunc:   isKeyword,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(KeywordAttributeType); a != nil {
			f.keywordAttr = a.(KeywordAttribute)
		}
	}
	return f
}

// IncrementToken pulls the next token and applies the keyword test.
func (f *KeywordMarkerFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.IsKeywordFunc != nil && f.keywordAttr != nil && f.IsKeywordFunc() {
		f.keywordAttr.SetKeyword(true)
	}
	return true, nil
}

// Ensure KeywordMarkerFilter implements TokenFilter.
var _ TokenFilter = (*KeywordMarkerFilter)(nil)

// SetKeywordMarkerFilter marks every token whose text appears in the
// provided CharArraySet as a keyword.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.SetKeywordMarkerFilter.
type SetKeywordMarkerFilter struct {
	*KeywordMarkerFilter
	keywordSet *CharArraySet
}

// NewSetKeywordMarkerFilter wraps input with a set-based marker.
// keywordSet must not be nil.
func NewSetKeywordMarkerFilter(input TokenStream, keywordSet *CharArraySet) *SetKeywordMarkerFilter {
	if keywordSet == nil {
		panic("SetKeywordMarkerFilter: keywordSet must not be nil")
	}
	s := &SetKeywordMarkerFilter{keywordSet: keywordSet}
	s.KeywordMarkerFilter = NewKeywordMarkerFilter(input, s.isKeyword)
	return s
}

func (s *SetKeywordMarkerFilter) isKeyword() bool {
	if s.termAttr == nil {
		return false
	}
	return s.keywordSet.ContainsString(s.termAttr.String())
}

// Ensure SetKeywordMarkerFilter implements TokenFilter.
var _ TokenFilter = (*SetKeywordMarkerFilter)(nil)

// KeywordMarkerFilterFactory is currently a marker factory that
// expects subclasses to provide the matching logic; the Lucene
// reference is also abstract.
type KeywordMarkerFilterFactory struct{}

// NewKeywordMarkerFilterFactory returns a fresh factory placeholder.
func NewKeywordMarkerFilterFactory() *KeywordMarkerFilterFactory {
	return &KeywordMarkerFilterFactory{}
}

// --- TypeAsSynonymFilter ---

// TypeAsSynonymFilter emits the current token's TypeAttribute as a
// secondary synonym token at the same position. Optionally a prefix
// is prepended, a set of types may be ignored, and the FlagsAttribute
// of the synonym can be masked.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.TypeAsSynonymFilter.
type TypeAsSynonymFilter struct {
	*BaseTokenFilter

	prefix       string
	ignore       map[string]struct{}
	synFlagsMask int

	saved bool // there is a saved state pending re-emission
	// saved state: the relevant attribute values captured at the
	// previous IncrementToken call.
	savedTerm    string
	savedType    string
	savedFlags   int
	savedOffsets [2]int
	savedPosLen  int

	termAttr    CharTermAttribute
	typeAttr    TypeAttribute
	posIncrAttr PositionIncrementAttribute
	flagsAttr   FlagsAttribute
}

// NewTypeAsSynonymFilter wraps input with default settings (no
// prefix, ignore nothing, mask = all bits).
func NewTypeAsSynonymFilter(input TokenStream) *TypeAsSynonymFilter {
	return NewTypeAsSynonymFilterWithConfig(input, "", nil, ^0)
}

// NewTypeAsSynonymFilterWithConfig wraps input with the full config.
func NewTypeAsSynonymFilterWithConfig(input TokenStream, prefix string, ignore []string, synFlagsMask int) *TypeAsSynonymFilter {
	ig := make(map[string]struct{}, len(ignore))
	for _, t := range ignore {
		ig[t] = struct{}{}
	}
	f := &TypeAsSynonymFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		prefix:          prefix,
		ignore:          ig,
		synFlagsMask:    synFlagsMask,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(TypeAttributeType); a != nil {
			f.typeAttr = a.(TypeAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); a != nil {
			f.posIncrAttr = a.(PositionIncrementAttribute)
		}
		if a := src.GetAttributeByType(FlagsAttributeType); a != nil {
			f.flagsAttr = a.(FlagsAttribute)
		}
	}
	return f
}

// IncrementToken emits the synonym (if any) after the original token,
// at position increment 0.
func (f *TypeAsSynonymFilter) IncrementToken() (bool, error) {
	if f.saved {
		// Emit the synonym
		f.saved = false
		if f.termAttr != nil {
			f.termAttr.SetEmpty()
			if f.prefix != "" {
				f.termAttr.AppendString(f.prefix)
			}
			f.termAttr.AppendString(f.savedType)
		}
		if f.posIncrAttr != nil {
			f.posIncrAttr.SetPositionIncrement(0)
		}
		if f.typeAttr != nil {
			f.typeAttr.SetType(f.savedType)
		}
		if f.flagsAttr != nil {
			f.flagsAttr.SetFlags(f.savedFlags & f.synFlagsMask)
		}
		return true, nil
	}
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.typeAttr != nil {
		t := f.typeAttr.GetType()
		if _, ignored := f.ignore[t]; !ignored {
			f.saved = true
			f.savedType = t
			if f.flagsAttr != nil {
				f.savedFlags = f.flagsAttr.GetFlags()
			}
		}
	}
	return true, nil
}

// Reset clears any saved state.
func (f *TypeAsSynonymFilter) Reset() error {
	if r, ok := f.input.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.saved = false
	return nil
}

// Ensure TypeAsSynonymFilter implements TokenFilter.
var _ TokenFilter = (*TypeAsSynonymFilter)(nil)

// TypeAsSynonymFilterFactory creates TypeAsSynonymFilter instances.
type TypeAsSynonymFilterFactory struct {
	prefix string
	ignore []string
	mask   int
}

// NewTypeAsSynonymFilterFactory returns a factory using defaults.
func NewTypeAsSynonymFilterFactory() *TypeAsSynonymFilterFactory {
	return &TypeAsSynonymFilterFactory{mask: ^0}
}

// NewTypeAsSynonymFilterFactoryWithConfig returns a factory using
// the given prefix, ignore set, and flag mask.
func NewTypeAsSynonymFilterFactoryWithConfig(prefix string, ignore []string, mask int) *TypeAsSynonymFilterFactory {
	return &TypeAsSynonymFilterFactory{prefix: prefix, ignore: ignore, mask: mask}
}

// Create wraps input.
func (f *TypeAsSynonymFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTypeAsSynonymFilterWithConfig(input, f.prefix, f.ignore, f.mask)
}

// helper to strip a known suffix (used by some downstream filters in
// the misc package).
func trimSuffix(s, suf string) string {
	return strings.TrimSuffix(s, suf)
}

// Compile-time guard against unused-import warnings when downstream
// edits remove trimSuffix; it remains exported through the package
// for shared use.
var _ = trimSuffix
