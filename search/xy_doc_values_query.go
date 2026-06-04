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
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// xyDocValuesQuery is the doc-values-backed query that matches every
// document whose indexed XYDocValuesField point lies inside any of the
// supplied XY geometries. It is the executable Go port of Apache Lucene
// 10.4.0 org.apache.lucene.document.XYDocValuesPointInGeometryQuery — the
// scorer that XYDocValuesField.newSlow{Box,Distance,Polygon,Geometry}Query
// build (the box/distance/polygon helpers are convenience constructors that
// wrap an XYRectangle / XYCircle / XYPolygon geometry and route through this
// query).
//
// # Package placement
//
// The Java type lives in the document package; the Gocene port lives in
// search/ because it needs ConstantScoreWeight + TwoPhaseIterator (search
// types). document.XYDocValuesPointInGeometryQuery remains the structural
// surface; this is its runnable counterpart, reusing the same
// SortedNumericDocValues-per-leaf resolution the LatLon doc-values queries
// use.
type xyDocValuesQuery struct {
	*BaseQuery

	field      string
	geometries []geo.XYGeometry
	tree       geo.Component2D
}

// NewXYDocValuesQuery builds a doc-values point-in-geometry query over the
// named XYDocValuesField. At least one geometry is required; the
// geometries are combined into a single Component2D tree (matching
// XYGeometry.create on the Java side).
//
// The box, distance and polygon variants of the Java API are obtained by
// passing the corresponding geometry: geo.NewXYRectangle for newSlowBoxQuery,
// geo.NewXYCircle for newSlowDistanceQuery, geo.NewXYPolygon for
// newSlowPolygonQuery.
func NewXYDocValuesQuery(field string, geometries ...geo.XYGeometry) (Query, error) {
	if field == "" {
		return nil, errors.New("search: XYDocValuesQuery: field must not be empty")
	}
	if len(geometries) == 0 {
		return nil, errors.New("search: XYDocValuesQuery: at least one geometry is required")
	}
	for i, g := range geometries {
		if g == nil {
			return nil, fmt.Errorf("search: XYDocValuesQuery: geometry at index %d is nil", i)
		}
	}
	tree, err := geo.CreateXYGeometry(geometries...)
	if err != nil {
		return nil, fmt.Errorf("search: XYDocValuesQuery: build geometry tree: %w", err)
	}
	dup := make([]geo.XYGeometry, len(geometries))
	copy(dup, geometries)
	return &xyDocValuesQuery{
		BaseQuery:  &BaseQuery{},
		field:      field,
		geometries: dup,
		tree:       tree,
	}, nil
}

// GetField returns the query's field name.
func (q *xyDocValuesQuery) GetField() string { return q.field }

// Visit dispatches to the QueryVisitor's VisitLeaf when the field is
// accepted, mirroring the two-step accept/visitLeaf protocol.
func (q *xyDocValuesQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

// Rewrite returns the query unchanged. The explicit override is required
// because the type embeds *BaseQuery: relying on the promoted
// BaseQuery.Rewrite would return the inner *BaseQuery receiver, erasing
// this query's CreateWeight override so the rewritten query would silently
// match zero documents.
func (q *xyDocValuesQuery) Rewrite(_ IndexReader) (Query, error) { return q, nil }

// Clone returns the query itself (logically immutable after construction).
func (q *xyDocValuesQuery) Clone() Query { return q }

// Equals reports structural equality: same field and same geometry slice.
func (q *xyDocValuesQuery) Equals(other Query) bool {
	o, ok := other.(*xyDocValuesQuery)
	if !ok {
		return false
	}
	if q.field != o.field || len(q.geometries) != len(o.geometries) {
		return false
	}
	for i := range q.geometries {
		if q.geometries[i] != o.geometries[i] {
			return false
		}
	}
	return true
}

// HashCode folds the field hash with the geometry count (the geometries
// themselves are interface values whose stable hash is out of scope here;
// Equals carries the precise comparison).
func (q *xyDocValuesQuery) HashCode() int {
	h := 0xC4_44_56_51 // "XYDV" salt
	h = 31*h + stringHash(q.field)
	h = 31*h + len(q.geometries)
	return h
}

// String renders a human-readable form.
func (q *xyDocValuesQuery) String() string {
	return fmt.Sprintf("XYDocValuesQuery(field=%s, geometries=%d)", q.field, len(q.geometries))
}

// CreateWeight builds a ConstantScoreWeight whose per-leaf supplier
// resolves the SortedNumericDocValues stream and wraps a TwoPhaseIterator
// that tests each indexed (x, y) point against the Component2D tree.
func (q *xyDocValuesQuery) CreateWeight(_ *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE_NO_SCORES
	if needsScores {
		mode = COMPLETE
	}

	supplier := func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
		values, err := leafSortedNumeric(ctx, q.field)
		if err != nil {
			return nil, err
		}
		if values == nil {
			return nil, nil
		}
		maxDoc := 0
		if r := ctx.LeafReader(); r != nil {
			maxDoc = r.MaxDoc()
		}
		approx := newSortedNumericApproximation(values, maxDoc)
		tpi := NewTwoPhaseIterator(approx, func() (bool, error) {
			return q.matches(values, approx.DocID())
		})
		return NewConstantScoreScorerSupplier(
			boost,
			mode,
			approx.Cost(),
			func(_ int64) (DocIdSetIterator, error) {
				return tpi.AsDocIdSetIterator(), nil
			},
		), nil
	}

	cacheable := func(ctx *index.LeafReaderContext) bool {
		return index.IsDocValuesCacheable(ctx, q.field)
	}

	return NewConstantScoreWeight(q, boost, supplier, cacheable), nil
}

// matches scans a document's packed (x, y) values and returns true as soon
// as one falls inside the Component2D tree. The packed long carries the
// sortable-int-encoded x in the high 32 bits and y in the low 32 bits,
// matching document.EncodeXYAsLong.
func (q *xyDocValuesQuery) matches(values index.SortedNumericDocValues, docID int) (bool, error) {
	vs, err := index.DrainSortedNumeric(values, docID)
	if err != nil {
		return false, err
	}
	for _, v := range vs {
		x := float64(geo.XYDecode(int32(uint64(v) >> 32)))
		y := float64(geo.XYDecode(int32(v & 0xFFFFFFFF)))
		if q.tree.Contains(x, y) {
			return true, nil
		}
	}
	return false, nil
}

var _ Query = (*xyDocValuesQuery)(nil)
