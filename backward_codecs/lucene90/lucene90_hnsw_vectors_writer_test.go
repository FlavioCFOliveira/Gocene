// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLucene90HnswGraphBuilder_New(t *testing.T) {
	b := NewLucene90HnswGraphBuilder("1.0")
	if b == nil {
		t.Fatal("NewLucene90HnswGraphBuilder returned nil")
	}
	if b.Name != "Lucene90HnswGraphBuilder" {
		t.Fatalf("got Name=%q, want %q", b.Name, "Lucene90HnswGraphBuilder")
	}
}

func TestLucene90HnswGraphBuilder_Version(t *testing.T) {
	b := NewLucene90HnswGraphBuilder("v2")
	if b.Version != "v2" {
		t.Fatalf("got Version=%q, want %q", b.Version, "v2")
	}
}

func TestLucene90NeighborArray_New(t *testing.T) {
	n := NewLucene90NeighborArray("1.0")
	if n == nil {
		t.Fatal("NewLucene90NeighborArray returned nil")
	}
	if n.Name != "Lucene90NeighborArray" {
		t.Fatalf("got Name=%q, want %q", n.Name, "Lucene90NeighborArray")
	}
}

func TestLucene90NeighborArray_Version(t *testing.T) {
	n := NewLucene90NeighborArray("v3")
	if n.Version != "v3" {
		t.Fatalf("got Version=%q, want %q", n.Version, "v3")
	}
}

func TestLucene90OnHeapHnswGraph_New(t *testing.T) {
	g := NewLucene90OnHeapHnswGraph("1.0")
	if g == nil {
		t.Fatal("NewLucene90OnHeapHnswGraph returned nil")
	}
	if g.Name != "Lucene90OnHeapHnswGraph" {
		t.Fatalf("got Name=%q, want %q", g.Name, "Lucene90OnHeapHnswGraph")
	}
}

func TestLucene90OnHeapHnswGraph_Version(t *testing.T) {
	g := NewLucene90OnHeapHnswGraph("v4")
	if g.Version != "v4" {
		t.Fatalf("got Version=%q, want %q", g.Version, "v4")
	}
}

// TestLucene90HnswVectorsWriter_FlushRoundTrip exercises the write path
// end-to-end: construct a writer, add float vectors, flush/finish/close,
// and assert the three segment files (.vec / .vem / .vex) exist on disk.
func TestLucene90HnswVectorsWriter_FlushRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si := schema.NewSegmentInfo("_0", 5, dir)
	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}

	w, err := NewLucene90HnswVectorsWriter(state, 16, 100)
	if err != nil {
		t.Fatalf("NewLucene90HnswVectorsWriter: %v", err)
	}

	opts := schema.DefaultFieldInfoOptions()
	opts.VectorDimension = 3
	opts.VectorEncoding = index.VectorEncodingFloat32
	opts.VectorSimilarityFunction = index.VectorSimilarityFunctionEuclidean
	fi := schema.NewFieldInfo("vec", 0, opts)

	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}

	for i := 0; i < 5; i++ {
		vec := []float32{float32(i), float32(i + 1), float32(i + 2)}
		if err := fw.AddValue(i, vec); err != nil {
			t.Fatalf("AddValue doc=%d: %v", i, err)
		}
	}
	if err := fw.Finish(); err != nil {
		t.Fatalf("Finish field: %v", err)
	}

	if err := w.Flush(5, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, ext := range []string{"vec", "vem", "vex"} {
		name := "_0." + ext
		if !dir.FileExists(name) {
			t.Errorf("expected file %q to exist in RAM directory", name)
		}
	}
}

// TestLucene90HnswVectorsWriter_RejectByteField proves that byte vector
// fields are rejected with a typed error, matching the Java
// UnsupportedOperationException.
func TestLucene90HnswVectorsWriter_RejectByteField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si := schema.NewSegmentInfo("_0", 1, dir)
	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}

	w, err := NewLucene90HnswVectorsWriter(state, 16, 100)
	if err != nil {
		t.Fatalf("NewLucene90HnswVectorsWriter: %v", err)
	}
	defer w.Close()

	opts := schema.DefaultFieldInfoOptions()
	opts.VectorDimension = 4
	opts.VectorEncoding = index.VectorEncodingByte
	opts.VectorSimilarityFunction = index.VectorSimilarityFunctionEuclidean
	fi := schema.NewFieldInfo("byte_vec", 0, opts)

	_, err = w.AddField(fi)
	if err == nil {
		t.Fatal("expected error for byte vector field, got nil")
	}
}
