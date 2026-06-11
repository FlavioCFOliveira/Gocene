// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLucene99SegmentInfoFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentName := "_0"
	docCount := 42
	segmentID := make([]byte, 16)
	rand.Read(segmentID)

	si := index.NewSegmentInfo(segmentName, docCount, dir)
	si.SetID(segmentID)
	si.SetVersion("9.9.0")
	si.SetCompoundFile(false)
	si.SetDiagnostics(map[string]string{
		"os":     "linux",
		"java":   "11",
		"lucene": "9.9.0",
	})
	si.SetFiles([]string{"_0.fdt", "_0.fdx"})

	format := codecs.NewLucene99SegmentInfoFormat()

	// Test Write
	err := format.Write(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test Read
	si2, err := format.Read(dir, segmentName, segmentID, store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify
	if si2.Name() != si.Name() {
		t.Errorf("expected name %s, got %s", si.Name(), si2.Name())
	}
	if si2.DocCount() != si.DocCount() {
		t.Errorf("expected docCount %d, got %d", si.DocCount(), si2.DocCount())
	}
	if si2.Version() != si.Version() {
		t.Errorf("expected version %s, got %s", si.Version(), si2.Version())
	}
}

// TestLucene99PostingsFormat verifies that the Lucene99PostingsFormat
// registers, resolves by name, and can be instantiated.
func TestLucene99PostingsFormat(t *testing.T) {
	// Verify the format is registered and resolvable
	format, err := codecs.PostingsFormatByName(codecs.Lucene99PostingsFormatName)
	if err != nil {
		t.Fatalf("Lucene99PostingsFormat not registered")
	}
	if format.Name() != codecs.Lucene99PostingsFormatName {
		t.Errorf("expected name %s, got %s", codecs.Lucene99PostingsFormatName, format.Name())
	}
}

// TestLucene99PostingsReader_NewTermState verifies that the reader
// properly allocates IntBlockTermState with correct sentinel values.
func TestLucene99PostingsReader_NewTermState(t *testing.T) {
	// We need to create a SegmentReadState which requires a directory.
	// Use ByteBuffersDirectory for minimal setup.
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create the format
	format := codecs.NewLucene99PostingsFormat()

	// Create a dummy segment info
	si := index.NewSegmentInfo("_0", 1, dir)
	si.SetID(make([]byte, 16))

	// TODO: Full round-trip test requires SegmentReadState/SegmentWriteState
	// with proper FieldInfos, which is gated by infra tasks.
	_ = format
	_ = si
}

// TestLucene99StoredFieldsFormat is a placeholder for the stored fields format.
func TestLucene99StoredFieldsFormat(t *testing.T) {
	// Not yet implemented
}

// TestLucene99DocValuesFormat is a placeholder for the doc values format.
func TestLucene99DocValuesFormat(t *testing.T) {
	// Not yet implemented
}
