// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

// TestBitDocIdSet_NoBit tests a BitDocIdSet with no bits set (length=1).
// Source: BaseDocIdSetTestCase.testNoBit()
func TestBitDocIdSet_NoBit(t *testing.T) {
	// Create a FixedBitSet with length 1, no bits set
	fs, err := NewFixedBitSet(1)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}

	bitSet, err := NewBitDocIdSetWithCardinality(fs)
	if err != nil {
		t.Fatalf("Failed to create BitDocIdSet: %v", err)
	}

	// Verify using assertEquals logic
	assertBitSetEquals(t, 1, []int{}, bitSet)
}

// TestBitDocIdSet_OneBit tests a BitDocIdSet with one bit.
// Source: BaseDocIdSetTestCase.test1Bit()
func TestBitDocIdSet_OneBit(t *testing.T) {
	// Test with bit set
	fs, _ := NewFixedBitSet(1)
	fs.Set(0)

	bitSet, err := NewBitDocIdSetWithCardinality(fs)
	if err != nil {
		t.Fatalf("Failed to create BitDocIdSet: %v", err)
	}

	assertBitSetEquals(t, 1, []int{0}, bitSet)

	// Test without bit set
	fs2, _ := NewFixedBitSet(1)
	bitSet2, _ := NewBitDocIdSetWithCardinality(fs2)
	assertBitSetEquals(t, 1, []int{}, bitSet2)
}

// TestBitDocIdSet_TwoBits tests a BitDocIdSet with two bits.
// Source: BaseDocIdSetTestCase.test2Bits()
func TestBitDocIdSet_TwoBits(t *testing.T) {
	// Test all combinations
	testCases := []struct {
		set0, set1 bool
		expected   []int
	}{
		{false, false, []int{}},
		{true, false, []int{0}},
		{false, true, []int{1}},
		{true, true, []int{0, 1}},
	}

	for _, tc := range testCases {
		fs, _ := NewFixedBitSet(2)
		if tc.set0 {
			fs.Set(0)
		}
		if tc.set1 {
			fs.Set(1)
		}

		bitSet, _ := NewBitDocIdSetWithCardinality(fs)
		assertBitSetEquals(t, 2, tc.expected, bitSet)
	}
}

// TestBitDocIdSet_IteratorBehavior tests the iterator's nextDoc and advance methods.
// Source: BaseDocIdSetTestCase.assertEquals() - nextDoc/advance testing
func TestBitDocIdSet_IteratorBehavior(t *testing.T) {
	// Create a bitset with specific bits set
	fs, _ := NewFixedBitSet(100)
	expectedDocs := []int{5, 10, 20, 50, 99}
	for _, doc := range expectedDocs {
		fs.Set(doc)
	}

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	// Test nextDoc
	iter := bitSet.Iterator()
	if iter.DocID() != -1 {
		t.Errorf("Expected initial DocID to be -1, got %d", iter.DocID())
	}

	for i, expectedDoc := range expectedDocs {
		doc, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc failed at iteration %d: %v", i, err)
		}
		if doc != expectedDoc {
			t.Errorf("Expected doc %d, got %d at iteration %d", expectedDoc, doc, i)
		}
		if iter.DocID() != expectedDoc {
			t.Errorf("Expected DocID() to return %d, got %d", expectedDoc, iter.DocID())
		}
	}

	// After exhausting, should return NO_MORE_DOCS
	doc, _ := iter.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS after exhausting iterator, got %d", doc)
	}

	// Test advance
	iter2 := bitSet.Iterator()

	// Advance to position
	doc, _ = iter2.Advance(15)
	if doc != 20 {
		t.Errorf("Expected doc 20 after advance(15), got %d", doc)
	}

	// Advance to exact position
	doc, _ = iter2.Advance(50)
	if doc != 50 {
		t.Errorf("Expected doc 50 after advance(50), got %d", doc)
	}

	// Advance beyond all docs
	doc, _ = iter2.Advance(100)
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS after advance(100), got %d", doc)
	}
}

// TestBitDocIdSet_AdvanceVariations tests various advance scenarios.
// Source: BaseDocIdSetTestCase.assertEquals() - random nextDoc/advance
func TestBitDocIdSet_AdvanceVariations(t *testing.T) {
	fs, _ := NewFixedBitSet(200)
	// Set bits at regular intervals
	expectedDocs := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, doc := range expectedDocs {
		fs.Set(doc)
	}

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	// Test alternating between nextDoc and advance
	iter := bitSet.Iterator()

	// First use nextDoc
	doc, _ := iter.NextDoc()
	if doc != 10 {
		t.Errorf("Expected doc 10, got %d", doc)
	}

	// Then advance
	doc, _ = iter.Advance(25)
	if doc != 30 {
		t.Errorf("Expected doc 30 after advance(25), got %d", doc)
	}

	// Then nextDoc
	doc, _ = iter.NextDoc()
	if doc != 40 {
		t.Errorf("Expected doc 40, got %d", doc)
	}

	// Advance to non-existent position
	doc, _ = iter.Advance(55)
	if doc != 60 {
		t.Errorf("Expected doc 60 after advance(55), got %d", doc)
	}

	// Advance to exact position
	doc, _ = iter.Advance(80)
	if doc != 80 {
		t.Errorf("Expected doc 80 after advance(80), got %d", doc)
	}
}

// TestBitDocIdSet_EmptyIterator tests iterator on empty bitset.
func TestBitDocIdSet_EmptyIterator(t *testing.T) {
	fs, _ := NewFixedBitSet(100)
	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	iter := bitSet.Iterator()
	if iter == nil {
		t.Fatal("Expected non-nil iterator")
	}

	doc, _ := iter.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS for empty bitset, got %d", doc)
	}
}

// TestBitDocIdSet_AllBitsSet tests iterator when all bits are set.
func TestBitDocIdSet_AllBitsSet(t *testing.T) {
	fs, _ := NewFixedBitSet(10)
	fs.SetAll()

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	iter := bitSet.Iterator()
	for i := 0; i < 10; i++ {
		doc, _ := iter.NextDoc()
		if doc != i {
			t.Errorf("Expected doc %d, got %d", i, doc)
		}
	}

	doc, _ := iter.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS after all docs, got %d", doc)
	}
}

// TestBitDocIdSet_Cardinality tests cardinality operations.
func TestBitDocIdSet_Cardinality(t *testing.T) {
	// Test with various cardinalities
	testCases := []struct {
		bitsToSet []int
		expected  int
	}{
		{[]int{}, 0},
		{[]int{0}, 1},
		{[]int{5, 10, 15}, 3},
		{[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 10},
	}

	for _, tc := range testCases {
		fs, _ := NewFixedBitSet(100)
		for _, bit := range tc.bitsToSet {
			fs.Set(bit)
		}

		bitSet, _ := NewBitDocIdSetWithCardinality(fs)
		if bitSet.Cost() != int64(tc.expected) {
			t.Errorf("Expected cost %d for bits %v, got %d", tc.expected, tc.bitsToSet, bitSet.Cost())
		}
	}
}

// TestBitDocIdSet_Cost tests the cost method.
func TestBitDocIdSet_Cost(t *testing.T) {
	fs, _ := NewFixedBitSet(1000)

	// Set some bits
	for i := 0; i < 1000; i += 10 {
		fs.Set(i)
	}

	// Test with explicit cost
	bitSet, _ := NewBitDocIdSet(fs, 100)
	if bitSet.Cost() != 100 {
		t.Errorf("Expected cost 100, got %d", bitSet.Cost())
	}

	// Test with cardinality as cost
	bitSet2, _ := NewBitDocIdSetWithCardinality(fs)
	if bitSet2.Cost() != 100 {
		t.Errorf("Expected cost 100 (cardinality), got %d", bitSet2.Cost())
	}
}

// TestBitDocIdSet_BitsAccess tests accessing the underlying FixedBitSet.
func TestBitDocIdSet_BitsAccess(t *testing.T) {
	fs, _ := NewFixedBitSet(100)
	fs.Set(50)

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	// Access underlying bits
	bits := bitSet.Bits()
	if bits == nil {
		t.Fatal("Expected non-nil bits")
	}

	if !bits.Get(50) {
		t.Error("Expected bit 50 to be set in underlying bits")
	}

	if bits.Length() != 100 {
		t.Errorf("Expected bits length 100, got %d", bits.Length())
	}
}

// TestBitDocIdSet_RandomAccess tests random access patterns.
// Source: BaseDocIdSetTestCase.testAgainstBitSet()
func TestBitDocIdSet_RandomAccess(t *testing.T) {
	// Test with various sizes and densities
	sizes := []int{100, 1000, 10000}
	densities := []float64{0.0, 0.01, 0.1, 0.5, 0.9, 1.0}

	for _, size := range sizes {
		for _, density := range densities {
			fs, _ := NewFixedBitSet(size)
			expectedDocs := make([]int, 0)

			// Randomly set bits based on density
			r := rand.New(rand.NewSource(42)) // Fixed seed for reproducibility
			for i := 0; i < size; i++ {
				if r.Float64() < density {
					fs.Set(i)
					expectedDocs = append(expectedDocs, i)
				}
			}

			bitSet, _ := NewBitDocIdSetWithCardinality(fs)
			assertBitSetEquals(t, size, expectedDocs, bitSet)
		}
	}
}

// TestBitDocIdSet_SingleDoc tests single document scenarios.
// Source: BaseDocIdSetTestCase.testAgainstBitSet() - one doc
func TestBitDocIdSet_SingleDoc(t *testing.T) {
	// Test with only first doc
	fs1, _ := NewFixedBitSet(1000)
	fs1.Set(0)
	bitSet1, _ := NewBitDocIdSetWithCardinality(fs1)
	assertBitSetEquals(t, 1000, []int{0}, bitSet1)

	// Test with only last doc
	fs2, _ := NewFixedBitSet(1000)
	fs2.Set(999)
	bitSet2, _ := NewBitDocIdSetWithCardinality(fs2)
	assertBitSetEquals(t, 1000, []int{999}, bitSet2)

	// Test with random single doc
	fs3, _ := NewFixedBitSet(1000)
	fs3.Set(500)
	bitSet3, _ := NewBitDocIdSetWithCardinality(fs3)
	assertBitSetEquals(t, 1000, []int{500}, bitSet3)
}

// TestBitDocIdSet_RegularIncrements tests regular increment patterns.
// Source: BaseDocIdSetTestCase.testAgainstBitSet() - regular increments
func TestBitDocIdSet_RegularIncrements(t *testing.T) {
	size := 1000

	for inc := 2; inc < 100; inc += 7 {
		fs, _ := NewFixedBitSet(size)
		expectedDocs := make([]int, 0)

		for d := 0; d < size; d += inc {
			fs.Set(d)
			expectedDocs = append(expectedDocs, d)
		}

		bitSet, _ := NewBitDocIdSetWithCardinality(fs)
		assertBitSetEquals(t, size, expectedDocs, bitSet)
	}
}

// TestBitDocIdSet_LargeBitSet tests with a large bitset.
func TestBitDocIdSet_LargeBitSet(t *testing.T) {
	// Test with a large bitset spanning multiple uint64 words
	fs, _ := NewFixedBitSet(10000)
	expectedDocs := []int{0, 63, 64, 127, 128, 5000, 9999}

	for _, doc := range expectedDocs {
		fs.Set(doc)
	}

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)
	assertBitSetEquals(t, 10000, expectedDocs, bitSet)
}

// TestBitDocIdSet_DocIDRunEnd tests the DocIDRunEnd method.
func TestBitDocIdSet_DocIDRunEnd(t *testing.T) {
	// Create bitset with consecutive runs
	fs, _ := NewFixedBitSet(100)
	// Set consecutive bits 10-14
	for i := 10; i <= 14; i++ {
		fs.Set(i)
	}
	// Set isolated bit 50
	fs.Set(50)

	bitSet, _ := NewBitDocIdSetWithCardinality(fs)
	iter := bitSet.Iterator()

	// Advance to first run
	iter.Advance(10)
	runEnd := iter.DocIDRunEnd()
	if runEnd != 15 {
		t.Errorf("Expected run end 15, got %d", runEnd)
	}

	// Move to next doc
	iter.NextDoc()
	runEnd = iter.DocIDRunEnd()
	if runEnd != 51 {
		t.Errorf("Expected run end 51 for isolated bit, got %d", runEnd)
	}
}

// TestBitDocIdSet_NewWithInvalidCost tests error handling for invalid cost.
func TestBitDocIdSet_NewWithInvalidCost(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	_, err := NewBitDocIdSet(fs, -1)
	if err == nil {
		t.Error("Expected error for negative cost")
	}
}

// TestBitDocIdSet_ImplementsDocIdSet verifies BitDocIdSet implements DocIdSet interface.
func TestBitDocIdSet_ImplementsDocIdSet(t *testing.T) {
	fs, _ := NewFixedBitSet(100)
	bitSet, _ := NewBitDocIdSetWithCardinality(fs)

	// This should compile if BitDocIdSet implements DocIdSet
	var _ DocIdSet = bitSet
}

// assertBitSetEquals verifies that a BitDocIdSet contains exactly the expected documents.
// This is the Go equivalent of BaseDocIdSetTestCase.assertEquals()
func assertBitSetEquals(t *testing.T, numBits int, expectedDocs []int, bitSet *BitDocIdSet) {
	t.Helper()

	iter := bitSet.Iterator()
	if iter == nil {
		if len(expectedDocs) > 0 {
			t.Errorf("Expected iterator to be non-nil for non-empty bitset")
		}
		return
	}

	// Verify initial DocID is -1
	if iter.DocID() != -1 {
		t.Errorf("Expected initial DocID to be -1, got %d", iter.DocID())
	}

	// Test nextDoc
	for i, expectedDoc := range expectedDocs {
		doc, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc failed at iteration %d: %v", i, err)
		}
		if doc != expectedDoc {
			t.Errorf("Expected doc %d, got %d at iteration %d", expectedDoc, doc, i)
		}
		if iter.DocID() != expectedDoc {
			t.Errorf("Expected DocID() to return %d, got %d", expectedDoc, iter.DocID())
		}
	}

	// After exhausting, should return NO_MORE_DOCS
	doc, _ := iter.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS after exhausting iterator, got %d", doc)
	}
	if iter.DocID() != NO_MORE_DOCS {
		t.Errorf("Expected DocID() to be NO_MORE_DOCS after exhaustion, got %d", iter.DocID())
	}

	// Test advance with random targets
	iter2 := bitSet.Iterator()
	if iter2 != nil {
		currentDoc := -1
		for currentDoc != NO_MORE_DOCS {
			// Randomly choose between nextDoc and advance
			if rand.Float64() < 0.5 {
				doc, _ = iter2.NextDoc()
				expectedDoc := nextExpectedDoc(expectedDocs, currentDoc)
				if doc != expectedDoc {
					t.Errorf("Expected doc %d from nextDoc, got %d", expectedDoc, doc)
				}
				currentDoc = doc
			} else {
				target := currentDoc + 1 + rand.Intn(maxInt(64, numBits/8))
				doc, _ = iter2.Advance(target)
				expectedDoc := nextExpectedDocAtOrAfter(expectedDocs, target)
				if doc != expectedDoc {
					t.Errorf("Expected doc %d from advance(%d), got %d", expectedDoc, target, doc)
				}
				currentDoc = doc
			}
		}
	}
}

// nextExpectedDoc returns the next expected document after the given doc
func nextExpectedDoc(expectedDocs []int, currentDoc int) int {
	for _, doc := range expectedDocs {
		if doc > currentDoc {
			return doc
		}
	}
	return NO_MORE_DOCS
}

// nextExpectedDocAtOrAfter returns the next expected document at or after the target
func nextExpectedDocAtOrAfter(expectedDocs []int, target int) int {
	for _, doc := range expectedDocs {
		if doc >= target {
			return doc
		}
	}
	return NO_MORE_DOCS
}

// maxInt returns the maximum of two ints
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
