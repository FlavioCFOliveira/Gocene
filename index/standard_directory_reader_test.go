// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// AC #1 for T4684 (Blocker A): OpenStandardDirectoryReader must populate
// SegmentCoreReaders so GetTermVectors returns a real TermVectorsReader on an
// on-disk index. The earlier revision called NewSegmentReader directly, which
// left coreReaders == nil and caused "core readers are nil" failures on any
// read-back through the standard composite reader.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestStandardDirectoryReader_TermVectorsRoundTrip writes one document with a
// term-vector-enabled field, commits, reopens through OpenStandardDirectoryReader,
// and verifies that GetTermVectors returns a non-nil Fields containing the
// expected terms. This is the canonical exercise of the
// OpenStandardDirectoryReader → openSegmentReader → NewSegmentReaderWithCore path.
func TestStandardDirectoryReader_TermVectorsRoundTrip(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetUseCompoundFile(false)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	titleField, err := tvTextField("title", "hello world")
	if err != nil {
		t.Fatalf("tvTextField(title): %v", err)
	}
	bodyField, err := tvTextField("body", "foo bar baz")
	if err != nil {
		t.Fatalf("tvTextField(body): %v", err)
	}
	doc.Add(titleField)
	doc.Add(bodyField)

	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Open through the StandardDirectoryReader path explicitly.
	sdr, err := index.OpenStandardDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenStandardDirectoryReader: %v", err)
	}
	defer sdr.Close()

	// AC #1: GetTermVectors must return a real Fields, not "core readers are nil".
	tvFields, err := sdr.GetTermVectors(0)
	if err != nil {
		t.Fatalf("GetTermVectors(0): %v", err)
	}
	if tvFields == nil {
		t.Fatalf("GetTermVectors(0): returned nil; SegmentCoreReaders were not populated through OpenStandardDirectoryReader")
	}

	checkFieldTerms(t, tvFields, "title", []string{"hello", "world"})
	checkFieldTerms(t, tvFields, "body", []string{"bar", "baz", "foo"})
}
