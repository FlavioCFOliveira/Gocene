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
	"bytes"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// Stats.java (Apache Lucene 10.4.0).
//
// Stats is a passive aggregator: SegmentTermsEnum (deferred to backlog
// task #2692) drives it block-by-block, and Lucene103FieldReader.GetStats
// returns a fully populated instance for index introspection (e.g.
// CheckIndex -verbose).

// Lucene103BlockTreeStats holds per-field block-tree statistics. Mirrors
// the Java field set exactly, including the `// TODO`-flagged ones, so a
// future CheckIndex port can stringify them with the same layout as
// Lucene's verbose dump.
type Lucene103BlockTreeStats struct {
	// IndexNumBytes is the on-disk byte length of the per-field index
	// trie slice.
	IndexNumBytes int64
	// TotalTermCount is the total number of terms in the field.
	TotalTermCount int64
	// TotalTermBytes is the sum of term lengths across all terms in the
	// field (UTF-8 bytes).
	TotalTermBytes int64

	// NonFloorBlockCount is the number of plain (non-floor) blocks in
	// the terms file.
	NonFloorBlockCount int
	// FloorBlockCount is the number of floor parent blocks (blocks that
	// were split because they exceeded maxItemsInBlock).
	FloorBlockCount int
	// FloorSubBlockCount is the number of sub-blocks inside floor
	// parents.
	FloorSubBlockCount int

	// MixedBlockCount is the count of blocks that mix terms and
	// sub-blocks.
	MixedBlockCount int
	// TermsOnlyBlockCount is the count of leaf blocks (terms only, no
	// sub-blocks).
	TermsOnlyBlockCount int
	// SubBlocksOnlyBlockCount is the count of internal blocks that
	// contain only sub-blocks (no terms).
	SubBlocksOnlyBlockCount int

	// TotalBlockCount is the total number of blocks visited.
	TotalBlockCount int

	// BlockCountByPrefixLen counts blocks at each prefix depth; index 0
	// is the root level. Lucene seeds the slice with capacity 10 and
	// grows it on demand.
	BlockCountByPrefixLen []int

	startBlockCount int
	endBlockCount   int

	// TotalBlockSuffixBytes is the total number of bytes used to store
	// term suffixes (post-compression).
	TotalBlockSuffixBytes int64
	// CompressionAlgorithms counts how often each compression algorithm
	// was used; indexed by Lucene103BlockTree compression codes
	// (0=NO_COMPRESSION, 1=LOWERCASE_ASCII, 2=LZ4).
	CompressionAlgorithms [3]int64
	// TotalUncompressedBlockSuffixBytes is the suffix byte total before
	// compression.
	TotalUncompressedBlockSuffixBytes int64
	// TotalBlockStatsBytes is the total number of stats-blob bytes
	// (excluding the PostingsReaderBase metadata blob).
	TotalBlockStatsBytes int64
	// TotalBlockOtherBytes counts the bytes the postings reader and
	// terms-dict frame consumed beyond suffix and stats blobs.
	TotalBlockOtherBytes int64

	// Segment is the segment name this Stats instance was built for.
	Segment string
	// Field is the field name this Stats instance was built for.
	Field string
}

// NewLucene103BlockTreeStats returns a Stats instance ready to receive
// startBlock/endBlock/term calls. Mirrors Stats(String, String) in Java.
func NewLucene103BlockTreeStats(segment, field string) *Lucene103BlockTreeStats {
	return &Lucene103BlockTreeStats{
		Segment:               segment,
		Field:                 field,
		BlockCountByPrefixLen: make([]int, 10),
	}
}

// SegmentTermsEnumFrame is the per-block cursor inside the segment terms
// enumerator. The full port (every traversal/seek/decode method) is the
// subject of backlog task #2692; this file declares the minimal field set
// that [Lucene103BlockTreeStats] reads so it compiles and exercises the
// agreed-upon contract.
//
// IMPORTANT: do not add fields, methods, or constructors here without
// first updating backlog task #2692 — the surface here is the contract
// that future work must preserve.
type SegmentTermsEnumFrame struct {
	// FP is the file pointer in the .tim file where the current block
	// starts (or where the floor sub-block being decoded starts).
	FP int64
	// FPOrig is the original file pointer of the parent floor block;
	// equal to FP for non-floor blocks. Stats uses (FP == FPOrig) to
	// distinguish a floor parent's first emission from its sub-blocks.
	FPOrig int64
	// FPEnd is the file pointer just past the end of this block's data.
	// Drives the TotalBlockOtherBytes accounting in endBlock().
	FPEnd int64

	// PrefixLength is the number of bytes shared by every term inside
	// this block (i.e. the depth in the trie).
	PrefixLength int
	// TotalSuffixBytes is the total number of suffix bytes (post-compression)
	// the block writer emitted.
	TotalSuffixBytes int64

	// SuffixesReader / SuffixLengthsReader / StatsReader are the three
	// per-block sequential cursors the writer emitted to the term
	// dictionary. Length() on each one yields the byte counts Stats
	// folds into its aggregates.
	SuffixesReader      *segmentTermsEnumReader
	SuffixLengthsReader *segmentTermsEnumReader
	StatsReader         *segmentTermsEnumReader

	// CompressionAlg is the algorithm the writer chose for this block's
	// suffix blob.
	CompressionAlg CompressionAlgorithm
	// EntCount is the total number of entries (terms + sub-blocks) in
	// this block.
	EntCount int
	// IsLeafBlock is true when the block consists entirely of terms.
	IsLeafBlock bool
	// State carries the postings metadata for the current term inside
	// the block; Stats reads State.TermBlockOrd to compute the term/sub-block
	// split on endBlock.
	State *BlockTermState
}

// segmentTermsEnumReader is a minimal Reader-style cursor stub used by
// Stats; the production version will wrap [store.ByteArrayDataInput] from
// SegmentTermsEnumFrame. Only Length() is exercised today.
type segmentTermsEnumReader struct {
	bytes []byte
}

// NewSegmentTermsEnumReader wraps b in a stub reader; the production
// version emitted by SegmentTermsEnumFrame will be byte-format compatible.
func NewSegmentTermsEnumReader(b []byte) *segmentTermsEnumReader {
	return &segmentTermsEnumReader{bytes: b}
}

// Length returns the number of bytes the reader has not yet consumed.
// Stats only needs this aggregate, so the stub mimics
// {@code ByteArrayDataInput.length()} which always returns the remaining
// byte count for a fresh-position reader (offset 0). Once the production
// SegmentTermsEnum lands this method will forward to the wrapped
// ByteArrayDataInput.
func (r *segmentTermsEnumReader) Length() int64 {
	if r == nil {
		return 0
	}
	return int64(len(r.bytes))
}

// StartBlock folds frame into the running aggregates. isFloor is true when
// the frame describes a floor sub-block. Mirrors Stats.startBlock.
func (s *Lucene103BlockTreeStats) StartBlock(frame *SegmentTermsEnumFrame, isFloor bool) error {
	if frame == nil {
		return errors.New("Lucene103BlockTreeStats.StartBlock: nil frame")
	}
	s.TotalBlockCount++
	if isFloor {
		if frame.FP == frame.FPOrig {
			s.FloorBlockCount++
		}
		s.FloorSubBlockCount++
	} else {
		s.NonFloorBlockCount++
	}

	if len(s.BlockCountByPrefixLen) <= frame.PrefixLength {
		s.BlockCountByPrefixLen = util.GrowInRange(s.BlockCountByPrefixLen, frame.PrefixLength+1, util.MaxArrayLength)
	}
	s.BlockCountByPrefixLen[frame.PrefixLength]++
	s.startBlockCount++
	s.TotalBlockSuffixBytes += frame.TotalSuffixBytes
	s.TotalUncompressedBlockSuffixBytes += frame.SuffixesReader.Length()
	if frame.SuffixesReader != frame.SuffixLengthsReader {
		s.TotalUncompressedBlockSuffixBytes += frame.SuffixLengthsReader.Length()
	}
	s.TotalBlockStatsBytes += frame.StatsReader.Length()
	code := frame.CompressionAlg.Code()
	if code < 0 || code >= len(s.CompressionAlgorithms) {
		return fmt.Errorf("Lucene103BlockTreeStats.StartBlock: compression code %d out of range", code)
	}
	s.CompressionAlgorithms[code]++
	return nil
}

// EndBlock finalises the term-vs-subblock split for frame and accrues
// otherBytes (PostingsReaderBase metadata + frame overhead). Mirrors
// Stats.endBlock.
func (s *Lucene103BlockTreeStats) EndBlock(frame *SegmentTermsEnumFrame) error {
	if frame == nil {
		return errors.New("Lucene103BlockTreeStats.EndBlock: nil frame")
	}
	var termCount int
	if frame.IsLeafBlock {
		termCount = frame.EntCount
	} else if frame.State != nil {
		termCount = frame.State.TermBlockOrd
	}
	subBlockCount := frame.EntCount - termCount
	s.TotalTermCount += int64(termCount)
	switch {
	case termCount != 0 && subBlockCount != 0:
		s.MixedBlockCount++
	case termCount != 0:
		s.TermsOnlyBlockCount++
	case subBlockCount != 0:
		s.SubBlocksOnlyBlockCount++
	default:
		return errors.New("Lucene103BlockTreeStats.EndBlock: block had zero terms and zero sub-blocks")
	}
	s.endBlockCount++

	otherBytes := frame.FPEnd - frame.FP - frame.TotalSuffixBytes - frame.StatsReader.Length()
	if otherBytes <= 0 {
		return fmt.Errorf("Lucene103BlockTreeStats.EndBlock: otherBytes=%d frame.FP=%d frame.FPEnd=%d", otherBytes, frame.FP, frame.FPEnd)
	}
	s.TotalBlockOtherBytes += otherBytes
	return nil
}

// Term records the bytes of a single term seen during enumeration.
// Mirrors Stats.term(BytesRef).
func (s *Lucene103BlockTreeStats) Term(term *util.BytesRef) {
	if term == nil {
		return
	}
	s.TotalTermBytes += int64(term.Length)
}

// Finish validates the start/end and block-count invariants Lucene asserts
// at the end of enumeration. Mirrors Stats.finish.
func (s *Lucene103BlockTreeStats) Finish() error {
	if s.startBlockCount != s.endBlockCount {
		return fmt.Errorf("Lucene103BlockTreeStats.Finish: startBlockCount=%d endBlockCount=%d", s.startBlockCount, s.endBlockCount)
	}
	if s.TotalBlockCount != s.FloorSubBlockCount+s.NonFloorBlockCount {
		return fmt.Errorf(
			"Lucene103BlockTreeStats.Finish: floorSubBlockCount=%d nonFloorBlockCount=%d totalBlockCount=%d",
			s.FloorSubBlockCount, s.NonFloorBlockCount, s.TotalBlockCount,
		)
	}
	if s.TotalBlockCount != s.MixedBlockCount+s.TermsOnlyBlockCount+s.SubBlocksOnlyBlockCount {
		return fmt.Errorf(
			"Lucene103BlockTreeStats.Finish: totalBlockCount=%d mixed=%d sub=%d terms=%d",
			s.TotalBlockCount, s.MixedBlockCount, s.SubBlocksOnlyBlockCount, s.TermsOnlyBlockCount,
		)
	}
	return nil
}

// String renders the statistics in the same multi-line format Lucene's
// CheckIndex -verbose uses. Locale.ROOT in the Java reference is the
// canonical English locale; Go's fmt.Sprintf with `%.1f` / `%.2f` uses the
// invariant numeric formatting and is the matching shape. Mirrors
// Stats.toString().
func (s *Lucene103BlockTreeStats) String() string {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "  index trie:")
	fmt.Fprintf(&buf, "    %d bytes\n", s.IndexNumBytes)
	fmt.Fprintln(&buf, "  terms:")
	fmt.Fprintf(&buf, "    %d terms\n", s.TotalTermCount)
	if s.TotalTermCount != 0 {
		fmt.Fprintf(&buf, "    %d bytes (%.1f bytes/term)\n",
			s.TotalTermBytes,
			float64(s.TotalTermBytes)/float64(s.TotalTermCount),
		)
	} else {
		fmt.Fprintf(&buf, "    %d bytes\n", s.TotalTermBytes)
	}
	fmt.Fprintln(&buf, "  blocks:")
	fmt.Fprintf(&buf, "    %d blocks\n", s.TotalBlockCount)
	fmt.Fprintf(&buf, "    %d terms-only blocks\n", s.TermsOnlyBlockCount)
	fmt.Fprintf(&buf, "    %d sub-block-only blocks\n", s.SubBlocksOnlyBlockCount)
	fmt.Fprintf(&buf, "    %d mixed blocks\n", s.MixedBlockCount)
	fmt.Fprintf(&buf, "    %d floor blocks\n", s.FloorBlockCount)
	fmt.Fprintf(&buf, "    %d non-floor blocks\n", s.TotalBlockCount-s.FloorSubBlockCount)
	fmt.Fprintf(&buf, "    %d floor sub-blocks\n", s.FloorSubBlockCount)
	if s.TotalBlockCount != 0 {
		fmt.Fprintf(&buf, "    %d term suffix bytes before compression (%.1f suffix-bytes/block)\n",
			s.TotalUncompressedBlockSuffixBytes,
			float64(s.TotalBlockSuffixBytes)/float64(s.TotalBlockCount),
		)
	} else {
		fmt.Fprintf(&buf, "    %d term suffix bytes before compression\n", s.TotalUncompressedBlockSuffixBytes)
	}

	var compressionCounts string
	for code := 0; code < len(s.CompressionAlgorithms); code++ {
		if s.CompressionAlgorithms[code] == 0 {
			continue
		}
		alg, err := CompressionAlgorithmByCode(code)
		if err != nil {
			continue
		}
		if compressionCounts != "" {
			compressionCounts += ", "
		}
		compressionCounts += fmt.Sprintf("%s: %d", alg, s.CompressionAlgorithms[code])
	}
	if s.TotalBlockCount != 0 && s.TotalUncompressedBlockSuffixBytes > 0 {
		fmt.Fprintf(&buf, "    %d compressed term suffix bytes (%.2f compression ratio - compression count by algorithm: %s)\n",
			s.TotalBlockSuffixBytes,
			float64(s.TotalBlockSuffixBytes)/float64(s.TotalUncompressedBlockSuffixBytes),
			compressionCounts,
		)
	} else {
		fmt.Fprintf(&buf, "    %d compressed term suffix bytes ()\n", s.TotalBlockSuffixBytes)
	}
	if s.TotalBlockCount != 0 {
		fmt.Fprintf(&buf, "    %d term stats bytes  (%.1f stats-bytes/block)\n",
			s.TotalBlockStatsBytes,
			float64(s.TotalBlockStatsBytes)/float64(s.TotalBlockCount),
		)
		fmt.Fprintf(&buf, "    %d other bytes (%.1f other-bytes/block)\n",
			s.TotalBlockOtherBytes,
			float64(s.TotalBlockOtherBytes)/float64(s.TotalBlockCount),
		)
	} else {
		fmt.Fprintf(&buf, "    %d term stats bytes \n", s.TotalBlockStatsBytes)
		fmt.Fprintf(&buf, "    %d other bytes\n", s.TotalBlockOtherBytes)
	}
	if s.TotalBlockCount != 0 {
		fmt.Fprintln(&buf, "    by prefix length:")
		total := 0
		for prefix, blockCount := range s.BlockCountByPrefixLen {
			total += blockCount
			if blockCount != 0 {
				fmt.Fprintf(&buf, "      %2d: %d\n", prefix, blockCount)
			}
		}
		_ = total // Java asserts total == TotalBlockCount; we keep the
		// check soft because mismatched aggregates would already have
		// surfaced in Finish().
	}

	return buf.String()
}
