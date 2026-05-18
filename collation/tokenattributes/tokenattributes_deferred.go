// Package tokenattributes hosts the deferred Sprint 28 ports for
// org.apache.lucene.collation.tokenattributes.
package tokenattributes

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CollatedTermAttributeImpl mirrors org.apache.lucene.collation.tokenattributes.CollatedTermAttributeImpl.
type CollatedTermAttributeImpl struct{}

// NewCollatedTermAttributeImpl builds a CollatedTermAttributeImpl.
func NewCollatedTermAttributeImpl() *CollatedTermAttributeImpl { return &CollatedTermAttributeImpl{} }

