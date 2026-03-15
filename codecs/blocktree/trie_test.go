// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file ported from Apache Lucene:
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene103/blocktree/TestTrie.java
// Purpose: Tests Trie data structure for blocktree

package blocktree_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/blocktree"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestTrie_StrategyChoose tests the ChildSaveStrategy.choose() method.
// This test verifies that the correct strategy is chosen based on byte usage.
// Source: TestTrie.testStrategyChoose()
func TestTrie_StrategyChoose(t *testing.T) {
	// bits use 32 bytes while reverse_array use 31 bytes, choose reverse_array
	strategy := blocktree.ChildSaveStrategyChoose(0, 255, 226)
	if strategy != blocktree.ChildSaveStrategyReverseArray {
		t.Errorf("Expected REVERSE_ARRAY for (0, 255, 226), got %v", strategy)
	}

	// array and bits both use 32 position bytes, we choose bits
	strategy = blocktree.ChildSaveStrategyChoose(0, 255, 33)
	if strategy != blocktree.ChildSaveStrategyBits {
		t.Errorf("Expected BITS for (0, 255, 33), got %v", strategy)
	}

	// reverse_array and bits both use 32 position bytes, we choose bits
	strategy = blocktree.ChildSaveStrategyChoose(0, 255, 225)
	if strategy != blocktree.ChildSaveStrategyBits {
		t.Errorf("Expected BITS for (0, 255, 225), got %v", strategy)
	}

	// bits use 32 bytes while array use 31 bytes, choose array
	strategy = blocktree.ChildSaveStrategyChoose(0, 255, 32)
	if strategy != blocktree.ChildSaveStrategyArray {
		t.Errorf("Expected ARRAY for (0, 255, 32), got %v", strategy)
	}
}

// TestTrie_RandomTerms tests trie building and lookup with random terms.
// Source: TestTrie.testRandomTerms()
func TestTrie_RandomTerms(t *testing.T) {
	supplier := randomBytes
	testTrieBuilder(t, supplier, atLeast(1000))
	testTrieLookup(t, supplier, 12)
}

// TestTrie_OneByteTerms heavily tests single byte terms to generate various label distribution.
// Source: TestTrie.testOneByteTerms()
func TestTrie_OneByteTerms(t *testing.T) {
	supplier := func() []byte {
		return []byte{byte(rand.Int())}
	}
	round := atLeast(5)
	for i := 0; i < round; i++ {
		testTrieLookup(t, supplier, 10)
	}
}

// testTrieBuilder tests building a trie with random keys and values.
// Source: TestTrie.testTrieBuilder()
func testTrieBuilder(t *testing.T, randomBytesSupplier func() []byte, count int) {
	expected := make(map[string]*blocktree.TrieOutput)
	emptyKey := util.NewBytesRefEmpty()
	emptyOutput := blocktree.NewTrieOutput(0, false, util.NewBytesRef([]byte("emptyOutput")))
	expected[emptyKey.String()] = emptyOutput

	for i := 0; i < count; i++ {
		key := util.NewBytesRef(randomBytesSupplier())
		value := blocktree.NewTrieOutput(
			rand.Int63()&(1<<62-1), // random positive long < 1L << 62
			rand.Int()%2 == 0,       // random boolean
			util.NewBytesRef(randomBytesSupplier()),
		)
		expected[key.String()] = value
	}

	trieBuilder := blocktree.BytesRefToTrie(emptyKey, emptyOutput)
	for keyStr, value := range expected {
		key := util.NewBytesRef([]byte(keyStr))
		if keyStr == "" {
			continue
		}
		add := blocktree.BytesRefToTrie(key, value)
		trieBuilder.Append(add)

		// Verify that appending to a destroyed trie throws error
		err := add.Append(trieBuilder)
		if err == nil {
			t.Error("Expected error when appending to destroyed trie, got nil")
		}

		// Verify that appending a destroyed trie throws error
		err = trieBuilder.Append(add)
		if err == nil {
			t.Error("Expected error when appending destroyed trie, got nil")
		}
	}

	actual := make(map[string]*blocktree.TrieOutput)
	trieBuilder.Visit(func(key *util.BytesRef, output *blocktree.TrieOutput) {
		actual[key.String()] = output
	})

	// Compare expected and actual
	if len(expected) != len(actual) {
		t.Errorf("Expected %d entries, got %d", len(expected), len(actual))
	}

	for key, expectedOutput := range expected {
		actualOutput, ok := actual[key]
		if !ok {
			t.Errorf("Missing key: %s", key)
			continue
		}
		if !trieOutputEquals(expectedOutput, actualOutput) {
			t.Errorf("Output mismatch for key %s: expected %+v, got %+v", key, expectedOutput, actualOutput)
		}
	}
}

// testTrieLookup tests trie lookup functionality with save/load.
// Source: TestTrie.testTrieLookup()
func testTrieLookup(t *testing.T, randomBytesSupplier func() []byte, round int) {
	for iter := 1; iter <= round; iter++ {
		expected := make(map[string]*blocktree.TrieOutput)
		emptyKey := util.NewBytesRefEmpty()
		emptyOutput := blocktree.NewTrieOutput(0, false, util.NewBytesRef([]byte("emptyOutput")))
		expected[emptyKey.String()] = emptyOutput

		n := 1 << iter
		for i := 0; i < n; i++ {
			key := util.NewBytesRef(randomBytesSupplier())
			var floorData *util.BytesRef
			if rand.Int()%2 == 0 {
				floorData = nil
			} else {
				floorData = util.NewBytesRef(randomBytesSupplier())
			}
			value := blocktree.NewTrieOutput(
				rand.Int63()&(1<<62-1),
				rand.Int()%2 == 0,
				floorData,
			)
			expected[key.String()] = value
		}

		trieBuilder := blocktree.BytesRefToTrie(emptyKey, emptyOutput)
		for keyStr, value := range expected {
			key := util.NewBytesRef([]byte(keyStr))
			if keyStr == "" {
				continue
			}
			add := blocktree.BytesRefToTrie(key, value)
			trieBuilder.Append(add)

			// Verify that appending to a destroyed trie throws error
			err := add.Append(trieBuilder)
			if err == nil {
				t.Error("Expected error when appending to destroyed trie, got nil")
			}

			// Verify that appending a destroyed trie throws error
			err = trieBuilder.Append(add)
			if err == nil {
				t.Error("Expected error when appending destroyed trie, got nil")
			}
		}

		directory := store.NewByteBuffersDirectory()
		defer directory.Close()

		indexOut, err := directory.CreateOutput("index", store.IOContextWrite)
		if err != nil {
			t.Fatalf("Failed to create index output: %v", err)
		}

		metaOut, err := directory.CreateOutput("meta", store.IOContextWrite)
		if err != nil {
			t.Fatalf("Failed to create meta output: %v", err)
		}

		err = trieBuilder.Save(metaOut, indexOut)
		if err != nil {
			t.Fatalf("Failed to save trie: %v", err)
		}

		// Verify that saving an already saved trie throws error
		err = trieBuilder.Save(metaOut, indexOut)
		if err == nil {
			t.Error("Expected error when saving already saved trie, got nil")
		}

		// Verify that appending to a saved trie throws error
		emptyTrie := blocktree.BytesRefToTrie(util.NewBytesRefEmpty(), blocktree.NewTrieOutput(0, true, nil))
		err = trieBuilder.Append(emptyTrie)
		if err == nil {
			t.Error("Expected error when appending to saved trie, got nil")
		}

		indexOut.Close()
		metaOut.Close()

		indexIn, err := directory.OpenInput("index", store.IOContextRead)
		if err != nil {
			t.Fatalf("Failed to open index input: %v", err)
		}
		defer indexIn.Close()

		metaIn, err := directory.OpenInput("meta", store.IOContextRead)
		if err != nil {
			t.Fatalf("Failed to open meta input: %v", err)
		}
		defer metaIn.Close()

		start, err := store.ReadVLong(metaIn)
		if err != nil {
			t.Fatalf("Failed to read start: %v", err)
		}

		rootFP, err := store.ReadVLong(metaIn)
		if err != nil {
			t.Fatalf("Failed to read rootFP: %v", err)
		}

		end, err := store.ReadVLong(metaIn)
		if err != nil {
			t.Fatalf("Failed to read end: %v", err)
		}

		slicedInput, err := indexIn.Slice("outputs", start, end-start)
		if err != nil {
			t.Fatalf("Failed to slice input: %v", err)
		}

		reader, err := blocktree.NewTrieReader(slicedInput, rootFP)
		if err != nil {
			t.Fatalf("Failed to create TrieReader: %v", err)
		}

		// Test all expected entries
		for keyStr, expectedOutput := range expected {
			key := util.NewBytesRef([]byte(keyStr))
			assertResult(t, reader, key, expectedOutput)
		}

		// Test not-found keys
		testNotFound := atLeast(100)
		for i := 0; i < testNotFound; i++ {
			key := util.NewBytesRef(randomBytes())
			for {
				if _, exists := expected[key.String()]; !exists {
					break
				}
				key = util.NewBytesRef(randomBytes())
			}

			var lastK *util.BytesRef
			for kStr := range expected {
				k := util.NewBytesRef([]byte(kStr))
				if util.BytesRefCompare(k, key) > 0 {
					if lastK != nil && util.BytesRefCompare(lastK, key) >= 0 {
						t.Error("lastK should be less than key")
					}
					mismatch1 := bytesMismatch(lastK, key)
					mismatch2 := bytesMismatch(k, key)
					assertNotFoundOnLevelN(t, reader, key, max(mismatch1, mismatch2))
					break
				}
				lastK = k
			}
		}
	}
}

// assertResult asserts that the trie lookup result matches the expected output.
// Source: TestTrie.assertResult()
func assertResult(t *testing.T, reader *blocktree.TrieReader, term *util.BytesRef, expected *blocktree.TrieOutput) {
	parent := reader.Root
	child := blocktree.NewTrieNode()

	termBytes := term.ValidBytes()
	for i := 0; i < len(termBytes); i++ {
		label := int(termBytes[i] & 0xFF)
		found, err := reader.LookupChild(label, parent, child)
		if err != nil {
			t.Fatalf("Error looking up child: %v", err)
		}
		if found == nil {
			t.Errorf("Expected to find child for label %d at position %d", label, i)
			return
		}
		parent = child
		child = blocktree.NewTrieNode()
	}

	if !parent.HasOutput() {
		t.Error("Expected parent to have output")
	}

	if parent.OutputFp != expected.Fp() {
		t.Errorf("Expected outputFp %d, got %d", expected.Fp(), parent.OutputFp)
	}

	if parent.HasTerms != expected.HasTerms() {
		t.Errorf("Expected hasTerms %v, got %v", expected.HasTerms(), parent.HasTerms)
	}

	if expected.FloorData() == nil {
		if parent.IsFloor() {
			t.Error("Expected parent to not be floor")
		}
	} else {
		if !parent.IsFloor() {
			t.Error("Expected parent to be floor")
		}
		floorData, err := parent.FloorData(reader)
		if err != nil {
			t.Fatalf("Error reading floor data: %v", err)
		}
		floorBytes := make([]byte, expected.FloorData().Length)
		err = floorData.ReadBytes(floorBytes)
		if err != nil {
			t.Fatalf("Error reading floor data bytes: %v", err)
		}
		expectedBytes := expected.FloorData().ValidBytes()
		if !bytes.Equal(floorBytes, expectedBytes) {
			t.Errorf("Floor data mismatch: expected %v, got %v", expectedBytes, floorBytes)
		}
	}
}

// assertNotFoundOnLevelN asserts that a term is not found at a specific level.
// Source: TestTrie.assertNotFoundOnLevelN()
func assertNotFoundOnLevelN(t *testing.T, reader *blocktree.TrieReader, term *util.BytesRef, n int) {
	parent := reader.Root
	child := blocktree.NewTrieNode()

	termBytes := term.ValidBytes()
	for i := 0; i < len(termBytes); i++ {
		label := int(termBytes[i] & 0xFF)
		found, err := reader.LookupChild(label, parent, child)
		if err != nil {
			t.Fatalf("Error looking up child: %v", err)
		}

		if i == n {
			if found != nil {
				t.Errorf("Expected not found at level %d, but found node", n)
			}
			return
		}

		if found == nil {
			t.Errorf("Expected to find child at level %d, but got nil", i)
			return
		}

		parent = child
		child = blocktree.NewTrieNode()
	}
}

// randomBytes generates random bytes for testing.
// Source: TestTrie.randomBytes()
func randomBytes() []byte {
	length := rand.Intn(256) + 1 // 1 to 256 bytes
	bytes := make([]byte, length)
	for i := 1; i < length; i++ {
		bytes[i] = byte(rand.Intn(1 << (i % 9)))
	}
	return bytes
}

// trieOutputEquals compares two TrieOutput values for equality.
func trieOutputEquals(a, b *blocktree.TrieOutput) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Fp() != b.Fp() {
		return false
	}
	if a.HasTerms() != b.HasTerms() {
		return false
	}
	return util.BytesRefEquals(a.FloorData(), b.FloorData())
}

// bytesMismatch finds the first index where two byte sequences differ.
// Returns the index of the first mismatch, or -1 if they are equal up to the length of the shorter sequence.
func bytesMismatch(a, b *util.BytesRef) int {
	if a == nil || b == nil {
		return 0
	}
	aBytes := a.ValidBytes()
	bBytes := b.ValidBytes()
	minLen := len(aBytes)
	if len(bBytes) < minLen {
		minLen = len(bBytes)
	}
	for i := 0; i < minLen; i++ {
		if aBytes[i] != bBytes[i] {
			return i
		}
	}
	if len(aBytes) != len(bBytes) {
		return minLen
	}
	return -1
}

// atLeast returns a value that is at least the given minimum, scaled for testing.
func atLeast(min int) int {
	// In Lucene tests, this scales with the test iteration
	// For simplicity, we return min * 2 to ensure good coverage
	return min * 2
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
