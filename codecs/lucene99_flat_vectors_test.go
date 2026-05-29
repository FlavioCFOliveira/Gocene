// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// vectorFieldInfoFloat builds a FLOAT32 vector FieldInfo.
func vectorFieldInfoFloat(name string, number, dim int, sim index.VectorSimilarityFunction) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: sim,
	})
}

// vectorFieldInfoByte builds a BYTE vector FieldInfo.
func vectorFieldInfoByte(name string, number, dim int, sim index.VectorSimilarityFunction) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingByte,
		VectorSimilarityFunction: sim,
	})
}

// TestLucene99FlatVectors_DenseFloatRoundTrip writes a float vector to
// every document of a dense segment through the HNSW writer (which now
// composes the flat writer), then reads the vectors back byte-exactly and
// verifies nearest-neighbour search returns the closest documents.
func TestLucene99FlatVectors_DenseFloatRoundTrip(t *testing.T) {
	const (
		field  = "vec"
		dim    = 3
		maxDoc = 6
	)
	vectors := [][]float32{
		{0, 0, 0},
		{1, 0, 0},
		{2, 0, 0},
		{10, 10, 10},
		{0, 5, 0},
		{0, 0, 7},
	}

	fis := index.NewFieldInfos()
	if err := fis.Add(vectorFieldInfoFloat(field, 0, dim, index.VectorSimilarityFunctionEuclidean)); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	if err := si.SetID(seqID()); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}

	// --- write ---
	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 0, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}
	fw, err := w.AddField(fis.GetByName(field))
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for doc, v := range vectors {
		if err := fw.AddValue(doc, v); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, err)
		}
	}
	if err := w.Flush(maxDoc, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- read ---
	rs := &SegmentReadState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsReader: %v", err)
	}
	defer r.Close()

	if err := r.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	// (a) byte-exact vectors.
	fvv, err := r.GetFloatVectorValues(field)
	if err != nil {
		t.Fatalf("GetFloatVectorValues: %v", err)
	}
	if fvv.Size() != maxDoc {
		t.Fatalf("Size = %d, want %d", fvv.Size(), maxDoc)
	}
	if fvv.Dimension() != dim {
		t.Fatalf("Dimension = %d, want %d", fvv.Dimension(), dim)
	}
	for doc := range vectors {
		got, err := fvv.GetVector(doc)
		if err != nil {
			t.Fatalf("GetVector(%d): %v", doc, err)
		}
		if !float32SlicesEqual(got, vectors[doc]) {
			t.Errorf("GetVector(%d) = %v, want %v", doc, got, vectors[doc])
		}
	}

	// (b) nearest-neighbour search. Query near doc 3 ({10,10,10}); the
	// nearest stored vector is doc 3 itself.
	td, err := r.SearchNearestFloat(field, []float32{9, 9, 9}, 3, nil)
	if err != nil {
		t.Fatalf("SearchNearestFloat: %v", err)
	}
	if len(td.ScoreDocs) == 0 {
		t.Fatalf("SearchNearestFloat returned no results")
	}
	if td.ScoreDocs[0].Doc != 3 {
		t.Errorf("nearest doc = %d, want 3 (scoreDocs=%v)", td.ScoreDocs[0].Doc, scoreDocsString(td))
	}

	// Query at exactly doc 0 ({0,0,0}); the nearest is doc 0.
	td, err = r.SearchNearestFloat(field, []float32{0, 0, 0}, 2, nil)
	if err != nil {
		t.Fatalf("SearchNearestFloat: %v", err)
	}
	if td.ScoreDocs[0].Doc != 0 {
		t.Errorf("nearest to origin = %d, want 0 (scoreDocs=%v)", td.ScoreDocs[0].Doc, scoreDocsString(td))
	}

	// (c) acceptDocs filter: exclude doc 3 (the true nearest to {9,9,9}).
	// The remaining vectors, ranked by Euclidean distance to {9,9,9}, are
	// doc 5 {0,0,7} (sq dist 166) < doc 4 {0,5,0} (178) < doc 2 {2,0,0}
	// (211), so doc 5 must rank first and doc 3 must never appear.
	accept, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for d := 0; d < maxDoc; d++ {
		if d != 3 {
			accept.Set(d)
		}
	}
	td, err = r.SearchNearestFloat(field, []float32{9, 9, 9}, 3, accept)
	if err != nil {
		t.Fatalf("SearchNearestFloat (filtered): %v", err)
	}
	for _, sd := range td.ScoreDocs {
		if sd.Doc == 3 {
			t.Errorf("filtered search returned excluded doc 3 (scoreDocs=%v)", scoreDocsString(td))
		}
	}
	if len(td.ScoreDocs) == 0 || td.ScoreDocs[0].Doc != 5 {
		t.Errorf("filtered nearest = %v, want doc 5 first", scoreDocsString(td))
	}
	// Scores must be in non-increasing order (TopDocs contract).
	for i := 1; i < len(td.ScoreDocs); i++ {
		if td.ScoreDocs[i-1].Score < td.ScoreDocs[i].Score {
			t.Errorf("scoreDocs not score-descending: %v", scoreDocsString(td))
		}
	}
}

// TestLucene99FlatVectors_DenseByteRoundTrip mirrors the float case for
// BYTE-encoded vectors.
func TestLucene99FlatVectors_DenseByteRoundTrip(t *testing.T) {
	const (
		field  = "bvec"
		dim    = 4
		maxDoc = 4
	)
	vectors := [][]byte{
		{0, 0, 0, 0},
		{10, 0, 0, 0},
		{0, 0, 0, 50},
		{1, 2, 3, 4},
	}

	fis := index.NewFieldInfos()
	if err := fis.Add(vectorFieldInfoByte(field, 0, dim, index.VectorSimilarityFunctionEuclidean)); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	if err := si.SetID(seqID()); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}

	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 0, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}
	fw, err := w.AddField(fis.GetByName(field))
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for doc, v := range vectors {
		if err := fw.AddValue(doc, v); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, err)
		}
	}
	if err := w.Flush(maxDoc, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rs := &SegmentReadState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsReader: %v", err)
	}
	defer r.Close()

	bvv, err := r.GetByteVectorValues(field)
	if err != nil {
		t.Fatalf("GetByteVectorValues: %v", err)
	}
	if bvv.Size() != maxDoc {
		t.Fatalf("Size = %d, want %d", bvv.Size(), maxDoc)
	}
	for doc := range vectors {
		got, err := bvv.GetVector(doc)
		if err != nil {
			t.Fatalf("GetVector(%d): %v", doc, err)
		}
		if string(got) != string(vectors[doc]) {
			t.Errorf("GetVector(%d) = %v, want %v", doc, got, vectors[doc])
		}
	}

	// Search near doc 2 ({0,0,0,50}).
	td, err := r.SearchNearestByte(field, []byte{0, 0, 0, 48}, 2, nil)
	if err != nil {
		t.Fatalf("SearchNearestByte: %v", err)
	}
	if len(td.ScoreDocs) == 0 || td.ScoreDocs[0].Doc != 2 {
		t.Errorf("nearest byte doc = %v, want doc 2 first", scoreDocsString(td))
	}
}

// TestLucene99FlatVectors_SparseUnsupported verifies that a field that is
// missing a value for some document (count < maxDoc) surfaces a typed error
// referencing rmp #4755, rather than silently producing a corrupt segment.
func TestLucene99FlatVectors_SparseUnsupported(t *testing.T) {
	const (
		field  = "vec"
		dim    = 2
		maxDoc = 5 // but only 3 docs carry a vector -> sparse
	)
	fis := index.NewFieldInfos()
	if err := fis.Add(vectorFieldInfoFloat(field, 0, dim, index.VectorSimilarityFunctionEuclidean)); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	if err := si.SetID(seqID()); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}

	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 0, 1)
	if err != nil {
		t.Fatalf("NewLucene99HnswVectorsWriter: %v", err)
	}
	fw, err := w.AddField(fis.GetByName(field))
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	// Only docs 0, 2, 4 have a vector -> 3 of 5 -> sparse.
	for _, doc := range []int{0, 2, 4} {
		if err := fw.AddValue(doc, []float32{float32(doc), 1}); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, err)
		}
	}
	err = w.Flush(maxDoc, nil)
	if err == nil {
		t.Fatalf("Flush of sparse field: want error, got nil")
	}
	if !errors.Is(err, errFlatSparseUnsupported) {
		t.Fatalf("Flush error = %v, want errFlatSparseUnsupported", err)
	}
	// The error message must reference the tracking task so the deferral is
	// discoverable.
	if !containsSubstring(err.Error(), "4755") {
		t.Errorf("sparse error %q does not reference rmp #4755", err.Error())
	}
	_ = w.Close()
}

// --- small test helpers ---

func seqID() []byte {
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 7)
	}
	return id
}

func float32SlicesEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !(math.IsNaN(float64(a[i])) && math.IsNaN(float64(b[i]))) {
			return false
		}
	}
	return true
}

func containsSubstring(s, sub string) bool {
	return strings.Contains(s, sub)
}

func scoreDocsString(td *utilhnsw.TopDocs) string {
	parts := make([]string, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		parts[i] = fmt.Sprintf("{doc=%d score=%.4f}", sd.Doc, sd.Score)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
