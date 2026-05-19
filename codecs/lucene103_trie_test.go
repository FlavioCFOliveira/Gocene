// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference:
// lucene/core/src/test/org/apache/lucene/codecs/lucene103/blocktree/TestTrie.java
// (Apache Lucene 10.4.0).
//
// Ports the four upstream test methods that exercise TrieBuilder and
// TrieReader end-to-end: the ChildSaveStrategy chooser, a builder-only
// round-trip via Visit, and two save/load round-trips driven by random
// keys (the second one using very long terms, the third hammering single
// byte terms to vary the children distributions).

// trieTestSeed seeds the deterministic RNG used by the ported tests. Java
// LuceneTestCase reseeds per run; we lock the seed so failures can be
// reproduced exactly without depending on the JUnit infrastructure.
const trieTestSeed uint64 = 0x10358571E32C1ABC

// newTrieRand mirrors LuceneTestCase.random() in deterministic mode. Each
// test gets a fresh instance so subtest ordering does not leak state.
func newTrieRand() *rand.Rand {
	return rand.New(rand.NewPCG(trieTestSeed, trieTestSeed^0xA5A5A5A5A5A5A5A5))
}

// trieAtLeast is the rough Go equivalent of LuceneTestCase.atLeast(n).
// The upstream method multiplies n by the test multiplier; in non-nightly
// runs the multiplier is 1, so a direct passthrough preserves coverage.
func trieAtLeast(n int) int { return n }

// trieRandomBytes mirrors TestTrie.randomBytes(). The first byte is left
// at zero (Java's default for fresh arrays) and the remaining bytes pull
// from increasingly narrow ranges so the resulting keys share long common
// prefixes — the workload the trie is designed for.
func trieRandomBytes(r *rand.Rand) []byte {
	bytes := make([]byte, r.IntN(256)+1)
	for i := 1; i < len(bytes); i++ {
		bytes[i] = byte(r.IntN(1 << (i % 9)))
	}
	return bytes
}

// TestTrie_StrategyChoose mirrors TestTrie.testStrategyChoose. The chooser
// must pick the smallest-footprint strategy and break ties in favour of
// BITS (constant-time lookup at the read side).
func TestTrie_StrategyChoose(t *testing.T) {
	t.Parallel()
	cases := []struct {
		minLabel, maxLabel, labelCount int
		want                           *ChildSaveStrategy
		label                          string
	}{
		{0, 255, 226, &strategyReverseArray, "bits=32B vs reverse_array=31B → REVERSE_ARRAY"},
		{0, 255, 32, &strategyArray, "bits=32B vs array=31B → ARRAY"},
		{0, 255, 33, &strategyBits, "bits=32B == array=32B → BITS (tie-break)"},
		{0, 255, 225, &strategyBits, "bits=32B == reverse_array=32B → BITS (tie-break)"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := chooseChildSaveStrategy(tc.minLabel, tc.maxLabel, tc.labelCount)
			if got != tc.want {
				t.Fatalf("chooseChildSaveStrategy(%d,%d,%d) = %s, want %s",
					tc.minLabel, tc.maxLabel, tc.labelCount, got.name, tc.want.name)
			}
		})
	}
}

// TestTrie_RandomTerms mirrors TestTrie.testRandomTerms. The builder-side
// round-trip (testTrieBuilder) checks that Visit yields exactly the
// inserted (key, output) pairs in key order; the reader-side round-trip
// (testTrieLookup) checks that every key resolves to its output and that
// keys outside the inserted set fail at the right trie level.
func TestTrie_RandomTerms(t *testing.T) {
	t.Parallel()
	r := newTrieRand()
	supplier := func() []byte { return trieRandomBytes(r) }
	testTrieBuilderRoundTrip(t, supplier, trieAtLeast(1000))
	testTrieLookupRoundTrip(t, r, supplier, 12)
}

// TestTrie_VeryLongTerms mirrors TestTrie.testVeryLongTerms. The 65 535
// byte keys exercise the deep-DFS path inside TrieBuilder.saveNodes and
// stress every variable-byte path-length encoder.
func TestTrie_VeryLongTerms(t *testing.T) {
	t.Parallel()
	r := newTrieRand()
	supplier := func() []byte {
		bytes := make([]byte, 65535)
		for i := 1; i < len(bytes); i++ {
			bytes[i] = byte(r.IntN(i/256 + 1))
		}
		return bytes
	}
	testTrieLookupRoundTrip(t, r, supplier, 5)
}

// TestTrie_OneByteTerms mirrors TestTrie.testOneByteTerms. Single-byte
// keys force the multi-child branch at the root, which is exactly the
// node shape the three ChildSaveStrategy variants compete over.
func TestTrie_OneByteTerms(t *testing.T) {
	t.Parallel()
	r := newTrieRand()
	supplier := func() []byte { return []byte{byte(r.IntN(256))} }
	round := trieAtLeast(50)
	for i := 0; i < round; i++ {
		testTrieLookupRoundTrip(t, r, supplier, 10)
	}
}

// testTrieBuilderRoundTrip is the Go port of TestTrie.testTrieBuilder.
// It populates a TrieBuilder, double-checks the Append immutability
// invariant after each merge, then walks the trie with Visit and compares
// the visited (key, output) sequence against the inserted map ordered by
// key.
func testTrieBuilderRoundTrip(t *testing.T, supplier func() []byte, count int) {
	t.Helper()
	r := newTrieRand()
	// Phase 1 — populate the expected map. Duplicates collapse to the
	// last-written value, exactly matching Java's TreeMap.put semantics.
	expected := make(map[string]*TrieOutput)
	empty := ""
	expected[empty] = NewTrieOutput(0, false, util.NewBytesRef([]byte("emptyOutput")))
	for i := 0; i < count; i++ {
		k := string(supplier())
		v := NewTrieOutput(
			r.Int64N(int64(1)<<62),
			r.IntN(2) == 1,
			util.NewBytesRef(supplier()),
		)
		expected[k] = v
	}

	// Phase 2 — iterate the materialised map in key order so the trie is
	// fed each (key, value) pair exactly once with its final value.
	keys := sortedKeys(expected)

	trie := BytesRefToTrie(util.NewBytesRef([]byte(empty)), expected[empty])
	for _, k := range keys {
		if k == empty {
			continue
		}
		add := BytesRefToTrie(util.NewBytesRef([]byte(k)), expected[k])
		if err := trie.Append(add); err != nil {
			t.Fatalf("trie.Append(%q): %v", k, err)
		}
		// IllegalStateException equivalents: both directions must fail
		// because `add` is now DESTROYED.
		if err := add.Append(trie); err == nil {
			t.Fatalf("add.Append(trie) after merge should fail, key=%q", k)
		}
		if err := trie.Append(add); err == nil {
			t.Fatalf("trie.Append(add) twice should fail, key=%q", k)
		}
	}

	got := make(map[string]*TrieOutput)
	trie.Visit(func(key *util.BytesRef, output *TrieOutput) {
		got[string(key.ValidBytes())] = output
	})
	if len(got) != len(expected) {
		t.Fatalf("Visit yielded %d entries, want %d", len(got), len(expected))
	}
	for _, k := range keys {
		want := expected[k]
		have, ok := got[k]
		if !ok {
			t.Fatalf("Visit missed key %q", k)
		}
		if !trieOutputsEqual(have, want) {
			t.Fatalf("Visit output mismatch for key %q: got %+v, want %+v", k, have, want)
		}
	}
}

// testTrieLookupRoundTrip is the Go port of TestTrie.testTrieLookup. For
// each iteration it builds a trie of 2^iter keys, saves it, reopens it
// from the byte buffer directory, and exercises both the positive lookup
// path (every inserted key) and the negative path (random keys known to
// be absent, with the failure level asserted to match Java's
// Arrays.mismatch contract).
func testTrieLookupRoundTrip(t *testing.T, r *rand.Rand, supplier func() []byte, round int) {
	t.Helper()
	for iter := 1; iter <= round; iter++ {
		// Phase 1 — fill the expected map; collapse duplicates to the
		// last value, matching Java's TreeMap.put.
		expected := make(map[string]*TrieOutput)
		empty := ""
		expected[empty] = NewTrieOutput(0, false, util.NewBytesRef([]byte("emptyOutput")))
		n := 1 << iter
		for i := 0; i < n; i++ {
			k := string(supplier())
			var floor *util.BytesRef
			if r.IntN(2) == 1 {
				floor = nil
			} else {
				floor = util.NewBytesRef(supplier())
			}
			v := NewTrieOutput(r.Int64N(int64(1)<<62), r.IntN(2) == 1, floor)
			expected[k] = v
		}

		// Phase 2 — sort and feed the trie in key order with each key's
		// final value.
		keys := sortedKeys(expected)

		trie := BytesRefToTrie(util.NewBytesRef([]byte(empty)), expected[empty])
		for _, k := range keys {
			if k == empty {
				continue
			}
			add := BytesRefToTrie(util.NewBytesRef([]byte(k)), expected[k])
			if err := trie.Append(add); err != nil {
				t.Fatalf("iter=%d trie.Append(%q): %v", iter, k, err)
			}
			if err := add.Append(trie); err == nil {
				t.Fatalf("iter=%d add.Append(trie) after merge should fail", iter)
			}
			if err := trie.Append(add); err == nil {
				t.Fatalf("iter=%d trie.Append(add) twice should fail", iter)
			}
		}

		dir := store.NewByteBuffersDirectory()

		start, rootFP, end, err := saveTrieToDirectory(dir, trie)
		if err != nil {
			t.Fatalf("iter=%d save: %v", iter, err)
		}
		// Second save must fail (SAVED → SAVED transition refused).
		if err := saveTrieAgain(dir, trie); err == nil {
			t.Fatalf("iter=%d second save should fail", iter)
		}
		// Appending to a sealed trie must also fail.
		if err := trie.Append(BytesRefToTrie(util.NewBytesRefEmpty(), NewTrieOutput(0, true, nil))); err == nil {
			t.Fatalf("iter=%d append after save should fail", iter)
		}

		reader, closeFn, err := openTrieReader(dir, start, rootFP, end)
		if err != nil {
			t.Fatalf("iter=%d openTrieReader: %v", iter, err)
		}

		for _, k := range keys {
			assertTrieLookupResult(t, reader, []byte(k), expected[k])
		}

		// Negative lookups: for each randomly synthesised absent key,
		// compute the first level at which both surrounding inserted
		// keys diverge from it and assert lookupChild returns nil at
		// exactly that depth.
		testNotFound := trieAtLeast(100)
		for i := 0; i < testNotFound; i++ {
			key := trieRandomBytes(r)
			for {
				if _, dup := expected[string(key)]; !dup {
					break
				}
				key = trieRandomBytes(r)
			}
			var lastK string
			found := false
			for _, k := range keys {
				if k > string(key) {
					mismatch1 := bytePrefixMismatch([]byte(lastK), key)
					mismatch2 := bytePrefixMismatch([]byte(k), key)
					depth := mismatch1
					if mismatch2 > depth {
						depth = mismatch2
					}
					assertTrieNotFoundAtLevel(t, reader, key, depth)
					found = true
					break
				}
				lastK = k
			}
			_ = found // last key in the map may be smaller than `key`; absence is then proven by the previous loop hitting end.
		}
		closeFn()
		if err := dir.Close(); err != nil {
			t.Fatalf("iter=%d dir.Close: %v", iter, err)
		}
	}
}

// saveTrieToDirectory writes the trie's index payload to file "index" and
// the meta triple (start, rootFP, end) to file "meta". Returns the parsed
// triple for the read-side helper to slice the index file.
func saveTrieToDirectory(dir store.Directory, trie *TrieBuilder) (int64, int64, int64, error) {
	idx, err := dir.CreateOutput("index", store.IOContextDefault)
	if err != nil {
		return 0, 0, 0, err
	}
	meta, err := dir.CreateOutput("meta", store.IOContextDefault)
	if err != nil {
		_ = idx.Close()
		return 0, 0, 0, err
	}
	if err := trie.Save(meta, idx); err != nil {
		_ = idx.Close()
		_ = meta.Close()
		return 0, 0, 0, err
	}
	if err := idx.Close(); err != nil {
		_ = meta.Close()
		return 0, 0, 0, err
	}
	if err := meta.Close(); err != nil {
		return 0, 0, 0, err
	}

	metaIn, err := dir.OpenInput("meta", store.IOContextDefault)
	if err != nil {
		return 0, 0, 0, err
	}
	defer metaIn.Close()
	start, err := store.ReadVLong(metaIn)
	if err != nil {
		return 0, 0, 0, err
	}
	rootFP, err := store.ReadVLong(metaIn)
	if err != nil {
		return 0, 0, 0, err
	}
	end, err := store.ReadVLong(metaIn)
	if err != nil {
		return 0, 0, 0, err
	}
	return start, rootFP, end, nil
}

// saveTrieAgain attempts to re-save a sealed trie; the underlying meta /
// index files are dummies that will never be filled because Save refuses
// to run when status != BUILDING.
func saveTrieAgain(dir store.Directory, trie *TrieBuilder) error {
	idx, err := dir.CreateOutput("index2", store.IOContextDefault)
	if err != nil {
		return err
	}
	meta, err := dir.CreateOutput("meta2", store.IOContextDefault)
	if err != nil {
		_ = idx.Close()
		return err
	}
	saveErr := trie.Save(meta, idx)
	_ = idx.Close()
	_ = meta.Close()
	return saveErr
}

// openTrieReader is the Go equivalent of the Java try-with-resources
// block that opens the index, slices out the trie payload, and constructs
// a TrieReader on top of it.
func openTrieReader(dir store.Directory, start, rootFP, end int64) (*TrieReader, func(), error) {
	idxIn, err := dir.OpenInput("index", store.IOContextDefault)
	if err != nil {
		return nil, nil, err
	}
	slice, err := idxIn.Slice("outputs", start, end-start)
	if err != nil {
		_ = idxIn.Close()
		return nil, nil, err
	}
	reader, err := NewTrieReader(slice, rootFP)
	if err != nil {
		_ = slice.Close()
		_ = idxIn.Close()
		return nil, nil, err
	}
	cleanup := func() {
		_ = slice.Close()
		_ = idxIn.Close()
	}
	return reader, cleanup, nil
}

// assertTrieLookupResult mirrors TestTrie.assertResult: walk down the
// trie one label at a time and assert the terminal node carries the
// expected output (fp, hasTerms, floorData).
func assertTrieLookupResult(t *testing.T, reader *TrieReader, term []byte, expected *TrieOutput) {
	t.Helper()
	parent := reader.Root()
	child := NewTrieNode()
	for i := 0; i < len(term); i++ {
		found, err := reader.LookupChild(int(term[i])&0xFF, parent, child)
		if err != nil {
			t.Fatalf("LookupChild(%d) at depth %d failed: %v", term[i], i, err)
		}
		if found == nil {
			t.Fatalf("LookupChild(%d) at depth %d returned nil for term %x", term[i], i, term)
		}
		parent = child
		child = NewTrieNode()
	}
	if !parent.HasOutput() {
		t.Fatalf("terminal node for term %x has no output", term)
	}
	if parent.OutputFP != expected.FP {
		t.Fatalf("term %x: OutputFP = %d, want %d", term, parent.OutputFP, expected.FP)
	}
	if parent.HasTerms != expected.HasTerms {
		t.Fatalf("term %x: HasTerms = %v, want %v", term, parent.HasTerms, expected.HasTerms)
	}
	if expected.FloorData == nil {
		if parent.IsFloor() {
			t.Fatalf("term %x: IsFloor() = true, want false", term)
		}
		return
	}
	got := make([]byte, expected.FloorData.Length)
	in, err := reader.FloorData(parent)
	if err != nil {
		t.Fatalf("FloorData(term %x): %v", term, err)
	}
	if err := in.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes(floorData term %x): %v", term, err)
	}
	want := util.BytesRefDeepCopyOf(expected.FloorData).Bytes
	if !bytesEqual(got, want) {
		t.Fatalf("term %x: floor bytes = %x, want %x", term, got, want)
	}
}

// assertTrieNotFoundAtLevel mirrors TestTrie.assertNotFoundOnLevelN: the
// first nLevel labels of term must resolve, and the (nLevel+1)-th label
// must return nil.
func assertTrieNotFoundAtLevel(t *testing.T, reader *TrieReader, term []byte, nLevel int) {
	t.Helper()
	parent := reader.Root()
	child := NewTrieNode()
	for i := 0; i < len(term); i++ {
		found, err := reader.LookupChild(int(term[i])&0xFF, parent, child)
		if err != nil {
			t.Fatalf("LookupChild(%d) at depth %d failed: %v", term[i], i, err)
		}
		if i == nLevel {
			if found != nil {
				t.Fatalf("LookupChild(%d) at depth %d should be nil for absent term %x", term[i], i, term)
			}
			return
		}
		if found == nil {
			t.Fatalf("LookupChild(%d) at depth %d returned nil before expected level %d for term %x",
				term[i], i, nLevel, term)
		}
		parent = child
		child = NewTrieNode()
	}
}

// sortedKeys returns m's keys in ascending lexicographic byte order. The
// trie expects each Append to receive a strictly greater minKey, so the
// caller must always feed it pairs in this order.
func sortedKeys(m map[string]*TrieOutput) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// bytePrefixMismatch returns the index of the first differing byte
// between a and b, matching java.util.Arrays.mismatch semantics on the
// equal-length range plus the prefix-of behaviour: when one slice is a
// proper prefix of the other, the shorter length is returned.
func bytePrefixMismatch(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) == len(b) {
		return -1
	}
	return n
}

// trieOutputsEqual compares two TrieOutputs structurally; floor payloads
// are compared by valid bytes so callers can pass aliasing BytesRefs.
func trieOutputsEqual(a, b *TrieOutput) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.FP != b.FP || a.HasTerms != b.HasTerms {
		return false
	}
	if (a.FloorData == nil) != (b.FloorData == nil) {
		return false
	}
	if a.FloorData == nil {
		return true
	}
	return bytesEqual(a.FloorData.ValidBytes(), b.FloorData.ValidBytes())
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
