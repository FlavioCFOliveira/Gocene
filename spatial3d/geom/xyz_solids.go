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
	return &StandardXYZSolid{
		BaseXYZSolid: BaseXYZSolid{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}},
		minX:         minX, maxX: maxX,
		minY: minY, maxY: maxY,
		minZ: minZ, maxZ: maxZ,
	}, nil
}

// IsWithin reports whether (x,y,z) is inside all six bounding planes.
func (s *StandardXYZSolid) IsWithin(x, y, z float64) bool {
	return x >= s.minX && x <= s.maxX &&
		y >= s.minY && y <= s.maxY &&
		z >= s.minZ && z <= s.maxZ
}

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
