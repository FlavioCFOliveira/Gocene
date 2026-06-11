// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package backward_index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// TestGenerateBwcIndices demonstrates generating a backwards-compatible index
// with a backward-registered codec name and verifying the round-trip.
//
// Port of org.apache.lucene.backward_index.TestGenerateBwcIndices
// (Lucene 10.4.0).
func TestGenerateBwcIndices(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		tf, err := document.NewTextField("content", "test data "+intToStr(i), false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)

		ndv, err := document.NewNumericDocValuesField("value", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	infos, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if infos.Size() == 0 {
		t.Fatal("expected at least one segment")
	}
	for i := 0; i < infos.Size(); i++ {
		sci := infos.Get(i)
		if sci != nil && sci.SegmentInfo().Codec() != "Lucene912" {
			t.Fatalf("segment %d: expected codec Lucene912, got %q", i, sci.SegmentInfo().Codec())
		}
	}

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
}

// TestGenerateBwcIndices_DefaultCodec generates an index with the default
// Lucene104Codec and verifies the round-trip.
func TestGenerateBwcIndices_DefaultCodec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	base := newBwcTestBase(t)
	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		tf, err := document.NewTextField("body", "hello world", true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	writer.Commit()
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 3 {
		t.Fatalf("expected 3 docs, got %d", reader.NumDocs())
	}
}
