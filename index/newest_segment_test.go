// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestNewestSegment.
//
// GOC-4191: Index Tests - TestNewestSegment
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNewestSegment verifies that a freshly created IndexWriter, before any
// flush, reports no newest segment.
func TestNewestSegment(t *testing.T) {
	t.Fatal("Sprint 55 option c: needs IndexWriter.newestSegment (not yet ported)")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createTestAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	// IndexWriter.newestSegment() must be nil before the first flush.
	// if seg := writer.NewestSegment(); seg != nil {
	// 	t.Errorf("NewestSegment() = %v, want nil", seg)
	// }

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
