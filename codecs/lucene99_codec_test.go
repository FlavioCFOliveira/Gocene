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

// Placeholder for other Lucene 9.9 format tests.
// As Lucene 10.x often uses Lucene 9.x formats for backward compatibility,
// Gocene will eventually need to implement these.

func TestLucene99PostingsFormat(t *testing.T) {
	t.Skip("Lucene99PostingsFormat is not yet implemented in Gocene")
}

func TestLucene99StoredFieldsFormat(t *testing.T) {
	t.Skip("Lucene99StoredFieldsFormat is not yet implemented in Gocene")
}

func TestLucene99DocValuesFormat(t *testing.T) {
	t.Skip("Lucene99DocValuesFormat is not yet implemented in Gocene")
}
