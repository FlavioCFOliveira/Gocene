// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLucene80NormsProducer_Constants verifies key format constants.
func TestLucene80NormsProducer_Constants(t *testing.T) {
	if lucene80NormsDataCodec != "Lucene80NormsData" {
		t.Errorf("dataCodec: got %q", lucene80NormsDataCodec)
	}
	if lucene80NormsMetaCodec != "Lucene80NormsMetadata" {
		t.Errorf("metaCodec: got %q", lucene80NormsMetaCodec)
	}
	if lucene80NormsVersionCurrent != 0 {
		t.Errorf("VERSION_CURRENT: got %d, want 0", lucene80NormsVersionCurrent)
	}
}

// TestLucene80NormsProducer_ClosedGetNorms verifies that GetNorms after Close
// returns an error.
func TestLucene80NormsProducer_ClosedGetNorms(t *testing.T) {
	p := &Lucene80NormsProducer{closed: true}
	if _, err := p.GetNorms(nil); err == nil {
		t.Error("GetNorms on closed producer should return error")
	}
}

// TestLucene80NormsProducer_ClosedCheckIntegrity verifies that CheckIntegrity
// after Close returns an error.
func TestLucene80NormsProducer_ClosedCheckIntegrity(t *testing.T) {
	p := &Lucene80NormsProducer{closed: true}
	if err := p.CheckIntegrity(); err == nil {
		t.Error("CheckIntegrity on closed producer should return error")
	}
}

// TestLucene80NormsProducer_NilDataClose verifies that Close on a producer
// with no data file does not panic and is idempotent.
func TestLucene80NormsProducer_NilDataClose(t *testing.T) {
	p := &Lucene80NormsProducer{}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second Close must be a no-op.
	if err := p.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}
}

// TestLucene80NormsEntry_ReadFields exercises readFields with a synthetic
// in-memory stream carrying one entry with bytesPerNorm=1.
func TestLucene80NormsEntry_ReadFields(t *testing.T) {
	// Stream layout (big-endian integers):
	//   fieldNumber   int32  = 7
	//   docsWithFieldOffset int64  = 1024
	//   docsWithFieldLength int64  = 512
	//   jumpTableEntryCount int16  = 2
	//   denseRankPower byte   = 8
	//   numDocsWithField int32  = 100
	//   bytesPerNorm   byte   = 1
	//   normsOffset    int64  = 2048
	//   sentinel       int32  = -1
	buf := int32BEBytes(7)                   // fieldNumber=7
	buf = append(buf, int64BEBytes(1024)...) // docsWithFieldOffset
	buf = append(buf, int64BEBytes(512)...)  // docsWithFieldLength
	buf = append(buf, int16BEBytes(2)...)    // jumpTableEntryCount
	buf = append(buf, byte(8))               // denseRankPower
	buf = append(buf, int32BEBytes(100)...)  // numDocsWithField
	buf = append(buf, byte(1))               // bytesPerNorm
	buf = append(buf, int64BEBytes(2048)...) // normsOffset
	buf = append(buf, int32BEBytes(-1)...)   // sentinel

	// Build a FieldInfos with a single field that has norms.
	fi := buildNormsFieldInfo(7, "myField")
	infos := buildSingleFieldInfos(fi)

	p := &Lucene80NormsProducer{
		norms: make(map[int]*lucene80NormsEntry),
	}
	in := newBEIndexInput(buf)
	if err := p.readFields(in, "testSeg", infos); err != nil {
		t.Fatalf("readFields: %v", err)
	}

	e, ok := p.norms[7]
	if !ok {
		t.Fatal("entry for field 7 not found")
	}
	if e.docsWithFieldOffset != 1024 {
		t.Errorf("docsWithFieldOffset: got %d, want 1024", e.docsWithFieldOffset)
	}
	if e.docsWithFieldLength != 512 {
		t.Errorf("docsWithFieldLength: got %d, want 512", e.docsWithFieldLength)
	}
	if e.jumpTableEntryCount != 2 {
		t.Errorf("jumpTableEntryCount: got %d, want 2", e.jumpTableEntryCount)
	}
	if e.denseRankPower != 8 {
		t.Errorf("denseRankPower: got %d, want 8", e.denseRankPower)
	}
	if e.numDocsWithField != 100 {
		t.Errorf("numDocsWithField: got %d, want 100", e.numDocsWithField)
	}
	if e.bytesPerNorm != 1 {
		t.Errorf("bytesPerNorm: got %d, want 1", e.bytesPerNorm)
	}
	if e.normsOffset != 2048 {
		t.Errorf("normsOffset: got %d, want 2048", e.normsOffset)
	}
}

// TestLucene80NormsEntry_InvalidBytesPerNorm verifies that readFields returns
// an error for an unsupported bytesPerNorm value.
func TestLucene80NormsEntry_InvalidBytesPerNorm(t *testing.T) {
	buf := int32BEBytes(7)
	buf = append(buf, int64BEBytes(0)...) // docsWithFieldOffset
	buf = append(buf, int64BEBytes(0)...) // docsWithFieldLength
	buf = append(buf, int16BEBytes(0)...) // jumpTableEntryCount
	buf = append(buf, byte(0))            // denseRankPower
	buf = append(buf, int32BEBytes(1)...) // numDocsWithField
	buf = append(buf, byte(3))            // bytesPerNorm=3 (invalid)
	buf = append(buf, int64BEBytes(0)...) // normsOffset

	fi := buildNormsFieldInfo(7, "myField")
	infos := buildSingleFieldInfos(fi)

	p := &Lucene80NormsProducer{norms: make(map[int]*lucene80NormsEntry)}
	in := newBEIndexInput(buf)
	if err := p.readFields(in, "testSeg", infos); err == nil {
		t.Error("expected error for invalid bytesPerNorm=3")
	}
}

// --- helpers ----------------------------------------------------------------

// buildNormsFieldInfo creates a FieldInfo that has norms enabled.
// HasNorms() returns true when the field is indexed (IndexOptionsDocs or higher)
// and OmitNorms is false (default).
func buildNormsFieldInfo(number int, name string) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeNone,
		IndexOptions:  index.IndexOptionsDocsAndFreqs,
	})
}

// buildSingleFieldInfos wraps a single FieldInfo in a FieldInfos.
func buildSingleFieldInfos(fi *index.FieldInfo) *index.FieldInfos {
	b := index.NewFieldInfosBuilder()
	b.Add(fi)
	return b.Build()
}
