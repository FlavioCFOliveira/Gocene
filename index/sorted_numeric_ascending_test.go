// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestSortedNumericDocValues_FlushSortsAscending is the regression for rmp #4783:
// Apache Lucene 10.4.0 stores each document's SortedNumericDocValues values in
// ascending order (SortedNumericDocValuesWriter sorts the per-doc buffer before
// flush). Gocene previously persisted them in insertion order. Index a document
// whose values are added unsorted and assert they read back ascending.
func TestSortedNumericDocValues_FlushSortsAscending(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sndv, err := document.NewSortedNumericDocValuesField("sndv", []int64{7, 3, 99, -5, 3})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesField: %v", err)
	}
	doc.Add(sndv)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	dv, err := segs[0].GetSortedNumericDocValues("sndv")
	if err != nil || dv == nil {
		t.Fatalf("GetSortedNumericDocValues: dv=%v err=%v", dv, err)
	}
	if doc, err := dv.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("NextDoc: doc=%d err=%v, want 0", doc, err)
	}
	cnt, err := dv.DocValueCount()
	if err != nil {
		t.Fatalf("DocValueCount: %v", err)
	}
	if cnt != 5 {
		t.Fatalf("DocValueCount = %d, want 5", cnt)
	}
	want := []int64{-5, 3, 3, 7, 99} // ascending, duplicates preserved
	for i, w := range want {
		got, err := dv.NextValue()
		if err != nil {
			t.Fatalf("NextValue[%d]: %v", i, err)
		}
		if got != w {
			t.Fatalf("value[%d] = %d, want %d (values must read back ascending)", i, got, w)
		}
	}
}
