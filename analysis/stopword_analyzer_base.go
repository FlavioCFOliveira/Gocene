// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"os"
)

// StopwordAnalyzerBase is the base type for Analyzers that need access to a
// stop-word set.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.StopwordAnalyzerBase. It stores an immutable
// (Lucene-equivalent: unmodifiable) CharArraySet of stop words and exposes
// it via Stopwords() / GetStopwordSet().
//
// Gocene's Analyzer is an interface rather than an abstract class, so this
// type is intended to be embedded in concrete Analyzer implementations (for
// example StopAnalyzer) to inherit the stop-word storage and the
// LoadStopwordSet helpers — not as a stand-alone Analyzer.
type StopwordAnalyzerBase struct {
	// stopwords holds the analyzer's stop-word set. It is an unmodifiable
	// wrapper around a defensively-copied CharArraySet.
	stopwords *UnmodifiableCharArraySet
}

// NewStopwordAnalyzerBase creates a new StopwordAnalyzerBase initialised with
// the given stop-word set. If stopwords is nil an empty set is used. The
// input set is defensively copied so later mutations do not affect this
// analyzer.
func NewStopwordAnalyzerBase(stopwords *CharArraySet) *StopwordAnalyzerBase {
	if stopwords == nil {
		return &StopwordAnalyzerBase{
			stopwords: NewUnmodifiableCharArraySet(NewCharArraySet(0, false)),
		}
	}
	return &StopwordAnalyzerBase{
		stopwords: UnmodifiableSet(CopySet(stopwords)),
	}
}

// NewEmptyStopwordAnalyzerBase creates a StopwordAnalyzerBase with an empty
// stop-word set. Mirrors Lucene's protected no-arg constructor.
func NewEmptyStopwordAnalyzerBase() *StopwordAnalyzerBase {
	return NewStopwordAnalyzerBase(nil)
}

// Stopwords returns the analyzer's immutable stop-word set.
func (s *StopwordAnalyzerBase) Stopwords() *UnmodifiableCharArraySet {
	return s.stopwords
}

// GetStopwordSet is the Lucene-name alias for Stopwords. Provided for
// consumers that want a name closer to the original Java API.
func (s *StopwordAnalyzerBase) GetStopwordSet() *UnmodifiableCharArraySet {
	return s.stopwords
}

// LoadStopwordSetFromPath reads a stop-word list from the file at the given
// path (one word per line, UTF-8 encoded) and returns it as a CharArraySet.
// Empty lines are skipped. Mirrors Lucene's protected static
// loadStopwordSet(Path) helper.
func LoadStopwordSetFromPath(path string) (*CharArraySet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	set := NewCharArraySet(initialCapacity, false)
	if _, err := GetWordSet(f, set); err != nil {
		return nil, err
	}
	return set, nil
}

// LoadStopwordSetFromReader reads a stop-word list from the given Reader
// (one word per line, UTF-8 encoded) and returns it as a CharArraySet.
// Mirrors Lucene's protected static loadStopwordSet(Reader) helper. The
// reader is not closed by this function — callers retain ownership.
func LoadStopwordSetFromReader(reader io.Reader) (*CharArraySet, error) {
	set := NewCharArraySet(initialCapacity, false)
	if _, err := GetWordSet(reader, set); err != nil {
		return nil, err
	}
	return set, nil
}
