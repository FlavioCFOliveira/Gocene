// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90_test

import (
	"crypto/rand"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakePointsSource is an in-memory PointsSource over a single 1-dimension,
// 4-byte field. It mirrors what DocumentsWriterPerThread.flushPoints feeds the
// Lucene90 points writer: a per-field point count and a doc-ordered walk over
// (docID, packedValue) pairs.
type fakePointsSource struct {
	field  string
	docIDs []int
	values [][]byte
}

func (s *fakePointsSource) PointValueCount(field string) int64 {
	if field != s.field {
		return 0
	}
	return int64(len(s.values))
}

func (s *fakePointsSource) VisitPoints(field string, fn func(docID int, packedValue []byte) error) error {
	if field != s.field {
		return nil
	}
	for i, v := range s.values {
		if err := fn(s.docIDs[i], v); err != nil {
			return err
		}
	}
	return nil
}

// CheckIntegrity / Close satisfy the narrow codecs.PointsReader surface the
// writer's WriteField parameter declares.
func (s *fakePointsSource) CheckIntegrity() error { return nil }
func (s *fakePointsSource) Close() error          { return nil }

func packInt32BE(v int32) []byte {
	b := make([]byte, 4)
	// Sortable encoding flips the sign bit so signed ints order correctly as
	// unsigned big-endian bytes (org.apache.lucene.util.NumericUtils.intToSortableBytes).
	binary.BigEndian.PutUint32(b, uint32(v)^0x80000000)
	return b
}

// rangeVisitor is a minimal index.PointTreeIntersectVisitor that collects every
// docID whose packed value lies within [lo, hi] (inclusive), driving the BKD
// walk exactly as search.PointRangeQuery does.
type rangeVisitor struct {
	lo, hi []byte
	hits   []int
}

func cmp(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func (v *rangeVisitor) Grow(int) {}

func (v *rangeVisitor) Visit(docID int) error {
	v.hits = append(v.hits, docID)
	return nil
}

func (v *rangeVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if cmp(packedValue, v.lo) >= 0 && cmp(packedValue, v.hi) <= 0 {
		v.hits = append(v.hits, docID)
	}
	return nil
}

func (v *rangeVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	if cmp(v.lo, maxPackedValue) > 0 || cmp(v.hi, minPackedValue) < 0 {
		return 0 // CELL_OUTSIDE_QUERY
	}
	if cmp(v.lo, minPackedValue) <= 0 && cmp(v.hi, maxPackedValue) >= 0 {
		return 1 // CELL_INSIDE_QUERY
	}
	return 2 // CELL_CROSSES_QUERY
}

// TestLucene90Points_BKDRoundTrip writes a 1D point field through the BKD
// writer, reads it back through the reader, and verifies the BKDReader-backed
// PointValues round-trips both metadata (docCount, dims, bytesPerDim,
// min/max) and a range intersection. It exercises the byte-faithful .kdd /
// .kdi / .kdm framing end to end (write -> read on the same directory).
func TestLucene90Points_BKDRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	dir, err := store.NewSimpleFSDirectory(tmp)
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	const numDocs = 50
	const field = "f"

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := schema.NewSegmentInfo("_0", numDocs, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}

	fis := schema.NewFieldInfos()
	fi := schema.NewFieldInfo(field, 0, schema.FieldInfoOptions{
		IndexOptions:             schema.IndexOptionsNone,
		PointDimensionCount:      1,
		PointIndexDimensionCount: 1,
		PointNumBytes:            4,
	})
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	src := &fakePointsSource{field: field}
	for i := 0; i < numDocs; i++ {
		src.docIDs = append(src.docIDs, i)
		src.values = append(src.values, packInt32BE(int32(i)))
	}

	writeState := &codecs.SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	format := codecs.NewLucene90PointsFormat()
	w, err := format.FieldsWriter(writeState)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	if err := w.WriteField(fi, src); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, ext := range []string{"kdd", "kdi", "kdm"} {
		if !dir.FileExists("_0." + ext) {
			t.Fatalf("expected _0.%s to exist", ext)
		}
	}

	readState := &codecs.SegmentReadState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	r, err := format.FieldsReader(readState)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer r.Close()
	if err := r.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	getter, ok := r.(interface {
		GetValues(string) (index.PointValues, error)
	})
	if !ok {
		t.Fatalf("reader %T does not expose GetValues", r)
	}
	pv, err := getter.GetValues(field)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if pv == nil {
		t.Fatal("GetValues returned nil PointValues")
	}

	if got := pv.GetDocCount(); got != numDocs {
		t.Errorf("GetDocCount = %d, want %d", got, numDocs)
	}
	if got := pv.GetNumDimensions(); got != 1 {
		t.Errorf("GetNumDimensions = %d, want 1", got)
	}
	if got := pv.GetBytesPerDimension(); got != 4 {
		t.Errorf("GetBytesPerDimension = %d, want 4", got)
	}
	min, _ := pv.GetMinPackedValue()
	max, _ := pv.GetMaxPackedValue()
	if cmp(min, packInt32BE(0)) != 0 {
		t.Errorf("GetMinPackedValue = %x, want %x", min, packInt32BE(0))
	}
	if cmp(max, packInt32BE(numDocs-1)) != 0 {
		t.Errorf("GetMaxPackedValue = %x, want %x", max, packInt32BE(numDocs-1))
	}

	// Range intersection [10, 19] -> docs 10..19.
	intersector, ok := pv.(interface {
		Intersect(index.PointTreeIntersectVisitor) error
	})
	if !ok {
		t.Fatalf("PointValues %T does not expose Intersect", pv)
	}
	v := &rangeVisitor{lo: packInt32BE(10), hi: packInt32BE(19)}
	if err := intersector.Intersect(v); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	sort.Ints(v.hits)
	want := []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	if len(v.hits) != len(want) {
		t.Fatalf("range hits = %v, want %v", v.hits, want)
	}
	for i := range want {
		if v.hits[i] != want[i] {
			t.Fatalf("range hits = %v, want %v", v.hits, want)
		}
	}
}
