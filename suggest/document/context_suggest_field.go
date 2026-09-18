// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/miscellaneous"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)

// CONTEXT_SEPARATOR is the separator used between context value and the suggest field value.
const CONTEXT_SEPARATOR = ''

// ContextSuggestFieldTYPE is the byte marker stamped on a context-enabled
// suggest field. Mirrors
// org.apache.lucene.search.suggest.document.ContextSuggestField.TYPE
// (ContextSuggestField.java:51), which Java writes as ContextSuggestField.TYPE
// at every use site. See SuggestFieldTYPE for the naming rationale.
const ContextSuggestFieldTYPE byte = 1

// ContextSuggestField is a SuggestField which additionally takes in a set of contexts.
type ContextSuggestField struct {
	*SuggestField

	// contextSet holds the associated contexts. Java declares this field as
	// `private final Set<CharSequence> contexts` alongside the protected
	// accessor `contexts()` (ContextSuggestField.java:53 and :74). Go forbids a
	// field and a method of the same name on one type, so the private field is
	// the one renamed and the accessor keeps the Java name, which is the member
	// subclasses actually override.
	contextSet []string
}

// NewContextSuggestField creates a context-enabled suggest field.
func NewContextSuggestField(name, value string, weight int, contexts ...string) *ContextSuggestField {
	sf := NewSuggestField(name, value, weight)
	for _, ctx := range contexts {
		if err := validate(ctx); err != nil {
			panic(err)
		}
	}

	return &ContextSuggestField{
		SuggestField: sf,
		contextSet:   contexts,
	}
}

// contexts lets sub-classes inject contexts at index time. Mirrors
// ContextSuggestField.contexts() (ContextSuggestField.java:74).
func (f *ContextSuggestField) contexts() []string {
	return f.contextSet
}

func (f *ContextSuggestField) TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream {
	ts := f.wrapTokenStream(f.Field.TokenStream(analyzer, reuse))
	cts := ts.(*CompletionTokenStream)
	cts.SetPayload(f.buildSuggestPayload())
	return ts
}

func (f *ContextSuggestField) wrapTokenStream(stream analysis.TokenStream) analysis.TokenStream {
	ctxs := f.contexts()
	prefixFilter := NewPrefixTokenFilter(stream, CONTEXT_SEPARATOR, ctxs)

	return NewCompletionTokenStreamFull(
		prefixFilter,
		true,
		true,
		miscellaneous.DefaultMaxGraphExpansions,
	)
}

func (f *ContextSuggestField) Type() byte {
	return ContextSuggestFieldTYPE
}

func validate(value string) error {
	for i, r := range value {
		if r == CONTEXT_SEPARATOR {
			return fmt.Errorf("Illegal value [%s] reserved character 0x%X at position %d", value, r, i)
		}
	}
	return nil
}

// PrefixTokenFilter wraps a TokenStream and adds a set prefixes ahead.
type PrefixTokenFilter struct {
	*analysis.BaseTokenStream

	input         analysis.TokenStream
	separator     rune
	prefixes      []string
	currentPrefix int
}

func NewPrefixTokenFilter(input analysis.TokenStream, separator rune, prefixes []string) *PrefixTokenFilter {
	return &PrefixTokenFilter{
		BaseTokenStream: analysis.NewBaseTokenStream(),
		input:           input,
		separator:       separator,
		prefixes:        prefixes,
		currentPrefix:   -1,
	}
}

func (f *PrefixTokenFilter) IncrementToken() (bool, error) {
	if f.currentPrefix == -1 {
		f.currentPrefix = 0
	}
	if f.currentPrefix < len(f.prefixes) {
		term := f.prefixes[f.currentPrefix]
		f.GetAttribute(analysis.CharTermAttributeType).(*analysis.CharTermAttribute).AppendString(term + string(f.separator))
		if f.currentPrefix == 0 {
			f.GetAttribute(tokenattributes.PositionIncrementAttributeType).(*tokenattributes.PositionIncrementAttribute).SetPositionIncrement(1)
		} else {
			f.GetAttribute(tokenattributes.PositionIncrementAttributeType).(*tokenattributes.PositionIncrementAttribute).SetPositionIncrement(0)
		}
		f.currentPrefix++
		return true, nil
	}
	return f.input.IncrementToken()
}

func (f *PrefixTokenFilter) Reset() error {
	f.BaseTokenStream.Reset()
	f.currentPrefix = -1
	return f.input.Reset()
}
