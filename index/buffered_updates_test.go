// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// TestBufferedUpdates
// Source: lucene/core/src/test/org/apache/lucene/index/TestBufferedUpdates.java
// Purpose: Unit test for BufferedUpdates - tests buffered deletes/updates handling
// and apply buffered updates during flush

package index

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// atLeast returns a value that is at least the given minimum, used for
// randomized testing similar to Lucene's atLeast() method
func atLeast(min int) int {
	// In actual Lucene tests, this uses a random multiplier
	// For deterministic tests, we use a fixed small multiplier
	return min + rand.Intn(3)
}

// mockQuery is a minimal Query implementation for testing purposes.
// This avoids import cycles with the search package.
type mockQuery struct {
	id int
}

func (q *mockQuery) Rewrite(reader *IndexReader) (Query, error) { return q, nil }
func (q *mockQuery) Clone() Query                               { return &mockQuery{id: q.id} }
func (q *mockQuery) Equals(other Query) bool {
	if o, ok := other.(*mockQuery); ok {
		return q.id == o.id
	}
	return false
}
func (q *mockQuery) HashCode() int { return q.id }
func (q *mockQuery) CreateWeight(searcher IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}

// TestBufferedUpdates_RamBytesUsed tests RAM usage tracking for BufferedUpdates
func TestBufferedUpdates_RamBytesUsed(t *testing.T) {
	bu := NewBufferedUpdates("seg1")

	// Initial state should have 0 RAM usage
	if bu.RamBytesUsed() != 0 {
		t.Errorf("Expected initial ramBytesUsed to be 0, got %d", bu.RamBytesUsed())
	}

	// Should not have any updates initially
	if bu.Any() {
		t.Error("Expected Any() to be false for empty BufferedUpdates")
	}

	// Add some query deletes
	queries := atLeast(1)
	for i := 0; i < queries; i++ {
		var docIDUpto int
		if rand.Intn(2) == 0 {
			docIDUpto = int(^uint(0) >> 1) // MaxInt
		} else {
			docIDUpto = rand.Intn(100000)
		}
		query := &mockQuery{id: rand.Intn(100)}
		bu.AddQuery(query, docIDUpto)
	}

	// Add some term deletes
	terms := atLeast(1)
	for i := 0; i < terms; i++ {
		var docIDUpto int
		if rand.Intn(2) == 0 {
			docIDUpto = int(^uint(0) >> 1) // MaxInt
		} else {
			docIDUpto = rand.Intn(100000)
		}
		term := NewTermFromBytesRef("id", util.NewBytesRef([]byte(intToString(rand.Intn(100)))))
		bu.AddTerm(term, docIDUpto)
	}

	// Now should have updates
	if !bu.Any() {
		t.Error("Expected Any() to be true after adding terms and queries")
	}

	totalUsed := bu.RamBytesUsed()
	if totalUsed <= 0 {
		t.Error("Expected ramBytesUsed to be > 0 after adding updates")
	}

	// Clear delete terms - should reduce RAM but queries remain
	bu.ClearDeleteTerms()
	if !bu.Any() {
		t.Error("Expected Any() to still be true after clearing terms (queries should remain)")
	}
	if totalUsed <= bu.RamBytesUsed() {
		t.Error("Expected ramBytesUsed to decrease after clearing terms")
	}

	// Clear all - should be empty again
	bu.Clear()
	if bu.Any() {
		t.Error("Expected Any() to be false after Clear()")
	}
	if bu.RamBytesUsed() != 0 {
		t.Errorf("Expected ramBytesUsed to be 0 after Clear(), got %d", bu.RamBytesUsed())
	}
}

// TestBufferedUpdates_DeletedTerms tests the DeletedTerms functionality
func TestBufferedUpdates_DeletedTerms(t *testing.T) {
	iters := atLeast(10)
	fields := []string{"a", "b", "c"}

	for iter := 0; iter < iters; iter++ {
		actual := NewDeletedTerms()

		// Should be empty initially
		if !actual.IsEmpty() {
			t.Error("Expected DeletedTerms to be empty initially")
		}

		expected := make(map[TermKey]int)
		termCount := atLeast(5000)
		maxBytesNum := rand.Intn(3) + 1

		for i := 0; i < termCount; i++ {
			byteNum := rand.Intn(maxBytesNum) + 1
			bytes := make([]byte, byteNum)
			rand.Read(bytes)
			field := fields[rand.Intn(len(fields))]
			term := NewTermFromBytesRef(field, util.NewBytesRef(bytes))
			value := rand.Intn(10000000)

			key := TermKey{Field: field, Bytes: string(bytes)}
			expected[key] = value
			actual.Put(term, value)
		}

		// Check size
		if actual.Size() != len(expected) {
			t.Errorf("Expected size %d, got %d", len(expected), actual.Size())
		}

		// Check all entries exist
		for key, expectedValue := range expected {
			term := NewTermFromBytesRef(key.Field, util.NewBytesRef([]byte(key.Bytes)))
			actualValue := actual.Get(term)
			if actualValue != expectedValue {
				t.Errorf("For term %v: expected %d, got %d", key, expectedValue, actualValue)
			}
		}

		// Build sorted lists for comparison
		expectedEntries := make([]TermEntry, 0, len(expected))
		for key, value := range expected {
			expectedEntries = append(expectedEntries, TermEntry{
				Field: key.Field,
				Bytes: []byte(key.Bytes),
				Value: value,
			})
		}
		sort.Slice(expectedEntries, func(i, j int) bool {
			if expectedEntries[i].Field != expectedEntries[j].Field {
				return expectedEntries[i].Field < expectedEntries[j].Field
			}
			return string(expectedEntries[i].Bytes) < string(expectedEntries[j].Bytes)
		})

		actualEntries := actual.ForEachOrdered()

		// Compare sorted lists
		if len(expectedEntries) != len(actualEntries) {
			t.Errorf("Expected %d entries, got %d", len(expectedEntries), len(actualEntries))
		} else {
			for i := range expectedEntries {
				if expectedEntries[i].Field != actualEntries[i].Field {
					t.Errorf("Entry %d: expected field %s, got %s", i, expectedEntries[i].Field, actualEntries[i].Field)
				}
				if string(expectedEntries[i].Bytes) != string(actualEntries[i].Bytes) {
					t.Errorf("Entry %d: bytes mismatch", i)
				}
				if expectedEntries[i].Value != actualEntries[i].Value {
					t.Errorf("Entry %d: expected value %d, got %d", i, expectedEntries[i].Value, actualEntries[i].Value)
				}
			}
		}

		// Clear and verify
		actual.Clear()
		if actual.Size() != 0 {
			t.Errorf("Expected size 0 after Clear(), got %d", actual.Size())
		}
		if !actual.IsEmpty() {
			t.Error("Expected IsEmpty() to be true after Clear()")
		}
		if actual.RamBytesUsed() != 0 {
			t.Errorf("Expected ramBytesUsed 0 after Clear(), got %d", actual.RamBytesUsed())
		}
	}
}

// TermKey is a helper struct used to key the expected-map in tests.
type TermKey struct {
	Field string
	Bytes string
}

// intToString converts an int to its decimal string representation without
// pulling in strconv, mirroring the in-test helper used by the Lucene port.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	if negative {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}
