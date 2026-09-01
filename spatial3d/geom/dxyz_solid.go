// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"encoding/binary"
	"fmt"
	"io"
)

// DXYZSolid is a planar solid (X degenerate), bounded on six sides by X,Y,Z limits.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.
type DXYZSolid struct {
	BaseXYZSolid
	x, minY, maxY, minZ, maxZ float64

	// Bounding planes.
	xPlane *Plane
	minYPlane, maxYPlane *SidedPlane
	minZPlane, maxZPlane *SidedPlane

	// Edge points of the shape.
	edgePoints []*GeoPoint

	// Notable points for XPlane.
	notableXPoints []*GeoPoint
}

// NewDXYZSolid constructs a DXYZSolid.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid constructor.
func NewDXYZSolid(pm *PlanetModel, x, minY, maxY, minZ, maxZ float64) (*DXYZSolid, error) {
	// Argument checking.
	if maxY-minY < MinimumResolution {
		return nil, fmt.Errorf("Y values in wrong order or identical")
	}
	if maxZ-minZ < MinimumResolution {
		return nil, fmt.Errorf("Z values in wrong order or identical")
	}

	s := &DXYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{planetModel: pm}},
		x:            x,
		minY:         minY,
		maxY:         maxY,
		minZ:         minZ,
		maxZ:         maxZ,
	}

	worldMinX := pm.GetMinimumXValue()
	worldMaxX := pm.GetMaximumXValue()

	// Construct the planes.
	s.xPlane = NewPlaneFromVectorD(xUnitVector, -x)
	s.minYPlane = NewSidedPlaneFromPointAndUnit(0.0, maxY, 0.0, yUnitVector, -minY)
	s.maxYPlane = NewSidedPlaneFromPointAndUnit(0.0, minY, 0.0, yUnitVector, -maxY)
	s.minZPlane = NewSidedPlaneFromPointAndUnit(0.0, 0.0, maxZ, zUnitVector, -minZ)
	s.maxZPlane = NewSidedPlaneFromPointAndUnit(0.0, 0.0, minZ, zUnitVector, -maxZ)

	// Find intersections for the four combinations of adjacent planes.
	spPlane := func(sp *SidedPlane) *Plane { return &sp.Plane }

	XminY := s.xPlane.FindIntersections(pm, spPlane(s.minYPlane), s.maxYPlane, s.minZPlane, s.maxZPlane)
	XmaxY := s.xPlane.FindIntersections(pm, spPlane(s.maxYPlane), s.minYPlane, s.minZPlane, s.maxZPlane)
	XminZ := s.xPlane.FindIntersections(pm, spPlane(s.minZPlane), s.maxZPlane, s.minYPlane, s.maxYPlane)
	XmaxZ := s.xPlane.FindIntersections(pm, spPlane(s.maxZPlane), s.minZPlane, s.minYPlane, s.maxYPlane)

	s.notableXPoints = glueTogether(XminY, XmaxY, XminZ, XmaxZ)

	// Compute the edge points.
	// If all four corners of the X-face are outside the world, and the X-plane intersects the world,
	// we must add a sample intersection point.
	XminYminZ := pm.PointOutside(x, minY, minZ)
	XminYmaxZ := pm.PointOutside(x, minY, maxZ)
	XmaxYminZ := pm.PointOutside(x, maxY, minZ)
	XmaxYmaxZ := pm.PointOutside(x, maxY, maxZ)

	var xEdges []*GeoPoint
	if x-worldMinX >= -MinimumResolution &&
		x-worldMaxX <= MinimumResolution &&
		minY < 0.0 && maxY > 0.0 &&
		minZ < 0.0 && maxZ > 0.0 &&
		XminYminZ && XminYmaxZ && XmaxYminZ && XmaxYmaxZ {
		if intPoint := s.xPlane.GetSampleIntersectionPoint(pm, xVerticalPlane); intPoint != nil {
			xEdges = []*GeoPoint{intPoint}
		} else {
			xEdges = []*GeoPoint{}
		}
	} else {
		xEdges = []*GeoPoint{}
	}

	s.edgePoints = glueTogether(XminY, XmaxY, XminZ, XmaxZ, xEdges)

	return s, nil
}

// Write serializes the DXYZSolid to the output stream.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.write.
func (s *DXYZSolid) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, s.x); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.minY); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.maxY); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.minZ); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, s.maxZ)
}

// IsWithin reports whether (x,y,z) is inside this planar solid.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.isWithin.
func (s *DXYZSolid) IsWithin(x, y, z float64) bool {
	return s.xPlane.EvaluateIsZeroXYZ(x, y, z) &&
		s.minYPlane.IsWithin(x, y, z) &&
		s.maxYPlane.IsWithin(x, y, z) &&
		s.minZPlane.IsWithin(x, y, z) &&
		s.maxZPlane.IsWithin(x, y, z)
}

// GetEdgePoints returns the edge points of this solid.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.getEdgePoints.
func (s *DXYZSolid) GetEdgePoints() []*GeoPoint {
	return s.edgePoints
}

// GetRelationship computes the spatial relationship between this solid and the provided shape.
//
// Port of org.apache.lucene.spatial3d.geom.dXYZSolid.getRelationship.
func (s *DXYZSolid) GetRelationship(path GeoShape) int {
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

	if path.Intersects(s.xPlane, s.notableXPoints, s.minYPlane, s.maxYPlane, s.minZPlane, s.maxZPlane) {
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
