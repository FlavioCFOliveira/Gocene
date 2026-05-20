// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package blocktree implements the Lucene 4.0 backward-compatibility
// block-tree terms dictionary reader.
package blocktree

import "fmt"

// Stats holds block-tree statistics for a single field, as returned by
// FieldReader.GetStats.
//
// Port of org.apache.lucene.backward_codecs.lucene40.blocktree.Stats
// (Lucene 10.4.0).
type Stats struct {
	// IndexNumBytes is the byte size of the field's FST terms index.
	IndexNumBytes int64

	// TotalTermCount is the total number of unique terms.
	TotalTermCount int64

	// TotalTermBytes is the sum of term lengths across all terms.
	TotalTermBytes int64

	// NonFloorBlockCount is the number of normal (non-floor) blocks.
	NonFloorBlockCount int

	// FloorBlockCount is the number of floor meta-blocks.
	FloorBlockCount int

	// FloorSubBlockCount is the number of sub-blocks inside floor blocks.
	FloorSubBlockCount int

	// MixedBlockCount is the number of blocks that contain both terms and
	// sub-blocks.
	MixedBlockCount int

	// TermsOnlyBlockCount is the number of leaf blocks (terms only, no
	// sub-blocks).
	TermsOnlyBlockCount int

	// SubBlocksOnlyBlockCount is the number of internal blocks that have
	// only sub-blocks (no terms).
	SubBlocksOnlyBlockCount int

	// TotalBlockCount is the total number of blocks.
	TotalBlockCount int

	// BlockCountByPrefixLen records the number of blocks at each prefix
	// depth. The slice is grown on demand.
	BlockCountByPrefixLen []int

	// TotalBlockSuffixBytes is the total number of bytes used to store term
	// suffixes (after compression).
	TotalBlockSuffixBytes int64

	// CompressionAlgorithms records how many times each CompressionAlgorithm
	// has been used. Index = algorithm code.
	CompressionAlgorithms [3]int64

	// TotalUncompressedBlockSuffixBytes is the total number of suffix bytes
	// before compression.
	TotalUncompressedBlockSuffixBytes int64

	// TotalBlockStatsBytes is the total bytes used for term-stats encoding
	// (not including what the PostingsReaderBase stores).
	TotalBlockStatsBytes int64

	// TotalBlockOtherBytes is the total bytes stored by the PostingsReaderBase
	// plus a few extra VInts per frame.
	TotalBlockOtherBytes int64

	// Segment is the name of the owning segment.
	Segment string

	// Field is the name of the field.
	Field string

	// internal counters used by frame methods
	startBlockCount int
	endBlockCount   int
}

// NewStats returns a Stats for the given segment and field.
func NewStats(segment, field string) *Stats {
	return &Stats{
		Segment:               segment,
		Field:                 field,
		BlockCountByPrefixLen: make([]int, 10),
	}
}

// String renders a human-readable summary matching the Java implementation.
func (s *Stats) String() string {
	var b []byte
	add := func(line string) { b = append(b, line...) }

	add("  index FST:\n")
	add(fmt.Sprintf("    %d bytes\n", s.IndexNumBytes))
	add("  terms:\n")
	add(fmt.Sprintf("    %d terms\n", s.TotalTermCount))
	if s.TotalTermCount != 0 {
		add(fmt.Sprintf("    %d bytes (%.1f bytes/term)\n",
			s.TotalTermBytes,
			float64(s.TotalTermBytes)/float64(s.TotalTermCount)))
	} else {
		add(fmt.Sprintf("    %d bytes\n", s.TotalTermBytes))
	}
	add("  blocks:\n")
	add(fmt.Sprintf("    %d blocks\n", s.TotalBlockCount))
	add(fmt.Sprintf("    %d terms-only blocks\n", s.TermsOnlyBlockCount))
	add(fmt.Sprintf("    %d sub-block-only blocks\n", s.SubBlocksOnlyBlockCount))
	add(fmt.Sprintf("    %d mixed blocks\n", s.MixedBlockCount))
	add(fmt.Sprintf("    %d floor blocks\n", s.FloorBlockCount))
	add(fmt.Sprintf("    %d non-floor blocks\n", s.TotalBlockCount-s.FloorSubBlockCount))
	add(fmt.Sprintf("    %d floor sub-blocks\n", s.FloorSubBlockCount))

	if s.TotalBlockCount != 0 {
		add(fmt.Sprintf("    %d term suffix bytes before compression (%.1f suffix-bytes/block)\n",
			s.TotalUncompressedBlockSuffixBytes,
			float64(s.TotalBlockSuffixBytes)/float64(s.TotalBlockCount)))
	} else {
		add(fmt.Sprintf("    %d term suffix bytes before compression\n",
			s.TotalUncompressedBlockSuffixBytes))
	}
	add(fmt.Sprintf("    %d compressed term suffix bytes", s.TotalBlockSuffixBytes))
	if s.TotalBlockCount != 0 && s.TotalUncompressedBlockSuffixBytes != 0 {
		add(fmt.Sprintf(" (%.2f compression ratio",
			float64(s.TotalBlockSuffixBytes)/float64(s.TotalUncompressedBlockSuffixBytes)))
		sep := false
		for code, cnt := range s.CompressionAlgorithms {
			if cnt == 0 {
				continue
			}
			alg, _ := CompressionAlgorithmByCode(code)
			if sep {
				add(", ")
			} else {
				add(" - compression count by algorithm: ")
				sep = true
			}
			add(fmt.Sprintf("%s: %d", alg, cnt))
		}
		add(")")
	}
	add("\n")

	if s.TotalBlockCount != 0 {
		add(fmt.Sprintf("    %d term stats bytes (%.1f stats-bytes/block)\n",
			s.TotalBlockStatsBytes,
			float64(s.TotalBlockStatsBytes)/float64(s.TotalBlockCount)))
		add(fmt.Sprintf("    %d other bytes (%.1f other-bytes/block)\n",
			s.TotalBlockOtherBytes,
			float64(s.TotalBlockOtherBytes)/float64(s.TotalBlockCount)))
		add("    by prefix length:\n")
		total := 0
		for prefix, cnt := range s.BlockCountByPrefixLen {
			total += cnt
			if cnt != 0 {
				add(fmt.Sprintf("      %2d: %d\n", prefix, cnt))
			}
		}
	} else {
		add(fmt.Sprintf("    %d term stats bytes\n", s.TotalBlockStatsBytes))
		add(fmt.Sprintf("    %d other bytes\n", s.TotalBlockOtherBytes))
	}

	return string(b)
}
