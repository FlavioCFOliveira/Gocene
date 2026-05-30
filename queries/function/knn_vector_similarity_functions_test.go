// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestKnnVectorSimilarityFunctions.java

package function

import "testing"

// TestKnnVectorSimilarityFunctions_All is skipped because it requires full KNN
// vector index infrastructure not yet complete in Gocene.
func TestKnnVectorSimilarityFunctions_All(t *testing.T) {
	t.Fatal("requires full KNN vector index infrastructure; deferred to backlog")
}
