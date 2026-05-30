// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/BaseDocValuesSkipperTests.java
//
// Deviation: BaseDocValuesSkipperTests is an abstract test base class with no
// own @Test methods. Concrete subclasses provide a DocValuesSkipper factory.
// Skipped because all concrete subclass tests also require IndexWriter+IndexSearcher
// integration not yet complete in Gocene.

package search

import "testing"

// TestBaseDocValuesSkipperTests is a placeholder for the abstract base class.
func TestBaseDocValuesSkipperTests(t *testing.T) {
	t.Fatal("abstract test base class — concrete subclasses supply a DocValuesSkipper factory; requires IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
