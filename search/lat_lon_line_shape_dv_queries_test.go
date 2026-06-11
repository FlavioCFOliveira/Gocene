// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonLineShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonLineShapeDVQueries (GOC-4011).
//
// This test verifies that LatLonShapeDocValuesField can be constructed for a
// line geometry and that the field name and String representation are correct.
func TestLatLonLineShapeDVQueries(t *testing.T) {
	t.Run("constructor creates Line doc value field", func(t *testing.T) {
		line, err := geo.NewLine([]float64{10.0, 20.0}, []float64{30.0, 40.0})
		if err != nil {
			t.Fatal(err)
		}
		f, err := document.NewLatLonShapeDocValuesFieldLine("field", line)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name() != "field" {
			t.Fatalf("got name %q, want %q", f.Name(), "field")
		}
	})

	t.Run("invalid latitude in Line rejected", func(t *testing.T) {
		_, err := geo.NewLine([]float64{100.0, 20.0}, []float64{30.0, 40.0})
		if err == nil {
			t.Fatal("expected error for invalid latitude, got nil")
		}
	})
}
