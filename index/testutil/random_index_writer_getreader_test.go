// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Coverage for RandomIndexWriter.GetReader (rmp #18/#127): a near-real-time
// reader opened over the in-flight writer reflects every added document,
// including ones not yet committed, so the ported search suites can obtain a
// searcher the upstream way.

package testutil_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestRandomIndexWriter_GetReader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Disable random commit/forceMerge so the test is deterministic regardless
	// of seed: GetReader must see the docs whether or not a commit happened.
	riw := testutil.NewWithConfig(w, 1, testutil.Config{
		CommitProbability:          0.0001,
		ForceMergeProbability:      0.0001,
		MaxNumSegmentsOnForceMerge: 1,
	})

	const n = 7
	for i := 0; i < n; i++ {
		doc := document.NewDocument()
		f, _ := document.NewStringField("id", string(rune('a'+i)), false)
		doc.Add(f)
		if err := riw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}

	// GetReader must reflect all added docs even without an explicit commit.
	r, err := riw.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != n {
		t.Fatalf("NumDocs = %d, want %d (uncommitted docs must be visible)", got, n)
	}

	// The reader is searchable through the standard term lookup path.
	subs := r.GetSequentialSubReaders()
	if len(subs) == 0 {
		t.Fatal("expected at least one segment reader")
	}
	terms, err := r.Terms("id")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(id) returned nil")
	}

	if err := riw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
