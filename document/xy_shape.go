// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// XYShape provides static factories for indexing Cartesian shapes as
// tessellated triangle fields.
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.XYShape.
//
// Static query factories deferred — backlog #2697.

// CreateIndexableFieldsFromXYPolygon tessellates an XY polygon and
// returns the per-triangle ShapeFieldTriangle fields ready to be added
// to a Document.
//
// Lucene reuses the same earcut tessellator for both lat/lon and XY
// coordinate spaces; this function routes through geo.TessellateXY,
// which is the raw-coordinate entry point of the full tessellator port.
func CreateIndexableFieldsFromXYPolygon(fieldName string, xs, ys []float64) ([]*ShapeFieldTriangle, error) {
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("xs/ys length mismatch: %d vs %d", len(xs), len(ys))
	}
	if len(xs) < 3 {
		return nil, fmt.Errorf("XY polygon requires at least three vertices")
	}
	triangles, err := geo.TessellateXY(xs, ys, 0, false)
	if err != nil {
		return nil, fmt.Errorf("tessellate xy polygon: %w", err)
	}
	out := make([]*ShapeFieldTriangle, 0, len(triangles))
	for _, tri := range triangles {
		ax := geo.XYEncode(float32(tri.AX()))
		ay := geo.XYEncode(float32(tri.AY()))
		bx := geo.XYEncode(float32(tri.BX()))
		by := geo.XYEncode(float32(tri.BY()))
		cx := geo.XYEncode(float32(tri.CX()))
		cy := geo.XYEncode(float32(tri.CY()))
		field, err := NewShapeFieldTriangle(fieldName, ax, ay, bx, by, cx, cy,
			tri.EdgeFromPolygon(0), tri.EdgeFromPolygon(1), tri.EdgeFromPolygon(2))
		if err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	return out, nil
}

// CreateIndexableFieldsFromXYLine encodes an XY line as one degenerate
// triangle per segment.
func CreateIndexableFieldsFromXYLine(fieldName string, xs, ys []float32) ([]*ShapeFieldTriangle, error) {
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("xs/ys length mismatch: %d vs %d", len(xs), len(ys))
	}
	if len(xs) < 2 {
		return nil, fmt.Errorf("XY line requires at least two vertices")
	}
	out := make([]*ShapeFieldTriangle, 0, len(xs)-1)
	for i := 0; i+1 < len(xs); i++ {
		ax := geo.XYEncode(xs[i])
		ay := geo.XYEncode(ys[i])
		bx := geo.XYEncode(xs[i+1])
		by := geo.XYEncode(ys[i+1])
		field, err := NewShapeFieldTriangle(fieldName, ax, ay, bx, by, bx, by, true, false, false)
		if err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	return out, nil
}

// CreateIndexableFieldsFromXYPoint encodes an XY point as a degenerate
// triangle where all vertices coincide.
func CreateIndexableFieldsFromXYPoint(fieldName string, x, y float32) (*ShapeFieldTriangle, error) {
	if _, err := geo.XYCheckVal(x); err != nil {
		return nil, fmt.Errorf("invalid x: %w", err)
	}
	if _, err := geo.XYCheckVal(y); err != nil {
		return nil, fmt.Errorf("invalid y: %w", err)
	}
	xi := geo.XYEncode(x)
	yi := geo.XYEncode(y)
	return NewShapeFieldTriangle(fieldName, xi, yi, xi, yi, xi, yi, false, false, false)
}
