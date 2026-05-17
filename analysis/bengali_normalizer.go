// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// Bengali (Bangla) Unicode code points used by the normalisation
// rules.
const (
	bnCHANDRABINDU       = 0x0981
	bnVISARGA            = 0x0983
	bnDIRGHO_I_KAR       = 0x09C0
	bnROSSHO_I_KAR       = 0x09BF
	bnDIRGHO_U_KAR       = 0x09C2
	bnROSSHO_U_KAR       = 0x09C1
	bnKA                 = 0x0995
	bnKHA                = 0x0996
	bnHOSHONTO           = 0x09CD
	bnROSSHO_I           = 0x09BF
	bnNGA                = 0x0999
	bnANUSVARA           = 0x0982
	bnJA                 = 0x09AF
	bnE_KAR              = 0x09C7
	bnAA_KAR             = 0x09BE
	bnBA                 = 0x09AC
	bnSHA                = 0x09B6
	bnSSHA               = 0x09B7
	bnSA                 = 0x09B8
	bnHA                 = 0x09B9
	bnNA_MURDHANYA       = 0x09A3
	bnNA                 = 0x09A8
	bnRA_DOWN            = 0x09DC
	bnRHA                = 0x09DD
	bnRA                 = 0x09B0
	bnKHANDA_TA          = 0x09CE
	bnTA                 = 0x09A4
)

// BengaliNormalizer normalises Bengali (Bangla) orthography.
//
// This is the Go port of
// org.apache.lucene.analysis.bn.BengaliNormalizer from Apache Lucene
// 10.4.0. The algorithm follows "A Double Metaphone encoding for
// Bangla and its application in spelling checker" by Naushad UzZaman
// and Mumit Khan.
//
// Deviation from Lucene: the reference operates in place on a
// char[]; this port returns a transformed []rune for clarity and
// safety with index-shifting deletions.
type BengaliNormalizer struct{}

// NewBengaliNormalizer returns a fresh stateless normaliser.
func NewBengaliNormalizer() *BengaliNormalizer {
	return &BengaliNormalizer{}
}

// Normalize applies the Bengali normalisation rules to s and returns
// the resulting rune slice.
func (n *BengaliNormalizer) Normalize(s []rune) []rune {
	out := make([]rune, 0, len(s))

	emit := func(r rune) {
		out = append(out, r)
	}

	for i := 0; i < len(s); i++ {
		r := s[i]
		switch r {
		case bnCHANDRABINDU:
			// delete
		case bnDIRGHO_I_KAR:
			emit(bnROSSHO_I_KAR)
		case bnDIRGHO_U_KAR:
			emit(bnROSSHO_U_KAR)
		case bnKA:
			// Khio sequence: Ka + Hoshonto + RosshoI
			if i+2 < len(s) && s[i+1] == bnHOSHONTO && s[i+2] == bnROSSHO_I {
				if i == 0 {
					emit(bnKHA)
					// drop next two
					i += 2
				} else {
					emit(bnKA)
					emit(bnKHA)
					i += 2
					// the original Java promotes s[i+1] (Hoshonto) to KHA
					// and deletes s[i+2] (RosshoI). The original Ka stays.
					// Our emit order: Ka, then Kha (in place of Hoshonto),
					// then skip RosshoI.
				}
			} else {
				emit(bnKA)
			}
		case bnNGA:
			emit(bnANUSVARA)
		case bnJA:
			// Ja Phala handling: requires a preceding Hoshonto
			if len(out) >= 1 && out[len(out)-1] == bnHOSHONTO {
				if len(out) == 1 {
					// pattern at index 1: replace Hoshonto with E_kar and
					// drop a following AA_kar if present
					out[len(out)-1] = bnE_KAR
					if i+1 < len(s) && s[i+1] == bnAA_KAR {
						i++ // skip AA_kar
					}
					// drop the Ja itself
				} else {
					// drop both Ja and the preceding Hoshonto
					out = out[:len(out)-1]
				}
			} else {
				emit(bnJA)
			}
		case bnBA:
			// Ba Phalaa requires a preceding Hoshonto
			if len(out) == 0 || out[len(out)-1] != bnHOSHONTO {
				emit(bnBA)
				break
			}
			// preceded by Hoshonto: handle three sub-cases
			if len(out) == 1 {
				// drop both Ba and Hoshonto
				out = out[:0]
			} else if len(out) >= 3 && out[len(out)-3] == bnHOSHONTO {
				// pattern with another Hoshonto two positions back: drop
				// Ba and the immediate Hoshonto.
				out = out[:len(out)-1]
			} else {
				// default: replace Hoshonto with the prior letter and
				// drop Ba.
				out[len(out)-1] = out[len(out)-2]
			}
		case bnVISARGA:
			if i == len(s)-1 {
				if len(s) <= 3 {
					emit(bnHA)
				} // else delete
			} else {
				// replace with next char (the reference does s[i] = s[i+1])
				if i+1 < len(s) {
					emit(s[i+1])
				}
			}
		case bnSHA, bnSSHA:
			emit(bnSA)
		case bnNA_MURDHANYA:
			emit(bnNA)
		case bnRA_DOWN, bnRHA:
			emit(bnRA)
		case bnKHANDA_TA:
			emit(bnTA)
		default:
			emit(r)
		}
	}
	return out
}

// NormalizeString applies Normalize to s.
func (n *BengaliNormalizer) NormalizeString(s string) string {
	if s == "" {
		return ""
	}
	return string(n.Normalize([]rune(s)))
}

// BengaliStemmer is a suffix stemmer for Bengali. It strips a
// progressively-shorter list of inflectional suffixes.
//
// This is the Go port of org.apache.lucene.analysis.bn.BengaliStemmer.
type BengaliStemmer struct{}

// NewBengaliStemmer returns a fresh stateless stemmer.
func NewBengaliStemmer() *BengaliStemmer {
	return &BengaliStemmer{}
}

// bengaliSuffixes lists the suffix groups in Lucene order: 8, 7, 6,
// 5, 4, 3, 2, 1 runes long. Each group has a minimum length guard
// and a strip count.
var bengaliSuffixGroups = []struct {
	minLen   int
	strip    int
	suffixes []string
}{
	{9, 8, []string{
		"িয়াছিলাম", "িতেছিলাম", "িতেছিলেন", "ইতেছিলেন",
		"িয়াছিলেন", "ইয়াছিলেন",
	}},
	{8, 7, []string{
		"িতেছিলি", "িতেছিলে", "িয়াছিলা", "িয়াছিলে",
		"িতেছিলা", "িয়াছিলি", "য়েদেরকে",
	}},
	{7, 6, []string{
		"িতেছিস", "িতেছেন", "িয়াছিস", "িয়াছেন",
		"েছিলাম", "েছিলেন", "েদেরকে",
	}},
	{6, 5, []string{
		"িতেছি", "িতেছা", "িতেছে", "ছিলাম", "ছিলেন",
		"িয়াছি", "িয়াছা", "িয়াছে", "েছিলে", "েছিলা",
		"য়েদের", "দেরকে",
	}},
	{5, 4, []string{
		"িলাম", "িলেন", "িতাম", "িতেন", "িবেন", "ছিলি",
		"ছিলে", "ছিলা", "তেছে", "িতেছ", "খানা", "খানি",
		"গুলো", "গুলি", "য়েরা", "েদের",
	}},
	{4, 3, []string{
		"লাম", "িলি", "ইলি", "িলে", "ইলে", "লেন", "িলা",
		"ইলা", "তাম", "িতি", "ইতি", "িতে", "ইতে", "তেন",
		"িতা", "িবা", "ইবা", "িবি", "ইবি", "বেন", "িবে",
		"ইবে", "ছেন", "য়োন", "য়ের", "েরা", "দের",
	}},
	{3, 2, []string{
		"িস", "েন", "লি", "লে", "লা", "তি", "তে", "তা",
		"বি", "বে", "বা", "ছি", "ছা", "ছে", "ুন", "ুক",
		"টা", "টি", "নি", "ের", "তে", "রা", "কে",
	}},
	{2, 1, []string{
		"ি", "ী", "া", "ো", "ে", "ব", "ত",
	}},
}

// Stem strips Bengali suffixes from runes[:length] and returns the
// new length.
func (s *BengaliStemmer) Stem(runes []rune, length int) int {
	for _, group := range bengaliSuffixGroups {
		if length <= group.minLen {
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

// StemString stems s and returns the result.
func (s *BengaliStemmer) StemString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// BengaliNormalizationFilter applies BengaliNormalizer to every
// non-keyword token.
//
// This is the Go port of
// org.apache.lucene.analysis.bn.BengaliNormalizationFilter.
type BengaliNormalizationFilter struct {
	*BaseTokenFilter

	normalizer  *BengaliNormalizer
	termAttr    CharTermAttribute
	keywordAttr *KeywordAttribute
}

// NewBengaliNormalizationFilter wraps input with the normaliser.
func NewBengaliNormalizationFilter(input TokenStream) *BengaliNormalizationFilter {
	f := &BengaliNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewBengaliNormalizer(),
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

// IncrementToken normalises the current token unless it is marked as
// a keyword.
func (f *BengaliNormalizationFilter) IncrementToken() (bool, error) {
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
	if f.keywordAttr != nil && f.keywordAttr.IsKeyword {
		return true, nil
	}
	s := f.termAttr.String()
	n := f.normalizer.NormalizeString(s)
	if n != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(n)
	}
	return true, nil
}

// Ensure BengaliNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*BengaliNormalizationFilter)(nil)

// BengaliNormalizationFilterFactory creates BengaliNormalizationFilter
// instances.
type BengaliNormalizationFilterFactory struct{}

// NewBengaliNormalizationFilterFactory returns a fresh factory.
func NewBengaliNormalizationFilterFactory() *BengaliNormalizationFilterFactory {
	return &BengaliNormalizationFilterFactory{}
}

// Create returns a BengaliNormalizationFilter wrapping input.
func (f *BengaliNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewBengaliNormalizationFilter(input)
}

// Ensure BengaliNormalizationFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*BengaliNormalizationFilterFactory)(nil)

// BengaliStemFilter applies BengaliStemmer to every non-keyword
// token.
//
// This is the Go port of
// org.apache.lucene.analysis.bn.BengaliStemFilter.
type BengaliStemFilter struct {
	*BaseTokenFilter

	stemmer     *BengaliStemmer
	termAttr    CharTermAttribute
	keywordAttr *KeywordAttribute
}

// NewBengaliStemFilter wraps input with the stemmer.
func NewBengaliStemFilter(input TokenStream) *BengaliStemFilter {
	f := &BengaliStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewBengaliStemmer(),
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

// IncrementToken stems the current token unless it is marked as a
// keyword.
func (f *BengaliStemFilter) IncrementToken() (bool, error) {
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

// Ensure BengaliStemFilter implements TokenFilter.
var _ TokenFilter = (*BengaliStemFilter)(nil)

// BengaliStemFilterFactory creates BengaliStemFilter instances.
type BengaliStemFilterFactory struct{}

// NewBengaliStemFilterFactory returns a fresh factory.
func NewBengaliStemFilterFactory() *BengaliStemFilterFactory {
	return &BengaliStemFilterFactory{}
}

// Create returns a BengaliStemFilter wrapping input.
func (f *BengaliStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewBengaliStemFilter(input)
}

// Ensure BengaliStemFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*BengaliStemFilterFactory)(nil)
