// Package compound hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.compound.
package compound

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CompoundWordTokenFilterBase mirrors org.apache.lucene.analysis.compound.CompoundWordTokenFilterBase.
type CompoundWordTokenFilterBase struct{}

// NewCompoundWordTokenFilterBase builds a CompoundWordTokenFilterBase.
func NewCompoundWordTokenFilterBase() *CompoundWordTokenFilterBase {
	return &CompoundWordTokenFilterBase{}
}

// DictionaryCompoundWordTokenFilter mirrors org.apache.lucene.analysis.compound.DictionaryCompoundWordTokenFilter.
type DictionaryCompoundWordTokenFilter struct{}

// NewDictionaryCompoundWordTokenFilter builds a DictionaryCompoundWordTokenFilter.
func NewDictionaryCompoundWordTokenFilter() *DictionaryCompoundWordTokenFilter {
	return &DictionaryCompoundWordTokenFilter{}
}

// DictionaryCompoundWordTokenFilterFactory mirrors org.apache.lucene.analysis.compound.DictionaryCompoundWordTokenFilterFactory.
type DictionaryCompoundWordTokenFilterFactory struct{}

// NewDictionaryCompoundWordTokenFilterFactory builds a DictionaryCompoundWordTokenFilterFactory.
func NewDictionaryCompoundWordTokenFilterFactory() *DictionaryCompoundWordTokenFilterFactory {
	return &DictionaryCompoundWordTokenFilterFactory{}
}

// HyphenationCompoundWordTokenFilter mirrors org.apache.lucene.analysis.compound.HyphenationCompoundWordTokenFilter.
type HyphenationCompoundWordTokenFilter struct{}

// NewHyphenationCompoundWordTokenFilter builds a HyphenationCompoundWordTokenFilter.
func NewHyphenationCompoundWordTokenFilter() *HyphenationCompoundWordTokenFilter {
	return &HyphenationCompoundWordTokenFilter{}
}

// HyphenationCompoundWordTokenFilterFactory mirrors org.apache.lucene.analysis.compound.HyphenationCompoundWordTokenFilterFactory.
type HyphenationCompoundWordTokenFilterFactory struct{}

// NewHyphenationCompoundWordTokenFilterFactory builds a HyphenationCompoundWordTokenFilterFactory.
func NewHyphenationCompoundWordTokenFilterFactory() *HyphenationCompoundWordTokenFilterFactory {
	return &HyphenationCompoundWordTokenFilterFactory{}
}
