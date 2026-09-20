// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// readOrdToDocConfiguration writes the OrdToDocDISIReaderConfiguration block
// that OrdToDocDISIReaderConfiguration.writeStoredMeta records for count
// vectors in a segment of maxDoc documents, and reads it back.
func readOrdToDocConfiguration(t *testing.T, dir store.Directory, name string, count, maxDoc int) *lucene95.OrdToDocDISIReaderConfiguration {
	t.Helper()
	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	docsWithField := index.NewDocsWithFieldSet()
	for doc := 0; doc < count; doc++ {
		if err := docsWithField.Add(doc); err != nil {
			t.Fatalf("DocsWithFieldSet.Add: %v", err)
		}
	}
	if err := lucene95.WriteStoredMeta(16, out, out, count, maxDoc, docsWithField); err != nil {
		t.Fatalf("WriteStoredMeta: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	in, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() {
		if err := in.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	conf, err := lucene95.FromStoredMeta(in, count)
	if err != nil {
		t.Fatalf("FromStoredMeta: %v", err)
	}
	return conf
}

// TestLucene102BinaryQuantizedVectorsWriter_Format verifies the format name
// and the maximum dimensions it reports.
func TestLucene102BinaryQuantizedVectorsWriter_Format(t *testing.T) {
	f := NewLucene102BinaryQuantizedVectorsFormat()
	if got, want := f.Name(), "Lucene102BinaryQuantizedVectorsFormat"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := f.GetMaxDimensions("field"), 1024; got != want {
		t.Errorf("GetMaxDimensions = %d, want %d", got, want)
	}
}

// TestLucene102BinaryQuantizedVectorsWriter_OffHeapValues verifies the empty
// OffHeapBinarizedVectorValues that load returns for a field without vectors.
func TestLucene102BinaryQuantizedVectorsWriter_OffHeapValues(t *testing.T) {
	conf := readOrdToDocConfiguration(t, store.NewByteBuffersDirectory(), "empty", 0, 1)
	values, err := loadOffHeapBinarizedVectorValues(
		conf, 8, 0, nil, index.VectorSimilarityFunctionEuclidean,
		lucene102BinaryFlatVectorsScorer, nil, float32(math.NaN()), 0, 0, nil)
	if err != nil {
		t.Fatalf("loadOffHeapBinarizedVectorValues: %v", err)
	}
	if got := values.Size(); got != 0 {
		t.Errorf("Size = %d, want 0", got)
	}
	if got := values.Dimension(); got != 8 {
		t.Errorf("Dimension = %d, want 8", got)
	}
	if got := values.GetVectorByteLength(); got != 8 {
		t.Errorf("GetVectorByteLength = %d, want 8", got)
	}
	if got := values.GetAcceptOrds(nil); got != nil {
		t.Errorf("GetAcceptOrds = %v, want nil", got)
	}
	scorer, err := values.ScorerFloat(make([]float32, 8))
	if err != nil || scorer != nil {
		t.Errorf("ScorerFloat = (%v, %v), want (nil, nil)", scorer, err)
	}
	if _, err := values.CopyBinarizedByteVectorValues(); err == nil {
		t.Error("CopyBinarizedByteVectorValues on empty values: want UnsupportedOperationException, got nil")
	}
}

// TestLucene102BinaryQuantizedVectorsWriter_Reader verifies that the reader
// constructor fails on a segment without a .vemb metadata file.
func TestLucene102BinaryQuantizedVectorsWriter_Reader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo("_0", 1, dir)
	rs := &codecs.SegmentReadState{Directory: dir, SegmentInfo: si}
	r, err := NewLucene102BinaryQuantizedVectorsReader(rs, nil, NewLucene102BinaryFlatVectorsScorer(hnsw.DefaultFlatVectorScorerInstance))
	if err == nil {
		t.Fatalf("NewLucene102BinaryQuantizedVectorsReader = %v, want an error for a missing .vemb file", r)
	}
}
