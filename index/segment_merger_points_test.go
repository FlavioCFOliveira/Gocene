// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the points (BKD) leg of the segment merge
// (rmp #14/#114): two committed segments with IntPoint values are merged and
// the merged BKD tree is read back, proving every point's packed value is
// preserved byte-for-byte at its remapped docID.

package index_test

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// collectPointsVisitor records docID -> packedValue, accepting every point
// (Compare returns CELL_CROSSES_QUERY so packed values are delivered).
type collectPointsVisitor struct {
	m map[int][]byte
}

func (c *collectPointsVisitor) Visit(docID int) error { return nil }
func (c *collectPointsVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	cp := make([]byte, len(packedValue))
	copy(cp, packedValue)
	c.m[docID] = cp
	return nil
}
func (c *collectPointsVisitor) Compare(minPackedValue, maxPackedValue []byte) int { return 2 }
func (c *collectPointsVisitor) Grow(count int)                                    {}

func intersectPoints(t *testing.T, pv index.PointValues) map[int][]byte {
	t.Helper()
	iv, ok := pv.(interface {
		Intersect(visitor index.PointTreeIntersectVisitor) error
	})
	if !ok {
		t.Fatalf("PointValues %T is not intersectable", pv)
	}
	c := &collectPointsVisitor{m: map[int][]byte{}}
	if err := iv.Intersect(c); err != nil {
		t.Fatalf("Intersect: %v", err)
	}
	return c.m
}

func TestSegmentMerger_PointsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPt := func(v int32) {
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("pt", v))
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1 -> 50,10.
	addPt(50)
	addPt(10)
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged docs 2,3 -> 30,20.
	addPt(30)
	addPt(20)
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segReaders := reader.GetSegmentReaders()
	if len(segReaders) < 2 {
		t.Fatalf("expected >= 2 segments, got %d", len(segReaders))
	}

	// Expected merged packed values: collect each segment's local points and
	// place them at the concatenated merged docIDs (seg0 -> [0..), seg1 -> ...).
	want := map[int][]byte{}
	base := 0
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		pv, err := sr.GetPointValues("pt")
		if err != nil || pv == nil {
			t.Fatalf("segment GetPointValues: pv=%v err=%v", pv, err)
		}
		local := intersectPoints(t, pv)
		for localDoc, packed := range local {
			want[base+localDoc] = packed
		}
		base += sr.MaxDoc()

		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
	}
	if len(want) != 4 {
		t.Fatalf("expected 4 source points, got %d", len(want))
	}

	mergedSI := index.NewSegmentInfo("_merged", total, dir)
	mergedSI.SetCodec(index.GetDefaultCodec().Name())
	merger, err := index.NewSegmentMerger(codecReaders, mergedSI, nil, nil, dir, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		t.Fatalf("NewSegmentMerger: %v", err)
	}
	ms, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Read the merged BKD tree back.
	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	pr, err := codec.PointsFormat().FieldsReader(rs)
	if err != nil {
		t.Fatalf("Points FieldsReader: %v", err)
	}
	defer pr.Close()

	getter, ok := pr.(interface {
		GetValues(field string) (index.PointValues, error)
	})
	if !ok {
		t.Fatalf("PointsReader %T has no GetValues", pr)
	}
	mpv, err := getter.GetValues("pt")
	if err != nil || mpv == nil {
		t.Fatalf("merged GetValues: pv=%v err=%v", mpv, err)
	}
	got := intersectPoints(t, mpv)

	if len(got) != len(want) {
		t.Fatalf("merged points count = %d, want %d", len(got), len(want))
	}
	for d, exp := range want {
		gv, ok := got[d]
		if !ok {
			t.Errorf("merged points missing doc %d", d)
			continue
		}
		if !bytes.Equal(gv, exp) {
			t.Errorf("merged point doc %d packed = %x, want %x", d, gv, exp)
		}
	}
}
