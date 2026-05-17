// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// ShapeDocValuesField stores a shape as binary doc-values, holding the
// already-tessellated triangle list. Mirrors the surface of Lucene
// 10.4.0's org.apache.lucene.document.ShapeDocValuesField.
//
// Lucene's ShapeDocValuesField writes a custom serialised format
// (centroid, bbox, encoded triangle list). The Gocene port currently
// ships the constructor + name/bbox/triangle-count accessors; full
// byte-for-byte serialisation is deferred to the geo query/scorer sprint
// (backlog #2697).
type ShapeDocValuesField struct {
	*BinaryDocValuesField
	numTriangles int
}

// NewShapeDocValuesField creates a new ShapeDocValuesField from raw
// triangle bytes. Callers typically obtain the triangle list from the
// tessellator. Each triangle occupies ShapeFieldBytes (28 bytes).
func NewShapeDocValuesField(name string, triangles []byte) (*ShapeDocValuesField, error) {
	b, err := NewBinaryDocValuesField(name, triangles)
	if err != nil {
		return nil, err
	}
	return &ShapeDocValuesField{
		BinaryDocValuesField: b,
		numTriangles:         len(triangles) / ShapeFieldBytes,
	}, nil
}

// NumTriangles returns the count of encoded triangles in the field's
// payload.
func (f *ShapeDocValuesField) NumTriangles() int { return f.numTriangles }
