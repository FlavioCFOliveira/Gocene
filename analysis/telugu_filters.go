// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// TeluguNormalizer applies spelling-variation normalisation to Telugu
// text. The rules below match
// org.apache.lucene.analysis.te.TeluguNormalizer from Apache Lucene
// 10.4.0.
type TeluguNormalizer struct{}

// NewTeluguNormalizer returns a fresh stateless normaliser.
func NewTeluguNormalizer() *TeluguNormalizer {
	return &TeluguNormalizer{}
}

// Normalize applies the Telugu normalisation rules. Returns the new
// length; the caller should slice the rune buffer accordingly.
func (n *TeluguNormalizer) Normalize(s []rune, length int) int {
	i := 0
	for i < length {
		c := s[i]
		switch c {
		case 0x0C00, 0x0C01:
			s[i] = 0x0C02
		case 0x0C03:
			length = runeDelete(s, i, length)
			continue
		case 0x200D, 0x200C:
			length = runeDelete(s, i, length)
			continue
		case 0x0C14:
			s[i] = 0x0C13
		case 0x0C10:
			s[i] = 0x0C0F
		case 0x0C06:
			s[i] = 0x0C05
		case 0x0C08:
			s[i] = 0x0C07
		case 0x0C0A:
			s[i] = 0x0C09
		case 0x0C40:
			s[i] = 0x0C3F
		case 0x0C42:
			s[i] = 0x0C41
		case 0x0C47:
			s[i] = 0x0C46
		case 0x0C4B:
			s[i] = 0x0C4A
		case 0x0C46:
			if i+1 < length && s[i+1] == 0x0C56 {
				s[i] = 0x0C48
				length = runeDelete(s, i+1, length)
			}
		case 0x0C12:
			if i+1 < length {
				if s[i+1] == 0x0C55 {
					s[i] = 0x0C13
					length = runeDelete(s, i+1, length)
				} else if s[i+1] == 0x0C4C {
					s[i] = 0x0C14
					length = runeDelete(s, i+1, length)
				}
			}
		}
		i++
	}
	return length
}

// TeluguNormalizationFilter wraps a TokenStream with
// TeluguNormalizer.
//
// This is the Go port of
// org.apache.lucene.analysis.te.TeluguNormalizationFilter.
type TeluguNormalizationFilter struct {
	*BaseTokenFilter

	normalizer *TeluguNormalizer
	termAttr   CharTermAttribute
}

// NewTeluguNormalizationFilter wraps input.
func NewTeluguNormalizationFilter(input TokenStream) *TeluguNormalizationFilter {
	f := &TeluguNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewTeluguNormalizer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken applies the Telugu normaliser.
func (f *TeluguNormalizationFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	runes := []rune(f.termAttr.String())
	n := f.normalizer.Normalize(runes, len(runes))
	res := string(runes[:n])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure TeluguNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*TeluguNormalizationFilter)(nil)

// TeluguNormalizationFilterFactory creates instances.
type TeluguNormalizationFilterFactory struct{}

// NewTeluguNormalizationFilterFactory returns a fresh factory.
func NewTeluguNormalizationFilterFactory() *TeluguNormalizationFilterFactory {
	return &TeluguNormalizationFilterFactory{}
}

// Create wraps input.
func (f *TeluguNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTeluguNormalizationFilter(input)
}

// TeluguStemmer applies a small suffix-stripping ruleset.
type TeluguStemmer struct{}

// NewTeluguStemmer returns a fresh stateless stemmer.
func NewTeluguStemmer() *TeluguStemmer {
	return &TeluguStemmer{}
}

// teluguSuffixes lists the suffix tiers used by the stemmer.
var teluguSuffixGroups = []struct {
	minLen   int
	strip    int
	suffixes []string
}{
	{6, 4, []string{"ళ్ళు", "డ్లు"}},
	{4, 2, []string{
		"డు", "ము", "వు", "లు", "ని", "ను", "చే", "కై",
		"లో", "డు", "ది", "కి", "సు", "వై", "పై",
	}},
	{3, 1, []string{
		"ి", "ీ", "ు", "ూ", "ె", "ే", "ొ", "ో", "ా",
	}},
}

// Stem strips Telugu suffixes from runes[:length] and returns the
// new length.
func (s *TeluguStemmer) Stem(runes []rune, length int) int {
	for _, group := range teluguSuffixGroups {
		if length < group.minLen {
			continue
		}
		for _, suf := range group.suffixes {
			if runesEndWith(runes, length, suf) {
				return length - group.strip
			}
		}
	}
	return length
}

// StemString stems input.
func (s *TeluguStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// TeluguStemFilter wraps a TokenStream with TeluguStemmer.
type TeluguStemFilter struct {
	*BaseTokenFilter

	stemmer     *TeluguStemmer
	termAttr    CharTermAttribute
	keywordAttr KeywordAttribute
}

// NewTeluguStemFilter wraps input.
func NewTeluguStemFilter(input TokenStream) *TeluguStemFilter {
	f := &TeluguStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewTeluguStemmer(),
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

// IncrementToken stems the current token unless it is a keyword.
func (f *TeluguStemFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	s := f.termAttr.String()
	st := f.stemmer.StemString(s)
	if st != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(st)
	}
	return true, nil
}

// Ensure TeluguStemFilter implements TokenFilter.
var _ TokenFilter = (*TeluguStemFilter)(nil)

// TeluguStemFilterFactory creates instances.
type TeluguStemFilterFactory struct{}

// NewTeluguStemFilterFactory returns a fresh factory.
func NewTeluguStemFilterFactory() *TeluguStemFilterFactory {
	return &TeluguStemFilterFactory{}
}

// Create wraps input.
func (f *TeluguStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTeluguStemFilter(input)
}
