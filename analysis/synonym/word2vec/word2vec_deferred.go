// Package word2vec hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.synonym.word2vec.
package word2vec

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// Dl4jModelReader mirrors org.apache.lucene.analysis.synonym.word2vec.Dl4jModelReader.
type Dl4jModelReader struct{}

// NewDl4jModelReader builds a Dl4jModelReader.
func NewDl4jModelReader() *Dl4jModelReader { return &Dl4jModelReader{} }

// TermAndBoost mirrors org.apache.lucene.analysis.synonym.word2vec.TermAndBoost.
type TermAndBoost struct{}

// NewTermAndBoost builds a TermAndBoost.
func NewTermAndBoost() *TermAndBoost { return &TermAndBoost{} }

// Word2VecModel mirrors org.apache.lucene.analysis.synonym.word2vec.Word2VecModel.
type Word2VecModel struct{}

// NewWord2VecModel builds a Word2VecModel.
func NewWord2VecModel() *Word2VecModel { return &Word2VecModel{} }

// Word2VecSynonymProvider mirrors org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymProvider.
type Word2VecSynonymProvider struct{}

// NewWord2VecSynonymProvider builds a Word2VecSynonymProvider.
func NewWord2VecSynonymProvider() *Word2VecSynonymProvider { return &Word2VecSynonymProvider{} }

// Word2VecSynonymProviderFactory mirrors org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymProviderFactory.
type Word2VecSynonymProviderFactory struct{}

// NewWord2VecSynonymProviderFactory builds a Word2VecSynonymProviderFactory.
func NewWord2VecSynonymProviderFactory() *Word2VecSynonymProviderFactory {
	return &Word2VecSynonymProviderFactory{}
}

// Word2VecSynonymFilterFactory mirrors org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymFilterFactory.
type Word2VecSynonymFilterFactory struct{}

// NewWord2VecSynonymFilterFactory builds a Word2VecSynonymFilterFactory.
func NewWord2VecSynonymFilterFactory() *Word2VecSynonymFilterFactory {
	return &Word2VecSynonymFilterFactory{}
}

// Word2VecSynonymFilter mirrors org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymFilter.
type Word2VecSynonymFilter struct{}

// NewWord2VecSynonymFilter builds a Word2VecSynonymFilter.
func NewWord2VecSynonymFilter() *Word2VecSynonymFilter { return &Word2VecSynonymFilter{} }
