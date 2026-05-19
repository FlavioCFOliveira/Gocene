// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestGroupVIntFixture pins the on-wire format for the input [0, 1, 256, 65536].
// Payload byte-lengths are 1,1,2,3 (one byte for 0, one byte for 1, two bytes
// for 256, three bytes for 65536). With the Lucene packing
// (n1m1 << 6) | (n2m1 << 4) | (n3m1 << 2) | n4m1, the control byte is
// (0 << 6) | (0 << 4) | (1 << 2) | 2 = 0x06. Payloads follow as
// 0x00, 0x01, 0x00 0x01, 0x00 0x00 0x01 (little-endian).
func TestGroupVIntFixture(t *testing.T) {
	values := []int32{0, 1, 256, 65536}
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(32)
	if err := WriteGroupVInts(out, scratch, values, len(values)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := out.GetBytes()
	want := []byte{0x06, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded bytes: got % x want % x", got, want)
	}

	in := store.NewByteArrayDataInput(got)
	dst := make([]int32, 4)
	if err := ReadGroupVInts(in, dst, 4); err != nil {
		t.Fatalf("read: %v", err)
	}
	for i, v := range values {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %d want %d", i, dst[i], v)
		}
	}
}

// TestGroupVIntAllMax exercises the worst-case path where every value
// requires the full 4 bytes (control byte = 0xFF, total 17 bytes).
func TestGroupVIntAllMax(t *testing.T) {
	values := []int32{-1, -1, -1, -1} // 0xFFFFFFFF as signed int32
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(32)
	if err := WriteGroupVInts(out, scratch, values, len(values)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := out.GetBytes()
	if got[0] != 0xFF {
		t.Fatalf("control byte got 0x%02x want 0xFF", got[0])
	}
	if len(got) != GroupVIntMaxLengthPerGroup {
		t.Fatalf("encoded length got %d want %d", len(got), GroupVIntMaxLengthPerGroup)
	}
	in := store.NewByteArrayDataInput(got)
	dst := make([]int32, 4)
	if err := ReadGroupVInts(in, dst, 4); err != nil {
		t.Fatalf("read: %v", err)
	}
	for i, v := range values {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %x want %x", i, uint32(dst[i]), uint32(v))
		}
	}
}

// TestGroupVIntAllMin exercises the best-case path where every value
// fits in a single byte (control byte = 0x00, total 5 bytes).
func TestGroupVIntAllMin(t *testing.T) {
	values := []int32{0, 1, 127, 255}
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(32)
	if err := WriteGroupVInts(out, scratch, values, len(values)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := out.GetBytes()
	if got[0] != 0x00 {
		t.Fatalf("control byte got 0x%02x want 0x00", got[0])
	}
	if len(got) != 5 {
		t.Fatalf("encoded length got %d want 5", len(got))
	}
	in := store.NewByteArrayDataInput(got)
	dst := make([]int32, 4)
	if err := ReadGroupVInts(in, dst, 4); err != nil {
		t.Fatalf("read: %v", err)
	}
	for i, v := range values {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %d want %d", i, dst[i], v)
		}
	}
}

// TestGroupVIntTailVInts verifies that counts not divisible by four emit the
// trailing values as regular VInts.
func TestGroupVIntTailVInts(t *testing.T) {
	// 6 values: one full group of 4 + 2 tail VInts.
	values := []int32{1, 2, 3, 4, 200, 300}
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(32)
	if err := WriteGroupVInts(out, scratch, values, len(values)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := out.GetBytes()
	// Group: control 0x00, payloads 1,2,3,4 = 5 bytes.
	// VInt 200 = 0xC8 0x01, VInt 300 = 0xAC 0x02. Total 5 + 2 + 2 = 9.
	want := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xC8, 0x01, 0xAC, 0x02}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded: got % x want % x", got, want)
	}

	in := store.NewByteArrayDataInput(got)
	dst := make([]int32, len(values))
	if err := ReadGroupVInts(in, dst, len(values)); err != nil {
		t.Fatalf("read: %v", err)
	}
	for i, v := range values {
		if dst[i] != v {
			t.Fatalf("dst[%d] got %d want %d", i, dst[i], v)
		}
	}
}

// TestGroupVIntRandomRoundTrip drives many groups (mixed full + tail) and
// checks bit-exact round-trip on both ReadGroupVInts and the baseline path.
func TestGroupVIntRandomRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0xdeadbeef))
	const total = 4097 // not a multiple of 4: forces tail handling
	src := make([]int32, total)
	for i := range src {
		bitsN := rng.Intn(32) + 1
		mask := uint32(1)<<uint(bitsN) - 1
		src[i] = int32(rng.Uint32() & mask)
	}

	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(total * 5)
	if err := WriteGroupVInts(out, scratch, src, total); err != nil {
		t.Fatalf("write: %v", err)
	}
	encoded := append([]byte(nil), out.GetBytes()...)

	// Fast path is not triggered for ByteArrayDataInput (it is not an
	// IndexInput), so this exercises the slow path identical to baseline.
	dst := make([]int32, total)
	in := store.NewByteArrayDataInput(encoded)
	if err := ReadGroupVInts(in, dst, total); err != nil {
		t.Fatalf("read: %v", err)
	}
	for i := range src {
		if dst[i] != src[i] {
			t.Fatalf("round-trip mismatch at %d: got %d want %d", i, dst[i], src[i])
		}
	}

	// Baseline variant must produce the same decoded sequence.
	dst2 := make([]int32, total)
	in2 := store.NewByteArrayDataInput(encoded)
	if err := ReadGroupVIntsBaseline(in2, dst2, total); err != nil {
		t.Fatalf("baseline read: %v", err)
	}
	for i := range src {
		if dst2[i] != src[i] {
			t.Fatalf("baseline mismatch at %d: got %d want %d", i, dst2[i], src[i])
		}
	}
}

// TestGroupVIntInt64RoundTrip pins the deprecated long[]-flavoured variants.
func TestGroupVIntInt64RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0xc0ffee))
	const total = 17 // 4 groups + 1 tail
	src := make([]int64, total)
	for i := range src {
		src[i] = int64(rng.Uint32())
	}
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	out := store.NewByteArrayDataOutput(total * 5)
	if err := WriteGroupVIntsInt64(out, scratch, src, total); err != nil {
		t.Fatalf("write64: %v", err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	dst := make([]int64, total)
	if err := ReadGroupVIntsInt64(in, dst, total); err != nil {
		t.Fatalf("read64: %v", err)
	}
	for i := range src {
		if dst[i] != src[i] {
			t.Fatalf("round-trip mismatch at %d: got %d want %d", i, dst[i], src[i])
		}
	}
}

// TestGroupVIntInt64Overflow verifies that values not fitting in uint32 are
// rejected by the deprecated long[] writer.
func TestGroupVIntInt64Overflow(t *testing.T) {
	out := store.NewByteArrayDataOutput(32)
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	bad := []int64{1, 2, 3, 0x1_0000_0000}
	err := WriteGroupVIntsInt64(out, scratch, bad, len(bad))
	if !errors.Is(err, ErrGroupVIntOverflow) {
		t.Fatalf("got err=%v want ErrGroupVIntOverflow", err)
	}
}

// indexRandomAccess satisfies both store.IndexInput and store.RandomAccessInput
// so we can exercise the branch-less fast path in readGroupVInt32.
type indexRandomAccess struct {
	*store.ByteArrayRandomAccessInput
	pos int64
}

func (i *indexRandomAccess) ReadByte() (byte, error) {
	b, err := i.ReadByteAt(i.pos)
	if err != nil {
		return 0, err
	}
	i.pos++
	return b, nil
}

func (i *indexRandomAccess) ReadBytes(b []byte) error {
	for k := range b {
		x, err := i.ReadByte()
		if err != nil {
			return err
		}
		b[k] = x
	}
	return nil
}

func (i *indexRandomAccess) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := i.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (i *indexRandomAccess) ReadShort() (int16, error) {
	v, err := i.ReadShortAt(i.pos)
	if err != nil {
		return 0, err
	}
	i.pos += 2
	return v, nil
}

func (i *indexRandomAccess) ReadInt() (int32, error) {
	v, err := i.ReadIntAt(i.pos)
	if err != nil {
		return 0, err
	}
	i.pos += 4
	return v, nil
}

func (i *indexRandomAccess) ReadLong() (int64, error) {
	v, err := i.ReadLongAt(i.pos)
	if err != nil {
		return 0, err
	}
	i.pos += 8
	return v, nil
}

func (i *indexRandomAccess) ReadString() (string, error) {
	return "", errors.New("not used")
}

func (i *indexRandomAccess) GetFilePointer() int64       { return i.pos }
func (i *indexRandomAccess) SetPosition(pos int64) error { i.pos = pos; return nil }
func (i *indexRandomAccess) Length() int64               { return i.ByteArrayRandomAccessInput.Length() }
func (i *indexRandomAccess) Clone() store.IndexInput     { c := *i; return &c }
func (i *indexRandomAccess) Close() error                { return nil }
func (i *indexRandomAccess) Slice(string, int64, int64) (store.IndexInput, error) {
	return nil, errors.New("not used")
}

// TestGroupVIntFastPath drives the random-access fast path explicitly. The
// concrete input satisfies both IndexInput and RandomAccessInput and has at
// least 16 bytes of trailing room (the group payload + a 4-byte sentinel),
// so the branch-less decode path runs.
func TestGroupVIntFastPath(t *testing.T) {
	values := []int32{1, 256, 65536, 1 << 24}
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)
	bufOut := store.NewByteArrayDataOutput(32)
	if err := WriteGroupVInts(bufOut, scratch, values, len(values)); err != nil {
		t.Fatalf("write: %v", err)
	}
	encoded := bufOut.GetBytes()
	// Pad with 16 zero bytes so the fast-path "length - pos >= 16" check
	// always succeeds while reading the single group at offset 0.
	padded := append(append([]byte(nil), encoded...), make([]byte, 16)...)
	in := &indexRandomAccess{
		ByteArrayRandomAccessInput: store.NewByteArrayRandomAccessInput(padded),
	}
	dst := make([]int32, 4)
	if err := ReadGroupVInts(in, dst, 4); err != nil {
		t.Fatalf("fast read: %v", err)
	}
	for i, v := range values {
		if dst[i] != v {
			t.Fatalf("fast[%d] got %d want %d", i, dst[i], v)
		}
	}
	if got := in.GetFilePointer(); got != int64(len(encoded)) {
		t.Fatalf("file pointer after fast read got %d want %d", got, len(encoded))
	}
}

// TestGroupVIntErrors covers the parameter-validation branches of the public
// surface.
func TestGroupVIntErrors(t *testing.T) {
	scratch := make([]byte, GroupVIntMaxLengthPerGroup)

	t.Run("write negative limit", func(t *testing.T) {
		out := store.NewByteArrayDataOutput(32)
		if err := WriteGroupVInts(out, scratch, []int32{1, 2, 3, 4}, -1); err == nil {
			t.Fatalf("expected error for negative limit")
		}
	})
	t.Run("write limit beyond values", func(t *testing.T) {
		out := store.NewByteArrayDataOutput(32)
		if err := WriteGroupVInts(out, scratch, []int32{1, 2}, 4); err == nil {
			t.Fatalf("expected error for limit > len(values)")
		}
	})
	t.Run("write scratch too small", func(t *testing.T) {
		out := store.NewByteArrayDataOutput(32)
		small := make([]byte, 4)
		if err := WriteGroupVInts(out, small, []int32{1, 2, 3, 4}, 4); err == nil {
			t.Fatalf("expected error for small scratch")
		}
	})
	t.Run("read negative limit", func(t *testing.T) {
		in := store.NewByteArrayDataInput([]byte{0})
		dst := make([]int32, 4)
		if err := ReadGroupVInts(in, dst, -1); err == nil {
			t.Fatalf("expected error for negative limit")
		}
	})
	t.Run("read limit beyond dst", func(t *testing.T) {
		in := store.NewByteArrayDataInput([]byte{0, 1, 2, 3, 4})
		dst := make([]int32, 2)
		if err := ReadGroupVInts(in, dst, 4); err == nil {
			t.Fatalf("expected error for limit > len(dst)")
		}
	})
	t.Run("read truncated source", func(t *testing.T) {
		// Control byte says four 1-byte values (5 bytes total), but only 3 present.
		in := store.NewByteArrayDataInput([]byte{0x00, 0x01, 0x02})
		dst := make([]int32, 4)
		if err := ReadGroupVInts(in, dst, 4); err == nil {
			t.Fatalf("expected error for truncated source")
		}
	})
	t.Run("custom overload is unsupported", func(t *testing.T) {
		in := store.NewByteArrayDataInput(nil)
		dst := make([]int32, 4)
		if _, err := ReadGroupVIntCustom(in, 0, nil, 0, dst, 0); !errors.Is(err, ErrReadGroupVIntCustom) {
			t.Fatalf("got %v want ErrReadGroupVIntCustom", err)
		}
	})
}

// TestNumBytes pins the numBytes table for boundary values.
func TestNumBytes(t *testing.T) {
	cases := []struct {
		v    uint32
		want int
	}{
		{0, 1},
		{1, 1},
		{0xFF, 1},
		{0x100, 2},
		{0xFFFF, 2},
		{0x10000, 3},
		{0xFFFFFF, 3},
		{0x1000000, 4},
		{0xFFFFFFFF, 4},
	}
	for _, c := range cases {
		if got := numBytes(c.v); got != c.want {
			t.Fatalf("numBytes(0x%x) got %d want %d", c.v, got, c.want)
		}
	}
}

// TestToInt32 pins the overflow check used by the deprecated long[]
// variants.
func TestToInt32(t *testing.T) {
	if v, err := ToInt32(0); err != nil || v != 0 {
		t.Fatalf("ToInt32(0) got (%d,%v)", v, err)
	}
	if v, err := ToInt32(0xFFFFFFFF); err != nil || uint32(v) != 0xFFFFFFFF {
		t.Fatalf("ToInt32(max) got (%d,%v)", v, err)
	}
	if _, err := ToInt32(0x1_0000_0000); !errors.Is(err, ErrGroupVIntOverflow) {
		t.Fatalf("ToInt32(2^32) got err=%v want ErrGroupVIntOverflow", err)
	}
}
