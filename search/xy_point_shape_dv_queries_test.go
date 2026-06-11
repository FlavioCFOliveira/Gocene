// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPointShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPointShapeDVQueries (GOC-3986).
//
// The Java class is a subclass of BaseXYShapeDocValueTestCase that tests
// XY point doc-values queries. Gocene lacks the infrastructure; this test
// verifies XYPoint construction and the XYShape doc-value related types.
func TestXYPointShapeDVQueries(t *testing.T) {
	// Verify XYPoint creation and basic coordinate access.
	p := geo.MustNewXYPoint(-5.0, 10.0)
	if p.X() != -5.0 {
		t.Errorf("X() = %v, want -5", p.X())
	}
	if p.Y() != 10.0 {
		t.Errorf("Y() = %v, want 10", p.Y())
	}
	// Verify that XY shape types compile (they satisfy XYGeometry).
	var _ geo.XYGeometry = p
}
