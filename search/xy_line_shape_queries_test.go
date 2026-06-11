// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestXYLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYLineShapeQueries (GOC-4014).
//
// The Java class is a subclass of BaseXYShapeTestCase that drives a
// random-test harness Gocene lacks. This test verifies the production
// XYShapeQuery construction and validation for XYLine geometries instead.
//
// Covers: basic construction, GetField, GetQueryRelation, GetQueryComponent2D,
// WITHIN+XYLine rejection, and non-WITHIN relations.
func TestXYLineShapeQueries(t *testing.T) {
	t.Parallel()

	line := testXYLine(t, []float32{0, 1, 2}, []float32{0, 1, 2})

	// Basic construction with INTERSECTS relation.
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, line)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	if got := q.GetField(); got != "shape" {
		t.Fatalf("GetField: got %q, want %q", got, "shape")
	}
	if got := q.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want %v", got, document.QueryRelationIntersects)
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil")
	}
	if len(q.GetGeometries()) != 1 {
		t.Fatalf("geometries length: got %d, want 1", len(q.GetGeometries()))
	}

	// WITHIN + XYLine is rejected (ErrXYShapeQueryWithinLine).
	if _, err := NewXYShapeQuery("shape", document.QueryRelationWithin, line); !errors.Is(err, ErrXYShapeQueryWithinLine) {
		t.Fatalf("WITHIN+XYLine: expected ErrXYShapeQueryWithinLine, got %v", err)
	}

	// Non-WITHIN relations accept XYLine.
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewXYShapeQuery("shape", rel, line); err != nil {
				t.Fatalf("%v + XYLine: unexpected error %v", rel, err)
			}
		})
	}
}
