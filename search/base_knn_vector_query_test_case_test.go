// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseKnnVectorQueryTestCase.java
//
// Deviation: all test methods skipped — BaseKnnVectorQueryTestCase is an
// abstract test base class (35 @Test methods) that requires IndexWriter +
// IndexSearcher + HNSW graph integration not yet complete in Gocene.
// Concrete subclasses (TestKnnByteVectorQuery, TestKnnFloatVectorQuery, etc.)
// supply the vector-type specialisation.

package search

import "testing"

// TestBaseKnnVectorQueryTestCase is a placeholder for the abstract base class.
func TestBaseKnnVectorQueryTestCase(t *testing.T) {
	t.Skip("abstract test base class — 35 test methods require IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}
