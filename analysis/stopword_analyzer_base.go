// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// StopwordAnalyzerBase is a base class for Analyzers that need to make use of stopword sets.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.StopwordAnalyzerBase.
type StopwordAnalyzerBase struct {
	Stopwords *CharArraySet
}

func (a *StopwordAnalyzerBase) GetStopwordSet() *CharArraySet {
	return a.Stopwords
}

// LoadStopwordSet creates a CharArraySet from a reader.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.StopwordAnalyzerBase.loadStopwordSet.
func LoadStopwordSet(reader io.Reader) (*CharArraySet, error) {
	return GetWordSet(reader)
}
