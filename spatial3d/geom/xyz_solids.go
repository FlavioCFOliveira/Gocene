// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// ---------------------------------------------------------------------------
// BaseXYZSolid — abstract base for all XYZ-bounded solid types.
//
// Port of org.apache.lucene.spatial3d.geom.BaseXYZSolid.
// ---------------------------------------------------------------------------

// xUnitVector is the unit vector along X.
var xUnitVector = &Vector{X: 1, Y: 0, Z: 0}

// yUnitVector is the unit vector along Y.
var yUnitVector = &Vector{X: 0, Y: 1, Z: 0}

// zUnitVector is the unit vector along Z.
var zUnitVector = &Vector{X: 0, Y: 0, Z: 1}

// xVerticalPlane is the vertical plane normal to the X axis through the origin.
var xVerticalPlane = NewPlane(0, 1, 0, 0)

// yVerticalPlane is the vertical plane normal to the Y axis through the origin.
var yVerticalPlane = NewPlane(1, 0, 0, 0)

// Constants for edge-point membership classification.
const (
	solidAllInside    = 0
	solidSomeInside   = 1
	solidNoneInside   = 2
	solidNoEdgePoints = 3
)

// BaseXYZSolid is the base for 3D rectangle shapes bounded by X, Y, Z planes.
//
// Port of org.apache.lucene.spatial3d.geom.BaseXYZSolid.
type BaseXYZSolid struct {
	BasePlanetObject
}

// glueTogether concatenates multiple GeoPoint slices into one.
func glueTogether(arrays ...[]*GeoPoint) []*GeoPoint {
	n := 0
	for _, a := range arrays {
		n += len(a)
	}
	out := make([]*GeoPoint, 0, n)
	for _, a := range arrays {
		out = append(out, a...)
	}
	return out
}

// IsWithin is deferred to concrete types — returns false.
func (b *BaseXYZSolid) IsWithin(_, _, _ float64) bool { return false }

// GetEdgePoints returns nil — deferred to #2693.
func (b *BaseXYZSolid) GetEdgePoints() []*GeoPoint { return nil }

// GetRelationship returns RelDisjoint — deferred to #2693.
func (b *BaseXYZSolid) GetRelationship(_ GeoShape) int { return RelDisjoint }

// ---------------------------------------------------------------------------
// StandardXYZSolid
//
// Port of org.apache.lucene.spatial3d.geom.StandardXYZSolid.
// ---------------------------------------------------------------------------

// StandardXYZSolid is a 3D solid bounded by six XYZ planes.
//
// Port of org.apache.lucene.spatial3d.geom.StandardXYZSolid.
type StandardXYZSolid struct {
	BaseXYZSolid
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64

	// Bounding planes with membership orientation (inside-facing normal).
	minXPlane, maxXPlane *SidedPlane
	minYPlane, maxYPlane *SidedPlane
	minZPlane, maxZPlane *SidedPlane

	// Whether each plane intersects the globe (used by getRelationship).
	minXPlaneIntersects bool
	maxXPlaneIntersects bool
	minYPlaneIntersects bool
	maxYPlaneIntersects bool
	minZPlaneIntersects bool
	maxZPlaneIntersects bool

	// Notable planet-surface points on each bounding plane edge pair.
	notableMinXPoints []*GeoPoint
	notableMaxXPoints []*GeoPoint
	notableMinYPoints []*GeoPoint
	notableMaxYPoints []*GeoPoint
	notableMinZPoints []*GeoPoint
	notableMaxZPoints []*GeoPoint

	// Representative points on the solid surface used for isAreaInsideShape.
	edgePoints []*GeoPoint
}

// NewStandardXYZSolid constructs a StandardXYZSolid.
func NewStandardXYZSolid(pm *PlanetModel, minX, maxX, minY, maxY, minZ, maxZ float64) (*StandardXYZSolid, error) {
	if maxX-minX < MinimumResolution {
		return nil, errorf("X values in wrong order or identical")
	}
	if maxY-minY < MinimumResolution {
		return nil, errorf("Y values in wrong order or identical")
	}
	if maxZ-minZ < MinimumResolution {
		return nil, errorf("Z values in wrong order or identical")
	}

	s := &StandardXYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		minX:         minX, maxX: maxX,
		minY: minY, maxY: maxY,
		minZ: minZ, maxZ: maxZ,
	}

	// Build the six bounding SidedPlanes, each pointing inward.
	// The inward reference point is on the OPPOSITE side: e.g. minX plane has
	// reference point at maxX so that points inside the solid satisfy
	// plane.isWithin(point) == true (the normal points in the +X direction for
	// the minX plane, and the reference maxX side is positive-valued).
	s.minXPlane = NewSidedPlaneFromPointAndUnit(maxX, 0, 0, xUnitVector, -minX)
	s.maxXPlane = NewSidedPlaneFromPointAndUnit(minX, 0, 0, xUnitVector, -maxX)
	s.minYPlane = NewSidedPlaneFromPointAndUnit(0, maxY, 0, yUnitVector, -minY)
	s.maxYPlane = NewSidedPlaneFromPointAndUnit(0, minY, 0, yUnitVector, -maxY)
	s.minZPlane = NewSidedPlaneFromPointAndUnit(0, 0, maxZ, zUnitVector, -minZ)
	s.maxZPlane = NewSidedPlaneFromPointAndUnit(0, 0, minZ, zUnitVector, -maxZ)

	// spPlane is a helper to extract the *Plane from a *SidedPlane for callers
	// that need *Plane rather than a Membership.
	spPlane := func(sp *SidedPlane) *Plane { return &sp.Plane }

	// Compute notable intersection points for each plane with adjacent planes.
	// FindIntersections(pm, q *Plane, bounds ...Membership) — the first arg after pm
	// is the intersecting plane, the rest are Membership filters. *SidedPlane
	// implements Membership so it can be passed directly as a bound.
	minXminY := s.minXPlane.FindIntersections(pm, spPlane(s.minYPlane), s.maxXPlane, s.maxYPlane, s.minZPlane, s.maxZPlane)
	minXmaxY := s.minXPlane.FindIntersections(pm, spPlane(s.maxYPlane), s.maxXPlane, s.minYPlane, s.minZPlane, s.maxZPlane)
	minXminZ := s.minXPlane.FindIntersections(pm, spPlane(s.minZPlane), s.maxXPlane, s.maxZPlane, s.minYPlane, s.maxYPlane)
	minXmaxZ := s.minXPlane.FindIntersections(pm, spPlane(s.maxZPlane), s.maxXPlane, s.minZPlane, s.minYPlane, s.maxYPlane)

	maxXminY := s.maxXPlane.FindIntersections(pm, spPlane(s.minYPlane), s.minXPlane, s.maxYPlane, s.minZPlane, s.maxZPlane)
	maxXmaxY := s.maxXPlane.FindIntersections(pm, spPlane(s.maxYPlane), s.minXPlane, s.minYPlane, s.minZPlane, s.maxZPlane)
	maxXminZ := s.maxXPlane.FindIntersections(pm, spPlane(s.minZPlane), s.minXPlane, s.maxZPlane, s.minYPlane, s.maxYPlane)
	maxXmaxZ := s.maxXPlane.FindIntersections(pm, spPlane(s.maxZPlane), s.minXPlane, s.minZPlane, s.minYPlane, s.maxYPlane)

	minYminZ := s.minYPlane.FindIntersections(pm, spPlane(s.minZPlane), s.maxYPlane, s.maxZPlane, s.minXPlane, s.maxXPlane)
	minYmaxZ := s.minYPlane.FindIntersections(pm, spPlane(s.maxZPlane), s.maxYPlane, s.minZPlane, s.minXPlane, s.maxXPlane)
	maxYminZ := s.maxYPlane.FindIntersections(pm, spPlane(s.minZPlane), s.minYPlane, s.maxZPlane, s.minXPlane, s.maxXPlane)
	maxYmaxZ := s.maxYPlane.FindIntersections(pm, spPlane(s.maxZPlane), s.minYPlane, s.minZPlane, s.minXPlane, s.maxXPlane)

	s.notableMinXPoints = glueTogether(minXminY, minXmaxY, minXminZ, minXmaxZ)
	s.notableMaxXPoints = glueTogether(maxXminY, maxXmaxY, maxXminZ, maxXmaxZ)
	s.notableMinYPoints = glueTogether(minXminY, maxXminY, minYminZ, minYmaxZ)
	s.notableMaxYPoints = glueTogether(minXmaxY, maxXmaxY, maxYminZ, maxYmaxZ)
	s.notableMinZPoints = glueTogether(minXminZ, maxXminZ, minYminZ, maxYminZ)
	s.notableMaxZPoints = glueTogether(minXmaxZ, maxXmaxZ, minYmaxZ, maxYmaxZ)

	// Record whether each plane intersects the globe.
	s.minXPlaneIntersects = len(s.notableMinXPoints) > 0 || s.minXPlane.GetSampleIntersectionPoint(pm, xVerticalPlane) != nil
	s.maxXPlaneIntersects = len(s.notableMaxXPoints) > 0 || s.maxXPlane.GetSampleIntersectionPoint(pm, xVerticalPlane) != nil
	s.minYPlaneIntersects = len(s.notableMinYPoints) > 0 || s.minYPlane.GetSampleIntersectionPoint(pm, yVerticalPlane) != nil
	s.maxYPlaneIntersects = len(s.notableMaxYPoints) > 0 || s.maxYPlane.GetSampleIntersectionPoint(pm, yVerticalPlane) != nil
	s.minZPlaneIntersects = len(s.notableMinZPoints) > 0 || s.minZPlane.GetSampleIntersectionPoint(pm, zVerticalPlane) != nil
	s.maxZPlaneIntersects = len(s.notableMaxZPoints) > 0 || s.maxZPlane.GetSampleIntersectionPoint(pm, zVerticalPlane) != nil

	// Compute edgePoints: at least one point per "manifestation" of the shape.
	// We use the plane-pair intersection points as the primary source, supplemented
	// by single-plane/world sample points when all four corners of a face are outside.
	//
	// The 8 corner points tell us which corners are outside the planet.
	corner := func(x, y, z float64) bool { return pm.PointOutside(x, y, z) }
	c000 := corner(minX, minY, minZ)
	c001 := corner(minX, minY, maxZ)
	c010 := corner(minX, maxY, minZ)
	c011 := corner(minX, maxY, maxZ)
	c100 := corner(maxX, minY, minZ)
	c101 := corner(maxX, minY, maxZ)
	c110 := corner(maxX, maxY, minZ)
	c111 := corner(maxX, maxY, maxZ)

	var ep []*GeoPoint

	// Face minX (corners c000,c001,c010,c011): add sample point if all outside.
	if c000 && c001 && c010 && c011 {
		if pt := s.minXPlane.GetSampleIntersectionPoint(pm, xVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}
	// Face maxX (corners c100,c101,c110,c111):
	if c100 && c101 && c110 && c111 {
		if pt := s.maxXPlane.GetSampleIntersectionPoint(pm, xVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}
	// Face minY (corners c000,c001,c100,c101):
	if c000 && c001 && c100 && c101 {
		if pt := s.minYPlane.GetSampleIntersectionPoint(pm, yVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}
	// Face maxY (corners c010,c011,c110,c111):
	if c010 && c011 && c110 && c111 {
		if pt := s.maxYPlane.GetSampleIntersectionPoint(pm, yVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}
	// Face minZ (corners c000,c010,c100,c110):
	if c000 && c010 && c100 && c110 {
		if pt := s.minZPlane.GetSampleIntersectionPoint(pm, zVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}
	// Face maxZ (corners c001,c011,c101,c111):
	if c001 && c011 && c101 && c111 {
		if pt := s.maxZPlane.GetSampleIntersectionPoint(pm, zVerticalPlane); pt != nil {
			ep = append(ep, pt)
		}
	}

	// Add plane-pair intersection points that are inside the solid.
	for _, pts := range [][]*GeoPoint{
		minXminY, minXmaxY, minXminZ, minXmaxZ,
		maxXminY, maxXmaxY, maxXminZ, maxXmaxZ,
		minYminZ, minYmaxZ, maxYminZ, maxYmaxZ,
	} {
		for _, pt := range pts {
			if s.IsWithin(pt.X, pt.Y, pt.Z) {
				ep = append(ep, pt)
				break // one per pair is sufficient
			}
		}
	}

	s.edgePoints = ep
	return s, nil
}

// IsWithin reports whether (x,y,z) is inside all six bounding planes.
func (s *StandardXYZSolid) IsWithin(x, y, z float64) bool {
	return x >= s.minX && x <= s.maxX &&
		y >= s.minY && y <= s.maxY &&
		z >= s.minZ && z <= s.maxZ
}

// GetEdgePoints returns the precomputed edge points of this solid.
//
// Port of StandardXYZSolid.getEdgePoints.
func (s *StandardXYZSolid) GetEdgePoints() []*GeoPoint {
	return s.edgePoints
}

// GetRelationship computes the spatial relationship between this XYZ solid and
// a GeoShape.
//
// Returns one of: RelContains (shape contains solid), RelWithin (solid contains
// shape), RelOverlaps (partial intersection), RelDisjoint.
//
// Port of org.apache.lucene.spatial3d.geom.StandardXYZSolid.getRelationship
// (Lucene 10.4.0 lines ~507-620). The logic follows the reference exactly.
func (s *StandardXYZSolid) GetRelationship(path GeoShape) int {
	insideRectangle := isShapeInsideArea(path, s)
	if insideRectangle == solidSomeInside {
		return RelOverlaps
	}

	insideShape := isAreaInsideShape(path, s.edgePoints)
	if insideShape == solidSomeInside {
		return RelOverlaps
	}

	if insideRectangle == solidAllInside && insideShape == solidAllInside {
		return RelOverlaps
	}

	// Check whether shape edges cross any of the six bounding planes.
	// path.Intersects takes (plane *Plane, notablePoints []*GeoPoint, bounds ...Membership).
	// &sp.Plane extracts the *Plane pointer from a *SidedPlane.
	if (s.minXPlaneIntersects && path.Intersects(&s.minXPlane.Plane, s.notableMinXPoints,
		s.maxXPlane, s.minYPlane, s.maxYPlane, s.minZPlane, s.maxZPlane)) ||
		(s.maxXPlaneIntersects && path.Intersects(&s.maxXPlane.Plane, s.notableMaxXPoints,
			s.minXPlane, s.minYPlane, s.maxYPlane, s.minZPlane, s.maxZPlane)) ||
		(s.minYPlaneIntersects && path.Intersects(&s.minYPlane.Plane, s.notableMinYPoints,
			s.maxYPlane, s.minXPlane, s.maxXPlane, s.minZPlane, s.maxZPlane)) ||
		(s.maxYPlaneIntersects && path.Intersects(&s.maxYPlane.Plane, s.notableMaxYPoints,
			s.minYPlane, s.minXPlane, s.maxXPlane, s.minZPlane, s.maxZPlane)) ||
		(s.minZPlaneIntersects && path.Intersects(&s.minZPlane.Plane, s.notableMinZPoints,
			s.maxZPlane, s.minXPlane, s.maxXPlane, s.minYPlane, s.maxYPlane)) ||
		(s.maxZPlaneIntersects && path.Intersects(&s.maxZPlane.Plane, s.notableMaxZPoints,
			s.minZPlane, s.minXPlane, s.maxXPlane, s.minYPlane, s.maxYPlane)) {
		return RelOverlaps
	}

	if insideRectangle == solidAllInside {
		return RelWithin
	}

	if insideShape == solidAllInside {
		return RelContains
	}
	return RelDisjoint
}

// isShapeInsideArea checks how many of path's edge points are inside the solid.
// Returns solidAllInside, solidSomeInside, solidNoneInside, or solidNoEdgePoints.
func isShapeInsideArea(path GeoShape, solid interface {
	IsWithin(float64, float64, float64) bool
}) int {
	pathPoints := path.GetEdgePoints()
	if len(pathPoints) == 0 {
		return solidNoEdgePoints
	}
	foundOutside, foundInside := false, false
	for _, p := range pathPoints {
		if solid.IsWithin(p.X, p.Y, p.Z) {
			foundInside = true
		} else {
			foundOutside = true
		}
		if foundInside && foundOutside {
			return solidSomeInside
		}
	}
	if foundInside {
		return solidAllInside
	}
	return solidNoneInside
}

// isAreaInsideShape checks how many of the solid's edge points are inside path.
// path must implement Membership (IsWithin); if not, returns solidNoEdgePoints.
func isAreaInsideShape(path GeoShape, edgePoints []*GeoPoint) int {
	if len(edgePoints) == 0 {
		return solidNoEdgePoints
	}
	withiner, ok := path.(Membership)
	if !ok {
		return solidNoEdgePoints
	}
	foundOutside, foundInside := false, false
	for _, p := range edgePoints {
		if withiner.IsWithin(p.X, p.Y, p.Z) {
			foundInside = true
		} else {
			foundOutside = true
		}
		if foundInside && foundOutside {
			return solidSomeInside
		}
	}
	if foundInside {
		return solidAllInside
	}
	return solidNoneInside
}

// zVerticalPlane is the vertical plane normal to the Z axis through the origin.
var zVerticalPlane = NewPlane(1, 0, 0, 0)

// String returns a debug representation.
func (s *StandardXYZSolid) String() string {
	return "StandardXYZSolid{minX=" + fmtFloat(s.minX) + ",maxX=" + fmtFloat(s.maxX) +
		",minY=" + fmtFloat(s.minY) + ",maxY=" + fmtFloat(s.maxY) +
		",minZ=" + fmtFloat(s.minZ) + ",maxZ=" + fmtFloat(s.maxZ) + "}"
}

// ---------------------------------------------------------------------------
// Degenerate XYZ solids — one or more dimensions collapsed to a single value.
// All full implementations deferred to #2693.
// ---------------------------------------------------------------------------

// DXDYDZSolid is a point solid (all three dimensions degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.dXdYdZSolid.
type DXDYDZSolid struct {
	BaseXYZSolid
	x, y, z float64
}

// NewDXDYDZSolid constructs a point solid.
func NewDXDYDZSolid(pm *PlanetModel, x, y, z float64) *DXDYDZSolid {
	return &DXDYDZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		x:            x, y: y, z: z,
	}
}

// DXDYZSolid is a line solid (X and Y degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.dXdYZSolid.
type DXDYZSolid struct {
	BaseXYZSolid
	x, y, minZ, maxZ float64
}

// NewDXDYZSolid constructs a DXDYZSolid.
func NewDXDYZSolid(pm *PlanetModel, x, y, minZ, maxZ float64) *DXDYZSolid {
	return &DXDYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		x:            x, y: y, minZ: minZ, maxZ: maxZ,
	}
}

// DXYDZSolid is a line solid (X and Z degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.dXYdZSolid.
type DXYDZSolid struct {
	BaseXYZSolid
	x, minY, maxY, z float64
}

// NewDXYDZSolid constructs a DXYDZSolid.
func NewDXYDZSolid(pm *PlanetModel, x, minY, maxY, z float64) *DXYDZSolid {
	return &DXYDZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		x:            x, minY: minY, maxY: maxY, z: z,
	}
}

// DXYZSolid is a planar solid (X degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.
type DXYZSolid struct {
	BaseXYZSolid
	x, minY, maxY, minZ, maxZ float64
}

// NewDXYZSolid constructs a DXYZSolid.
func NewDXYZSolid(pm *PlanetModel, x, minY, maxY, minZ, maxZ float64) *DXYZSolid {
	return &DXYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		x:            x, minY: minY, maxY: maxY, minZ: minZ, maxZ: maxZ,
	}
}

// XDYDZSolid is a line solid (Y and Z degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.XdYdZSolid.
type XDYDZSolid struct {
	BaseXYZSolid
	minX, maxX, y, z float64
}

// NewXDYDZSolid constructs a XDYDZSolid.
func NewXDYDZSolid(pm *PlanetModel, minX, maxX, y, z float64) *XDYDZSolid {
	return &XDYDZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		minX:         minX, maxX: maxX, y: y, z: z,
	}
}

// XDYZSolid is a planar solid (Y degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.XdYZSolid.
type XDYZSolid struct {
	BaseXYZSolid
	minX, maxX, y, minZ, maxZ float64
}

// NewXDYZSolid constructs a XDYZSolid.
func NewXDYZSolid(pm *PlanetModel, minX, maxX, y, minZ, maxZ float64) *XDYZSolid {
	return &XDYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		minX:         minX, maxX: maxX, y: y, minZ: minZ, maxZ: maxZ,
	}
}

// XYDZSolid is a planar solid (Z degenerate).
//
// Port of org.apache.lucene.spatial3d.geom.XYdZSolid.
type XYDZSolid struct {
	BaseXYZSolid
	minX, maxX, minY, maxY, z float64
}

// NewXYDZSolid constructs a XYDZSolid.
func NewXYDZSolid(pm *PlanetModel, minX, maxX, minY, maxY, z float64) *XYDZSolid {
	return &XYDZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		minX:         minX, maxX: maxX, minY: minY, maxY: maxY, z: z,
	}
}

// ---------------------------------------------------------------------------
// XYZSolidFactory
//
// Port of org.apache.lucene.spatial3d.geom.XYZSolidFactory.
// ---------------------------------------------------------------------------

// MakeXYZSolid constructs the appropriate XYZSolid given the six bounds.
//
// Port of org.apache.lucene.spatial3d.geom.XYZSolidFactory.makeXYZSolid.
func MakeXYZSolid(pm *PlanetModel, minX, maxX, minY, maxY, minZ, maxZ float64) XYZSolid {
	dX := math.Abs(maxX-minX) < MinimumResolution
	dY := math.Abs(maxY-minY) < MinimumResolution
	dZ := math.Abs(maxZ-minZ) < MinimumResolution
	midX := (minX + maxX) * 0.5
	midY := (minY + maxY) * 0.5
	midZ := (minZ + maxZ) * 0.5
	switch {
	case dX && dY && dZ:
		return NewDXDYDZSolid(pm, midX, midY, minZ)
	case dX && dY:
		return NewDXDYZSolid(pm, midX, midY, minZ, maxZ)
	case dX && dZ:
		return NewDXYDZSolid(pm, midX, minY, maxY, midZ)
	case dX:
		return NewDXYZSolid(pm, midX, minY, maxY, minZ, maxZ)
	case dY && dZ:
		return NewXDYDZSolid(pm, minX, maxX, midY, midZ)
	case dY:
		return NewXDYZSolid(pm, minX, maxX, midY, minZ, maxZ)
	case dZ:
		return NewXYDZSolid(pm, minX, maxX, minY, maxY, midZ)
	default:
		s, _ := NewStandardXYZSolid(pm, minX, maxX, minY, maxY, minZ, maxZ)
		return s
	}
}

// MakeXYZSolidFromBounds constructs an XYZSolid from an XYZBounds accumulator.
//
// Port of org.apache.lucene.spatial3d.geom.XYZSolidFactory.makeXYZSolid(PlanetModel,XYZBounds).
func MakeXYZSolidFromBounds(pm *PlanetModel, bounds *XYZBounds) XYZSolid {
	return MakeXYZSolid(pm,
		bounds.MinimumX, bounds.MaximumX,
		bounds.MinimumY, bounds.MaximumY,
		bounds.MinimumZ, bounds.MaximumZ)
}

// errorf is a minimal error helper to avoid importing fmt in hot-path code.
func errorf(msg string) error {
	return &solidError{msg: msg}
}

type solidError struct{ msg string }

func (e *solidError) Error() string { return e.msg }
