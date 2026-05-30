// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene95HnswVectorsReader_ReadMetadata_LittleEndianPayload writes a .vem
// metadata file byte-shaped exactly the way Apache Lucene 9.5 writes it — the
// CodecUtil header/footer framing is big-endian while every payload integer is
// little-endian (Lucene 9.0+ DataOutput is little-endian) — and verifies that
// readMetadata decodes every fixed-width payload field correctly. This guards
// the BE->LE payload-read fix (rmp #4787).
//
// An empty field (size == 0) is used so the FieldEntry carries no HNSW
// neighbour-offset block (numberOfOffsets == 0), which keeps the fixture free
// of a DirectMonotonic meta sub-stream while still exercising every fixed-width
// integer read that the fix touches (field number, encoding, similarity, size,
// docsWithFieldOffset/Length, jumpTableEntryCount).
func TestLucene95HnswVectorsReader_ReadMetadata_LittleEndianPayload(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer func() { _ = dir.Close() }()

	const (
		segName = "_0"
		dim     = 8
	)
	segID := []byte("fedcba9876543210") // 16 bytes

	fi := index.NewFieldInfo("vec", 0, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	})
	infos := index.NewFieldInfos()
	if err := infos.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	metaName := index.SegmentFileName(segName, "", lucene95MetaExtension)
	rawOut, err := dir.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out := store.NewChecksumIndexOutput(rawOut)

	if err := codecs.WriteIndexHeader(out, lucene95MetaCodecName,
		lucene95VersionCurrent, segID, ""); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}

	mustWriteInt(t, out, 0)                                  // field number
	mustWriteInt(t, out, int32(index.VectorEncodingFloat32)) // encoding ordinal
	mustWriteInt(t, out, int32(index.VectorSimilarityFunctionEuclidean))
	mustWriteVLong(t, out, 0)  // vectorDataOffset
	mustWriteVLong(t, out, 0)  // vectorDataLength (size == 0)
	mustWriteVLong(t, out, 0)  // vectorIndexOffset
	mustWriteVLong(t, out, 0)  // vectorIndexLength
	mustWriteVInt(t, out, dim) // dimension (VInt in Lucene95)
	mustWriteInt(t, out, 0)    // size
	// OrdToDocDISIReaderConfiguration.writeStoredMeta, dense/empty case.
	mustWriteLong(t, out, -2) // docsWithFieldOffset = -2 (empty)
	mustWriteLong(t, out, 0)  // docsWithFieldLength
	mustWriteShort(t, out, 0) // jumpTableEntryCount
	if err := out.WriteByte(0); err != nil {
		t.Fatalf("WriteByte denseRankPower: %v", err)
	}
	mustWriteVInt(t, out, 16) // M
	mustWriteVInt(t, out, 1)  // numLevels (level 0 only; size 0 => 0 offsets)
	// numberOfOffsets == 0 => no neighbour-offset block.

	mustWriteInt(t, out, -1) // end-of-fields sentinel

	if err := codecs.WriteFooter(out); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	si := index.NewSegmentInfo(segName, 0, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	state := &index.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  infos,
	}

	r := &Lucene95HnswVectorsReader{
		fields:     make(map[int]*lucene95FieldEntry),
		fieldInfos: infos,
	}
	version, err := r.readMetadata(state)
	if err != nil {
		t.Fatalf("readMetadata: %v", err)
	}
	if version != lucene95VersionCurrent {
		t.Errorf("version: got %d, want %d", version, lucene95VersionCurrent)
	}

	e, ok := r.fields[0]
	if !ok {
		t.Fatal("field 0 not decoded")
	}
	if e.vectorEncoding != index.VectorEncodingFloat32 {
		t.Errorf("encoding: got %v, want FLOAT32", e.vectorEncoding)
	}
	if e.similarityFunction != index.VectorSimilarityFunctionEuclidean {
		t.Errorf("similarity: got %v, want EUCLIDEAN", e.similarityFunction)
	}
	if e.dimension != dim {
		t.Errorf("dimension: got %d, want %d", e.dimension, dim)
	}
	if e.size != 0 {
		t.Errorf("size: got %d, want 0", e.size)
	}
	if e.docsWithFieldOffset != -2 {
		t.Errorf("docsWithFieldOffset: got %d, want -2", e.docsWithFieldOffset)
	}
	if e.maxConn != 16 {
		t.Errorf("maxConn: got %d, want 16", e.maxConn)
	}
	if e.numLevels != 1 {
		t.Errorf("numLevels: got %d, want 1", e.numLevels)
	}
	if e.numberOfOffsets != 0 {
		t.Errorf("numberOfOffsets: got %d, want 0", e.numberOfOffsets)
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

func mustWriteVInt(t *testing.T, out store.IndexOutput, v int32) {
	t.Helper()
	if err := store.WriteVInt(out, v); err != nil {
		t.Fatalf("WriteVInt(%d): %v", v, err)
	}
}

func mustWriteVLong(t *testing.T, out store.IndexOutput, v int64) {
	t.Helper()
	if err := store.WriteVLong(out, v); err != nil {
		t.Fatalf("WriteVLong(%d): %v", v, err)
	}
}
