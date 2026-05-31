// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the numeric / sorted-numeric doc-values legs of the
// segment merge (rmp #14/#114): two committed segments are merged and the
// merged segment's doc values are read back per document, proving each value's
// docID is remapped into the merged doc space.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestSegmentMerger_NumericDocValuesRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addNum := func(v int64) {
		doc := document.NewDocument()
		f, _ := document.NewNumericDocValuesField("nval", v)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1 -> 50, 10.
	addNum(50)
	addNum(10)
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged docs 2,3 -> 30, 20.
	addNum(30)
	addNum(20)
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

	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
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

	// Read the merged numeric doc values back.
	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	prod, err := codec.DocValuesFormat().FieldsProducer(rs)
	if err != nil {
		t.Fatalf("DocValues FieldsProducer: %v", err)
	}
	defer prod.Close()

	fi := ms.MergeFieldInfos.GetByName("nval")
	if fi == nil {
		t.Fatalf("merged field infos missing nval")
	}
	ndv, err := prod.GetNumeric(fi)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	if ndv == nil {
		t.Fatalf("GetNumeric returned nil")
	}

	got := map[int]int64{}
	for {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= total {
			break
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		got[d] = v
	}

	want := map[int]int64{0: 50, 1: 10, 2: 30, 3: 20}
	if len(got) != len(want) {
		t.Fatalf("merged DV count = %d, want %d (got %v)", len(got), len(want), got)
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("merged DV doc %d = %d, want %d", d, got[d], v)
		}
	}
}

func TestSegmentMerger_SortedDocValuesRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addSorted := func(v string) {
		doc := document.NewDocument()
		f, _ := document.NewSortedDocValuesField("sval", []byte(v))
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: docs 0,1 -> "ccc","aaa".
	addSorted("ccc")
	addSorted("aaa")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: docs 2,3 -> "bbb","aaa".
	addSorted("bbb")
	addSorted("aaa")
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
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
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

	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	prod, err := codec.DocValuesFormat().FieldsProducer(rs)
	if err != nil {
		t.Fatalf("DocValues FieldsProducer: %v", err)
	}
	defer prod.Close()

	fi := ms.MergeFieldInfos.GetByName("sval")
	sdv, err := prod.GetSorted(fi)
	if err != nil {
		t.Fatalf("GetSorted: %v", err)
	}
	if sdv == nil {
		t.Fatalf("GetSorted returned nil")
	}

	got := map[int]string{}
	for {
		d, err := sdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= total {
			break
		}
		ord, err := sdv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue: %v", err)
		}
		bv, err := sdv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd: %v", err)
		}
		got[d] = string(bv)
	}
	want := map[int]string{0: "ccc", 1: "aaa", 2: "bbb", 3: "aaa"}
	if len(got) != len(want) {
		t.Fatalf("merged sorted DV count = %d, want %d (got %v)", len(got), len(want), got)
	}
	for d, v := range want {
		if got[d] != v {
			t.Errorf("merged sorted DV doc %d = %q, want %q", d, got[d], v)
		}
	}
}

func TestSegmentMerger_SortedSetDocValuesRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addSet := func(vals ...string) {
		doc := document.NewDocument()
		bb := make([][]byte, len(vals))
		for i, v := range vals {
			bb[i] = []byte(v)
		}
		f, err := document.NewSortedSetDocValuesField("ssval", bb)
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: doc0 {b,d}, doc1 {a}.
	addSet("b", "d")
	addSet("a")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: doc2 {c,a}, doc3 {d}.
	addSet("c", "a")
	addSet("d")
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
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
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

	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	prod, err := codec.DocValuesFormat().FieldsProducer(rs)
	if err != nil {
		t.Fatalf("DocValues FieldsProducer: %v", err)
	}
	defer prod.Close()

	fi := ms.MergeFieldInfos.GetByName("ssval")
	ssdv, err := prod.GetSortedSet(fi)
	if err != nil {
		t.Fatalf("GetSortedSet: %v", err)
	}
	if ssdv == nil {
		t.Fatalf("GetSortedSet returned nil")
	}

	got := map[int][]string{}
	for {
		d, err := ssdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= total {
			break
		}
		var vals []string
		for {
			ord, err := ssdv.NextOrd()
			if err != nil {
				t.Fatalf("NextOrd: %v", err)
			}
			if ord < 0 {
				break
			}
			bv, err := ssdv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("LookupOrd: %v", err)
			}
			vals = append(vals, string(bv))
		}
		got[d] = vals
	}

	// Merged value table: a,b,c,d. Each doc's values come back ascending.
	want := map[int][]string{
		0: {"b", "d"},
		1: {"a"},
		2: {"a", "c"},
		3: {"d"},
	}
	if len(got) != len(want) {
		t.Fatalf("merged sorted-set DV doc count = %d, want %d (got %v)", len(got), len(want), got)
	}
	for d, exp := range want {
		gv := got[d]
		if len(gv) != len(exp) {
			t.Errorf("doc %d values = %v, want %v", d, gv, exp)
			continue
		}
		for i := range exp {
			if gv[i] != exp[i] {
				t.Errorf("doc %d value[%d] = %q, want %q (got %v)", d, i, gv[i], exp[i], gv)
			}
		}
	}
}
