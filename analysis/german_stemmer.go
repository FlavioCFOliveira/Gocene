// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"unicode"
)

// GermanStemmer is the Go port of
// org.apache.lucene.analysis.de.GermanStemmer from Apache Lucene
// 10.4.0. The algorithm follows Joerg Caumann's German stemmer:
// substitute multi-character sequences with single-character
// placeholders, strip common suffixes, optimise, then re-substitute
// the placeholders back to their original digraphs.
//
// Deviation from Lucene: the reference uses a StringBuilder; the
// Go port operates on []rune for code-point correctness and uses a
// fresh slice on every call instead of a shared receiver field, so
// the stemmer is safe to share across goroutines.
type GermanStemmer struct{}

// NewGermanStemmer returns a fresh stateless stemmer.
func NewGermanStemmer() *GermanStemmer {
	return &GermanStemmer{}
}

// Stem returns the stem of term using the German stemming rules.
// Non-letter input is returned unchanged.
func (s *GermanStemmer) Stem(term string) string {
	if term == "" {
		return term
	}
	lower := strings.ToLower(term)
	if !germanIsStemmable(lower) {
		return lower
	}
	buf := []rune(lower)
	substCount := 0
	buf, substCount = germanSubstitute(buf, substCount)
	buf = germanStrip(buf, substCount)
	buf = germanOptimize(buf, substCount)
	buf = germanResubstitute(buf)
	buf = germanRemoveParticleDenotion(buf)
	return string(buf)
}

func germanIsStemmable(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func germanSubstitute(buf []rune, substCount int) ([]rune, int) {
	out := buf[:0]
	for i := 0; i < len(buf); i++ {
		r := buf[i]
		// Replace double letter with '*'
		if i > 0 && r == buf[i-1] {
			out = append(out, '*')
			continue
		}
		switch r {
		case 'ä':
			out = append(out, 'a')
			continue
		case 'ö':
			out = append(out, 'o')
			continue
		case 'ü':
			out = append(out, 'u')
			continue
		case 'ß':
			out = append(out, 's', 's')
			substCount++
			continue
		}
		// Digraph substitutions: must inspect next chars in buf.
		if i+2 < len(buf) && r == 's' && buf[i+1] == 'c' && buf[i+2] == 'h' {
			out = append(out, '$')
			substCount += 2
			i += 2
			continue
		}
		if i+1 < len(buf) {
			next := buf[i+1]
			switch {
			case r == 'c' && next == 'h':
				out = append(out, '§')
				substCount++
				i++
				continue
			case r == 'e' && next == 'i':
				out = append(out, '%')
				substCount++
				i++
				continue
			case r == 'i' && next == 'e':
				out = append(out, '&')
				substCount++
				i++
				continue
			case r == 'i' && next == 'g':
				out = append(out, '#')
				substCount++
				i++
				continue
			case r == 's' && next == 't':
				out = append(out, '!')
				substCount++
				i++
				continue
			}
		}
		out = append(out, r)
	}
	return out, substCount
}

func germanStrip(buf []rune, substCount int) []rune {
	doMore := true
	for doMore && len(buf) > 3 {
		switch {
		case len(buf)+substCount > 5 && len(buf) >= 2 && buf[len(buf)-2] == 'n' && buf[len(buf)-1] == 'd':
			buf = buf[:len(buf)-2]
		case len(buf)+substCount > 4 && len(buf) >= 2 && buf[len(buf)-2] == 'e' && buf[len(buf)-1] == 'm':
			buf = buf[:len(buf)-2]
		case len(buf)+substCount > 4 && len(buf) >= 2 && buf[len(buf)-2] == 'e' && buf[len(buf)-1] == 'r':
			buf = buf[:len(buf)-2]
		case len(buf) >= 1 && buf[len(buf)-1] == 'e':
			buf = buf[:len(buf)-1]
		case len(buf) >= 1 && buf[len(buf)-1] == 's':
			buf = buf[:len(buf)-1]
		case len(buf) >= 1 && buf[len(buf)-1] == 'n':
			buf = buf[:len(buf)-1]
		case len(buf) >= 1 && buf[len(buf)-1] == 't':
			buf = buf[:len(buf)-1]
		default:
			doMore = false
		}
	}
	return buf
}

func germanOptimize(buf []rune, substCount int) []rune {
	if len(buf) > 5 && string(buf[len(buf)-5:]) == "erin*" {
		buf = buf[:len(buf)-1]
		buf = germanStrip(buf, substCount)
	}
	if len(buf) > 0 && buf[len(buf)-1] == 'z' {
		buf[len(buf)-1] = 'x'
	}
	return buf
}

func germanResubstitute(buf []rune) []rune {
	out := make([]rune, 0, len(buf)+4)
	for i := 0; i < len(buf); i++ {
		switch buf[i] {
		case '*':
			if i > 0 {
				out = append(out, out[len(out)-1])
			}
		case '$':
			out = append(out, 's', 'c', 'h')
		case '§':
			out = append(out, 'c', 'h')
		case '%':
			out = append(out, 'e', 'i')
		case '&':
			out = append(out, 'i', 'e')
		case '#':
			out = append(out, 'i', 'g')
		case '!':
			out = append(out, 's', 't')
		default:
			out = append(out, buf[i])
		}
	}
	return out
}

func germanRemoveParticleDenotion(buf []rune) []rune {
	if len(buf) > 4 {
		for i := 0; i < len(buf)-3; i++ {
			if string(buf[i:i+4]) == "gege" {
				return append(append([]rune{}, buf[:i]...), buf[i+2:]...)
			}
		}
	}
	return buf
}

// GermanStemFilter wraps a TokenStream with GermanStemmer.
//
// This is the Go port of
// org.apache.lucene.analysis.de.GermanStemFilter from Apache Lucene
// 10.4.0.
type GermanStemFilter struct {
	*BaseTokenFilter

	stemmer     *GermanStemmer
	termAttr    CharTermAttribute
	keywordAttr *KeywordAttribute
}

// NewGermanStemFilter wraps input with the German stemmer.
func NewGermanStemFilter(input TokenStream) *GermanStemFilter {
	f := &GermanStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewGermanStemmer(),
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
func (f *GermanStemFilter) IncrementToken() (bool, error) {
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
	stem := f.stemmer.Stem(s)
	if stem != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(stem)
	}
	return true, nil
}

// Ensure GermanStemFilter implements TokenFilter.
var _ TokenFilter = (*GermanStemFilter)(nil)

// GermanStemFilterFactory creates GermanStemFilter instances.
type GermanStemFilterFactory struct{}

// NewGermanStemFilterFactory returns a fresh factory.
func NewGermanStemFilterFactory() *GermanStemFilterFactory {
	return &GermanStemFilterFactory{}
}

// Create wraps input.
func (f *GermanStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGermanStemFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*GermanStemFilterFactory)(nil)

// GermanNormalizationFilterFactory creates GermanNormalizationFilter
// instances (filter ported earlier).
//
// Note: a type with the same name was added in
// german_normalization_filter.go. This file does not redeclare it.
