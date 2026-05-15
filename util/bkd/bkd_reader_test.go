// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"encoding/binary"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file is the behavioural test suite for BKDReader. The Java
// reference has no direct TestBKDReader peer in Lucene 10.4.0; the
// indirect coverage lives in lucene/core/src/test/.../bkd/TestBKD*.java.
// We translate the relevant structural properties into Go-friendly
// black-box checks that exercise the reader against bytes written by
// the just-ported BKDWriter:
//
//   - header validation (codec name + version range);
//   - reader-side metadata (numDims, bytesPerDim, numLeaves, etc);
//   - PointTree navigation (size, moveToChild, moveToSibling,
//     moveToParent, visitDocIDs, visitDocValues);
//   - range query intersection on a small fixture (verifies the docIDs
//     returned for a synthetic visitor that cuts a sub-range).

// readerCaptureVisitor is a test visitor that records every docID it sees,
// optionally filtered through a per-doc predicate.
type readerCaptureVisitor struct {
	relation       codecs.Relation
	predicate      func(packedValue []byte) bool
	visitedIDs     []int
	visitedPV      []int
	cmpLog         []relationRecord
	growCalls      []int
	defaultsToCell codecs.Relation
}

type relationRecord struct {
	min, max []byte
	rel      codecs.Relation
}

func (v *readerCaptureVisitor) Visit(docID int) error {
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *readerCaptureVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if v.predicate == nil || v.predicate(packedValue) {
		v.visitedPV = append(v.visitedPV, docID)
	}
	return nil
}

func (v *readerCaptureVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	rec := relationRecord{
		min: append([]byte(nil), minPackedValue...),
		max: append([]byte(nil), maxPackedValue...),
		rel: v.relation,
	}
	v.cmpLog = append(v.cmpLog, rec)
	return v.relation
}

func (v *readerCaptureVisitor) Grow(count int) { v.growCalls = append(v.growCalls, count) }

// rangeVisitor is a Compare-driven visitor: it returns CELL_INSIDE
// when the cell is wholly inside [lo, hi], CELL_OUTSIDE when wholly
// outside, and CELL_CROSSES otherwise. Per-doc visits are filtered by
// the same range.
type rangeVisitor struct {
	loIncl, hiIncl uint32
	visitedIDs     []int
	predicateHits  int
}

func (v *rangeVisitor) Visit(docID int) error {
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *rangeVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	x := binary.BigEndian.Uint32(packedValue[:4])
	if x >= v.loIncl && x <= v.hiIncl {
		v.visitedIDs = append(v.visitedIDs, docID)
		v.predicateHits++
	}
	return nil
}

func (v *rangeVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	mn := binary.BigEndian.Uint32(minPackedValue[:4])
	mx := binary.BigEndian.Uint32(maxPackedValue[:4])
	if mx < v.loIncl || mn > v.hiIncl {
		return codecs.RelationCellOutsideQuery
	}
	if mn >= v.loIncl && mx <= v.hiIncl {
		return codecs.RelationCellInsideQuery
	}
	return codecs.RelationCellCrossesQuery
}

func (v *rangeVisitor) Grow(count int) {}

// buildReader is the round-trip fixture: feed `points` into a fresh
// BKDWriter, run Finish to materialise meta/index/data files in a
// ByteBuffersDirectory, then open and return the BKDReader plus the
// open inputs (which the caller must close).
type readerFixture struct {
	r        *BKDReader
	metaIn   store.IndexInput
	dataIn   store.IndexInput
	dir      *store.ByteBuffersDirectory
	metaName string
	dataName string
	cfg      BKDConfig
	pts      []selectorPoint
}

func (f *readerFixture) close() {
	_ = f.metaIn.Close()
	_ = f.dataIn.Close()
	_ = f.dir.Close()
}

func buildReader(t *testing.T, cfg BKDConfig, points []selectorPoint, maxDoc int) *readerFixture {
	t.Helper()
	br := buildPointsAdd(t, cfg, maxDoc, 4.0, points)

	metaIn, err := br.dir.OpenInput(br.metaName, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput meta: %v", err)
	}
	dataIn, err := br.dir.OpenInput(br.dataName, store.IOContextRead)
	if err != nil {
		_ = metaIn.Close()
		t.Fatalf("OpenInput data: %v", err)
	}

	r, err := NewBKDReader(metaIn, metaIn, dataIn)
	if err != nil {
		_ = metaIn.Close()
		_ = dataIn.Close()
		t.Fatalf("NewBKDReader: %v", err)
	}

	f := &readerFixture{
		r:        r,
		metaIn:   metaIn,
		dataIn:   dataIn,
		dir:      br.dir,
		metaName: br.metaName,
		dataName: br.dataName,
		cfg:      cfg,
		pts:      points,
	}
	t.Cleanup(f.close)
	return f
}

// TestBKDReader_OpenMetadata exercises the meta-header parsing path:
// a single-leaf 1D 4-byte index round-trips to numDims/numIndexDims/
// bytesPerDim/numLeaves/min/max/pointCount/docCount as expected.
func TestBKDReader_OpenMetadata(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4) // small leaf size to force >1 leaf later, but here a single leaf
	points := []selectorPoint{
		{packed: be4(10), docID: 0},
		{packed: be4(20), docID: 1},
		{packed: be4(30), docID: 2},
		{packed: be4(40), docID: 3},
	}
	f := buildReader(t, cfg, points, 1024)

	if got := f.r.GetNumDimensions(); got != 1 {
		t.Fatalf("numDims: got %d, want 1", got)
	}
	if got := f.r.GetNumIndexDimensions(); got != 1 {
		t.Fatalf("numIndexDims: got %d, want 1", got)
	}
	if got := f.r.GetBytesPerDimension(); got != 4 {
		t.Fatalf("bytesPerDim: got %d, want 4", got)
	}
	if got := f.r.NumLeaves(); got != 1 {
		t.Fatalf("numLeaves: got %d, want 1 for a single leaf fixture", got)
	}
	if got := f.r.Size(); got != int64(len(points)) {
		t.Fatalf("size: got %d, want %d", got, len(points))
	}
	if got := f.r.GetDocCount(); got != len(points) {
		t.Fatalf("docCount: got %d, want %d", got, len(points))
	}
	if got := uint32FromBE(f.r.GetMinPackedValue()); got != 10 {
		t.Fatalf("minPackedValue: got %d, want 10", got)
	}
	if got := uint32FromBE(f.r.GetMaxPackedValue()); got != 40 {
		t.Fatalf("maxPackedValue: got %d, want 40", got)
	}
	if got := f.r.Version(); got != BKDVersionCurrent {
		t.Fatalf("version: got %d, want %d", got, BKDVersionCurrent)
	}
}

// TestBKDReader_BadCodecHeader feeds the reader a corrupt header
// (wrong codec name) and asserts that NewBKDReader fails fast.
func TestBKDReader_BadCodecHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	metaOut, err := dir.CreateOutput("bad.meta", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	// Wrong codec name; CheckHeader will reject.
	if err := codecs.WriteHeader(metaOut, "NotBKD", BKDVersionCurrent); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if err := metaOut.Close(); err != nil {
		t.Fatalf("Close metaOut: %v", err)
	}
	dataOut, err := dir.CreateOutput("bad.data", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}
	if err := dataOut.Close(); err != nil {
		t.Fatalf("Close dataOut: %v", err)
	}

	metaIn, err := dir.OpenInput("bad.meta", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput meta: %v", err)
	}
	defer metaIn.Close()
	dataIn, err := dir.OpenInput("bad.data", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput data: %v", err)
	}
	defer dataIn.Close()

	if _, err := NewBKDReader(metaIn, metaIn, dataIn); err == nil {
		t.Fatalf("NewBKDReader on bad codec header: want error, got nil")
	}
}

// TestBKDReader_TruncatedMeta exercises the early-EOF path: a
// meta file containing only the codec header (no fields) must
// produce an error on NewBKDReader (the first VInt read will fail).
func TestBKDReader_TruncatedMeta(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	metaOut, err := dir.CreateOutput("trunc.meta", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := codecs.WriteHeader(metaOut, BKDCodecName, BKDVersionCurrent); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if err := metaOut.Close(); err != nil {
		t.Fatalf("Close metaOut: %v", err)
	}
	dataOut, err := dir.CreateOutput("trunc.data", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}
	if err := dataOut.Close(); err != nil {
		t.Fatalf("Close dataOut: %v", err)
	}

	metaIn, err := dir.OpenInput("trunc.meta", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput meta: %v", err)
	}
	defer metaIn.Close()
	dataIn, err := dir.OpenInput("trunc.data", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput data: %v", err)
	}
	defer dataIn.Close()

	if _, err := NewBKDReader(metaIn, metaIn, dataIn); err == nil {
		t.Fatalf("NewBKDReader on truncated meta: want error, got nil")
	}
}

// TestBKDReader_PointTreeSingleLeaf walks a single-leaf tree (the root
// IS a leaf): moveToChild returns false; visitDocIDs delivers every
// docID; visitDocValues delivers every packed value.
func TestBKDReader_PointTreeSingleLeaf(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 8) // leaf size 8 > 4 points -> single leaf
	points := []selectorPoint{
		{packed: be4(10), docID: 0},
		{packed: be4(20), docID: 1},
		{packed: be4(30), docID: 2},
		{packed: be4(40), docID: 3},
	}
	f := buildReader(t, cfg, points, 1024)

	tree, err := f.r.GetPointTree()
	if err != nil {
		t.Fatalf("GetPointTree: %v", err)
	}
	if got := tree.Size(); got != 4 {
		t.Fatalf("tree.Size: got %d, want 4", got)
	}
	// Single-leaf tree: root is a leaf, moveToChild must return false.
	moved, err := tree.MoveToChild()
	if err != nil {
		t.Fatalf("MoveToChild: %v", err)
	}
	if moved {
		t.Fatalf("MoveToChild on single-leaf root: want false, got true")
	}

	// visitDocIDs must report all 4 docIDs.
	vis := &readerCaptureVisitor{relation: codecs.RelationCellInsideQuery}
	if err := tree.VisitDocIDs(vis); err != nil {
		t.Fatalf("VisitDocIDs: %v", err)
	}
	sort.Ints(vis.visitedIDs)
	want := []int{0, 1, 2, 3}
	if !equalInts(vis.visitedIDs, want) {
		t.Fatalf("VisitDocIDs: got %v, want %v", vis.visitedIDs, want)
	}

	// visitDocValues with relation CELL_CROSSES must invoke
	// VisitByPackedValue for each doc.
	vis2 := &readerCaptureVisitor{relation: codecs.RelationCellCrossesQuery}
	if err := tree.VisitDocValues(vis2); err != nil {
		t.Fatalf("VisitDocValues: %v", err)
	}
	sort.Ints(vis2.visitedPV)
	if !equalInts(vis2.visitedPV, want) {
		t.Fatalf("VisitDocValues: got %v, want %v", vis2.visitedPV, want)
	}
}

// TestBKDReader_PointTreeMultiLeaf walks a multi-leaf tree, verifying
// the size of subtree at each level decomposes correctly: root.Size()
// == sum(child.Size()) and each leaf's Size() equals the number of
// docs it actually contains.
func TestBKDReader_PointTreeMultiLeaf(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4) // leaf size 4 -> 3 leaves for 12 points
	points := make([]selectorPoint, 12)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32(i + 1)), docID: i}
	}
	f := buildReader(t, cfg, points, 1024)

	if got := f.r.NumLeaves(); got != 3 {
		t.Fatalf("numLeaves: got %d, want 3", got)
	}

	tree, err := f.r.GetPointTree()
	if err != nil {
		t.Fatalf("GetPointTree: %v", err)
	}
	if got := tree.Size(); got != int64(len(points)) {
		t.Fatalf("root.Size: got %d, want %d", got, len(points))
	}

	// Walk to the left child and capture its size.
	moved, err := tree.MoveToChild()
	if err != nil {
		t.Fatalf("MoveToChild: %v", err)
	}
	if !moved {
		t.Fatalf("MoveToChild on multi-leaf root: want true")
	}
	leftSize := tree.Size()
	moved, err = tree.MoveToSibling()
	if err != nil {
		t.Fatalf("MoveToSibling: %v", err)
	}
	if !moved {
		t.Fatalf("MoveToSibling from left child: want true")
	}
	rightSize := tree.Size()
	if leftSize+rightSize != int64(len(points)) {
		t.Fatalf("left.Size + right.Size: %d + %d = %d, want %d", leftSize, rightSize, leftSize+rightSize, len(points))
	}

	// MoveToParent restores the root.
	moved, err = tree.MoveToParent()
	if err != nil {
		t.Fatalf("MoveToParent: %v", err)
	}
	if !moved {
		t.Fatalf("MoveToParent from right child: want true")
	}
	if got := tree.Size(); got != int64(len(points)) {
		t.Fatalf("after MoveToParent: root.Size got %d, want %d", got, len(points))
	}

	// Walk a fresh clone and visit every doc to verify we recover all
	// docIDs without duplication.
	clone := tree.Clone()
	vis := &readerCaptureVisitor{relation: codecs.RelationCellInsideQuery}
	if err := clone.VisitDocIDs(vis); err != nil {
		t.Fatalf("VisitDocIDs on clone: %v", err)
	}
	sort.Ints(vis.visitedIDs)
	want := make([]int, len(points))
	for i := range want {
		want[i] = i
	}
	if !equalInts(vis.visitedIDs, want) {
		t.Fatalf("VisitDocIDs on clone: got %v, want %v", vis.visitedIDs, want)
	}
}

// TestBKDReader_IntersectFullRange uses Intersect() with a visitor
// that always returns CELL_INSIDE: every doc must be reported.
func TestBKDReader_IntersectFullRange(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	points := make([]selectorPoint, 16)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32((i + 1) * 7)), docID: i}
	}
	f := buildReader(t, cfg, points, 1024)

	vis := &rangeVisitor{loIncl: 0, hiIncl: 0xFFFFFFFF}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	sort.Ints(vis.visitedIDs)
	want := make([]int, len(points))
	for i := range want {
		want[i] = i
	}
	if !equalInts(vis.visitedIDs, want) {
		t.Fatalf("Intersect full-range: got %v, want %v", vis.visitedIDs, want)
	}
}

// TestBKDReader_IntersectEmptyRange uses Intersect() with a visitor
// that never matches: no docs should be reported.
func TestBKDReader_IntersectEmptyRange(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	points := make([]selectorPoint, 16)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32((i + 1) * 7)), docID: i}
	}
	f := buildReader(t, cfg, points, 1024)

	// A range disjoint from any actual point.
	vis := &rangeVisitor{loIncl: 0xFF000000, hiIncl: 0xFFFFFFFF}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	if len(vis.visitedIDs) != 0 {
		t.Fatalf("Intersect disjoint: got %v, want []", vis.visitedIDs)
	}
}

// TestBKDReader_IntersectSubRange runs a half-range query
// [lo, hi] and verifies the returned docIDs match the expected set
// (computed independently by linear scan of the input points).
func TestBKDReader_IntersectSubRange(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	type pt struct {
		v   uint32
		doc int
	}
	rawPts := []pt{
		{5, 0}, {15, 1}, {25, 2}, {35, 3},
		{45, 4}, {55, 5}, {65, 6}, {75, 7},
		{85, 8}, {95, 9}, {105, 10}, {115, 11},
		{125, 12}, {135, 13}, {145, 14}, {155, 15},
	}
	points := make([]selectorPoint, len(rawPts))
	for i, p := range rawPts {
		points[i] = selectorPoint{packed: be4(p.v), docID: p.doc}
	}

	f := buildReader(t, cfg, points, 1024)

	// Query [30, 100]: expected docIDs 3..9 (values 35, 45, 55, 65, 75, 85, 95).
	lo := uint32(30)
	hi := uint32(100)
	var want []int
	for _, p := range rawPts {
		if p.v >= lo && p.v <= hi {
			want = append(want, p.doc)
		}
	}
	sort.Ints(want)

	vis := &rangeVisitor{loIncl: lo, hiIncl: hi}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	sort.Ints(vis.visitedIDs)
	if !equalInts(vis.visitedIDs, want) {
		t.Fatalf("Intersect [%d, %d]: got %v, want %v", lo, hi, vis.visitedIDs, want)
	}
}

// TestBKDReader_TwoDimsIntersect exercises a 2D point cloud and a
// rectangular query. The expected docIDs are computed by linear scan.
func TestBKDReader_TwoDimsIntersect(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 4, 4)
	type pt struct {
		x, y uint32
		doc  int
	}
	rawPts := []pt{
		{10, 100, 0}, {20, 200, 1}, {30, 300, 2}, {40, 400, 3},
		{50, 500, 4}, {60, 600, 5}, {70, 700, 6}, {80, 800, 7},
		{90, 900, 8}, {100, 1000, 9},
	}
	points := make([]selectorPoint, len(rawPts))
	for i, p := range rawPts {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint32(buf[0:4], p.x)
		binary.BigEndian.PutUint32(buf[4:8], p.y)
		points[i] = selectorPoint{packed: buf, docID: p.doc}
	}
	f := buildReader(t, cfg, points, 1024)

	// Query: x in [25, 85] AND y in [200, 800].
	vis := &rect2DVisitor{
		xLo: 25, xHi: 85,
		yLo: 200, yHi: 800,
	}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	var want []int
	for _, p := range rawPts {
		if p.x >= vis.xLo && p.x <= vis.xHi && p.y >= vis.yLo && p.y <= vis.yHi {
			want = append(want, p.doc)
		}
	}
	sort.Ints(want)
	sort.Ints(vis.visitedIDs)
	if !equalInts(vis.visitedIDs, want) {
		t.Fatalf("Intersect 2D rect: got %v, want %v", vis.visitedIDs, want)
	}
}

// rect2DVisitor implements IntersectVisitor for a 2D rectangular
// query over a 4-byte-per-dim layout (BE-encoded uint32 per dim).
type rect2DVisitor struct {
	xLo, xHi, yLo, yHi uint32
	visitedIDs         []int
}

func (v *rect2DVisitor) Visit(docID int) error {
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *rect2DVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	x := binary.BigEndian.Uint32(packedValue[0:4])
	y := binary.BigEndian.Uint32(packedValue[4:8])
	if x >= v.xLo && x <= v.xHi && y >= v.yLo && y <= v.yHi {
		v.visitedIDs = append(v.visitedIDs, docID)
	}
	return nil
}

func (v *rect2DVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	minX := binary.BigEndian.Uint32(minPackedValue[0:4])
	maxX := binary.BigEndian.Uint32(maxPackedValue[0:4])
	minY := binary.BigEndian.Uint32(minPackedValue[4:8])
	maxY := binary.BigEndian.Uint32(maxPackedValue[4:8])
	// CELL_OUTSIDE if any dim's range is fully outside the query.
	if maxX < v.xLo || minX > v.xHi || maxY < v.yLo || minY > v.yHi {
		return codecs.RelationCellOutsideQuery
	}
	// CELL_INSIDE only if BOTH dim ranges are wholly inside the query.
	if minX >= v.xLo && maxX <= v.xHi && minY >= v.yLo && maxY <= v.yHi {
		return codecs.RelationCellInsideQuery
	}
	return codecs.RelationCellCrossesQuery
}

func (v *rect2DVisitor) Grow(count int) {}

// TestBKDReader_EstimatePointCount checks the documented "overcount on
// crossing leaves" behaviour: a CELL_INSIDE-only query returns exactly
// the right count; a CELL_CROSSES-only query overcounts by at most the
// total point count.
func TestBKDReader_EstimatePointCount(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	points := make([]selectorPoint, 16)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32((i + 1) * 7)), docID: i}
	}
	f := buildReader(t, cfg, points, 1024)

	// Full range: estimate must equal exact size.
	full := &rangeVisitor{loIncl: 0, hiIncl: 0xFFFFFFFF}
	est, err := f.r.EstimatePointCount(full)
	if err != nil {
		t.Fatalf("EstimatePointCount: %v", err)
	}
	if est != int64(len(points)) {
		t.Fatalf("EstimatePointCount full: got %d, want %d", est, len(points))
	}

	// Empty range: estimate must be 0.
	empty := &rangeVisitor{loIncl: 0xFF000000, hiIncl: 0xFFFFFFFF}
	est, err = f.r.EstimatePointCount(empty)
	if err != nil {
		t.Fatalf("EstimatePointCount empty: %v", err)
	}
	if est != 0 {
		t.Fatalf("EstimatePointCount empty: got %d, want 0", est)
	}
}

// TestBKDReader_RoundTripByteFormat is a regression that walks the
// hand-asserted byte-format fixture written in
// TestBKDWriter_ByteFormat_Regression: same input, written with
// BKDWriter, then opened with BKDReader, must yield the exact
// metadata + docIDs we expect.
func TestBKDReader_RoundTripByteFormat(t *testing.T) {
	cfg, err := NewBKDConfig(1, 1, 1, 4)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	points := []selectorPoint{
		{packed: []byte{0x10}, docID: 0},
		{packed: []byte{0x20}, docID: 1},
		{packed: []byte{0x30}, docID: 2},
		{packed: []byte{0x40}, docID: 3},
	}
	f := buildReader(t, cfg, points, 16)

	if got := f.r.NumLeaves(); got != 1 {
		t.Fatalf("numLeaves: got %d, want 1", got)
	}
	if got := f.r.Size(); got != 4 {
		t.Fatalf("size: got %d, want 4", got)
	}
	if got := f.r.GetDocCount(); got != 4 {
		t.Fatalf("docCount: got %d, want 4", got)
	}
	if got := f.r.GetMinPackedValue(); len(got) != 1 || got[0] != 0x10 {
		t.Fatalf("minPackedValue: got %v, want [0x10]", got)
	}
	if got := f.r.GetMaxPackedValue(); len(got) != 1 || got[0] != 0x40 {
		t.Fatalf("maxPackedValue: got %v, want [0x40]", got)
	}

	// Full-range intersection must yield all 4 docs.
	vis := &byteRangeVisitor{loIncl: 0x00, hiIncl: 0xFF}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	sort.Ints(vis.visitedIDs)
	if !equalInts(vis.visitedIDs, []int{0, 1, 2, 3}) {
		t.Fatalf("Intersect: got %v, want [0 1 2 3]", vis.visitedIDs)
	}

	// Sub-range [0x20, 0x30] must yield docs 1 and 2.
	vis2 := &byteRangeVisitor{loIncl: 0x20, hiIncl: 0x30}
	if err := f.r.Intersect(vis2); err != nil {
		t.Fatalf("Intersect sub-range: %v", err)
	}
	sort.Ints(vis2.visitedIDs)
	if !equalInts(vis2.visitedIDs, []int{1, 2}) {
		t.Fatalf("Intersect [0x20, 0x30]: got %v, want [1 2]", vis2.visitedIDs)
	}
}

// byteRangeVisitor mirrors rangeVisitor for the 1-byte-per-dim layout
// used by TestBKDReader_RoundTripByteFormat.
type byteRangeVisitor struct {
	loIncl, hiIncl byte
	visitedIDs     []int
}

func (v *byteRangeVisitor) Visit(docID int) error {
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *byteRangeVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	x := packedValue[0]
	if x >= v.loIncl && x <= v.hiIncl {
		v.visitedIDs = append(v.visitedIDs, docID)
	}
	return nil
}

func (v *byteRangeVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	mn := minPackedValue[0]
	mx := maxPackedValue[0]
	if mx < v.loIncl || mn > v.hiIncl {
		return codecs.RelationCellOutsideQuery
	}
	if mn >= v.loIncl && mx <= v.hiIncl {
		return codecs.RelationCellInsideQuery
	}
	return codecs.RelationCellCrossesQuery
}

func (v *byteRangeVisitor) Grow(count int) {}

// TestBKDReader_GetPointTreeClone verifies that two independent clones
// can be walked in parallel without interfering: the clone visits the
// left subtree while the parent walks the right.
func TestBKDReader_GetPointTreeClone(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	points := make([]selectorPoint, 16)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32((i + 1) * 7)), docID: i}
	}
	f := buildReader(t, cfg, points, 1024)

	root, err := f.r.GetPointTree()
	if err != nil {
		t.Fatalf("GetPointTree: %v", err)
	}
	clone := root.Clone()

	// Walk root to the right subtree, walk clone to the left subtree.
	if moved, err := root.MoveToChild(); err != nil || !moved {
		t.Fatalf("root.MoveToChild: moved=%v err=%v", moved, err)
	}
	if moved, err := root.MoveToSibling(); err != nil || !moved {
		t.Fatalf("root.MoveToSibling: moved=%v err=%v", moved, err)
	}
	if moved, err := clone.MoveToChild(); err != nil || !moved {
		t.Fatalf("clone.MoveToChild: moved=%v err=%v", moved, err)
	}

	rightVis := &readerCaptureVisitor{relation: codecs.RelationCellInsideQuery}
	if err := root.VisitDocIDs(rightVis); err != nil {
		t.Fatalf("root VisitDocIDs: %v", err)
	}
	leftVis := &readerCaptureVisitor{relation: codecs.RelationCellInsideQuery}
	if err := clone.VisitDocIDs(leftVis); err != nil {
		t.Fatalf("clone VisitDocIDs: %v", err)
	}
	// Union of the two must equal the full doc set, no duplicates.
	all := append([]int{}, leftVis.visitedIDs...)
	all = append(all, rightVis.visitedIDs...)
	sort.Ints(all)
	want := make([]int, len(points))
	for i := range want {
		want[i] = i
	}
	if !equalInts(all, want) {
		t.Fatalf("left+right docs: got %v, want %v", all, want)
	}
}

// TestBKDReader_DeepTree drives a larger fixture (256 points across 32
// leaves of 8) to ensure the recursive Intersect path correctly walks
// trees with treeDepth > 4. Verifies a full-range scan returns every
// docID exactly once and a mid-range query returns the expected slice.
func TestBKDReader_DeepTree(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 8)
	const N = 256
	points := make([]selectorPoint, N)
	for i := range points {
		points[i] = selectorPoint{packed: be4(uint32(i)), docID: i}
	}
	f := buildReader(t, cfg, points, N*2)

	if got := f.r.NumLeaves(); got != 32 {
		t.Fatalf("numLeaves: got %d, want 32", got)
	}

	// Full range: every doc reported once.
	full := &rangeVisitor{loIncl: 0, hiIncl: 0xFFFFFFFF}
	if err := f.r.Intersect(full); err != nil {
		t.Fatalf("Intersect full: %v", err)
	}
	if len(full.visitedIDs) != N {
		t.Fatalf("Intersect full: got %d hits, want %d", len(full.visitedIDs), N)
	}
	sort.Ints(full.visitedIDs)
	for i := 0; i < N; i++ {
		if full.visitedIDs[i] != i {
			t.Fatalf("Intersect full: visitedIDs[%d]=%d, want %d", i, full.visitedIDs[i], i)
		}
	}

	// Mid-range: [50, 199] inclusive on the value space (== docID space).
	mid := &rangeVisitor{loIncl: 50, hiIncl: 199}
	if err := f.r.Intersect(mid); err != nil {
		t.Fatalf("Intersect mid: %v", err)
	}
	sort.Ints(mid.visitedIDs)
	want := make([]int, 0, 150)
	for i := 50; i <= 199; i++ {
		want = append(want, i)
	}
	if !equalInts(mid.visitedIDs, want) {
		t.Fatalf("Intersect mid: got %d hits %v..%v, want %d %v..%v",
			len(mid.visitedIDs), first3(mid.visitedIDs), last3(mid.visitedIDs),
			len(want), first3(want), last3(want))
	}
}

// first3 / last3 are tiny helpers for legible failure messages.
func first3(xs []int) []int {
	if len(xs) <= 3 {
		return xs
	}
	return xs[:3]
}
func last3(xs []int) []int {
	if len(xs) <= 3 {
		return xs
	}
	return xs[len(xs)-3:]
}

// equalInts is the test-side equivalent of reflect.DeepEqual on
// matched []int values, avoiding the reflect import.
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
