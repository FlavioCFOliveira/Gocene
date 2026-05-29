// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestBlockTreeHighCardinality_DeleteByIDReopen is acceptance criterion (4) of
// rmp #4754 (the #4753 follow-through): deleting a high-cardinality id term that
// lives in a sub-block, committing, and reopening must reduce the reader's
// NumDocs by one and make the deleted term no longer match. Before the
// multi-block reader landed, the commit-time delete resolution could not find
// the term (SeekExact missed sub-block terms), so the deletion was silently
// dropped and NumDocs stayed unchanged.
func TestBlockTreeHighCardinality_DeleteByIDReopen(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	indexHighCardinalityIDs(t, dir)

	// Baseline: the freshly committed index has every doc live and every term
	// resolvable.
	sdr0, terms0 := openHighCardinalityTerms(t, dir)
	if got := sdr0.NumDocs(); got != highCardinalityTermCount {
		_ = sdr0.Close()
		t.Fatalf("baseline NumDocs = %d, want %d", got, highCardinalityTermCount)
	}
	en0, err := terms0.GetIterator()
	if err != nil {
		_ = sdr0.Close()
		t.Fatalf("GetIterator (baseline): %v", err)
	}
	const victim = "id_0500" // mid-tree, lives in a sub-block
	if found, err := en0.SeekExact(index.NewTerm("id", victim)); err != nil || !found {
		_ = sdr0.Close()
		t.Fatalf("baseline SeekExact(%q) = (%v, %v), want (true, nil)", victim, found, err)
	}
	_ = sdr0.Close()

	// Reopen the writer in APPEND mode and delete the victim term.
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetOpenMode(index.APPEND)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("reopen IndexWriter: %v", err)
	}
	if err := writer.DeleteDocuments(index.NewTerm("id", victim)); err != nil {
		_ = writer.Close()
		t.Fatalf("DeleteDocuments(%q): %v", victim, err)
	}
	if err := writer.Commit(); err != nil {
		_ = writer.Close()
		t.Fatalf("Commit after delete: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close after delete: %v", err)
	}

	// Reopen the reader: NumDocs must have dropped by exactly one and the
	// victim term's postings must no longer reach a live document.
	sdr1, terms1 := openHighCardinalityTerms(t, dir)
	defer sdr1.Close()

	if got, want := sdr1.NumDocs(), highCardinalityTermCount-1; got != want {
		t.Fatalf("post-delete NumDocs = %d, want %d", got, want)
	}

	// The victim's docID must be cleared in live docs. We confirm this by
	// resolving the term, fetching its single posting, and checking that doc
	// against the segment's live-docs view.
	en1, err := terms1.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator (post-delete): %v", err)
	}
	found, err := en1.SeekExact(index.NewTerm("id", victim))
	if err != nil {
		t.Fatalf("post-delete SeekExact(%q): %v", victim, err)
	}
	// The term itself still exists in the dictionary (the .tim is unchanged by
	// a delete; only live-docs are updated), so SeekExact still returns true.
	if !found {
		t.Fatalf("post-delete SeekExact(%q) = false; the term should remain in the dictionary", victim)
	}
	pe, err := en1.Postings(0)
	if err != nil {
		t.Fatalf("post-delete Postings(%q): %v", victim, err)
	}
	deletedDoc, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("post-delete NextDoc(%q): %v", victim, err)
	}
	if deletedDoc == index.NO_MORE_DOCS {
		t.Fatalf("post-delete Postings(%q) yielded no documents", victim)
	}

	// The deleted doc must be marked not-live in the single segment's live docs.
	srs := sdr1.GetSegmentReaders()
	if len(srs) != 1 {
		t.Fatalf("expected 1 segment reader, got %d", len(srs))
	}
	live := srs[0].GetLiveDocs()
	if live == nil {
		t.Fatal("post-delete segment has nil live docs; deletion was not applied")
	}
	if live.Get(deletedDoc) {
		t.Fatalf("deleted doc %d (term %q) is still marked live", deletedDoc, victim)
	}

	// Every other id must still resolve to a live document.
	checkEnum, err := terms1.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator (survivors): %v", err)
	}
	survivors := 0
	for i := 0; i < highCardinalityTermCount; i++ {
		id := highCardinalityID(i)
		if id == victim {
			continue
		}
		found, err := checkEnum.SeekExact(index.NewTerm("id", id))
		if err != nil {
			t.Fatalf("survivor SeekExact(%q): %v", id, err)
		}
		if !found {
			t.Fatalf("survivor SeekExact(%q) = false", id)
		}
		pe, err := checkEnum.Postings(0)
		if err != nil {
			t.Fatalf("survivor Postings(%q): %v", id, err)
		}
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("survivor NextDoc(%q): %v", id, err)
		}
		if doc == index.NO_MORE_DOCS || !live.Get(doc) {
			t.Fatalf("survivor %q maps to doc %d which is not live", id, doc)
		}
		survivors++
	}
	if survivors != highCardinalityTermCount-1 {
		t.Fatalf("verified %d survivors, want %d", survivors, highCardinalityTermCount-1)
	}
}
