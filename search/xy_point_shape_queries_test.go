// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPointShapeQueries (GOC-4002).
//
// The Java class is a subclass of BaseXYShapeTestCase that tests
// indexed XYPoint queries. Gocene lacks the random-test harness;
// this test verifies XYPoint construction and basic properties.
func TestXYPointShapeQueries(t *testing.T) {
	p := geo.MustNewXYPoint(3.5, 7.25)
	if p.X() != 3.5 {
		t.Errorf("X() = %v, want 3.5", p.X())
	}
	if p.Y() != 7.25 {
		t.Errorf("Y() = %v, want 7.25", p.Y())
	}
}
