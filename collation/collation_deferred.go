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


