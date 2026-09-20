// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
)

// This file ports the distance-sort SortField factories that Apache
// Lucene 10.4.0 places on the doc-values field types:
//
//   - org.apache.lucene.document.LatLonDocValuesField#newDistanceSort
//   - org.apache.lucene.document.XYDocValuesField#newDistanceSort
//
// Both return a SortField whose comparator orders documents by distance
// to a fixed origin, reading the SortedNumericDocValues stream produced
// by the corresponding doc-values field (LatLon: Haversine metres on the
// WGS-84 sphere; XY: Euclidean/Cartesian distance). Documents without a
// value sort last with a distance of +Inf.
//
// # Package placement
//
// The Java factories live in the document package (next to the field
// types). In Gocene the SortField + comparator machinery lives in
// search/, and the concrete distance comparators
// ([LatLonPointDistanceComparator] / [XYPointDistanceComparator]) are
// already here, so the factories live here too — avoiding the
// document → search import cycle (search already imports document).
//
// # Wiring
//
// Java returns a LatLonPointSortField / XYPointSortField, a SortField
// subclass whose getComparator override builds the distance comparator.
// Gocene's SortField is an alias of spi.SortField, so a Go subclass that
// embeds it loses the override the moment the value is held as a
// *SortField — which is how a Sort holds it. The factories therefore
// build a CUSTOM SortField carrying a [FieldComparatorSource] that
// produces exactly the same comparator, which is the one shape that
// survives the erasure. The comparators themselves are
// [LatLonPointDistanceComparator] and [XYPointDistanceComparator], which
// satisfy [FieldComparator] and [LeafFieldComparator] directly, as their
// Java counterparts extend FieldComparator<Double> and implement
// LeafFieldComparator.

// NewLatLonDocValuesDistanceSort builds the SortField that orders
// documents by Haversine distance (in metres) from (latitude, longitude)
// to the point(s) stored in the named LatLonDocValuesField. Documents
// without a value sort last (distance +Inf).
//
// Go port of org.apache.lucene.document.LatLonDocValuesField#newDistanceSort.
func NewLatLonDocValuesDistanceSort(field string, latitude, longitude float64) (*SortField, error) {
	if field == "" {
		return nil, errLatLonDocValuesQueryNilField
	}
	src := &latLonDistanceComparatorSource{
		field:     field,
		latitude:  latitude,
		longitude: longitude,
	}
	sf := NewSortFieldCustom(field, src, false)
	// Missing values sort last (Java sets the missing sentinel to +Inf).
	sf.Missing = spi.MissingValueLast
	sf.MissingValue = math.Inf(1)
	return sf, nil
}

// NewXYDocValuesDistanceSort builds the SortField that orders documents
// by Euclidean (Cartesian) distance from (x, y) to the point(s) stored
// in the named XYDocValuesField. Documents without a value sort last
// (distance +Inf).
//
// Go port of org.apache.lucene.document.XYDocValuesField#newDistanceSort.
func NewXYDocValuesDistanceSort(field string, x, y float32) (*SortField, error) {
	if field == "" {
		return nil, errLatLonDocValuesQueryNilField
	}
	src := &xyDistanceComparatorSource{field: field, x: x, y: y}
	sf := NewSortFieldCustom(field, src, false)
	sf.Missing = spi.MissingValueLast
	sf.MissingValue = math.Inf(1)
	return sf, nil
}

// -----------------------------------------------------------------------------
// LatLon distance comparator source
// -----------------------------------------------------------------------------

// latLonDistanceComparatorSource produces a FieldComparator that orders
// documents by Haversine distance to (latitude, longitude). It mirrors
// the anonymous FieldComparatorSource returned by
// LatLonDocValuesField.newDistanceSort.
type latLonDistanceComparatorSource struct {
	field     string
	latitude  float64
	longitude float64
}

func (s *latLonDistanceComparatorSource) NewComparator(fieldname string, numHits int, pruning Pruning, reversed bool) FieldComparator {
	return NewLatLonPointDistanceComparator(s.field, s.latitude, s.longitude, numHits)
}

var _ FieldComparatorSource = (*latLonDistanceComparatorSource)(nil)

// -----------------------------------------------------------------------------
// XY distance comparator source
// -----------------------------------------------------------------------------

// xyDistanceComparatorSource produces a FieldComparator that orders
// documents by Euclidean distance to (x, y). It mirrors the anonymous
// FieldComparatorSource returned by XYDocValuesField.newDistanceSort.
type xyDistanceComparatorSource struct {
	field string
	x     float32
	y     float32
}

func (s *xyDistanceComparatorSource) NewComparator(fieldname string, numHits int, pruning Pruning, reversed bool) FieldComparator {
	return NewXYPointDistanceComparator(s.field, s.x, s.y, numHits)
}

var _ FieldComparatorSource = (*xyDistanceComparatorSource)(nil)
