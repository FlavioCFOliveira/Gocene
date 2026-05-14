// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// bytesArrayImpl adapts a []*BytesRef slice to the MSBRadixSorterImpl
// interface for testing.
type bytesArrayImpl struct {
	refs      []*BytesRef
	maxLength int
}

func (a *bytesArrayImpl) ByteAt(i, k int) int {
	if k >= a.maxLength {
		panic("k out of range")
	}
	ref := a.refs[i]
	if ref.Length <= k {
		return -1
	}
	return int(ref.Bytes[ref.Offset+k]) & 0xFF
}

func (a *bytesArrayImpl) Swap(i, j int) {
	a.refs[i], a.refs[j] = a.refs[j], a.refs[i]
}

// runMSBRadixCheck mirrors test(BytesRef[] refs, int len) in TestMSBRadixSorter.java.
func runMSBRadixCheck(t *testing.T, rng *rand.Rand, refs []*BytesRef, length int) {
	t.Helper()
	expected := make([]*BytesRef, length)
	for i := 0; i < length; i++ {
		expected[i] = refs[i]
	}
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(
			expected[i].Bytes[expected[i].Offset:expected[i].Offset+expected[i].Length],
			expected[j].Bytes[expected[j].Offset:expected[j].Offset+expected[j].Length],
		) < 0
	})

	maxLength := 0
	for i := 0; i < length; i++ {
		if refs[i].Length > maxLength {
			maxLength = refs[i].Length
		}
	}
	switch rng.Intn(3) {
	case 0:
		maxLength += 1 + rng.Intn(5)
	case 1:
		maxLength = math.MaxInt32
	}

	impl := &bytesArrayImpl{refs: refs, maxLength: maxLength}
	NewMSBRadixSorter(impl, maxLength).Sort(0, length)

	for i := 0; i < length; i++ {
		if !bytesRefEqualsContents(refs[i], expected[i]) {
			t.Fatalf("at %d: got=%v want=%v (len=%d maxLen=%d)",
				i, refs[i], expected[i], length, maxLength)
		}
	}
}

func bytesRefEqualsContents(a, b *BytesRef) bool {
	return bytes.Equal(
		a.Bytes[a.Offset:a.Offset+a.Length],
		b.Bytes[b.Offset:b.Offset+b.Length],
	)
}

func randomSimpleBytes(rng *rand.Rand, maxLen int) *BytesRef {
	n := rng.Intn(maxLen)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + rng.Intn(26))
	}
	return &BytesRef{Bytes: b, Length: n}
}

func TestMSBRadixSorter_Empty(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	refs := make([]*BytesRef, rng.Intn(5))
	for i := range refs {
		refs[i] = randomSimpleBytes(rng, 10)
	}
	runMSBRadixCheck(t, rng, refs, 0)
}

func TestMSBRadixSorter_OneValue(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	refs := []*BytesRef{randomSimpleBytes(rng, 20)}
	runMSBRadixCheck(t, rng, refs, 1)
}

func TestMSBRadixSorter_TwoValues(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	refs := []*BytesRef{randomSimpleBytes(rng, 20), randomSimpleBytes(rng, 20)}
	runMSBRadixCheck(t, rng, refs, 2)
}

func testRandomMSBRadix(t *testing.T, rng *rand.Rand, commonPrefixLen, maxLen int) {
	t.Helper()
	commonPrefix := make([]byte, commonPrefixLen)
	rng.Read(commonPrefix)
	length := rng.Intn(2000)
	refs := make([]*BytesRef, length+rng.Intn(50))
	for i := 0; i < length; i++ {
		b := make([]byte, commonPrefixLen+rng.Intn(maxLen))
		rng.Read(b)
		copy(b, commonPrefix)
		refs[i] = &BytesRef{Bytes: b, Length: len(b)}
	}
	runMSBRadixCheck(t, rng, refs, length)
}

func TestMSBRadixSorter_Random(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for iter := 0; iter < 10; iter++ {
		testRandomMSBRadix(t, rng, 0, 10)
	}
}

func TestMSBRadixSorter_RandomLotsOfDuplicates(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for iter := 0; iter < 10; iter++ {
		testRandomMSBRadix(t, rng, 0, 2)
	}
}

func TestMSBRadixSorter_RandomWithSharedPrefix(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	for iter := 0; iter < 10; iter++ {
		testRandomMSBRadix(t, rng, 1+rng.Intn(30), 10)
	}
}

func TestMSBRadixSorter_RandomWithSharedPrefixAndLotsOfDuplicates(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 5; iter++ {
		testRandomMSBRadix(t, rng, 1+rng.Intn(30), 2)
	}
}

func TestMSBRadixSorter_FallbackSortInsideRange(t *testing.T) {
	// Force the introsort fallback by using < 100 keys (LENGTH_THRESHOLD).
	rng := rand.New(rand.NewSource(8))
	refs := make([]*BytesRef, 50)
	for i := range refs {
		refs[i] = randomSimpleBytes(rng, 20)
	}
	original := make([]*BytesRef, len(refs))
	copy(original, refs)
	runMSBRadixCheck(t, rng, refs, len(refs))
}

func TestMSBRadixSorter_LargeMaxLengthBound(t *testing.T) {
	// math.MaxInt32 maxLength is the common production setting when the
	// caller doesn't know the upper bound.
	rng := rand.New(rand.NewSource(9))
	refs := make([]*BytesRef, 1024)
	for i := range refs {
		refs[i] = randomSimpleBytes(rng, 64)
	}
	impl := &bytesArrayImpl{refs: refs, maxLength: math.MaxInt32}
	NewMSBRadixSorter(impl, math.MaxInt32).Sort(0, len(refs))
	for i := 1; i < len(refs); i++ {
		if bytes.Compare(refs[i-1].Bytes[:refs[i-1].Length], refs[i].Bytes[:refs[i].Length]) > 0 {
			t.Fatalf("not sorted at %d", i)
		}
	}
}
