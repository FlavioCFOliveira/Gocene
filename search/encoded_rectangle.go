// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/geo"

// EncodedRectangle holds spatial logic for a bounding box expressed
// in the encoded longitude/latitude (or x/y) coordinate space. It
// mirrors the public nested class SpatialQuery.EncodedRectangle in
// org.apache.lucene.document.SpatialQuery (Lucene 10.4.0).
//
// Unlike geo.Rectangle (which lives in WGS-84 degrees), this type
// operates on the int32 values produced by GeoEncodingUtils.encode*
// — i.e. on the same packed values the BKD tree stores. All
// computations here are pure int arithmetic except for the edge
// intersection helpers, which delegate to geo.LineCrossesLineWithBoundary
// (float64-based) and geo.PointInTriangle.
//
// The wrapsCoordinateSystem flag mirrors the Java reference: when
// the bounding box wraps the dateline (or wraps the X axis in the
// XYShape case), the X comparison flips from "outside iff (x < minX
// || x > maxX)" to "outside iff (x > maxX && x < minX)" — i.e. the
// valid X interval is the union of [-inf, maxX] and [minX, +inf]
// rather than a single contiguous range.
//
// All methods are read-only after construction; the type is safe
// for concurrent use.
type EncodedRectangle struct {
	minX                  int32
	maxX                  int32
	minY                  int32
	maxY                  int32
	wrapsCoordinateSystem bool
}

// NewEncodedRectangle builds a rectangle in the encoded coordinate
// space. The four bounds are expected to satisfy minY <= maxY; the
// X bounds may form either a contiguous interval (minX <= maxX,
// wrapsCoordinateSystem=false) or a wrap-around interval
// (wrapsCoordinateSystem=true), where any X >= minX or <= maxX is
// inside.
//
// Mirrors the protected constructor of SpatialQuery.EncodedRectangle.
func NewEncodedRectangle(minX, maxX, minY, maxY int32, wrapsCoordinateSystem bool) *EncodedRectangle {
	return &EncodedRectangle{
		minX:                  minX,
		maxX:                  maxX,
		minY:                  minY,
		maxY:                  maxY,
		wrapsCoordinateSystem: wrapsCoordinateSystem,
	}
}

// MinX returns the X lower bound (inclusive).
func (r *EncodedRectangle) MinX() int32 { return r.minX }

// MaxX returns the X upper bound (inclusive).
func (r *EncodedRectangle) MaxX() int32 { return r.maxX }

// MinY returns the Y lower bound (inclusive).
func (r *EncodedRectangle) MinY() int32 { return r.minY }

// MaxY returns the Y upper bound (inclusive).
func (r *EncodedRectangle) MaxY() int32 { return r.maxY }

// WrapsCoordinateSystem reports whether the rectangle's X interval
// wraps the dateline / X axis edge.
func (r *EncodedRectangle) WrapsCoordinateSystem() bool { return r.wrapsCoordinateSystem }

// Contains reports whether (x, y) lies inside the rectangle.
// Mirrors EncodedRectangle.contains.
func (r *EncodedRectangle) Contains(x, y int32) bool {
	if y < r.minY || y > r.maxY {
		return false
	}
	if r.wrapsCoordinateSystem {
		return !(x > r.maxX && x < r.minX)
	}
	return !(x > r.maxX || x < r.minX)
}

// IntersectsLine reports whether the segment (aX, aY)-(bX, bY)
// intersects the rectangle. The check is short-circuited by the
// cheap endpoint-containment and bounding-box-disjoint paths before
// falling back to the per-edge intersection test.
//
// Mirrors EncodedRectangle.intersectsLine.
func (r *EncodedRectangle) IntersectsLine(aX, aY, bX, bY int32) bool {
	if r.Contains(aX, aY) || r.Contains(bX, bY) {
		return true
	}
	// Bounding box of the segment in Y.
	if maxInt32(aY, bY) < r.minY || minInt32(aY, bY) > r.maxY {
		return false
	}
	if r.wrapsCoordinateSystem {
		if minInt32(aX, bX) > r.maxX && maxInt32(aX, bX) < r.minX {
			return false
		}
	} else {
		if minInt32(aX, bX) > r.maxX || maxInt32(aX, bX) < r.minX {
			return false
		}
	}
	return r.edgeIntersectsQuery(aX, aY, bX, bY)
}

// IntersectsTriangle reports whether the triangle (a, b, c)
// intersects the rectangle. The check is short-circuited by the
// cheap vertex-containment and bounding-box-disjoint paths before
// falling back to the per-edge intersection test (and the
// rectangle-inside-triangle test via geo.PointInTriangle).
//
// Mirrors EncodedRectangle.intersectsTriangle.
func (r *EncodedRectangle) IntersectsTriangle(aX, aY, bX, bY, cX, cY int32) bool {
	if r.Contains(aX, aY) || r.Contains(bX, bY) || r.Contains(cX, cY) {
		return true
	}
	tMinY := minInt32(minInt32(aY, bY), cY)
	tMaxY := maxInt32(maxInt32(aY, bY), cY)
	if tMaxY < r.minY || tMinY > r.maxY {
		return false
	}
	tMinX := minInt32(minInt32(aX, bX), cX)
	tMaxX := maxInt32(maxInt32(aX, bX), cX)
	if r.wrapsCoordinateSystem {
		if tMinX > r.maxX && tMaxX < r.minX {
			return false
		}
	} else {
		if tMinX > r.maxX || tMaxX < r.minX {
			return false
		}
	}
	// The "rectangle inside triangle" check uses the triangle's
	// bounding box and the rectangle's minimum corner as a
	// representative point. geo.PointInTriangle operates in
	// float64; the int32 values cast to float64 are exact (every
	// int32 is representable in float64 without loss).
	if geo.PointInTriangle(
		float64(tMinX), float64(tMaxX), float64(tMinY), float64(tMaxY),
		float64(r.minX), float64(r.minY),
		float64(aX), float64(aY),
		float64(bX), float64(bY),
		float64(cX), float64(cY),
	) {
		return true
	}
	return r.edgeIntersectsQuery(aX, aY, bX, bY) ||
		r.edgeIntersectsQuery(bX, bY, cX, cY) ||
		r.edgeIntersectsQuery(cX, cY, aX, aY)
}

// IntersectsRectangle reports whether the rectangle (minX..maxX,
// minY..maxY) intersects this rectangle.
// Mirrors EncodedRectangle.intersectsRectangle.
func (r *EncodedRectangle) IntersectsRectangle(minX, maxX, minY, maxY int32) bool {
	if r.minY > maxY || r.maxY < minY {
		return false
	}
	if r.minX <= maxX {
		if r.wrapsCoordinateSystem || r.maxX >= minX {
			return true
		}
	}
	return r.wrapsCoordinateSystem
}

// ContainsRectangle reports whether this rectangle fully contains
// the rectangle (minX..maxX, minY..maxY).
// Mirrors EncodedRectangle.containsRectangle.
func (r *EncodedRectangle) ContainsRectangle(minX, maxX, minY, maxY int32) bool {
	return r.minX <= minX && r.maxX >= maxX && r.minY <= minY && r.maxY >= maxY
}

// ContainsLine reports whether the segment (aX, aY)-(bX, bY) lies
// entirely inside the rectangle. When the rectangle wraps the
// coordinate system, the segment is contained when both endpoints
// fall on the same wrap "side" of the axis.
//
// Mirrors EncodedRectangle.containsLine.
func (r *EncodedRectangle) ContainsLine(aX, aY, bX, bY int32) bool {
	if aY < r.minY || bY < r.minY || aY > r.maxY || bY > r.maxY {
		return false
	}
	if r.wrapsCoordinateSystem {
		return (aX >= r.minX && bX >= r.minX) || (aX <= r.maxX && bX <= r.maxX)
	}
	return aX >= r.minX && bX >= r.minX && aX <= r.maxX && bX <= r.maxX
}

// ContainsTriangle reports whether the triangle (a, b, c) lies
// entirely inside the rectangle.
// Mirrors EncodedRectangle.containsTriangle.
func (r *EncodedRectangle) ContainsTriangle(aX, aY, bX, bY, cX, cY int32) bool {
	if aY < r.minY || bY < r.minY || cY < r.minY || aY > r.maxY || bY > r.maxY || cY > r.maxY {
		return false
	}
	if r.wrapsCoordinateSystem {
		return (aX >= r.minX && bX >= r.minX && cX >= r.minX) ||
			(aX <= r.maxX && bX <= r.maxX && cX <= r.maxX)
	}
	return aX >= r.minX && bX >= r.minX && cX >= r.minX &&
		aX <= r.maxX && bX <= r.maxX && cX <= r.maxX
}

// WithinLine returns the Within relation between the rectangle and
// the line segment (aX, aY)-(bX, bY). When ab is true the segment
// is considered "real" (a shape edge); when false it is a virtual
// connector and crossings do not count as NOTWITHIN.
//
// Mirrors EncodedRectangle.withinLine.
func (r *EncodedRectangle) WithinLine(aX, aY int32, ab bool, bX, bY int32) geo.WithinRelation {
	if r.Contains(aX, aY) || r.Contains(bX, bY) {
		return geo.WithinNotWithin
	}
	if ab && edgeIntersectsBox(aX, aY, bX, bY, r.minX, r.maxX, r.minY, r.maxY) {
		return geo.WithinNotWithin
	}
	return geo.WithinDisjoint
}

// WithinTriangle returns the Within relation between the rectangle
// and the triangle (a, b, c) with edge "realness" flags ab, bc, ca.
// Mirrors EncodedRectangle.withinTriangle.
func (r *EncodedRectangle) WithinTriangle(
	aX, aY int32, ab bool,
	bX, bY int32, bc bool,
	cX, cY int32, ca bool,
) geo.WithinRelation {
	if r.Contains(aX, aY) || r.Contains(bX, bY) || r.Contains(cX, cY) {
		return geo.WithinNotWithin
	}
	tMinY := minInt32(minInt32(aY, bY), cY)
	tMaxY := maxInt32(maxInt32(aY, bY), cY)
	if tMaxY < r.minY || tMinY > r.maxY {
		return geo.WithinDisjoint
	}
	tMinX := minInt32(minInt32(aX, bX), cX)
	tMaxX := maxInt32(maxInt32(aX, bX), cX)
	if r.wrapsCoordinateSystem {
		if tMinX > r.maxX && tMaxX < r.minX {
			return geo.WithinDisjoint
		}
	} else {
		if tMinX > r.maxX || tMaxX < r.minX {
			return geo.WithinDisjoint
		}
	}

	relation := geo.WithinDisjoint
	if edgeIntersectsBox(aX, aY, bX, bY, r.minX, r.maxX, r.minY, r.maxY) {
		if ab {
			return geo.WithinNotWithin
		}
		relation = geo.WithinCandidate
	}
	if edgeIntersectsBox(bX, bY, cX, cY, r.minX, r.maxX, r.minY, r.maxY) {
		if bc {
			return geo.WithinNotWithin
		}
		relation = geo.WithinCandidate
	}
	if edgeIntersectsBox(cX, cY, aX, aY, r.minX, r.maxX, r.minY, r.maxY) {
		if ca {
			return geo.WithinNotWithin
		}
		relation = geo.WithinCandidate
	}
	if relation == geo.WithinCandidate ||
		geo.PointInTriangle(
			float64(tMinX), float64(tMaxX), float64(tMinY), float64(tMaxY),
			float64(r.minX), float64(r.minY),
			float64(aX), float64(aY),
			float64(bX), float64(bY),
			float64(cX), float64(cY),
		) {
		return geo.WithinCandidate
	}
	return relation
}

// edgeIntersectsQuery reports whether the segment (aX, aY)-(bX, bY)
// intersects this rectangle's outline. When the rectangle wraps the
// coordinate system the check is split into the two non-wrapping
// halves; this mirrors the Java reference's MIN_LON_ENCODED /
// MAX_LON_ENCODED expansion.
func (r *EncodedRectangle) edgeIntersectsQuery(aX, aY, bX, bY int32) bool {
	if r.wrapsCoordinateSystem {
		return edgeIntersectsBox(aX, aY, bX, bY, geo.MinLonEncoded, r.maxX, r.minY, r.maxY) ||
			edgeIntersectsBox(aX, aY, bX, bY, r.minX, geo.MaxLonEncoded, r.minY, r.maxY)
	}
	return edgeIntersectsBox(aX, aY, bX, bY, r.minX, r.maxX, r.minY, r.maxY)
}

// edgeIntersectsBox reports whether the segment (aX, aY)-(bX, bY)
// intersects the rectangle (minX..maxX, minY..maxY). The four edge
// crossings are computed with geo.LineCrossesLineWithBoundary
// (closed-interior intersection), matching the Java reference's
// GeoUtils.lineCrossesLineWithBoundary call.
func edgeIntersectsBox(aX, aY, bX, bY, minX, maxX, minY, maxY int32) bool {
	if maxInt32(aX, bX) < minX ||
		minInt32(aX, bX) > maxX ||
		minInt32(aY, bY) > maxY ||
		maxInt32(aY, bY) < minY {
		return false
	}
	faX, faY := float64(aX), float64(aY)
	fbX, fbY := float64(bX), float64(bY)
	fMinX, fMaxX := float64(minX), float64(maxX)
	fMinY, fMaxY := float64(minY), float64(maxY)
	return geo.LineCrossesLineWithBoundary(faX, faY, fbX, fbY, fMinX, fMaxY, fMaxX, fMaxY) || // top
		geo.LineCrossesLineWithBoundary(faX, faY, fbX, fbY, fMaxX, fMaxY, fMaxX, fMinY) || // bottom
		geo.LineCrossesLineWithBoundary(faX, faY, fbX, fbY, fMaxX, fMinY, fMinX, fMinY) || // left
		geo.LineCrossesLineWithBoundary(faX, faY, fbX, fbY, fMinX, fMinY, fMinX, fMaxY) // right
}

// minInt32 returns the smaller of a, b. Kept package-local so the
// hot-path arithmetic in EncodedRectangle stays branch-only and
// allocation-free.
func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// maxInt32 returns the larger of a, b.
func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
