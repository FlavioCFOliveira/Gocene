// Package hunspell hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.hunspell.
package hunspell

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// DictEntries mirrors org.apache.lucene.analysis.hunspell.DictEntries.
type DictEntries struct{}

// NewDictEntries builds a DictEntries.
func NewDictEntries() *DictEntries { return &DictEntries{} }

// DictEntry mirrors org.apache.lucene.analysis.hunspell.DictEntry.
type DictEntry struct{}

// NewDictEntry builds a DictEntry.
func NewDictEntry() *DictEntry { return &DictEntry{} }

// EntrySuggestion mirrors org.apache.lucene.analysis.hunspell.EntrySuggestion.
type EntrySuggestion struct{}

// NewEntrySuggestion builds a EntrySuggestion.
func NewEntrySuggestion() *EntrySuggestion { return &EntrySuggestion{} }

// FragmentChecker mirrors org.apache.lucene.analysis.hunspell.FragmentChecker.
type FragmentChecker struct{}

// NewFragmentChecker builds a FragmentChecker.
func NewFragmentChecker() *FragmentChecker { return &FragmentChecker{} }

// HunspellStemFilter mirrors org.apache.lucene.analysis.hunspell.HunspellStemFilter.
type HunspellStemFilter struct{}

// NewHunspellStemFilter builds a HunspellStemFilter.
func NewHunspellStemFilter() *HunspellStemFilter { return &HunspellStemFilter{} }

// HunspellStemFilterFactory mirrors org.apache.lucene.analysis.hunspell.HunspellStemFilterFactory.
type HunspellStemFilterFactory struct{}

// NewHunspellStemFilterFactory builds a HunspellStemFilterFactory.
func NewHunspellStemFilterFactory() *HunspellStemFilterFactory { return &HunspellStemFilterFactory{} }

// NGramFragmentChecker mirrors org.apache.lucene.analysis.hunspell.NGramFragmentChecker.
type NGramFragmentChecker struct{}

// NewNGramFragmentChecker builds a NGramFragmentChecker.
func NewNGramFragmentChecker() *NGramFragmentChecker { return &NGramFragmentChecker{} }

// SortingStrategy mirrors org.apache.lucene.analysis.hunspell.SortingStrategy.
type SortingStrategy struct{}

// NewSortingStrategy builds a SortingStrategy.
func NewSortingStrategy() *SortingStrategy { return &SortingStrategy{} }

// SuggestionTimeoutException mirrors org.apache.lucene.analysis.hunspell.SuggestionTimeoutException.
type SuggestionTimeoutException struct{}

// NewSuggestionTimeoutException builds a SuggestionTimeoutException.
func NewSuggestionTimeoutException() *SuggestionTimeoutException { return &SuggestionTimeoutException{} }

// TimeoutPolicy mirrors org.apache.lucene.analysis.hunspell.TimeoutPolicy.
type TimeoutPolicy struct{}

// NewTimeoutPolicy builds a TimeoutPolicy.
func NewTimeoutPolicy() *TimeoutPolicy { return &TimeoutPolicy{} }

// Dictionary mirrors org.apache.lucene.analysis.hunspell.Dictionary.
type Dictionary struct{}

// NewDictionary builds a Dictionary.
func NewDictionary() *Dictionary { return &Dictionary{} }

// AffixedWord mirrors org.apache.lucene.analysis.hunspell.AffixedWord.
type AffixedWord struct{}

// NewAffixedWord builds a AffixedWord.
func NewAffixedWord() *AffixedWord { return &AffixedWord{} }

// Hunspell mirrors org.apache.lucene.analysis.hunspell.Hunspell.
type Hunspell struct{}

// NewHunspell builds a Hunspell.
func NewHunspell() *Hunspell { return &Hunspell{} }

// Suggester mirrors org.apache.lucene.analysis.hunspell.Suggester.
type Suggester struct{}

// NewSuggester builds a Suggester.
func NewSuggester() *Suggester { return &Suggester{} }

// WordFormGenerator mirrors org.apache.lucene.analysis.hunspell.WordFormGenerator.
type WordFormGenerator struct{}

// NewWordFormGenerator builds a WordFormGenerator.
func NewWordFormGenerator() *WordFormGenerator { return &WordFormGenerator{} }

