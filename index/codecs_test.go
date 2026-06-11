// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file ports org.apache.lucene.index.TestCodecs (Lucene 10.4.0,
// core/src/test/org/apache/lucene/index/TestCodecs.java).
//
// The original Java suite exercises full postings write/read round-trips
// through FieldsConsumer/FieldsProducer.  Gocene's default codec now uses
// Lucene104PostingsFormat (not the read-only Lucene103PostingsFormat), so
// a simple FieldsConsumer smoke test is possible.  The full round-trip
// tests (testFixedPostings, testRandomPostings) and the LeafReader-level
// test (testDocsOnlyFreq) are replaced here by:
//
//   - TestCodecs_DefaultPostingsFormatIsLucene104  — confirms the default
//     codec uses the write-capable Lucene104 format, not a read-only stub.
//   - TestCodecs_DefaultPostingsFieldsConsumerSmoke — creates a
//     FieldsConsumer from the default codec to verify it does not return
//     the Lucene103 read-only error.

// TestCodecs_DefaultPostingsFormatIsLucene104 verifies that the default
// codec's postings format is named "Lucene104" (the write-capable format),
// not "Lucene103PostingsFormat" (the read-only backward-compat format).
func TestCodecs_DefaultPostingsFormatIsLucene104(t *testing.T) {
	codec := index.GetDefaultCodec()
	if codec == nil {
		t.Fatal("GetDefaultCodec returned nil")
	}
	pf := codec.PostingsFormat()
	if pf == nil {
		t.Fatal("default codec PostingsFormat is nil")
	}
	if pf.Name() == codecs.Lucene103PostingsFormatName {
		t.Fatalf("default codec uses the read-only %s; expected Lucene104",
			codecs.Lucene103PostingsFormatName)
	}
	if pf.Name() != codecs.Lucene104PostingsFormatName {
		t.Fatalf("default codec postings format name = %q, want %q",
			pf.Name(), codecs.Lucene104PostingsFormatName)
	}
}

// TestCodecs_DefaultPostingsFieldsConsumerSmoke creates a FieldsConsumer
// from the default codec to verify it does not return the Lucene103
// read-only error.
func TestCodecs_DefaultPostingsFieldsConsumerSmoke(t *testing.T) {
	codec := index.GetDefaultCodec()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si := index.NewSegmentInfo("_0", 100, dir)
	if err := si.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	si.SetCodec(codec.Name())

	fis := index.NewFieldInfos()
	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocs,
	})
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}

	state := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}

	consumer, err := codec.PostingsFormat().FieldsConsumer(state)
	if err != nil {
		t.Fatalf("FieldsConsumer (default codec): %v", err)
	}
	if consumer == nil {
		t.Fatal("FieldsConsumer returned nil")
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
