// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package query provides query-time analysis utilities.
package query

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// IndexReaderForAutoStop is the subset of index.IndexReader required by
// QueryAutoStopWordAnalyzer: document count and per-field Terms access.
//
// Both *index.IndexReader (and its subtypes) satisfy this interface.
type IndexReaderForAutoStop interface {
	NumDocs() int
	Terms(field string) (index.Terms, error)
}

// DefaultMaxDocFreqPercent is the default maximum fraction (40 %) of index
// documents that may contain a term before it is promoted to a stop word.
const DefaultMaxDocFreqPercent float64 = 0.4

// QueryAutoStopWordAnalyzer wraps a delegate Analyzer and, at construction
// time, computes per-field stop-word sets by scanning the provided
// IndexReader. Any term whose document frequency exceeds the configured
// threshold is added to the stop-word set for its field. The per-field
// stop-word set is then applied via a StopFilter injected into the delegate's
// token stream.
//
// This is the Go port of
// org.apache.lucene.analysis.query.QueryAutoStopWordAnalyzer from
// Apache Lucene 10.4.0.
//
// Deviation: Lucene's implementation extends AnalyzerWrapper and overrides
// wrapComponents, which reconstructs TokenStreamComponents. Gocene's
// AnalyzerWrapper uses function fields rather than subclassing; therefore
// QueryAutoStopWordAnalyzer embeds *analysis.AnalyzerWrapper directly and
// configures WrapTokenStream at construction time to inject the StopFilter.
//
// Deviation: FieldInfos.getIndexedFields(IndexReader) does not yet exist in
// Gocene. Callers must pass the field list explicitly when using
// NewQueryAutoStopWordAnalyzerWithFields; the convenience constructors that
// discover fields automatically are absent until that helper lands.
type QueryAutoStopWordAnalyzer struct {
	*analysis.AnalyzerWrapper

	delegate          analysis.Analyzer
	stopWordsPerField map[string][]string
}

// NewQueryAutoStopWordAnalyzer creates a QueryAutoStopWordAnalyzer using the
// default threshold (DefaultMaxDocFreqPercent) applied to the given fields.
//
// fields is the list of indexed field names to scan for high-frequency terms.
func NewQueryAutoStopWordAnalyzer(
	delegate analysis.Analyzer,
	reader IndexReaderForAutoStop,
	fields []string,
) (*QueryAutoStopWordAnalyzer, error) {
	maxDocFreq := int(float64(reader.NumDocs()) * DefaultMaxDocFreqPercent)
	return NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, fields, maxDocFreq)
}

// NewQueryAutoStopWordAnalyzerWithPercent creates a QueryAutoStopWordAnalyzer
// using a custom percentage threshold applied to the given fields.
//
// maxPercentDocs must be in [0.0, 1.0].
func NewQueryAutoStopWordAnalyzerWithPercent(
	delegate analysis.Analyzer,
	reader IndexReaderForAutoStop,
	fields []string,
	maxPercentDocs float64,
) (*QueryAutoStopWordAnalyzer, error) {
	maxDocFreq := int(float64(reader.NumDocs()) * maxPercentDocs)
	return NewQueryAutoStopWordAnalyzerWithMaxDocFreq(delegate, reader, fields, maxDocFreq)
}

// NewQueryAutoStopWordAnalyzerWithMaxDocFreq creates a
// QueryAutoStopWordAnalyzer using a concrete document-frequency threshold
// applied to the given fields.
//
// Any term with docFreq > maxDocFreq is treated as a stop word.
func NewQueryAutoStopWordAnalyzerWithMaxDocFreq(
	delegate analysis.Analyzer,
	reader IndexReaderForAutoStop,
	fields []string,
	maxDocFreq int,
) (*QueryAutoStopWordAnalyzer, error) {
	stopWordsPerField := make(map[string][]string, len(fields))

	for _, field := range fields {
		terms, err := reader.Terms(field)
		if err != nil {
			return nil, err
		}
		if terms == nil {
			stopWordsPerField[field] = nil
			continue
		}

		te, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}

		var stopWords []string
		for {
			t, err := te.Next()
			if err != nil {
				return nil, err
			}
			if t == nil {
				break
			}
			df, err := te.DocFreq()
			if err != nil {
				return nil, err
			}
			if df > maxDocFreq {
				stopWords = append(stopWords, t.Text())
			}
		}
		stopWordsPerField[field] = stopWords
	}

	a := &QueryAutoStopWordAnalyzer{
		delegate:          delegate,
		stopWordsPerField: stopWordsPerField,
	}

	wrapper := analysis.NewAnalyzerWrapper(func(fieldName string) analysis.Analyzer {
		return delegate
	})
	wrapper.WrapTokenStream = func(fieldName string, in analysis.TokenStream) analysis.TokenStream {
		stopWords := a.stopWordsPerField[fieldName]
		if len(stopWords) == 0 {
			return in
		}
		return analysis.NewStopFilter(in, stopWords)
	}

	a.AnalyzerWrapper = wrapper
	return a, nil
}

// GetStopWordsForField returns the stop words identified for the given field.
// Returns an empty slice if no stop words were computed for that field.
func (a *QueryAutoStopWordAnalyzer) GetStopWordsForField(fieldName string) []string {
	sw := a.stopWordsPerField[fieldName]
	if sw == nil {
		return []string{}
	}
	result := make([]string, len(sw))
	copy(result, sw)
	return result
}

// GetStopWords returns all stop words across all fields as index.Term values.
func (a *QueryAutoStopWordAnalyzer) GetStopWords() []*index.Term {
	var all []*index.Term
	for field, words := range a.stopWordsPerField {
		for _, text := range words {
			all = append(all, index.NewTerm(field, text))
		}
	}
	return all
}

// TokenStream delegates to the wrapped AnalyzerWrapper, which injects the
// per-field StopFilter via WrapTokenStream.
func (a *QueryAutoStopWordAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.AnalyzerWrapper.TokenStream(fieldName, reader)
}

// Close releases resources. The delegate Analyzer lifecycle is owned by the caller.
func (a *QueryAutoStopWordAnalyzer) Close() error {
	return a.AnalyzerWrapper.Close()
}

// Ensure QueryAutoStopWordAnalyzer satisfies the Analyzer interface.
var _ analysis.Analyzer = (*QueryAutoStopWordAnalyzer)(nil)
