// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// makeFrame returns a SegmentTermsEnumFrame stub pre-populated for the
// happy path through Stats. The reader byte counts drive the
// totalUncompressedBlockSuffixBytes / totalBlockStatsBytes aggregates.
func makeFrame(t *testing.T, prefixLen int, isLeaf bool, entCount, termBlockOrd int, suffixBytes int64, fp, fpOrig, fpEnd int64, alg CompressionAlgorithm) *SegmentTermsEnumFrame {
	t.Helper()
	suffixes := NewSegmentTermsEnumReader(make([]byte, 32))
	lengths := suffixes
	stats := NewSegmentTermsEnumReader(make([]byte, 16))
	state := &BlockTermState{TermBlockOrd: termBlockOrd}
	return &SegmentTermsEnumFrame{
		FP:                  fp,
		FPOrig:              fpOrig,
		FPEnd:               fpEnd,
		PrefixLength:        prefixLen,
		TotalSuffixBytes:    suffixBytes,
		SuffixesReader:      suffixes,
		SuffixLengthsReader: lengths,
		StatsReader:         stats,
		CompressionAlg:      alg,
		EntCount:            entCount,
		IsLeafBlock:         isLeaf,
		State:               state,
	}
}

func TestLucene103BlockTreeStats_StartBlockNonFloor(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	// Plain leaf block: 5 terms, 4 prefix bytes.
	// args: prefixLen, isLeaf, entCount, termBlockOrd, suffixBytes, fp, fpOrig, fpEnd, alg
	frame := makeFrame(t, 4, true, 5, 5, 64, 100, 100, 200, CompressionNoCompression)
	if err := s.StartBlock(frame, false); err != nil {
		t.Fatalf("StartBlock: %v", err)
	}
	if err := s.EndBlock(frame); err != nil {
		t.Fatalf("EndBlock: %v", err)
	}
	if s.NonFloorBlockCount != 1 {
		t.Errorf("NonFloorBlockCount: want 1, got %d", s.NonFloorBlockCount)
	}
	if s.FloorBlockCount != 0 {
		t.Errorf("FloorBlockCount: want 0, got %d", s.FloorBlockCount)
	}
	if s.TermsOnlyBlockCount != 1 {
		t.Errorf("TermsOnlyBlockCount: want 1, got %d", s.TermsOnlyBlockCount)
	}
	if s.TotalTermCount != 5 {
		t.Errorf("TotalTermCount: want 5, got %d", s.TotalTermCount)
	}
	if s.TotalBlockSuffixBytes != 64 {
		t.Errorf("TotalBlockSuffixBytes: want 64, got %d", s.TotalBlockSuffixBytes)
	}
	if got := s.CompressionAlgorithms[CompressionNoCompression.Code()]; got != 1 {
		t.Errorf("CompressionAlgorithms[NO_COMPRESSION]: want 1, got %d", got)
	}
	if err := s.Finish(); err != nil {
		t.Errorf("Finish: %v", err)
	}
}

func TestLucene103BlockTreeStats_StartBlockFloorParentAndSubBlocks(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")

	// Floor parent: FP == FPOrig, isFloor=true bumps both FloorBlockCount
	// and FloorSubBlockCount.
	// args: prefixLen, isLeaf, entCount, termBlockOrd, suffixBytes, fp, fpOrig, fpEnd, alg
	parent := makeFrame(t, 2, false, 30, 25, 256, 500, 500, 1024, CompressionNoCompression)
	if err := s.StartBlock(parent, true); err != nil {
		t.Fatalf("StartBlock parent: %v", err)
	}
	if err := s.EndBlock(parent); err != nil {
		t.Fatalf("EndBlock parent: %v", err)
	}

	// Floor sub-block: FP != FPOrig bumps FloorSubBlockCount only.
	// args: prefixLen, isLeaf, entCount, termBlockOrd, suffixBytes, fp, fpOrig, fpEnd, alg
	sub := makeFrame(t, 2, false, 12, 8, 128, 1024, 500, 1280, CompressionNoCompression)
	if err := s.StartBlock(sub, true); err != nil {
		t.Fatalf("StartBlock sub: %v", err)
	}
	if err := s.EndBlock(sub); err != nil {
		t.Fatalf("EndBlock sub: %v", err)
	}

	if s.FloorBlockCount != 1 {
		t.Errorf("FloorBlockCount: want 1, got %d", s.FloorBlockCount)
	}
	if s.FloorSubBlockCount != 2 {
		t.Errorf("FloorSubBlockCount: want 2, got %d", s.FloorSubBlockCount)
	}
	if s.NonFloorBlockCount != 0 {
		t.Errorf("NonFloorBlockCount: want 0, got %d", s.NonFloorBlockCount)
	}
	if s.MixedBlockCount != 2 {
		// Both parent (25 terms + 5 sub-blocks) and sub (8 terms + 4 sub-blocks) are mixed.
		t.Errorf("MixedBlockCount: want 2, got %d", s.MixedBlockCount)
	}
	if err := s.Finish(); err != nil {
		t.Errorf("Finish: %v", err)
	}
}

func TestLucene103BlockTreeStats_BlockCountByPrefixLenGrowsBeyondInitialTen(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	// Initial capacity is 10; emit a block at prefix length 15 to force a grow.
	frame := makeFrame(t, 15, true, 3, 3, 8, 10, 10, 64, CompressionLZ4)
	if err := s.StartBlock(frame, false); err != nil {
		t.Fatalf("StartBlock: %v", err)
	}
	if err := s.EndBlock(frame); err != nil {
		t.Fatalf("EndBlock: %v", err)
	}
	if len(s.BlockCountByPrefixLen) <= 15 {
		t.Errorf("BlockCountByPrefixLen length: want > 15, got %d", len(s.BlockCountByPrefixLen))
	}
	if s.BlockCountByPrefixLen[15] != 1 {
		t.Errorf("BlockCountByPrefixLen[15]: want 1, got %d", s.BlockCountByPrefixLen[15])
	}
}

func TestLucene103BlockTreeStats_TermAccumulatesBytes(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	s.Term(util.NewBytesRef([]byte("hello")))
	s.Term(util.NewBytesRef([]byte("worldworld")))
	if s.TotalTermBytes != 15 {
		t.Errorf("TotalTermBytes: want 15, got %d", s.TotalTermBytes)
	}
}

func TestLucene103BlockTreeStats_FinishDetectsImbalance(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	frame := makeFrame(t, 1, true, 2, 2, 4, 0, 0, 16, CompressionNoCompression)
	if err := s.StartBlock(frame, false); err != nil {
		t.Fatalf("StartBlock: %v", err)
	}
	// Skip EndBlock to force an imbalance.
	if err := s.Finish(); err == nil {
		t.Fatal("Finish should report start/end imbalance")
	}
}

func TestLucene103BlockTreeStats_StringContainsHeaders(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	s.Term(util.NewBytesRef([]byte("a")))
	frame := makeFrame(t, 0, true, 1, 1, 1, 0, 0, 32, CompressionNoCompression)
	if err := s.StartBlock(frame, false); err != nil {
		t.Fatalf("StartBlock: %v", err)
	}
	if err := s.EndBlock(frame); err != nil {
		t.Fatalf("EndBlock: %v", err)
	}
	if err := s.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	out := s.String()
	for _, want := range []string{"index trie:", "terms:", "blocks:", "1 terms", "by prefix length:"} {
		if !strings.Contains(out, want) {
			t.Errorf("Stats.String() missing %q in:\n%s", want, out)
		}
	}
}

func TestLucene103BlockTreeStats_CompressionCodeOutOfRangeIsError(t *testing.T) {
	s := NewLucene103BlockTreeStats("_0", "field1")
	frame := makeFrame(t, 0, true, 1, 1, 1, 0, 0, 16, CompressionAlgorithm(7))
	if err := s.StartBlock(frame, false); err == nil {
		t.Fatal("StartBlock should reject out-of-range compression code")
	}
}
