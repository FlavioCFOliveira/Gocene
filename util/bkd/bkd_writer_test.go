// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file is the behavioural test suite for BKDWriter. There is no
// direct Java TestBKDWriter peer in Lucene 10.4.0; the indirect peers
// live under lucene/core/src/test/org/apache/lucene/index/TestBKD*.java.
// We translate the structural properties exercised by those tests into
// Go-friendly black-box checks that drive the writer end-to-end and
// then re-parse the emitted bytes manually (BKDReader is not ported
// yet at the time of writing).

// runBuild drives Add() + Finish() for the supplied configuration and
// points, writing meta/index/data into a single output file. The
// returned bytes are read back as a slice for downstream parsing.
type buildResult struct {
	dir      *store.ByteBuffersDirectory
	metaName string
	dataName string
	metaLen  int64
	dataLen  int64
	cfg      BKDConfig
	writer   *BKDWriter
}

// buildPointsAdd creates a writer, calls Add for each point, then
// Finish using two separate IndexOutputs (one for meta+index, one for
// data). The two outputs are kept distinct so we can read them back
// without needing to know the meta-vs-data boundary.
func buildPointsAdd(
	t *testing.T,
	cfg BKDConfig,
	maxDoc int,
	maxMBSortInHeap float64,
	points []selectorPoint,
) *buildResult {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	w, err := NewBKDWriter(maxDoc, dir, "test", cfg, maxMBSortInHeap, int64(len(points)))
	if err != nil {
		t.Fatalf("NewBKDWriter: %v", err)
	}

	for _, p := range points {
		if err := w.Add(p.packed, p.docID); err != nil {
			t.Fatalf("Add(%x, %d): %v", p.packed, p.docID, err)
		}
	}

	metaName := "bkd.meta"
	dataName := "bkd.data"
	metaOut, err := dir.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	dataOut, err := dir.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}

	runnable, err := w.Finish(metaOut, metaOut, dataOut)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if runnable != nil {
		if err := runnable(); err != nil {
			t.Fatalf("Finish runnable: %v", err)
		}
	}

	metaLen := metaOut.GetFilePointer()
	dataLen := dataOut.GetFilePointer()
	if err := metaOut.Close(); err != nil {
		t.Fatalf("Close metaOut: %v", err)
	}
	if err := dataOut.Close(); err != nil {
		t.Fatalf("Close dataOut: %v", err)
	}

	return &buildResult{
		dir:      dir,
		metaName: metaName,
		dataName: dataName,
		metaLen:  metaLen,
		dataLen:  dataLen,
		cfg:      cfg,
		writer:   w,
	}
}

// readBytesAll reads the full content of file `name` from `dir`.
func readBytesAll(t *testing.T, dir *store.ByteBuffersDirectory, name string) []byte {
	t.Helper()
	in, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", name, err)
	}
	defer in.Close()
	out := make([]byte, in.Length())
	if err := in.ReadBytes(out); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadBytes %q: %v", name, err)
	}
	return out
}

// metaHeaderView captures the fixed-shape parsed meta header. Mirrors
// the prefix of BKDWriter.writeIndex output up to (but excluding) the
// packedIndex bytes.
type metaHeaderView struct {
	numDims             int
	numIndexDims        int
	maxPointsInLeafNode int
	bytesPerDim         int
	numLeaves           int
	minPackedValue      []byte
	maxPackedValue      []byte
	pointCount          int64
	docsSeenCardinality int
	packedIndexBytesLen int
	dataStartFP         int64
	indexStartFP        int64
}

// parseMetaHeader parses the codec header + the BKDWriter meta-header
// fields out of `meta`. Returns the parsed view and the file pointer
// position immediately after the indexStartFP long (where the packed
// index begins in the combined meta/index file).
func parseMetaHeader(t *testing.T, meta []byte, codecName string, version int32) (metaHeaderView, int) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput("scratch", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(meta); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	in, err := dir.OpenInput("scratch", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	if _, err := codecs.CheckHeader(in, codecName, version, version); err != nil {
		t.Fatalf("CheckHeader: %v", err)
	}

	mhv := metaHeaderView{}
	mhv.numDims = int(mustReadVInt(t, in))
	mhv.numIndexDims = int(mustReadVInt(t, in))
	mhv.maxPointsInLeafNode = int(mustReadVInt(t, in))
	mhv.bytesPerDim = int(mustReadVInt(t, in))
	mhv.numLeaves = int(mustReadVInt(t, in))

	pibl := mhv.numIndexDims * mhv.bytesPerDim
	mhv.minPackedValue = make([]byte, pibl)
	if err := in.ReadBytes(mhv.minPackedValue); err != nil {
		t.Fatalf("ReadBytes minPackedValue: %v", err)
	}
	mhv.maxPackedValue = make([]byte, pibl)
	if err := in.ReadBytes(mhv.maxPackedValue); err != nil {
		t.Fatalf("ReadBytes maxPackedValue: %v", err)
	}
	mhv.pointCount = mustReadVLong(t, in)
	mhv.docsSeenCardinality = int(mustReadVInt(t, in))
	mhv.packedIndexBytesLen = int(mustReadVInt(t, in))
	dataFP, err := store.ReadInt64(in)
	if err != nil {
		t.Fatalf("ReadInt64 dataStartFP: %v", err)
	}
	mhv.dataStartFP = dataFP
	indexFP, err := store.ReadInt64(in)
	if err != nil {
		t.Fatalf("ReadInt64 indexStartFP: %v", err)
	}
	mhv.indexStartFP = indexFP

	return mhv, int(in.GetFilePointer())
}

func mustReadVInt(t *testing.T, in store.IndexInput) int32 {
	t.Helper()
	v, err := store.ReadVInt(in)
	if err != nil {
		t.Fatalf("ReadVInt: %v", err)
	}
	return v
}

func mustReadVLong(t *testing.T, in store.IndexInput) int64 {
	t.Helper()
	v, err := store.ReadVLong(in)
	if err != nil {
		t.Fatalf("ReadVLong: %v", err)
	}
	return v
}

// be4 packs an unsigned 32-bit value into a big-endian 4-byte slice.
func be4(v uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, v)
	return out
}

// expectedNumLeaves returns ceil(n/leafSize), the leaf count produced
// by BKDWriter for `n` points and `leafSize` per leaf.
func expectedNumLeaves(n, leafSize int) int { return (n + leafSize - 1) / leafSize }

// TestBKDWriter_SingleDim_FourBytes_Heap exercises the smallest happy
// path: a handful of 4-byte 1-dim points, written entirely on heap.
func TestBKDWriter_SingleDim_FourBytes_Heap(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4) // tiny leaf size to force >1 leaf
	points := []selectorPoint{
		{packed: be4(10), docID: 0},
		{packed: be4(20), docID: 1},
		{packed: be4(30), docID: 2},
		{packed: be4(40), docID: 3},
		{packed: be4(50), docID: 4},
		{packed: be4(60), docID: 5},
		{packed: be4(70), docID: 6},
		{packed: be4(80), docID: 7},
		{packed: be4(90), docID: 8},
	}
	br := buildPointsAdd(t, cfg, 1024, 4.0, points)
	meta := readBytesAll(t, br.dir, br.metaName)
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(br.writer.Version()))

	if got, want := mhv.numDims, cfg.NumDims(); got != want {
		t.Fatalf("numDims: got %d, want %d", got, want)
	}
	if got, want := mhv.numIndexDims, cfg.NumIndexDims(); got != want {
		t.Fatalf("numIndexDims: got %d, want %d", got, want)
	}
	if got, want := mhv.bytesPerDim, cfg.BytesPerDim(); got != want {
		t.Fatalf("bytesPerDim: got %d, want %d", got, want)
	}
	if got, want := mhv.maxPointsInLeafNode, cfg.MaxPointsInLeafNode(); got != want {
		t.Fatalf("maxPointsInLeaf: got %d, want %d", got, want)
	}
	if got, want := mhv.numLeaves, expectedNumLeaves(len(points), cfg.MaxPointsInLeafNode()); got != want {
		t.Fatalf("numLeaves: got %d, want %d", got, want)
	}
	if got, want := mhv.pointCount, int64(len(points)); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
	if got, want := mhv.docsSeenCardinality, len(points); got != want {
		t.Fatalf("docsSeenCardinality: got %d, want %d", got, want)
	}
	if got := mhv.minPackedValue; len(got) == 4 && uint32FromBE(got) != 10 {
		t.Fatalf("minPackedValue: got %v, want BE(10)", got)
	}
	if got := mhv.maxPackedValue; len(got) == 4 && uint32FromBE(got) != 90 {
		t.Fatalf("maxPackedValue: got %v, want BE(90)", got)
	}
	if mhv.packedIndexBytesLen <= 0 {
		t.Fatalf("packedIndexBytesLen must be > 0, got %d", mhv.packedIndexBytesLen)
	}
	if mhv.dataStartFP != 0 {
		t.Fatalf("dataStartFP: got %d, want 0", mhv.dataStartFP)
	}
}

// TestBKDWriter_TwoDims_FourBytes_Heap exercises the multi-dim path:
// two 4-byte indexed dimensions, on-heap build, small leaf size.
func TestBKDWriter_TwoDims_FourBytes_Heap(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 4, 4)
	var points []selectorPoint
	for i := 0; i < 10; i++ {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint32(buf[0:4], uint32(i*7+1))
		binary.BigEndian.PutUint32(buf[4:8], uint32(i*3+11))
		points = append(points, selectorPoint{packed: buf, docID: i})
	}

	br := buildPointsAdd(t, cfg, 1024, 4.0, points)
	meta := readBytesAll(t, br.dir, br.metaName)
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(br.writer.Version()))

	if got, want := mhv.numDims, 2; got != want {
		t.Fatalf("numDims: got %d, want %d", got, want)
	}
	if got, want := mhv.numIndexDims, 2; got != want {
		t.Fatalf("numIndexDims: got %d, want %d", got, want)
	}
	if got, want := mhv.numLeaves, expectedNumLeaves(len(points), cfg.MaxPointsInLeafNode()); got != want {
		t.Fatalf("numLeaves: got %d, want %d", got, want)
	}
	if got, want := mhv.pointCount, int64(len(points)); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
	if mhv.packedIndexBytesLen <= 0 {
		t.Fatalf("packedIndexBytesLen must be > 0, got %d", mhv.packedIndexBytesLen)
	}

	// Check the min/max packed values match the per-dim min/max of
	// the points list.
	var minD0, minD1, maxD0, maxD1 uint32
	for i, p := range points {
		d0 := binary.BigEndian.Uint32(p.packed[0:4])
		d1 := binary.BigEndian.Uint32(p.packed[4:8])
		if i == 0 {
			minD0, maxD0, minD1, maxD1 = d0, d0, d1, d1
		} else {
			if d0 < minD0 {
				minD0 = d0
			}
			if d0 > maxD0 {
				maxD0 = d0
			}
			if d1 < minD1 {
				minD1 = d1
			}
			if d1 > maxD1 {
				maxD1 = d1
			}
		}
	}
	if got := binary.BigEndian.Uint32(mhv.minPackedValue[0:4]); got != minD0 {
		t.Errorf("min dim0: got %d, want %d", got, minD0)
	}
	if got := binary.BigEndian.Uint32(mhv.minPackedValue[4:8]); got != minD1 {
		t.Errorf("min dim1: got %d, want %d", got, minD1)
	}
	if got := binary.BigEndian.Uint32(mhv.maxPackedValue[0:4]); got != maxD0 {
		t.Errorf("max dim0: got %d, want %d", got, maxD0)
	}
	if got := binary.BigEndian.Uint32(mhv.maxPackedValue[4:8]); got != maxD1 {
		t.Errorf("max dim1: got %d, want %d", got, maxD1)
	}
}

// TestBKDWriter_OfflineSpill drives a build whose point count exceeds
// the heap budget, forcing the writer to spill through the offline
// pipeline.
func TestBKDWriter_OfflineSpill(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 8)
	// Tiny heap budget: 4 points worth of heap allowance forces the
	// initial pointWriter onto disk.
	maxMB := float64(cfg.BytesPerDoc()*4) / (1024.0 * 1024.0)
	// But we need maxPointsSortInHeap >= MaxPointsInLeafNode. Reduce
	// MaxPointsInLeafNode and grow the heap accordingly so the heap
	// budget covers the leaf size but is still well below totalPoints.
	cfg = mustConfig(t, 1, 1, 4, 4)
	maxMB = float64(cfg.BytesPerDoc()*4) / (1024.0 * 1024.0)
	if int(maxMB*1024.0*1024.0)/cfg.BytesPerDoc() < cfg.MaxPointsInLeafNode() {
		// Bump until the constructor accepts it.
		maxMB = float64(cfg.BytesPerDoc()*cfg.MaxPointsInLeafNode()) / (1024.0 * 1024.0)
	}

	const total = 64
	var points []selectorPoint
	for i := 0; i < total; i++ {
		points = append(points, selectorPoint{packed: be4(uint32(total - i)), docID: i})
	}

	br := buildPointsAdd(t, cfg, 1024, maxMB, points)
	meta := readBytesAll(t, br.dir, br.metaName)
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(br.writer.Version()))

	if got, want := mhv.pointCount, int64(total); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
	if got, want := mhv.numLeaves, expectedNumLeaves(total, cfg.MaxPointsInLeafNode()); got != want {
		t.Fatalf("numLeaves: got %d, want %d", got, want)
	}
	if got := binary.BigEndian.Uint32(mhv.minPackedValue); got != 1 {
		t.Errorf("min: got %d, want 1", got)
	}
	if got := binary.BigEndian.Uint32(mhv.maxPackedValue); got != uint32(total) {
		t.Errorf("max: got %d, want %d", got, total)
	}

	// After Finish, no temp files should remain in the directory.
	files, err := br.dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, name := range files {
		if name != br.metaName && name != br.dataName {
			t.Errorf("unexpected residual file in tempDir: %q", name)
		}
	}
}

// TestBKDWriter_SingleValueRange covers the all-equal cell: every
// point shares the same packed value but maps to a distinct docID.
func TestBKDWriter_SingleValueRange(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	const n = 6
	var points []selectorPoint
	for i := 0; i < n; i++ {
		points = append(points, selectorPoint{packed: be4(42), docID: i})
	}

	br := buildPointsAdd(t, cfg, 16, 4.0, points)
	meta := readBytesAll(t, br.dir, br.metaName)
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(br.writer.Version()))

	if got, want := mhv.pointCount, int64(n); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
	if binary.BigEndian.Uint32(mhv.minPackedValue) != 42 {
		t.Errorf("min: got %v, want BE(42)", mhv.minPackedValue)
	}
	if binary.BigEndian.Uint32(mhv.maxPackedValue) != 42 {
		t.Errorf("max: got %v, want BE(42)", mhv.maxPackedValue)
	}
}

// TestBKDWriter_EmptyInput verifies that Finish on a writer that
// received zero Add calls returns a nil IORunnable, exactly mirroring
// Lucene's behaviour.
func TestBKDWriter_EmptyInput(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, DefaultMaxPointsInLeafNode)
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := NewBKDWriter(16, dir, "test", cfg, 4.0, 0)
	if err != nil {
		t.Fatalf("NewBKDWriter: %v", err)
	}
	metaOut, err := dir.CreateOutput("meta", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	dataOut, err := dir.CreateOutput("data", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}
	defer metaOut.Close()
	defer dataOut.Close()

	r, err := w.Finish(metaOut, metaOut, dataOut)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if r != nil {
		t.Fatalf("Finish on empty input should return nil runnable; got non-nil")
	}
	if metaOut.GetFilePointer() != 0 {
		t.Errorf("metaOut should be empty; got %d bytes", metaOut.GetFilePointer())
	}
	if dataOut.GetFilePointer() != 0 {
		t.Errorf("dataOut should be empty; got %d bytes", dataOut.GetFilePointer())
	}
}

// TestBKDWriter_WriteField_OneDim drives the WriteField fast path with
// a MutablePointTree backed by an in-memory slab.
func TestBKDWriter_WriteField_OneDim(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 4)
	const n = 10
	tree := newSimpleMutableTree(cfg, n)
	for i := 0; i < n; i++ {
		tree.setPoint(i, be4(uint32(n-i)), n-i-1)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := NewBKDWriter(64, dir, "test", cfg, 4.0, int64(n))
	if err != nil {
		t.Fatalf("NewBKDWriter: %v", err)
	}
	metaOut, err := dir.CreateOutput("meta", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	dataOut, err := dir.CreateOutput("data", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}

	r, err := w.WriteField(metaOut, metaOut, dataOut, "f", tree, n)
	if err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if r == nil {
		t.Fatalf("WriteField returned nil runnable for non-empty input")
	}
	if err := r(); err != nil {
		t.Fatalf("runnable: %v", err)
	}
	_ = metaOut.Close()
	_ = dataOut.Close()

	meta := readBytesAll(t, dir, "meta")
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(w.Version()))
	if got, want := mhv.pointCount, int64(n); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
	if got, want := mhv.numLeaves, expectedNumLeaves(n, cfg.MaxPointsInLeafNode()); got != want {
		t.Fatalf("numLeaves: got %d, want %d", got, want)
	}
	if got := binary.BigEndian.Uint32(mhv.minPackedValue); got != 1 {
		t.Errorf("min: got %d, want 1", got)
	}
	if got := binary.BigEndian.Uint32(mhv.maxPackedValue); got != uint32(n) {
		t.Errorf("max: got %d, want %d", got, n)
	}
}

// TestBKDWriter_WriteField_TwoDims exercises the writeFieldNDims path
// driving the recursive MutablePointTree build.
func TestBKDWriter_WriteField_TwoDims(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 4, 4)
	const n = 12
	tree := newSimpleMutableTree(cfg, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint32(buf[0:4], uint32(i*5+1))
		binary.BigEndian.PutUint32(buf[4:8], uint32((n-i)*3+7))
		tree.setPoint(i, buf, i)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := NewBKDWriter(64, dir, "test", cfg, 4.0, int64(n))
	if err != nil {
		t.Fatalf("NewBKDWriter: %v", err)
	}
	metaOut, err := dir.CreateOutput("meta", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	dataOut, err := dir.CreateOutput("data", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}

	r, err := w.WriteField(metaOut, metaOut, dataOut, "f", tree, n)
	if err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if r == nil {
		t.Fatalf("WriteField returned nil runnable")
	}
	if err := r(); err != nil {
		t.Fatalf("runnable: %v", err)
	}
	_ = metaOut.Close()
	_ = dataOut.Close()

	meta := readBytesAll(t, dir, "meta")
	mhv, _ := parseMetaHeader(t, meta, BKDCodecName, int32(w.Version()))
	if got, want := mhv.pointCount, int64(n); got != want {
		t.Fatalf("pointCount: got %d, want %d", got, want)
	}
}

// TestBKDWriter_ByteFormat_Regression is the deterministic fixture
// test. It exercises a known-shape build (4 single-byte 1-dim points,
// leaf size 4 so there is exactly one leaf) and asserts the byte
// layout of the meta and data files prefix-by-prefix.
//
// The fixture is self-generated: there is no JVM available in this
// environment to cross-check against Lucene's reference output. The
// downstream sprint that ports BKDReader will read these bytes back
// through the reader, providing the closing-the-loop validation.
func TestBKDWriter_ByteFormat_Regression(t *testing.T) {
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
	br := buildPointsAdd(t, cfg, 16, 4.0, points)
	meta := readBytesAll(t, br.dir, br.metaName)
	data := readBytesAll(t, br.dir, br.dataName)

	mhv, postHeaderPos := parseMetaHeader(t, meta, BKDCodecName, int32(br.writer.Version()))

	// Single leaf: numLeaves == 1, no split values stored.
	if mhv.numLeaves != 1 {
		t.Fatalf("numLeaves: got %d, want 1", mhv.numLeaves)
	}
	if mhv.pointCount != 4 {
		t.Fatalf("pointCount: got %d, want 4", mhv.pointCount)
	}
	if mhv.docsSeenCardinality != 4 {
		t.Fatalf("docsSeenCardinality: got %d, want 4", mhv.docsSeenCardinality)
	}
	if got, want := mhv.minPackedValue, []byte{0x10}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("minPackedValue: got %v, want %v", got, want)
	}
	if got, want := mhv.maxPackedValue, []byte{0x40}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("maxPackedValue: got %v, want %v", got, want)
	}

	// dataStartFP must be 0 (single leaf starts at the beginning of dataOut).
	if mhv.dataStartFP != 0 {
		t.Fatalf("dataStartFP: got %d, want 0", mhv.dataStartFP)
	}

	// The single-leaf data stream layout for this fixture (all distinct
	// values, all distinct docIDs in [0..3] -> continuous run, 4
	// entries, 1-byte values) is:
	//   - VInt count = 4                                (1 byte: 0x04)
	//   - byte 0xFE                                      (continuous-ids marker)
	//   - VInt 0                                          (start docID)
	//   - VInt commonPrefix[0] = 0                       (no shared prefix on dim 0)
	//   - leaf packed values payload (one byte marker + bytes)
	// We assert the first three bytes byte-for-byte.
	if br.dataLen == 0 {
		t.Fatalf("dataLen must be > 0")
	}
	// Full byte-exact data layout for the fixture:
	//   04           VInt count=4
	//   fe           docID marker: continuous run
	//   00           VInt start docID=0
	//   00           VInt commonPrefix[0]=0 (no shared prefix on dim 0)
	//   00           sortedDim marker (high-cardinality path)
	//   10 01        prefixByte=0x10, runLen=1
	//   20 01        prefixByte=0x20, runLen=1
	//   30 01        prefixByte=0x30, runLen=1
	//   40 01        prefixByte=0x40, runLen=1
	expectedData := []byte{0x04, 0xFE, 0x00, 0x00, 0x00, 0x10, 0x01, 0x20, 0x01, 0x30, 0x01, 0x40, 0x01}
	if len(data) != len(expectedData) {
		t.Fatalf("data length: got %d, want %d", len(data), len(expectedData))
	}
	for i := range expectedData {
		if data[i] != expectedData[i] {
			t.Fatalf("data[%d]: got 0x%02x, want 0x%02x (full data=%x)", i, data[i], expectedData[i], data)
		}
	}

	// Parse the leaf header out of `data` (a second time, end-to-end).
	readBack := make([]byte, br.dataLen)
	copy(readBack, data[:br.dataLen])
	di := newSliceReader(readBack)
	count, err := di.readVInt()
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if count != 4 {
		t.Fatalf("leaf count: got %d, want 4", count)
	}

	// Verify the index trailing portion of the meta file is exactly
	// packedIndexBytesLen bytes after the metaHeader.
	expectedTotal := postHeaderPos + mhv.packedIndexBytesLen
	if int64(expectedTotal) != br.metaLen {
		t.Fatalf("meta total: got %d, want %d (postHeader=%d, indexBytes=%d)",
			br.metaLen, expectedTotal, postHeaderPos, mhv.packedIndexBytesLen)
	}

	// For a single-leaf tree the packed index is a single VLong delta
	// equal to dataStartFP - minBlockFP == 0 - 0 == 0, written as one
	// VLong byte 0x00.
	if mhv.packedIndexBytesLen != 1 {
		t.Fatalf("single-leaf packedIndexBytesLen: got %d, want 1", mhv.packedIndexBytesLen)
	}
	packedIdx := meta[postHeaderPos]
	if packedIdx != 0x00 {
		t.Fatalf("single-leaf packed index byte: got 0x%02x, want 0x00", packedIdx)
	}
}

// uint32FromBE decodes a big-endian uint32 from a 4-byte slice.
func uint32FromBE(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}

// mustConfig from bkd_radix_selector_test is reused; it is package-level.

// ---------------------------------------------------------------------
// Test fixtures: a minimal MutablePointTree backed by a flat byte slab.
// ---------------------------------------------------------------------

// simpleMutableTree is a black-box mutable point tree used by the
// WriteField tests. It mirrors the in-memory layout of Java's
// MutablePointsReaderUtils test helpers (heap-backed BytesRef slots
// indexed by an external docID array).
type simpleMutableTree struct {
	cfg         BKDConfig
	packed      []byte
	scratch     []byte
	docIDs      []int
	count       int
	savedPacked []byte // populated lazily by Save
	savedDocs   []int  // populated lazily by Save
}

func newSimpleMutableTree(cfg BKDConfig, n int) *simpleMutableTree {
	return &simpleMutableTree{
		cfg:     cfg,
		packed:  make([]byte, n*cfg.PackedBytesLength()),
		scratch: make([]byte, cfg.PackedBytesLength()),
		docIDs:  make([]int, n),
		count:   n,
	}
}

func (t *simpleMutableTree) setPoint(i int, packed []byte, docID int) {
	start := i * t.cfg.PackedBytesLength()
	copy(t.packed[start:start+t.cfg.PackedBytesLength()], packed)
	t.docIDs[i] = docID
}

func (t *simpleMutableTree) Swap(i, j int) {
	start1 := i * t.cfg.PackedBytesLength()
	start2 := j * t.cfg.PackedBytesLength()
	copy(t.scratch, t.packed[start1:start1+t.cfg.PackedBytesLength()])
	copy(t.packed[start1:start1+t.cfg.PackedBytesLength()],
		t.packed[start2:start2+t.cfg.PackedBytesLength()])
	copy(t.packed[start2:start2+t.cfg.PackedBytesLength()], t.scratch)
	t.docIDs[i], t.docIDs[j] = t.docIDs[j], t.docIDs[i]
}

func (t *simpleMutableTree) GetValue(i int, dst *util.BytesRef) {
	dst.Bytes = t.packed
	dst.Offset = i * t.cfg.PackedBytesLength()
	dst.Length = t.cfg.PackedBytesLength()
}

func (t *simpleMutableTree) GetByteAt(i, k int) byte {
	return t.packed[i*t.cfg.PackedBytesLength()+k]
}

func (t *simpleMutableTree) GetDocID(i int) int { return t.docIDs[i] }

// Save and Restore implement the StableMSBRadixSorterImpl scratch
// contract: the implementation needs to be able to stash a value at
// slot i into scratch position j and restore [i, j) from scratch
// later. We back the scratch with a parallel slab.
func (t *simpleMutableTree) Save(i, j int) {
	if t.savedPacked == nil {
		t.savedPacked = make([]byte, t.count*t.cfg.PackedBytesLength())
		t.savedDocs = make([]int, t.count)
	}
	srcOffset := i * t.cfg.PackedBytesLength()
	dstOffset := j * t.cfg.PackedBytesLength()
	if dstOffset+t.cfg.PackedBytesLength() > len(t.savedPacked) {
		next := make([]byte, dstOffset+t.cfg.PackedBytesLength())
		copy(next, t.savedPacked)
		t.savedPacked = next
	}
	if j+1 > len(t.savedDocs) {
		next := make([]int, j+1)
		copy(next, t.savedDocs)
		t.savedDocs = next
	}
	copy(t.savedPacked[dstOffset:dstOffset+t.cfg.PackedBytesLength()],
		t.packed[srcOffset:srcOffset+t.cfg.PackedBytesLength()])
	t.savedDocs[j] = t.docIDs[i]
}

func (t *simpleMutableTree) Restore(i, j int) {
	for k := i; k < j; k++ {
		srcOffset := k * t.cfg.PackedBytesLength()
		copy(t.packed[srcOffset:srcOffset+t.cfg.PackedBytesLength()],
			t.savedPacked[srcOffset:srcOffset+t.cfg.PackedBytesLength()])
		t.docIDs[k] = t.savedDocs[k]
	}
}

// ---------------------------------------------------------------------
// Inline slice reader (no public DataInput needed for tests).
// ---------------------------------------------------------------------

type sliceReader struct {
	buf []byte
	pos int
}

func newSliceReader(b []byte) *sliceReader { return &sliceReader{buf: b} }

func (s *sliceReader) readByte() (byte, error) {
	if s.pos >= len(s.buf) {
		return 0, io.EOF
	}
	b := s.buf[s.pos]
	s.pos++
	return b, nil
}

func (s *sliceReader) readVInt() (int32, error) {
	b, err := s.readByte()
	if err != nil {
		return 0, err
	}
	if b < 0x80 {
		return int32(b), nil
	}
	var i int32 = int32(b) & 0x7F
	shift := uint(7)
	for {
		b, err = s.readByte()
		if err != nil {
			return 0, err
		}
		i |= int32(b&0x7F) << shift
		if b < 0x80 {
			break
		}
		shift += 7
		if shift >= 35 {
			return 0, fmt.Errorf("vint overflow")
		}
	}
	return i, nil
}
