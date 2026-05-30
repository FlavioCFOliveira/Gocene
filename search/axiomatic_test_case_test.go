// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/AxiomaticTestCase.java
//
// Deviation: AxiomaticTestCase is an abstract test base class (no own @Test
// methods) that extends BaseSimilarityTestCase. Concrete subclasses supply an
// Axiomatic similarity variant. Skipped because concrete subclass tests require
// IndexWriter+IndexSearcher integration not yet complete in Gocene.

package search

import "testing"

// TestAxiomaticTestCase is a placeholder for the abstract base class.
func TestAxiomaticTestCase(t *testing.T) {
	t.Fatal("abstract test base class — concrete subclasses supply an Axiomatic similarity variant; requires IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
