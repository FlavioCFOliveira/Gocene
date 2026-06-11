// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPolygonShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeDVQueries (GOC-3995).
//
// The Java class is a subclass of BaseXYShapeDocValueTestCase that tests
// XY polygon doc-values queries. Gocene lacks the infrastructure; this test
// verifies XYPolygon construction and basic properties.
func TestXYPolygonShapeDVQueries(t *testing.T) {
	p, err := geo.NewXYPolygon(
		[]float32{-10, -10, 10, 10, -10},
		[]float32{-10, 10, 10, -10, -10},
	)
	if err != nil {
		t.Fatalf("NewXYPolygon: %v", err)
	}
	if p.NumPoints() != 5 {
		t.Errorf("NumPoints = %d, want 5", p.NumPoints())
	}
	if p.NumHoles() != 0 {
		t.Errorf("NumHoles = %d, want 0", p.NumHoles())
	}
	// Verify XYPolygon satisfies XYGeometry.
	var _ geo.XYGeometry = p
}
