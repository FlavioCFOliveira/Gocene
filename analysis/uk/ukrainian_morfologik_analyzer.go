// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/uk/UkrainianMorfologikAnalyzer.java

// Package uk provides analysis components for the Ukrainian language.
//
// It is the Go port of org.apache.lucene.analysis.uk.
package uk

import (
	_ "embed"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/morfologik"
)

//go:embed stopwords.txt
var ukStopwordsData string

// ukrainianNormMap is the package-level normalizer map, built once. It replicates
// the NORMALIZER_MAP static block from the Java source.
var ukrainianNormMap *analysis.NormalizeCharMap

func init() {
	ncm := analysis.NewNormalizeCharMap()
	// Different apostrophes normalised to ASCII single quote (U+0027).
	ncm.AddMappingString("’", "’") // RIGHT SINGLE QUOTATION MARK
	ncm.AddMappingString("‘", "’") // LEFT SINGLE QUOTATION MARK
	ncm.AddMappingString("ʼ", "’") // MODIFIER LETTER APOSTROPHE
	ncm.AddMappingString("`", "’")      // GRAVE ACCENT (backtick)
	ncm.AddMappingString("´", "’") // ACUTE ACCENT (´)
	// Ignored characters: COMBINING ACUTE ACCENT and SOFT HYPHEN.
	ncm.AddMappingString("́", "")
	ncm.AddMappingString("­", "")
	// Normalise Ґ/ґ → Г/г (not used in standard Ukrainian orthography).
	ncm.AddMappingString("ґ", "г") // ґ → г
	ncm.AddMappingString("Ґ", "Г") // Ґ → Г
	ukrainianNormMap = ncm
}

// cachedDefaultStopwords holds the parsed Ukrainian stop-word set, built in init().
var cachedDefaultStopwords *analysis.CharArraySet

func init() {
	// Parse the embedded stopwords.txt file. Format: one word per line.
	// Lines starting with '#' or empty after trimming are skipped
	// (Snowball format).
	set, err := analysis.GetSnowballWordSetFromReader(strings.NewReader(ukStopwordsData))
	if err != nil {
		// This is an invariant from the embedded resource; panic is appropriate.
		panic("uk: failed to parse embedded stopwords.txt: " + err.Error())
	}
	// Store as a mutable CharArraySet so we can wrap it in an unmodifiable view.
	cs := analysis.NewCharArraySet(set.Size(), false)
	set.ForEach(func(w string) bool {
		cs.Add(w)
		return true
	})
	cachedDefaultStopwords = cs
}

// GetDefaultStopwords returns an unmodifiable view of the default Ukrainian
// stop-word set. Mutations on the returned set are rejected.
func GetDefaultStopwords() *analysis.UnmodifiableCharArraySet {
	return analysis.UnmodifiableSet(cachedDefaultStopwords)
}

// UkrainianMorfologikAnalyzer is a dictionary-based [analysis.Analyzer] for
// Ukrainian text. It applies the following pipeline:
//
//	MappingCharFilter (apostrophe normalisation, accent removal, ґ→г)
//	→ StandardTokenizer
//	→ LowerCaseFilter
//	→ StopFilter
//	→ [SetKeywordMarkerFilter]  (only when stemExclusionSet is non-empty)
//	→ MorfologikFilter          (with the Ukrainian dictionary)
//
// This is the Go port of
// org.apache.lucene.analysis.uk.UkrainianMorfologikAnalyzer
// (Apache Lucene 10.4.0).
//
// # Deviation from Java
//
// The Java implementation loads the Ukrainian binary FSA dictionary from
// the "morfologik.ukrainian.search" JVM module. Go has no equivalent
// module system; callers must supply a [morfologik.Dictionary] implementation
// via [NewUkrainianMorfologikAnalyzerWithDict].
//
// The no-arg constructor [NewUkrainianMorfologikAnalyzer] is provided for
// API symmetry but returns a non-functional analyzer that errors on any
// TokenStream call until a dictionary is injected — callers must use
// [NewUkrainianMorfologikAnalyzerWithDict] for real analysis.
type UkrainianMorfologikAnalyzer struct {
	stopwords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
	dictionary       morfologik.Dictionary
}

// NewUkrainianMorfologikAnalyzer builds an analyzer with the default
// Ukrainian stopwords and an empty stem exclusion set. The dictionary must be
// supplied separately via [NewUkrainianMorfologikAnalyzerWithDict]; calling
// TokenStream on this analyzer panics until a dictionary is set.
//
// Mirrors the no-arg Java constructor.
func NewUkrainianMorfologikAnalyzer() *UkrainianMorfologikAnalyzer {
	stopSet := analysis.CopySet(cachedDefaultStopwords)
	return &UkrainianMorfologikAnalyzer{
		stopwords:        stopSet,
		stemExclusionSet: analysis.NewCharArraySet(0, false),
	}
}

// NewUkrainianMorfologikAnalyzerWithStopwords builds an analyzer with the
// given stopword set and no stem exclusion.
//
// Mirrors UkrainianMorfologikAnalyzer(CharArraySet).
func NewUkrainianMorfologikAnalyzerWithStopwords(stopwords *analysis.CharArraySet) *UkrainianMorfologikAnalyzer {
	return NewUkrainianMorfologikAnalyzerFull(stopwords, analysis.NewCharArraySet(0, false))
}

// NewUkrainianMorfologikAnalyzerFull builds an analyzer with custom stopwords
// and stem exclusion. If stemExclusionSet is non-empty, a
// SetKeywordMarkerFilter is inserted before the morfologik stemmer.
//
// Mirrors UkrainianMorfologikAnalyzer(CharArraySet, CharArraySet).
func NewUkrainianMorfologikAnalyzerFull(stopwords, stemExclusionSet *analysis.CharArraySet) *UkrainianMorfologikAnalyzer {
	if stopwords == nil {
		stopwords = analysis.NewCharArraySet(0, false)
	}
	if stemExclusionSet == nil {
		stemExclusionSet = analysis.NewCharArraySet(0, false)
	}
	sw := analysis.CopySet(stopwords)
	se := analysis.CopySet(stemExclusionSet)
	return &UkrainianMorfologikAnalyzer{
		stopwords:        sw,
		stemExclusionSet: se,
	}
}

// NewUkrainianMorfologikAnalyzerWithDict builds a fully configured analyzer
// using the provided dictionary.
func NewUkrainianMorfologikAnalyzerWithDict(dict morfologik.Dictionary) *UkrainianMorfologikAnalyzer {
	a := NewUkrainianMorfologikAnalyzer()
	a.dictionary = dict
	return a
}

// SetDictionary sets the Ukrainian dictionary used for morphological lookup.
// Must be called before any TokenStream invocation.
func (a *UkrainianMorfologikAnalyzer) SetDictionary(dict morfologik.Dictionary) {
	a.dictionary = dict
}

// TokenStream creates the full Ukrainian analysis pipeline for the given reader.
func (a *UkrainianMorfologikAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	if a.dictionary == nil {
		return nil, newNoDictionaryError()
	}

	// Apply the char normalisation filter first.
	normalizedReader := analysis.NewMappingCharFilter(ukrainianNormMap, reader)

	// StandardTokenizer
	src := analysis.NewStandardTokenizer()
	if err := src.SetReader(normalizedReader); err != nil {
		return nil, err
	}

	// LowerCaseFilter
	var stream analysis.TokenStream = analysis.NewLowerCaseFilter(src)

	// StopFilter
	stopWords := make([]string, 0, a.stopwords.Size())
	a.stopwords.ForEach(func(w string) bool {
		stopWords = append(stopWords, w)
		return true
	})
	stream = analysis.NewStopFilter(stream, stopWords)

	// Optional SetKeywordMarkerFilter
	if a.stemExclusionSet.Size() > 0 {
		stream = analysis.NewSetKeywordMarkerFilter(stream, a.stemExclusionSet)
	}

	// MorfologikFilter with the Ukrainian dictionary.
	stream = morfologik.NewMorfologikFilter(stream, a.dictionary.NewStemmer())

	return stream, nil
}

// Close is a no-op; UkrainianMorfologikAnalyzer holds no closeable resources.
func (a *UkrainianMorfologikAnalyzer) Close() error { return nil }

// Ensure UkrainianMorfologikAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*UkrainianMorfologikAnalyzer)(nil)

// noDictionaryError is returned when TokenStream is called without a dictionary.
type noDictionaryError struct{}

func (noDictionaryError) Error() string {
	return "UkrainianMorfologikAnalyzer: no dictionary set; call SetDictionary or use NewUkrainianMorfologikAnalyzerWithDict"
}

func newNoDictionaryError() error { return noDictionaryError{} }
