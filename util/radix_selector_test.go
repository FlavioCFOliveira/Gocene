// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
)

// byteSliceSelector adapts a slice of byte-slices to
// RadixSelectorInterface for testing.
type byteSliceSelector struct {
	data [][]byte
}

func (b *byteSliceSelector) Swap(i, j int) {
	b.data[i], b.data[j] = b.data[j], b.data[i]
}

func (b *byteSliceSelector) ByteAt(i, k int) int {
	if k >= len(b.data[i]) {
		return -1
	}
	return int(b.data[i][k]) & 0xff
}

// runRadixSelectExpected sorts a copy and returns the value that
// should sit at slot k after Select.
func runRadixSelectExpected(in [][]byte, k int) []byte {
	cp := make([][]byte, len(in))
	for i, b := range in {
		nb := make([]byte, len(b))
		copy(nb, b)
		cp[i] = nb
	}
	sort.Slice(cp, func(i, j int) bool { return bytes.Compare(cp[i], cp[j]) < 0 })
	return cp[k]
}

// TestRadixSelector_Basic exercises the algorithm with a small
// hand-picked input.
func TestRadixSelector_Basic(t *testing.T) {
	data := [][]byte{
		[]byte("banana"),
		[]byte("apple"),
		[]byte("cherry"),
		[]byte("date"),
		[]byte("apricot"),
	}
	sel := &byteSliceSelector{data: append([][]byte(nil), data...)}
	rs := NewRadixSelector(sel, 16)
	rs.Select(0, len(data), 2)
	want := runRadixSelectExpected(data, 2)
	if !bytes.Equal(sel.data[2], want) {
		t.Fatalf("slot 2 = %q, want %q", sel.data[2], want)
	}
	// Left side <= slot k; right side >= slot k.
	for i := 0; i < 2; i++ {
		if bytes.Compare(sel.data[i], sel.data[2]) > 0 {
			t.Fatalf("invariant violated: data[%d]=%q > data[2]=%q", i, sel.data[i], sel.data[2])
		}
	}
	for i := 3; i < len(sel.data); i++ {
		if bytes.Compare(sel.data[i], sel.data[2]) < 0 {
			t.Fatalf("invariant violated: data[%d]=%q < data[2]=%q", i, sel.data[i], sel.data[2])
		}
	}
}

// TestRadixSelector_Random stresses Select with random byte slices of
// random lengths and checks the invariant for several values of k.
func TestRadixSelector_Random(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	for trial := 0; trial < 100; trial++ {
		n := 50 + rnd.Intn(150)
		maxLen := 1 + rnd.Intn(40)
		data := make([][]byte, n)
		for i := range data {
			l := rnd.Intn(maxLen + 1)
			data[i] = make([]byte, l)
			for j := range data[i] {
				data[i][j] = byte(rnd.Intn(256))
			}
		}
		k := rnd.Intn(n)
		want := runRadixSelectExpected(data, k)

		sel := &byteSliceSelector{data: append([][]byte(nil), data...)}
		rs := NewRadixSelector(sel, maxLen)
		rs.Select(0, n, k)

		if !bytes.Equal(sel.data[k], want) {
			t.Fatalf("trial %d: slot %d = %x, want %x", trial, k, sel.data[k], want)
		}
		for i := 0; i < k; i++ {
			if bytes.Compare(sel.data[i], sel.data[k]) > 0 {
				t.Fatalf("trial %d: invariant violated at i=%d", trial, i)
			}
		}
		for i := k + 1; i < n; i++ {
			if bytes.Compare(sel.data[i], sel.data[k]) < 0 {
				t.Fatalf("trial %d: invariant violated at i=%d", trial, i)
			}
		}
	}
}

// TestRadixSelector_CommonPrefix exercises the common-prefix fast
// path: many keys sharing a long prefix.
func TestRadixSelector_CommonPrefix(t *testing.T) {
	data := [][]byte{
		[]byte("zzzzzzzzz1"),
		[]byte("zzzzzzzzz3"),
		[]byte("zzzzzzzzz0"),
		[]byte("zzzzzzzzz2"),
		[]byte("zzzzzzzzz4"),
	}
	sel := &byteSliceSelector{data: append([][]byte(nil), data...)}
	rs := NewRadixSelector(sel, 16)
	rs.Select(0, len(data), 1)
	want := runRadixSelectExpected(data, 1)
	if !bytes.Equal(sel.data[1], want) {
		t.Fatalf("slot 1 = %q, want %q", sel.data[1], want)
	}
}

// TestRadixSelector_LengthVariation exercises the end-of-string
// histogram bucket (bucket 0) when keys have different lengths.
func TestRadixSelector_LengthVariation(t *testing.T) {
	data := [][]byte{
		[]byte("a"),
		[]byte("ab"),
		[]byte("abc"),
		[]byte("abcd"),
		[]byte("abcde"),
	}
	sel := &byteSliceSelector{data: append([][]byte(nil), data...)}
	rs := NewRadixSelector(sel, 8)
	rs.Select(0, len(data), 3)
	if !bytes.Equal(sel.data[3], []byte("abcd")) {
		t.Fatalf("slot 3 = %q, want %q", sel.data[3], "abcd")
	}
}
