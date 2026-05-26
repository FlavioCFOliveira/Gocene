// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// ShapeFieldBytes is the per-triangle byte width: 7 dimensions × 4 bytes.
// Matches Lucene 10.4.0's ShapeField.BYTES = 7 * Integer.BYTES = 28.
const ShapeFieldBytes = 7 * 4

// QueryRelation enumerates the four spatial relations supported by the
// Shape query family. Mirrors Lucene 10.4.0's ShapeField.QueryRelation.
type QueryRelation int

const (
	// QueryRelationIntersects matches shapes overlapping the query region.
	QueryRelationIntersects QueryRelation = iota
	// QueryRelationWithin matches shapes fully inside the query region.
	QueryRelationWithin
	// QueryRelationContains matches shapes that contain the query region.
	QueryRelationContains
	// QueryRelationDisjoint matches shapes that do not overlap the query.
	QueryRelationDisjoint
)

// String returns the Lucene-canonical name.
func (r QueryRelation) String() string {
	switch r {
	case QueryRelationIntersects:
		return "INTERSECTS"
	case QueryRelationWithin:
		return "WITHIN"
	case QueryRelationContains:
		return "CONTAINS"
	case QueryRelationDisjoint:
		return "DISJOINT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(r))
	}
}

// DecodedTriangleType discriminates the shape carried by a DecodedTriangle.
type DecodedTriangleType int

const (
	// DecodedTriangleTypePoint is a degenerate single-vertex triangle.
	DecodedTriangleTypePoint DecodedTriangleType = iota
	// DecodedTriangleTypeLine is a degenerate two-vertex triangle.
	DecodedTriangleTypeLine
	// DecodedTriangleTypeTriangle is a non-degenerate triangle.
	DecodedTriangleTypeTriangle
)

// DecodedTriangle is the result of decoding a 28-byte ShapeField payload
// back to a triangle. Mirrors Lucene's ShapeField.DecodedTriangle. The
// three vertices are stored as already-decoded int32 values per
// dimension (caller may run geo.DecodeLatitude/Longitude or XYDecode to
// recover degrees).
type DecodedTriangle struct {
	Kind                   DecodedTriangleType
	AX, AY, BX, BY, CX, CY int32
	AB, BC, CA             bool
}

// ShapeFieldTriangle is the underlying Field used to index a single
// triangle produced by the tessellator.  High-level factories such as
// CreateIndexableFieldsFromLatLonPolygon tessellate the input polygon
// and return a slice of ShapeFieldTriangle values, one per triangle.
type ShapeFieldTriangle struct {
	*Field
}

var (
	// ShapeFieldType is the FieldType for ShapeField triangles.
	// dimensionCount=7, numBytes=4.
	ShapeFieldType *FieldType

	// ShapeFieldTYPE is the Lucene-canonical alias.
	ShapeFieldTYPE *FieldType
)

func init() {
	ShapeFieldType = NewFieldType()
	ShapeFieldType.SetIndexed(true)
	ShapeFieldType.SetDimensions(7, 4)
	ShapeFieldType.Freeze()
	ShapeFieldTYPE = ShapeFieldType
}

// NewShapeFieldTriangle creates a ShapeFieldTriangle from raw int32 vertex
// coordinates and edge-membership bits. The caller is responsible for
// pre-encoding lat/lon or XY values via the appropriate util helpers.
//
// This is the low-level constructor; high-level NewLatLonShape /
// NewXYShape constructors that tessellate polygons depend on the full
// tessellator (backlog #2697).
func NewShapeFieldTriangle(name string, ax, ay, bx, by, cx, cy int32, ab, bc, ca bool) (*ShapeFieldTriangle, error) {
	buf, err := EncodeTriangle(ax, ay, bx, by, cx, cy, ab, bc, ca)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, buf, ShapeFieldType)
	if err != nil {
		return nil, err
	}
	return &ShapeFieldTriangle{Field: field}, nil
}

// EncodeTriangle encodes a triangle into the 28-byte ShapeField wire
// format. The vertices are NOT rotated/oriented here — callers should
// preserve Lucene's "min X first, counter-clockwise" invariant if exact
// byte parity with JVM indices is required. The current implementation
// targets structural correctness; the rotation/orientation normalisation
// step belongs to the deferred tessellator integration (backlog #2697).
func EncodeTriangle(ax, ay, bx, by, cx, cy int32, ab, bc, ca bool) ([]byte, error) {
	buf := make([]byte, ShapeFieldBytes)
	put := func(off int, v int32) {
		buf[off] = byte(v >> 24)
		buf[off+1] = byte(v >> 16)
		buf[off+2] = byte(v >> 8)
		buf[off+3] = byte(v)
	}
	// Bounding box: minY, minX, maxY, maxX (Lucene order).
	minX := ax
	if bx < minX {
		minX = bx
	}
	if cx < minX {
		minX = cx
	}
	maxX := ax
	if bx > maxX {
		maxX = bx
	}
	if cx > maxX {
		maxX = cx
	}
	minY := ay
	if by < minY {
		minY = by
	}
	if cy < minY {
		minY = cy
	}
	maxY := ay
	if by > maxY {
		maxY = by
	}
	if cy > maxY {
		maxY = cy
	}
	put(0, minY)
	put(4, minX)
	put(8, maxY)
	put(12, maxX)
	// Sixth dimension carries vertex Ay/Ax (one vertex) — Lucene packs
	// AX/AY into dims 5+6, edges into dim 7's low bits.
	put(16, ay)
	put(20, ax)
	bits := int32(0)
	if ab {
		bits |= 1
	}
	if bc {
		bits |= 2
	}
	if ca {
		bits |= 4
	}
	put(24, bits)
	return buf, nil
}

// DecodeTriangle decodes a 28-byte triangle back to a DecodedTriangle.
// Note: the high-level Lucene encoder rotates vertices for canonical
// orientation; the corresponding decode reads the encoded bits and
// reconstructs A/B/C. The Gocene encoder above writes a simplified layout
// (no rotation), so DecodeTriangle is the matching inverse for that
// simplified layout. Full Lucene-byte-compatible decode is deferred.
func DecodeTriangle(buf []byte) (DecodedTriangle, error) {
	if len(buf) != ShapeFieldBytes {
		return DecodedTriangle{}, fmt.Errorf("ShapeField payload must be %d bytes; got %d", ShapeFieldBytes, len(buf))
	}
	get := func(off int) int32 {
		return int32(buf[off])<<24 | int32(buf[off+1])<<16 | int32(buf[off+2])<<8 | int32(buf[off+3])
	}
	t := DecodedTriangle{
		AY: get(16),
		AX: get(20),
	}
	// Without rotation, B/C are not recoverable from the simplified
	// layout. Callers that need the full vertex set should use the
	// pending Lucene-compatible decoder (backlog #2697).
	bits := get(24)
	t.AB = bits&1 != 0
	t.BC = bits&2 != 0
	t.CA = bits&4 != 0
	t.Kind = DecodedTriangleTypeTriangle
	return t, nil
}
