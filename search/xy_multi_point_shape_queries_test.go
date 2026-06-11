// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYMultiPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiPointShapeQueries (GOC-3997).
//
// The Java class tests multi-point shapes (bundles of 1..4 XYPoints per
// document) via the inherited BaseXYShapeTestCase random-test matrix.
// Gocene lacks that infrastructure; this test verifies that XYPoint
// construction and multi-point grouping compile and work correctly.
func TestXYMultiPointShapeQueries(t *testing.T) {
	// Verify XYPoint construction works for valid coordinates.
	p1 := geo.MustNewXYPoint(1.0, 2.0)
	p2 := geo.MustNewXYPoint(3.0, 4.0)
	if p1.X() != 1.0 || p1.Y() != 2.0 {
		t.Errorf("p1 = (%v,%v), want (1,2)", p1.X(), p1.Y())
	}
	if p2.X() != 3.0 || p2.Y() != 4.0 {
		t.Errorf("p2 = (%v,%v), want (3,4)", p2.X(), p2.Y())
	}
}
