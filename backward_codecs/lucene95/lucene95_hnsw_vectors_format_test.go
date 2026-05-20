// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"errors"
	"testing"
)

// TestLucene95HnswVectorsFormat_DefaultParams verifies default constructor params.
func TestLucene95HnswVectorsFormat_DefaultParams(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	if f.MaxConn() != Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("MaxConn: got %d, want %d", f.MaxConn(), Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if f.BeamWidth() != Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH {
		t.Errorf("BeamWidth: got %d, want %d", f.BeamWidth(), Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
}

// TestLucene95HnswVectorsFormat_CustomParams verifies custom constructor params.
func TestLucene95HnswVectorsFormat_CustomParams(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(10, 20)
	if f.MaxConn() != 10 {
		t.Errorf("MaxConn: got %d, want 10", f.MaxConn())
	}
	if f.BeamWidth() != 20 {
		t.Errorf("BeamWidth: got %d, want 20", f.BeamWidth())
	}
}

// TestLucene95HnswVectorsFormat_Name verifies the codec name.
func TestLucene95HnswVectorsFormat_Name(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	if f.Name() != "Lucene95HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name(), "Lucene95HnswVectorsFormat")
	}
}

// TestLucene95HnswVectorsFormat_String mirrors testToString from the Java test peer.
func TestLucene95HnswVectorsFormat_String(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(10, 20)
	const want = "Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=10, beamWidth=20)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene95HnswVectorsFormat_DefaultString verifies toString with default params.
func TestLucene95HnswVectorsFormat_DefaultString(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	const want = "Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=16, beamWidth=100)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene95HnswVectorsFormat_GetMaxDimensions verifies the 1024-dimension cap.
func TestLucene95HnswVectorsFormat_GetMaxDimensions(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	if got := f.GetMaxDimensions("any"); got != 1024 {
		t.Errorf("GetMaxDimensions: got %d, want 1024", got)
	}
}

// TestLucene95HnswVectorsFormat_FieldsWriterUnsupported verifies write is forbidden.
func TestLucene95HnswVectorsFormat_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene95WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene95WriteUnsupported, got %v", err)
	}
}

// TestLucene95HnswVectorsFormat_Constants verifies package-level constants.
func TestLucene95HnswVectorsFormat_Constants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"DEFAULT_MAX_CONN", Lucene95HnswVectorsFormat_DEFAULT_MAX_CONN, 16},
		{"DEFAULT_BEAM_WIDTH", Lucene95HnswVectorsFormat_DEFAULT_BEAM_WIDTH, 100},
		{"MAX_DIMENSIONS", Lucene95HnswVectorsFormat_MAX_DIMENSIONS, 1024},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}
