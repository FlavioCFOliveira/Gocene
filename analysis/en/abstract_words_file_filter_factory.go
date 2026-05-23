// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package en

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// FormatWordset is the name for the wordset format (one word per line, # comments).
const FormatWordset = "wordset"

// FormatSnowball is the name for the Snowball format (multiple words per line,
// vertical-bar comments).
const FormatSnowball = "snowball"

// AbstractWordsFileFilterFactory is the abstract base class for token-filter
// factories that accept a stopwords / wordlist file as configuration input.
//
// Go port of org.apache.lucene.analysis.en.AbstractWordsFileFilterFactory
// (Apache Lucene 10.4.0).
//
// Concrete sub-types must embed this struct and call Init, then override
// CreateDefaultWords to supply a built-in word set when no external file
// is configured.
//
// Supported configuration keys (all optional):
//   - ignoreCase  – defaults to false
//   - words       – path to a stopwords file
//   - format      – "wordset" (default) or "snowball"
type AbstractWordsFileFilterFactory struct {
	words      *analysis.CharArraySet
	wordFiles  string
	format     string
	ignoreCase bool
}

// Init populates the factory fields from the provided configuration map.
// Call this from the concrete factory's constructor.
func (f *AbstractWordsFileFilterFactory) Init(args map[string]string) {
	f.wordFiles = args["words"]
	if f.wordFiles != "" {
		if v, ok := args["format"]; ok {
			f.format = v
		} else {
			f.format = FormatWordset
		}
	}
	if v, ok := args["ignoreCase"]; ok {
		f.ignoreCase = v == "true"
	}
}

// SetWords directly sets the word set (useful when loading from an embedded
// default rather than an external file).
func (f *AbstractWordsFileFilterFactory) SetWords(ws *analysis.CharArraySet) {
	f.words = ws
}

// Words returns the resolved word set.
func (f *AbstractWordsFileFilterFactory) Words() *analysis.CharArraySet {
	return f.words
}

// WordFiles returns the configured words file path.
func (f *AbstractWordsFileFilterFactory) WordFiles() string { return f.wordFiles }

// Format returns the configured format string.
func (f *AbstractWordsFileFilterFactory) Format() string { return f.format }

// IgnoreCase reports whether case-insensitive matching is enabled.
func (f *AbstractWordsFileFilterFactory) IgnoreCase() bool { return f.ignoreCase }
