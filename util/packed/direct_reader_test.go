// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"encoding/binary"
	"errors"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// byteSliceRandomAccess adapts a byte slice into the RandomAccessInput
// surface used by DirectReader, so DirectWriter's output bytes can be
// fed straight back to DirectReader for round-trip tests.
type byteSliceRandomAccess struct {
	data []byte
}

func (s *byteSliceRandomAccess) ReadByteAt(pos int64) (byte, error) {
	return s.data[pos], nil
}

func (s *byteSliceRandomAccess) ReadShortAt(pos int64) (int16, error) {
	return int16(binary.LittleEndian.Uint16(s.data[pos:])), nil
}

func (s *byteSliceRandomAccess) ReadIntAt(pos int64) (int32, error) {
	return int32(binary.LittleEndian.Uint32(s.data[pos:])), nil
}

func (s *byteSliceRandomAccess) ReadLongAt(pos int64) (int64, error) {
	return int64(binary.LittleEndian.Uint64(s.data[pos:])), nil
}

// TestDirectReaderRoundTrip verifies that DirectReader recovers every
// value written by DirectWriter across the supported bitsPerValue
// spectrum.
func TestDirectReaderRoundTrip(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range directBitsPerValueSpectrum {
		r := rand.New(rand.NewSource(int64(bpv) * 7919))
		values := make([]int64, n)
		mask := uint64(MaxValue(bpv))
		for i := range values {
			if bpv == 64 {
				values[i] = int64(r.Uint64())
			} else {
				values[i] = int64(r.Uint64() & mask)
			}
		}

		out := store.NewByteArrayDataOutput(64)
		w, err := GetDirectWriter(out, n, bpv)
		if err != nil {
			t.Fatalf("GetDirectWriter bpv=%d: %v", bpv, err)
		}
		for _, v := range values {
			if err := w.Add(v); err != nil {
				t.Fatalf("Add bpv=%d: %v", bpv, err)
			}
		}
		if err := w.Finish(); err != nil {
			t.Fatalf("Finish bpv=%d: %v", bpv, err)
		}

		in := &byteSliceRandomAccess{data: out.GetBytes()}
		reader, err := GetDirectReader(in, bpv)
		if err != nil {
			t.Fatalf("GetDirectReader bpv=%d: %v", bpv, err)
		}
		for i, want := range values {
			got, err := reader.Get(int64(i))
			if err != nil {
				t.Fatalf("Get bpv=%d [%d]: unexpected error: %v", bpv, i, err)
			}
			if got != want {
				t.Fatalf("bpv=%d [%d]: got %d want %d", bpv, i, got, want)
			}
		}
	}
}

// TestDirectReaderRejectsInvalidBitsPerValue mirrors the writer test:
// DirectReader must only accept the supported widths.
func TestDirectReaderRejectsInvalidBitsPerValue(t *testing.T) {
	t.Parallel()
	in := &byteSliceRandomAccess{data: make([]byte, 64)}
	for _, bpv := range []int{0, 3, 5, 6, 7, 9, 10, 11, 13, 14, 15, 30, 33, 65} {
		if _, err := GetDirectReader(in, bpv); err == nil {
			t.Errorf("bpv=%d: expected error, got nil", bpv)
		}
	}
}

// TestDirectReaderAt verifies that the byte offset parameter shifts
// the read window correctly.
func TestDirectReaderAt(t *testing.T) {
	t.Parallel()
	// Write a 4-byte prefix, then encode {1, 2, 3, 4} with bpv=8.
	out := store.NewByteArrayDataOutput(8)
	for _, p := range []byte{0xAA, 0xBB, 0xCC, 0xDD} {
		_ = out.WriteByte(p)
	}
	w, err := GetDirectWriter(out, 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []int64{1, 2, 3, 4} {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}

	in := &byteSliceRandomAccess{data: out.GetBytes()}
	reader, err := GetDirectReaderAt(in, 8, 4) // skip the 4-byte prefix
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{1, 2, 3, 4} {
		got, err := reader.Get(int64(i))
		if err != nil {
			t.Fatalf("[%d]: unexpected error: %v", i, err)
		}
		if got != want {
			t.Errorf("[%d]: got %d want %d", i, got, want)
		}
	}
}

// errRandomAccess is a RandomAccessInput that returns an error on every read.
type errRandomAccess struct{}

func (errRandomAccess) ReadByteAt(int64) (byte, error) { return 0, errors.New("read error") }
func (errRandomAccess) ReadShortAt(int64) (int16, error) { return 0, errors.New("read error") }
func (errRandomAccess) ReadIntAt(int64) (int32, error)   { return 0, errors.New("read error") }
func (errRandomAccess) ReadLongAt(int64) (int64, error)  { return 0, errors.New("read error") }

// TestDirectReader_Get_CorruptInput_ReturnsError verifies that every
// supported bpv returns an error from Get (instead of panicking) when
// the backing RandomAccessInput fails.
func TestDirectReader_Get_CorruptInput_ReturnsError(t *testing.T) {
	t.Parallel()
	for _, bpv := range directBitsPerValueSpectrum {
		r, err := GetDirectReader(errRandomAccess{}, bpv)
		if err != nil {
			t.Fatalf("GetDirectReader bpv=%d: %v", bpv, err)
		}
		_, err = r.Get(0)
		if err == nil {
			t.Errorf("bpv=%d: expected error from Get, got nil", bpv)
		}
	}
}

// TestDirectReaderAt_Get_CorruptInput_ReturnsError verifies the same
// for DirectReaderAt.
func TestDirectReaderAt_Get_CorruptInput_ReturnsError(t *testing.T) {
	t.Parallel()
	for _, bpv := range directBitsPerValueSpectrum {
		r, err := GetDirectReaderAt(errRandomAccess{}, bpv, 0)
		if err != nil {
			t.Fatalf("GetDirectReaderAt bpv=%d: %v", bpv, err)
		}
		_, err = r.Get(0)
		if err == nil {
			t.Errorf("bpv=%d: expected error from Get, got nil", bpv)
		}
	}
}

// FuzzDirectReaderRoundTrip runs randomised round-trips for all
// supported bpv to detect subtle data-dependent bugs.
func FuzzDirectReaderRoundTrip(f *testing.F) {
	for _, seed := range []int64{42, 12345, 99999, 1<<31 - 1} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed int64) {
		bpv := directBitsPerValueSpectrum[int(seed&0xF)%len(directBitsPerValueSpectrum)]
		const n = 64
		r := rand.New(rand.NewSource(seed))
		values := make([]int64, n)
		mask := uint64(MaxValue(bpv))
		for i := range values {
			if bpv == 64 {
				values[i] = int64(r.Uint64())
			} else {
				values[i] = int64(r.Uint64() & mask)
			}
		}

		out := store.NewByteArrayDataOutput(64)
		w, err := GetDirectWriter(out, n, bpv)
		if err != nil {
			t.Fatalf("GetDirectWriter bpv=%d: %v", bpv, err)
		}
		for _, v := range values {
			if err := w.Add(v); err != nil {
				t.Fatalf("Add bpv=%d: %v", bpv, err)
			}
		}
		if err := w.Finish(); err != nil {
			t.Fatalf("Finish bpv=%d: %v", bpv, err)
		}
		in := &byteSliceRandomAccess{data: out.GetBytes()}
		reader, err := GetDirectReader(in, bpv)
		if err != nil {
			t.Fatalf("GetDirectReader bpv=%d: %v", bpv, err)
		}
		for i, want := range values {
			got, err := reader.Get(int64(i))
			if err != nil {
				t.Errorf("Get bpv=%d [%d]: unexpected error: %v", bpv, i, err)
			}
			if got != want {
				t.Errorf("bpv=%d [%d]: got %d want %d", bpv, i, got, want)
			}
		}
	})
}

// Ensure all readers are detected as unused only when the compile-time
// assertion infrastructure expects them.
var (
	_ LongValues = (*directReader1)(nil)
	_ LongValues = (*directReader2)(nil)
	_ LongValues = (*directReader4)(nil)
	_ LongValues = (*directReader8)(nil)
	_ LongValues = (*directReader12)(nil)
	_ LongValues = (*directReader16)(nil)
	_ LongValues = (*directReader20)(nil)
	_ LongValues = (*directReader24)(nil)
	_ LongValues = (*directReader28)(nil)
	_ LongValues = (*directReader32)(nil)
	_ LongValues = (*directReader40)(nil)
	_ LongValues = (*directReader48)(nil)
	_ LongValues = (*directReader56)(nil)
	_ LongValues = (*directReader64)(nil)
)
