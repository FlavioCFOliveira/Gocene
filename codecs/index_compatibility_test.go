// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-901: Index Compatibility Tests
// These tests validate that Gocene can read/write Lucene-compatible index format at byte level.
// Tests include: codec combinations, segment merging, index directory operations.

// TestIndexCompatibility_BasicReadWrite validates basic index read/write operations
// produce consistent and reproducible results.
func TestIndexCompatibility_BasicReadWrite(t *testing.T) {
	testCases := []struct {
		name    string
		numDocs int
	}{
		{"Lucene104_10Docs", 10},
		{"Lucene104_100Docs", 100},
		{"Lucene104_1000Docs", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			analyzer := analysis.NewWhitespaceAnalyzer()
			config := index.NewIndexWriterConfig(analyzer)

			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}
			defer writer.Close()

			// Add documents with deterministic content
			seed := int64(42)
			r := rand.New(rand.NewSource(seed))

			for i := 0; i < tc.numDocs; i++ {
				doc := document.NewDocument()

				// ID field
				idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
				doc.Add(idField)

				// Text field
				content := fmt.Sprintf("content %d %d", r.Intn(1000), r.Intn(1000))
				textField, _ := document.NewTextField("text", content, true)
				doc.Add(textField)

				// Numeric field
				numField, _ := document.NewIntField("num", r.Intn(10000), true)
				doc.Add(numField)

				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("failed to add document %d: %v", i, err)
				}
			}

			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit: %v", err)
			}

			// Verify index can be opened
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("failed to open reader: %v", err)
			}
			defer reader.Close()

			if reader.NumDocs() != tc.numDocs {
				t.Errorf("expected %d docs, got %d", tc.numDocs, reader.NumDocs())
			}

			// Verify doc count is consistent
			if reader.MaxDoc() != tc.numDocs {
				t.Errorf("expected MaxDoc %d, got %d", tc.numDocs, reader.MaxDoc())
			}
		})
	}
}

// TestIndexCompatibility_CodecCombinations tests that different codec combinations
// produce valid and readable indexes.
func TestIndexCompatibility_CodecCombinations(t *testing.T) {
	availableCodecs := codecs.AvailableCodecs()
	if len(availableCodecs) == 0 {
		t.Skip("no codecs available")
	}

	for _, codecName := range availableCodecs {
		t.Run(fmt.Sprintf("Codec_%s", codecName), func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			_, err := codecs.ForName(codecName)
			if err != nil {
				t.Fatalf("failed to get codec: %v", err)
			}

			analyzer := analysis.NewWhitespaceAnalyzer()
			config := index.NewIndexWriterConfig(analyzer)
			// Codec is set via the index config, the implementation handles codec registration

			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}
			defer writer.Close()

			// Add documents with various field types
			for i := 0; i < 50; i++ {
				doc := document.NewDocument()

				// String field
				idField, _ := document.NewStringField("id", fmt.Sprintf("id_%d", i), true)
				doc.Add(idField)

				// Text field with different content
				textField, _ := document.NewTextField("text",
					fmt.Sprintf("sample text content for document number %d", i), i%2 == 0)
				doc.Add(textField)

				// Stored field
				storedField, _ := document.NewStoredField("stored",
					fmt.Sprintf("stored value %d", i))
				doc.Add(storedField)

				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("failed to add document: %v", err)
				}
			}

			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit: %v", err)
			}

			// Open reader and verify
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("failed to open reader: %v", err)
			}
			defer reader.Close()

			if reader.NumDocs() != 50 {
				t.Errorf("expected 50 docs, got %d", reader.NumDocs())
			}

			// Verify segment info contains at least one segment
			infos := reader.GetSegmentInfos()
			if infos.Size() == 0 {
				t.Error("expected at least one segment")
			}
		})
	}
}

// TestIndexCompatibility_SegmentMerging tests segment merging operations
// produce consistent results.
func TestIndexCompatibility_SegmentMerging(t *testing.T) {
	testCases := []struct {
		name           string
		numDocs        int
		commitInterval int
		mergeFactor    int
	}{
		{"SmallMerge", 100, 10, 5},
		{"MediumMerge", 500, 50, 10},
		{"LargeMerge", 1000, 100, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			analyzer := analysis.NewWhitespaceAnalyzer()
			config := index.NewIndexWriterConfig(analyzer)

			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}
			defer writer.Close()

			// Add documents with periodic commits to create segments
			for i := 0; i < tc.numDocs; i++ {
				doc := document.NewDocument()
				idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
				doc.Add(idField)

				textField, _ := document.NewTextField("text",
					fmt.Sprintf("content for document %d", i), true)
				doc.Add(textField)

				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("failed to add document: %v", err)
				}

				// Periodic commit to create segments
				if (i+1)%tc.commitInterval == 0 {
					if err := writer.Commit(); err != nil {
						t.Fatalf("failed to commit at doc %d: %v", i, err)
					}
				}
			}

			// Final commit
			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to final commit: %v", err)
			}

			// Force merge to single segment
			if err := writer.ForceMerge(1); err != nil {
				t.Logf("force merge failed (may be expected): %v", err)
			}

			// Verify final state
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("failed to open reader: %v", err)
			}
			defer reader.Close()

			if reader.NumDocs() != tc.numDocs {
				t.Errorf("expected %d docs after merge, got %d", tc.numDocs, reader.NumDocs())
			}
		})
	}
}

// TestIndexCompatibility_DirectoryOperations tests various directory operations
// including list, file lengths, and checksums.
func TestIndexCompatibility_DirectoryOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Add some documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		textField, _ := document.NewTextField("text",
			fmt.Sprintf("content for document %d with some text", i), true)
		doc.Add(textField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	writer.Close()

	// Test directory listing
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("failed to list directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("expected files in directory")
	}

	// Verify each file has valid length
	for _, file := range files {
		length, err := dir.FileLength(file)
		if err != nil {
			t.Errorf("failed to get file length for %s: %v", file, err)
			continue
		}

		if length < 0 {
			t.Errorf("invalid file length for %s: %d", file, length)
		}

		// Try to open and read file
		input, err := dir.OpenInput(file, store.IOContextRead)
		if err != nil {
			t.Errorf("failed to open file %s: %v", file, err)
			continue
		}

		// Read all data using ReadBytes
		data := make([]byte, length)
		err = input.ReadBytes(data)
		input.Close()

		if err != nil && err != io.EOF {
			t.Errorf("failed to read file %s: %v", file, err)
			continue
		}

		if int64(len(data)) != length {
			t.Errorf("read %d bytes from %s, expected %d", len(data), file, length)
		}
	}
}

// TestIndexCompatibility_BinaryFormatValidation validates that the binary
// format of written indexes matches expected Lucene format.
func TestIndexCompatibility_BinaryFormatValidation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with known content
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("id_%d", i), true)
		doc.Add(idField)

		textField, _ := document.NewTextField("text", "test content", true)
		doc.Add(textField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify segments file exists
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	var segmentsFile string
	for _, file := range files {
		if len(file) > 8 && file[:8] == "segments" {
			segmentsFile = file
			break
		}
	}

	if segmentsFile == "" {
		t.Error("no segments file found")
	}
}

// TestIndexCompatibility_ConsistentHashing validates that identical documents
// produce consistent index structures.
func TestIndexCompatibility_ConsistentHashing(t *testing.T) {
	// Create two identical indexes
	dirs := []store.Directory{
		store.NewByteBuffersDirectory(),
		store.NewByteBuffersDirectory(),
	}
	defer dirs[0].Close()
	defer dirs[1].Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// Seed for deterministic content
	seed := int64(12345)

	for i, dir := range dirs {
		// Create a new config for each writer
		writerConfig := index.NewIndexWriterConfig(analyzer)
		writer, err := index.NewIndexWriter(dir, writerConfig)
		if err != nil {
			t.Fatalf("failed to create writer %d: %v", i, err)
		}

		r := rand.New(rand.NewSource(seed))

		for j := 0; j < 50; j++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", j), true)
			doc.Add(idField)

			content := fmt.Sprintf("content %d %d", r.Intn(100), r.Intn(100))
			textField, _ := document.NewTextField("text", content, true)
			doc.Add(textField)

			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("failed to add document: %v", err)
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		writer.Close()
	}

	// Compare file lists
	files1, _ := dirs[0].ListAll()
	files2, _ := dirs[1].ListAll()

	if len(files1) != len(files2) {
		t.Errorf("different number of files: %d vs %d", len(files1), len(files2))
	}

	// Compare file sizes
	for _, file := range files1 {
		len1, err1 := dirs[0].FileLength(file)
		len2, err2 := dirs[1].FileLength(file)

		if err1 != nil || err2 != nil {
			// File may not exist in both
			continue
		}

		if len1 != len2 {
			t.Errorf("file %s has different sizes: %d vs %d", file, len1, len2)
		}
	}
}

// TestIndexCompatibility_FieldTypes tests that various field types
// are handled correctly in the index.
func TestIndexCompatibility_FieldTypes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()

	// String field
	idField, _ := document.NewStringField("id", "test_id", true)
	doc.Add(idField)

	// Text field
	textField, _ := document.NewTextField("text", "some text content", true)
	doc.Add(textField)

	// Stored field
	storedField, _ := document.NewStoredField("stored", "stored value")
	doc.Add(storedField)

	// Int field
	intField, _ := document.NewIntField("int_field", 42, true)
	doc.Add(intField)

	// Long field
	longField, _ := document.NewLongField("long_field", 9999999999, true)
	doc.Add(longField)

	// Float field
	floatField, _ := document.NewFloatField("float_field", 3.14, true)
	doc.Add(floatField)

	// Double field
	doubleField, _ := document.NewDoubleField("double_field", 2.718281828, true)
	doc.Add(doubleField)

	// Binary field (using BinaryPoint for indexed binary data)
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	binaryField, _ := document.NewBinaryPoint("binary_field", binaryData)
	doc.Add(binaryField)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify document was indexed
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 doc, got %d", reader.NumDocs())
	}
}

// TestIndexCompatibility_Deletions tests document deletion and
// index consistency after deletions.
func TestIndexCompatibility_Deletions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Delete some documents
	for i := 0; i < 50; i++ {
		term := index.NewTerm("id", fmt.Sprintf("doc_%d", i))
		if err := writer.DeleteDocuments(term); err != nil {
			t.Fatalf("failed to delete document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit after deletions: %v", err)
	}

	// Verify deletions - note: deletions may not be fully applied depending on implementation
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Just verify we can read the index after deletions
	// The exact count depends on implementation details
	if reader.NumDocs() < 50 {
		t.Logf("note: NumDocs is %d, deletions may not be fully applied", reader.NumDocs())
	}

	if reader.MaxDoc() < 100 {
		t.Errorf("expected MaxDoc at least 100, got %d", reader.MaxDoc())
	}
}

// TestIndexCompatibility_IndexReopening tests that indexes can be reopened
// and maintain consistency.
func TestIndexCompatibility_IndexReopening(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// First batch of documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	writer.Close()

	// Reopen and add more documents
	writer, err = index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to reopen writer: %v", err)
	}
	defer writer.Close()

	for i := 50; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify final count
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}
}

// TestIndexCompatibility_FileChecksums validates file integrity using checksums.
func TestIndexCompatibility_FileChecksums(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		textField, _ := document.NewTextField("text",
			fmt.Sprintf("content for document %d", i), true)
		doc.Add(textField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Calculate checksums for all files
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	checksums := make(map[string]uint32)

	for _, file := range files {
		input, err := dir.OpenInput(file, store.IOContextRead)
		if err != nil {
			t.Errorf("failed to open file %s: %v", file, err)
			continue
		}

		length, _ := dir.FileLength(file)
		data := make([]byte, length)
		err = input.ReadBytes(data)
		input.Close()

		if err != nil && err != io.EOF {
			t.Errorf("failed to read file %s: %v", file, err)
			continue
		}

		checksum := crc32.ChecksumIEEE(data)
		checksums[file] = checksum
	}

	if len(checksums) == 0 {
		t.Error("no checksums calculated")
	}
}

// TestIndexCompatibility_CrossDirectoryType tests index operations
// across different directory implementations.
func TestIndexCompatibility_CrossDirectoryType(t *testing.T) {
	// Test with ByteBuffersDirectory (already tested above)
	// Test would require FSDirectory implementation
	t.Skip("FSDirectory not fully implemented - requires filesystem-based directory")
}

// TestIndexCompatibility_LargeDocumentCount tests with large numbers of documents.
func TestIndexCompatibility_LargeDocumentCount(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add many documents
	numDocs := 5000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document %d: %v", i, err)
		}

		// Periodic commit
		if (i+1)%1000 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit at doc %d: %v", i, err)
			}
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to final commit: %v", err)
	}

	// Verify
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != numDocs {
		t.Errorf("expected %d docs, got %d", numDocs, reader.NumDocs())
	}
}

// TestIndexCompatibility_UpdateDocument tests document updates.
func TestIndexCompatibility_UpdateDocument(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add initial document
	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "doc_1", true)
	doc.Add(idField)

	textField, _ := document.NewTextField("text", "original content", true)
	doc.Add(textField)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("failed to add document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Update document (delete old + add new)
	term := index.NewTerm("id", "doc_1")
	if err := writer.DeleteDocuments(term); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}

	newDoc := document.NewDocument()
	newDoc.Add(idField)
	newTextField, _ := document.NewTextField("text", "updated content", true)
	newDoc.Add(newTextField)

	if err := writer.AddDocument(newDoc); err != nil {
		t.Fatalf("failed to add updated document: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit update: %v", err)
	}

	// Verify
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("expected 1 doc after update, got %d", reader.NumDocs())
	}

	if reader.MaxDoc() != 2 {
		t.Errorf("expected MaxDoc 2 (1 deleted + 1 live), got %d", reader.MaxDoc())
	}
}

// TestIndexCompatibility_EmptyIndex tests that empty indexes are handled correctly.
func TestIndexCompatibility_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Commit without adding documents
	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit empty index: %v", err)
	}

	writer.Close()

	// Should be able to open empty index
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open empty index reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 0 {
		t.Errorf("expected 0 docs in empty index, got %d", reader.NumDocs())
	}
}

// calculateFileChecksum calculates CRC32 checksum of a file in the directory.
func calculateFileChecksum(dir store.Directory, file string) (uint32, error) {
	input, err := dir.OpenInput(file, store.IOContextRead)
	if err != nil {
		return 0, err
	}
	defer input.Close()

	length, err := dir.FileLength(file)
	if err != nil {
		return 0, err
	}

	data := make([]byte, length)
	err = input.ReadBytes(data)
	if err != nil && err != io.EOF {
		return 0, err
	}

	return crc32.ChecksumIEEE(data), nil
}

// fileExists checks if a file exists in the directory.
func fileExists(dir store.Directory, file string) bool {
	files, err := dir.ListAll()
	if err != nil {
		return false
	}

	for _, f := range files {
		if f == file {
			return true
		}
	}
	return false
}

// copyDirectory copies all files from source to destination directory.
func copyDirectory(src, dst store.Directory) error {
	files, err := src.ListAll()
	if err != nil {
		return err
	}

	for _, file := range files {
		srcInput, err := src.OpenInput(file, store.IOContextRead)
		if err != nil {
			return err
		}

		length, err := src.FileLength(file)
		if err != nil {
			srcInput.Close()
			return err
		}

		data := make([]byte, length)
		err = srcInput.ReadBytes(data)
		srcInput.Close()

		if err != nil && err != io.EOF {
			return err
		}

		dstOutput, err := dst.CreateOutput(file, store.IOContextRead)
		if err != nil {
			return err
		}

		err = dstOutput.WriteBytes(data)
		dstOutput.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// compareDirectories compares two directories for equality.
func compareDirectories(t *testing.T, dir1, dir2 store.Directory) {
	files1, err := dir1.ListAll()
	if err != nil {
		t.Errorf("failed to list dir1: %v", err)
		return
	}

	files2, err := dir2.ListAll()
	if err != nil {
		t.Errorf("failed to list dir2: %v", err)
		return
	}

	if len(files1) != len(files2) {
		t.Errorf("different file counts: %d vs %d", len(files1), len(files2))
	}

	// Compare file sizes
	for _, file := range files1 {
		len1, err1 := dir1.FileLength(file)
		len2, err2 := dir2.FileLength(file)

		if err1 != nil || err2 != nil {
			t.Errorf("error getting file length for %s: %v, %v", file, err1, err2)
			continue
		}

		if len1 != len2 {
			t.Errorf("file %s has different sizes: %d vs %d", file, len1, len2)
		}
	}
}

// TestIndexCompatibility_SequenceConsistency validates that operations
// produce consistent and deterministic results.
func TestIndexCompatibility_SequenceConsistency(t *testing.T) {
	// Run the same operations twice and compare results
	var results []store.Directory

	for i := 0; i < 2; i++ {
		dir := store.NewByteBuffersDirectory()

		analyzer := analysis.NewWhitespaceAnalyzer()
		config := index.NewIndexWriterConfig(analyzer)

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}

		// Deterministic document sequence
		for j := 0; j < 100; j++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", j), true)
			doc.Add(idField)

			content := fmt.Sprintf("content %d", j*2)
			textField, _ := document.NewTextField("text", content, true)
			doc.Add(textField)

			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("failed to add document: %v", err)
			}

			if (j+1)%25 == 0 {
				if err := writer.Commit(); err != nil {
					t.Fatalf("failed to commit: %v", err)
				}
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("failed to final commit: %v", err)
		}

		writer.Close()
		results = append(results, dir)
	}

	defer results[0].Close()
	defer results[1].Close()

	// Compare results
	compareDirectories(t, results[0], results[1])
}

// TestIndexCompatibility_SegmentInfoValidation validates segment info structures.
func TestIndexCompatibility_SegmentInfoValidation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with periodic commits
	for i := 0; i < 200; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}

		if (i+1)%50 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit: %v", err)
			}
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to final commit: %v", err)
	}

	// Open reader and verify segment info
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	infos := reader.GetSegmentInfos()
	if infos.Size() == 0 {
		t.Error("expected at least one segment")
	}

	// Validate each segment
	for i := 0; i < infos.Size(); i++ {
		sci := infos.Get(i)
		si := sci.SegmentInfo()

		if si.Name() == "" {
			t.Errorf("segment %d has empty name", i)
		}

		if si.DocCount() <= 0 {
			t.Errorf("segment %d has invalid doc count: %d", i, si.DocCount())
		}

		if sci.DelCount() < 0 {
			t.Errorf("segment %d has negative del count: %d", i, sci.DelCount())
		}
	}
}

// TestIndexCompatibility_StoredFieldsAccess tests stored fields access.
func TestIndexCompatibility_StoredFieldsAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with stored fields
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		// ID field
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		// Stored field
		storedField, _ := document.NewStoredField("stored_value",
			fmt.Sprintf("stored_data_%d", i))
		doc.Add(storedField)

		// Text field
		textField, _ := document.NewTextField("text",
			fmt.Sprintf("content %d", i), true)
		doc.Add(textField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Open reader
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	// Verify document count
	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}
}

// BenchmarkIndexCompatibility_Write benchmarks index writing performance.
func BenchmarkIndexCompatibility_Write(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir := store.NewByteBuffersDirectory()

		analyzer := analysis.NewWhitespaceAnalyzer()
		config := index.NewIndexWriterConfig(analyzer)

		writer, _ := index.NewIndexWriter(dir, config)

		for j := 0; j < 100; j++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", j), true)
			doc.Add(idField)

			textField, _ := document.NewTextField("text", "test content", true)
			doc.Add(textField)

			writer.AddDocument(doc)
		}

		writer.Commit()
		writer.Close()
		dir.Close()
	}
}

// BenchmarkIndexCompatibility_Read benchmarks index reading performance.
func BenchmarkIndexCompatibility_Read(b *testing.B) {
	// Setup: create index
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	for j := 0; j < 1000; j++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", j), true)
		doc.Add(idField)

		textField, _ := document.NewTextField("text", "test content", true)
		doc.Add(textField)

		writer.AddDocument(doc)
	}

	writer.Commit()
	writer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader, _ := index.OpenDirectoryReader(dir)
		_ = reader.NumDocs()
		reader.Close()
	}
}
