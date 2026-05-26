// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// live_docs_compat_test.go cross-validates the .liv (Lucene90LiveDocs)
// bitmap. The audit rows covered here are .liv parsing AND the
// soft-deletes carve-out for the hard-delete path: Lucene's writer
// emits the same .liv format for both kinds of deletion, so this file
// pins the hard-delete contract while soft_deletes_compat_test.go pins
// the soft-delete one.
//
// Audit rows cited (docs/compat-coverage.tsv, package == "index"):
//
//	".liv (Lucene90LiveDocsFormat)" — gap_notes:
//	  "Existing tests only assert the codec header on .liv; no test
//	   reads the actual bitset and confirms the deleted ordinals
//	   match the writer's expectations."
//	"soft-deletes" — partial coverage; see soft_deletes_compat_test.go
//	  for the full row citation.
//
// Three classes per file:
//
//	(a) Lucene-emitted .liv -> Gocene parses the bitset -> live count
//	    and specific deleted ordinals match the scenario contract.
//	(b) Byte-determinism — the bitset content is stable across two
//	    regenerations at the same seed.
//	(c) Cross-engine — the new index-deletions-and-dv-updates fixture
//	    carries a generational .liv at delGen=1, parsing under both
//	    canary seeds.
package index

import (
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLiveDocs_LiveDocsFormatScenarioBitset (class a) confirms the
// .liv emitted by the live-docs-format scenario (which deletes
// "id-3" and "id-7" out of 10 docs) is parsed correctly: 8 live docs,
// ordinals 3 and 7 cleared, all other ordinals set.
func TestLiveDocs_LiveDocsFormatScenarioBitset(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioLiveDocs, seed)
			seg := openSegmentInfo(t, dir)
			sci := openSegmentInfos(t, dir).Get(0)
			bits := openLiveDocs(t, dir, seg, sci.DelGen(), sci.DelCount())
			if bits.Length() != seg.DocCount() {
				t.Fatalf("bitset length = %d, want %d (maxDoc)",
					bits.Length(), seg.DocCount())
			}
			live := countLive(bits)
			if live != seg.DocCount()-2 {
				t.Errorf("live = %d, want %d", live, seg.DocCount()-2)
			}
			// Specific ordinals: the live-docs scenario deletes id-3 and
			// id-7. Lucene assigns docids in insertion order under
			// NoMergePolicy, so ordinals 3 and 7 must be cleared.
			if bits.Get(3) {
				t.Errorf("ordinal 3 (id-3) is live, want deleted")
			}
			if bits.Get(7) {
				t.Errorf("ordinal 7 (id-7) is live, want deleted")
			}
			for _, alive := range []int{0, 1, 2, 4, 5, 6, 8, 9} {
				if !bits.Get(alive) {
					t.Errorf("ordinal %d is deleted, want live", alive)
				}
			}
		})
	}
}

// TestLiveDocs_DeletionsScenarioBitset (class c) cross-engine: the
// new index-deletions-and-dv-updates fixture deletes "doc-3" and
// "doc-7" out of 12. The .liv is at delGen=1 with delCount=2.
func TestLiveDocs_DeletionsScenarioBitset(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			seg := openSegmentInfo(t, dir)
			sci := openSegmentInfos(t, dir).Get(0)
			if sci.DelGen() != 1 {
				t.Fatalf("DelGen = %d, want 1", sci.DelGen())
			}
			bits := openLiveDocs(t, dir, seg, sci.DelGen(), sci.DelCount())
			if got, want := countLive(bits), 10; got != want {
				t.Errorf("live count = %d, want %d", got, want)
			}
		})
	}
}

// TestLiveDocs_ByteDeterminism (class b) confirms the parsed bitset
// content is stable across regenerations.
func TestLiveDocs_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioLiveDocs, seed)
			b := generate(t, ScenarioLiveDocs, seed)
			segA := openSegmentInfo(t, a)
			segB := openSegmentInfo(t, b)
			sciA := openSegmentInfos(t, a).Get(0)
			sciB := openSegmentInfos(t, b).Get(0)
			bitsA := openLiveDocs(t, a, segA, sciA.DelGen(), sciA.DelCount())
			bitsB := openLiveDocs(t, b, segB, sciB.DelGen(), sciB.DelCount())
			if bitsA.Length() != bitsB.Length() {
				t.Fatalf("bitset length drift: %d vs %d",
					bitsA.Length(), bitsB.Length())
			}
			for i := 0; i < bitsA.Length(); i++ {
				if bitsA.Get(i) != bitsB.Get(i) {
					t.Fatalf("bit %d drift: A=%v B=%v",
						i, bitsA.Get(i), bitsB.Get(i))
				}
			}
		})
	}
}

// openLiveDocs reads .liv via Gocene's Lucene90LiveDocsFormat. The
// delGen, delCount and maxDoc must match what segments_N records.
func openLiveDocs(t *testing.T, dir string, seg *gindex.SegmentInfo, delGen int64, delCount int) util.Bits {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	livFormat := gcodecs.NewLucene90LiveDocsFormat()
	bits, _, err := livFormat.ReadLiveDocsLucene90(d, seg, delGen, delCount, seg.DocCount())
	if err != nil {
		t.Fatalf("ReadLiveDocsLucene90(gen=%d,count=%d): %v", delGen, delCount, err)
	}
	return bits
}

// countLive returns the number of set bits in b.
func countLive(b util.Bits) int {
	n := 0
	for i := 0; i < b.Length(); i++ {
		if b.Get(i) {
			n++
		}
	}
	return n
}
