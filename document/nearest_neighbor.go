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

package document

import (
	"bytes"
	"container/heap"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NearestNeighbor is the Go port of Lucene 10.4.0
// org.apache.lucene.document.NearestNeighbor (lucene/core/src/java/
// org/apache/lucene/document/NearestNeighbor.java).
//
// It performs KNN search on top of the 2D lat/lon BKD-indexed points
// produced by [LatLonPoint]. Given a query (pointLat, pointLon) it
// walks the per-segment BKD trees from each reader in order of the
// best possible (approximate) distance to the query, collecting the
// top-N closest documents into a bounded max-heap keyed by the
// SloppyMath haversin sort key.
//
// # Package placement
//
// The sole Java caller of [NearestNeighbor] is
// org.apache.lucene.document.LatLonPoint#nearest, which lives in the
// same package and treats the helper as package-private. The Gocene
// port preserves the same neighbourhood: [NearestNeighbor] lives next
// to [LatLonPoint] in the document package, and stays unexported at
// the type level. The factory entry point [Nearest] is exported so
// that the to-be-ported LatLonPoint.Nearest factory (backlog #2697)
// can delegate to it without breaking the document/search layering.
//
// # Visitor surface and the deferred PointValues port
//
// Lucene's [NearestNeighbor] consumes the full
// org.apache.lucene.index.PointValues.PointTree SPI
// (moveToChild / moveToSibling / clone / visitDocValues /
// getMinPackedValue / getMaxPackedValue), plus an IntersectVisitor and
// a Relation enum. The Gocene index.PointValues interface currently
// exposes only the metadata-only surface (min/max/numDims/bytesPerDim/
// docCount); the visitor-driven PointTree contract is being landed in
// stages alongside the broader BKD reader work.
//
// To keep this port self-contained and unblock the document/* layer
// without churning the index layer, [NearestNeighbor] consumes the
// following package-local interfaces:
//
//   - [PointTreeWalker] mirrors the subset of PointTree the algorithm
//     touches: cursor movement, packed-value accessors, a leaf-only
//     visitDocValues entry point, and a defensive clone.
//
//   - [PointTreeNearestVisitor] is the per-doc / per-(doc,packedValue)
//     visitor handed to [PointTreeWalker.VisitDocValues]. It also
//     exposes the cell-relation compare that the cell-queue uses to
//     prune subtrees, mirroring PointValues.IntersectVisitor + Relation.
//
//   - [PointTreeNearestReader] is the per-segment reader surface — it
//     bundles a [PointTreeWalker] root, a [util.Bits] live-docs view
//     and the doc-base offset.
//
// These types live in this file so the document package can ship a
// working KNN helper today; once the full PointValues port lands the
// public Nearest factory can be re-pointed at the index-layer types
// without changing observable behaviour. The semantics encoded here —
// approximate-best-distance cell ordering, max-heap top-N collection,
// periodic bbox refinement, dateline-aware longitude tests — match
// the Java reference line-for-line.
type NearestNeighbor struct{}

// nearestCell holds one BKD subtree cell in the priority queue, keyed
// by the approximate best distance from the query point.
//
// It is the Go port of the Java record
// org.apache.lucene.document.NearestNeighbor.Cell. As in Java, the
// constructor defensively clones the packed-value byte slices so the
// caller is free to reuse its scratch buffers between offers.
//
// The receiver order in [nearestCellHeap] mirrors Java's
// Comparable<Cell>: ascending distanceSortKey, so the min-heap pops
// the closest-by-approximation cell first.
type nearestCell struct {
	tree            PointTreeWalker
	readerIndex     int
	minPacked       []byte
	maxPacked       []byte
	distanceSortKey float64
}

// newNearestCell constructs a [nearestCell], cloning the packed-value
// slices to mirror the defensive copy in the Java record's compact
// constructor.
func newNearestCell(tree PointTreeWalker, readerIndex int, minPacked, maxPacked []byte, distanceSortKey float64) *nearestCell {
	return &nearestCell{
		tree:            tree,
		readerIndex:     readerIndex,
		minPacked:       bytes.Clone(minPacked),
		maxPacked:       bytes.Clone(maxPacked),
		distanceSortKey: distanceSortKey,
	}
}

// String mirrors Java NearestNeighbor.Cell#toString, including the
// decoded lat/lon range and the distanceSortKey. The treeRepr field is
// derived from PointTreeWalker.String() when available; otherwise we
// fall back to a generic placeholder so the output stays useful in
// tests and debug logging.
func (c *nearestCell) String() string {
	minLat := geo.DecodeLatitudeBytes(c.minPacked, 0)
	minLon := geo.DecodeLongitudeBytes(c.minPacked, 4)
	maxLat := geo.DecodeLatitudeBytes(c.maxPacked, 0)
	maxLon := geo.DecodeLongitudeBytes(c.maxPacked, 4)
	treeRepr := "<tree>"
	if s, ok := c.tree.(fmt.Stringer); ok {
		treeRepr = s.String()
	}
	return fmt.Sprintf(
		"Cell(readerIndex=%d %s lat=%g TO %g, lon=%g TO %g; distanceSortKey=%g)",
		c.readerIndex, treeRepr, minLat, maxLat, minLon, maxLon, c.distanceSortKey,
	)
}

// nearestCellHeap is a min-heap of [nearestCell] ordered by
// distanceSortKey ascending. It mirrors the natural ordering of
// Java's PriorityQueue<Cell>.
type nearestCellHeap []*nearestCell

// Len implements heap.Interface.
func (h nearestCellHeap) Len() int { return len(h) }

// Less implements heap.Interface: ascending distanceSortKey,
// equivalent to Double.compare in Java's Cell.compareTo.
func (h nearestCellHeap) Less(i, j int) bool {
	return h[i].distanceSortKey < h[j].distanceSortKey
}

// Swap implements heap.Interface.
func (h nearestCellHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push implements heap.Interface.
func (h *nearestCellHeap) Push(x any) { *h = append(*h, x.(*nearestCell)) }

// Pop implements heap.Interface.
func (h *nearestCellHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// NearestHit holds one match from [NearestNeighbor.Nearest].
//
// It is the Go port of Lucene's NearestNeighbor.NearestHit. The doc id
// is fully-globalised (segment doc-base added), and DistanceSortKey
// is the SloppyMath haversin sort key (see
// [util.HaversinSortKey]); convert to metres with
// [util.HaversinMetersFromSortKey].
type NearestHit struct {
	DocID           int
	DistanceSortKey float64
}

// String returns a human-readable summary mirroring the Java toString.
func (h NearestHit) String() string {
	return fmt.Sprintf("NearestHit(docID=%d distanceSortKey=%g)", h.DocID, h.DistanceSortKey)
}

// nearestHitHeap is a max-heap of [NearestHit] keyed primarily by
// DistanceSortKey (worst — i.e. largest — at the head, so it is the
// one popped to make room for a better candidate), and secondarily by
// DocID (higher first — so two hits at the same distance prefer the
// lower doc id, matching Java's tie-break "b.docID - a.docID").
//
// The corresponding Java construct is the PriorityQueue<NearestHit>
// in NearestNeighbor.nearest, whose comparator returns
// {@code -Double.compare(a, b)} on the distance and
// {@code b.docID - a.docID} on the tie-break.
type nearestHitHeap []*NearestHit

// Len implements heap.Interface.
func (h nearestHitHeap) Len() int { return len(h) }

// Less implements heap.Interface: worst-first (max-heap by
// distanceSortKey; tie-broken by higher docID).
func (h nearestHitHeap) Less(i, j int) bool {
	if h[i].DistanceSortKey != h[j].DistanceSortKey {
		return h[i].DistanceSortKey > h[j].DistanceSortKey
	}
	return h[i].DocID > h[j].DocID
}

// Swap implements heap.Interface.
func (h nearestHitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push implements heap.Interface.
func (h *nearestHitHeap) Push(x any) { *h = append(*h, x.(*NearestHit)) }

// Pop implements heap.Interface.
func (h *nearestHitHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// PointTreeCellRelation classifies how a BKD cell intersects the
// query, mirroring org.apache.lucene.index.PointValues.Relation. The
// document-local copy avoids a dependency on the not-yet-ported index
// SPI; see the package overview for details.
type PointTreeCellRelation int

const (
	// PointTreeCellOutsideQuery means the cell lies fully outside the
	// query and can be pruned.
	PointTreeCellOutsideQuery PointTreeCellRelation = iota
	// PointTreeCellInsideQuery means the cell lies fully inside the
	// query. [NearestNeighbor] never returns this value (it does not
	// distinguish "inside" from "crosses") but the enum is defined for
	// completeness with PointValues.Relation.
	PointTreeCellInsideQuery
	// PointTreeCellCrossesQuery means the cell partially overlaps the
	// query and must be explored further.
	PointTreeCellCrossesQuery
)

// String mirrors Relation.name() for diagnostic output.
func (r PointTreeCellRelation) String() string {
	switch r {
	case PointTreeCellOutsideQuery:
		return "CELL_OUTSIDE_QUERY"
	case PointTreeCellInsideQuery:
		return "CELL_INSIDE_QUERY"
	case PointTreeCellCrossesQuery:
		return "CELL_CROSSES_QUERY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(r))
	}
}

// PointTreeNearestVisitor is the per-cell visitor surface
// [PointTreeWalker.VisitDocValues] receives. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor exercised by
// NearestNeighbor: a per-(docID, packedValue) callback and a cell
// relation compare. The doc-id-only visit hook is intentionally
// omitted — Java's NearestNeighbor.NearestVisitor throws
// AssertionError if called, so any implementation that exposes one
// would be exercising an unreachable path.
type PointTreeNearestVisitor interface {
	// VisitWithPackedValue is invoked once per leaf point. docID is
	// the per-segment doc id (not yet globalised); packedValue is the
	// 8-byte lat/lon wire form (4 bytes lat || 4 bytes lon).
	VisitWithPackedValue(docID int, packedValue []byte) error

	// Compare classifies how the cell [minPackedValue, maxPackedValue]
	// intersects the visitor's current bbox.
	Compare(minPackedValue, maxPackedValue []byte) PointTreeCellRelation
}

// PointTreeWalker is the cursor-shaped subset of
// org.apache.lucene.index.PointValues.PointTree consumed by
// [NearestNeighbor]. Implementations expose the current cell's packed
// bounds and a defensive clone so the algorithm can recurse left/right
// "concurrently" without disturbing the shared cursor.
type PointTreeWalker interface {
	// MinPackedValue returns the packed minimum point of the current
	// cell. The returned slice is read-only; callers that need to
	// retain it must clone.
	MinPackedValue() []byte

	// MaxPackedValue returns the packed maximum point of the current
	// cell. The returned slice is read-only; callers that need to
	// retain it must clone.
	MaxPackedValue() []byte

	// MoveToChild moves the cursor to the first child of the current
	// cell and returns true. It returns false when the current cell is
	// a leaf, leaving the cursor unchanged.
	MoveToChild() bool

	// MoveToSibling moves the cursor to the next sibling of the
	// current cell and returns true. It returns false when no sibling
	// remains, leaving the cursor unchanged.
	MoveToSibling() bool

	// Clone returns an independent cursor positioned at the same cell
	// as the receiver. Mirrors PointTree.clone in Java, which the
	// algorithm uses to recurse into the left subtree without losing
	// the cursor's position before visiting the right subtree.
	Clone() PointTreeWalker

	// VisitDocValues invokes visitor.VisitWithPackedValue for every
	// point in the current (leaf) cell. The contract mirrors Java's
	// PointTree.visitDocValues(IntersectVisitor): visitor.Compare is
	// not re-invoked here (the cell relation is established by the
	// caller before the call).
	VisitDocValues(visitor PointTreeNearestVisitor) error
}

// PointTreeNearestReader is the per-segment input bundle handed to
// [Nearest]. It mirrors the parallel List<PointValues>/List<Bits>/
// IntArrayList trio used by the Java reference, collapsed into a
// single struct so the call site does not have to keep three lists in
// lock-step.
//
// The Tree field carries the root [PointTreeWalker] for the segment;
// LiveDocs may be nil to indicate "all docs are live"; DocBase is the
// global doc-id offset to add to per-segment ids before pushing them
// into a [NearestHit].
type PointTreeNearestReader struct {
	Tree     PointTreeWalker
	LiveDocs util.Bits
	DocBase  int
}

// Nearest is the Go port of Lucene 10.4.0
// org.apache.lucene.document.NearestNeighbor#nearest. It returns the
// top n hits closest to (pointLat, pointLon) across all readers,
// sorted best-first (smallest haversin sort key at index 0).
//
// The algorithm:
//
//  1. Seed a min-heap of cells (one root cell per reader), ordered by
//     the approximate best distance from the cell to the query.
//
//  2. Pop the closest cell, prune it against the visitor's current
//     dynamic bbox, and either descend (non-leaf) by cloning the
//     cursor into the left child and offering both children back into
//     the queue, or visit it (leaf) by streaming its packed points
//     through the visitor.
//
//  3. The visitor maintains a bounded max-heap of n hits keyed by
//     the haversin sort key. Each time the heap is full and a new
//     candidate evicts the previous worst, the visitor periodically
//     tightens its bbox to the spherical cap of the new worst
//     distance — the same 1024/64 throttling schedule as Java.
//
//  4. When the cell queue is exhausted, drain the hit heap into the
//     output slice, filling from the tail so the result is best-first.
//
// n must be > 0; readers may be empty (Nearest then returns an empty
// slice). Errors from any [PointTreeWalker.VisitDocValues] call are
// surfaced verbatim.
func Nearest(pointLat, pointLon float64, readers []PointTreeNearestReader, n int) ([]NearestHit, error) {
	if n <= 0 {
		return nil, fmt.Errorf("nearest: n must be > 0, got %d", n)
	}

	hitQueue := &nearestHitHeap{}
	heap.Init(hitQueue)

	visitor := newNearestVisitor(hitQueue, n, pointLat, pointLon)

	cellQueue := &nearestCellHeap{}
	heap.Init(cellQueue)

	// Add the root cell of each reader into the cell queue.
	for i, reader := range readers {
		if reader.Tree == nil {
			return nil, fmt.Errorf("nearest: reader[%d].Tree is nil", i)
		}
		minPacked := reader.Tree.MinPackedValue()
		maxPacked := reader.Tree.MaxPackedValue()
		heap.Push(cellQueue, newNearestCell(
			reader.Tree,
			i,
			minPacked,
			maxPacked,
			approxBestDistancePacked(minPacked, maxPacked, pointLat, pointLon),
		))
	}

	for cellQueue.Len() > 0 {
		cell := heap.Pop(cellQueue).(*nearestCell)

		// Prune cells that fall entirely outside the visitor's bbox.
		if visitor.Compare(cell.minPacked, cell.maxPacked) == PointTreeCellOutsideQuery {
			continue
		}

		// Leaf: stream its points through the visitor.
		if !cell.tree.MoveToChild() {
			visitor.curDocBase = readers[cell.readerIndex].DocBase
			visitor.curLiveDocs = readers[cell.readerIndex].LiveDocs
			if err := cell.tree.VisitDocValues(visitor); err != nil {
				return nil, err
			}
			continue
		}

		// Non-leaf: clone the cursor (we are now positioned on the
		// first child after MoveToChild) and offer both the cloned
		// child and the cursor's next sibling back into the queue.
		newTree := cell.tree.Clone()
		heap.Push(cellQueue, newNearestCell(
			newTree,
			cell.readerIndex,
			newTree.MinPackedValue(),
			newTree.MaxPackedValue(),
			approxBestDistancePacked(
				newTree.MinPackedValue(),
				newTree.MaxPackedValue(),
				pointLat, pointLon,
			),
		))

		// TODO(Lucene parity): we are assuming a binary tree, matching
		// the Java reference's "we are assuming a binary tree" comment.
		if cell.tree.MoveToSibling() {
			heap.Push(cellQueue, newNearestCell(
				cell.tree,
				cell.readerIndex,
				cell.tree.MinPackedValue(),
				cell.tree.MaxPackedValue(),
				approxBestDistancePacked(
					cell.tree.MinPackedValue(),
					cell.tree.MaxPackedValue(),
					pointLat, pointLon,
				),
			))
		}
	}

	// Drain the hit heap into the output slice, filling from the tail
	// so the worst hit lands at the end and the best lands at index 0.
	hits := make([]NearestHit, hitQueue.Len())
	downTo := len(hits) - 1
	for hitQueue.Len() > 0 {
		hit := heap.Pop(hitQueue).(*NearestHit)
		hits[downTo] = *hit
		downTo--
	}
	return hits, nil
}

// nearestVisitor implements [PointTreeNearestVisitor] and maintains
// the bounded max-heap of top-N hits, plus the dynamic bbox that
// prunes cells once enough hits have been collected.
//
// It is the Go port of the inner Java class
// NearestNeighbor.NearestVisitor; the field-for-field semantics are
// preserved, including the longitude double-range used to handle
// dateline-crossing bboxes (minLon..maxLon and the second
// minLon2..+inf range when the cap straddles the antimeridian).
type nearestVisitor struct {
	curDocBase  int
	curLiveDocs util.Bits

	topN     int
	hitQueue *nearestHitHeap

	pointLat float64
	pointLon float64

	setBottomCounter int

	minLon  float64
	maxLon  float64
	minLat  float64
	maxLat  float64
	minLon2 float64
}

// newNearestVisitor builds a [nearestVisitor] with the bbox set to
// the unconstrained extents Java seeds it with (lat/lon both spanning
// the full real line, minLon2 disabled).
func newNearestVisitor(hitQueue *nearestHitHeap, topN int, pointLat, pointLon float64) *nearestVisitor {
	return &nearestVisitor{
		topN:     topN,
		hitQueue: hitQueue,
		pointLat: pointLat,
		pointLon: pointLon,
		minLon:   math.Inf(-1),
		maxLon:   math.Inf(+1),
		minLat:   math.Inf(-1),
		maxLat:   math.Inf(+1),
		minLon2:  math.Inf(+1),
	}
}

// maybeUpdateBBox tightens the search bbox to the spherical cap of
// the current worst hit's distance, on the same 1024/64 schedule as
// Java: the first 1024 evictions always refresh, then once the
// counter rolls past 1024, only every 64th eviction refreshes (the
// "(setBottomCounter & 0x3F) == 0x3F" guard).
//
// Errors from [geo.FromPointDistance] are swallowed: the Java
// reference cannot fail this call (the input is always a valid
// (lat,lon) plus a non-negative radius). If our defensive validation
// were ever to reject the input, we keep the previous bbox unchanged
// and merely skip this update, mirroring "no tightening this round".
func (v *nearestVisitor) maybeUpdateBBox() {
	defer func() { v.setBottomCounter++ }()

	if v.setBottomCounter >= 1024 && (v.setBottomCounter&0x3F) != 0x3F {
		return
	}

	// The hit heap is non-empty here (we only call this after pushing
	// at least one hit); the peek is the current worst.
	worst := (*v.hitQueue)[0]
	radius := util.HaversinMetersFromSortKey(worst.DistanceSortKey)
	box, err := geo.FromPointDistance(v.pointLat, v.pointLon, radius)
	if err != nil {
		// Leave bbox untouched; the next eviction will retry on the
		// same schedule.
		return
	}

	v.minLat = box.MinLat()
	v.maxLat = box.MaxLat()
	if box.CrossesDateline() {
		// First range covers (-inf, maxLon].
		v.minLon = math.Inf(-1)
		v.maxLon = box.MaxLon()
		// Second range covers [minLon, +inf).
		v.minLon2 = box.MinLon()
	} else {
		v.minLon = box.MinLon()
		v.maxLon = box.MaxLon()
		// Disable the second range.
		v.minLon2 = math.Inf(+1)
	}
}

// VisitWithPackedValue implements [PointTreeNearestVisitor]. It
// decodes the per-doc lat/lon, applies the dateline-aware bbox
// filter, computes the haversin sort key and either pushes the hit
// (heap not yet full) or evicts the current worst when the new
// candidate is strictly closer (with a lower-docID tie-break).
func (v *nearestVisitor) VisitWithPackedValue(docID int, packedValue []byte) error {
	if v.curLiveDocs != nil && !v.curLiveDocs.Get(docID) {
		return nil
	}

	docLat := geo.DecodeLatitudeBytes(packedValue, 0)
	docLon := geo.DecodeLongitudeBytes(packedValue, 4)

	// Bounding-box test, matching the two-range longitude check the
	// Java visitor uses to support dateline-crossing caps.
	if docLat < v.minLat || docLat > v.maxLat {
		return nil
	}
	if (docLon < v.minLon || docLon > v.maxLon) && docLon < v.minLon2 {
		return nil
	}

	distanceSortKey := util.HaversinSortKey(v.pointLat, v.pointLon, docLat, docLon)
	fullDocID := v.curDocBase + docID

	if v.hitQueue.Len() == v.topN {
		worst := (*v.hitQueue)[0]
		if distanceSortKey < worst.DistanceSortKey ||
			(distanceSortKey == worst.DistanceSortKey && fullDocID < worst.DocID) {
			// In-place update + heap.Fix is one heap op (log n) vs the
			// Pop+Push pair the Java reference uses (2 log n). The
			// observable contents of the heap are identical.
			worst.DocID = fullDocID
			worst.DistanceSortKey = distanceSortKey
			heap.Fix(v.hitQueue, 0)
			v.maybeUpdateBBox()
		}
		return nil
	}

	heap.Push(v.hitQueue, &NearestHit{
		DocID:           fullDocID,
		DistanceSortKey: distanceSortKey,
	})
	return nil
}

// Compare implements [PointTreeNearestVisitor]. It mirrors the
// two-range Java compare: a cell falls outside the query when its
// latitudes do not overlap [minLat,maxLat] OR when both longitude
// ranges miss the cell's longitude span.
func (v *nearestVisitor) Compare(minPackedValue, maxPackedValue []byte) PointTreeCellRelation {
	cellMinLat := geo.DecodeLatitudeBytes(minPackedValue, 0)
	cellMinLon := geo.DecodeLongitudeBytes(minPackedValue, 4)
	cellMaxLat := geo.DecodeLatitudeBytes(maxPackedValue, 0)
	cellMaxLon := geo.DecodeLongitudeBytes(maxPackedValue, 4)

	if cellMaxLat < v.minLat ||
		v.maxLat < cellMinLat ||
		((cellMaxLon < v.minLon || v.maxLon < cellMinLon) && cellMaxLon < v.minLon2) {
		return PointTreeCellOutsideQuery
	}
	return PointTreeCellCrossesQuery
}

// approxBestDistancePacked computes the approximate best (i.e.
// smallest) haversin sort key from any point in the [min, max] packed
// cell to (pointLat, pointLon). The packed values must not cross the
// dateline (a BKD cell never does, by construction).
//
// This is the Go port of the package-private double-argument overload
// of NearestNeighbor#approxBestDistance, fed by the packed-value
// overload that decodes the four corners up front.
func approxBestDistancePacked(minPacked, maxPacked []byte, pointLat, pointLon float64) float64 {
	minLat := geo.DecodeLatitudeBytes(minPacked, 0)
	minLon := geo.DecodeLongitudeBytes(minPacked, 4)
	maxLat := geo.DecodeLatitudeBytes(maxPacked, 0)
	maxLon := geo.DecodeLongitudeBytes(maxPacked, 4)
	return approxBestDistanceLatLon(minLat, maxLat, minLon, maxLon, pointLat, pointLon)
}

// approxBestDistanceLatLon mirrors the inner double-overload of
// NearestNeighbor#approxBestDistance: if the query point falls inside
// the cell return 0, otherwise return the smallest haversin sort key
// to any of the four corners.
//
// TODO(Lucene parity): the Java reference notes this is an
// approximation and could be tightened to the true minimum distance
// between the point and any point on the box; we preserve the
// approximation so result ordering matches the Java byte-for-byte.
func approxBestDistanceLatLon(minLat, maxLat, minLon, maxLon, pointLat, pointLon float64) float64 {
	if pointLat >= minLat && pointLat <= maxLat && pointLon >= minLon && pointLon <= maxLon {
		return 0.0
	}
	d1 := util.HaversinSortKey(pointLat, pointLon, minLat, minLon)
	d2 := util.HaversinSortKey(pointLat, pointLon, minLat, maxLon)
	d3 := util.HaversinSortKey(pointLat, pointLon, maxLat, maxLon)
	d4 := util.HaversinSortKey(pointLat, pointLon, maxLat, minLon)
	return math.Min(math.Min(d1, d2), math.Min(d3, d4))
}

// Compile-time check: ensure [nearestVisitor] satisfies the visitor
// surface even as the interface evolves.
var _ PointTreeNearestVisitor = (*nearestVisitor)(nil)

// NearestNeighbor is exported as a zero-value type only so callers can
// reference the algorithm by name in documentation or test scaffolds;
// the algorithm itself is the package-level [Nearest] function.
var _ = NearestNeighbor{}
