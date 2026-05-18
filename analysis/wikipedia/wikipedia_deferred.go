// Package wikipedia hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.wikipedia.
package wikipedia

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// WikipediaTokenizer mirrors org.apache.lucene.analysis.wikipedia.WikipediaTokenizer.
type WikipediaTokenizer struct{}

// NewWikipediaTokenizer builds a WikipediaTokenizer.
func NewWikipediaTokenizer() *WikipediaTokenizer { return &WikipediaTokenizer{} }

// WikipediaTokenizerFactory mirrors org.apache.lucene.analysis.wikipedia.WikipediaTokenizerFactory.
type WikipediaTokenizerFactory struct{}

// NewWikipediaTokenizerFactory builds a WikipediaTokenizerFactory.
func NewWikipediaTokenizerFactory() *WikipediaTokenizerFactory { return &WikipediaTokenizerFactory{} }
