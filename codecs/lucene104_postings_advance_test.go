// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/
//         Lucene104PostingsReader.java (Lucene 10.4.0)
//
// Regression coverage for rmp #4763: PostingsEnum.Advance after a prior
// NextDoc/Advance within an already-decoded block must still find a later
// doc that lives in the same buffer.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// openPostings writes a single field with the supplied (term -> docIDs)
// postings and returns a PostingsEnum positioned before the first doc.
func openPostings(t *testing.T, opts index.IndexOptions, field, text string, docIDs []int) index.PostingsEnum {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	format := NewLucene104PostingsFormat()
	fis := rtFieldInfos(t, struct {
		name string
		opts index.IndexOptions
	}{name: field, opts: opts})
	maxDoc := 1
	for _, d := range docIDs {
		if d+1 > maxDoc {
			maxDoc = d + 1
		}
	}
	ws := rtWriteState(dir, "_0", fis, maxDoc)

	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	seeds := make([]SeedPosting, len(docIDs))
	for i, d := range docIDs {
		seeds[i] = SeedPosting{docID: d, freq: 1}
	}
	st := &SeedTerms{
		field:      field,
		terms:      []*index.Term{index.NewTerm(field, text)},
		termToDocs: map[string][]SeedPosting{text: seeds},
		options:    opts,
	}
	if err := consumer.Write(field, st); err != nil {
		_ = consumer.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	producer, err := format.FieldsProducer(rtReadState(ws))
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close() })

	terms, err := producer.Terms(field)
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	found, err := te.SeekExact(index.NewTerm(field, text))
	if err != nil || !found {
		t.Fatalf("SeekExact(%q): found=%v err=%v", text, found, err)
	}
	pe, err := te.Postings(index.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	return pe
}

// TestLucene104Postings_AdvanceAfterPosition_SingleBlock reproduces rmp #4763.
//
// A term at docIDs {1,6} fits in a single (remainder) block.  Mixing
// NextDoc/Advance with Advance must keep finding later in-buffer docs:
//
//	fresh   Advance(2)            -> 6
//	        Advance(0)->1 then Advance(4) -> 6   (NOT NO_MORE_DOCS)
//	        NextDoc()->1 then Advance(2)  -> 6   (NOT NO_MORE_DOCS)
func TestLucene104Postings_AdvanceAfterPosition_SingleBlock(t *testing.T) {
	const (
		field = "f"
		text  = "java"
	)
	docIDs := []int{1, 6}

	t.Run("fresh_advance", func(t *testing.T) {
		pe := openPostings(t, index.IndexOptionsDocsAndFreqs, field, text, docIDs)
		d, err := pe.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if d != 6 {
			t.Fatalf("fresh Advance(2): got %d, want 6", d)
		}
	})

	t.Run("advance_then_advance", func(t *testing.T) {
		pe := openPostings(t, index.IndexOptionsDocsAndFreqs, field, text, docIDs)
		if d, err := pe.Advance(0); err != nil || d != 1 {
			t.Fatalf("Advance(0): got %d err=%v, want 1", d, err)
		}
		d, err := pe.Advance(4)
		if err != nil {
			t.Fatalf("Advance(4): %v", err)
		}
		if d != 6 {
			t.Fatalf("Advance(0)->1 then Advance(4): got %d, want 6", d)
		}
	})

	t.Run("nextdoc_then_advance", func(t *testing.T) {
		pe := openPostings(t, index.IndexOptionsDocsAndFreqs, field, text, docIDs)
		if d, err := pe.NextDoc(); err != nil || d != 1 {
			t.Fatalf("NextDoc: got %d err=%v, want 1", d, err)
		}
		d, err := pe.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if d != 6 {
			t.Fatalf("NextDoc()->1 then Advance(2): got %d, want 6", d)
		}
	})
}
