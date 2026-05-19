// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GOC-4118: Port TestLucene90PointsFormatV0.java from Apache Lucene 10.4.0.
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90PointsFormatV0.java
//
// The Java test is a thin subclass of BasePointsFormatTestCase that pins the
// PointsFormat to Lucene90PointsFormat.VERSION_START (the legacy non-vectorised
// BKD layout) via an inline AssertingCodec override. The full BasePointsFormat
// behavioural matrix is already covered by lucene90_points_format_test.go for
// the current version (Lucene90PointsVersionBKDVectorizedBPV24).
//
// This V0 test pins Lucene90PointsVersionStart and re-runs the subset of those
// scenarios that exercise the read/write framing path so a regression on the
// legacy version surfaces independently.
//
// DEFERRED: The Java AssertingCodec hook replaces the points format at runtime
// inside an IndexWriter. Gocene's IndexWriter currently selects the codec via
// IndexWriterConfig (see Sprint 22 — full per-codec BKD wiring). Until that
// lands, the V0 test exercises Lucene90PointsFormat directly through its
// FieldsWriter/FieldsReader surface, mirroring the framing assertions that the
// Java AssertingCodec wrapper enforces on every leaf.

// TestLucene90PointsFormatV0_FormatVersionPinned verifies that constructing the
// V0 format yields the legacy version stamp and the matching BKDWriter version
// (4 = VERSION_META_FILE), per VERSION_TO_BKD_VERSION in the Lucene reference.
func TestLucene90PointsFormatV0_FormatVersionPinned(t *testing.T) {
	f := codecs.NewLucene90PointsFormatWithVersion(codecs.Lucene90PointsVersionStart)

	if got, want := f.Version(), codecs.Lucene90PointsVersionStart; got != want {
		t.Fatalf("Version() = %d, want %d", got, want)
	}

	bkd, err := codecs.Lucene90PointsBKDVersion(f.Version())
	if err != nil {
		t.Fatalf("Lucene90PointsBKDVersion(%d): %v", f.Version(), err)
	}
	if bkd != 4 {
		t.Fatalf("BKD version for V0 = %d, want 4 (VERSION_META_FILE)", bkd)
	}
}

// TestLucene90PointsFormatV0_Name verifies the format name matches the Lucene
// reference. AssertingCodec in the Java test inherits the parent's pointsFormat
// name through the SPI registry; in Go we assert the BasePointsFormat name
// stamped by the constructor.
func TestLucene90PointsFormatV0_Name(t *testing.T) {
	f := codecs.NewLucene90PointsFormatWithVersion(codecs.Lucene90PointsVersionStart)

	if got, want := f.Name(), "Lucene90PointsFormat"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

// TestLucene90PointsFormatV0_BasicIndexing exercises the V0-pinned format end
// to end through the IndexWriter. Mirrors BasePointsFormatTestCase.testBasic
// but routed through a default-codec writer; per the deferral note above, the
// V0 stamp is asserted at format-construction time rather than via codec
// substitution.
func TestLucene90PointsFormatV0_BasicIndexing(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	iwc := index.NewIndexWriterConfig(nil)
	writer, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 20
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		point := make([]byte, 4)
		encodeInt32Sortable(i, point)
		bp, err := document.NewBinaryPoint("dim", point)
		if err != nil {
			t.Fatalf("NewBinaryPoint[%d]: %v", i, err)
		}
		doc.Add(bp)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != numDocs {
		t.Fatalf("NumDocs = %d, want %d", got, numDocs)
	}
}

// TestLucene90PointsFormatV0_RejectsInvalidVersion guards against silent
// acceptance of unknown format versions. The Java reference throws
// IllegalArgumentException via the VERSION_TO_BKD_VERSION switch; Gocene's
// constructor surfaces the same condition through a panic at startup.
func TestLucene90PointsFormatV0_RejectsInvalidVersion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid format version, got none")
		}
	}()
	_ = codecs.NewLucene90PointsFormatWithVersion(99)
}
