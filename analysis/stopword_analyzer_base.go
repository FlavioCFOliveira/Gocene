// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"os"
)

// StopwordAnalyzerBase is the base class for Analyzers that need to make use
// of stopword sets.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.StopwordAnalyzerBase
// (lucene/core/src/java/org/apache/lucene/analysis/StopwordAnalyzerBase.java).
// The Java class is abstract: a concrete analyzer embeds
// *StopwordAnalyzerBase and sets CreateComponents.
type StopwordAnalyzerBase struct {
	*BaseAnalyzer

	// Stopwords is the analyzer's immutable stopword set (the protected final
	// field stopwords). Gocene renders an unmodifiable CharArraySet as
	// *UnmodifiableCharArraySet.
	Stopwords *UnmodifiableCharArraySet
}

// GetStopwordSet returns the analyzer's stopword set or an empty set if the
// analyzer has no stopwords.
func (a *StopwordAnalyzerBase) GetStopwordSet() *UnmodifiableCharArraySet {
	return a.Stopwords
}

// NewStopwordAnalyzerBase creates a new instance initialized with the given
// stopword set (StopwordAnalyzerBase(CharArraySet)). A nil set yields the
// empty set; otherwise the analyzer keeps an unmodifiable copy.
func NewStopwordAnalyzerBase(stopwords *CharArraySet) *StopwordAnalyzerBase {
	// analyzers should use char array set for stopwords!
	var set *UnmodifiableCharArraySet
	if stopwords == nil {
		set = NewEmptyCharArraySet().UnmodifiableCharArraySet
	} else {
		set = UnmodifiableSet(CopySet(stopwords))
	}
	return &StopwordAnalyzerBase{
		BaseAnalyzer: NewAnalyzer(GlobalReuseStrategy),
		Stopwords:    set,
	}
}

// NewEmptyStopwordAnalyzerBase creates a new Analyzer with an empty stopword
// set (StopwordAnalyzerBase()).
func NewEmptyStopwordAnalyzerBase() *StopwordAnalyzerBase {
	return NewStopwordAnalyzerBase(nil)
}

// LoadStopwordSetFromPath creates a CharArraySet from a path
// (loadStopwordSet(Path)): the file is read as UTF-8 and parsed with
// WordlistLoader.getWordSet(Reader).
func LoadStopwordSetFromPath(stopwords string) (set *CharArraySet, err error) {
	reader, err := os.Open(stopwords)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil && err == nil {
			set, err = nil, cerr
		}
	}()
	return GetWordSet(reader)
}

// LoadStopwordSet creates a CharArraySet from a reader
// (loadStopwordSet(Reader)). The reader is closed afterwards when it
// implements io.Closer, as IOUtils.close does.
func LoadStopwordSet(stopwords io.Reader) (set *CharArraySet, err error) {
	defer func() {
		if c, ok := stopwords.(io.Closer); ok {
			if cerr := c.Close(); cerr != nil && err == nil {
				set, err = nil, cerr
			}
		}
	}()
	return GetWordSet(stopwords)
}
