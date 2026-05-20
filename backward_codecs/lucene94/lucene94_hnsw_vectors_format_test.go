// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"errors"
	"testing"
)

// TestLucene94HnswVectorsFormat_DefaultParams verifies default constructor params.
func TestLucene94HnswVectorsFormat_DefaultParams(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	if f.MaxConn() != Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("MaxConn: got %d, want %d", f.MaxConn(), Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if f.BeamWidth() != Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH {
		t.Errorf("BeamWidth: got %d, want %d", f.BeamWidth(), Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
}

// TestLucene94HnswVectorsFormat_CustomParams verifies custom constructor params.
func TestLucene94HnswVectorsFormat_CustomParams(t *testing.T) {
	f := NewLucene94HnswVectorsFormatWithParams(10, 20)
	if f.MaxConn() != 10 {
		t.Errorf("MaxConn: got %d, want 10", f.MaxConn())
	}
	if f.BeamWidth() != 20 {
		t.Errorf("BeamWidth: got %d, want 20", f.BeamWidth())
	}
}

// TestLucene94HnswVectorsFormat_Name verifies the codec name.
func TestLucene94HnswVectorsFormat_Name(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	if f.Name() != "Lucene94HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name(), "Lucene94HnswVectorsFormat")
	}
}

// TestLucene94HnswVectorsFormat_String mirrors testToString from the Java test peer.
// Note: Lucene94 uses "name=" (no spaces around =) unlike Lucene92's "name = ".
func TestLucene94HnswVectorsFormat_String(t *testing.T) {
	f := NewLucene94HnswVectorsFormatWithParams(10, 20)
	const want = "Lucene94HnswVectorsFormat(name=Lucene94HnswVectorsFormat, maxConn=10, beamWidth=20)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene94HnswVectorsFormat_DefaultString verifies toString with default params.
func TestLucene94HnswVectorsFormat_DefaultString(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	const want = "Lucene94HnswVectorsFormat(name=Lucene94HnswVectorsFormat, maxConn=16, beamWidth=100)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestLucene94HnswVectorsFormat_GetMaxDimensions verifies the 1024-dimension cap.
func TestLucene94HnswVectorsFormat_GetMaxDimensions(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	if got := f.GetMaxDimensions("any"); got != 1024 {
		t.Errorf("GetMaxDimensions: got %d, want 1024", got)
	}
}

// TestLucene94HnswVectorsFormat_FieldsWriterUnsupported verifies write is forbidden.
func TestLucene94HnswVectorsFormat_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene94HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene94WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene94WriteUnsupported, got %v", err)
	}
}

// TestLucene94HnswVectorsFormat_Constants verifies package-level constants.
func TestLucene94HnswVectorsFormat_Constants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"DEFAULT_MAX_CONN", Lucene94HnswVectorsFormat_DEFAULT_MAX_CONN, 16},
		{"DEFAULT_BEAM_WIDTH", Lucene94HnswVectorsFormat_DEFAULT_BEAM_WIDTH, 100},
		{"MAX_DIMENSIONS", Lucene94HnswVectorsFormat_MAX_DIMENSIONS, 1024},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}
