package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

type VectorScorer = util.VectorScorer
type Bulk = util.Bulk

// DocAndFloatFeatureBuffer is NOT declared by VectorScorer.java; it is its own
// Lucene class, org.apache.lucene.search.DocAndFloatFeatureBuffer, and is
// declared in doc_feature_buffers.go.
