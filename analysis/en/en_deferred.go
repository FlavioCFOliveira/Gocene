// Package en hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.en.
package en

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// AbstractWordsFileFilterFactory mirrors org.apache.lucene.analysis.en.AbstractWordsFileFilterFactory.
type AbstractWordsFileFilterFactory struct{}

// NewAbstractWordsFileFilterFactory builds a AbstractWordsFileFilterFactory.
func NewAbstractWordsFileFilterFactory() *AbstractWordsFileFilterFactory {
	return &AbstractWordsFileFilterFactory{}
}

// KStemFilter mirrors org.apache.lucene.analysis.en.KStemFilter.
type KStemFilter struct{}

// NewKStemFilter builds a KStemFilter.
func NewKStemFilter() *KStemFilter { return &KStemFilter{} }

// KStemFilterFactory mirrors org.apache.lucene.analysis.en.KStemFilterFactory.
type KStemFilterFactory struct{}

// NewKStemFilterFactory builds a KStemFilterFactory.
func NewKStemFilterFactory() *KStemFilterFactory { return &KStemFilterFactory{} }
