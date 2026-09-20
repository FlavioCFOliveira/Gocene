// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
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
	if err := validate(value); err != nil {
		panic(err)
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
	for _, ctx := range ctxs {
		if err := validate(ctx); err != nil {
			panic(err)
		}
	}
	if cts, ok := stream.(*CompletionTokenStream); ok {
		prefixTokenFilter := NewPrefixTokenFilter(cts.inputTokenStream, CONTEXT_SEPARATOR, ctxs)
		return NewCompletionTokenStreamFull(
			prefixTokenFilter,
			cts.PreserveSep,
			cts.PreservePositionIncrements,
			cts.MaxGraphExpansions,
		)
	}
	return NewCompletionTokenStream(NewPrefixTokenFilter(stream, CONTEXT_SEPARATOR, ctxs))
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

// PrefixTokenFilter wraps a TokenStream and adds a set prefixes ahead. The
// position attribute will not be incremented for the prefixes.
//
// Mirrors ContextSuggestField.PrefixTokenFilter of Apache Lucene 10.5.0.
type PrefixTokenFilter struct {
	*analysis.BaseTokenFilter

	separator byte
	termAttr  analysis.CharTermAttribute
	posAttr   tokenattributes.PositionIncrementAttribute
	prefixes  []string

	// currentPrefixIdx / currentPrefixSet render Java's
	// `Iterator<CharSequence> currentPrefix`: a nil iterator is
	// currentPrefixSet == false, and hasNext() is
	// currentPrefixIdx < len(prefixes).
	currentPrefixIdx int
	currentPrefixSet bool
}

// NewPrefixTokenFilter creates a new PrefixTokenFilter.
func NewPrefixTokenFilter(input analysis.TokenStream, separator byte, prefixes []string) *PrefixTokenFilter {
	f := &PrefixTokenFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		separator:       separator,
		prefixes:        prefixes,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(tokenattributes.PositionIncrementAttributeType); a != nil {
			f.posAttr = a.(tokenattributes.PositionIncrementAttribute)
		}
	}
	return f
}

// IncrementToken emits every configured prefix, each followed by the
// separator, before delegating to the wrapped stream.
func (f *PrefixTokenFilter) IncrementToken() (bool, error) {
	if f.currentPrefixSet {
		if f.currentPrefixIdx >= len(f.prefixes) {
			return f.GetInput().IncrementToken()
		}
		f.posAttr.SetPositionIncrement(0)
	} else {
		f.currentPrefixSet = true
		f.currentPrefixIdx = 0
		f.termAttr.SetEmpty()
		f.posAttr.SetPositionIncrement(1)
	}
	f.termAttr.SetEmpty()
	if f.currentPrefixIdx < len(f.prefixes) {
		f.termAttr.AppendString(f.prefixes[f.currentPrefixIdx])
		f.currentPrefixIdx++
	}
	f.termAttr.AppendChar(f.separator)
	return true, nil
}

// Reset restarts the prefix enumeration.
func (f *PrefixTokenFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.currentPrefixIdx = 0
	f.currentPrefixSet = false
	return nil
}

var _ analysis.TokenFilter = (*PrefixTokenFilter)(nil)
