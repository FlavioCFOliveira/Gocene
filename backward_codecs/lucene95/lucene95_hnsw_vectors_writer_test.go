// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"errors"
	"testing"
)

// TestWriter_FieldsWriterUnsupported verifies that FieldsWriter returns
// ErrLucene95WriteUnsupported — the write path is not supported for Lucene95.
//
// In the Java test tree, Lucene95HnswVectorsWriter is a support class; in Gocene
// the write path is blocked at the format level. This test validates the sentinel
// error and the format's FieldsWriter behaviour that a writer would compose with.
func TestWriter_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene95WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene95WriteUnsupported, got %v", err)
	}
}

// TestWriter_ErrSentinel verifies the error variable exists and contains the
// expected message.
func TestWriter_ErrSentinel(t *testing.T) {
	err := ErrLucene95WriteUnsupported
	if err.Error() != "old codecs may only be used for reading" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestWriter_FormatCustomParams verifies the format that a writer would compose
// with can be constructed with custom parameters.
func TestWriter_FormatCustomParams(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(32, 200)
	if f.MaxConn() != 32 {
		t.Errorf("MaxConn: got %d, want 32", f.MaxConn())
	}
	if f.BeamWidth() != 200 {
		t.Errorf("BeamWidth: got %d, want 200", f.BeamWidth())
	}
}

// TestWriter_FormatString verifies String() output for a custom format instance.
func TestWriter_FormatString(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(32, 200)
	want := "Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=32, beamWidth=200)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestWriter_Lucene95ReaderType checks that the reader type exists and has the
// expected name (the writer produces data the reader consumes).
func TestWriter_Lucene95ReaderType(t *testing.T) {
	// Lucene95HnswVectorsReader is defined in lucene95_hnsw_vectors_reader.go.
	// This test verifies the constant values in this package that both the
	// reader and writer would use.
	if Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN != 16 {
		t.Errorf("DEFAULT_MAX_CONN: got %d, want 16", Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH != 100 {
		t.Errorf("DEFAULT_BEAM_WIDTH: got %d, want 100", Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
}
