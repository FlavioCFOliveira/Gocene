// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sort"
	"testing"
)

// Port of org.apache.lucene.index.TestTermVectorsReader (Lucene 10.4.0).
//
// Sprint 55, option (c): the full test fixture is reproduced below, but most
// test methods are skipped. The reference test exercises the end-to-end
// IndexWriter -> term-vectors codec -> DirectoryReader/TermVectorsReader
// pipeline, which depends on Gocene infrastructure not yet ported into the
// index package:
//
//   - IndexWriter.newestSegment() (SegmentCommitInfo of the last flush).
//   - IndexWriter.readFieldInfos(seg) (re-reading FieldInfos from a segment).
//   - Codec.getDefault().termVectorsFormat().vectorsReader(...) accessible
//     from the index package (the term-vectors format lives in codecs/ and
//     is not yet wired through a default-codec accessor here).
//   - The org.apache.lucene.tests helpers RandomIndexWriter and
//     expectThrows used by the testIllegal* methods.
//
// The data-builder portion of setUp() is faithfully ported and unit-tested
// by TestTermVectorsReader_Fixture, which is the only runnable check until
// the pipeline lands.

const termVectorsReaderTermFreq = 3

// testToken mirrors the inner class TestToken; ordered by position.
type testToken struct {
	text        string
	pos         int
	startOffset int
	endOffset   int
}

// termVectorsReaderFixture reproduces the fields and setUp() data builder of
// the reference TestTermVectorsReader.
type termVectorsReaderFixture struct {
	testFields         []string
	testFieldsStorePos []bool
	testFieldsStoreOff []bool
	testTerms          []string
	positions          [][]int
	tokens             []testToken
}

// newTermVectorsReaderFixture ports the deterministic part of setUp(): it
// sorts the terms, builds the positions matrix and the position-sorted token
// stream. The reference uses random()*10 jitter for positions; a fixed seed
// is used here so the fixture is reproducible.
func newTermVectorsReaderFixture() *termVectorsReaderFixture {
	f := &termVectorsReaderFixture{
		testFields:         []string{"f1", "f2", "f3", "f4"},
		testFieldsStorePos: []bool{true, false, true, false},
		testFieldsStoreOff: []bool{true, false, false, true},
		testTerms:          []string{"this", "is", "a", "test"},
	}

	sort.Strings(f.testTerms)

	f.positions = make([][]int, len(f.testTerms))
	f.tokens = make([]testToken, 0, len(f.testTerms)*termVectorsReaderTermFreq)
	for i := range f.testTerms {
		f.positions[i] = make([]int, termVectorsReaderTermFreq)
		for j := 0; j < termVectorsReaderTermFreq; j++ {
			// Deterministic stand-in for j*10 + random()*10: positions stay
			// sorted and the first is 0.
			f.positions[i][j] = j * 10
			f.tokens = append(f.tokens, testToken{
				text:        f.testTerms[i],
				pos:         f.positions[i][j],
				startOffset: j * 10,
				endOffset:   j*10 + len(f.testTerms[i]),
			})
		}
	}
	sort.SliceStable(f.tokens, func(a, b int) bool {
		return f.tokens[a].pos < f.tokens[b].pos
	})
	return f
}

// TestTermVectorsReader_Fixture validates the ported setUp() data builder.
func TestTermVectorsReader_Fixture(t *testing.T) {
	f := newTermVectorsReaderFixture()

	if got, want := f.testTerms, []string{"a", "is", "test", "this"}; !equalStrings(got, want) {
		t.Fatalf("testTerms not lexicographically sorted: got %v, want %v", got, want)
	}
	if len(f.tokens) != len(f.testTerms)*termVectorsReaderTermFreq {
		t.Fatalf("tokens len = %d, want %d", len(f.tokens), len(f.testTerms)*termVectorsReaderTermFreq)
	}
	for i, p := range f.positions {
		if len(p) != termVectorsReaderTermFreq {
			t.Fatalf("positions[%d] len = %d, want %d", i, len(p), termVectorsReaderTermFreq)
		}
		if p[0] != 0 {
			t.Errorf("positions[%d][0] = %d, want 0 (first position must be 0)", i, p[0])
		}
		for j := 1; j < len(p); j++ {
			if p[j] < p[j-1] {
				t.Errorf("positions[%d] not increasing: %v", i, p)
			}
		}
	}
	for i := 1; i < len(f.tokens); i++ {
		if f.tokens[i].pos < f.tokens[i-1].pos {
			t.Errorf("tokens not sorted by pos at %d: %v", i, f.tokens)
		}
	}
}

func equalStrings(a, b []string) bool {
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

// test ports TestTermVectorsReader.test().
func TestTermVectorsReader_FilesCreated(t *testing.T) {
	t.Skip("Sprint 55 option c: needs IndexWriter term-vectors flush + DirectoryReader.open with SegmentReader.getFieldInfos().hasTermVectors()")
}

// testReader ports TestTermVectorsReader.testReader().
func TestTermVectorsReader_Reader(t *testing.T) {
	t.Skip("Sprint 55 option c: needs Codec.getDefault().termVectorsFormat().vectorsReader and IndexWriter.newestSegment/readFieldInfos")
}

// testDocsEnum ports TestTermVectorsReader.testDocsEnum().
func TestTermVectorsReader_DocsEnum(t *testing.T) {
	t.Skip("Sprint 55 option c: needs term-vectors TermVectorsReader + PostingsEnum (NONE) over a flushed segment")
}

// testPositionReader ports TestTermVectorsReader.testPositionReader().
func TestTermVectorsReader_PositionReader(t *testing.T) {
	t.Skip("Sprint 55 option c: needs term-vectors TermVectorsReader + PostingsEnum positions/offsets over a flushed segment")
}

// testOffsetReader ports TestTermVectorsReader.testOffsetReader().
func TestTermVectorsReader_OffsetReader(t *testing.T) {
	t.Skip("Sprint 55 option c: needs term-vectors TermVectorsReader + PostingsEnum offsets over a flushed segment")
}

// testIllegalPayloadsWithoutPositions ports the same-named reference method.
func TestTermVectorsReader_IllegalPayloadsWithoutPositions(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalOffsetsWithoutVectors ports the same-named reference method.
func TestTermVectorsReader_IllegalOffsetsWithoutVectors(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalPositionsWithoutVectors ports the same-named reference method.
func TestTermVectorsReader_IllegalPositionsWithoutVectors(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalVectorPayloadsWithoutVectors ports the same-named reference method.
func TestTermVectorsReader_IllegalVectorPayloadsWithoutVectors(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalVectorsWithoutIndexed ports the same-named reference method.
func TestTermVectorsReader_IllegalVectorsWithoutIndexed(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalVectorPositionsWithoutIndexed ports the same-named reference method.
func TestTermVectorsReader_IllegalVectorPositionsWithoutIndexed(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalVectorOffsetsWithoutIndexed ports the same-named reference method.
func TestTermVectorsReader_IllegalVectorOffsetsWithoutIndexed(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}

// testIllegalVectorPayloadsWithoutIndexed ports the same-named reference method.
func TestTermVectorsReader_IllegalVectorPayloadsWithoutIndexed(t *testing.T) {
	t.Skip("Sprint 55 option c: needs RandomIndexWriter + expectThrows on IndexWriter.AddDocument validation")
}
