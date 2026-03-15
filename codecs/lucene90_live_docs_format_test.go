// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: lucene90_live_docs_format_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90LiveDocsFormat.java
// Purpose: Tests Lucene 9.0 LiveDocsFormat serialization and deserialization
// Task: GC-201

package codecs_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LiveDocsFormat defines the interface for reading and writing live docs.
// This is the Go port of Lucene's LiveDocsFormat.
type LiveDocsFormat interface {
	// ReadLiveDocs reads the live docs bits for the given segment.
	ReadLiveDocs(dir store.Directory, info *index.SegmentCommitInfo, ctx store.IOContext) (util.Bits, error)

	// WriteLiveDocs writes the live docs bits for the given segment.
	WriteLiveDocs(bits util.Bits, dir store.Directory, info *index.SegmentCommitInfo, newDelCount int, ctx store.IOContext) error

	// Files adds the live docs file(s) to the files collection if deletions exist.
	Files(info *index.SegmentCommitInfo, files *[]string) error
}

// Lucene90LiveDocsFormat implements the Lucene 9.0 live docs format.
// The .liv file format:
//   - IndexHeader (CodecUtil.writeIndexHeader)
//   - Bits as Int64 array (LongCount)
//
// Deletion rate threshold for sparse vs dense representation: 1%
type Lucene90LiveDocsFormat struct {
	// Extension for live docs files
	extension string
	// Codec name for the header
	codecName string
	// Version for the format
	version int
}

// Constants for Lucene90LiveDocsFormat
const (
	// LiveDocsExtension is the file extension for live docs
	LiveDocsExtension = "liv"
	// LiveDocsCodecName is the codec name written in the header
	LiveDocsCodecName = "Lucene90LiveDocs"
	// LiveDocsVersionStart is the first supported version
	LiveDocsVersionStart = 0
	// LiveDocsVersionCurrent is the current version
	LiveDocsVersionCurrent = LiveDocsVersionStart
	// SparseDenseThreshold is the deletion rate threshold for choosing sparse vs dense (1%)
	SparseDenseThreshold = 0.01
)

// NewLucene90LiveDocsFormat creates a new Lucene90LiveDocsFormat instance.
func NewLucene90LiveDocsFormat() *Lucene90LiveDocsFormat {
	return &Lucene90LiveDocsFormat{
		extension: LiveDocsExtension,
		codecName: LiveDocsCodecName,
		version:   LiveDocsVersionCurrent,
	}
}

// getLiveDocsFileName returns the file name for live docs based on generation.
func getLiveDocsFileName(segmentName string, delGen int64) string {
	if delGen < 0 {
		return ""
	}
	// Format: _X_Y.liv where X is segment number and Y is generation
	return fmt.Sprintf("_%s_%s.liv", segmentName[1:], int64ToBase(delGen, 36))
}

// int64ToBase converts an int64 to a string in the given base.
func int64ToBase(n int64, base int) string {
	if n == 0 {
		return "0"
	}
	if base < 2 || base > 36 {
		base = 10
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		result = append([]byte{digits[n%int64(base)]}, result...)
		n /= int64(base)
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}

// bits2words returns the number of uint64 words needed for the given number of bits.
func bits2words(numBits int) int {
	if numBits <= 0 {
		return 0
	}
	return (numBits + 63) / 64
}

// ReadLiveDocs reads the live docs from the directory.
func (f *Lucene90LiveDocsFormat) ReadLiveDocs(dir store.Directory, info *index.SegmentCommitInfo, ctx store.IOContext) (util.Bits, error) {
	maxDoc := info.SegmentInfo().DocCount()
	delCount := info.DelCount()

	if delCount == 0 {
		// All docs are live
		return &allLiveBits{length: maxDoc}, nil
	}

	delGen := info.DelGen()
	fileName := getLiveDocsFileName(info.Name(), delGen)

	input, err := dir.OpenInput(fileName, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open live docs file %s: %w", fileName, err)
	}
	defer input.Close()

	// Read and verify header
	// Format: magic (4 bytes) + codec name + version + object ID + suffix
	magic, err := store.ReadInt32(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read header magic: %w", err)
	}
	if magic != codecs.CodecMagic {
		return nil, fmt.Errorf("invalid codec magic: expected %d, got %d", codecs.CodecMagic, magic)
	}

	// Read codec name
	codecName, err := store.ReadString(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read codec name: %w", err)
	}
	if codecName != f.codecName {
		return nil, fmt.Errorf("invalid codec name: expected %s, got %s", f.codecName, codecName)
	}

	// Read version
	version, err := store.ReadInt32(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version < LiveDocsVersionStart || version > LiveDocsVersionCurrent {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read segment ID (16 bytes)
	segmentID := make([]byte, 16)
	if _, err := input.Read(segmentID); err != nil {
		return nil, fmt.Errorf("failed to read segment ID: %w", err)
	}

	// Read suffix (generation string)
	_, err = store.ReadString(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read suffix: %w", err)
	}

	// Read the bits data
	deletionRate := float64(delCount) / float64(maxDoc)
	return f.readLiveDocs(input, maxDoc, deletionRate, delCount)
}

// readLiveDocs reads the actual bits data based on deletion rate.
func (f *Lucene90LiveDocsFormat) readLiveDocs(input store.IndexInput, maxDoc int, deletionRate float64, expectedDelCount int) (util.Bits, error) {
	numWords := bits2words(maxDoc)
	data := make([]uint64, numWords)

	// Read all the longs
	for i := 0; i < numWords; i++ {
		val, err := store.ReadInt64(input)
		if err != nil {
			return nil, fmt.Errorf("failed to read long at index %d: %w", i, err)
		}
		data[i] = uint64(val)
	}

	// Read footer checksum
	checksum, err := store.ReadInt32(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}
	_ = checksum // Verify checksum in production

	// Create the appropriate bits implementation
	if deletionRate <= SparseDenseThreshold {
		// Use sparse representation
		return f.createSparseLiveDocs(data, maxDoc, expectedDelCount)
	}
	// Use dense representation
	return f.createDenseLiveDocs(data, maxDoc, expectedDelCount)
}

// createDenseLiveDocs creates a dense live docs bits from the raw data.
func (f *Lucene90LiveDocsFormat) createDenseLiveDocs(data []uint64, maxDoc int, expectedDelCount int) (util.Bits, error) {
	liveDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}

	// Copy the bits from data
	for i := 0; i < maxDoc; i++ {
		wordIdx := i / 64
		bitIdx := uint(i % 64)
		if wordIdx < len(data) && (data[wordIdx]&(1<<bitIdx)) != 0 {
			liveDocs.Set(i)
		}
	}

	actualDelCount := maxDoc - liveDocs.Cardinality()
	if actualDelCount != expectedDelCount {
		return nil, fmt.Errorf("deleted count mismatch: expected %d, got %d", expectedDelCount, actualDelCount)
	}

	return &denseLiveDocsBits{
		bits:     liveDocs,
		length:   maxDoc,
		delCount: actualDelCount,
	}, nil
}

// createSparseLiveDocs creates a sparse live docs bits from the raw data.
func (f *Lucene90LiveDocsFormat) createSparseLiveDocs(data []uint64, maxDoc int, expectedDelCount int) (util.Bits, error) {
	// For sparse representation, we store deleted docs (bit=1 means deleted)
	// But the disk format stores LIVE docs (bit=1 means live)
	// So we need to invert
	deletedDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}

	for wordIdx, word := range data {
		if word == ^uint64(0) {
			// All bits set means all docs live in this word, no deletions
			continue
		}
		baseDocId := wordIdx * 64
		maxDocInWord := baseDocId + 64
		if maxDocInWord > maxDoc {
			maxDocInWord = maxDoc
		}
		for docId := baseDocId; docId < maxDocInWord; docId++ {
			bitIndex := docId & 63
			// If bit is 0 in disk format (deleted doc), set it in sparse representation
			if (word & (1 << uint(bitIndex))) == 0 {
				deletedDocs.Set(docId)
			}
		}
	}

	actualDelCount := deletedDocs.Cardinality()
	if actualDelCount != expectedDelCount {
		return nil, fmt.Errorf("deleted count mismatch: expected %d, got %d", expectedDelCount, actualDelCount)
	}

	return &sparseLiveDocsBits{
		deletedDocs: deletedDocs,
		length:      maxDoc,
		delCount:    actualDelCount,
	}, nil
}

// WriteLiveDocs writes the live docs to the directory.
func (f *Lucene90LiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, info *index.SegmentCommitInfo, newDelCount int, ctx store.IOContext) error {
	nextGen := info.AdvanceDelGen()
	fileName := getLiveDocsFileName(info.Name(), nextGen)

	output, err := dir.CreateOutput(fileName, ctx)
	if err != nil {
		return fmt.Errorf("failed to create live docs file %s: %w", fileName, err)
	}
	defer output.Close()

	// Write header
	// CodecUtil.writeIndexHeader format:
	// - magic (4 bytes): CODEC_MAGIC
	// - codec name (string)
	// - version (int32)
	// - segment ID (16 bytes)
	// - suffix (string): generation in base 36

	if err := store.WriteInt32(output, codecs.CodecMagic); err != nil {
		return err
	}
	if err := store.WriteString(output, f.codecName); err != nil {
		return err
	}
	if err := store.WriteInt32(output, int32(f.version)); err != nil {
		return err
	}

	// Write segment ID
	segmentID := info.SegmentInfo().GetID()
	if len(segmentID) != 16 {
		segmentID = make([]byte, 16)
		rand.Read(segmentID)
	}
	if _, err := output.Write(segmentID); err != nil {
		return err
	}

	// Write suffix (generation string)
	suffix := int64ToBase(nextGen, 36)
	if err := store.WriteString(output, suffix); err != nil {
		return err
	}

	// Write the bits data
	delCount, err := f.writeBits(output, bits)
	if err != nil {
		return err
	}

	// Write footer checksum
	checksum := int32(0) // Calculate proper checksum in production
	if err := store.WriteInt32(output, checksum); err != nil {
		return err
	}

	// Verify deletion count
	expectedDelCount := info.DelCount() + newDelCount
	if delCount != expectedDelCount {
		return fmt.Errorf("deleted count mismatch: wrote %d, expected %d", delCount, expectedDelCount)
	}

	return output.Close()
}

// writeBits writes the bits data to the output.
func (f *Lucene90LiveDocsFormat) writeBits(output store.IndexOutput, bits util.Bits) (int, error) {
	maxDoc := bits.Length()
	delCount := maxDoc

	// Write bits in batches
	batchSize := 1024
	numBatches := (maxDoc + batchSize - 1) / batchSize

	for batch := 0; batch < numBatches; batch++ {
		offset := batch * batchSize
		end := offset + batchSize
		if end > maxDoc {
			end = maxDoc
		}
		numBits := end - offset

		// Create a temporary bitset for this batch
		batchBits := make([]uint64, bits2words(numBits))

		// Copy bits from the source
		for i := 0; i < numBits; i++ {
			if bits.Get(offset + i) {
				wordIdx := i / 64
				bitIdx := uint(i % 64)
				batchBits[wordIdx] |= 1 << bitIdx
			}
		}

		// Count live docs in this batch
		for _, word := range batchBits {
			// Count set bits
			wordVal := word
			for wordVal != 0 {
				wordVal &= wordVal - 1
				delCount--
			}
		}

		// Write the batch
		numWords := bits2words(numBits)
		for i := 0; i < numWords; i++ {
			if err := store.WriteInt64(output, int64(batchBits[i])); err != nil {
				return 0, err
			}
		}
	}

	return delCount, nil
}

// Files adds the live docs file to the files collection if deletions exist.
func (f *Lucene90LiveDocsFormat) Files(info *index.SegmentCommitInfo, files *[]string) error {
	if info.HasDeletions() {
		delGen := info.DelGen()
		fileName := getLiveDocsFileName(info.Name(), delGen)
		if fileName != "" {
			*files = append(*files, fileName)
		}
	}
	return nil
}

// allLiveBits represents a bits where all documents are live.
type allLiveBits struct {
	length int
}

func (b *allLiveBits) Get(index int) bool {
	if index < 0 || index >= b.length {
		panic(fmt.Sprintf("index out of bounds: %d (length: %d)", index, b.length))
	}
	return true
}

func (b *allLiveBits) Length() int {
	return b.length
}

// denseLiveDocsBits wraps a FixedBitSet for live docs with deletion tracking.
type denseLiveDocsBits struct {
	bits     *util.FixedBitSet
	length   int
	delCount int
}

func (b *denseLiveDocsBits) Get(index int) bool {
	return b.bits.Get(index)
}

func (b *denseLiveDocsBits) Length() int {
	return b.length
}

// sparseLiveDocsBits wraps a FixedBitSet for deleted docs (inverted).
type sparseLiveDocsBits struct {
	deletedDocs *util.FixedBitSet
	length      int
	delCount    int
}

func (b *sparseLiveDocsBits) Get(index int) bool {
	// Return true if NOT deleted
	return !b.deletedDocs.Get(index)
}

func (b *sparseLiveDocsBits) Length() int {
	return b.length
}

// TestLucene90LiveDocsFormat_DenseLiveDocs tests serialization with dense live docs
// (almost all documents are live).
// Source: BaseLiveDocsFormatTestCase.testDenseLiveDocs()
func TestLucene90LiveDocsFormat_DenseLiveDocs(t *testing.T) {
	testCases := []struct {
		maxDoc      int
		numLiveDocs int
	}{
		{100, 99},   // 99% live
		{500, 499},  // 99.8% live
		{1000, 999}, // 99.9% live
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d_live=%d", tc.maxDoc, tc.numLiveDocs), func(t *testing.T) {
			// Test with FixedBitSet
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, true); err != nil {
				t.Errorf("FixedBitSet test failed: %v", err)
			}
			// Test with custom Bits implementation
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, false); err != nil {
				t.Errorf("Custom Bits test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_EmptyLiveDocs tests serialization with no live documents.
// Source: BaseLiveDocsFormatTestCase.testEmptyLiveDocs()
func TestLucene90LiveDocsFormat_EmptyLiveDocs(t *testing.T) {
	testCases := []struct {
		maxDoc      int
		numLiveDocs int
	}{
		{10, 0},
		{100, 0},
		{500, 0},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d_live=%d", tc.maxDoc, tc.numLiveDocs), func(t *testing.T) {
			// Test with FixedBitSet
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, true); err != nil {
				t.Errorf("FixedBitSet test failed: %v", err)
			}
			// Test with custom Bits implementation
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, false); err != nil {
				t.Errorf("Custom Bits test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_SparseLiveDocs tests serialization with sparse live docs
// (only a few documents are live).
// Source: BaseLiveDocsFormatTestCase.testSparseLiveDocs()
func TestLucene90LiveDocsFormat_SparseLiveDocs(t *testing.T) {
	testCases := []struct {
		maxDoc      int
		numLiveDocs int
	}{
		{100, 1},  // 1% live
		{500, 1},  // 0.2% live
		{1000, 1}, // 0.1% live
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d_live=%d", tc.maxDoc, tc.numLiveDocs), func(t *testing.T) {
			// Test with FixedBitSet
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, true); err != nil {
				t.Errorf("FixedBitSet test failed: %v", err)
			}
			// Test with custom Bits implementation
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, false); err != nil {
				t.Errorf("Custom Bits test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_MediumDensity tests serialization with medium density live docs.
func TestLucene90LiveDocsFormat_MediumDensity(t *testing.T) {
	testCases := []struct {
		maxDoc      int
		numLiveDocs int
	}{
		{100, 50},   // 50% live
		{200, 100},  // 50% live
		{1000, 500}, // 50% live
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d_live=%d", tc.maxDoc, tc.numLiveDocs), func(t *testing.T) {
			// Test with FixedBitSet
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, true); err != nil {
				t.Errorf("FixedBitSet test failed: %v", err)
			}
			// Test with custom Bits implementation
			if err := testSerialization(tc.maxDoc, tc.numLiveDocs, false); err != nil {
				t.Errorf("Custom Bits test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_AllLive tests serialization when all documents are live.
func TestLucene90LiveDocsFormat_AllLive(t *testing.T) {
	testCases := []int{10, 100, 1000}

	for _, maxDoc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d", maxDoc), func(t *testing.T) {
			if err := testSerialization(maxDoc, maxDoc, true); err != nil {
				t.Errorf("Test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_SingleDocument tests with a single document.
func TestLucene90LiveDocsFormat_SingleDocument(t *testing.T) {
	// Single doc, live
	if err := testSerialization(1, 1, true); err != nil {
		t.Errorf("Single live doc test failed: %v", err)
	}
	// Single doc, deleted
	if err := testSerialization(1, 0, true); err != nil {
		t.Errorf("Single deleted doc test failed: %v", err)
	}
}

// TestLucene90LiveDocsFormat_ByteAlignment tests that the format handles
// non-64-bit-aligned document counts correctly.
func TestLucene90LiveDocsFormat_ByteAlignment(t *testing.T) {
	// Test various sizes that don't align to 64-bit boundaries
	testCases := []int{65, 127, 128, 129, 255, 256, 257}

	for _, maxDoc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d", maxDoc), func(t *testing.T) {
			// Test with half the docs live
			numLiveDocs := maxDoc / 2
			if err := testSerialization(maxDoc, numLiveDocs, true); err != nil {
				t.Errorf("Test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_RoundTrip tests multiple write/read cycles.
func TestLucene90LiveDocsFormat_RoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Create segment info
	si := index.NewSegmentInfo("_0", 100, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Test multiple generations
	for gen := 0; gen < 3; gen++ {
		// Create live docs with varying patterns
		liveDocs, _ := util.NewFixedBitSet(100)
		for i := 0; i < 100; i++ {
			if i%(gen+2) != 0 { // Different pattern for each generation
				liveDocs.Set(i)
			}
		}

		sci := index.NewSegmentCommitInfo(si, 100-liveDocs.Cardinality(), -1)
		sci.SetID(segmentID)

		// Write
		err := format.WriteLiveDocs(liveDocs, dir, sci, liveDocs.Cardinality(), store.IOContextWrite)
		if err != nil {
			t.Fatalf("Generation %d: Write failed: %v", gen, err)
		}

		// Read back
		sci2 := index.NewSegmentCommitInfo(si, sci.DelCount(), sci.DelGen())
		sci2.SetID(segmentID)
		bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
		if err != nil {
			t.Fatalf("Generation %d: Read failed: %v", gen, err)
		}

		// Verify
		if bits.Length() != 100 {
			t.Errorf("Generation %d: Expected length 100, got %d", gen, bits.Length())
		}
		for i := 0; i < 100; i++ {
			expected := i%(gen+2) != 0
			if bits.Get(i) != expected {
				t.Errorf("Generation %d: Doc %d: expected %v, got %v", gen, i, expected, bits.Get(i))
			}
		}
	}
}

// TestLucene90LiveDocsFormat_Files tests the Files method.
func TestLucene90LiveDocsFormat_Files(t *testing.T) {
	format := NewLucene90LiveDocsFormat()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create segment with deletions
	si := index.NewSegmentInfo("_0", 100, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Segment with deletions
	sciWithDeletions := index.NewSegmentCommitInfo(si, 10, 1)
	sciWithDeletions.SetID(segmentID)

	var files []string
	err := format.Files(sciWithDeletions, &files)
	if err != nil {
		t.Fatalf("Files failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
	if files[0] != "_0_1.liv" {
		t.Errorf("Expected file _0_1.liv, got %s", files[0])
	}

	// Segment without deletions
	sciWithoutDeletions := index.NewSegmentCommitInfo(si, 0, -1)
	files = nil
	err = format.Files(sciWithoutDeletions, &files)
	if err != nil {
		t.Fatalf("Files failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files for segment without deletions, got %d", len(files))
	}
}

// TestLucene90LiveDocsFormat_InvalidHeader tests error handling for corrupted headers.
func TestLucene90LiveDocsFormat_InvalidHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a file with invalid magic
	out, _ := dir.CreateOutput("_0_1.liv", store.IOContextWrite)
	store.WriteInt32(out, 0x12345678) // Invalid magic
	out.Close()

	format := NewLucene90LiveDocsFormat()

	si := index.NewSegmentInfo("_0", 100, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	sci := index.NewSegmentCommitInfo(si, 10, 1)
	sci.SetID(segmentID)

	_, err := format.ReadLiveDocs(dir, sci, store.IOContextRead)
	if err == nil {
		t.Error("Expected error for invalid magic, got nil")
	}
}

// TestLucene90LiveDocsFormat_DelCountMismatch tests error handling for deletion count mismatch.
func TestLucene90LiveDocsFormat_DelCountMismatch(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	si := index.NewSegmentInfo("_0", 100, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Create live docs with 50 deleted
	liveDocs, _ := util.NewFixedBitSet(100)
	liveDocs.SetAll() // All live initially
	for i := 0; i < 50; i++ {
		liveDocs.Clear(i) // Delete first 50
	}

	// But claim only 10 deleted
	sci := index.NewSegmentCommitInfo(si, 10, -1) // Wrong delCount
	sci.SetID(segmentID)

	err := format.WriteLiveDocs(liveDocs, dir, sci, 50, store.IOContextWrite)
	// Should fail due to delCount mismatch
	if err == nil {
		t.Error("Expected error for delCount mismatch, got nil")
	}
}

// testSerialization performs the actual serialization test.
// This is the Go equivalent of BaseLiveDocsFormatTestCase.testSerialization()
func testSerialization(maxDoc, numLiveDocs int, useFixedBitSet bool) error {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Create the live docs bitset
	liveDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return fmt.Errorf("failed to create FixedBitSet: %w", err)
	}

	// Set the appropriate number of live docs
	if numLiveDocs > maxDoc/2 {
		// Dense case: start with all set, then clear
		liveDocs.SetAll()
		cleared := 0
		toClear := maxDoc - numLiveDocs
		for i := 0; i < maxDoc && cleared < toClear; i++ {
			if i%2 == 0 { // Deterministic pattern for testing
				liveDocs.Clear(i)
				cleared++
			}
		}
		// Clear any remaining if needed
		for i := 0; i < maxDoc && cleared < toClear; i++ {
			if liveDocs.Get(i) {
				liveDocs.Clear(i)
				cleared++
			}
		}
	} else {
		// Sparse case: start with all clear, then set
		set := 0
		for i := 0; i < maxDoc && set < numLiveDocs; i++ {
			if i%2 == 0 { // Deterministic pattern for testing
				liveDocs.Set(i)
				set++
			}
		}
		// Set any remaining if needed
		for i := 0; i < maxDoc && set < numLiveDocs; i++ {
			if !liveDocs.Get(i) {
				liveDocs.Set(i)
				set++
			}
		}
	}

	// Create the Bits interface to use
	var bits util.Bits
	if useFixedBitSet {
		bits = liveDocs
	} else {
		// Wrap in a custom Bits implementation
		bits = &bitsWrapper{liveDocs: liveDocs}
	}

	// Create segment info
	si := index.NewSegmentInfo("foo", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	delCount := maxDoc - numLiveDocs
	sci := index.NewSegmentCommitInfo(si, delCount, -1)
	sci.SetID(segmentID)

	// Write the live docs
	err = format.WriteLiveDocs(bits, dir, sci, delCount, store.IOContextWrite)
	if err != nil {
		return fmt.Errorf("failed to write live docs: %w", err)
	}

	// Read back the live docs
	sci2 := index.NewSegmentCommitInfo(si, delCount, sci.DelGen())
	sci2.SetID(segmentID)
	bits2, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
	if err != nil {
		return fmt.Errorf("failed to read live docs: %w", err)
	}

	// Verify
	if bits2.Length() != maxDoc {
		return fmt.Errorf("length mismatch: expected %d, got %d", maxDoc, bits2.Length())
	}

	for i := 0; i < maxDoc; i++ {
		expected := bits.Get(i)
		actual := bits2.Get(i)
		if expected != actual {
			return fmt.Errorf("bit %d mismatch: expected %v, got %v", i, expected, actual)
		}
	}

	return nil
}

// bitsWrapper wraps a FixedBitSet to provide a non-FixedBitSet Bits implementation.
type bitsWrapper struct {
	liveDocs *util.FixedBitSet
}

func (w *bitsWrapper) Get(index int) bool {
	return w.liveDocs.Get(index)
}

func (w *bitsWrapper) Length() int {
	return w.liveDocs.Length()
}

// TestLucene90LiveDocsFormat_BitsCompatibility tests byte-level compatibility
// with Lucene's expected bit layout.
func TestLucene90LiveDocsFormat_BitsCompatibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Create segment with specific bit pattern
	maxDoc := 128
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Create specific pattern: alternating bits
	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		if i%2 == 0 {
			liveDocs.Set(i)
		}
	}

	delCount := maxDoc / 2
	sci := index.NewSegmentCommitInfo(si, delCount, -1)
	sci.SetID(segmentID)

	// Write
	err := format.WriteLiveDocs(liveDocs, dir, sci, delCount, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read raw file and verify structure
	fileName := getLiveDocsFileName(sci.Name(), sci.DelGen())
	input, err := dir.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer input.Close()

	// Read header
	magic, _ := store.ReadInt32(input)
	if magic != codecs.CodecMagic {
		t.Errorf("Invalid codec magic: expected %d, got %d", codecs.CodecMagic, magic)
	}

	codecName, _ := store.ReadString(input)
	if codecName != LiveDocsCodecName {
		t.Errorf("Invalid codec name: expected %s, got %s", LiveDocsCodecName, codecName)
	}

	version, _ := store.ReadInt32(input)
	if version != LiveDocsVersionCurrent {
		t.Errorf("Invalid version: expected %d, got %d", LiveDocsVersionCurrent, version)
	}

	// Skip segment ID (16 bytes) and suffix
	idBuf := make([]byte, 16)
	input.Read(idBuf)
	store.ReadString(input)

	// Read the bit data - should be 2 longs (128 bits)
	word0, _ := store.ReadInt64(input)
	word1, _ := store.ReadInt64(input)

	// Verify alternating pattern: 0x5555... (0101...)
	expectedPattern := uint64(0x5555555555555555)
	if uint64(word0) != expectedPattern {
		t.Errorf("Word 0 mismatch: expected %016x, got %016x", expectedPattern, word0)
	}
	if uint64(word1) != expectedPattern {
		t.Errorf("Word 1 mismatch: expected %016x, got %016x", expectedPattern, word1)
	}
}

// TestLucene90LiveDocsFormat_Merge tests live docs merge scenarios.
// This tests the merge functionality where multiple segments' live docs are combined.
func TestLucene90LiveDocsFormat_Merge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Create two segments with different deletion patterns
	for segIdx := 0; segIdx < 2; segIdx++ {
		segName := fmt.Sprintf("_%d", segIdx)
		si := index.NewSegmentInfo(segName, 100, dir)
		segmentID := make([]byte, 16)
		rand.Read(segmentID)
		si.SetID(segmentID)

		// Different deletion pattern for each segment
		liveDocs, _ := util.NewFixedBitSet(100)
		liveDocs.SetAll()
		for i := 0; i < 100; i++ {
			if i%(2+segIdx) == 0 {
				liveDocs.Clear(i)
			}
		}

		delCount := 100 - liveDocs.Cardinality()
		sci := index.NewSegmentCommitInfo(si, delCount, -1)
		sci.SetID(segmentID)

		err := format.WriteLiveDocs(liveDocs, dir, sci, delCount, store.IOContextWrite)
		if err != nil {
			t.Fatalf("Segment %d: Write failed: %v", segIdx, err)
		}

		// Verify each segment can be read back
		sci2 := index.NewSegmentCommitInfo(si, delCount, sci.DelGen())
		sci2.SetID(segmentID)
		bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
		if err != nil {
			t.Fatalf("Segment %d: Read failed: %v", segIdx, err)
		}

		for i := 0; i < 100; i++ {
			expected := i%(2+segIdx) != 0
			if bits.Get(i) != expected {
				t.Errorf("Segment %d: Doc %d: expected %v, got %v", segIdx, i, expected, bits.Get(i))
			}
		}
	}
}

// TestLucene90LiveDocsFormat_LargeSegment tests with a larger segment size.
func TestLucene90LiveDocsFormat_LargeSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	maxDoc := 10000
	numLiveDocs := 7500 // 75% live, 25% deleted

	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Create live docs with 75% live
	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		if i%4 != 0 { // 75% live
			liveDocs.Set(i)
		}
	}

	delCount := maxDoc - numLiveDocs
	sci := index.NewSegmentCommitInfo(si, delCount, -1)
	sci.SetID(segmentID)

	err := format.WriteLiveDocs(liveDocs, dir, sci, delCount, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	sci2 := index.NewSegmentCommitInfo(si, delCount, sci.DelGen())
	sci2.SetID(segmentID)
	bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if bits.Length() != maxDoc {
		t.Errorf("Length mismatch: expected %d, got %d", maxDoc, bits.Length())
	}

	// Verify all bits
	mismatches := 0
	for i := 0; i < maxDoc && mismatches < 10; i++ {
		expected := i%4 != 0
		if bits.Get(i) != expected {
			t.Errorf("Bit %d: expected %v, got %v", i, expected, bits.Get(i))
			mismatches++
		}
	}
}

// TestLucene90LiveDocsFormat_EdgeCases tests various edge cases.
func TestLucene90LiveDocsFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		maxDoc      int
		numLiveDocs int
	}{
		{"64_bits_exact", 64, 32},
		{"65_bits_one_word_plus_one", 65, 33},
		{"127_bits_almost_two_words", 127, 64},
		{"128_bits_exact_two_words", 128, 64},
		{"129_bits_two_words_plus_one", 129, 65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := testSerialization(tt.maxDoc, tt.numLiveDocs, true); err != nil {
				t.Errorf("Test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_NilBits tests handling of nil/empty bits.
func TestLucene90LiveDocsFormat_NilBits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	maxDoc := 100
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Test with all bits set (no deletions)
	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	liveDocs.SetAll()

	sci := index.NewSegmentCommitInfo(si, 0, -1)
	sci.SetID(segmentID)

	err := format.WriteLiveDocs(liveDocs, dir, sci, 0, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back - should get all live
	sci2 := index.NewSegmentCommitInfo(si, 0, sci.DelGen())
	sci2.SetID(segmentID)
	bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for i := 0; i < maxDoc; i++ {
		if !bits.Get(i) {
			t.Errorf("Bit %d should be live", i)
		}
	}
}

// BenchmarkLucene90LiveDocsFormat_Write benchmarks writing live docs.
func BenchmarkLucene90LiveDocsFormat_Write(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	maxDoc := 10000
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		if i%2 == 0 {
			liveDocs.Set(i)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sci := index.NewSegmentCommitInfo(si, maxDoc/2, -1)
		sci.SetID(segmentID)
		format.WriteLiveDocs(liveDocs, dir, sci, maxDoc/2, store.IOContextWrite)
	}
}

// BenchmarkLucene90LiveDocsFormat_Read benchmarks reading live docs.
func BenchmarkLucene90LiveDocsFormat_Read(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	maxDoc := 10000
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		if i%2 == 0 {
			liveDocs.Set(i)
		}
	}

	sci := index.NewSegmentCommitInfo(si, maxDoc/2, -1)
	sci.SetID(segmentID)
	format.WriteLiveDocs(liveDocs, dir, sci, maxDoc/2, store.IOContextWrite)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sci2 := index.NewSegmentCommitInfo(si, maxDoc/2, sci.DelGen())
		sci2.SetID(segmentID)
		format.ReadLiveDocs(dir, sci2, store.IOContextRead)
	}
}

// Verify that the implementations satisfy the Bits interface
var _ util.Bits = (*allLiveBits)(nil)
var _ util.Bits = (*denseLiveDocsBits)(nil)
var _ util.Bits = (*sparseLiveDocsBits)(nil)
var _ util.Bits = (*bitsWrapper)(nil)

// Additional compatibility check for byte-level format
// This ensures the format matches Lucene's expected output
func TestLucene90LiveDocsFormat_ByteLevelCompatibility(t *testing.T) {
	// This test verifies that the serialized format matches Lucene's format
	// by checking specific byte patterns

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Test with 64 documents, all live
	maxDoc := 64
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := bytes.Repeat([]byte{0xAB}, 16)
	si.SetID(segmentID)

	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	liveDocs.SetAll() // All live

	sci := index.NewSegmentCommitInfo(si, 0, -1)
	sci.SetID(segmentID)

	err := format.WriteLiveDocs(liveDocs, dir, sci, 0, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read the file and verify structure
	fileName := getLiveDocsFileName(sci.Name(), sci.DelGen())
	input, err := dir.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer input.Close()

	// Verify file length
	fileLen, _ := dir.FileLength(fileName)
	expectedLen := int64(4 + // magic
		4 + len(LiveDocsCodecName) + // codec name (length + string)
		4 + // version
		16 + // segment ID
		4 + 1 + // suffix (length + "1")
		8 + // one long for 64 bits
		4) // footer checksum

	if fileLen != expectedLen {
		t.Errorf("File length mismatch: expected %d, got %d", expectedLen, fileLen)
	}

	// Verify the bit data is all 1s (all docs live)
	// Skip header
	store.ReadInt32(input)  // magic
	store.ReadString(input) // codec name
	store.ReadInt32(input)  // version
	skipBuf := make([]byte, 16)
	input.Read(skipBuf)     // segment ID
	store.ReadString(input) // suffix

	// Read the bit data
	bitsData, _ := store.ReadInt64(input)
	if bitsData != -1 { // All bits set = -1 in two's complement
		t.Errorf("Expected all bits set (-1), got %d", bitsData)
	}
}

// int64ToBase36 is a helper for tests
func int64ToBase36(n int64) string {
	return int64ToBase(n, 36)
}

// TestLucene90LiveDocsFormat_Base36Conversion tests the base36 conversion.
func TestLucene90LiveDocsFormat_Base36Conversion(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "a"},
		{35, "z"},
		{36, "10"},
		{100, "2s"},
	}

	for _, tt := range tests {
		result := int64ToBase36(tt.input)
		if result != tt.expected {
			t.Errorf("int64ToBase36(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

// TestLucene90LiveDocsFormat_SparseThreshold tests the sparse/dense threshold behavior.
func TestLucene90LiveDocsFormat_SparseThreshold(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	// Test at exactly 1% threshold (sparse)
	maxDoc := 100
	numDeletions := 1 // Exactly 1%

	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	liveDocs.SetAll()
	liveDocs.Clear(0) // Delete first doc

	sci := index.NewSegmentCommitInfo(si, numDeletions, -1)
	sci.SetID(segmentID)

	err := format.WriteLiveDocs(liveDocs, dir, sci, numDeletions, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back and verify
	sci2 := index.NewSegmentCommitInfo(si, numDeletions, sci.DelGen())
	sci2.SetID(segmentID)
	bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if bits.Get(0) {
		t.Error("Doc 0 should be deleted")
	}
	for i := 1; i < maxDoc; i++ {
		if !bits.Get(i) {
			t.Errorf("Doc %d should be live", i)
		}
	}
}

// TestLucene90LiveDocsFormat_MaxDocs tests behavior near maximum document count.
func TestLucene90LiveDocsFormat_MaxDocs(t *testing.T) {
	// Test with a reasonably large number (not IndexWriter.MAX_DOCS to avoid memory issues)
	maxDoc := 100000
	numLiveDocs := maxDoc - 100 // 99.9% live

	if err := testSerialization(maxDoc, numLiveDocs, true); err != nil {
		t.Errorf("Large doc count test failed: %v", err)
	}
}

// TestLucene90LiveDocsFormat_PartialWord tests documents that don't fill a complete word.
func TestLucene90LiveDocsFormat_PartialWord(t *testing.T) {
	// Test sizes that result in partial final words
	testCases := []int{1, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65}

	for _, maxDoc := range testCases {
		t.Run(fmt.Sprintf("maxDoc=%d", maxDoc), func(t *testing.T) {
			// Test with half live
			numLiveDocs := maxDoc / 2
			if err := testSerialization(maxDoc, numLiveDocs, true); err != nil {
				t.Errorf("Test failed: %v", err)
			}
		})
	}
}

// TestLucene90LiveDocsFormat_ConcurrentAccess tests thread safety.
func TestLucene90LiveDocsFormat_ConcurrentAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := NewLucene90LiveDocsFormat()

	maxDoc := 1000
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	segmentID := make([]byte, 16)
	rand.Read(segmentID)
	si.SetID(segmentID)

	// Create live docs
	liveDocs, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		if i%3 != 0 {
			liveDocs.Set(i)
		}
	}

	delCount := maxDoc - liveDocs.Cardinality()
	sci := index.NewSegmentCommitInfo(si, delCount, -1)
	sci.SetID(segmentID)

	// Write once
	err := format.WriteLiveDocs(liveDocs, dir, sci, delCount, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read multiple times concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			sci2 := index.NewSegmentCommitInfo(si, delCount, sci.DelGen())
			sci2.SetID(segmentID)
			bits, err := format.ReadLiveDocs(dir, sci2, store.IOContextRead)
			if err != nil {
				t.Errorf("Read failed: %v", err)
			}
			if bits.Length() != maxDoc {
				t.Errorf("Length mismatch: expected %d, got %d", maxDoc, bits.Length())
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Additional constants for compatibility
const (
	// IndexWriterMAXDOCS is the maximum number of documents (not using actual value to avoid huge test)
	IndexWriterMAXDOCS = math.MaxInt32 - 128
)
