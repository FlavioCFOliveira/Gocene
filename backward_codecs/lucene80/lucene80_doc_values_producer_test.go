// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"fmt"
	"testing"

	bcpacked "github.com/FlavioCFOliveira/Gocene/backward_codecs/packed"
)

// TestLucene80DVNumericEntry_ReadInto verifies that readNumericEntryInto
// correctly populates a lucene80DVNumericEntry from a byte stream.
// The byte layout mirrors Lucene80DocValuesProducer.readNumeric(IndexInput, NumericEntry).
func TestLucene80DVNumericEntry_ReadInto(t *testing.T) {
	// Build a synthetic stream for one numeric entry.
	//  docsWithFieldOffset  int64 = 100
	//  docsWithFieldLength  int64 = 200
	//  jumpTableEntryCount  int16 = 3
	//  denseRankPower       byte  = 2
	//  numValues            int64 = 50
	//  tableSize            int32 = 0   (no table)
	//  bitsPerValue         byte  = 8
	//  minValue             int64 = 0
	//  gcd                  int64 = 1
	//  valuesOffset         int64 = 1000
	//  valuesLength         int64 = 400
	//  valueJumpTableOffset int64 = -1
	buf := newInt64BEBuf(
		100, // docsWithFieldOffset
		200, // docsWithFieldLength
	)
	buf = append(buf, int16BEBytes(3)...)    // jumpTableEntryCount
	buf = append(buf, byte(2))               // denseRankPower
	buf = append(buf, int64BEBytes(50)...)   // numValues
	buf = append(buf, int32BEBytes(0)...)    // tableSize=0
	buf = append(buf, byte(8))               // bitsPerValue
	buf = append(buf, int64BEBytes(0)...)    // minValue
	buf = append(buf, int64BEBytes(1)...)    // gcd
	buf = append(buf, int64BEBytes(1000)...) // valuesOffset
	buf = append(buf, int64BEBytes(400)...)  // valuesLength
	buf = append(buf, int64BEBytes(-1)...)   // valueJumpTableOffset

	in := newBEIndexInput(buf)
	e := &lucene80DVNumericEntry{}
	if err := readNumericEntryInto(in, e); err != nil {
		t.Fatalf("readNumericEntryInto: %v", err)
	}
	if e.docsWithFieldOffset != 100 {
		t.Errorf("docsWithFieldOffset: got %d, want 100", e.docsWithFieldOffset)
	}
	if e.docsWithFieldLength != 200 {
		t.Errorf("docsWithFieldLength: got %d, want 200", e.docsWithFieldLength)
	}
	if e.jumpTableEntryCount != 3 {
		t.Errorf("jumpTableEntryCount: got %d, want 3", e.jumpTableEntryCount)
	}
	if e.denseRankPower != 2 {
		t.Errorf("denseRankPower: got %d, want 2", e.denseRankPower)
	}
	if e.numValues != 50 {
		t.Errorf("numValues: got %d, want 50", e.numValues)
	}
	if len(e.table) != 0 {
		t.Errorf("table should be empty when tableSize=0; got len=%d", len(e.table))
	}
	if e.bitsPerValue != 8 {
		t.Errorf("bitsPerValue: got %d, want 8", e.bitsPerValue)
	}
	if e.valuesOffset != 1000 {
		t.Errorf("valuesOffset: got %d, want 1000", e.valuesOffset)
	}
	if e.valuesLength != 400 {
		t.Errorf("valuesLength: got %d, want 400", e.valuesLength)
	}
	if e.valueJumpTableOffset != -1 {
		t.Errorf("valueJumpTableOffset: got %d, want -1", e.valueJumpTableOffset)
	}
}

// TestLucene80DVNumericEntry_WithTable verifies that readNumericEntryInto
// populates e.table when tableSize > 0.
func TestLucene80DVNumericEntry_WithTable(t *testing.T) {
	// tableSize = 2 → two int64 values in table.
	buf := newInt64BEBuf(100, 200)         // docsWithFieldOffset/Length
	buf = append(buf, int16BEBytes(0)...)  // jumpTableEntryCount
	buf = append(buf, byte(0))             // denseRankPower
	buf = append(buf, int64BEBytes(4)...)  // numValues
	buf = append(buf, int32BEBytes(2)...)  // tableSize=2
	buf = append(buf, int64BEBytes(11)...) // table[0]
	buf = append(buf, int64BEBytes(22)...) // table[1]
	buf = append(buf, byte(4))             // bitsPerValue
	buf = append(buf, int64BEBytes(0)...)  // minValue
	buf = append(buf, int64BEBytes(1)...)  // gcd
	buf = append(buf, int64BEBytes(0)...)  // valuesOffset
	buf = append(buf, int64BEBytes(0)...)  // valuesLength
	buf = append(buf, int64BEBytes(0)...)  // valueJumpTableOffset

	in := newBEIndexInput(buf)
	e := &lucene80DVNumericEntry{}
	if err := readNumericEntryInto(in, e); err != nil {
		t.Fatalf("readNumericEntryInto: %v", err)
	}
	if len(e.table) != 2 {
		t.Fatalf("table len: got %d, want 2", len(e.table))
	}
	if e.table[0] != 11 {
		t.Errorf("table[0]: got %d, want 11", e.table[0])
	}
	if e.table[1] != 22 {
		t.Errorf("table[1]: got %d, want 22", e.table[1])
	}
}

// TestLoadLegacyDirectMonotonicMeta_ZeroBlocks verifies that when numValues==0
// the meta has NumBlocks==0 and no I/O is performed.
func TestLoadLegacyDirectMonotonicMeta_ZeroBlocks(t *testing.T) {
	// Empty stream — no bytes should be read for 0 values.
	in := newBEIndexInput([]byte{})
	m, err := loadLegacyDirectMonotonicMeta(in, 0, 4)
	if err != nil {
		t.Fatalf("loadMeta: %v", err)
	}
	if m.NumBlocks != 0 {
		t.Errorf("NumBlocks: got %d, want 0", m.NumBlocks)
	}
}

// TestLoadLegacyDirectMonotonicMeta_OneBlock verifies that a single block's
// metadata is decoded correctly (min, avg as float32 bits, offset, bpv).
func TestLoadLegacyDirectMonotonicMeta_OneBlock(t *testing.T) {
	// Build stream for 1 block: min=5, avg bits=0 (0.0f), offset=20, bpv=4.
	buf := int64BEBytes(5)                 // min
	buf = append(buf, int32BEBytes(0)...)  // avg as IEEE float32 bits = 0.0
	buf = append(buf, int64BEBytes(20)...) // offset
	buf = append(buf, byte(4))             // bpv

	// numValues=8 with blockShift=3 → numBlocks = ceil(8/8) = 1.
	in := newBEIndexInput(buf)
	m, err := loadLegacyDirectMonotonicMeta(in, 8, 3)
	if err != nil {
		t.Fatalf("loadMeta: %v", err)
	}
	if m.NumBlocks != 1 {
		t.Errorf("NumBlocks: got %d, want 1", m.NumBlocks)
	}
	if m.Mins[0] != 5 {
		t.Errorf("Mins[0]: got %d, want 5", m.Mins[0])
	}
	if m.Avgs[0] != 0.0 {
		t.Errorf("Avgs[0]: got %v, want 0.0", m.Avgs[0])
	}
	if m.Offsets[0] != 20 {
		t.Errorf("Offsets[0]: got %d, want 20", m.Offsets[0])
	}
	if m.BPVs[0] != 4 {
		t.Errorf("BPVs[0]: got %d, want 4", m.BPVs[0])
	}
}

// TestNewLegacyDirectMonotonicMeta_NumBlocks verifies block-count calculation.
func TestNewLegacyDirectMonotonicMeta_NumBlocks(t *testing.T) {
	tests := []struct {
		numValues  int64
		blockShift int
		wantBlocks int
	}{
		{0, 4, 0},
		{1, 4, 1},
		{16, 4, 1}, // exact: 16 >> 4 = 1
		{17, 4, 2}, // 17 >> 4 = 1, but (1<<4)=16 < 17 → +1
		{32, 4, 2},
	}
	for _, tc := range tests {
		m := bcpacked.NewLegacyDirectMonotonicMeta(tc.numValues, tc.blockShift)
		if m.NumBlocks != tc.wantBlocks {
			t.Errorf("numValues=%d blockShift=%d: NumBlocks=%d, want %d",
				tc.numValues, tc.blockShift, m.NumBlocks, tc.wantBlocks)
		}
	}
}

// TestLucene80DocValuesProducer_Constants verifies key format constants.
func TestLucene80DocValuesProducer_Constants(t *testing.T) {
	if lucene80DVDataCodec != "Lucene80DocValuesData" {
		t.Errorf("dataCodec: got %q", lucene80DVDataCodec)
	}
	if lucene80DVMetaCodec != "Lucene80DocValuesMetadata" {
		t.Errorf("metaCodec: got %q", lucene80DVMetaCodec)
	}
	if lucene80VersionCurrent != 2 {
		t.Errorf("VERSION_CURRENT: got %d, want 2", lucene80VersionCurrent)
	}
	if lucene80DVNumeric != 0 {
		t.Errorf("NUMERIC: got %d", lucene80DVNumeric)
	}
	if lucene80DVBinary != 1 {
		t.Errorf("BINARY: got %d", lucene80DVBinary)
	}
}

// TestLucene80DocValuesProducer_ClosedGetNumeric verifies that Get* after
// Close returns an error.
func TestLucene80DocValuesProducer_ClosedGetNumeric(t *testing.T) {
	p := &Lucene80DocValuesProducer{closed: true}
	if _, err := p.GetNumeric(nil); err == nil {
		t.Error("GetNumeric on closed producer should return error")
	}
}

// TestLucene80DocValuesProducer_NilDataClose verifies that Close on a producer
// with no data file does not panic.
func TestLucene80DocValuesProducer_NilDataClose(t *testing.T) {
	p := &Lucene80DocValuesProducer{}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}
}

// --- in-package helpers for building big-endian byte streams ----------------

func int64BEBytes(v int64) []byte {
	return []byte{
		byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32),
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	}
}

func int32BEBytes(v int32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func int16BEBytes(v int16) []byte {
	return []byte{byte(v >> 8), byte(v)}
}

// newInt64BEBuf builds a byte slice from a variadic list of int64 values,
// big-endian encoded.
func newInt64BEBuf(vals ...int64) []byte {
	var buf []byte
	for _, v := range vals {
		buf = append(buf, int64BEBytes(v)...)
	}
	return buf
}

// beIndexInput is a minimal store.DataInput that reads big-endian integers
// from a byte slice.  It is used only in package-level tests.
type beIndexInput struct {
	data []byte
	pos  int
}

func newBEIndexInput(data []byte) *beIndexInput {
	return &beIndexInput{data: data}
}

func (b *beIndexInput) ReadByte() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, fmt.Errorf("EOF")
	}
	v := b.data[b.pos]
	b.pos++
	return v, nil
}

func (b *beIndexInput) ReadBytes(dst []byte) error {
	for i := range dst {
		v, err := b.ReadByte()
		if err != nil {
			return err
		}
		dst[i] = v
	}
	return nil
}

func (b *beIndexInput) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := b.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort reads a big-endian int16.
func (b *beIndexInput) ReadShort() (int16, error) {
	hi, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	lo, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	return int16(hi)<<8 | int16(lo), nil
}

// ReadInt reads a big-endian int32.
func (b *beIndexInput) ReadInt() (int32, error) {
	var buf [4]byte
	if err := b.ReadBytes(buf[:]); err != nil {
		return 0, err
	}
	return int32(buf[0])<<24 | int32(buf[1])<<16 | int32(buf[2])<<8 | int32(buf[3]), nil
}

// ReadLong reads a big-endian int64.
func (b *beIndexInput) ReadLong() (int64, error) {
	hi, err := b.ReadInt()
	if err != nil {
		return 0, err
	}
	lo, err := b.ReadInt()
	if err != nil {
		return 0, err
	}
	return int64(hi)<<32 | int64(uint32(lo)), nil
}

func (b *beIndexInput) ReadString() (string, error) {
	n, err := b.ReadVInt()
	if err != nil {
		return "", err
	}
	raw, err := b.ReadBytesN(int(n))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ReadVInt reads a variable-length int32 (little-endian VInt encoding).
func (b *beIndexInput) ReadVInt() (int32, error) {
	// Lucene VInt: LSB first, 7 bits per byte, MSB is continuation bit.
	var v int32
	shift := 0
	for {
		byt, err := b.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= int32(byt&0x7F) << shift
		if byt&0x80 == 0 {
			break
		}
		shift += 7
	}
	return v, nil
}

// ReadVLong reads a variable-length int64.
func (b *beIndexInput) ReadVLong() (int64, error) {
	var v int64
	shift := 0
	for {
		byt, err := b.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= int64(byt&0x7F) << shift
		if byt&0x80 == 0 {
			break
		}
		shift += 7
	}
	return v, nil
}

// GetFilePointer returns the current read position.
func (b *beIndexInput) GetFilePointer() int64 { return int64(b.pos) }

// Length returns the total length of the input.
func (b *beIndexInput) Length() int64 { return int64(len(b.data)) }

// SetPosition moves the read cursor.
func (b *beIndexInput) SetPosition(pos int64) error { b.pos = int(pos); return nil }
