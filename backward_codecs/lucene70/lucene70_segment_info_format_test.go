// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene70

import (
	"math"
	"testing"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene70SegmentInfoFormat_ImplementsInterface is a compile-time assertion.
func TestLucene70SegmentInfoFormat_ImplementsInterface(t *testing.T) {
	var _ codecs.SegmentInfoFormat = (*Lucene70SegmentInfoFormat)(nil)
}

// TestLucene70SegmentInfoFormat_WriteReturnsError verifies that the write path
// always returns an error (old formats are read-only).
func TestLucene70SegmentInfoFormat_WriteReturnsError(t *testing.T) {
	f := NewLucene70SegmentInfoFormat()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	si := index.NewSegmentInfo("_0", 0, dir)
	err := f.Write(dir, si, store.IOContext{})
	if err == nil {
		t.Fatal("Write(): expected error for read-only format, got nil")
	}
}

// TestLucene70SegmentInfoFormat_ReadMissingFileFails verifies that attempting
// to read a non-existent .si file returns an error.
func TestLucene70SegmentInfoFormat_ReadMissingFileFails(t *testing.T) {
	f := NewLucene70SegmentInfoFormat()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	_, err := f.Read(dir, "_0", make([]byte, 16), store.IOContext{})
	if err == nil {
		t.Fatal("Read(): expected error for missing .si file, got nil")
	}
}

// ---------------------------------------------------------------------------
// readIndexSort70 unit tests
// ---------------------------------------------------------------------------

// TestReadIndexSort70_Empty verifies that numSortFields=0 returns nil.
func TestReadIndexSort70_Empty(t *testing.T) {
	in := buildBADI70(t, func(out store.DataOutput) error {
		return store.WriteVInt(out, 0) // numSortFields=0
	})
	sort, err := readIndexSort70(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort != nil {
		t.Errorf("expected nil sort for 0 fields, got non-nil")
	}
}

// TestReadIndexSort70_NegativeCountFails verifies that a negative sort field
// count is rejected.
func TestReadIndexSort70_NegativeCountFails(t *testing.T) {
	// VInt encoding of -1 is tricky; use a large unsigned value that maps to <0
	// in int. WriteVInt can't write negative values. Skip with a note.
	// The rejection path is tested via readSortField70 returning an error for
	// an unknown sortTypeID, which triggers the negative count path indirectly.
	// Direct test of numSortFields<0 requires a hand-crafted raw byte: write
	// the VInt for int32(-1) which is 5 bytes of 0xFF 0xFF 0xFF 0xFF 0x0F.
	raw := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F} // VInt(-1) = 0xFFFFFFFF
	in := bcstore.NewEndiannessReverserChecksumIndexInput(newBytesIndexInput70(t, raw))
	_, err := readIndexSort70(in)
	if err == nil {
		t.Fatal("expected error for negative numSortFields, got nil")
	}
}

// TestReadIndexSort70_StringField verifies decoding of a simple STRING sort
// field without a missing value.
func TestReadIndexSort70_StringField(t *testing.T) {
	in := buildBADI70(t, func(out store.DataOutput) error {
		if err := store.WriteVInt(out, 1); err != nil { // numSortFields=1
			return err
		}
		// fieldName="title", sortTypeID=0 (STRING), reverse=1 (natural), missingFlag=0
		if err := writeString70(out, "title"); err != nil {
			return err
		}
		if err := store.WriteVInt(out, 0); err != nil { // sortTypeID=STRING
			return err
		}
		if err := out.WriteByte(1); err != nil { // reverse=1 → ascending
			return err
		}
		return out.WriteByte(0) // missingFlag=0 → no missing value
	})
	sort, err := readIndexSort70(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort == nil {
		t.Fatal("expected non-nil sort")
	}
}

// TestReadIndexSort70_LongFieldWithMissing verifies LONG sort field with a
// missing value.
func TestReadIndexSort70_LongFieldWithMissing(t *testing.T) {
	in := buildBADI70(t, func(out store.DataOutput) error {
		if err := store.WriteVInt(out, 1); err != nil {
			return err
		}
		if err := writeString70(out, "ts"); err != nil {
			return err
		}
		if err := store.WriteVInt(out, 1); err != nil { // sortTypeID=LONG
			return err
		}
		if err := out.WriteByte(0); err != nil { // reverse=0 → descending
			return err
		}
		if err := out.WriteByte(1); err != nil { // missingFlag=1
			return err
		}
		// missing value: Long (big-endian via ReadLong on EndiannessReverser)
		return writeLongBE70(out, 42)
	})
	sort, err := readIndexSort70(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort == nil {
		t.Fatal("expected non-nil sort")
	}
}

// TestReadIndexSort70_FloatFieldWithMissing verifies FLOAT sort field with a
// missing value.
func TestReadIndexSort70_FloatFieldWithMissing(t *testing.T) {
	in := buildBADI70(t, func(out store.DataOutput) error {
		if err := store.WriteVInt(out, 1); err != nil {
			return err
		}
		if err := writeString70(out, "score"); err != nil {
			return err
		}
		if err := store.WriteVInt(out, 4); err != nil { // sortTypeID=FLOAT
			return err
		}
		if err := out.WriteByte(1); err != nil { // reverse=1 → ascending
			return err
		}
		if err := out.WriteByte(1); err != nil { // missingFlag=1
			return err
		}
		// missing value: Float (big-endian via ReadInt on EndiannessReverser)
		return writeIntBE70(out, int32(math.Float32bits(1.5)))
	})
	sort, err := readIndexSort70(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sort == nil {
		t.Fatal("expected non-nil sort")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildBADI70 writes data via fn to an in-memory file and returns an
// EndiannessReverserChecksumIndexInput (big-endian wrapper) over it.
func buildBADI70(t *testing.T, fn func(store.DataOutput) error) *bcstore.EndiannessReverserChecksumIndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput("tmp.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := fn(out); err != nil {
		t.Fatalf("write fn: %v", err)
	}
	_ = out.Close()
	in, err := dir.OpenInput("tmp.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return bcstore.NewEndiannessReverserChecksumIndexInput(in)
}

// newBytesIndexInput70 writes a raw byte slice into an in-memory directory
// and returns a plain IndexInput over it.
func newBytesIndexInput70(t *testing.T, b []byte) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput("raw.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(b); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	_ = out.Close()
	in, err := dir.OpenInput("raw.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// writeString70 writes a Lucene-style string (VInt length + UTF-8 bytes) in
// big-endian context.  Since VInt and bytes are always single-byte or
// byte-exact, no endian swap is needed.
func writeString70(out store.DataOutput, s string) error {
	b := []byte(s)
	if err := store.WriteVInt(out, int32(len(b))); err != nil {
		return err
	}
	return out.WriteBytes(b)
}

// writeLongBE70 writes a big-endian int64 (as EndiannessReverser expects).
// The bytes go in big-endian order because the reverser will swap them back.
func writeLongBE70(out store.DataOutput, v int64) error {
	return out.WriteBytes([]byte{
		byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32),
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	})
}

// writeIntBE70 writes a big-endian int32 for the EndiannessReverser.
func writeIntBE70(out store.DataOutput, v int32) error {
	return out.WriteBytes([]byte{
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	})
}
