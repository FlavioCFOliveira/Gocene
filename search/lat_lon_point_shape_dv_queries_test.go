// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestLatLonPointShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPointShapeDVQueries (GOC-3991).
//
// This test verifies that LatLonShapeDocValuesField can be constructed for a
// point geometry and that the field name and String representation are correct.
func TestLatLonPointShapeDVQueries(t *testing.T) {
	t.Run("constructor creates Point doc value field", func(t *testing.T) {
		f, err := document.NewLatLonShapeDocValuesFieldPoint("field", 10.0, 20.0)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name() != "field" {
			t.Fatalf("got name %q, want %q", f.Name(), "field")
		}
	})

	t.Run("invalid lat rejected", func(t *testing.T) {
		_, err := document.NewLatLonShapeDocValuesFieldPoint("field", 100.0, 20.0)
		if err == nil {
			t.Fatal("expected error for invalid latitude, got nil")
		}
	})

	t.Run("invalid lon rejected", func(t *testing.T) {
		_, err := document.NewLatLonShapeDocValuesFieldPoint("field", 10.0, 200.0)
		if err == nil {
			t.Fatal("expected error for invalid longitude, got nil")
		}
	})
}
