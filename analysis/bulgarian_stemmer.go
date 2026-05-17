// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// BulgarianStemmer is the Go port of
// org.apache.lucene.analysis.bg.BulgarianStemmer from Apache Lucene
// 10.4.0. It strips Bulgarian article and plural suffixes from
// noun and adjective tokens.
type BulgarianStemmer struct{}

// NewBulgarianStemmer returns a fresh stateless stemmer.
func NewBulgarianStemmer() *BulgarianStemmer {
	return &BulgarianStemmer{}
}

// Stem strips Bulgarian suffixes from runes[:length] and returns
// the new length.
func (s *BulgarianStemmer) Stem(runes []rune, length int) int {
	if length < 4 {
		return length
	}
	if length > 5 && runesEndWith(runes, length, "ища") {
		return length - 3
	}
	length = bulgarianRemoveArticle(runes, length)
	length = bulgarianRemovePlural(runes, length)
	if length > 3 {
		if runesEndWith(runes, length, "я") {
			length--
		}
		if length > 0 && (runes[length-1] == 'а' || runes[length-1] == 'о' || runes[length-1] == 'е') {
			length--
		}
	}
	if length > 4 && runesEndWith(runes, length, "ен") {
		runes[length-2] = 'н'
		length--
	}
	if length > 5 && runes[length-2] == 'ъ' {
		runes[length-2] = runes[length-1]
		length--
	}
	return length
}

func bulgarianRemoveArticle(s []rune, length int) int {
	if length > 6 && runesEndWith(s, length, "ият") {
		return length - 3
	}
	if length > 5 {
		if runesEndWith(s, length, "ът") ||
			runesEndWith(s, length, "то") ||
			runesEndWith(s, length, "те") ||
			runesEndWith(s, length, "та") ||
			runesEndWith(s, length, "ия") {
			return length - 2
		}
	}
	if length > 4 && runesEndWith(s, length, "ят") {
		return length - 2
	}
	return length
}

func bulgarianRemovePlural(s []rune, length int) int {
	if length > 6 {
		if runesEndWith(s, length, "овци") {
			return length - 3
		}
		if runesEndWith(s, length, "ове") {
			return length - 3
		}
		if runesEndWith(s, length, "еве") {
			s[length-3] = 'й'
			return length - 2
		}
	}
	if length > 5 {
		if runesEndWith(s, length, "ища") {
			return length - 3
		}
		if runesEndWith(s, length, "та") {
			return length - 2
		}
		if runesEndWith(s, length, "ци") {
			s[length-2] = 'к'
			return length - 1
		}
		if runesEndWith(s, length, "зи") {
			s[length-2] = 'г'
			return length - 1
		}
		if length >= 3 && s[length-3] == 'е' && s[length-1] == 'и' {
			s[length-3] = 'я'
			return length - 1
		}
	}
	if length > 4 {
		if runesEndWith(s, length, "си") {
			s[length-2] = 'х'
			return length - 1
		}
		if runesEndWith(s, length, "и") {
			return length - 1
		}
	}
	return length
}

// StemString stems input.
func (s *BulgarianStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// BulgarianStemFilter wraps a TokenStream with BulgarianStemmer.
//
// This is the Go port of
// org.apache.lucene.analysis.bg.BulgarianStemFilter.
type BulgarianStemFilter struct {
	*BaseTokenFilter

	stemmer     *BulgarianStemmer
	termAttr    CharTermAttribute
	keywordAttr *KeywordAttribute
}

// NewBulgarianStemFilter wraps input.
func NewBulgarianStemFilter(input TokenStream) *BulgarianStemFilter {
	f := &BulgarianStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewBulgarianStemmer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&KeywordAttribute{})); a != nil {
			f.keywordAttr = a.(*KeywordAttribute)
		}
	}
	return f
}

// IncrementToken stems the current token unless it is a keyword.
func (f *BulgarianStemFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeyword {
		return true, nil
	}
	s := f.termAttr.String()
	stem := f.stemmer.StemString(s)
	if stem != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(stem)
	}
	return true, nil
}

// Ensure BulgarianStemFilter implements TokenFilter.
var _ TokenFilter = (*BulgarianStemFilter)(nil)

// BulgarianStemFilterFactory creates BulgarianStemFilter instances.
type BulgarianStemFilterFactory struct{}

// NewBulgarianStemFilterFactory returns a fresh factory.
func NewBulgarianStemFilterFactory() *BulgarianStemFilterFactory {
	return &BulgarianStemFilterFactory{}
}

// Create wraps input.
func (f *BulgarianStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewBulgarianStemFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*BulgarianStemFilterFactory)(nil)
