// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/morfologik/MorfologikAnalyzer.java

package morfologik

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Dictionary is the Go equivalent of morfologik.stemming.Dictionary.
//
// A Dictionary provides an [IStemmer] for performing morphological stem
// lookup. Implementations typically wrap a compiled Morfologik FSA binary
// (*.dict + *.info). The interface is intentionally minimal: Gocene does not
// ship binary dictionaries, so callers must supply their own implementation.
//
// Deviation from Java: Java's Dictionary is a concrete class backed by an
// FSA binary. Go cannot depend on the JVM library, so Dictionary is an
// interface that any backing implementation can satisfy.
type Dictionary interface {
	// NewStemmer returns a new [IStemmer] backed by this dictionary.
	NewStemmer() IStemmer
}

// MorfologikAnalyzer is an [analysis.Analyzer] that uses a Morfologik
// dictionary to transform tokens into their lemma form(s).
//
// This is the Go port of
// org.apache.lucene.analysis.morfologik.MorfologikAnalyzer
// (Apache Lucene 10.4.0).
//
// The analysis chain is:
//
//	StandardTokenizer → MorfologikFilter (with the provided dictionary)
type MorfologikAnalyzer struct {
	dictionary Dictionary
}

// NewMorfologikAnalyzer creates an analyzer backed by the given [Dictionary].
// dict must not be nil.
func NewMorfologikAnalyzer(dict Dictionary) *MorfologikAnalyzer {
	if dict == nil {
		panic("MorfologikAnalyzer: dictionary must not be nil")
	}
	return &MorfologikAnalyzer{dictionary: dict}
}

// TokenStream creates a [StandardTokenizer] → [MorfologikFilter] chain for
// the supplied reader. A fresh tokenizer and filter are created on every
// call (no component reuse), matching the Java reference.
func (a *MorfologikAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	src := analysis.NewStandardTokenizer()
	if err := src.SetReader(reader); err != nil {
		return nil, err
	}
	return NewMorfologikFilter(src, a.dictionary.NewStemmer()), nil
}

// Close is a no-op; MorfologikAnalyzer holds no closeable resources.
func (a *MorfologikAnalyzer) Close() error { return nil }

// Ensure MorfologikAnalyzer satisfies analysis.Analyzer.
var _ analysis.Analyzer = (*MorfologikAnalyzer)(nil)
