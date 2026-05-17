// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// ElisionFilter removes elided particles (e.g. "l'", "qu'", "d'") at
// the head of each token when the prefix is present in the configured
// articles set.
//
// This is the Go port of
// org.apache.lucene.analysis.util.ElisionFilter from Apache Lucene
// 10.4.0.
//
// The articles set is required (passing nil panics). Apostrophe
// variants recognised: ASCII U+0027 and U+2019.
type ElisionFilter struct {
	*BaseTokenFilter

	articles *CharArraySet
	termAttr CharTermAttribute
}

// NewElisionFilter wraps input with the elision filter using the
// given articles set.
func NewElisionFilter(input TokenStream, articles *CharArraySet) *ElisionFilter {
	if articles == nil {
		panic("ElisionFilter: articles must not be nil")
	}
	f := &ElisionFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		articles:        articles,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken strips a leading article + apostrophe prefix when
// the article appears in the configured set.
func (f *ElisionFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	runes := []rune(f.termAttr.String())
	index := -1
	for i, r := range runes {
		if r == '\'' || r == 0x2019 {
			index = i
			break
		}
	}
	if index >= 0 && f.articles.ContainsString(string(runes[:index])) {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(string(runes[index+1:]))
	}
	return true, nil
}

// Ensure ElisionFilter implements TokenFilter.
var _ TokenFilter = (*ElisionFilter)(nil)

// ElisionFilterFactory creates ElisionFilter instances. The
// articles set is required.
type ElisionFilterFactory struct {
	articles *CharArraySet
}

// NewElisionFilterFactory returns a factory configured with the
// given articles set.
func NewElisionFilterFactory(articles *CharArraySet) *ElisionFilterFactory {
	if articles == nil {
		panic("ElisionFilterFactory: articles must not be nil")
	}
	return &ElisionFilterFactory{articles: articles}
}

// Create wraps input.
func (f *ElisionFilterFactory) Create(input TokenStream) TokenFilter {
	return NewElisionFilter(input, f.articles)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*ElisionFilterFactory)(nil)
