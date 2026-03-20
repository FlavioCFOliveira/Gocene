// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"strings"
	"unicode"
)

// TurkishStopWords contains common Turkish stop words.
// Source: Apache Lucene Turkish stop words list
var TurkishStopWords = []string{
	"acaba", "ama", "aslında", "bana", "bazı", "belki", "ben", "bende",
	"beni", "benim", "beri", "beş", "bile", "bin", "bir", "birçok",
	"biri", "birkaç", "birkez", "birşey", "birşeyi", "biz", "bize",
	"bizi", "bizim", "böyle", "böylece", "bu", "buna", "bunda", "bundan",
	"bunlar", "bunları", "bunların", "bunu", "bunun", "burada", "çok",
	"çünkü", "da", "daha", "dahi", "de", "defa", "değil", "diğer",
	"diğeri", "diğerine", "diye", "doğru", "dokuz", "dolayı", "dolayısıyla",
	"dört", "edecek", "eden", "eder", "ederek", "edilecek", "ediliyor",
	"edilmesi", "ediyor", "eğer", "elli", "en", "etmesi", "etti",
	"ettiği", "ettiğini", "gibi", "göre", "halde", "hangi", "hatta",
	"hem", "henüz", "hep", "hepsi", "her", "herhangi", "herkes",
	"herkesin", "hiç", "hiçbir", "için", "iki", "ile", "ilgili",
	"ise", "işte", "itibaren", "itibariyle", "kaç", "kadar", "karşın",
	"katrilyon", "kendi", "kendilerine", "kendini", "kendisi",
	"kendisine", "kendisini", "kez", "ki", "kim", "kimden", "kime",
	"kimi", "kimse", "kırk", "milyar", "milyon", "mı", "mu", "mü",
	"nasıl", "ne", "neden", "nedenle", "nerde", "nerede", "nereye",
	"niye", "niçin", "o", "olan", "olarak", "oldu", "olduğu",
	"olduğunu", "olduklarını", "olmadı", "olmadığı", "olmak", "olması",
	"olmayan", "olmaz", "olsa", "olsun", "olup", "olur", "olursa",
	"oluyor", "on", "ona", "ondan", "onlar", "onlardan", "onları",
	"onların", "onu", "onun", "otuz", "oysa", "oysaki", "öyle",
	"pek", "rağmen", "sadece", "sanki", "sekiz", "sen", "senden",
	"seni", "senin", "siz", "sizden", "sizi", "sizin", "şey",
	"şeyden", "şeyi", "şeyler", "şöyle", "şu", "şuna", "şunda",
	"şundan", "şunları", "şunu", "tarafından", "trilyon", "tüm",
	"üç", "üzere", "var", "vardı", "ve", "veya", "ya", "yani",
	"yapacak", "yapılan", "yapılması", "yapıyor", "yapmak", "yaptı",
	"yaptığı", "yaptığını", "yaptıkları", "yedi", "yerine", "yetmiş",
	"yine", "yirmi", "yoksa", "yüz", "zaten", "zira",
}

// TurkishAnalyzer is an analyzer for Turkish language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.tr.TurkishAnalyzer.
//
// TurkishAnalyzer uses the StandardTokenizer with Turkish-specific lowercasing and stop words removal.
// Turkish has special casing rules: dotted i (İ) and dotless i (I) need special handling.
type TurkishAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewTurkishAnalyzer creates a new TurkishAnalyzer with default Turkish stop words.
func NewTurkishAnalyzer() *TurkishAnalyzer {
	stopSet := GetWordSetFromStrings(TurkishStopWords, true)
	return NewTurkishAnalyzerWithWords(stopSet)
}

// NewTurkishAnalyzerWithWords creates a TurkishAnalyzer with custom stop words.
func NewTurkishAnalyzerWithWords(stopWords *CharArraySet) *TurkishAnalyzer {
	a := &TurkishAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	// Turkish requires special lowercasing - use TurkishLowerCaseFilter
	a.AddTokenFilter(NewTurkishLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *TurkishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *TurkishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *TurkishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*TurkishAnalyzer)(nil)
var _ AnalyzerInterface = (*TurkishAnalyzer)(nil)

// TurkishLowerCaseFilter implements Turkish-specific lowercasing.
//
// Turkish has two i's: dotted i (i, İ) and dotless i (ı, I)
// This filter handles the special casing rules for Turkish.
type TurkishLowerCaseFilter struct {
	*BaseTokenFilter
}

// NewTurkishLowerCaseFilter creates a new TurkishLowerCaseFilter.
func NewTurkishLowerCaseFilter(input TokenStream) *TurkishLowerCaseFilter {
	return &TurkishLowerCaseFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies Turkish lowercasing.
func (f *TurkishLowerCaseFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				lowered := turkishToLower(term)
				if lowered != term {
					termAttr.SetEmpty()
					termAttr.AppendString(lowered)
				}
			}
		}
	}

	return hasToken, nil
}

// turkishToLower converts text to lowercase using Turkish-specific rules.
// In Turkish:
// - I (uppercase dotless) → ı (lowercase dotless)
// - İ (uppercase dotted) → i (lowercase dotted)
// - Other characters use standard Unicode lowercasing
func turkishToLower(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		// Capital I (dotless) → lowercase dotless ı
		case 'I':
			result.WriteRune('ı')
		// Capital İ (dotted) → lowercase dotted i
		case 'İ':
			result.WriteRune('i')
		default:
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

// TurkishLowerCaseFilterFactory creates TurkishLowerCaseFilter instances.
type TurkishLowerCaseFilterFactory struct{}

// NewTurkishLowerCaseFilterFactory creates a new TurkishLowerCaseFilterFactory.
func NewTurkishLowerCaseFilterFactory() *TurkishLowerCaseFilterFactory {
	return &TurkishLowerCaseFilterFactory{}
}

// Create creates a new TurkishLowerCaseFilter.
func (f *TurkishLowerCaseFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTurkishLowerCaseFilter(input)
}

var _ TokenFilterFactory = (*TurkishLowerCaseFilterFactory)(nil)
