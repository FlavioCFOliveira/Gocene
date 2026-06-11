// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYLineShapeQueries (GOC-4014).
//
// The Java class is a subclass of BaseXYShapeTestCase that tests
// indexed XYLine queries. Gocene lacks the random-test harness;
// this test verifies XYLine construction and basic properties.
func TestXYLineShapeQueries(t *testing.T) {
	l := geo.MustNewXYLine(
		[]float32{0, 10, 20},
		[]float32{0, 10, 5},
	)
	if l.NumPoints() != 3 {
		t.Errorf("NumPoints = %d, want 3", l.NumPoints())
	}
	if l.X(0) != 0 || l.Y(0) != 0 {
		t.Errorf("vertex 0 = (%v,%v), want (0,0)", l.X(0), l.Y(0))
	}
	if l.X(2) != 20 || l.Y(2) != 5 {
		t.Errorf("vertex 2 = (%v,%v), want (20,5)", l.X(2), l.Y(2))
	}
}
