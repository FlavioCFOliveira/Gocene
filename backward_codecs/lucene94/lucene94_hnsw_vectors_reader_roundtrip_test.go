// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene94HnswVectorsReader_ReadMetadata_LittleEndianPayload writes a .vem
// metadata file byte-shaped exactly the way Apache Lucene 9.4 writes it — the
// CodecUtil header/footer framing is big-endian while every payload integer is
// little-endian (Lucene 9.0+ DataOutput is little-endian) — and verifies that
// readMetadata decodes every payload field correctly. This guards the BE->LE
// payload-read fix (rmp #4787): before the fix the fixed-width integers were
// read big-endian via store.ReadInt32/64/16 and would not match a real Lucene
// 9.4 index.
func TestLucene94HnswVectorsReader_ReadMetadata_LittleEndianPayload(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer func() { _ = dir.Close() }()

	const (
		segName = "_0"
		dim     = 4
		size    = 3
		maxConn = 16
	)
	segID := []byte("0123456789abcdef") // 16 bytes
	vectorDataLength := int64(size) * int64(dim) * 4

	// FieldInfo the reader cross-checks against (dimension/encoding/similarity).
	fi := index.NewFieldInfo("vec", 0, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionDotProduct,
	})
	infos := index.NewFieldInfos()
	if err := infos.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	// Write the .vem exactly like Lucene94HnswVectorsWriter: BE framing via
	// CodecUtil, LE payload via the IndexOutput.WriteInt/WriteLong/WriteShort
	// methods (which are little-endian per rmp #4786).
	metaName := index.SegmentFileName(segName, "", lucene94MetaExtension)
	rawOut, err := dir.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out := store.NewChecksumIndexOutput(rawOut)

	if err := codecs.WriteIndexHeader(out, lucene94MetaCodecName,
		lucene94VersionCurrent, segID, ""); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}

	// One field entry (dense docsWithField, single graph level).
	mustWriteInt(t, out, 0)                                  // field number
	mustWriteInt(t, out, int32(index.VectorEncodingFloat32)) // encoding ordinal
	mustWriteInt(t, out, int32(index.VectorSimilarityFunctionDotProduct))
	mustWriteVLong(t, out, 0)                // vectorDataOffset
	mustWriteVLong(t, out, vectorDataLength) // vectorDataLength
	mustWriteVLong(t, out, 0)                // vectorIndexOffset
	mustWriteVLong(t, out, 0)                // vectorIndexLength
	mustWriteInt(t, out, dim)                // dimension
	mustWriteInt(t, out, size)               // size
	mustWriteLong(t, out, -1)                // docsWithFieldOffset = -1 (dense)
	mustWriteLong(t, out, 0)                 // docsWithFieldLength
	mustWriteShort(t, out, 0)                // jumpTableEntryCount
	if err := out.WriteByte(0); err != nil { // denseRankPower
		t.Fatalf("WriteByte denseRankPower: %v", err)
	}
	// dense => no sparse ordToDoc block.
	mustWriteInt(t, out, maxConn) // M
	mustWriteInt(t, out, 1)       // numLevels
	mustWriteInt(t, out, 0)       // nodesByLevel[0] count (ignored for level 0)

	mustWriteInt(t, out, -1) // end-of-fields sentinel

	if err := codecs.WriteFooter(out); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	// Read it back through the production path.
	si := index.NewSegmentInfo(segName, size, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	state := &index.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  infos,
	}

	r := &Lucene94HnswVectorsReader{
		fields:     make(map[int]*lucene94FieldEntry),
		fieldInfos: infos,
	}
	version, err := r.readMetadata(state)
	if err != nil {
		t.Fatalf("readMetadata: %v", err)
	}
	if version != lucene94VersionCurrent {
		t.Errorf("version: got %d, want %d", version, lucene94VersionCurrent)
	}

	e, ok := r.fields[0]
	if !ok {
		t.Fatal("field 0 not decoded")
	}
	if e.vectorEncoding != index.VectorEncodingFloat32 {
		t.Errorf("encoding: got %v, want FLOAT32", e.vectorEncoding)
	}
	if e.similarityFunction != index.VectorSimilarityFunctionDotProduct {
		t.Errorf("similarity: got %v, want DOT_PRODUCT", e.similarityFunction)
	}
	if e.dimension != dim {
		t.Errorf("dimension: got %d, want %d", e.dimension, dim)
	}
	if e.size != size {
		t.Errorf("size: got %d, want %d", e.size, size)
	}
	if e.docsWithFieldOffset != -1 {
		t.Errorf("docsWithFieldOffset: got %d, want -1", e.docsWithFieldOffset)
	}
	if e.maxConn != maxConn {
		t.Errorf("maxConn: got %d, want %d", e.maxConn, maxConn)
	}
	if e.numLevels != 1 {
		t.Errorf("numLevels: got %d, want 1", e.numLevels)
	}
	if e.vectorDataLength != vectorDataLength {
		t.Errorf("vectorDataLength: got %d, want %d", e.vectorDataLength, vectorDataLength)
	}
}

func mustWriteInt(t *testing.T, out store.IndexOutput, v int32) {
	t.Helper()
	if err := out.WriteInt(v); err != nil {
		t.Fatalf("WriteInt(%d): %v", v, err)
	}
}

func mustWriteLong(t *testing.T, out store.IndexOutput, v int64) {
	t.Helper()
	if err := out.WriteLong(v); err != nil {
		t.Fatalf("WriteLong(%d): %v", v, err)
	}
}

func mustWriteShort(t *testing.T, out store.IndexOutput, v int16) {
	t.Helper()
	if err := out.WriteShort(v); err != nil {
		t.Fatalf("WriteShort(%d): %v", v, err)
	}
}

func mustWriteVLong(t *testing.T, out store.IndexOutput, v int64) {
	t.Helper()
	if err := store.WriteVLong(out, v); err != nil {
		t.Fatalf("WriteVLong(%d): %v", v, err)
	}
}
