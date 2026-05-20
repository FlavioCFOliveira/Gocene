// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestRollback.
// Source: lucene/core/src/test/org/apache/lucene/index/TestRollback.java
//
// GOC-4189: Test rollback integrity when the document buffer is small enough
// to force a flush during the updates that are later rolled back.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// addRollbackPKDoc adds a document with a single stored "pk" string field.
func addRollbackPKDoc(t *testing.T, w *index.IndexWriter, pk string) {
	t.Helper()
	doc := document.NewDocument()
	sf, err := document.NewStringField("pk", pk, true)
	if err != nil {
		t.Fatalf("NewStringField(pk): %v", err)
	}
	doc.Add(sf)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(pk=%s): %v", pk, err)
	}
}

// TestRollbackIntegrityWithBufferFlush mirrors TestRollback.testRollbackIntegrityWithBufferFlush
// (LUCENE-2536): five committed docs, then three buffered updates with a small
// maxBufferedDocs to force a flush, followed by a rollback. The index must
// still contain exactly the five originally committed documents.
func TestRollbackIntegrityWithBufferFlush(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Seed the index with five committed documents.
	seedCfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	seed, err := index.NewIndexWriter(dir, seedCfg)
	if err != nil {
		t.Fatalf("NewIndexWriter(seed): %v", err)
	}
	for i := 0; i < 5; i++ {
		addRollbackPKDoc(t, seed, string(rune('0'+i)))
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("seed.Close: %v", err)
	}

	// Small maxBufferedDocs forces a flush during the updates below.
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetMaxBufferedDocs(2)
	cfg.SetOpenMode(index.APPEND)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter(append): %v", err)
	}

	for i := 0; i < 3; i++ {
		value := string(rune('0' + i))
		doc := document.NewDocument()
		pk, err := document.NewStringField("pk", value, true)
		if err != nil {
			t.Fatalf("NewStringField(pk): %v", err)
		}
		doc.Add(pk)
		text, err := document.NewStringField("text", "foo", true)
		if err != nil {
			t.Fatalf("NewStringField(text): %v", err)
		}
		doc.Add(text)
		if err := w.UpdateDocument(index.NewTerm("pk", value), doc); err != nil {
			t.Fatalf("UpdateDocument(pk=%s): %v", value, err)
		}
	}

	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	if got := r.NumDocs(); got != 5 {
		t.Errorf("index should contain same number of docs post rollback: got %d, want 5", got)
	}
}
