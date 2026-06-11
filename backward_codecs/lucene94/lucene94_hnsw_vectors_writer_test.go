// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"errors"
	"testing"
)

// TestWriter_FieldsWriterUnsupported verifies that FieldsWriter returns
// ErrLucene94WriteUnsupported — the write path is not supported for Lucene94.
//
// In the Java test tree, Lucene94HnswVectorsWriter is a support class; in Gocene
// the write path is blocked at the format level. This test validates the sentinel
// error and the format's FieldsWriter behaviour that a writer would compose with.
func TestWriter_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene94WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene94WriteUnsupported, got %v", err)
	}
}

// TestWriter_ErrSentinel verifies the error variable exists and contains the
// expected message.
func TestWriter_ErrSentinel(t *testing.T) {
	err := ErrLucene94WriteUnsupported
	if err.Error() != "old codecs may only be used for reading" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestWriter_FormatCustomParams verifies the format that a writer would compose
// with can be constructed with custom parameters.
func TestWriter_FormatCustomParams(t *testing.T) {
	f := NewLucene94HnswVectorsFormatWithParams(32, 200)
	if f.MaxConn() != 32 {
		t.Errorf("MaxConn: got %d, want 32", f.MaxConn())
	}
	if f.BeamWidth() != 200 {
		t.Errorf("BeamWidth: got %d, want 200", f.BeamWidth())
	}
}

// TestWriter_FormatString verifies String() output for a custom format instance.
func TestWriter_FormatString(t *testing.T) {
	f := NewLucene94HnswVectorsFormatWithParams(32, 200)
	want := "Lucene94HnswVectorsFormat(name=Lucene94HnswVectorsFormat, maxConn=32, beamWidth=200)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestWriter_FormatConstants verifies constants that both reader and writer use.
func TestWriter_FormatConstants(t *testing.T) {
	if Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN != 16 {
		t.Errorf("DEFAULT_MAX_CONN: got %d, want 16", Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH != 100 {
		t.Errorf("DEFAULT_BEAM_WIDTH: got %d, want 100", Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
}

// TestWriter_LoadByteEmpty verifies that LoadByte returns an empty variant
// when the field entry has docsWithFieldOffset == -2.
func TestWriter_LoadByteEmpty(t *testing.T) {
	fe := &lucene94FieldEntry{
		docsWithFieldOffset: -2,
		dimension:           4,
	}
	v, err := LoadByte(fe, nil)
	if err != nil {
		t.Fatalf("LoadByte empty: unexpected error: %v", err)
	}
	if v.Size() != 0 {
		t.Errorf("Size: got %d, want 0", v.Size())
	}
}

// TestWriter_LoadFloatEmpty verifies that LoadFloat returns an empty variant
// when the field entry has docsWithFieldOffset == -2.
func TestWriter_LoadFloatEmpty(t *testing.T) {
	fe := &lucene94FieldEntry{
		docsWithFieldOffset: -2,
		dimension:           4,
	}
	v, err := LoadFloat(fe, nil)
	if err != nil {
		t.Fatalf("LoadFloat empty: unexpected error: %v", err)
	}
	if v.Size() != 0 {
		t.Errorf("Size: got %d, want 0", v.Size())
	}
}
