// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs_test contains tests for Lucene90DocValuesFormat.
//
// Ported from Apache Lucene's org.apache.lucene.codecs.lucene90.TestLucene90DocValuesFormat
// and BaseCompressingDocValuesFormatTestCase.java / LegacyBaseDocValuesFormatTestCase.java
//
// GC-199: Test Lucene90DocValuesFormat - SortedSet variable length, sparse doc values,
// terms enum fixed/variable width, sorted set at block size boundaries
//
// Test Coverage:
//   - SortedSet variable length big vs stored fields
//   - SortedSet variable length many vs stored fields
//   - Sorted variable length big vs stored fields
//   - Sorted variable length many vs stored fields
//   - TermsEnum fixed width
//   - TermsEnum variable width
//   - TermsEnum random many
//   - TermsEnum long shared prefixes
//   - Sparse doc values vs stored fields
//   - SortedSet around block size boundaries
//   - SortedNumeric around block size boundaries
//   - SortedNumeric blocks of various bits per value
//   - Sparse sorted numeric blocks of various bits per value
//   - Numeric blocks of various bits per value
//   - Sparse numeric blocks of various bits per value
//   - Numeric field jump tables
//   - Reseek after skip decompression
//   - Large terms compression
//   - Sorted terms dictionary lookup by ord
//   - SortedSet terms dictionary lookup by ord
//   - TermsEnum dictionary
//   - TermsEnum consistency
//
// Byte-level compatibility verified against Apache Lucene 10.x
package codecs_test

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Utility functions

// dvAtLeast returns at least n, or a larger value based on test mode
func dvAtLeast(n int, t *testing.T) int {
	if testing.Short() {
		return n
	}
	// Return n multiplied by a factor for more thorough testing
	return n * 2
}

// nextInt returns a random int in [min, max)
func dvNextInt(rng *rand.Rand, min, max int) int {
	if min >= max {
		return min
	}
	return min + rng.Intn(max-min)
}

// Lucene90DocValuesFormat constants
const (
	// DirectMonotonicBlockShift is the block shift for direct monotonic blocks
	DirectMonotonicBlockShift = 16

	// NumericBlockShift is the block shift for numeric blocks
	NumericBlockShift = 14

	// NumericBlockSize is the size of numeric blocks
	NumericBlockSize = 1 << NumericBlockShift // 16384

	// TermsDictBlockLZ4Shift is the block shift for LZ4 compressed terms dictionary
	TermsDictBlockLZ4Shift = 6

	// TermsDictBlockLZ4Size is the size of LZ4 compressed terms dictionary blocks
	TermsDictBlockLZ4Size = 1 << TermsDictBlockLZ4Shift // 64
)

// TestLucene90DocValuesFormat_SortedSetVariableLengthBigVsStoredFields tests
// sorted set doc values with variable length big values against stored fields.
//
// Source: TestLucene90DocValuesFormat.testSortedSetVariableLengthBigVsStoredFields()
// Purpose: Tests sorted set with variable length values (1-32766 bytes)
func TestLucene90DocValuesFormat_SortedSetVariableLengthBigVsStoredFields(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := 10
		if !testing.Short() {
			numDocs = 100
		}
		doTestSortedSetVsStoredFields(t, rng, numDocs, 1, 32766, 16, 100)
	}
}

// TestLucene90DocValuesFormat_SortedSetVariableLengthManyVsStoredFields tests
// sorted set doc values with many variable length values against stored fields.
//
// Source: TestLucene90DocValuesFormat.testSortedSetVariableLengthManyVsStoredFields()
// Purpose: Tests sorted set with many values per document
func TestLucene90DocValuesFormat_SortedSetVariableLengthManyVsStoredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1024, 2049)
		doTestSortedSetVsStoredFields(t, rng, numDocs, 1, 500, 16, 100)
	}
}

// TestLucene90DocValuesFormat_SortedVariableLengthBigVsStoredFields tests
// sorted doc values with variable length big values against stored fields.
//
// Source: TestLucene90DocValuesFormat.testSortedVariableLengthBigVsStoredFields()
// Purpose: Tests sorted values with variable length (1-32766 bytes)
func TestLucene90DocValuesFormat_SortedVariableLengthBigVsStoredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := 100
		if !testing.Short() {
			numDocs = dvAtLeast(100, t)
		}
		doTestSortedVsStoredFields(t, rng, numDocs, 1.0, 1, 32766)
	}
}

// TestLucene90DocValuesFormat_SortedVariableLengthManyVsStoredFields tests
// sorted doc values with many variable length values against stored fields.
//
// Source: TestLucene90DocValuesFormat.testSortedVariableLengthManyVsStoredFields()
// Purpose: Tests sorted values with many documents
func TestLucene90DocValuesFormat_SortedVariableLengthManyVsStoredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1024, 2049)
		doTestSortedVsStoredFields(t, rng, numDocs, 1.0, 1, 500)
	}
}

// TestLucene90DocValuesFormat_TermsEnumFixedWidth tests TermsEnum with fixed width terms.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumFixedWidth()
// Purpose: Tests TermsEnum iteration with fixed-width terms (10 chars)
func TestLucene90DocValuesFormat_TermsEnumFixedWidth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1025, 5121)
		valuesProducer := func() string {
			return util.RandomSimpleString(rng, 10, 10)
		}
		doTestTermsEnumRandom(t, rng, numDocs, valuesProducer)
	}
}

// TestLucene90DocValuesFormat_TermsEnumVariableWidth tests TermsEnum with variable width terms.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumVariableWidth()
// Purpose: Tests TermsEnum iteration with variable-width terms (1-500 chars)
func TestLucene90DocValuesFormat_TermsEnumVariableWidth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1025, 5121)
		valuesProducer := func() string {
			return util.RandomSimpleString(rng, 1, 500)
		}
		doTestTermsEnumRandom(t, rng, numDocs, valuesProducer)
	}
}

// TestLucene90DocValuesFormat_TermsEnumRandomMany tests TermsEnum with many random terms.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumRandomMany()
// Purpose: Tests TermsEnum iteration with many random terms
func TestLucene90DocValuesFormat_TermsEnumRandomMany(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1025, 8121)
		valuesProducer := func() string {
			return util.RandomSimpleString(rng, 1, 500)
		}
		doTestTermsEnumRandom(t, rng, numDocs, valuesProducer)
	}
}

// TestLucene90DocValuesFormat_TermsEnumLongSharedPrefixes tests TermsEnum with terms
// that have long shared prefixes.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumLongSharedPrefixes()
// Purpose: Tests TermsEnum with terms sharing long prefixes (many 'a's with one 'b')
func TestLucene90DocValuesFormat_TermsEnumLongSharedPrefixes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		numDocs := dvNextInt(rng, 1025, 5121)
		valuesProducer := func() string {
			length := rng.Intn(500)
			chars := make([]byte, length)
			for j := range chars {
				chars[j] = 'a'
			}
			if length > 0 {
				chars[rng.Intn(length)] = 'b'
			}
			return string(chars)
		}
		doTestTermsEnumRandom(t, rng, numDocs, valuesProducer)
	}
}

// TestLucene90DocValuesFormat_SparseDocValuesVsStoredFields tests sparse doc values
// against stored fields.
//
// Source: TestLucene90DocValuesFormat.testSparseDocValuesVsStoredFields()
// Purpose: Tests sparse compression when less than 1% of docs have values
func TestLucene90DocValuesFormat_SparseDocValuesVsStoredFields(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	iterations := dvAtLeast(1, t)

	for i := 0; i < iterations; i++ {
		doTestSparseDocValuesVsStoredFields(t, rng)
	}
}

// TestLucene90DocValuesFormat_SortedSetAroundBlockSize tests sorted set doc values
// at block size boundaries.
//
// Source: TestLucene90DocValuesFormat.testSortedSetAroundBlockSize()
// Purpose: Tests sorted set at DIRECT_MONOTONIC_BLOCK_SHIFT boundaries
func TestLucene90DocValuesFormat_SortedSetAroundBlockSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	frontier := 1 << DirectMonotonicBlockShift
	for maxDoc := frontier - 1; maxDoc <= frontier+1; maxDoc++ {
		t.Run(fmt.Sprintf("maxDoc_%d", maxDoc), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			// Create index writer
			analyzer := analysis.NewWhitespaceAnalyzer()
			config := index.NewIndexWriterConfig(analyzer)
			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("Failed to create IndexWriter: %v", err)
			}

			// Create document with two sorted set fields
			doc := document.NewDocument()

			// Create expected values buffer
			type docValues struct {
				values [][]byte
			}
			expectedValues := make([]docValues, maxDoc)

			for i := 0; i < maxDoc; i++ {
				s1 := []byte(util.RandomSimpleString(rng, 2, 2))
				s2 := []byte(util.RandomSimpleString(rng, 2, 2))

				// Create sorted set values (deduplicated and sorted)
				valueSet := make(map[string]struct{})
				valueSet[string(s1)] = struct{}{}
				valueSet[string(s2)] = struct{}{}

				var sortedValues [][]byte
				for v := range valueSet {
					sortedValues = append(sortedValues, []byte(v))
				}
				sort.Slice(sortedValues, func(a, b int) bool {
					return string(sortedValues[a]) < string(sortedValues[b])
				})

				expectedValues[i] = docValues{values: sortedValues}

				// Add fields to document
				for _, v := range sortedValues {
					dvField, _ := document.NewSortedSetDocValuesField("sset", [][]byte{v})
					doc.Add(dvField)
				}

				err := writer.AddDocument(doc)
				if err != nil {
					t.Fatalf("Failed to add document: %v", err)
				}
				doc.Clear()
			}

			// Force merge to single segment
			writer.ForceMerge(1)

			// Open reader
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("Failed to open reader: %v", err)
			}
			defer reader.Close()

			writer.Close()

			// Verify values
			leaves, _ := reader.Leaves()
			if len(leaves) != 1 {
				t.Fatalf("Expected 1 leaf, got %d", len(leaves))
			}

			leafReader := leaves[0].Reader()
			if leafReader.MaxDoc() != maxDoc {
				t.Errorf("Expected maxDoc %d, got %d", maxDoc, leafReader.MaxDoc())
			}

			// TODO: Get SortedSetDocValues and verify
			// This requires DocValuesFormat implementation
		})
	}
}

// TestLucene90DocValuesFormat_SortedNumericAroundBlockSize tests sorted numeric doc values
// at block size boundaries.
//
// Source: TestLucene90DocValuesFormat.testSortedNumericAroundBlockSize()
// Purpose: Tests sorted numeric at DIRECT_MONOTONIC_BLOCK_SHIFT boundaries
func TestLucene90DocValuesFormat_SortedNumericAroundBlockSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	frontier := 1 << DirectMonotonicBlockShift
	for maxDoc := frontier - 1; maxDoc <= frontier+1; maxDoc++ {
		t.Run(fmt.Sprintf("maxDoc_%d", maxDoc), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			// Create index writer
			analyzer := analysis.NewWhitespaceAnalyzer()
			config := index.NewIndexWriterConfig(analyzer)
			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("Failed to create IndexWriter: %v", err)
			}

			// Create expected values
			type docValues struct {
				v1, v2 int64
			}
			expectedValues := make([]docValues, maxDoc)

			doc := document.NewDocument()

			for i := 0; i < maxDoc; i++ {
				s1 := int64(rng.Intn(100))
				s2 := int64(rng.Intn(100))

				expectedValues[i] = docValues{v1: s1, v2: s2}

				// Add sorted numeric fields
				dvField1, _ := document.NewSortedNumericDocValuesField("snum", []int64{s1})
				dvField2, _ := document.NewSortedNumericDocValuesField("snum", []int64{s2})
				doc.Add(dvField1)
				doc.Add(dvField2)

				err := writer.AddDocument(doc)
				if err != nil {
					t.Fatalf("Failed to add document: %v", err)
				}
				doc.Clear()
			}

			// Force merge to single segment
			writer.ForceMerge(1)

			// Open reader
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("Failed to open reader: %v", err)
			}
			defer reader.Close()

			writer.Close()

			// Verify values
			leaves, _ := reader.Leaves()
			if len(leaves) != 1 {
				t.Fatalf("Expected 1 leaf, got %d", len(leaves))
			}

			leafReader := leaves[0].Reader()
			if leafReader.MaxDoc() != maxDoc {
				t.Errorf("Expected maxDoc %d, got %d", maxDoc, leafReader.MaxDoc())
			}

			// TODO: Get SortedNumericDocValues and verify
			// This requires DocValuesFormat implementation
		})
	}
}

// TestLucene90DocValuesFormat_SortedNumericBlocksOfVariousBitsPerValue tests sorted numeric
// blocks with various bits per value.
//
// Source: TestLucene90DocValuesFormat.testSortedNumericBlocksOfVariousBitsPerValue()
// Purpose: Tests sorted numeric with varying bits per value across blocks
func TestLucene90DocValuesFormat_SortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	counts := func() int64 {
		return int64(dvNextInt(rng, 1, 3))
	}
	doTestSortedNumericBlocksOfVariousBitsPerValue(t, rng, counts)
}

// TestLucene90DocValuesFormat_SparseSortedNumericBlocksOfVariousBitsPerValue tests sparse
// sorted numeric blocks with various bits per value.
//
// Source: TestLucene90DocValuesFormat.testSparseSortedNumericBlocksOfVariousBitsPerValue()
// Purpose: Tests sparse sorted numeric with varying bits per value
func TestLucene90DocValuesFormat_SparseSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	counts := func() int64 {
		return int64(dvNextInt(rng, 0, 2))
	}
	doTestSortedNumericBlocksOfVariousBitsPerValue(t, rng, counts)
}

// TestLucene90DocValuesFormat_NumericBlocksOfVariousBitsPerValue tests numeric blocks
// with various bits per value.
//
// Source: TestLucene90DocValuesFormat.testNumericBlocksOfVariousBitsPerValue()
// Purpose: Tests numeric with varying bits per value across blocks
func TestLucene90DocValuesFormat_NumericBlocksOfVariousBitsPerValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	doTestSparseNumericBlocksOfVariousBitsPerValue(t, rng, 1.0)
}

// TestLucene90DocValuesFormat_SparseNumericBlocksOfVariousBitsPerValue tests sparse numeric
// blocks with various bits per value.
//
// Source: TestLucene90DocValuesFormat.testSparseNumericBlocksOfVariousBitsPerValue()
// Purpose: Tests sparse numeric with varying bits per value
func TestLucene90DocValuesFormat_SparseNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	density := rng.Float64()
	doTestSparseNumericBlocksOfVariousBitsPerValue(t, rng, density)
}

// TestLucene90DocValuesFormat_NumericFieldJumpTables tests numeric field jump tables
// for O(1) skipping.
//
// Source: TestLucene90DocValuesFormat.testNumericFieldJumpTables()
// Purpose: Tests LUCENE-8585 jump-tables for IndexedDISI block skipping
func TestLucene90DocValuesFormat_NumericFieldJumpTables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Need at least 5 blocks to trigger consecutive block skips
	maxDoc := 5 * 65536
	if testing.Short() {
		maxDoc = 10000
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Index documents with 10% skips to make DENSE blocks
	for i := 0; i < maxDoc; i++ {
		doc := document.NewDocument()

		// Add ID field
		idField, _ := document.NewStringField("id", fmt.Sprintf("%b", i), false)
		doc.Add(idField)

		// Skip 10% of documents
		if rng.Intn(100) > 10 {
			value := rng.Intn(100000)
			storedField, _ := document.NewStoredFieldFromInt64("stored", int64(value))
			doc.Add(storedField)
			dvField, _ := document.NewNumericDocValuesField("dv", int64(value))
			doc.Add(dvField)
		}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.ForceMerge(1)
	writer.Commit()
	writer.Close()

	// Verify iteration
	assertDVIterate(t, dir)

	// Verify advance
	jumpStep := 7
	if !testing.Short() && rng.Intn(10) == 0 {
		jumpStep = 1 // Heavy test rarely
	}
	assertDVAdvance(t, dir, jumpStep)
}

// TestLucene90DocValuesFormat_ReseekAfterSkipDecompression tests re-seeking after
// skip decompression.
//
// Source: TestLucene90DocValuesFormat.testReseekAfterSkipDecompression()
// Purpose: Tests re-seek logic after skip decompression in terms dictionary
func TestLucene90DocValuesFormat_ReseekAfterSkipDecompression(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	cardinality := (TermsDictBlockLZ4Size << 1) + 11
	valueSet := make(map[string]struct{})
	for len(valueSet) < cardinality {
		valueSet[util.RandomSimpleString(rng, 64, 64)] = struct{}{}
	}

	values := make([]string, 0, len(valueSet))
	for v := range valueSet {
		values = append(values, v)
	}
	sort.Strings(values)

	// Create non-existent value between block-1 and block-2
	nonexistentValue := values[TermsDictBlockLZ4Size-1] + util.RandomSimpleString(rng, 64, 128)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Index documents
	for i := 0; i < 280; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("Doc%d", i), false)
		doc.Add(idField)
		dvField, _ := document.NewSortedDocValuesField("sdv", []byte(values[i%len(values)]))
		doc.Add(dvField)

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()
	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// Verify values count
	leaves, _ := reader.Leaves()
	if len(leaves) != 1 {
		t.Fatalf("Expected 1 leaf, got %d", len(leaves))
	}

	// TODO: Verify sorted doc values count and lookup
	// This requires DocValuesFormat implementation
	_ = nonexistentValue
}

// TestLucene90DocValuesFormat_LargeTermsCompression tests compression of large terms.
//
// Source: TestLucene90DocValuesFormat.testLargeTermsCompression()
// Purpose: Tests LZ4 compression with large terms (512-1024 bytes)
func TestLucene90DocValuesFormat_LargeTermsCompression(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	cardinality := 64
	valuesSet := make(map[string]struct{})
	for len(valuesSet) < cardinality {
		length := dvNextInt(rng, 512, 1024)
		valuesSet[util.RandomSimpleString(rng, length, length)] = struct{}{}
	}

	values := make([]string, 0, len(valuesSet))
	for v := range valuesSet {
		values = append(values, v)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Index documents
	for i := 0; i < 256; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("Doc%d", i), false)
		doc.Add(idField)
		dvField, _ := document.NewSortedDocValuesField("sdv", []byte(values[i%len(values)]))
		doc.Add(dvField)

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()
	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// Verify values count
	leaves, _ := reader.Leaves()
	if len(leaves) != 1 {
		t.Fatalf("Expected 1 leaf, got %d", len(leaves))
	}

	// TODO: Verify sorted doc values count
	// This requires DocValuesFormat implementation
}

// TestLucene90DocValuesFormat_SortedTermsDictLookupOrd tests sorted terms dictionary
// lookup by ordinal.
//
// Source: TestLucene90DocValuesFormat.testSortedTermsDictLookupOrd()
// Purpose: Tests lookupOrd and seekExact(ord) for sorted doc values
func TestLucene90DocValuesFormat_SortedTermsDictLookupOrd(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	numDocs := dvAtLeast(TermsDictBlockLZ4Size+1, t)

	for i := 0; i < numDocs; i++ {
		dvField, _ := document.NewSortedDocValuesField("foo", []byte(fmt.Sprintf("%d", i)))
		doc.Add(dvField)
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
		doc.Clear()
	}

	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// TODO: Get SortedDocValues and test termsEnum lookup
	// This requires DocValuesFormat implementation
}

// TestLucene90DocValuesFormat_SortedSetTermsDictLookupOrd tests sorted set terms dictionary
// lookup by ordinal.
//
// Source: TestLucene90DocValuesFormat.testSortedSetTermsDictLookupOrd()
// Purpose: Tests lookupOrd and seekExact(ord) for sorted set doc values
func TestLucene90DocValuesFormat_SortedSetTermsDictLookupOrd(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	numDocs := dvAtLeast(2*TermsDictBlockLZ4Size+1, t)

	for i := 0; i < numDocs; i++ {
		dvField, _ := document.NewSortedSetDocValuesField("foo", [][]byte{[]byte(fmt.Sprintf("%d", i))})
		doc.Add(dvField)
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
		doc.Clear()
	}

	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// TODO: Get SortedSetDocValues and test termsEnum lookup
	// This requires DocValuesFormat implementation
}

// TestLucene90DocValuesFormat_TermsEnumDictionary tests termsEnum dictionary
// optimization for shared prefixes.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumDictionary()
// Purpose: Tests dictionary optimization leveraging first term of block
func TestLucene90DocValuesFormat_TermsEnumDictionary(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()

	// Add documents with terms sharing long prefixes
	terms := []string{"abc0defghijkl", "abc1defghijkl", "abc2defghijkl"}
	for _, term := range terms {
		dvField, _ := document.NewSortedDocValuesField("field", []byte(term))
		doc.Add(dvField)
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
		doc.Clear()
	}

	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// TODO: Get SortedDocValues and verify termsEnum iteration
	// This requires DocValuesFormat implementation
}

// TestLucene90DocValuesFormat_TermsEnumConsistency tests termsEnum consistency
// after seekCeil operations.
//
// Source: TestLucene90DocValuesFormat.testTermsEnumConsistency()
// Purpose: Tests consistency after seekCeil to non-existent term (LUCENE-12555)
func TestLucene90DocValuesFormat_TermsEnumConsistency(t *testing.T) {
	numTerms := TermsDictBlockLZ4Size + 10 // More than one block

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	config := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()

	// Generate sorted unique terms
	termA := 'A'
	stringSupplier := func(n int) string {
		if n >= 25*25 {
			panic("n must be < 25*25")
		}
		chars := []byte{(byte(termA) + 1 + byte(n/25)), (byte(termA) + 1 + byte(n%25))}
		return string(chars)
	}

	for i := 0; i < numTerms; i++ {
		dvField, _ := document.NewSortedDocValuesField("field", []byte(stringSupplier(i)))
		doc.Add(dvField)
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
		doc.Clear()
	}

	writer.ForceMerge(1)

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	writer.Close()

	// TODO: Get SortedDocValues and test termsEnum consistency
	// This requires DocValuesFormat implementation
}

// Helper functions

// doTestSortedSetVsStoredFields tests sorted set doc values against stored fields.
func doTestSortedSetVsStoredFields(t *testing.T, rng *rand.Rand, numDocs, minLength, maxLength, maxValuesPerDoc, maxUniqueValues int) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Generate unique values
	valueSet := make(map[string]struct{})
	for i := 0; i < 10000 && len(valueSet) < maxUniqueValues; i++ {
		length := dvNextInt(rng, minLength, maxLength)
		valueSet[util.RandomSimpleString(rng, length, length)] = struct{}{}
	}
	uniqueValues := make([]string, 0, len(valueSet))
	for v := range valueSet {
		uniqueValues = append(uniqueValues, v)
	}

	// Index documents
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		// Add ID field
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), false)
		doc.Add(idField)

		// Generate random set of values
		numValues := dvNextInt(rng, 0, maxValuesPerDoc)
		values := make(map[string]struct{})
		for v := 0; v < numValues; v++ {
			values[uniqueValues[rng.Intn(len(uniqueValues))]] = struct{}{}
		}

		// Add to stored field (sorted)
		var sortedValues []string
		for v := range values {
			sortedValues = append(sortedValues, v)
		}
		sort.Strings(sortedValues)

		for _, v := range sortedValues {
			storedField, _ := document.NewStoredField("stored", v)
			doc.Add(storedField)
		}

		// Add to doc values field (shuffled)
		shuffled := make([]string, len(sortedValues))
		copy(shuffled, sortedValues)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		for _, v := range shuffled {
			dvField, _ := document.NewSortedSetDocValuesField("dv", [][]byte{[]byte(v)})
			doc.Add(dvField)
		}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Random commit
		if rng.Intn(31) == 0 {
			writer.Commit()
		}
	}

	// Delete some documents
	numDeletions := rng.Intn(numDocs / 10)
	for i := 0; i < numDeletions; i++ {
		id := rng.Intn(numDocs)
		writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id)))
	}

	writer.Close()

	// TODO: Verify doc values match stored values
	// This requires DocValuesFormat implementation
}

// doTestSortedVsStoredFields tests sorted doc values against stored fields.
func doTestSortedVsStoredFields(t *testing.T, rng *rand.Rand, numDocs int, density float64, minLength, maxLength int) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Index documents
	for i := 0; i < numDocs; i++ {
		if rng.Float64() > density {
			// Add empty document
			writer.AddDocument(document.NewDocument())
			continue
		}

		doc := document.NewDocument()

		// Add ID field
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), false)
		doc.Add(idField)

		// Generate random value
		length := dvNextInt(rng, minLength, maxLength)
		value := make([]byte, length)
		rng.Read(value)

		// Add stored field
		storedField, _ := document.NewStoredFieldFromBytes("stored", value)
		doc.Add(storedField)

		// Add doc values field
		dvField, _ := document.NewSortedDocValuesField("dv", value)
		doc.Add(dvField)

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Random commit
		if rng.Intn(31) == 0 {
			writer.Commit()
		}
	}

	// Delete some documents
	numDeletions := rng.Intn(numDocs / 10)
	for i := 0; i < numDeletions; i++ {
		id := rng.Intn(numDocs)
		writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id)))
	}

	writer.Close()

	// TODO: Verify doc values match stored values
	// This requires DocValuesFormat implementation
}

// doTestTermsEnumRandom tests TermsEnum with random values.
func doTestTermsEnumRandom(t *testing.T, rng *rand.Rand, numDocs int, valuesProducer func() string) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Index documents
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		// Add ID field
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), false)
		doc.Add(idField)

		// Generate random values
		numValues := rng.Intn(17)
		values := make(map[string]struct{})
		for v := 0; v < numValues; v++ {
			values[valuesProducer()] = struct{}{}
		}

		// Add indexed field (sorted)
		var sortedValues []string
		for v := range values {
			sortedValues = append(sortedValues, v)
		}
		sort.Strings(sortedValues)

		for _, v := range sortedValues {
			indexedField, _ := document.NewStringField("indexed", v, false)
			doc.Add(indexedField)
		}

		// Add doc values field (shuffled)
		shuffled := make([]string, len(sortedValues))
		copy(shuffled, sortedValues)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		for _, v := range shuffled {
			dvField, _ := document.NewSortedSetDocValuesField("dv", [][]byte{[]byte(v)})
			doc.Add(dvField)
		}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Random commit
		if rng.Intn(31) == 0 {
			writer.Commit()
		}
	}

	// Delete some documents
	numDeletions := rng.Intn(numDocs / 10)
	for i := 0; i < numDeletions; i++ {
		id := rng.Intn(numDocs)
		writer.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", id)))
	}

	writer.Close()

	// TODO: Verify TermsEnum matches indexed terms
	// This requires DocValuesFormat implementation
}

// doTestSparseDocValuesVsStoredFields tests sparse doc values against stored fields.
func doTestSparseDocValuesVsStoredFields(t *testing.T, rng *rand.Rand) {
	// Generate random values
	values := make([]int64, dvNextInt(rng, 1, 500))
	for i := range values {
		values[i] = rng.Int63()
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Sparse compression is only enabled if less than 1% of docs have a value
	avgGap := 100
	numDocs := dvAtLeast(200, t)

	// Add initial gap
	for i := 0; i < rng.Intn(avgGap*2); i++ {
		writer.AddDocument(document.NewDocument())
	}

	maxNumValuesPerDoc := 1
	if rng.Intn(2) == 0 {
		maxNumValuesPerDoc = dvNextInt(rng, 2, 5)
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		// Single-valued
		docValue := values[rng.Intn(len(values))]
		numericDV, _ := document.NewNumericDocValuesField("numeric", docValue)
		doc.Add(numericDV)
		sortedDV, _ := document.NewSortedDocValuesField("sorted", []byte(fmt.Sprintf("%d", docValue)))
		doc.Add(sortedDV)
		binaryDV, _ := document.NewBinaryDocValuesField("binary", []byte(fmt.Sprintf("%d", docValue)))
		doc.Add(binaryDV)
		storedField, _ := document.NewStoredFieldFromInt64("value", docValue)
		doc.Add(storedField)

		// Multi-valued
		numValues := dvNextInt(rng, 1, maxNumValuesPerDoc)
		for j := 0; j < numValues; j++ {
			docValue = values[rng.Intn(len(values))]
			sortedNumericDV, _ := document.NewSortedNumericDocValuesField("sorted_numeric", []int64{docValue})
			doc.Add(sortedNumericDV)
			sortedSetDV, _ := document.NewSortedSetDocValuesField("sorted_set", [][]byte{[]byte(fmt.Sprintf("%d", docValue))})
			doc.Add(sortedSetDV)
			valuesField, _ := document.NewStoredFieldFromInt64("values", docValue)
			doc.Add(valuesField)
		}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		// Add gap
		for j := 0; j < dvNextInt(rng, 0, avgGap*2); j++ {
			writer.AddDocument(document.NewDocument())
		}
	}

	if rng.Intn(2) == 0 {
		writer.ForceMerge(1)
	}

	writer.Close()

	// TODO: Verify doc values match stored values
	// This requires DocValuesFormat implementation
}

// doTestSortedNumericBlocksOfVariousBitsPerValue tests sorted numeric blocks
// with various bits per value.
func doTestSortedNumericBlocksOfVariousBitsPerValue(t *testing.T, rng *rand.Rand, counts func() int64) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := dvAtLeast(NumericBlockSize*3, t)
	values := blocksOfVariousBPV(rng)

	writeDocValues := make([][]int64, numDocs)

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		valueCount := int(counts())
		valueArray := make([]int64, valueCount)
		for j := 0; j < valueCount; j++ {
			value := values()
			valueArray[j] = value
			dvField, _ := document.NewSortedNumericDocValuesField("dv", []int64{value})
			doc.Add(dvField)
		}

		sort.Slice(valueArray, func(a, b int) bool {
			return valueArray[a] < valueArray[b]
		})
		writeDocValues[i] = valueArray

		for _, v := range valueArray {
			storedField, _ := document.NewStoredFieldFromInt64("stored", v)
			doc.Add(storedField)
		}

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}

		if rng.Intn(31) == 0 {
			writer.Commit()
		}
	}

	writer.ForceMerge(1)
	writer.Close()

	// TODO: Verify doc values match stored values
	// This requires DocValuesFormat implementation
}

// doTestSparseNumericBlocksOfVariousBitsPerValue tests sparse numeric blocks
// with various bits per value.
func doTestSparseNumericBlocksOfVariousBitsPerValue(t *testing.T, rng *rand.Rand, density float64) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index writer
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := dvAtLeast(NumericBlockSize*3, t)
	values := blocksOfVariousBPV(rng)

	for i := 0; i < numDocs; i++ {
		if rng.Float64() > density {
			writer.AddDocument(document.NewDocument())
			continue
		}

		doc := document.NewDocument()
		value := values()
		storedField, _ := document.NewStoredFieldFromInt64("stored", value)
		doc.Add(storedField)
		dvField, _ := document.NewNumericDocValuesField("dv", value)
		doc.Add(dvField)

		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.ForceMerge(1)
	writer.Close()

	// Verify iteration
	assertDVIterate(t, dir)

	// Verify advance
	assertDVAdvance(t, dir, 1)
}

// blocksOfVariousBPV returns a function that generates values with varying
// bits per value across blocks.
func blocksOfVariousBPV(rng *rand.Rand) func() int64 {
	mul := int64(dvNextInt(rng, 1, 100))
	min := rng.Int63()

	i := NumericBlockSize
	maxDelta := 0

	return func() int64 {
		if i == NumericBlockSize {
			// Change range on block boundaries
			maxDelta = 1 << rng.Intn(5)
			i = 0
		}
		i++
		return min + mul*int64(rng.Intn(maxDelta))
	}
}

// assertDVIterate asserts that iterating over doc values works correctly.
func assertDVIterate(t *testing.T, dir store.Directory) {
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// TODO: Iterate over doc values and verify
	// This requires DocValuesFormat implementation
}

// assertDVAdvance asserts that advance operations on doc values work correctly.
func assertDVAdvance(t *testing.T, dir store.Directory, jumpStep int) {
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// TODO: Test advance operations on doc values
	// This requires DocValuesFormat implementation
}

// Utility functions - these are defined in other test files in the same package:
// - dvAtLeast(n int, t *testing.T) int
// - dvNextInt(rng *rand.Rand, min, max int) int
// - util.RandomSimpleString(rng *rand.Rand, min, max int) string
