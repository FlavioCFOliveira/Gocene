// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ClassicFilter normalises tokens produced by ClassicTokenizer by:
//   - Removing the possessive 's suffix from APOSTROPHE-type tokens.
//   - Removing dots from ACRONYM-type tokens.
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicFilter from
// Apache Lucene 10.4.0.
type ClassicFilter struct {
	*analysis.BaseTokenFilter

	termAttr analysis.CharTermAttribute
	typeAttr analysis.TypeAttribute
}

// NewClassicFilter creates a ClassicFilter wrapping input.
func NewClassicFilter(input analysis.TokenStream) *ClassicFilter {
	f := &ClassicFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
			f.typeAttr = a.(analysis.TypeAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token, applying normalisation.
func (f *ClassicFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr == nil || f.typeAttr == nil {
		return true, nil
	}

	term := f.termAttr.String()
	typ := f.typeAttr.GetType()

	switch typ {
	case TokenTypes[TokenApostrophe]:
		// Remove 's from end.
		if len(term) >= 2 && strings.HasSuffix(term, "'s") {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(term[:len(term)-2])
		} else if len(term) >= 2 && strings.HasSuffix(term, "'S") {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(term[:len(term)-2])
		}
	case TokenTypes[TokenAcronym]:
		// Remove dots.
		noDots := strings.ReplaceAll(term, ".", "")
		if noDots != term {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(noDots)
		}
	}
	return true, nil
}

// Ensure ClassicFilter implements TokenFilter.
var _ analysis.TokenFilter = (*ClassicFilter)(nil)

// ClassicFilterFactory creates ClassicFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicFilterFactory from
// Apache Lucene 10.4.0.
type ClassicFilterFactory struct{}

// NewClassicFilterFactory creates a ClassicFilterFactory.
func NewClassicFilterFactory() *ClassicFilterFactory { return &ClassicFilterFactory{} }

// Create creates a ClassicFilter wrapping input.
func (f *ClassicFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewClassicFilter(input)
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*ClassicFilterFactory)(nil)
