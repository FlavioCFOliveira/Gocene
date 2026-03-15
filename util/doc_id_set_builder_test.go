// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// assertDocIdSetEquals checks if two DocIdSets are equal
func assertDocIdSetEquals(t *testing.T, d1, d2 util.DocIdSet) {
	t.Helper()

	// Handle nil cases
	if d1 == nil {
		if d2 != nil {
			iter := d2.Iterator()
			doc, err := iter.NextDoc()
			if err != nil {
				t.Fatalf("Error iterating d2: %v", err)
			}
			if doc != util.NO_MORE_DOCS {
				t.Errorf("Expected empty set, got doc %d", doc)
			}
		}
		return
	}
	if d2 == nil {
		iter := d1.Iterator()
		doc, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating d1: %v", err)
		}
		if doc != util.NO_MORE_DOCS {
			t.Errorf("Expected empty set, got doc %d", doc)
		}
		return
	}

	i1 := d1.Iterator()
	i2 := d2.Iterator()

	for {
		doc1, err1 := i1.NextDoc()
		if err1 != nil {
			t.Fatalf("Error from i1: %v", err1)
		}
		doc2, err2 := i2.NextDoc()
		if err2 != nil {
			t.Fatalf("Error from i2: %v", err2)
		}
		if doc1 != doc2 {
			t.Errorf("Doc mismatch: %d vs %d", doc1, doc2)
			return
		}
		if doc1 == util.NO_MORE_DOCS {
			break
		}
	}
}

// TestDocIdSetBuilder_Empty tests that an empty builder returns nil
func TestDocIdSetBuilder_Empty(t *testing.T) {
	maxDoc := 1 + util.RandomIntN(1000)
	builder := util.NewDocIdSetBuilder(maxDoc)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Empty builder should return nil or empty iterator
	if result != nil {
		iter := result.Iterator()
		doc, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc != util.NO_MORE_DOCS {
			t.Errorf("Expected NO_MORE_DOCS, got %d", doc)
		}
	}
}

// TestDocIdSetBuilder_Sparse tests building from sparse data
// Should create IntArrayDocIdSet
func TestDocIdSetBuilder_Sparse(t *testing.T) {
	maxDoc := 1000000 + util.RandomIntN(1000000)
	builder := util.NewDocIdSetBuilder(maxDoc)
	numIterators := 1 + util.RandomIntN(10)

	ref, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}

	for i := 0; i < numIterators; i++ {
		baseInc := 200000 + util.RandomIntN(10000)
		docs := make([]int, 0)
		for doc := util.RandomIntN(100); doc < maxDoc; doc += baseInc + util.RandomIntN(10000) {
			docs = append(docs, doc)
			ref.Set(doc)
		}
		// Add via iterator
		if len(docs) > 0 {
			adder := builder.Grow(len(docs))
			for _, doc := range docs {
				adder.Add(doc)
			}
		}
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should be IntArrayDocIdSet for sparse data
	if _, ok := result.(*util.IntArrayDocIdSet); !ok {
		t.Errorf("Expected IntArrayDocIdSet for sparse data, got %T", result)
	}

	expected, err := util.NewBitDocIdSetWithCardinality(ref)
	if err != nil {
		t.Fatalf("Failed to create BitDocIdSet: %v", err)
	}
	assertDocIdSetEquals(t, expected, result)
}

// TestDocIdSetBuilder_Dense tests building from dense data
// Should create BitDocIdSet
func TestDocIdSetBuilder_Dense(t *testing.T) {
	maxDoc := 1000000 + util.RandomIntN(1000000)
	builder := util.NewDocIdSetBuilder(maxDoc)
	numIterators := 1 + util.RandomIntN(10)

	ref, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}

	for i := 0; i < numIterators; i++ {
		docs := make([]int, 0)
		for doc := util.RandomIntN(1000); doc < maxDoc; doc += 1 + util.RandomIntN(100) {
			docs = append(docs, doc)
			ref.Set(doc)
		}
		// Add via iterator
		if len(docs) > 0 {
			adder := builder.Grow(len(docs))
			for _, doc := range docs {
				adder.Add(doc)
			}
		}
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should be BitDocIdSet for dense data
	if _, ok := result.(*util.BitDocIdSet); !ok {
		t.Errorf("Expected BitDocIdSet for dense data, got %T", result)
	}

	expected, err := util.NewBitDocIdSetWithCardinality(ref)
	if err != nil {
		t.Fatalf("Failed to create BitDocIdSet: %v", err)
	}
	assertDocIdSetEquals(t, expected, result)
}

// TestDocIdSetBuilder_Random tests with random data
func TestDocIdSetBuilder_Random(t *testing.T) {
	maxDoc := util.RandomIntN(100000) + 1
	if maxDoc < 2 {
		maxDoc = 2
	}

	for i := 1; i < maxDoc/2; i <<= 1 {
		numDocs := util.RandomIntN(i) + 1
		docs := make(map[int]bool)

		// Generate unique random docs
		for len(docs) < numDocs {
			d := util.RandomIntN(maxDoc)
			docs[d] = true
		}

		// Convert to array
		array := make([]int, 0, numDocs+util.RandomIntN(100))
		for doc := range docs {
			array = append(array, doc)
		}

		// Add some duplicates
		for len(array) < cap(array) {
			array = append(array, array[util.RandomIntN(numDocs)])
		}

		// Shuffle
		for j := len(array) - 1; j >= 1; j-- {
			k := util.RandomIntN(j + 1)
			array[j], array[k] = array[k], array[j]
		}

		// Build using DocIdSetBuilder
		builder := util.NewDocIdSetBuilder(maxDoc)
		j := 0
		for j < len(array) {
			l := util.RandomIntN(len(array)-j) + 1
			if util.RandomBool() {
				// Add one by one with budget
				budget := 0
				var adder util.BulkAdder
				for k := 0; k < l; k++ {
					if budget == 0 || util.RandomIntN(10) == 0 {
						budget = util.RandomIntN(l-k) + 1
						adder = builder.Grow(budget)
					}
					adder.Add(array[j])
					j++
					budget--
				}
			} else {
				// Add batch
				adder := builder.Grow(l)
				adder.AddBatch(array[j : j+l])
				j += l
			}
		}

		result, err := builder.Build()
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		// Create expected result
		ref, _ := util.NewFixedBitSet(maxDoc)
		for doc := range docs {
			ref.Set(doc)
		}
		expected, _ := util.NewBitDocIdSetWithCardinality(ref)

		assertDocIdSetEquals(t, expected, result)
	}
}

// TestDocIdSetBuilder_MisleadingDISICost tests with wrong cost estimates
func TestDocIdSetBuilder_MisleadingDISICost(t *testing.T) {
	maxDoc := util.RandomIntN(9000) + 1000
	builder := util.NewDocIdSetBuilder(maxDoc)
	expected, _ := util.NewFixedBitSet(maxDoc)

	for i := 0; i < 10; i++ {
		docs, _ := util.NewFixedBitSet(maxDoc)
		numDocs := util.RandomIntN(maxDoc / 1000)
		for j := 0; j < numDocs; j++ {
			docs.Set(util.RandomIntN(maxDoc))
		}
		expected.Or(docs)

		// Create iterator with cost 0 (misleading)
		iter := util.NewBitSetIterator(docs, 0)
		err := builder.Add(iter)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	expectedSet, _ := util.NewBitDocIdSetWithCardinality(expected)
	assertDocIdSetEquals(t, expectedSet, result)
}

// TestDocIdSetBuilder_LeverageStats tests leveraging stats
func TestDocIdSetBuilder_LeverageStats(t *testing.T) {
	// Test with single-valued stats (docCount == valueCount)
	builder := util.NewDocIdSetBuilderWithStats(100, 42, 42)
	if builder.Multivalued {
		t.Error("Expected single-valued, got multivalued")
	}
	if math.Abs(builder.NumValuesPerDoc-1.0) > 0.0001 {
		t.Errorf("Expected NumValuesPerDoc=1.0, got %f", builder.NumValuesPerDoc)
	}

	adder := builder.Grow(2)
	adder.Add(5)
	adder.Add(7)
	set, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if _, ok := set.(*util.BitDocIdSet); !ok {
		t.Errorf("Expected BitDocIdSet, got %T", set)
	}
	if set.Iterator().Cost() != 2 {
		t.Errorf("Expected cost 2, got %d", set.Iterator().Cost())
	}

	// Test with multi-valued stats (docCount < valueCount)
	builder = util.NewDocIdSetBuilderWithStats(100, 42, 63)
	if !builder.Multivalued {
		t.Error("Expected multivalued")
	}
	if math.Abs(builder.NumValuesPerDoc-1.5) > 0.0001 {
		t.Errorf("Expected NumValuesPerDoc=1.5, got %f", builder.NumValuesPerDoc)
	}

	adder = builder.Grow(2)
	adder.Add(5)
	adder.Add(7)
	set, err = builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if _, ok := set.(*util.BitDocIdSet); !ok {
		t.Errorf("Expected BitDocIdSet, got %T", set)
	}
	// Cost should be 1 because it thinks the same doc was added twice
	if set.Iterator().Cost() != 1 {
		t.Errorf("Expected cost 1, got %d", set.Iterator().Cost())
	}

	// Test with incomplete stats (docCount = -1)
	builder = util.NewDocIdSetBuilderWithStats(100, -1, 84)
	if !builder.Multivalued {
		t.Error("Expected multivalued for incomplete stats")
	}
	if math.Abs(builder.NumValuesPerDoc-1.0) > 0.0001 {
		t.Errorf("Expected NumValuesPerDoc=1.0 for incomplete stats, got %f", builder.NumValuesPerDoc)
	}

	// Test with incomplete stats (valueCount = -1)
	builder = util.NewDocIdSetBuilderWithStats(100, 42, -1)
	if !builder.Multivalued {
		t.Error("Expected multivalued for incomplete stats")
	}
	if math.Abs(builder.NumValuesPerDoc-1.0) > 0.0001 {
		t.Errorf("Expected NumValuesPerDoc=1.0 for incomplete stats, got %f", builder.NumValuesPerDoc)
	}
}

// TestDocIdSetBuilder_CostIsCorrectAfterBitsetUpgrade tests cost after upgrading to bitset
func TestDocIdSetBuilder_CostIsCorrectAfterBitsetUpgrade(t *testing.T) {
	maxDoc := 1000000
	builder := util.NewDocIdSetBuilder(maxDoc)

	// Add enough iterators to trigger bitset upgrade
	// 1000000 >> 6 is greater than threshold which is 1000000 >> 7
	for i := 0; i < maxDoc>>6; i++ {
		iter := util.NewRangeDocIdSetIterator(i, i+1)
		err := builder.Add(iter)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, ok := result.(*util.BitDocIdSet); !ok {
		t.Errorf("Expected BitDocIdSet, got %T", result)
	}

	expectedCost := int64(maxDoc >> 6)
	if result.Iterator().Cost() != expectedCost {
		t.Errorf("Expected cost %d, got %d", expectedCost, result.Iterator().Cost())
	}
}

// TestDocIdSetBuilder_AddIterator tests adding from DocIdSetIterator
func TestDocIdSetBuilder_AddIterator(t *testing.T) {
	maxDoc := 1000
	builder := util.NewDocIdSetBuilder(maxDoc)

	// Create a bitset with some docs
	bits, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i += 3 {
		bits.Set(i)
	}

	iter := util.NewBitSetIterator(bits, int64(bits.Cardinality()))
	err := builder.Add(iter)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify result
	resultIter := result.Iterator()
	count := 0
	for {
		doc, err := resultIter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		if doc%3 != 0 {
			t.Errorf("Expected doc divisible by 3, got %d", doc)
		}
		count++
	}
	expectedCount := (maxDoc + 2) / 3 // ceil(maxDoc/3)
	if count != expectedCount {
		t.Errorf("Expected %d docs, got %d", expectedCount, count)
	}
}

// TestDocIdSetBuilder_Grow tests the Grow method
func TestDocIdSetBuilder_Grow(t *testing.T) {
	maxDoc := 1000
	builder := util.NewDocIdSetBuilder(maxDoc)

	// Grow and add some docs
	adder := builder.Grow(10)
	for i := 0; i < 10; i++ {
		adder.Add(i * 2)
	}

	// Grow again
	adder = builder.Grow(5)
	for i := 0; i < 5; i++ {
		adder.Add(i*2 + 1)
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify all docs are present
	resultIter := result.Iterator()
	found := make(map[int]bool)
	for {
		doc, err := resultIter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		found[doc] = true
	}

	// First batch adds: 0, 2, 4, 6, 8, 10, 12, 14, 16, 18
	// Second batch adds: 1, 3, 5, 7, 9
	// So we should find: 0-10 and 12, 14, 16, 18
	expectedDocs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 14, 16, 18}
	for _, doc := range expectedDocs {
		if !found[doc] {
			t.Errorf("Expected to find doc %d", doc)
		}
	}

	// Verify we have exactly 15 docs
	if len(found) != 15 {
		t.Errorf("Expected 15 docs, got %d", len(found))
	}
}

// TestDocIdSetBuilder_BatchAdd tests batch adding
func TestDocIdSetBuilder_BatchAdd(t *testing.T) {
	maxDoc := 1000
	builder := util.NewDocIdSetBuilder(maxDoc)

	docs := []int{10, 20, 30, 40, 50}
	adder := builder.Grow(len(docs))
	adder.AddBatch(docs)

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	resultIter := result.Iterator()
	idx := 0
	for {
		doc, err := resultIter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		if idx >= len(docs) {
			t.Errorf("More docs than expected")
			break
		}
		if doc != docs[idx] {
			t.Errorf("Expected doc %d, got %d", docs[idx], doc)
		}
		idx++
	}
	if idx != len(docs) {
		t.Errorf("Expected %d docs, got %d", len(docs), idx)
	}
}

// TestDocIdSetBuilder_Duplicates tests handling of duplicate doc IDs
func TestDocIdSetBuilder_Duplicates(t *testing.T) {
	maxDoc := 100
	builder := util.NewDocIdSetBuilderWithStats(maxDoc, 10, 20) // multivalued

	// Add same doc multiple times
	adder := builder.Grow(5)
	for i := 0; i < 5; i++ {
		adder.Add(42) // Same doc ID
	}

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should only have one doc
	resultIter := result.Iterator()
	count := 0
	for {
		doc, err := resultIter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		if doc != 42 {
			t.Errorf("Expected doc 42, got %d", doc)
		}
		count++
	}
	if count != 1 {
		t.Errorf("Expected 1 unique doc, got %d", count)
	}
}

// TestDocIdSetBuilder_EmptyMaxDoc tests with small maxDoc
func TestDocIdSetBuilder_EmptyMaxDoc(t *testing.T) {
	maxDoc := 1
	builder := util.NewDocIdSetBuilder(maxDoc)

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should return nil or empty
	if result != nil {
		iter := result.Iterator()
		doc, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("Error iterating: %v", err)
		}
		if doc != util.NO_MORE_DOCS {
			t.Errorf("Expected NO_MORE_DOCS, got %d", doc)
		}
	}
}
