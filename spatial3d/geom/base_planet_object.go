// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"io"
)

// BasePlanetObject is a base implementation for planet-based shapes.
// It provides the common planet model embedded in all derived objects.
//
// Port of org.apache.lucene.spatial3d.geom.BasePlanetObject.
type BasePlanetObject struct {
	planetModel *PlanetModel
}

// NewBasePlanetObject creates a BasePlanetObject given a planet model.
//
// Port of BasePlanetObject(PlanetModel).
func NewBasePlanetObject(pm *PlanetModel) BasePlanetObject {
	return BasePlanetObject{
		planetModel: pm,
	}
}

// GetPlanetModel returns the planet model associated with this object.
//
// Port of BasePlanetObject.getPlanetModel().
func (b BasePlanetObject) GetPlanetModel() *PlanetModel {
	return b.planetModel
}

// Write serialises the object to a stream.
// This base implementation throws an unsupported operation exception.
//
// Port of BasePlanetObject.write(OutputStream).
func (b BasePlanetObject) Write(w io.Writer) error {
	return fmt.Errorf("unsupported operation: BasePlanetObject.Write")
}

// HashCode returns the hash code of the planet model.
//
// Port of BasePlanetObject.hashCode().
func (b BasePlanetObject) HashCode() int {
	return b.planetModel.HashCode()
}

// Equals reports whether two BasePlanetObjects have the same planet model.
//
// Port of BasePlanetObject.equals(Object).
func (b BasePlanetObject) Equals(other interface{}) bool {
	if other == nil {
		return false
	}
	o, ok := other.(BasePlanetObject)
	if !ok {
		return false
	}
	return b.planetModel.Equals(o.planetModel)
}
