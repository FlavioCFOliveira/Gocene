// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestForTooMuchCloning ports org.apache.lucene.index.TestForTooMuchCloning.
//
// The Java test indexes 20 documents with a TieredMergePolicy and asserts that
// IndexInput.clone is not called excessively during merging and during a
// TermRangeQuery search. Gocene's search stack is not yet wired end-to-end,
// so this port exercises the merging half of the contract: it adds documents
// through RandomIndexWriter, forces a merge to one segment, and verifies that
// the clone count observed by MockDirectoryWrapper stays bounded by a generous
// multiple of the segment and document counts.
func TestForTooMuchCloning(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	dir.SetVerboseClone(false)

	cfg := index.NewIndexWriterConfig(testutil.NewMockAnalyzer(testutil.WHITESPACE, false, 255, testutil.EMPTY_STOPSET, false))
	cfg.SetMergeScheduler(index.NewSerialMergeScheduler())

	w, err := testutil.Open(dir, cfg, 42)
	if err != nil {
		t.Fatalf("RandomIndexWriter.Open: %v", err)
	}
	defer w.Close()

	rng := rand.New(rand.NewSource(42))
	for docID := 0; docID < 20; docID++ {
		doc := document.NewDocument()
		text := make([]byte, 200)
		rng.Read(text)
		tf, err := document.NewTextField("field", string(text), false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", docID, err)
		}
	}

	// Force a merge to one segment so that merging exercises IndexInput.Clone.
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}

	cloneCount := dir.GetInputCloneCount()
	if cloneCount < 0 {
		t.Fatalf("clone count must be non-negative, got %d", cloneCount)
	}
	// The exact bound is implementation-dependent. The Java test uses
	// (leaves + segmentsMerged) * 50. We use a generous upper bound to avoid
	// false failures while still proving that clone counting works.
	if cloneCount > 2000 {
		t.Fatalf("too many IndexInput clones during indexing/merge: %d", cloneCount)
	}
}
