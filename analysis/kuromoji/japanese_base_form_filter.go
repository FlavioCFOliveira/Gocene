// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/tokenattributes"
)

// JapaneseBaseFormFilter replaces the term text with the base form of the
// token as reported by BaseFormAttribute. This acts as a lemmatizer for
// verbs and adjectives.
//
// Tokens marked with KeywordAttribute are left unchanged.
//
// This is the Go port of org.apache.lucene.analysis.ja.JapaneseBaseFormFilter
// from Apache Lucene 10.4.0.
type JapaneseBaseFormFilter struct {
	*analysis.BaseTokenFilter
	termAttr      analysis.CharTermAttribute
	baseFormAttr  tokenattributes.BaseFormAttribute
	keywordAttr   analysis.KeywordAttribute
}

// NewJapaneseBaseFormFilter creates a new JapaneseBaseFormFilter wrapping input.
func NewJapaneseBaseFormFilter(input analysis.TokenStream) *JapaneseBaseFormFilter {
	f := &JapaneseBaseFormFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(tokenattributes.BaseFormAttributeType); a != nil {
			f.baseFormAttr = a.(tokenattributes.BaseFormAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr = a.(analysis.KeywordAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token, optionally replacing the term
// with its base form.
func (f *JapaneseBaseFormFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	isKeyword := f.keywordAttr != nil && f.keywordAttr.IsKeywordToken()
	if !isKeyword && f.baseFormAttr != nil && f.termAttr != nil {
		baseForm := f.baseFormAttr.BaseForm()
		if baseForm != "" {
			f.termAttr.SetValue(baseForm)
		}
	}
	return true, nil
}

// Ensure JapaneseBaseFormFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseBaseFormFilter)(nil)
