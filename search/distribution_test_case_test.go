// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/DistributionTestCase.java
//
// Deviation: DistributionTestCase is an abstract test base class (no own @Test
// methods) that extends BaseSimilarityTestCase. Concrete subclasses supply a
// Distribution implementation. Skipped because concrete subclasses also require
// IndexWriter+IndexSearcher integration not yet complete in Gocene.

package search

import "testing"

// TestDistributionTestCase is a placeholder for the abstract base class.
func TestDistributionTestCase(t *testing.T) {
	t.Skip("abstract test base class — concrete subclasses supply a Distribution impl; requires IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
