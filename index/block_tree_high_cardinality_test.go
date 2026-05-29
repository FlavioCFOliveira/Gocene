// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// highCardinalityTermCount is the number of unique terms the round-trip uses.
// With the block-tree writer's default minItemsInBlock=25 / maxItemsInBlock=48,
// 1000 sorted terms force the .tim dictionary to spill into many sub-blocks and
// floor blocks — exactly the structure the Next()-only reader stub used to skip
// (backlog #2692, fixed under rmp #4754). The previous reader returned 0 hits
// for any SeekExact that landed below the root block.
const highCardinalityTermCount = 1000

// highCardinalityID returns the zero-padded id for document i, so byte order
// matches integer order (id_0000 < id_0001 < ... < id_0999).
func highCardinalityID(i int) string {
	return fmt.Sprintf("id_%04d", i)
}

// indexHighCardinalityIDs writes highCardinalityTermCount documents, each with a
// unique "id" StringField id_0000..id_0999 plus a shared indexed "body" field.
// It commits once so the whole field lands in a single segment, then closes the
// writer.
func indexHighCardinalityIDs(t *testing.T, dir store.Directory) {
	t.Helper()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Disable auto-flush so every doc lands in one segment on Commit.
	config.SetMaxBufferedDocs(highCardinalityTermCount + 10)
	config.SetRAMBufferSizeMB(index.DISABLE_AUTO_FLUSH)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < highCardinalityTermCount; i++ {
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", highCardinalityID(i), false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(idField)
		bodyField, err := document.NewTextField("body", "shared body text", false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(bodyField)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// openHighCardinalityTerms opens the committed index through
// OpenStandardDirectoryReader — the reader path that wires SegmentCoreReaders
// (and therefore reaches the real block-tree TermsEnum) — and returns the Terms
// for "id" together with the reader so the caller can close it.
func openHighCardinalityTerms(t *testing.T, dir store.Directory) (*index.StandardDirectoryReader, index.Terms) {
	t.Helper()
	sdr, err := index.OpenStandardDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenStandardDirectoryReader: %v", err)
	}
	terms, err := sdr.Terms("id")
	if err != nil {
		_ = sdr.Close()
		t.Fatalf("Terms(id): %v", err)
	}
	if terms == nil {
		_ = sdr.Close()
		t.Fatal("Terms(id) returned nil; core readers were not wired")
	}
	return sdr, terms
}

// TestBlockTreeHighCardinality_SeekExactEveryTerm is acceptance criterion (1)
// of rmp #4754: every one of the 1000 unique terms — most of which live in
// sub-blocks below the root — must be found by SeekExact, return DocFreq==1, and
// a single-doc Postings; a known-absent term must return false; and a full
// Next() walk must enumerate every term in sorted order exactly once.
func TestBlockTreeHighCardinality_SeekExactEveryTerm(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	indexHighCardinalityIDs(t, dir)

	sdr, terms := openHighCardinalityTerms(t, dir)
	defer sdr.Close()

	// (a) Full Next() walk: every term, in sorted order, exactly once.
	walkEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator (walk): %v", err)
	}
	var prev []byte
	seen := 0
	for {
		term, err := walkEnum.Next()
		if err != nil {
			t.Fatalf("Next() at position %d: %v", seen, err)
		}
		if term == nil {
			break
		}
		ref := term.BytesValue()
		got := append([]byte(nil), ref.Bytes[ref.Offset:ref.Offset+ref.Length]...)
		if prev != nil && bytes.Compare(prev, got) >= 0 {
			t.Fatalf("Next() out of order at position %d: %q after %q", seen, got, prev)
		}
		if want := highCardinalityID(seen); string(got) != want {
			t.Fatalf("Next() position %d: got %q, want %q", seen, got, want)
		}
		prev = got
		seen++
	}
	if seen != highCardinalityTermCount {
		t.Fatalf("Next() walk enumerated %d terms, want %d", seen, highCardinalityTermCount)
	}

	// (b) SeekExact every term; assert DocFreq==1 and a single matching doc.
	// Collect the docID per id to prove the mapping is a bijection onto
	// {0..N-1}.
	seekEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator (seek): %v", err)
	}
	docForID := make(map[string]int, highCardinalityTermCount)
	for i := 0; i < highCardinalityTermCount; i++ {
		id := highCardinalityID(i)
		found, err := seekEnum.SeekExact(index.NewTerm("id", id))
		if err != nil {
			t.Fatalf("SeekExact(%q): %v", id, err)
		}
		if !found {
			t.Fatalf("SeekExact(%q) returned not-found; a sub-block term was missed", id)
		}
		df, err := seekEnum.DocFreq()
		if err != nil {
			t.Fatalf("DocFreq(%q): %v", id, err)
		}
		if df != 1 {
			t.Fatalf("DocFreq(%q) = %d, want 1", id, df)
		}
		pe, err := seekEnum.Postings(0)
		if err != nil {
			t.Fatalf("Postings(%q): %v", id, err)
		}
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc(%q): %v", id, err)
		}
		if doc == index.NO_MORE_DOCS {
			t.Fatalf("Postings(%q) yielded no documents", id)
		}
		docForID[id] = doc
		if next, err := pe.NextDoc(); err != nil {
			t.Fatalf("second NextDoc(%q): %v", id, err)
		} else if next != index.NO_MORE_DOCS {
			t.Fatalf("Postings(%q) yielded more than one document (%d then %d)", id, doc, next)
		}
	}

	if len(docForID) != highCardinalityTermCount {
		t.Fatalf("collected %d id->doc pairs, want %d", len(docForID), highCardinalityTermCount)
	}
	usedDocs := make(map[int]struct{}, highCardinalityTermCount)
	for id, doc := range docForID {
		if doc < 0 || doc >= highCardinalityTermCount {
			t.Fatalf("id %q maps to out-of-range doc %d", id, doc)
		}
		if _, dup := usedDocs[doc]; dup {
			t.Fatalf("doc %d is referenced by more than one id (last %q)", doc, id)
		}
		usedDocs[doc] = struct{}{}
	}

	// (c) Known-absent terms must report not-found.
	absent := []string{"id_9999", "aaaaa", "zzzzz", "id_00005_extra", "i"}
	for _, a := range absent {
		ae, err := terms.GetIterator()
		if err != nil {
			t.Fatalf("GetIterator (absent %q): %v", a, err)
		}
		found, err := ae.SeekExact(index.NewTerm("id", a))
		if err != nil {
			t.Fatalf("SeekExact(absent %q): %v", a, err)
		}
		if found {
			t.Fatalf("SeekExact(%q) reported found for a term that was never indexed", a)
		}
	}
}

// TestBlockTreeHighCardinality_SeekCeil exercises SeekCeil on the same
// multi-block dictionary: an exact hit deep in the tree, a NOT_FOUND ceiling
// that lands on the next greater term, and an END past the last term.
func TestBlockTreeHighCardinality_SeekCeil(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	indexHighCardinalityIDs(t, dir)

	sdr, terms := openHighCardinalityTerms(t, dir)
	defer sdr.Close()

	// Exact hit deep in the dictionary.
	enum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	got, err := enum.SeekCeil(index.NewTerm("id", "id_0500"))
	if err != nil {
		t.Fatalf("SeekCeil(id_0500): %v", err)
	}
	if got == nil || got.Text() != "id_0500" {
		t.Fatalf("SeekCeil(id_0500) landed on %v, want id_0500", got)
	}

	// NOT_FOUND ceiling: "id_0500a" sits between id_0500 and id_0501, so the
	// ceiling must be id_0501.
	enum2, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	got2, err := enum2.SeekCeil(index.NewTerm("id", "id_0500a"))
	if err != nil {
		t.Fatalf("SeekCeil(id_0500a): %v", err)
	}
	if got2 == nil || got2.Text() != "id_0501" {
		t.Fatalf("SeekCeil(id_0500a) landed on %v, want id_0501", got2)
	}

	// END: a term greater than every id returns nil.
	enum3, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	got3, err := enum3.SeekCeil(index.NewTerm("id", "id_9999"))
	if err != nil {
		t.Fatalf("SeekCeil(id_9999): %v", err)
	}
	if got3 != nil {
		t.Fatalf("SeekCeil(id_9999) landed on %v, want END (nil)", got3)
	}
}
