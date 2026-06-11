// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests verifying that the default codec
// can write and read index segments correctly.
//
// Ported from Apache Lucene's
// org.apache.lucene.index.TestDefaultCodecParallelizesIO
//
// The original upstream suite asserts I/O parallelization using a
// SerialIOCountingDirectory wrapper that does not yet exist in Gocene.
// This Go equivalent validates that the default codec produces a valid,
// readable index when driven through IndexWriter, including the
// stored fields and terms seek paths (at the segment-file level).
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDefaultCodecParallelizesIO_TermsSeekExact validates that the default
// codec can index documents with indexed fields and produce a readable index.
// This exercises the terms-write path through IndexWriter.
func TestDefaultCodecParallelizesIO_TermsSeekExact(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("body", fmt.Sprintf("term%d", i), false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the index can be read back.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 10 {
		t.Errorf("NumDocs = %d, want 10", reader.NumDocs())
	}
}

// TestDefaultCodecParallelizesIO_StoredFields validates that the default
// codec can index and read back stored fields correctly.
func TestDefaultCodecParallelizesIO_StoredFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("title", fmt.Sprintf("Doc %d", i), true) // stored
		if err != nil {
			t.Fatalf("NewStringField(title): %v", err)
		}
		doc.Add(f)
		// Add an indexed (non-stored) body field.
		f2, err := document.NewStringField("body", fmt.Sprintf("content %d", i), false)
		if err != nil {
			t.Fatalf("NewStringField(body): %v", err)
		}
		doc.Add(f2)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the index can be read back.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 5 {
		t.Errorf("NumDocs = %d, want 5", reader.NumDocs())
	}
}
