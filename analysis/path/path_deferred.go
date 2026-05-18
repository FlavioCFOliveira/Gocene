// Package path hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.path.
package path

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ReversePathHierarchyTokenizer mirrors org.apache.lucene.analysis.path.ReversePathHierarchyTokenizer.
type ReversePathHierarchyTokenizer struct{}

// NewReversePathHierarchyTokenizer builds a ReversePathHierarchyTokenizer.
func NewReversePathHierarchyTokenizer() *ReversePathHierarchyTokenizer { return &ReversePathHierarchyTokenizer{} }

