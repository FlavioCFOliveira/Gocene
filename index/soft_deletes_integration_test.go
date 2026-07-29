// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func newSoftDeletesWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetSoftDeletesField("soft_delete")
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return writer
}

// TestSoftDeletesIntegration_BasicSoftDelete verifies that SoftUpdateDocument
// replaces a document while keeping the original in MaxDoc but not in NumDocs.
func TestSoftDeletesIntegration_BasicSoftDelete(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newSoftDeletesWriter(t, dir)
	defer writer.Close()

	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "1", true)
	contentField, _ := document.NewTextField("content", "original", true)
	doc.Add(idField)
	doc.Add(contentField)
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Soft-update: add replacement doc with soft-delete field.
	replacement := document.NewDocument()
	replacement.Add(idField)
	replacement.Add(contentField)
	sdField, _ := document.NewNumericDocValuesField("soft_delete", 1)
	replacement.Add(sdField)
	if _, err := writer.SoftUpdateDocument(index.NewTerm("id", "1"), replacement); err != nil {
		t.Fatalf("SoftUpdateDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.MaxDoc() != 2 {
		t.Fatalf("MaxDoc = %d, want 2", reader.MaxDoc())
	}
	if reader.NumDocs() != 1 {
		t.Fatalf("NumDocs = %d, want 1", reader.NumDocs())
	}
}

// TestSoftDeletesIntegration_Purging verifies that a merge purges soft-deleted
// documents so that MaxDoc equals NumDocs afterwards.
//
// Blocker: merge purge of soft-deleted documents is not yet implemented.
func TestSoftDeletesIntegration_Purging(t *testing.T) {
	t.Fatal("needs merge purge of soft-deleted documents (not yet implemented)")
}
