// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the stored-fields leg of the segment merge
// (rmp #14/#114): two committed segments are merged through SegmentMerger and
// the merged segment's stored fields are read back and compared, proving the
// merge preserves every live document's stored content in order.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// smCapture records the stored fields visited for one document.
type smCapture struct {
	fields map[string]string
}

func (c *smCapture) StringField(field string, value string)  { c.fields[field] = value }
func (c *smCapture) BinaryField(field string, value []byte)  { c.fields[field] = string(value) }
func (c *smCapture) IntField(field string, value int)        {}
func (c *smCapture) LongField(field string, value int64)     {}
func (c *smCapture) FloatField(field string, value float32)  {}
func (c *smCapture) DoubleField(field string, value float64) {}

func TestSegmentMerger_StoredFieldsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addStored := func(id, body string) {
		doc := document.NewDocument()
		idF, _ := document.NewStringField("id", id, true)
		doc.Add(idF)
		bF, _ := document.NewTextField("body", body, true)
		doc.Add(bF)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1.
	addStored("1", "alpha")
	addStored("2", "beta")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2.
	addStored("3", "gamma")
	addStored("4", "delta")
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
		// NewCodecReader builds a stub SegmentInfo with docCount 0; set the real
		// maxDoc so the merge enumerates every document.
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

	// Read the merged stored fields back.
	codec := index.GetDefaultCodec()
	sfr, err := codec.StoredFieldsFormat().FieldsReader(dir, mergedSI, ms.MergeFieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("merged FieldsReader: %v", err)
	}
	defer sfr.Close()

	want := []struct{ id, body string }{
		{"1", "alpha"}, {"2", "beta"}, {"3", "gamma"}, {"4", "delta"},
	}
	for docID := 0; docID < total; docID++ {
		cap := &smCapture{fields: map[string]string{}}
		if err := sfr.VisitDocument(docID, cap); err != nil {
			t.Fatalf("VisitDocument(%d): %v", docID, err)
		}
		if cap.fields["id"] != want[docID].id {
			t.Errorf("doc %d id = %q, want %q", docID, cap.fields["id"], want[docID].id)
		}
		if cap.fields["body"] != want[docID].body {
			t.Errorf("doc %d body = %q, want %q", docID, cap.fields["body"], want[docID].body)
		}
	}
}
