// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// ArcDistance computes great-circle arc distances.
//
// Port of org.apache.lucene.spatial3d.geom.ArcDistance.
type ArcDistance struct{}

// ArcDistanceInstance is the singleton.
var ArcDistanceInstance = &ArcDistance{}

func (d *ArcDistance) ToAggregationForm(distance float64) float64 { return distance }
func (d *ArcDistance) FromAggregationForm(agg float64) float64    { return agg }
func (d *ArcDistance) ToSlice(distance float64) float64           { return math.Sin(distance) }
func (d *ArcDistance) GetMagnitude(agg float64) float64           { return agg }
func (d *ArcDistance) IsLessThan(agg1, agg2 float64) bool         { return agg1 < agg2 }

var _ DistanceStyle = (*ArcDistance)(nil)

// LinearDistance computes Euclidean linear distances.
//
// Port of org.apache.lucene.spatial3d.geom.LinearDistance.
type LinearDistance struct{}

// LinearDistanceInstance is the singleton.
var LinearDistanceInstance = &LinearDistance{}

func (d *LinearDistance) ToAggregationForm(distance float64) float64 { return distance * distance }
func (d *LinearDistance) FromAggregationForm(agg float64) float64    { return math.Sqrt(agg) }
func (d *LinearDistance) ToSlice(distance float64) float64           { return distance * distance }
func (d *LinearDistance) GetMagnitude(agg float64) float64           { return math.Sqrt(agg) }
func (d *LinearDistance) IsLessThan(agg1, agg2 float64) bool         { return agg1 < agg2 }

var _ DistanceStyle = (*LinearDistance)(nil)

// LinearSquaredDistance computes squared linear distances (no sqrt).
//
// Port of org.apache.lucene.spatial3d.geom.LinearSquaredDistance.
type LinearSquaredDistance struct{}

// LinearSquaredDistanceInstance is the singleton.
var LinearSquaredDistanceInstance = &LinearSquaredDistance{}

func (d *LinearSquaredDistance) ToAggregationForm(distance float64) float64 { return distance }
func (d *LinearSquaredDistance) FromAggregationForm(agg float64) float64    { return agg }
func (d *LinearSquaredDistance) ToSlice(distance float64) float64           { return distance }
func (d *LinearSquaredDistance) GetMagnitude(agg float64) float64           { return agg }
func (d *LinearSquaredDistance) IsLessThan(agg1, agg2 float64) bool         { return agg1 < agg2 }

var _ DistanceStyle = (*LinearSquaredDistance)(nil)

// NormalDistance computes normal (chord-length) distances.
//
// Port of org.apache.lucene.spatial3d.geom.NormalDistance.
type NormalDistance struct{}

// NormalDistanceInstance is the singleton.
var NormalDistanceInstance = &NormalDistance{}

func (d *NormalDistance) ToAggregationForm(distance float64) float64 { return distance * distance }
func (d *NormalDistance) FromAggregationForm(agg float64) float64    { return math.Sqrt(agg) }
func (d *NormalDistance) ToSlice(distance float64) float64           { return distance * distance }
func (d *NormalDistance) GetMagnitude(agg float64) float64           { return math.Sqrt(agg) }
func (d *NormalDistance) IsLessThan(agg1, agg2 float64) bool         { return agg1 < agg2 }

var _ DistanceStyle = (*NormalDistance)(nil)

// NormalSquaredDistance computes squared normal distances.
//
// Port of org.apache.lucene.spatial3d.geom.NormalSquaredDistance.
type NormalSquaredDistance struct{}

// NormalSquaredDistanceInstance is the singleton.
var NormalSquaredDistanceInstance = &NormalSquaredDistance{}

func (d *NormalSquaredDistance) ToAggregationForm(distance float64) float64 { return distance }
func (d *NormalSquaredDistance) FromAggregationForm(agg float64) float64    { return agg }
func (d *NormalSquaredDistance) ToSlice(distance float64) float64           { return distance }
func (d *NormalSquaredDistance) GetMagnitude(agg float64) float64           { return agg }
func (d *NormalSquaredDistance) IsLessThan(agg1, agg2 float64) bool         { return agg1 < agg2 }

var _ DistanceStyle = (*NormalSquaredDistance)(nil)
