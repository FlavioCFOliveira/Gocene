package codecs

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// roundTripStoredFieldsInts encodes values via WriteStoredFieldsInts using a
// ByteArrayDataOutput, decodes the bytes via ReadStoredFieldsInts using a
// ByteArrayDataInput, and returns the decoded slice for comparison. Both
// transports are little-endian and self-consistent in this repo.
func roundTripStoredFieldsInts(t *testing.T, values []int32) []int64 {
	t.Helper()

	out := store.NewByteArrayDataOutput(0)
	if err := WriteStoredFieldsInts(values, 0, len(values), out); err != nil {
		t.Fatalf("WriteStoredFieldsInts: %v", err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	got := make([]int64, len(values))
	if err := ReadStoredFieldsInts(in, len(values), got, 0); err != nil {
		t.Fatalf("ReadStoredFieldsInts: %v", err)
	}
	return got
}

// expectedUnsigned widens int32 values to int64 using unsigned semantics —
// this mirrors what the reader does (and what the Java reference does
// implicitly via the long[] target with Byte/Short/Integer.toUnsignedX
// helpers, except for the all-equal branch which preserves VInt's signed
// extension).
func expectedUnsigned(values []int32) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = int64(uint32(v))
	}
	return out
}

func TestStoredFieldsInts_AllEqual_Zero(t *testing.T) {
	// All-equal with count == 0 still emits header 0 + VInt(values[0]).
	values := []int32{0}
	got := roundTripStoredFieldsInts(t, values)
	want := []int64{0}
	if got[0] != want[0] {
		t.Fatalf("all-equal single zero: got %v want %v", got, want)
	}
}

func TestStoredFieldsInts_AllEqual_Positive(t *testing.T) {
	values := []int32{42, 42, 42, 42, 42, 42, 42, 42}
	got := roundTripStoredFieldsInts(t, values)
	for i, g := range got {
		if g != 42 {
			t.Fatalf("all-equal pos at %d: got %d want 42", i, g)
		}
	}
}

func TestStoredFieldsInts_8bpv_TailOnly(t *testing.T) {
	// Fewer than BlockSize values, all <= 0xff but not all equal → 8bpv tail.
	values := []int32{1, 2, 3, 0xff, 0, 17, 42, 250, 7}
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("8bpv tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_8bpv_BlockPlusTail(t *testing.T) {
	// BlockSize + a few tail values, all within 0..255 but not all equal.
	rng := rand.New(rand.NewPCG(0xC0DE, 0xBEEF))
	values := make([]int32, storedFieldsIntsBlockSize+5)
	for i := range values {
		values[i] = int32(rng.UintN(0x100))
	}
	values[0], values[1] = 0, 255 // ensure not all-equal and exercise extremes
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("8bpv block+tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_16bpv_TailOnly(t *testing.T) {
	values := []int32{1, 0x100, 0xffff, 0x1234, 0xabcd, 0, 7}
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("16bpv tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_16bpv_BlockPlusTail(t *testing.T) {
	rng := rand.New(rand.NewPCG(0xCAFE, 0xF00D))
	values := make([]int32, storedFieldsIntsBlockSize+9)
	for i := range values {
		values[i] = int32(rng.UintN(0x10000))
	}
	// Force a value > 0xff somewhere to lock-in 16bpv selection, and a
	// non-equal pair to skip the all-equal branch.
	values[0] = 0x100
	values[1] = 0xffff
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("16bpv block+tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_32bpv_TailOnly(t *testing.T) {
	values := []int32{1, 0x10000, 0x7fffffff, -1, 0}
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("32bpv tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_32bpv_BlockPlusTail(t *testing.T) {
	rng := rand.New(rand.NewPCG(0xABCD, 0x1234))
	values := make([]int32, storedFieldsIntsBlockSize+13)
	for i := range values {
		values[i] = int32(rng.Uint32())
	}
	// Guarantee we cross the 16-bit ceiling and have a negative (high-bit
	// set) value so that the unsigned-max computation selects 32bpv.
	values[0] = 0x10000
	values[1] = -42
	got := roundTripStoredFieldsInts(t, values)
	want := expectedUnsigned(values)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("32bpv block+tail mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestStoredFieldsInts_OffsetWindow(t *testing.T) {
	// Use start/offset != 0 on both ends. The values outside the window
	// must be ignored on encode and untouched on decode.
	values := []int32{99, 99, 99, // unused prefix
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		99, 99, // unused suffix
	}
	start, count := 3, 10

	out := store.NewByteArrayDataOutput(0)
	if err := WriteStoredFieldsInts(values, start, count, out); err != nil {
		t.Fatalf("WriteStoredFieldsInts: %v", err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	dst := make([]int64, count+4)
	for i := range dst {
		dst[i] = -111
	}
	if err := ReadStoredFieldsInts(in, count, dst, 2); err != nil {
		t.Fatalf("ReadStoredFieldsInts: %v", err)
	}
	for i := 0; i < 2; i++ {
		if dst[i] != -111 {
			t.Fatalf("offset prefix clobbered at %d: %d", i, dst[i])
		}
	}
	for i := 0; i < count; i++ {
		want := int64(values[start+i])
		if dst[2+i] != want {
			t.Fatalf("offset window mismatch at %d: got %d want %d", i, dst[2+i], want)
		}
	}
	for i := 2 + count; i < len(dst); i++ {
		if dst[i] != -111 {
			t.Fatalf("offset suffix clobbered at %d: %d", i, dst[i])
		}
	}
}

func TestStoredFieldsInts_UnsupportedBPV(t *testing.T) {
	// Hand-craft an input with a header byte that is not one of {0, 8, 16, 32}.
	in := store.NewByteArrayDataInput([]byte{7})
	dst := make([]int64, 4)
	err := ReadStoredFieldsInts(in, 4, dst, 0)
	if err == nil {
		t.Fatalf("expected error for unsupported bpv, got nil")
	}
	if !errors.Is(err, ErrStoredFieldsIntsUnsupportedBPV) {
		t.Fatalf("expected ErrStoredFieldsIntsUnsupportedBPV, got %v", err)
	}
}
