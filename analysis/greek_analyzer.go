// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// GreekStopWords contains common Greek stop words.
var GreekStopWords = []string{
	"αλλά", "αν", "αντι", "από", "αυτά", "αυτές", "αυτή", "αυτό", "αυτοί", "αυτός",
	"αυτούς", "αυτών", "για", "δεν", "εάν", "είμαστε", "είσαι", "είστε", "εκείνα",
	"εκείνες", "εκείνη", "εκείνο", "εκείνοι", "εκείνος", "εκείνους", "εκείνων",
	"ενώ", "επί", "η", "θα", "ίσως", "και", "κάποια", "κάποιες", "κάποιο", "κάποιοι",
	"κάποιος", "κάποιους", "κάποιων", "κάτι", "κι", "μα", "με", "μέσα", "μη", "μην",
	"μια", "μιας", "μόνο", "μου", "να", "ο", "οι", "όλα", "όλες", "όλη", "όλο",
	"όλοι", "όλος", "όλους", "όλων", "όμως", "όπου", "όσο", "όταν", "ότι", "παρά",
	"πριν", "προς", "πως", "σαν", "σας", "σε", "σου", "στη", "στην", "στο", "στον",
	"στα", "στις", "στους", "στο", "τα", "τη", "την", "της", "τι", "τις", "το",
	"τον", "του", "τους", "των", "ώστε",
}

// GreekAnalyzer is an analyzer for Greek language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.el.GreekAnalyzer.
//
// GreekAnalyzer uses the StandardTokenizer with Greek stop words removal
// and light stemming.
type GreekAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewGreekAnalyzer creates a new GreekAnalyzer with default Greek stop words.
func NewGreekAnalyzer() *GreekAnalyzer {
	stopSet := GetWordSetFromStrings(GreekStopWords, true)
	return NewGreekAnalyzerWithWords(stopSet)
}

// NewGreekAnalyzerWithWords creates a GreekAnalyzer with custom stop words.
func NewGreekAnalyzerWithWords(stopWords *CharArraySet) *GreekAnalyzer {
	a := &GreekAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewGreekLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *GreekAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *GreekAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *GreekAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure GreekAnalyzer implements Analyzer
var _ Analyzer = (*GreekAnalyzer)(nil)
var _ AnalyzerInterface = (*GreekAnalyzer)(nil)

// GreekLightStemFilter implements light stemming for Greek.
type GreekLightStemFilter struct {
	*BaseTokenFilter
}

// NewGreekLightStemFilter creates a new GreekLightStemFilter.
func NewGreekLightStemFilter(input TokenStream) *GreekLightStemFilter {
	return &GreekLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *GreekLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := greekLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// greekLightStem applies light Greek stemming.
func greekLightStem(term string) string {
	if len(term) < 3 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Greek suffixes
	switch {
	// -ος, -ης, -ας (masculine endings)
	case length > 2 && (runes[length-1] == 'ς' || runes[length-1] == 'σ') &&
		(runes[length-2] == 'ο' || runes[length-2] == 'η' || runes[length-2] == 'α'):
		return string(runes[:length-2])
	// -ες, -ες (plural)
	case length > 2 && runes[length-1] == 'ς' && runes[length-2] == 'ε':
		return string(runes[:length-2])
	// -α, -η (feminine endings)
	case length > 1 && (runes[length-1] == 'α' || runes[length-1] == 'η'):
		return string(runes[:length-1])
	// -ο, -ου (neuter endings)
	case length > 1 && (runes[length-1] == 'ο' || runes[length-1] == 'υ'):
		return string(runes[:length-1])
	}

	return term
}

// GreekLightStemFilterFactory creates GreekLightStemFilter instances.
type GreekLightStemFilterFactory struct{}

// NewGreekLightStemFilterFactory creates a new GreekLightStemFilterFactory.
func NewGreekLightStemFilterFactory() *GreekLightStemFilterFactory {
	return &GreekLightStemFilterFactory{}
}

// Create creates a new GreekLightStemFilter.
func (f *GreekLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGreekLightStemFilter(input)
}

// Ensure GreekLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*GreekLightStemFilterFactory)(nil)
