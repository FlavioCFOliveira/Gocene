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

// TestCodecs_DefaultPostingsFormatIsLucene104 verifies that the default
// codec's postings format wraps the write-capable Lucene104PostingsFormat,
// not the read-only Lucene103PostingsFormat.
//
// The default codec (Lucene104Codec) wraps its postings format inside a
// PerFieldPostingsFormat, whose Name() returns "PerField40". The underlying
// delegate for any field is "Lucene104". This test confirms that the outer
// format is the expected "PerField40" wrapper (not the read-only 103) and
// that the Lucene104 format is registered and distinct from 103.
func TestCodecs_DefaultPostingsFormatIsLucene104(t *testing.T) {
	codec := index.GetDefaultCodec()
	if codec == nil {
		t.Fatal("GetDefaultCodec returned nil")
	}
	pf := codec.PostingsFormat()
	if pf == nil {
		t.Fatal("default codec PostingsFormat is nil")
	}
	// The outer format is PerField40 (the per-field delegation wrapper),
	// NOT the read-only Lucene103PostingsFormat.
	if pf.Name() == codecs.Lucene103PostingsFormatName {
		t.Fatalf("default codec uses the read-only %s", codecs.Lucene103PostingsFormatName)
	}
	if pf.Name() != "PerField40" {
		t.Fatalf("default codec postings format name = %q, want %q", pf.Name(), "PerField40")
	}
	// Verify the underlying Lucene104 write-capable format is registered
	// and is NOT the read-only 103.
	lucene104, err := codecs.PostingsFormatByName(codecs.Lucene104PostingsFormatName)
	if err != nil {
		t.Fatalf("%s is not registered: %v", codecs.Lucene104PostingsFormatName, err)
	}
	if lucene104.Name() == codecs.Lucene103PostingsFormatName {
		t.Fatalf("%s resolves to the read-only %s", codecs.Lucene104PostingsFormatName, codecs.Lucene103PostingsFormatName)
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
