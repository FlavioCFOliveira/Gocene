// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Port of lucene/core/src/test/org/apache/lucene/util/TestStringSorter.java
// (Apache Lucene 10.5.0). The Java anonymous subclasses of StringSorter and
// StableStringSorter are rendered as the implementation types below; the two
// comparators are BytesRefComparator.NATURAL (NewStringSorter) and
// Comparator.naturalOrder(), which is not a BytesRefComparator
// (NewStringSorterFn with BytesRef.compareTo).

import (
	"math/rand"
	"slices"
	"testing"
)

// stringSorterTestComparator is one of the two comparators of
// TestStringSorter.test(BytesRef[], int).
type stringSorterTestComparator struct {
	natural bool // BytesRefComparator.NATURAL when true, Comparator.naturalOrder() otherwise
}

var (
	stringSorterNatural      = stringSorterTestComparator{natural: true}
	stringSorterNaturalOrder = stringSorterTestComparator{natural: false}
)

// bytesRefCompareTo renders BytesRef.compareTo, the naturalOrder comparator.
func bytesRefCompareTo(o1, o2 *BytesRef) int {
	return o1.BytesRefCompareTo(o2)
}

// stringSorterRefs is the anonymous StringSorter subclass of test(...).
type stringSorterRefs struct {
	refs []*BytesRef
}

func (s *stringSorterRefs) Get(builder *BytesRefBuilder, result *BytesRef, i int) {
	ref := s.refs[i]
	result.Offset = ref.Offset
	result.Length = ref.Length
	result.Bytes = ref.Bytes
}

func (s *stringSorterRefs) Swap(i, j int) {
	s.refs[i], s.refs[j] = s.refs[j], s.refs[i]
}

// stableStringSorterOrds is the anonymous StableStringSorter subclass of
// testStable(...).
type stableStringSorterOrds struct {
	refs []*BytesRef
	ord  []int
	tmp  []int
}

func (s *stableStringSorterOrds) Save(i, j int) {
	s.tmp[j] = s.ord[i]
}

func (s *stableStringSorterOrds) Restore(i, j int) {
	copy(s.ord[i:j], s.tmp[i:j])
}

func (s *stableStringSorterOrds) Get(builder *BytesRefBuilder, result *BytesRef, i int) {
	ref := s.refs[s.ord[i]]
	result.Offset = ref.Offset
	result.Length = ref.Length
	result.Bytes = ref.Bytes
}

func (s *stableStringSorterOrds) Swap(i, j int) {
	s.ord[i], s.ord[j] = s.ord[j], s.ord[i]
}

func sortedBytesRefCopy(refs []*BytesRef, length int) []*BytesRef {
	expected := slices.Clone(refs[:length])
	// Arrays.sort(Object[]) is a stable merge sort over BytesRef.compareTo.
	slices.SortStableFunc(expected, bytesRefCompareTo)
	return expected
}

func testStringSorterAll(t *testing.T, refs []*BytesRef, length int) {
	t.Helper()
	testStringSorter(t, slices.Clone(refs[:length]), length, stringSorterNatural)
	testStringSorter(t, slices.Clone(refs[:length]), length, stringSorterNaturalOrder)
	testStableStringSorter(t, slices.Clone(refs[:length]), length, stringSorterNatural)
	testStableStringSorter(t, slices.Clone(refs[:length]), length, stringSorterNaturalOrder)
}

func testStringSorter(t *testing.T, refs []*BytesRef, length int, comparator stringSorterTestComparator) {
	t.Helper()
	expected := sortedBytesRefCopy(refs, length)

	impl := &stringSorterRefs{refs: refs}
	if comparator.natural {
		NewStringSorter(impl, NaturalBytesRefComparator).Sort(0, length)
	} else {
		NewStringSorterFn(impl, bytesRefCompareTo).Sort(0, length)
	}
	actual := refs[:length]
	for i := range expected {
		if !BytesRefEquals(expected[i], actual[i]) {
			t.Fatalf("arrays first differed at element [%d]; expected:<%v> but was:<%v>", i, expected[i], actual[i])
		}
	}
}

func testStableStringSorter(t *testing.T, refs []*BytesRef, length int, comparator stringSorterTestComparator) {
	t.Helper()
	expected := sortedBytesRefCopy(refs, length)

	ord := make([]int, length)
	for i := range ord {
		ord[i] = i
	}
	impl := &stableStringSorterOrds{refs: refs, ord: ord, tmp: make([]int, length)}
	if comparator.natural {
		NewStableStringSorter(impl, NaturalBytesRefComparator).Sort(0, length)
	} else {
		NewStableStringSorterFn(impl, bytesRefCompareTo).Sort(0, length)
	}

	for i := 0; i < length; i++ {
		if !BytesRefEquals(expected[i], refs[ord[i]]) {
			t.Fatalf("at %d expected:<%v> but was:<%v>", i, expected[i], refs[ord[i]])
		}
		if i > 0 && BytesRefEquals(expected[i], expected[i-1]) {
			if !(ord[i] > ord[i-1]) {
				t.Fatalf("not stable: %d <= %d", ord[i], ord[i-1])
			}
		}
	}
}

func TestStringSorter_testEmpty(t *testing.T) {
	r := newTestRandom(t)
	testStringSorterAll(t, make([]*BytesRef, r.Intn(5)), 0)
}

func TestStringSorter_testOneValue(t *testing.T) {
	r := newTestRandom(t)
	b := NewBytesRef([]byte(randomSimpleString(r)))
	testStringSorterAll(t, []*BytesRef{b}, 1)
}

func TestStringSorter_testTwoValues(t *testing.T) {
	r := newTestRandom(t)
	bytes1 := NewBytesRef([]byte(randomSimpleString(r)))
	bytes2 := NewBytesRef([]byte(randomSimpleString(r)))
	testStringSorterAll(t, []*BytesRef{bytes1, bytes2}, 2)
}

func testStringSorterRandom(t *testing.T, r *rand.Rand, commonPrefixLen, maxLen int) {
	t.Helper()
	commonPrefix := make([]byte, commonPrefixLen)
	r.Read(commonPrefix)
	length := r.Intn(100000)
	refs := make([]*BytesRef, length+r.Intn(50))
	for i := 0; i < length; i++ {
		b := make([]byte, commonPrefixLen+r.Intn(maxLen))
		r.Read(b)
		copy(b, commonPrefix)
		refs[i] = NewBytesRef(b)
	}
	testStringSorterAll(t, refs, length)
}

func TestStringSorter_testRandom(t *testing.T) {
	r := newTestRandom(t)
	numIters := atLeast(r, 3)
	for iter := 0; iter < numIters; iter++ {
		testStringSorterRandom(t, r, 0, 10)
	}
}

func TestStringSorter_testRandomWithLotsOfDuplicates(t *testing.T) {
	r := newTestRandom(t)
	numIters := atLeast(r, 3)
	for iter := 0; iter < numIters; iter++ {
		testStringSorterRandom(t, r, 0, 2)
	}
}

func TestStringSorter_testRandomWithSharedPrefix(t *testing.T) {
	r := newTestRandom(t)
	numIters := atLeast(r, 3)
	for iter := 0; iter < numIters; iter++ {
		testStringSorterRandom(t, r, nextInt(r, 1, 30), 10)
	}
}

func TestStringSorter_testRandomWithSharedPrefixAndLotsOfDuplicates(t *testing.T) {
	r := newTestRandom(t)
	numIters := atLeast(r, 3)
	for iter := 0; iter < numIters; iter++ {
		testStringSorterRandom(t, r, nextInt(r, 1, 30), 2)
	}
}
