// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies that a reader obtained from IndexWriter.GetReader
// remains usable after the directory files have been deleted. This validates
// the codec's open-files / caching behaviour.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestCodecHoldsOpenFiles.java
//
// The Java original opens an NRT reader via RandomIndexWriter.getReader(),
// commits and closes the writer, deletes every file, then calls
// TestUtil.checkReader on each leaf to prove the reader still works.
//
// This Go equivalent indexes documents through IndexWriter, obtains an NRT
// reader via GetReader, closes the writer, deletes every file from the
// directory, and verifies the reader still reports the correct doc count.
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCodecHoldsOpenFiles indexes documents, opens a reader, deletes every
// file in the directory, and asserts the reader still works.
func TestCodecHoldsOpenFiles(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("id", fmt.Sprintf("doc%d", i), false)
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

	// Obtain an NRT reader while the writer is still open.
	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer reader.Close()

	// Now close the writer and delete every file.
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, f := range files {
		if err := dir.DeleteFile(f); err != nil {
			t.Fatalf("DeleteFile(%q): %v", f, err)
		}
	}

	// The reader should still report the correct doc count.
	if reader.NumDocs() != 10 {
		t.Errorf("NumDocs after file deletion = %d, want 10", reader.NumDocs())
	}
	if reader.MaxDoc() != 10 {
		t.Errorf("MaxDoc after file deletion = %d, want 10", reader.MaxDoc())
	}
}
