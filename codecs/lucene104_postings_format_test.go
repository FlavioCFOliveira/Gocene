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
	if len(reader.Leaves()) != 1 {
		t.Errorf("Expected 1 leaf, got %d", len(reader.Leaves()))
	}

	// Get the terms for the field
	// Note: This requires block tree implementation to expose stats
	// For now, we verify the basic structure
	leaf := reader.Leaves()[0]
	terms, err := leaf.Reader().Terms("field")
	if err != nil {
		t.Fatalf("Failed to get terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms should not be nil")
	}

	// Verify we can iterate all terms
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("Failed to get terms iterator: %v", err)
	}

	termCount := 0
	for {
		term, err := te.Next()
		if err != nil {
			t.Fatalf("Error iterating terms: %v", err)
		}
		if term == nil {
			break
		}
		termCount++
	}

	// We should have 50 terms (25 single chars + 25 z-prefixed)
	if termCount != 50 {
		t.Errorf("Expected 50 terms, got %d", termCount)
	}

	// TODO: Once block tree stats are implemented, verify:
	// - stats.floorBlockCount == 0
	// - stats.nonFloorBlockCount == 2 (root block + z* block)
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
	// Create accumulator and add impacts
	acc := codecs.NewCompetitiveImpactAccumulator()
	for _, impact := range impacts {
		acc.Add(impact.Freq, impact.Norm)
	}

	// Create a directory for temporary storage
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Write impacts to file
	out, err := dir.CreateOutput("impacts.tmp", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	err = codecs.WriteImpacts(acc.GetCompetitiveFreqNormPairs(), out)
	if err != nil {
		t.Fatalf("Failed to write impacts: %v", err)
	}

	err = out.Close()
	if err != nil {
		t.Fatalf("Failed to close output: %v", err)
	}

	// Read back the impacts
	in, err := dir.OpenInput("impacts.tmp", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	length := in.Length()
	data := make([]byte, length)
	err = in.ReadBytes(data)
	if err != nil {
		t.Fatalf("Failed to read impacts: %v", err)
	}

	// Deserialize impacts
	dataIn := store.NewByteArrayDataInput(data)
	buffer := index.NewFreqAndNormBuffer()
	buffer.GrowNoCopy(len(impacts) + 1) // Add some extra capacity like Java test

	result, err := codecs.ReadImpacts(dataIn, buffer)
	if err != nil {
		t.Fatalf("Failed to read impacts: %v", err)
	}

	// Verify the results
	if result.Size() != len(impacts) {
		t.Errorf("Expected %d impacts, got %d", len(impacts), result.Size())
	}

	for i := 0; i < result.Size() && i < len(impacts); i++ {
		if result.Freqs[i] != impacts[i].Freq {
			t.Errorf("Impact %d: expected freq %d, got %d", i, impacts[i].Freq, result.Freqs[i])
		}
		if result.Norms[i] != impacts[i].Norm {
			t.Errorf("Impact %d: expected norm %d, got %d", i, impacts[i].Norm, result.Norms[i])
		}
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

// Placeholder functions for VInt15/VLong15 encoding/decoding.
// These should be implemented in the store package or codecs package.
// The VInt15 encoding uses 15 bits per byte with a continuation bit.

func writeVInt15(out *store.ByteArrayDataOutput, value int32) error {
	// VInt15 encoding: 15 bits of data per byte, high bit is continuation
	// This is more efficient than VInt for small values
	for (value & ^int32(0x7FFF)) != 0 {
		if err := out.WriteByte(byte((value & 0x7FFF) >> 7)); err != nil {
			return err
		}
		value <<= 8
	}
	return out.WriteByte(byte(value & 0x7F))
}

func readVInt15(in *store.ByteArrayDataInput) (int32, error) {
	// VInt15 decoding
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	var result int32 = int32(b & 0x7F)
	shift := 7
	for (b&0x80) != 0 && shift < 28 {
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		shift += 7
	}
	return result, nil
}

func writeVLong15(out *store.ByteArrayDataOutput, value int64) error {
	// VLong15 encoding: similar to VInt15 but for 64-bit values
	for (value & ^int64(0x7FFF)) != 0 {
		if err := out.WriteByte(byte((value & 0x7FFF) >> 7)); err != nil {
			return err
		}
		value <<= 8
	}
	return out.WriteByte(byte(value & 0x7F))
}

func readVLong15(in *store.ByteArrayDataInput) (int64, error) {
	// VLong15 decoding
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	var result int64 = int64(b & 0x7F)
	shift := 7
	for (b&0x80) != 0 && shift < 63 {
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		shift += 7
	}
	return result, nil
}
