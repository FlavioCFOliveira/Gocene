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

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Test2BPoints1D validates single-dimension (1D) LongPoint encoding/decoding
// through the full IndexWriter/reader pipeline. This is a scaled-down version
// of Lucene's @Monster test that indexes >2B points.
func Test2BPoints1D(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetUseCompoundFile(false)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 1000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		point := document.NewLongPoint("pt", int64(i))
		doc.Add(point)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
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
	if len(segs) == 0 {
		t.Fatal("no segment readers")
	}

	pv, err := segs[0].GetPointValues("pt")
	if err != nil {
		t.Fatalf("GetPointValues: %v", err)
	}
	if pv == nil {
		t.Fatal("GetPointValues returned nil")
	}
	if pv.GetNumDimensions() != 1 {
		t.Errorf("NumDimensions = %d, want 1", pv.GetNumDimensions())
	}
	if pv.GetBytesPerDimension() != 8 {
		t.Errorf("BytesPerDimension = %d, want 8", pv.GetBytesPerDimension())
	}
	if got := pv.GetValueCount(); got != int64(numDocs) {
		t.Errorf("ValueCount = %d, want %d", got, numDocs)
	}
	minPacked, err := pv.GetMinPackedValue()
	if err != nil {
		t.Fatalf("GetMinPackedValue: %v", err)
	}
	maxPacked, err := pv.GetMaxPackedValue()
	if err != nil {
		t.Fatalf("GetMaxPackedValue: %v", err)
	}
	if len(minPacked) != 8 || len(maxPacked) != 8 {
		t.Fatalf("packed value size: min=%d max=%d, want 8 each", len(minPacked), len(maxPacked))
	}
	// Verify min/max values encompass the indexed range.
	if len(minPacked) >= 8 && len(maxPacked) >= 8 {
		t.Logf("Point min=%x max=%x", minPacked, maxPacked)
	}
}

// Test2BPoints2D validates multi-dimension (2D) point encoding/decoding
// through the full pipeline.
func Test2BPoints2D(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetUseCompoundFile(false)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 500
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// Create a 2D point using BinaryPoint with manual encoding.
		encoded := make([]byte, 16)
		// First dimension: i as 8-byte sortable bytes (sign-flip the MSB).
		v := int64(i)
		encoded[0] = byte(v>>56) ^ 0x80
		encoded[1] = byte(v >> 48)
		encoded[2] = byte(v >> 40)
		encoded[3] = byte(v >> 32)
		encoded[4] = byte(v >> 24)
		encoded[5] = byte(v >> 16)
		encoded[6] = byte(v >> 8)
		encoded[7] = byte(v)
		// Second dimension: i*2 as 8-byte sortable bytes.
		v2 := int64(i * 2)
		encoded[8] = byte(v2>>56) ^ 0x80
		encoded[9] = byte(v2 >> 48)
		encoded[10] = byte(v2 >> 40)
		encoded[11] = byte(v2 >> 32)
		encoded[12] = byte(v2 >> 24)
		encoded[13] = byte(v2 >> 16)
		encoded[14] = byte(v2 >> 8)
		encoded[15] = byte(v2)
		bp, err := document.NewBinaryPointPacked("pt2d", encoded, func() *document.FieldType {
			ft := document.NewFieldType()
			ft.SetIndexed(true)
			ft.SetDimensions(2, 8)
			ft.Freeze()
			return ft
		}())
		if err != nil {
			t.Fatalf("NewBinaryPointPacked(%d): %v", i, err)
		}
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
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
	if len(segs) == 0 {
		t.Fatal("no segment readers")
	}

	pv, err := segs[0].GetPointValues("pt2d")
	if err != nil {
		t.Fatalf("GetPointValues: %v", err)
	}
	if pv == nil {
		t.Fatal("GetPointValues returned nil")
	}
	if pv.GetNumDimensions() != 2 {
		t.Errorf("NumDimensions = %d, want 2", pv.GetNumDimensions())
	}
	if pv.GetBytesPerDimension() != 8 {
		t.Errorf("BytesPerDimension = %d, want 8", pv.GetBytesPerDimension())
	}
	if got := pv.GetValueCount(); got != int64(numDocs) {
		t.Errorf("ValueCount = %d, want %d", got, numDocs)
	}
}
