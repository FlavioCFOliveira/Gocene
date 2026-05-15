// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Go counterpart to
// lucene/core/src/test/org/apache/lucene/util/fst/TestFSTDirectAddressing.java.
//
// The Java tests construct an FST from a list of pre-sorted inputs
// and then assert various properties about the resulting structure
// (locatability, size, RAM footprint). These three Go tests cover the
// non-@Nightly Java cases (testDenseWithGap, testDeDupTails) plus a
// deterministic substitute for the @Nightly testWorstCaseForDirectAddressing.

package fst

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestDenseWithGap mirrors the Java testDenseWithGap. Six entries
// share the labels 'a', 'b', 'c', 'd', 'f', 'g' (a gap at 'e'), which
// produces a direct-addressing node with a presence-bit hole. Each
// input must be locatable via BytesRefFSTEnum.SeekExact — this is
// what the Java original (TestFSTDirectAddressing.testDenseWithGap)
// uses, so the Go test exercises the same code path.
func TestDenseWithGap(t *testing.T) {
	words := []string{"ah", "bi", "cj", "dk", "fl", "gm"}
	entries := make([][]byte, len(words))
	for i, w := range words {
		entries[i] = []byte(w)
	}
	fst := buildNoOutputsFST(t, entries, DirectAddressingMaxOversizingFactor)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	for _, entry := range entries {
		got, err := enum.SeekExact(util.NewBytesRef(entry))
		if err != nil {
			t.Fatalf("SeekExact %q: %v", entry, err)
		}
		if got == nil {
			t.Fatalf("entry %q not found in FST", entry)
		}
	}
}

// TestDeDupTails mirrors the Java testDeDupTails. A synthetic 250 000
// entry dictionary is built (every fourth value in [0, 1_000_000) as
// a 3-byte big-endian integer) and the resulting FST byte count must
// stay within 1 % of the list-only baseline of 1648 bytes.
//
// This is the regression guard that catches direct-addressing tail
// dedup regressions. The threshold is identical to Lucene's.
func TestDeDupTails(t *testing.T) {
	const baselineBytes int64 = 1648
	tolerance := 0.01
	limit := int64(float64(baselineBytes) * (1.0 + tolerance))

	var entries [][]byte
	for i := 0; i < 1_000_000; i += 4 {
		b := make([]byte, 3)
		val := i
		for j := len(b) - 1; j >= 0; j-- {
			b[j] = byte(val & 0xff)
			val >>= 8
		}
		entries = append(entries, b)
	}
	fst := buildNoOutputsFST(t, entries, DirectAddressingMaxOversizingFactor)
	size := fst.NumBytes()
	if size > limit {
		t.Fatalf("FST size = %d B, exceeds limit %d B (baseline %d, tolerance %.0f %%)",
			size, limit, baselineBytes, tolerance*100)
	}
}

// TestWorstCaseForDirectAddressing is a deterministic counterpart to
// the Java @Nightly testWorstCaseForDirectAddressing. The Java test
// random-generates 1 000 000 5-byte entries (each byte made a multiple
// of 4 to force the worst-case oversizing); we use a seeded
// math/rand.Rand for reproducibility and shrink the corpus to a size
// that is exercised every run rather than only nightly. The
// assertion mirrors the Java original: enabling direct addressing on
// this worst-case input must not blow up the RAM footprint beyond
// the configured percentage limit.
func TestWorstCaseForDirectAddressing(t *testing.T) {
	const memoryIncreaseLimitPercent = 1.0
	const numWords = 50_000 // tractable subset of Java's 1_000_000

	rng := rand.New(rand.NewSource(0xCAFEBABE))
	wordSet := make(map[string]struct{}, numWords)
	for len(wordSet) < numWords {
		b := make([]byte, 5)
		rng.Read(b)
		for j := range b {
			b[j] &= 0xfc // make each byte a multiple of 4
		}
		wordSet[string(b)] = struct{}{}
	}
	words := make([][]byte, 0, len(wordSet))
	for w := range wordSet {
		words = append(words, []byte(w))
	}
	sort.Slice(words, func(i, j int) bool { return string(words[i]) < string(words[j]) })

	// Disable direct addressing (negative factor) and measure RAM.
	noDA := buildNoOutputsFST(t, words, -1.0)
	withDA := buildNoOutputsFST(t, words, DirectAddressingMaxOversizingFactor)

	ramNoDA := noDA.RAMBytesUsed()
	ramWithDA := withDA.RAMBytesUsed()
	if ramNoDA == 0 {
		t.Fatalf("baseline FST has zero RAM footprint")
	}
	increasePercent := (float64(ramWithDA)/float64(ramNoDA) - 1) * 100
	if increasePercent >= memoryIncreaseLimitPercent {
		t.Fatalf("direct-addressing FST RAM = %d B, no-DA RAM = %d B, increase = %.2f %% (limit %.2f %%)",
			ramWithDA, ramNoDA, increasePercent, memoryIncreaseLimitPercent)
	}
}

// buildNoOutputsFST is a small test helper that constructs an FST
// over the given byte-string entries using NoOutputs. Mirrors the
// Java helper buildFST. Entries must already be deduplicated and
// sorted; this is a precondition of FSTCompiler.Add.
func buildNoOutputsFST(t *testing.T, entries [][]byte, oversizing float32) *FST[*noOutputMarker] {
	t.Helper()
	compiler := NewFSTCompilerBuilder[*noOutputMarker](
		InputTypeByte1, NoOutputs(),
	).DirectAddressingMaxOversizingFactor(oversizing).Build()

	// Strict sort + dedup mirroring Lucene's `if (entry.equals(last) == false)`.
	sortedEntries := append([][]byte(nil), entries...)
	sort.Slice(sortedEntries, func(i, j int) bool { return string(sortedEntries[i]) < string(sortedEntries[j]) })
	var last []byte
	scratch := util.NewIntsRefBuilder()
	for _, entry := range sortedEntries {
		if last != nil && string(entry) == string(last) {
			continue
		}
		bytesToIntsRef(entry, scratch)
		if err := compiler.Add(scratch.Get(), NoOutputValue()); err != nil {
			t.Fatalf("Add: %v", err)
		}
		last = entry
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if meta == nil {
		t.Fatalf("Compile returned nil metadata")
	}
	fst, err := FromFSTReader[*noOutputMarker](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	return fst
}

// bytesToIntsRef populates scratch with the unsigned-byte values of
// entry; mirrors Lucene's Util.toIntsRef.
func bytesToIntsRef(entry []byte, scratch *util.IntsRefBuilder) {
	scratch.GrowNoCopy(len(entry))
	for i, b := range entry {
		scratch.SetIntAt(i, int(b))
	}
	scratch.SetLength(len(entry))
}
