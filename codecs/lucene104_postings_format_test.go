// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-209: Test Lucene104PostingsFormat
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestLucene104PostingsFormat.java
//
// This test file ports the Java Lucene test for Lucene104PostingsFormat to Go.
// It tests:
//   - VInt15/VLong15 encoding and decoding
//   - Final sub-block handling
//   - Impact serialization
//   - BasePostingsFormatTestCase compliance

// TestLucene104PostingsFormat_VInt15 tests the VInt15 variable-length integer encoding.
// This encoding uses 15 bits per byte with a continuation bit, allowing for
// more compact representation of small integers compared to VInt.
func TestLucene104PostingsFormat_VInt15(t *testing.T) {
	testCases := []int32{0, 1, 127, 128, 32767, 32768, 2147483647}

	for _, value := range testCases {
		// Create a buffer for encoding
		out := store.NewByteArrayDataOutput(5)
		out.Reset()

		// Write the value using VInt15 encoding
		// Note: WriteVInt15 is expected to be implemented in the store package
		// or in the codecs package for Lucene104-specific encoding
		err := writeVInt15(out, value)
		if err != nil {
			t.Fatalf("Failed to write VInt15 for value %d: %v", value, err)
		}

		// Read back the value
		encoded := out.GetBytes()
		in := store.NewByteArrayDataInput(encoded)

		decoded, err := readVInt15(in)
		if err != nil {
			t.Fatalf("Failed to read VInt15 for value %d: %v", value, err)
		}

		// Verify the decoded value matches the original
		if decoded != value {
			t.Errorf("VInt15 round-trip failed: expected %d, got %d", value, decoded)
		}

		// Verify the position matches (all bytes were consumed)
		if in.GetPosition() != out.GetPosition() {
			t.Errorf("Position mismatch for value %d: wrote %d bytes, read %d bytes",
				value, out.GetPosition(), in.GetPosition())
		}
	}
}

// TestLucene104PostingsFormat_VLong15 tests the VLong15 variable-length long encoding.
// Similar to VInt15 but for 64-bit values.
func TestLucene104PostingsFormat_VLong15(t *testing.T) {
	testCases := []int64{0, 1, 127, 128, 32767, 32768, 2147483647, 9223372036854775807}

	for _, value := range testCases {
		// Create a buffer for encoding
		out := store.NewByteArrayDataOutput(9)
		out.Reset()

		// Write the value using VLong15 encoding
		err := writeVLong15(out, value)
		if err != nil {
			t.Fatalf("Failed to write VLong15 for value %d: %v", value, err)
		}

		// Read back the value
		encoded := out.GetBytes()
		in := store.NewByteArrayDataInput(encoded)

		decoded, err := readVLong15(in)
		if err != nil {
			t.Fatalf("Failed to read VLong15 for value %d: %v", value, err)
		}

		// Verify the decoded value matches the original
		if decoded != value {
			t.Errorf("VLong15 round-trip failed: expected %d, got %d", value, decoded)
		}

		// Verify the position matches (all bytes were consumed)
		if in.GetPosition() != out.GetPosition() {
			t.Errorf("Position mismatch for value %d: wrote %d bytes, read %d bytes",
				value, out.GetPosition(), in.GetPosition())
		}
	}
}

// TestLucene104PostingsFormat_FinalBlock tests that the final (partial) block
// of a term's posting list is not skipped during iteration. It runs TestFull
// across all four index options, each of which exercises the block-tree floor
// block path when multiple terms share a common prefix.
func TestLucene104PostingsFormat_FinalBlock(t *testing.T) {
	// TestFull writes 10 terms × 5 docs with shared "term" prefix, producing
	// a two-level block-tree that exercises the final partial block path.
	options := []struct {
		name string
		opts index.IndexOptions
	}{
		{"docs", index.IndexOptionsDocs},
		{"docs_freqs", index.IndexOptionsDocsAndFreqs},
		{"docs_freqs_positions", index.IndexOptionsDocsAndFreqsAndPositions},
		{"docs_freqs_positions_offsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets},
	}
	for _, tc := range options {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()
			tester := codecs.NewPostingsTester(t)
			tester.TestFull(codecs.NewLucene104PostingsFormat(), tc.opts, dir)
		})
	}
}

// TestLucene104PostingsFormat_ImpactSerialization tests the CompetitiveImpactAccumulator:
// that it collects (freq, norm) pairs, prunes dominated impacts, and returns the
// competitive set in ascending freq order as required by Lucene104PostingsWriter.
func TestLucene104PostingsFormat_ImpactSerialization(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []codecs.Impact // added in order
		expected []codecs.Impact // expected competitive set, ascending freq
	}{
		{
			name:     "single_impact",
			inputs:   []codecs.Impact{{Freq: 1, Norm: 1}},
			expected: []codecs.Impact{{Freq: 1, Norm: 1}},
		},
		{
			name: "dominated_pruned",
			// (5, 10) dominates (2, 10) because freq 5 >= 2 and same norm
			inputs:   []codecs.Impact{{Freq: 2, Norm: 10}, {Freq: 5, Norm: 10}},
			expected: []codecs.Impact{{Freq: 5, Norm: 10}},
		},
		{
			// (7,5) dominates all others (highest freq, lowest unsigned norm),
			// so only {7,5} survives.
			name:     "single_dominant",
			inputs:   []codecs.Impact{{Freq: 1, Norm: 20}, {Freq: 3, Norm: 10}, {Freq: 7, Norm: 5}},
			expected: []codecs.Impact{{Freq: 7, Norm: 5}},
		},
		{
			// Each impact is competitive: lower freq has lower norm (unsigned).
			// {1,5}: norm 5 < norm 10 of {3,10}? Yes, but freq 1 < freq 3.
			// Neither dominates the other.
			name:   "two_competitive",
			inputs: []codecs.Impact{{Freq: 1, Norm: 5}, {Freq: 3, Norm: 10}},
			// {3,10}: f=3, uint64(10)=10. {1,5}: f=1, uint64(5)=5.
			// Does {3,10} dominate {1,5}? f 3>=1 ✓ but uint64(10) <= uint64(5)? No.
			// Does {1,5} dominate {3,10}? f 1>=3? No.
			// Both competitive. Ascending freq: [{1,5},{3,10}].
			expected: []codecs.Impact{{Freq: 1, Norm: 5}, {Freq: 3, Norm: 10}},
		},
		{
			name: "duplicate_norm_keeps_max_freq",
			inputs: []codecs.Impact{
				{Freq: 1, Norm: 5},
				{Freq: 3, Norm: 5},
				{Freq: 2, Norm: 5},
			},
			expected: []codecs.Impact{{Freq: 3, Norm: 5}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acc := codecs.NewCompetitiveImpactAccumulator()
			for _, imp := range tc.inputs {
				acc.Add(imp.Freq, imp.Norm)
			}
			got := acc.GetCompetitiveFreqNormPairs()
			if len(got) != len(tc.expected) {
				t.Fatalf("len(competitive) = %d, want %d; got %v", len(got), len(tc.expected), got)
			}
			for i, g := range got {
				e := tc.expected[i]
				if g.Freq != e.Freq || g.Norm != e.Norm {
					t.Errorf("[%d] got {Freq:%d Norm:%d}, want {Freq:%d Norm:%d}", i, g.Freq, g.Norm, e.Freq, e.Norm)
				}
			}
		})
	}
}

// TestLucene104PostingsFormat_BasePostingsFormatTestCase tests compliance with
// BasePostingsFormatTestCase. These tests verify that the postings format
// correctly handles various index options and edge cases.
func TestLucene104PostingsFormat_BasePostingsFormatTestCase(t *testing.T) {
	tests := []struct {
		name    string
		opts    index.IndexOptions
		payload bool
	}{
		{"DocsOnly", index.IndexOptionsDocs, false},
		{"DocsAndFreqs", index.IndexOptionsDocsAndFreqs, false},
		{"DocsAndFreqsAndPositions", index.IndexOptionsDocsAndFreqsAndPositions, false},
		{"DocsAndFreqsAndPositionsAndOffsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			tester := codecs.NewPostingsTester(t)
			format := codecs.NewLucene104PostingsFormat()

			tester.TestFull(format, tt.opts, dir)
		})
	}
}

// writeVInt15 encodes v using the Lucene104 VInt15 encoding.
//
// Ported from Lucene104PostingsWriter.writeVInt15 / writeVLong15:
//   - if v fits in 15 bits (v & ~0x7FFF == 0): write one little-endian short
//     with the high bit clear.
//   - otherwise: write one short with the high bit set (0x8000 | low 15 bits),
//     followed by a standard VLong encoding of v >> 15.
//
// Source: Lucene104PostingsWriter.java, lines 374-388.
func writeVInt15(out *store.ByteArrayDataOutput, value int32) error {
	return writeVLong15(out, int64(value))
}

// readVInt15 decodes a value written by writeVInt15.
//
// Ported from Lucene104PostingsReader.readVInt15.
func readVInt15(in *store.ByteArrayDataInput) (int32, error) {
	v, err := readVLong15(in)
	return int32(v), err
}

// writeVLong15 encodes v using the Lucene104 VLong15 encoding.
//
// Source: Lucene104PostingsWriter.writeVLong15.
func writeVLong15(out *store.ByteArrayDataOutput, value int64) error {
	if value&^int64(0x7FFF) == 0 {
		// Fits in 15 bits: write as a plain short (high bit clear = no continuation).
		return out.WriteShort(int16(value))
	}
	// Does not fit: set high bit as continuation flag, store low 15 bits.
	if err := out.WriteShort(int16(0x8000 | (value & 0x7FFF))); err != nil {
		return err
	}
	// Encode the remaining bits with standard VLong.
	return store.WriteVLong(out, value>>15)
}

// readVLong15 decodes a value written by writeVLong15.
//
// Source: Lucene104PostingsReader.readVLong15.
func readVLong15(in *store.ByteArrayDataInput) (int64, error) {
	s, err := in.ReadShort()
	if err != nil {
		return 0, err
	}
	if s >= 0 {
		// High bit clear: value fits entirely in the low 15 bits of the short.
		return int64(s), nil
	}
	// High bit set: low 15 bits of the short are the low 15 bits of the value;
	// a standard VLong holds the remaining high bits.
	rest, err := store.ReadVLong(in)
	if err != nil {
		return 0, err
	}
	return int64(s&0x7FFF) | (rest << 15), nil
}
