// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Phase 1 structural tests for Lucene90PointsFormat. Per-field BKD
// encoding is deferred to Sprint 22; these tests pin the format
// constants and validate that the writer's IndexHeader/Footer framing
// round-trips through the reader's header-validation path.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90PointsFormat_Constants pins the codec names, extensions
// and version constants. These are part of the wire contract.
func TestLucene90PointsFormat_Constants(t *testing.T) {
	for _, c := range []struct {
		name, got, want string
	}{
		{"DataCodec", codecs.Lucene90PointsDataCodec, "Lucene90PointsFormatData"},
		{"IndexCodec", codecs.Lucene90PointsIndexCodec, "Lucene90PointsFormatIndex"},
		{"MetaCodec", codecs.Lucene90PointsMetaCodec, "Lucene90PointsFormatMeta"},
		{"DataExtension", codecs.Lucene90PointsDataExtension, "kdd"},
		{"IndexExtension", codecs.Lucene90PointsIndexExtension, "kdi"},
		{"MetaExtension", codecs.Lucene90PointsMetaExtension, "kdm"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if got, want := codecs.Lucene90PointsVersionStart, int32(0); got != want {
		t.Errorf("VersionStart = %d, want %d", got, want)
	}
	if got, want := codecs.Lucene90PointsVersionCurrent, int32(1); got != want {
		t.Errorf("VersionCurrent = %d, want %d", got, want)
	}
}

// TestLucene90PointsFormat_BKDVersionMapping pins the
// PointsFormat-version -> BKDWriter-version mapping per Lucene 10.4.0.
func TestLucene90PointsFormat_BKDVersionMapping(t *testing.T) {
	cases := []struct {
		version int32
		want    int32
	}{
		{codecs.Lucene90PointsVersionStart, 4},                // VERSION_META_FILE
		{codecs.Lucene90PointsVersionBKDVectorizedBPV24, 6},   // VERSION_VECTORIZE_BPV24_AND_INTRODUCE_BPV21
	}
	for _, c := range cases {
		got, err := codecs.Lucene90PointsBKDVersion(c.version)
		if err != nil {
			t.Errorf("version=%d: %v", c.version, err)
			continue
		}
		if got != c.want {
			t.Errorf("version=%d: got BKD version %d, want %d", c.version, got, c.want)
		}
	}
	if _, err := codecs.Lucene90PointsBKDVersion(99); err == nil {
		t.Error("version=99: expected error, got nil")
	}
}

// TestLucene90PointsFormat_FramingRoundTrip exercises the
// writer-Close-then-reader path: Close on a writer with zero fields
// stamps a CodecUtil-framed file trio, and constructing a reader against
// the same segment must succeed (header validation only).
func TestLucene90PointsFormat_FramingRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo("_0", 0, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}
	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}

	format := codecs.NewLucene90PointsFormat()
	writer, err := format.FieldsWriter(state)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	if !dir.FileExists("_0.kdd") || !dir.FileExists("_0.kdi") || !dir.FileExists("_0.kdm") {
		t.Fatal("expected _0.kdd / _0.kdi / _0.kdm to exist")
	}

	readState := &codecs.SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}
	reader, err := format.FieldsReader(readState)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close: %v", err)
	}
}

// TestLucene90PointsFormat_InvalidVersion verifies the constructor
// rejects an unknown format version (matches the Java IAE).
func TestLucene90PointsFormat_InvalidVersion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on invalid version")
		}
	}()
	codecs.NewLucene90PointsFormatWithVersion(99)
}
