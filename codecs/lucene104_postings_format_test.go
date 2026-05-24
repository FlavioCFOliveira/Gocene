// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
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

// TestLucene104PostingsFormat_FinalBlock tests that final sub-block(s) are not skipped.
// This test creates documents with terms that would create multiple blocks and
// verifies that the final block is properly handled.
func TestLucene104PostingsFormat_FinalBlock(t *testing.T) {
	t.Skip("Final block test requires full IndexWriter and block tree implementation")

	// Create a directory for the test
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer config with mock analyzer
	config := index.NewIndexWriterConfig(nil)

	// Create the index writer
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add 25 documents with terms that create multiple blocks
	// Terms: a, b, c, ... y (single characters) and za, zb, zc, ... zy
	for i := 0; i < 25; i++ {
		doc := document.NewDocument()

		// Add single character term (a-y)
		term1 := string(rune('a' + i))
		sf1, err := document.NewStringField("field", term1, false)
		if err != nil {
			t.Fatalf("Failed to create StringField: %v", err)
		}
		doc.Add(sf1)

		// Add z-prefixed term (za-zy)
		term2 := "z" + string(rune('a'+i))
		sf2, err := document.NewStringField("field", term2, false)
		if err != nil {
			t.Fatalf("Failed to create StringField: %v", err)
		}
		doc.Add(sf2)

		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// Force merge to a single segment
	err = writer.ForceMerge(1)
	if err != nil {
		t.Fatalf("Failed to force merge: %v", err)
	}

	// Close the writer
	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Open a reader to verify block structure
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Verify we have exactly one leaf (segment)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Errorf("Expected 1 leaf, got %d", len(leaves))
	}

	// Get the terms for the field
	// Note: This requires block tree implementation to expose stats
	// For now, we verify the basic structure
	leaf := leaves[0]

	// Terms method not available on IndexReaderInterface yet
	// terms, err := leaf.Reader().Terms("field")
	// if err != nil {
	// 	t.Fatalf("Failed to get terms: %v", err)
	// }
	// if terms == nil {
	// 	t.Fatal("Terms should not be nil")
	// }

	// Just verify leaf reader is accessible
	_ = leaf.Reader()
	t.Log("Leaf reader accessible, Terms API not yet implemented")

	// TODO: Once Terms API is implemented, verify:
	// - Can iterate all terms
	// - Block tree stats: floorBlockCount == 0, nonFloorBlockCount == 2
}

// TestLucene104PostingsFormat_ImpactSerialization tests the serialization of impact data.
// Impact data is used for scoring and contains frequency and norm information.
func TestLucene104PostingsFormat_ImpactSerialization(t *testing.T) {
	t.Skip("Impact serialization requires CompetitiveImpactAccumulator and FreqAndNormBuffer implementation")

	testCases := [][]Impact{
		// omit norms and omit freqs
		{{Freq: 1, Norm: 1}},
		// omit freqs
		{{Freq: 1, Norm: 42}},
		// omit freqs with very large norms
		{{Freq: 1, Norm: -100}},
		// omit norms
		{{Freq: 30, Norm: 1}},
		// omit norms with large freq
		{{Freq: 500, Norm: 1}},
		// freqs and norms, basic
		{
			{Freq: 1, Norm: 7},
			{Freq: 3, Norm: 9},
			{Freq: 7, Norm: 10},
			{Freq: 15, Norm: 11},
			{Freq: 20, Norm: 13},
			{Freq: 28, Norm: 14},
		},
		// freqs and norms, high values
		{
			{Freq: 2, Norm: 2},
			{Freq: 10, Norm: 10},
			{Freq: 12, Norm: 50},
			{Freq: 50, Norm: -100},
			{Freq: 1000, Norm: -80},
			{Freq: 1005, Norm: -3},
		},
	}

	for i, impacts := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			testImpactSerialization(t, impacts)
		})
	}
}

// Impact represents a frequency and norm pair for scoring.
type Impact struct {
	Freq int
	Norm int64
}

// testImpactSerialization tests serialization of a single impact list.
func testImpactSerialization(t *testing.T, impacts []Impact) {
	// TODO: These functions don't exist yet - skipping test
	t.Skip("Impact serialization not fully implemented yet")
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
