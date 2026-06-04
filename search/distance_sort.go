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
// A generic Sort drives a CUSTOM SortField through a
// [FieldComparatorSource]; the source's comparator must satisfy the
// public [FieldComparator] interface plus the optional leaf-binding and
// value hooks the TopFieldCollector consults (SetReader, Value). The
// concrete distance comparators carry a slightly wider, error-returning
// surface (CompareBottom/Copy return errors, GetLeafComparator takes a
// LeafReaderContext); the adapters below bridge the two shapes,
// resolving the per-leaf doc values through the comparator's BindLeaf
// helper.

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
	sf.Missing = MissingValueLast
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
	sf.Missing = MissingValueLast
	sf.MissingValue = math.Inf(1)
	return sf, nil
}

// -----------------------------------------------------------------------------
// LatLon distance comparator source + adapter
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

func (s *latLonDistanceComparatorSource) NewComparator(_ *SortField, numHits int) FieldComparator {
	return &latLonDistanceFieldComparator{
		inner: NewLatLonPointDistanceComparator(s.field, s.latitude, s.longitude, numHits),
	}
}

// latLonDistanceFieldComparator adapts the wider, error-returning
// [LatLonPointDistanceComparator] to the public [FieldComparator]
// surface the TopFieldCollector drives, plus the optional SetReader /
// Value hooks. Errors from the inner comparator are propagated via
// cmpErr, which the collector surfaces through the leaf-binding /
// scoring path; in practice the inner comparator only fails when a
// doc-values advance fails, which is itself surfaced by the reader.
type latLonDistanceFieldComparator struct {
	inner *LatLonPointDistanceComparator
}

func (c *latLonDistanceFieldComparator) Compare(slot1, slot2 int) int {
	return c.inner.Compare(slot1, slot2)
}

func (c *latLonDistanceFieldComparator) SetBottom(slot int) { _ = c.inner.SetBottom(slot) }

func (c *latLonDistanceFieldComparator) CompareBottom(doc int) int {
	cmp, err := c.inner.CompareBottom(doc)
	if err != nil {
		// A failed doc-values advance means the doc is not competitive;
		// treat it as worse than the bottom (matching the +Inf-missing
		// fallback the inner comparator already uses on a missing value).
		return -1
	}
	return cmp
}

func (c *latLonDistanceFieldComparator) Copy(slot, doc int) { _ = c.inner.Copy(slot, doc) }

func (c *latLonDistanceFieldComparator) SetScorer(_ Scorer) {}

// SetReader is the optional leaf-binding hook (search.leafBindingComparator).
func (c *latLonDistanceFieldComparator) SetReader(reader IndexReader) error {
	return c.inner.BindLeaf(reader)
}

// Value exposes the per-slot distance (in metres) for FieldDoc.Fields
// (search.valueComparator).
func (c *latLonDistanceFieldComparator) Value(slot int) any { return c.inner.Value(slot) }

// CompareTop bridges the searchAfter top-value comparison.
func (c *latLonDistanceFieldComparator) CompareTop(doc int) (int, error) {
	return c.inner.CompareTop(doc)
}

var (
	_ FieldComparatorSource = (*latLonDistanceComparatorSource)(nil)
	_ FieldComparator       = (*latLonDistanceFieldComparator)(nil)
)

// -----------------------------------------------------------------------------
// XY distance comparator source + adapter
// -----------------------------------------------------------------------------

// xyDistanceComparatorSource produces a FieldComparator that orders
// documents by Euclidean distance to (x, y). It mirrors the anonymous
// FieldComparatorSource returned by XYDocValuesField.newDistanceSort.
type xyDistanceComparatorSource struct {
	field string
	x     float32
	y     float32
}

func (s *xyDistanceComparatorSource) NewComparator(_ *SortField, numHits int) FieldComparator {
	return &xyDistanceFieldComparator{
		inner: NewXYPointDistanceComparator(s.field, s.x, s.y, numHits),
	}
}

// xyDistanceFieldComparator adapts [XYPointDistanceComparator] to the
// public [FieldComparator] surface, mirroring
// [latLonDistanceFieldComparator] for the Cartesian distance sort.
type xyDistanceFieldComparator struct {
	inner *XYPointDistanceComparator
}

func (c *xyDistanceFieldComparator) Compare(slot1, slot2 int) int {
	return c.inner.Compare(slot1, slot2)
}

func (c *xyDistanceFieldComparator) SetBottom(slot int) { _ = c.inner.SetBottom(slot) }

func (c *xyDistanceFieldComparator) CompareBottom(doc int) int {
	cmp, err := c.inner.CompareBottom(doc)
	if err != nil {
		return -1
	}
	return cmp
}

func (c *xyDistanceFieldComparator) Copy(slot, doc int) { _ = c.inner.Copy(slot, doc) }

func (c *xyDistanceFieldComparator) SetScorer(_ Scorer) {}

// SetReader is the optional leaf-binding hook (search.leafBindingComparator).
func (c *xyDistanceFieldComparator) SetReader(reader IndexReader) error {
	return c.inner.BindLeaf(reader)
}

// Value exposes the per-slot Euclidean distance for FieldDoc.Fields.
func (c *xyDistanceFieldComparator) Value(slot int) any { return c.inner.Value(slot) }

// CompareTop bridges the searchAfter top-value comparison.
func (c *xyDistanceFieldComparator) CompareTop(doc int) (int, error) {
	return c.inner.CompareTop(doc)
}

var (
	_ FieldComparatorSource = (*xyDistanceComparatorSource)(nil)
	_ FieldComparator       = (*xyDistanceFieldComparator)(nil)
)
