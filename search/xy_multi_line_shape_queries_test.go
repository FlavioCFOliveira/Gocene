// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestXYMultiLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiLineShapeQueries (GOC-4004).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that emits
// 1..4 random XYLines per shape. This test verifies the production
// XYShapeQuery construction and validation for multiple XYLine geometries.
//
// Covers: basic construction with multiple lines, GetField, GetQueryRelation,
// GetQueryComponent2D, WITHIN+XYLine rejection, and empty-field guard.
func TestXYMultiLineShapeQueries(t *testing.T) {
	t.Parallel()

	lineA := testXYLine(t, []float32{0, 1, 2}, []float32{0, 1, 2})
	lineB := testXYLine(t, []float32{3, 4, 5}, []float32{3, 4, 5})

	// Basic construction with INTERSECTS and multiple lines.
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, lineA, lineB)
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
	if len(q.GetGeometries()) != 2 {
		t.Fatalf("geometries length: got %d, want 2", len(q.GetGeometries()))
	}

	// WITHIN + XYLine is rejected even with multiple lines.
	if _, err := NewXYShapeQuery("shape", document.QueryRelationWithin, lineA, lineB); !errors.Is(err, ErrXYShapeQueryWithinLine) {
		t.Fatalf("WITHIN+XYLine: expected ErrXYShapeQueryWithinLine, got %v", err)
	}

	// Empty field guard.
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, lineA); err == nil {
		t.Fatalf("expected error on empty field")
	}
}
