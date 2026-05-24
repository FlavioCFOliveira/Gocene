// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

// StandardObjects maps type codes to serializable shape types for the Lucene binary format.
//
// Port of org.apache.lucene.spatial3d.geom.StandardObjects.
//
// Full serialization support deferred to #2693.

// TypeCode is the integer code used in the Lucene serialization format.
type TypeCode = int

// Standard type codes matching the Lucene binary format.
const (
	CodeGeoPoint                    TypeCode = 0
	CodeGeoRectangle                TypeCode = 1
	CodeGeoStandardCircle           TypeCode = 2
	CodeGeoStandardPath             TypeCode = 3
	CodeGeoConvexPolygon            TypeCode = 4
	CodeGeoConcavePolygon           TypeCode = 5
	CodeGeoComplexPolygon           TypeCode = 6
	CodeGeoCompositePolygon         TypeCode = 7
	CodeGeoCompositeMembershipShape TypeCode = 8
	CodeGeoCompositeAreaShape       TypeCode = 9
	CodeGeoDegeneratePoint          TypeCode = 10
	CodeGeoDegenerateHorizontalLine TypeCode = 11
	CodeGeoDegenerateLatitudeZone   TypeCode = 12
	CodeGeoDegenerateLongitudeSlice TypeCode = 13
	CodeGeoDegenerateVerticalLine   TypeCode = 14
	CodeGeoLatitudeZone             TypeCode = 15
	CodeGeoLongitudeSlice           TypeCode = 16
	CodeGeoNorthLatitudeZone        TypeCode = 17
	CodeGeoNorthRectangle           TypeCode = 18
	CodeGeoSouthLatitudeZone        TypeCode = 19
	CodeGeoSouthRectangle           TypeCode = 20
	CodeGeoWideDegenerateHLine      TypeCode = 21
	CodeGeoWideLongitudeSlice       TypeCode = 22
	CodeGeoWideNorthRectangle       TypeCode = 23
	CodeGeoWideRectangle            TypeCode = 24
	CodeGeoWideSouthRectangle       TypeCode = 25
	CodeGeoWorld                    TypeCode = 26
	CodeDXDYDZSolid                 TypeCode = 27
	CodeDXDYZSolid                  TypeCode = 28
	CodeDXYDZSolid                  TypeCode = 29
	CodeDXYZSolid                   TypeCode = 30
	CodeXDYDZSolid                  TypeCode = 31
	CodeXDYZSolid                   TypeCode = 32
	CodeXYDZSolid                   TypeCode = 33
	CodeStandardXYZSolid            TypeCode = 34
	CodePlanetModel                 TypeCode = 35
	CodeGeoDegeneratePath           TypeCode = 36
	CodeGeoExactCircle              TypeCode = 37
	CodeGeoS2Shape                  TypeCode = 38
)
