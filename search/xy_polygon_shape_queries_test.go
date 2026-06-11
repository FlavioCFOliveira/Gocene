// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeQueries (GOC-4009).
//
// The Java class is a subclass of BaseXYShapeTestCase that tests
// indexed XYPolygon queries. Gocene lacks the random-test harness;
// this test verifies XYPolygon construction and basic properties.
func TestXYPolygonShapeQueries(t *testing.T) {
	// Verify XYPolygon construction works for a closed rectangle.
	p, err := geo.NewXYPolygon(
		[]float32{-5, -5, 5, 5, -5},
		[]float32{-5, 5, 5, -5, -5},
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
}
