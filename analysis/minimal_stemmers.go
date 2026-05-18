// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// This file gathers the language-specific "minimal" stemmers ported
// from the Apache Lucene 10.4.0 analysis-common module. Each
// language has three exported types:
//
//   <Language>MinimalStemmer        — a stateless stemmer struct
//   <Language>MinimalStemFilter     — a TokenFilter wrapping the stemmer
//   <Language>MinimalStemFilterFactory — a no-arg factory
//
// The stemmers all operate on a []rune buffer plus a logical length
// integer (matching Lucene's char[] + int len protocol) and return
// the new length. Filters convert the term text to a rune slice,
// invoke the stemmer, and write the truncated/transformed text back
// to the CharTermAttribute.
//
// Sources ported in this file:
//   en: EnglishMinimalStemmer / EnglishMinimalStemFilter
//   de: GermanMinimalStemmer / GermanMinimalStemFilter
//   fr: FrenchMinimalStemmer / FrenchMinimalStemFilter
//   sv: SwedishMinimalStemmer / SwedishMinimalStemFilter
//   no: NorwegianMinimalStemmer / NorwegianMinimalStemFilter
//   es: SpanishMinimalStemmer / SpanishMinimalStemFilter

import (
	"reflect"
	"unicode"
)

// minimalStemFilter is the common embedded helper used by every
// language-specific minimal-stem filter in this file. Subclasses
// supply the StemFunc that transforms (runes, len) -> new length.
type minimalStemFilter struct {
	*BaseTokenFilter

	stemFunc    func(runes []rune, length int) int
	termAttr    CharTermAttribute
	keywordAttr KeywordAttribute
}

func newMinimalStemFilter(input TokenStream, stemFunc func([]rune, int) int) minimalStemFilter {
	f := minimalStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemFunc:        stemFunc,
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

// incrementToken pulls the next input token, applies stemFunc when
// the token is not a keyword, and writes any transformed text back
// to the CharTermAttribute.
func (f *minimalStemFilter) incrementToken() (bool, error) {
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
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	runes := []rune(f.termAttr.String())
	n := f.stemFunc(runes, len(runes))
	if n != len(runes) || !runeEq(runes, []rune(f.termAttr.String())) {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(string(runes[:n]))
	}
	return true, nil
}

// runeEq reports whether two rune slices have identical contents.
func runeEq(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- English ---

// EnglishMinimalStemmer implements the S-Stemmer from Donna Harman's
// "How Effective Is Suffixing?". This is the Go port of
// org.apache.lucene.analysis.en.EnglishMinimalStemmer.
type EnglishMinimalStemmer struct{}

// NewEnglishMinimalStemmer returns a fresh stateless stemmer.
func NewEnglishMinimalStemmer() *EnglishMinimalStemmer {
	return &EnglishMinimalStemmer{}
}

// Stem applies the S-Stemmer rules.
func (s *EnglishMinimalStemmer) Stem(runes []rune, length int) int {
	if length < 3 || runes[length-1] != 's' {
		return length
	}
	switch runes[length-2] {
	case 'u', 's':
		return length
	case 'e':
		if length > 3 && runes[length-3] == 'i' && runes[length-4] != 'a' && runes[length-4] != 'e' {
			runes[length-3] = 'y'
			return length - 2
		}
		if runes[length-3] == 'i' || runes[length-3] == 'a' || runes[length-3] == 'o' || runes[length-3] == 'e' {
			return length
		}
		// Java has intentional fall-through to default.
		return length - 1
	default:
		return length - 1
	}
}

// EnglishMinimalStemFilter applies EnglishMinimalStemmer to every
// non-keyword token.
type EnglishMinimalStemFilter struct {
	minimalStemFilter
	stemmer *EnglishMinimalStemmer
}

// NewEnglishMinimalStemFilter wraps input.
func NewEnglishMinimalStemFilter(input TokenStream) *EnglishMinimalStemFilter {
	st := NewEnglishMinimalStemmer()
	f := &EnglishMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

// IncrementToken delegates to the embedded helper.
func (f *EnglishMinimalStemFilter) IncrementToken() (bool, error) {
	return f.incrementToken()
}

// Ensure interface satisfaction.
var _ TokenFilter = (*EnglishMinimalStemFilter)(nil)

// EnglishMinimalStemFilterFactory creates EnglishMinimalStemFilter
// instances.
type EnglishMinimalStemFilterFactory struct{}

// NewEnglishMinimalStemFilterFactory returns a fresh factory.
func NewEnglishMinimalStemFilterFactory() *EnglishMinimalStemFilterFactory {
	return &EnglishMinimalStemFilterFactory{}
}

// Create wraps input with EnglishMinimalStemFilter.
func (f *EnglishMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewEnglishMinimalStemFilter(input)
}

// --- German ---

// GermanMinimalStemmer is the Go port of
// org.apache.lucene.analysis.de.GermanMinimalStemmer; it normalises
// umlauts and removes simple inflectional suffixes.
type GermanMinimalStemmer struct{}

// NewGermanMinimalStemmer returns a fresh stateless stemmer.
func NewGermanMinimalStemmer() *GermanMinimalStemmer {
	return &GermanMinimalStemmer{}
}

// Stem applies the German minimal stemmer rules.
func (s *GermanMinimalStemmer) Stem(runes []rune, length int) int {
	if length < 5 {
		return length
	}
	for i := 0; i < length; i++ {
		switch runes[i] {
		case 'ä':
			runes[i] = 'a'
		case 'ö':
			runes[i] = 'o'
		case 'ü':
			runes[i] = 'u'
		}
	}
	if length > 6 && runes[length-3] == 'n' && runes[length-2] == 'e' && runes[length-1] == 'n' {
		return length - 3
	}
	if length > 5 {
		switch runes[length-1] {
		case 'n':
			if runes[length-2] == 'e' {
				return length - 2
			}
		case 'e':
			if runes[length-2] == 's' {
				return length - 2
			}
		case 's':
			if runes[length-2] == 'e' {
				return length - 2
			}
		case 'r':
			if runes[length-2] == 'e' {
				return length - 2
			}
		}
	}
	switch runes[length-1] {
	case 'n', 'e', 's', 'r':
		return length - 1
	}
	return length
}

// GermanMinimalStemFilter / Factory / Constructor follow.
type GermanMinimalStemFilter struct {
	minimalStemFilter
	stemmer *GermanMinimalStemmer
}

func NewGermanMinimalStemFilter(input TokenStream) *GermanMinimalStemFilter {
	st := NewGermanMinimalStemmer()
	f := &GermanMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

func (f *GermanMinimalStemFilter) IncrementToken() (bool, error) { return f.incrementToken() }

var _ TokenFilter = (*GermanMinimalStemFilter)(nil)

type GermanMinimalStemFilterFactory struct{}

func NewGermanMinimalStemFilterFactory() *GermanMinimalStemFilterFactory {
	return &GermanMinimalStemFilterFactory{}
}

func (f *GermanMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGermanMinimalStemFilter(input)
}

// --- French ---

// FrenchMinimalStemmer is the Go port of Jacques Savoy's French
// minimal stemmer (org.apache.lucene.analysis.fr.FrenchMinimalStemmer).
type FrenchMinimalStemmer struct{}

// NewFrenchMinimalStemmer returns a fresh stateless stemmer.
func NewFrenchMinimalStemmer() *FrenchMinimalStemmer {
	return &FrenchMinimalStemmer{}
}

// Stem applies the French minimal stemmer rules.
func (s *FrenchMinimalStemmer) Stem(runes []rune, length int) int {
	if length < 6 {
		return length
	}
	if runes[length-1] == 'x' {
		if runes[length-3] == 'a' && runes[length-2] == 'u' {
			runes[length-2] = 'l'
		}
		return length - 1
	}
	if runes[length-1] == 's' {
		length--
	}
	if runes[length-1] == 'r' {
		length--
	}
	if runes[length-1] == 'e' {
		length--
	}
	if runes[length-1] == 'é' {
		length--
	}
	if length >= 2 && runes[length-1] == runes[length-2] && unicode.IsLetter(runes[length-1]) {
		length--
	}
	return length
}

type FrenchMinimalStemFilter struct {
	minimalStemFilter
	stemmer *FrenchMinimalStemmer
}

func NewFrenchMinimalStemFilter(input TokenStream) *FrenchMinimalStemFilter {
	st := NewFrenchMinimalStemmer()
	f := &FrenchMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

func (f *FrenchMinimalStemFilter) IncrementToken() (bool, error) { return f.incrementToken() }

var _ TokenFilter = (*FrenchMinimalStemFilter)(nil)

type FrenchMinimalStemFilterFactory struct{}

func NewFrenchMinimalStemFilterFactory() *FrenchMinimalStemFilterFactory {
	return &FrenchMinimalStemFilterFactory{}
}

func (f *FrenchMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFrenchMinimalStemFilter(input)
}

// --- Swedish ---

// SwedishMinimalStemmer is the Go port of
// org.apache.lucene.analysis.sv.SwedishMinimalStemmer.
type SwedishMinimalStemmer struct{}

// NewSwedishMinimalStemmer returns a fresh stateless stemmer.
func NewSwedishMinimalStemmer() *SwedishMinimalStemmer {
	return &SwedishMinimalStemmer{}
}

// Stem applies the Swedish minimal stemmer rules.
func (s *SwedishMinimalStemmer) Stem(runes []rune, length int) int {
	if length > 4 && runes[length-1] == 's' {
		length--
	}
	if length > 6 && (runesEndWith(runes, length, "arne") ||
		runesEndWith(runes, length, "erna") ||
		runesEndWith(runes, length, "arna") ||
		runesEndWith(runes, length, "orna") ||
		runesEndWith(runes, length, "aren")) {
		return length - 4
	}
	if length > 5 && runesEndWith(runes, length, "are") {
		return length - 3
	}
	if length > 4 && (runesEndWith(runes, length, "ar") ||
		runesEndWith(runes, length, "at") ||
		runesEndWith(runes, length, "er") ||
		runesEndWith(runes, length, "et") ||
		runesEndWith(runes, length, "or") ||
		runesEndWith(runes, length, "en")) {
		return length - 2
	}
	if length > 3 {
		switch runes[length-1] {
		case 'a', 'e', 'n':
			return length - 1
		}
	}
	return length
}

type SwedishMinimalStemFilter struct {
	minimalStemFilter
	stemmer *SwedishMinimalStemmer
}

func NewSwedishMinimalStemFilter(input TokenStream) *SwedishMinimalStemFilter {
	st := NewSwedishMinimalStemmer()
	f := &SwedishMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

func (f *SwedishMinimalStemFilter) IncrementToken() (bool, error) { return f.incrementToken() }

var _ TokenFilter = (*SwedishMinimalStemFilter)(nil)

type SwedishMinimalStemFilterFactory struct{}

func NewSwedishMinimalStemFilterFactory() *SwedishMinimalStemFilterFactory {
	return &SwedishMinimalStemFilterFactory{}
}

func (f *SwedishMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSwedishMinimalStemFilter(input)
}

// --- Norwegian ---

// NorwegianMinimalStemmer is the Go port of
// org.apache.lucene.analysis.no.NorwegianMinimalStemmer. The Nynorsk
// flag enables the Nynorsk-specific masc plural rules; the default
// is Bokmal.
type NorwegianMinimalStemmer struct {
	UseNynorsk bool
}

// NewNorwegianMinimalStemmer returns a Bokmal stemmer.
func NewNorwegianMinimalStemmer() *NorwegianMinimalStemmer {
	return &NorwegianMinimalStemmer{}
}

// NewNorwegianMinimalStemmerNynorsk returns a Nynorsk stemmer.
func NewNorwegianMinimalStemmerNynorsk() *NorwegianMinimalStemmer {
	return &NorwegianMinimalStemmer{UseNynorsk: true}
}

// Stem applies the Norwegian minimal stemmer rules.
func (s *NorwegianMinimalStemmer) Stem(runes []rune, length int) int {
	if length > 4 && runes[length-1] == 's' {
		length--
	}
	if length > 5 && (runesEndWith(runes, length, "ene") ||
		(runesEndWith(runes, length, "ane") && s.UseNynorsk)) {
		return length - 3
	}
	if length > 4 && (runesEndWith(runes, length, "er") ||
		runesEndWith(runes, length, "en") ||
		runesEndWith(runes, length, "et") ||
		(runesEndWith(runes, length, "ar") && s.UseNynorsk)) {
		return length - 2
	}
	if length > 3 {
		switch runes[length-1] {
		case 'a', 'e':
			return length - 1
		}
	}
	return length
}

type NorwegianMinimalStemFilter struct {
	minimalStemFilter
	stemmer *NorwegianMinimalStemmer
}

func NewNorwegianMinimalStemFilter(input TokenStream) *NorwegianMinimalStemFilter {
	st := NewNorwegianMinimalStemmer()
	f := &NorwegianMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

func (f *NorwegianMinimalStemFilter) IncrementToken() (bool, error) { return f.incrementToken() }

var _ TokenFilter = (*NorwegianMinimalStemFilter)(nil)

type NorwegianMinimalStemFilterFactory struct{}

func NewNorwegianMinimalStemFilterFactory() *NorwegianMinimalStemFilterFactory {
	return &NorwegianMinimalStemFilterFactory{}
}

func (f *NorwegianMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewNorwegianMinimalStemFilter(input)
}

// --- Spanish ---

// SpanishMinimalStemmer is the Go port of (the deprecated)
// org.apache.lucene.analysis.es.SpanishMinimalStemmer. Lucene
// recommends SpanishPluralStemmer; we preserve this for parity with
// the upstream class set.
type SpanishMinimalStemmer struct{}

// NewSpanishMinimalStemmer returns a fresh stateless stemmer.
func NewSpanishMinimalStemmer() *SpanishMinimalStemmer {
	return &SpanishMinimalStemmer{}
}

// Stem applies the Spanish minimal stemmer rules: it normalises
// accented vowels and ñ, then strips simple plural suffixes.
func (s *SpanishMinimalStemmer) Stem(runes []rune, length int) int {
	if length < 4 || runes[length-1] != 's' {
		return length
	}
	for i := 0; i < length; i++ {
		switch runes[i] {
		case 'à', 'á', 'â', 'ä':
			runes[i] = 'a'
		case 'ò', 'ó', 'ô', 'ö':
			runes[i] = 'o'
		case 'è', 'é', 'ê', 'ë':
			runes[i] = 'e'
		case 'ù', 'ú', 'û', 'ü':
			runes[i] = 'u'
		case 'ì', 'í', 'î', 'ï':
			runes[i] = 'i'
		case 'ñ':
			runes[i] = 'n'
		}
	}
	if runes[length-1] == 's' {
		if length >= 2 && (runes[length-2] == 'a' || runes[length-2] == 'o') {
			return length - 1
		}
		if length >= 2 && runes[length-2] == 'e' {
			if length >= 4 && runes[length-3] == 's' && runes[length-4] == 'e' {
				return length - 2
			}
			if length >= 3 && runes[length-3] == 'c' {
				runes[length-3] = 'z'
				return length - 2
			}
			return length - 2
		}
		return length - 1
	}
	return length
}

type SpanishMinimalStemFilter struct {
	minimalStemFilter
	stemmer *SpanishMinimalStemmer
}

func NewSpanishMinimalStemFilter(input TokenStream) *SpanishMinimalStemFilter {
	st := NewSpanishMinimalStemmer()
	f := &SpanishMinimalStemFilter{stemmer: st}
	f.minimalStemFilter = newMinimalStemFilter(input, st.Stem)
	return f
}

func (f *SpanishMinimalStemFilter) IncrementToken() (bool, error) { return f.incrementToken() }

var _ TokenFilter = (*SpanishMinimalStemFilter)(nil)

type SpanishMinimalStemFilterFactory struct{}

func NewSpanishMinimalStemFilterFactory() *SpanishMinimalStemFilterFactory {
	return &SpanishMinimalStemFilterFactory{}
}

func (f *SpanishMinimalStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSpanishMinimalStemFilter(input)
}
