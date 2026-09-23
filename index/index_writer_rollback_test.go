// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestRollback.java
// (Apache Lucene 10.5.0). The Java class uses the test-framework
// RandomIndexWriter (tests/index imports index), so the port lives in the
// external index_test package.

package index_test

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// TestRollbackIntegrityWithBufferFlush ports testRollbackIntegrityWithBufferFlush
// (LUCENE-2536).
func TestRollbackIntegrityWithBufferFlush(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	rw, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("pk", strconv.Itoa(i), true)
		if err != nil {
			t.Fatalf("newStringField: %v", err)
		}
		doc.Add(f)
		if _, err := rw.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// If buffer size is small enough to cause a flush, errors ensue...
	iwc := index.NewIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
	iwc.SetMaxBufferedDocs(2)
	iwc.SetOpenMode(index.Append)
	w, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		value := strconv.Itoa(i)
		pk, err := document.NewStringField("pk", value, true)
		if err != nil {
			t.Fatalf("newStringField: %v", err)
		}
		doc.Add(pk)
		text, err := document.NewStringField("text", "foo", true)
		if err != nil {
			t.Fatalf("newStringField: %v", err)
		}
		doc.Add(text)
		if _, err := w.UpdateDocument(index.NewTerm("pk", value), doc); err != nil {
			t.Fatalf("updateDocument: %v", err)
		}
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	if r.NumDocs() != 5 {
		t.Fatalf("index should contain same number of docs post rollback: expected 5, got %d", r.NumDocs())
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}
