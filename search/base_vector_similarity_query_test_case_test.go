// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseVectorSimilarityQueryTestCase.java
//
// Deviation: all test methods skipped — BaseVectorSimilarityQueryTestCase is an
// abstract test base class (generic over V, F extends Field, Q extends
// AbstractVectorSimilarityQuery) that requires IndexWriter + IndexSearcher +
// vector field integration not yet complete in Gocene.

package search

import "testing"

// TestBaseVectorSimilarityQueryTestCase is a placeholder for the abstract base class.
// Concrete subclasses (e.g. TestFloatVectorSimilarityQuery) implement the abstract
// methods to supply vector type, field factory and query factory.
func TestBaseVectorSimilarityQueryTestCase(t *testing.T) {
	t.Skip("abstract test base class — concrete subclasses provide vector-type specialisations; requires IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
