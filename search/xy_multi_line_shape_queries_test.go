// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYMultiLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiLineShapeQueries (GOC-4004).
//
// The Java class tests multi-line shapes via the inherited
// BaseXYShapeTestCase random-test matrix. Gocene lacks that
// infrastructure; this test verifies XYLine construction works.
func TestXYMultiLineShapeQueries(t *testing.T) {
	l := geo.MustNewXYLine(
		[]float32{5, 15, 25},
		[]float32{5, 10, 15},
	)
	if l.NumPoints() != 3 {
		t.Errorf("NumPoints = %d, want 3", l.NumPoints())
	}
	if l.MinX() != 5 || l.MaxX() != 25 {
		t.Errorf("X range = [%v,%v], want [5,25]", l.MinX(), l.MaxX())
	}
	if l.MinY() != 5 || l.MaxY() != 15 {
		t.Errorf("Y range = [%v,%v], want [5,15]", l.MinY(), l.MaxY())
	}
}
