// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the KNN vectors (HNSW) leg of the segment merge
// (rmp #14/#114): two committed segments with float vectors are merged and the
// merged segment's vectors are read back, proving each vector is preserved at
// its remapped docID.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func collectFloatVectors(t *testing.T, fvv index.FloatVectorValues, maxDoc int) map[int][]float32 {
	t.Helper()
	out := map[int][]float32{}
	for {
		d, err := fvv.NextDoc()
		if err != nil {
			t.Fatalf("vector NextDoc: %v", err)
		}
		if d < 0 || d >= maxDoc {
			break
		}
		v, err := fvv.Get(d)
		if err != nil {
			t.Fatalf("vector Get(%d): %v", d, err)
		}
		cp := make([]float32, len(v))
		copy(cp, v)
		out[d] = cp
	}
	return out
}

func TestSegmentMerger_VectorsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addVec := func(v []float32) {
		doc := document.NewDocument()
		f, err := document.NewKnnFloatVectorFieldEuclidean("vec", v)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1.
	addVec([]float32{1, 0})
	addVec([]float32{0, 1})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged docs 2,3.
	addVec([]float32{1, 1})
	addVec([]float32{0.5, 0.25})
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

	// Expected merged vectors: each segment's vectors at the concatenated docIDs.
	want := map[int][]float32{}
	base := 0
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		fvv, err := sr.GetFloatVectorValues("vec")
		if err != nil || fvv == nil {
			t.Fatalf("segment GetFloatVectorValues: fvv=%v err=%v", fvv, err)
		}
		for d, v := range collectFloatVectors(t, fvv, sr.MaxDoc()) {
			want[base+d] = v
		}
		base += sr.MaxDoc()

		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
	}
	if len(want) != 4 {
		t.Fatalf("expected 4 source vectors, got %d", len(want))
	}

	mergedSI := index.NewSegmentInfo("_merged", total, dir)
	mergedSI.SetCodec(index.GetDefaultCodec().Name())
	merger, err := index.NewSegmentMerger(codecReaders, mergedSI, nil, dir, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		t.Fatalf("NewSegmentMerger: %v", err)
	}
	ms, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Read the merged vectors back.
	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	vr, err := codec.KnnVectorsFormat().FieldsReader(rs)
	if err != nil {
		t.Fatalf("KnnVectors FieldsReader: %v", err)
	}
	defer vr.Close()

	delegate, ok := vr.(interface {
		FloatVectorValues(field string) (index.FloatVectorValues, error)
	})
	if !ok {
		t.Fatalf("KnnVectorsReader %T has no FloatVectorValues", vr)
	}
	mfvv, err := delegate.FloatVectorValues("vec")
	if err != nil || mfvv == nil {
		t.Fatalf("merged FloatVectorValues: fvv=%v err=%v", mfvv, err)
	}
	got := collectFloatVectors(t, mfvv, total)

	if len(got) != len(want) {
		t.Fatalf("merged vectors count = %d, want %d", len(got), len(want))
	}
	for d, exp := range want {
		gv, ok := got[d]
		if !ok {
			t.Errorf("merged vectors missing doc %d", d)
			continue
		}
		if len(gv) != len(exp) {
			t.Errorf("merged vector doc %d len = %d, want %d", d, len(gv), len(exp))
			continue
		}
		for k := range exp {
			if gv[k] != exp[k] {
				t.Errorf("merged vector doc %d = %v, want %v", d, gv, exp)
				break
			}
		}
	}
}
