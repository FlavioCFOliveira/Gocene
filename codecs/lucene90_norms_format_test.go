// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Phase 1 structural tests for Lucene90NormsFormat. Per-field encoding
// is deferred to Sprint 22; these tests only validate the format
// constants, the IndexHeader/Footer framing emitted by Close, and the
// producer's ability to validate said framing.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90NormsFormat_Constants pins the codec and extension
// constants that the wire format depends on.
func TestLucene90NormsFormat_Constants(t *testing.T) {
	if got, want := codecs.Lucene90NormsDataCodec, "Lucene90NormsData"; got != want {
		t.Errorf("DataCodec = %q, want %q", got, want)
	}
	if got, want := codecs.Lucene90NormsMetadataCodec, "Lucene90NormsMetadata"; got != want {
		t.Errorf("MetadataCodec = %q, want %q", got, want)
	}
	if got, want := codecs.Lucene90NormsDataExtension, "nvd"; got != want {
		t.Errorf("DataExtension = %q, want %q", got, want)
	}
	if got, want := codecs.Lucene90NormsMetadataExtension, "nvm"; got != want {
		t.Errorf("MetadataExtension = %q, want %q", got, want)
	}
	if got, want := codecs.Lucene90NormsVersionCurrent, int32(0); got != want {
		t.Errorf("VersionCurrent = %d, want %d", got, want)
	}
}

// TestLucene90NormsFormat_EmptyRoundTrip verifies the producer accepts
// the framing emitted by closing an unused consumer.
func TestLucene90NormsFormat_EmptyRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo("_0", 10, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}

	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}

	format := codecs.NewLucene90NormsFormat()
	consumer, err := format.NormsConsumer(state)
	if err != nil {
		t.Fatalf("NormsConsumer: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close consumer: %v", err)
	}

	// Confirm both files exist.
	if !dir.FileExists("_0.nvd") {
		t.Error("expected _0.nvd to exist")
	}
	if !dir.FileExists("_0.nvm") {
		t.Error("expected _0.nvm to exist")
	}

	// Producer must accept the headers without error.
	readState := &codecs.SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}
	producer, err := format.NormsProducer(readState)
	if err != nil {
		t.Fatalf("NormsProducer: %v", err)
	}
	if err := producer.Close(); err != nil {
		t.Fatalf("Close producer: %v", err)
	}
}
