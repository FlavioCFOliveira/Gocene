// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the term-vectors leg of the segment merge
// (rmp #14/#114): two committed segments with term-vector fields are merged
// and each merged document's term vectors (terms + positions) are read back.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// tvField builds a tokenized, term-vector-with-positions field.
func tvField(t *testing.T, name, value string) *document.Field {
	t.Helper()
	ft := document.NewFieldType()
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.SetTokenized(true)
	ft.SetStored(false)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	f, err := document.NewField(name, value, ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	return f
}

// readDocTV returns field "body"'s terms with their frequency (TotalTermFreq)
// for one document. Per-occurrence positions/offsets are not asserted because
// the term-vectors TermsEnum exposes no Postings enum yet (rmp #121); the merge
// preserves terms and frequencies, which this verifies.
func readDocTV(t *testing.T, fields index.Fields) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	if fields == nil {
		return out
	}
	terms, err := fields.Terms("body")
	if err != nil || terms == nil {
		return out
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("TV GetIterator: %v", err)
	}
	for {
		term, err := te.Next()
		if err != nil {
			t.Fatalf("TV Next: %v", err)
		}
		if term == nil {
			break
		}
		ttf, err := te.TotalTermFreq()
		if err != nil {
			t.Fatalf("TV TotalTermFreq: %v", err)
		}
		out[term.Text()] = ttf
	}
	return out
}

func TestSegmentMerger_TermVectorsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(text string) {
		doc := document.NewDocument()
		doc.Add(tvField(t, "body", text))
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1. doc0 repeats "aaa" to exercise freq>1.
	add("aaa aaa bbb")
	add("bbb ccc")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged doc 2.
	add("aaa ccc")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segReaders := reader.GetSegmentReaders()
	if len(segReaders) < 2 {
		t.Fatalf("expected >= 2 segments, got %d", len(segReaders))
	}
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
	}
	mergedSI := index.NewSegmentInfo("_merged", total, dir)
	mergedSI.SetCodec(index.GetDefaultCodec().Name())
	merger, err := index.NewSegmentMerger(codecReaders, mergedSI, nil, dir, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		t.Fatalf("NewSegmentMerger: %v", err)
	}
	ms, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	codec := index.GetDefaultCodec()
	tvr, err := codec.TermVectorsFormat().VectorsReader(dir, mergedSI, ms.MergeFieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("TermVectors VectorsReader: %v", err)
	}
	defer tvr.Close()

	// term -> frequency (TotalTermFreq) per merged doc.
	want := []map[string]int64{
		{"aaa": 2, "bbb": 1},
		{"bbb": 1, "ccc": 1},
		{"aaa": 1, "ccc": 1},
	}
	for docID := 0; docID < total; docID++ {
		fields, err := tvr.Get(docID)
		if err != nil {
			t.Fatalf("TV Get(%d): %v", docID, err)
		}
		got := readDocTV(t, fields)
		exp := want[docID]
		if len(got) != len(exp) {
			t.Errorf("doc %d terms = %v, want %v", docID, got, exp)
			continue
		}
		for term, freq := range exp {
			gf, ok := got[term]
			if !ok {
				t.Errorf("doc %d missing term %q (got %v)", docID, term, got)
				continue
			}
			if gf != freq {
				t.Errorf("doc %d term %q freq = %d, want %d", docID, term, gf, freq)
			}
		}
	}
}
