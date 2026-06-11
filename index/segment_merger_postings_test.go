// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the postings leg of the segment merge (rmp #14/#114):
// two committed segments are merged through SegmentMerger and the merged
// segment's postings are read back per term, proving the postings are merged
// with docIDs remapped into the merged (concatenated) doc space.

package index_test

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestSegmentMerger_PostingsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addBody := func(text string) {
		doc := document.NewDocument()
		f, _ := document.NewTextField("body", text, true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1.
	addBody("aaa bbb")
	addBody("aaa ccc")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged docs 2,3.
	addBody("bbb ddd")
	addBody("aaa")
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

	merger, err := index.NewSegmentMerger(codecReaders, mergedSI, nil, nil, dir, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		t.Fatalf("NewSegmentMerger: %v", err)
	}
	ms, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Read the merged postings back.
	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	fp, err := codec.PostingsFormat().FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer fp.Close()

	terms, err := fp.Terms("body")
	if err != nil {
		t.Fatalf("Terms(body): %v", err)
	}
	if terms == nil {
		t.Fatalf("Terms(body) is nil")
	}

	postingsOf := func(term string) []int {
		te, err := terms.GetIterator()
		if err != nil {
			t.Fatalf("GetIterator: %v", err)
		}
		var pe interface {
			NextDoc() (int, error)
		}
		for {
			tm, err := te.Next()
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if tm == nil {
				return nil // term not present
			}
			if tm.Text() == term {
				p, err := te.Postings(index.PostingsFlagAll)
				if err != nil {
					t.Fatalf("Postings(%q): %v", term, err)
				}
				pe = p
				break
			}
		}
		if pe == nil {
			return nil
		}
		var docs []int
		for {
			d, err := pe.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc(%q): %v", term, err)
			}
			if d == index.NO_MORE_DOCS {
				break
			}
			docs = append(docs, d)
		}
		sort.Ints(docs)
		return docs
	}

	// Expected merged docIDs:
	//   doc0 "aaa bbb", doc1 "aaa ccc", doc2 "bbb ddd", doc3 "aaa"
	want := map[string][]int{
		"aaa": {0, 1, 3},
		"bbb": {0, 2},
		"ccc": {1},
		"ddd": {2},
	}
	for term, exp := range want {
		got := postingsOf(term)
		if !equalIntSlice(got, exp) {
			t.Errorf("postings(%q) = %v, want %v", term, got, exp)
		}
	}
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
