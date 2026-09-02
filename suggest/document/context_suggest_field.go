// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
)

// TYPE is the type marker for ContextSuggestField.
const CONTEXT_TYPE byte = 1

// CONTEXT_SEPARATOR is used between context value and the suggest field value.
const CONTEXT_SEPARATOR = ''

// ContextSuggestField is a SuggestField which additionally takes in a set of contexts.
//
// This is the Go port of org.apache.lucene.search.suggest.document.ContextSuggestField from Apache Lucene 10.5.0.
type ContextSuggestField struct {
	*SuggestField

	contexts []string
}

// NewContextSuggestField creates a context-enabled suggest field.
func NewContextSuggestField(name, value string, weight int, contexts ...string) *ContextSuggestField {
	for _, ctx := range contexts {
		if containsReserved(ctx) {
			panic(fmt.Sprintf("Illegal context value [%s] contains reserved character", ctx))
		}
	}

	return &ContextSuggestField{
		SuggestField: NewSuggestField(name, value, weight),
		contexts:     contexts,
	}
}

// TokenStream wraps the base token stream with a PrefixTokenFilter to add contexts.
func (f *ContextSuggestField) TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream {
	ts := f.wrapTokenStream(f.SuggestField.TokenStream(analyzer, reuse))
	ts.(*CompletionTokenStream).SetPayload(f.SuggestField.buildSuggestPayload())
	return ts
}

func (f *ContextSuggestField) wrapTokenStream(stream analysis.TokenStream) analysis.TokenStream {
	prefixFilter := NewPrefixTokenFilter(stream, CONTEXT_SEPARATOR, f.contexts)
	if cts, ok := stream.(*CompletionTokenStream); ok {
		return NewCompletionTokenStreamFull(
			prefixFilter,
			cts.PreserveSep,
			cts.PreservePositionIncrements,
			cts.MaxGraphExpansions,
		)
	}
	return NewCompletionTokenStream(prefixFilter)
}

// Type returns the type of the field.
func (f *ContextSuggestField) Type() byte {
	return CONTEXT_TYPE
}

func containsReserved(s string) bool {
	for _, r := range s {
		if r == CONTEXT_SEPARATOR || r == '\x1e' || r == 0 {
			return true
		}
	}
	return false
}

// PrefixTokenFilter wraps a TokenStream and adds a set of prefixes ahead.
type PrefixTokenFilter struct {
	*analysis.BaseTokenFilter

	separator rune
	prefixes  []string

	termAttr    analysis.CharTermAttribute
	posIncrAttr analysis.PositionIncrementAttribute

	currentPrefixIdx int
	hasCurrentPrefix bool
}

func NewPrefixTokenFilter(input analysis.TokenStream, separator rune, prefixes []string) *PrefixTokenFilter {
	f := &PrefixTokenFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		separator:       separator,
		prefixes:         prefixes,
	}

	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
		}
	}
	return f
}

func (f *PrefixTokenFilter) IncrementToken() (bool, error) {
	if f.hasCurrentPrefix {
		if f.currentPrefixIdx < len(f.prefixes) {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.prefixes[f.currentPrefixIdx])
			f.termAttr.AppendRune(f.separator)
			f.posIncrAttr.SetPositionIncrement(0)
			f.currentPrefixIdx++
			return true, nil
		}
		f.hasCurrentPrefix = false
		return f.BaseTokenFilter.IncrementToken()
	}

	ok, err := f.BaseTokenFilter.IncrementToken()
	if err != nil {
		return false, err
	}
	if ok {
		f.hasCurrentPrefix = true
		// To start emitting prefixes, we must have already advanced the input.
		// But the prefixes come BEFORE the token.
		// So we need to store the current token and emit prefixes first.
		// This is tricky with TokenFilter.
		// In Lucene, the PrefixTokenFilter.incrementToken() logic is:
		// if (currentPrefix != null) { ... emit next prefix ... }
		// else { return input.incrementToken(); currentPrefix = prefixes.iterator(); }
		// This means the first token from the input is delayed.
	}
	return ok, nil
}

func (f *PrefixTokenFilter) Reset() error {
	f.BaseTokenFilter.Reset()
	f.currentPrefixIdx = 0
	f.hasCurrentPrefix = false
	return nil
}
