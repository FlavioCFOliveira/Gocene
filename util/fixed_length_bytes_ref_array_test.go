// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"testing"
)

// TestFixedLengthBytesRefArray exercises the public surface required by
// Lucene's TestFixedLengthBytesRefArray: construction, append validation,
// size, clear, raw and sorted iteration, and the value-length accessor.
func TestFixedLengthBytesRefArray(t *testing.T) {
	t.Run("constructor rejects non-positive length", func(t *testing.T) {
		if _, err := NewFixedLengthBytesRefArray(0); err == nil {
			t.Fatalf("valueLength=0 must error")
		}
		if _, err := NewFixedLengthBytesRefArray(-1); err == nil {
			t.Fatalf("valueLength<0 must error")
		}
	})

	t.Run("append validates length", func(t *testing.T) {
		arr, _ := NewFixedLengthBytesRefArray(4)
		if _, err := arr.Append(nil); err == nil {
			t.Fatalf("nil bytes ref must error")
		}
		if _, err := arr.Append(NewBytesRef([]byte("abc"))); err == nil {
			t.Fatalf("short bytes ref must error")
		}
		if _, err := arr.Append(NewBytesRef([]byte("abcde"))); err == nil {
			t.Fatalf("long bytes ref must error")
		}
		if _, err := arr.Append(NewBytesRef([]byte("abcd"))); err != nil {
			t.Fatalf("matching length must succeed, got %v", err)
		}
		if arr.Size() != 1 {
			t.Fatalf("Size got %d want 1", arr.Size())
		}
	})

	t.Run("raw iteration preserves order", func(t *testing.T) {
		arr, _ := NewFixedLengthBytesRefArray(2)
		inputs := []string{"zz", "aa", "mm", "bb"}
		for _, s := range inputs {
			if _, err := arr.Append(NewBytesRef([]byte(s))); err != nil {
				t.Fatalf("append: %v", err)
			}
		}
		it := arr.Iterator(nil)
		var got []string
		for {
			br, err := it.Next()
			if err != nil {
				t.Fatalf("iter next: %v", err)
			}
			if br == nil {
				break
			}
			got = append(got, string(br.ValidBytes()))
		}
		if len(got) != len(inputs) {
			t.Fatalf("len(got)=%d want %d", len(got), len(inputs))
		}
		for i := range inputs {
			if got[i] != inputs[i] {
				t.Fatalf("entry %d got %q want %q", i, got[i], inputs[i])
			}
		}
	})

	t.Run("sorted iteration", func(t *testing.T) {
		arr, _ := NewFixedLengthBytesRefArray(3)
		inputs := []string{"zzz", "aaa", "mmm", "bbb"}
		for _, s := range inputs {
			if _, err := arr.Append(NewBytesRef([]byte(s))); err != nil {
				t.Fatalf("append: %v", err)
			}
		}
		it := arr.Iterator(NaturalBytesRefComparator)
		var got []string
		for {
			br, err := it.Next()
			if err != nil {
				t.Fatalf("iter next: %v", err)
			}
			if br == nil {
				break
			}
			got = append(got, string(br.ValidBytes()))
		}
		want := []string{"aaa", "bbb", "mmm", "zzz"}
		for i, w := range want {
			if got[i] != w {
				t.Fatalf("sorted entry %d got %q want %q", i, got[i], w)
			}
		}
	})

	t.Run("multi-block append", func(t *testing.T) {
		// Force a small maxValuesPerBlock by using a large valueLength.
		// At valueLength=1<<23, ceilLog2=23 so shift=1 -> max=2 -> floor 16.
		arr, _ := NewFixedLengthBytesRefArray(1 << 23)
		if arr.maxValuesPerBlock != 16 {
			t.Fatalf("expected maxValuesPerBlock=16 for valueLength=%d, got %d",
				arr.valueLength, arr.maxValuesPerBlock)
		}

		// Use a smaller, but still block-crossing, valueLength for the
		// round-trip test so we don't allocate gigabytes.
		small, _ := NewFixedLengthBytesRefArray(4) // maxValuesPerBlock = 1<<22
		// Force multi-block by lowering maxValuesPerBlock for this test.
		small.maxValuesPerBlock = 8
		for i := 0; i < 25; i++ {
			b := []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)}
			id, err := small.Append(NewBytesRef(b))
			if err != nil {
				t.Fatalf("append %d: %v", i, err)
			}
			if id != i {
				t.Fatalf("append returned id %d want %d", id, i)
			}
		}
		if got := len(small.blocks); got != 4 {
			t.Fatalf("multi-block expected 4 blocks got %d", got)
		}
		it := small.Iterator(nil)
		for i := 0; i < 25; i++ {
			br, _ := it.Next()
			if br == nil {
				t.Fatalf("unexpected end at i=%d", i)
			}
			want := []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3)}
			if !bytes.Equal(br.ValidBytes(), want) {
				t.Fatalf("entry %d: got % x want % x", i, br.ValidBytes(), want)
			}
		}
		if br, _ := it.Next(); br != nil {
			t.Fatalf("iterator should be exhausted")
		}
	})

	t.Run("clear resets state", func(t *testing.T) {
		arr, _ := NewFixedLengthBytesRefArray(4)
		_, _ = arr.Append(NewBytesRef([]byte("abcd")))
		_, _ = arr.Append(NewBytesRef([]byte("efgh")))
		if arr.Size() != 2 {
			t.Fatalf("pre-clear size got %d want 2", arr.Size())
		}
		arr.Clear()
		if arr.Size() != 0 {
			t.Fatalf("post-clear size got %d want 0", arr.Size())
		}
		it := arr.Iterator(nil)
		br, _ := it.Next()
		if br != nil {
			t.Fatalf("post-clear iterator must be empty")
		}
	})

	t.Run("value length accessor", func(t *testing.T) {
		arr, _ := NewFixedLengthBytesRefArray(7)
		if arr.ValueLength() != 7 {
			t.Fatalf("ValueLength got %d want 7", arr.ValueLength())
		}
	})

	t.Run("byte content correctness", func(t *testing.T) {
		// Verify that the bytes copied into the backing block are byte-for-byte
		// identical to the input.
		arr, _ := NewFixedLengthBytesRefArray(8)
		raw := [][]byte{
			{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88},
			{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80},
		}
		for _, r := range raw {
			if _, err := arr.Append(NewBytesRef(r)); err != nil {
				t.Fatalf("append: %v", err)
			}
		}
		it := arr.Iterator(nil)
		i := 0
		for {
			br, err := it.Next()
			if err != nil {
				t.Fatalf("iter: %v", err)
			}
			if br == nil {
				break
			}
			if !bytes.Equal(br.ValidBytes(), raw[i]) {
				t.Fatalf("entry %d bytes mismatch: got % x want % x",
					i, br.ValidBytes(), raw[i])
			}
			i++
		}
	})
}

func TestComputeMaxValuesPerBlock(t *testing.T) {
	tests := []struct {
		valueLength int
		want        int
	}{
		{1, 1 << 24},
		{2, 1 << 23},
		{4, 1 << 22},
		{16, 1 << 20},
		{1 << 20, 16}, // ceilLog2(2^20)=20, shift=4, max=16, floor=16
		{1 << 24, 16}, // ceilLog2=24, shift=0, floor=16
	}
	for _, tc := range tests {
		got := computeMaxValuesPerBlock(tc.valueLength)
		if got != tc.want {
			t.Fatalf("computeMaxValuesPerBlock(%d) got %d want %d",
				tc.valueLength, got, tc.want)
		}
	}
}
