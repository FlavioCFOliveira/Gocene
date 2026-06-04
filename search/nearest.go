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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// NearestLatLonPoint returns the n documents whose indexed
// [document.LatLonPoint] in the named field are closest to
// (latitude, longitude), ordered nearest-first. The returned
// [TopFieldDocs] carries one [FieldDoc] per hit whose Fields[0] is the
// great-circle distance to the query in metres (a float64).
//
// It is the Go port of Apache Lucene 10.4.0
// org.apache.lucene.document.LatLonPoint#nearest. The Java factory lives
// in the document package next to LatLonPoint; the Gocene port lives in
// search/ because it consumes IndexSearcher/TopFieldDocs (search types)
// while delegating the BKD KNN traversal to the already-ported
// [document.Nearest] (which owns the cell/heap algorithm).
//
// # Wiring
//
// The per-segment BKD cursor is reached through the codec's
// index.PointValues: the on-disk reader exposes a GetPointTree() method
// returning a [bkd.PointTree], which this function bridges to the
// document package's [document.PointTreeWalker] cursor surface. A leaf
// whose PointValues does not expose the cursor (e.g. an in-test
// metadata-only stub) is skipped, matching Lucene's "no points for this
// field in this segment" behaviour.
func NearestLatLonPoint(
	searcher *IndexSearcher,
	field string,
	latitude, longitude float64,
	n int,
) (*TopFieldDocs, error) {
	if err := geo.CheckLatitude(latitude); err != nil {
		return nil, err
	}
	if err := geo.CheckLongitude(longitude); err != nil {
		return nil, err
	}
	if n < 1 {
		return nil, fmt.Errorf("nearest: n must be at least 1; got %d", n)
	}
	if field == "" {
		return nil, errors.New("nearest: field must not be empty")
	}
	if searcher == nil {
		return nil, errors.New("nearest: searcher must not be nil")
	}

	reader := searcher.GetIndexReader()
	if reader == nil {
		return nil, errors.New("nearest: searcher has no reader")
	}
	leaves, err := reader.Leaves()
	if err != nil {
		return nil, fmt.Errorf("nearest: enumerate leaves: %w", err)
	}

	readers := make([]document.PointTreeNearestReader, 0, len(leaves))
	totalHits := 0
	for _, leafCtx := range leaves {
		leaf := leafCtx.LeafReader()
		if leaf == nil {
			continue
		}
		pv, ok := leafPointValues(leaf, field)
		if !ok || pv == nil {
			continue
		}
		treeProvider, ok := pv.(pointTreeProvider)
		if !ok {
			continue
		}
		tree, err := treeProvider.GetPointTree()
		if err != nil {
			return nil, fmt.Errorf("nearest: open point tree for %q: %w", field, err)
		}
		totalHits += pv.GetDocCount()
		var liveDocs util.Bits
		if lr, ok := leaf.(interface{ GetLiveDocs() util.Bits }); ok {
			liveDocs = lr.GetLiveDocs()
		}
		readers = append(readers, document.PointTreeNearestReader{
			Tree:     &bkdPointTreeWalker{tree: tree},
			LiveDocs: liveDocs,
			DocBase:  leafCtx.DocBase(),
		})
	}

	hits, err := document.Nearest(latitude, longitude, readers, n)
	if err != nil {
		return nil, err
	}

	fieldDocs := make([]*FieldDoc, len(hits))
	for i, hit := range hits {
		meters := util.HaversinMetersFromSortKey(hit.DistanceSortKey)
		fieldDocs[i] = NewFieldDocWithFields(hit.DocID, 0.0, []any{meters})
	}
	totalHitsObj := NewTotalHits(int64(totalHits), EQUAL_TO)
	return NewTopFieldDocsWithFieldDocs(totalHitsObj, fieldDocs, nil), nil
}

// pointTreeProvider is the extended index.PointValues surface a
// BKD-backed reader exposes beyond the metadata-only / Intersect
// surfaces: a cursor factory used by the nearest-neighbour walk. The
// codec's on-disk *pointValues satisfies it; the type assertion succeeds
// for the real reader (the same pattern PointRangeQuery uses to reach
// the Intersect surface).
type pointTreeProvider interface {
	GetPointTree() (bkd.PointTree, error)
}

// leafPointValues resolves the field's index.PointValues from a leaf
// reader, returning (nil, false) when the leaf does not expose points or
// the field is unknown.
func leafPointValues(leaf index.LeafReaderInterface, field string) (index.PointValues, bool) {
	type pointReader interface {
		GetPointValues(field string) (index.PointValues, error)
	}
	pr, ok := leaf.(pointReader)
	if !ok {
		return nil, false
	}
	pv, err := pr.GetPointValues(field)
	if err != nil || pv == nil {
		return nil, false
	}
	return pv, true
}

// bkdPointTreeWalker adapts a [bkd.PointTree] cursor to the
// [document.PointTreeWalker] surface the nearest-neighbour algorithm
// drives. It reconciles two shape differences:
//
//   - bkd.PointTree.MoveToChild / MoveToSibling return (bool, error)
//     while document.PointTreeWalker returns a plain bool. A traversal
//     error is recorded on the walker and surfaced through the next
//     VisitDocValues call (the algorithm always visits a cell after
//     descending into it), so no error is silently dropped.
//
//   - bkd.PointTree.VisitDocValues takes a bkd.IntersectVisitor (Compare
//     → codecs.Relation) while document.PointTreeWalker.VisitDocValues
//     takes a document.PointTreeNearestVisitor (Compare →
//     document.PointTreeCellRelation). The adapter bridges the two
//     visitor surfaces.
type bkdPointTreeWalker struct {
	tree    bkd.PointTree
	moveErr error
}

// MinPackedValue returns the current cell's packed minimum. The bkd
// cursor owns the slice; document.Nearest clones it defensively (via
// newNearestCell) before retaining it, so returning it directly is safe.
func (w *bkdPointTreeWalker) MinPackedValue() []byte { return w.tree.GetMinPackedValue() }

// MaxPackedValue returns the current cell's packed maximum.
func (w *bkdPointTreeWalker) MaxPackedValue() []byte { return w.tree.GetMaxPackedValue() }

// MoveToChild descends to the left child, returning false on a leaf. A
// cursor error is recorded and reported on the next VisitDocValues.
func (w *bkdPointTreeWalker) MoveToChild() bool {
	moved, err := w.tree.MoveToChild()
	if err != nil {
		w.moveErr = err
		return false
	}
	return moved
}

// MoveToSibling moves to the right sibling, returning false when none
// remains. A cursor error is recorded and reported on the next
// VisitDocValues.
func (w *bkdPointTreeWalker) MoveToSibling() bool {
	moved, err := w.tree.MoveToSibling()
	if err != nil {
		w.moveErr = err
		return false
	}
	return moved
}

// Clone returns an independent cursor positioned at the same cell.
func (w *bkdPointTreeWalker) Clone() document.PointTreeWalker {
	return &bkdPointTreeWalker{tree: w.tree.Clone()}
}

// VisitDocValues streams every point in the current (leaf) cell through
// visitor.VisitWithPackedValue, after surfacing any deferred cursor
// movement error.
func (w *bkdPointTreeWalker) VisitDocValues(visitor document.PointTreeNearestVisitor) error {
	if w.moveErr != nil {
		err := w.moveErr
		w.moveErr = nil
		return err
	}
	return w.tree.VisitDocValues(&nearestVisitorToBKD{v: visitor})
}

var _ document.PointTreeWalker = (*bkdPointTreeWalker)(nil)

// nearestVisitorToBKD adapts a document.PointTreeNearestVisitor to the
// bkd.IntersectVisitor surface bkd.PointTree.VisitDocValues invokes. The
// document visitor only exposes a per-(docID, packedValue) hook and a
// cell Compare; the bkd reader's Visit (doc-id-only) and Grow hooks are
// not used by the nearest-neighbour traversal (Lucene's NearestVisitor
// throws on the doc-id-only Visit), so they are no-ops here.
type nearestVisitorToBKD struct {
	v document.PointTreeNearestVisitor
}

// Visit is unreachable for the nearest-neighbour walk (the BKD reader
// only calls it for fully-inside leaves when bulk-visiting doc ids,
// which VisitDocValues does not do). It is a no-op for safety.
func (a *nearestVisitorToBKD) Visit(_ int) error { return nil }

func (a *nearestVisitorToBKD) VisitByPackedValue(docID int, packedValue []byte) error {
	return a.v.VisitWithPackedValue(docID, packedValue)
}

func (a *nearestVisitorToBKD) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	switch a.v.Compare(minPackedValue, maxPackedValue) {
	case document.PointTreeCellInsideQuery:
		return codecs.RelationCellInsideQuery
	case document.PointTreeCellCrossesQuery:
		return codecs.RelationCellCrossesQuery
	default:
		return codecs.RelationCellOutsideQuery
	}
}

func (a *nearestVisitorToBKD) Grow(_ int) {}

var _ bkd.IntersectVisitor = (*nearestVisitorToBKD)(nil)
