// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90StoredFieldsFormat_DefaultModeIsBestSpeed mirrors the
// Java reference behaviour where the no-arg constructor selects
// Mode.BEST_SPEED.
func TestLucene90StoredFieldsFormat_DefaultModeIsBestSpeed(t *testing.T) {
	t.Parallel()
	f := NewLucene90StoredFieldsFormat()
	if got, want := f.Mode(), Lucene90StoredFieldsBestSpeed; got != want {
		t.Fatalf("default mode = %v, want %v", got, want)
	}
	if got, want := f.Name(), "Lucene90StoredFieldsFormat"; got != want {
		t.Fatalf("format name = %q, want %q", got, want)
	}
}

// TestLucene90StoredFieldsFormat_ModeStringRoundTrip ensures the
// textual form persisted on SegmentInfo is parseable back into the
// typed mode.
func TestLucene90StoredFieldsFormat_ModeStringRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode Lucene90StoredFieldsMode
		text string
	}{
		{Lucene90StoredFieldsBestSpeed, "BEST_SPEED"},
		{Lucene90StoredFieldsBestCompression, "BEST_COMPRESSION"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.text, func(t *testing.T) {
			t.Parallel()
			if got := tc.mode.String(); got != tc.text {
				t.Fatalf("String() = %q, want %q", got, tc.text)
			}
			parsed, err := parseLucene90StoredFieldsMode(tc.text)
			if err != nil {
				t.Fatalf("parse(%q): %v", tc.text, err)
			}
			if parsed != tc.mode {
				t.Fatalf("parse(%q) = %v, want %v", tc.text, parsed, tc.mode)
			}
		})
	}
}

// TestLucene90StoredFieldsFormat_ParseModeRejectsUnknown matches Java's
// IllegalArgumentException from Mode.valueOf(String) on unknown input.
func TestLucene90StoredFieldsFormat_ParseModeRejectsUnknown(t *testing.T) {
	t.Parallel()
	if _, err := parseLucene90StoredFieldsMode("NOPE"); err == nil {
		t.Fatal("expected error for unknown mode, got nil")
	}
}

// TestLucene90StoredFieldsFormat_ModeKeyConstant pins the SegmentInfo
// attribute key to the Java MODE_KEY value.
func TestLucene90StoredFieldsFormat_ModeKeyConstant(t *testing.T) {
	t.Parallel()
	if got, want := Lucene90StoredFieldsModeKey, "Lucene90StoredFieldsFormat.mode"; got != want {
		t.Fatalf("MODE_KEY = %q, want %q", got, want)
	}
}

// TestLucene90StoredFieldsFormat_NewWithInvalidModePanics mirrors the
// NullPointerException raised by Objects.requireNonNull on a null mode.
func TestLucene90StoredFieldsFormat_NewWithInvalidModePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid mode, got none")
		}
	}()
	_ = NewLucene90StoredFieldsFormatWithMode(Lucene90StoredFieldsMode(42))
}

// TestLucene90StoredFieldsFormat_FieldsWriterStampsModeAttribute checks
// that FieldsWriter stamps the MODE_KEY attribute on the SegmentInfo
// before delegating to the compressing implementation. The attribute is
// stamped prior to any I/O, so it is set even when the writer is
// subsequently closed without indexing any documents.
func TestLucene90StoredFieldsFormat_FieldsWriterStampsModeAttribute(t *testing.T) {
	t.Parallel()
	for _, mode := range []Lucene90StoredFieldsMode{
		Lucene90StoredFieldsBestSpeed,
		Lucene90StoredFieldsBestCompression,
	} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			f := NewLucene90StoredFieldsFormatWithMode(mode)
			dir, err := store.NewSimpleFSDirectory(t.TempDir())
			if err != nil {
				t.Fatalf("create dir: %v", err)
			}
			defer dir.Close()
			si := index.NewSegmentInfo("_0", 0, dir)
			if err := si.SetID(make([]byte, 16)); err != nil {
				t.Fatalf("set segment ID: %v", err)
			}
			w, err := f.FieldsWriter(dir, si, store.IOContext{})
			if err != nil {
				t.Fatalf("FieldsWriter failed: %v", err)
			}
			// Close immediately without indexing — attribute must already be
			// set at this point regardless of Close success/failure.
			_ = w.Close()
			if got, want := si.GetAttribute(Lucene90StoredFieldsModeKey), mode.String(); got != want {
				t.Fatalf("MODE_KEY = %q, want %q", got, want)
			}
		})
	}
}

// TestLucene90StoredFieldsFormat_FieldsWriterRejectsModeMismatch
// mirrors the Java IllegalStateException when the SegmentInfo already
// carries a different mode value.
func TestLucene90StoredFieldsFormat_FieldsWriterRejectsModeMismatch(t *testing.T) {
	t.Parallel()
	f := NewLucene90StoredFieldsFormatWithMode(Lucene90StoredFieldsBestSpeed)
	si := index.NewSegmentInfo("_0", 0, nil)
	si.SetAttribute(Lucene90StoredFieldsModeKey, "BEST_COMPRESSION")
	_, err := f.FieldsWriter(nil, si, store.IOContext{})
	if err == nil {
		t.Fatal("expected error for mode mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "found existing value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLucene90StoredFieldsFormat_FieldsReaderRequiresModeAttribute
// mirrors the Java IllegalStateException raised when the SegmentInfo
// attribute is absent.
func TestLucene90StoredFieldsFormat_FieldsReaderRequiresModeAttribute(t *testing.T) {
	t.Parallel()
	f := NewLucene90StoredFieldsFormat()
	si := index.NewSegmentInfo("_0", 0, nil)
	_, err := f.FieldsReader(nil, si, nil, store.IOContext{})
	if err == nil {
		t.Fatal("expected error for missing MODE_KEY, got nil")
	}
	if !strings.Contains(err.Error(), "missing value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLucene90StoredFieldsFormat_FieldsReaderRejectsBogusMode pins the
// segment-name-tagged parse error path.
func TestLucene90StoredFieldsFormat_FieldsReaderRejectsBogusMode(t *testing.T) {
	t.Parallel()
	f := NewLucene90StoredFieldsFormat()
	si := index.NewSegmentInfo("_42", 0, nil)
	si.SetAttribute(Lucene90StoredFieldsModeKey, "TURBO")
	_, err := f.FieldsReader(nil, si, nil, store.IOContext{})
	if err == nil {
		t.Fatal("expected error for unknown mode, got nil")
	}
	if !strings.Contains(err.Error(), "_42") || !strings.Contains(err.Error(), "TURBO") {
		t.Fatalf("error %q lacks expected context", err.Error())
	}
}

// TestLucene90StoredFieldsFormat_ImplCarriesTuningConstants pins the
// per-mode tuning parameters against the Apache Lucene 10.4.0 reference.
func TestLucene90StoredFieldsFormat_ImplCarriesTuningConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode            Lucene90StoredFieldsMode
		formatName      string
		chunkSize       int
		maxDocsPerChunk int
	}{
		{Lucene90StoredFieldsBestSpeed, "Lucene90StoredFieldsFastData", 10 * 8 * 1024, 1024},
		{Lucene90StoredFieldsBestCompression, "Lucene90StoredFieldsHighData", 10 * 48 * 1024, 4096},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.mode.String(), func(t *testing.T) {
			t.Parallel()
			f := NewLucene90StoredFieldsFormatWithMode(tc.mode)
			impl, ok := f.impl(tc.mode).(interface {
				FormatName() string
				ChunkSize() int
				MaxDocsPerChunk() int
				BlockShift() int
			})
			if !ok {
				t.Fatal("impl() did not return a Lucene90CompressingStoredFieldsFormat")
			}
			if got := impl.FormatName(); got != tc.formatName {
				t.Fatalf("FormatName = %q, want %q", got, tc.formatName)
			}
			if got := impl.ChunkSize(); got != tc.chunkSize {
				t.Fatalf("ChunkSize = %d, want %d", got, tc.chunkSize)
			}
			if got := impl.MaxDocsPerChunk(); got != tc.maxDocsPerChunk {
				t.Fatalf("MaxDocsPerChunk = %d, want %d", got, tc.maxDocsPerChunk)
			}
			if got := impl.BlockShift(); got != 10 {
				t.Fatalf("BlockShift = %d, want 10", got)
			}
		})
	}
}
