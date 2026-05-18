// Package collation hosts the deferred Sprint 28 ports for
// org.apache.lucene.collation.
package collation

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CollationDocValuesField mirrors org.apache.lucene.collation.CollationDocValuesField.
type CollationDocValuesField struct{}

// NewCollationDocValuesField builds a CollationDocValuesField.
func NewCollationDocValuesField() *CollationDocValuesField { return &CollationDocValuesField{} }

// CollationKeyAnalyzer mirrors org.apache.lucene.collation.CollationKeyAnalyzer.
type CollationKeyAnalyzer struct{}

// NewCollationKeyAnalyzer builds a CollationKeyAnalyzer.
func NewCollationKeyAnalyzer() *CollationKeyAnalyzer { return &CollationKeyAnalyzer{} }

// CollationAttributeFactory mirrors org.apache.lucene.collation.CollationAttributeFactory.
type CollationAttributeFactory struct{}

// NewCollationAttributeFactory builds a CollationAttributeFactory.
func NewCollationAttributeFactory() *CollationAttributeFactory { return &CollationAttributeFactory{} }

