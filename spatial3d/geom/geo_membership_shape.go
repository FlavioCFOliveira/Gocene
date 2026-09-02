// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

// GeoMembershipShape has capabilities of both geohashing and membership determination.
//
// Port of org.apache.lucene.spatial3d.geom.GeoMembershipShape.
type GeoMembershipShape interface {
	GeoShape
	GeoOutsideDistance
}
