// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"errors"
	"testing"
)

// TestLucene92HnswVectorsFormat_DefaultParams verifies that the default constructor
// sets the expected maxConn and beamWidth values.
// Port of TestLucene92HnswVectorsFormat.testToString (Java).
func TestLucene92HnswVectorsFormat_DefaultParams(t *testing.T) {
	f := NewLucene92HnswVectorsFormat()
	if f.MaxConn() != Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("MaxConn: got %d, want %d", f.MaxConn(), Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if f.BeamWidth() != Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH {
		t.Errorf("BeamWidth: got %d, want %d", f.BeamWidth(), Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
}

// TestLucene92HnswVectorsFormat_CustomParams verifies custom constructor parameters.
func TestLucene92HnswVectorsFormat_CustomParams(t *testing.T) {
	f := NewLucene92HnswVectorsFormatWithParams(10, 20)
	if f.MaxConn() != 10 {
		t.Errorf("MaxConn: got %d, want 10", f.MaxConn())
	}
	if f.BeamWidth() != 20 {
		t.Errorf("BeamWidth: got %d, want 20", f.BeamWidth())
	}
}

// TestLucene92HnswVectorsFormat_Name verifies the codec name.
func TestLucene92HnswVectorsFormat_Name(t *testing.T) {
	f := NewLucene92HnswVectorsFormat()
	const want = "lucene92HnswVectorsFormat"
	if f.Name() != want {
		t.Errorf("Name: got %q, want %q", f.Name(), want)
	}
}

// TestLucene92HnswVectorsFormat_String mirrors testToString in the Java test peer.
// Expected output matches Java Lucene92HnswVectorsFormat.toString().
func TestLucene92HnswVectorsFormat_String(t *testing.T) {
	f := NewLucene92HnswVectorsFormatWithParams(10, 20)
	const want = "Lucene92HnswVectorsFormat(name = Lucene92HnswVectorsFormat, maxConn = 10, beamWidth=20)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene92HnswVectorsFormat_DefaultString verifies toString with default params.
func TestLucene92HnswVectorsFormat_DefaultString(t *testing.T) {
	f := NewLucene92HnswVectorsFormat()
	const want = "Lucene92HnswVectorsFormat(name = Lucene92HnswVectorsFormat, maxConn = 16, beamWidth=100)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene92HnswVectorsFormat_GetMaxDimensions verifies the 1024-dimension cap.
func TestLucene92HnswVectorsFormat_GetMaxDimensions(t *testing.T) {
	f := NewLucene92HnswVectorsFormat()
	if got := f.GetMaxDimensions("any"); got != 1024 {
		t.Errorf("GetMaxDimensions: got %d, want 1024", got)
	}
}

// TestLucene92HnswVectorsFormat_FieldsWriterUnsupported confirms that FieldsWriter
// returns ErrLucene92WriteUnsupported — write operations are forbidden on old formats.
func TestLucene92HnswVectorsFormat_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene92HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene92WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene92WriteUnsupported, got %v", err)
	}
}

// TestLucene92HnswVectorsFormat_Constants verifies that the package-level constants
// match the Java source values.
func TestLucene92HnswVectorsFormat_Constants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"DEFAULT_MAX_CONN", Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN, 16},
		{"DEFAULT_BEAM_WIDTH", Lucene92HnswVectorsFormat_DEFAULT_BEAM_WIDTH, 100},
		{"MAX_DIMENSIONS", Lucene92HnswVectorsFormat_MAX_DIMENSIONS, 1024},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}
