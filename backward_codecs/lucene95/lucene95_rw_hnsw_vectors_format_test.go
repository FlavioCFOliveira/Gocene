// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"errors"
	"testing"
)

// TestRWHnswVectorsFormat_CustomParams verifies custom constructor params.
//
// In the Java test tree, Lucene95RWHnswVectorsFormat is a test-support class;
// this test validates the production format that it would extend.
func TestRWHnswVectorsFormat_CustomParams(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(10, 20)
	if f.MaxConn() != 10 {
		t.Errorf("MaxConn: got %d, want 10", f.MaxConn())
	}
	if f.BeamWidth() != 20 {
		t.Errorf("BeamWidth: got %d, want 20", f.BeamWidth())
	}
}

// TestRWHnswVectorsFormat_String verifies String() formatting.
func TestRWHnswVectorsFormat_String(t *testing.T) {
	f := NewLucene95HnswVectorsFormatWithParams(10, 20)
	const want = "Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=10, beamWidth=20)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestRWHnswVectorsFormat_DefaultString verifies toString with default params.
func TestRWHnswVectorsFormat_DefaultString(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	const want = "Lucene95HnswVectorsFormat(name=Lucene95HnswVectorsFormat, maxConn=16, beamWidth=100)"
	if f.String() != want {
		t.Errorf("String:\n got  %q\n want %q", f.String(), want)
	}
}

// TestRWHnswVectorsFormat_GetMaxDimensions verifies the 1024-dimension cap.
func TestRWHnswVectorsFormat_GetMaxDimensions(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	if got := f.GetMaxDimensions("any"); got != 1024 {
		t.Errorf("GetMaxDimensions: got %d, want 1024", got)
	}
}

// TestRWHnswVectorsFormat_FieldsWriterUnsupported verifies write is forbidden.
func TestRWHnswVectorsFormat_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene95HnswVectorsFormat()
	err := f.FieldsWriter()
	if !errors.Is(err, ErrLucene95WriteUnsupported) {
		t.Errorf("FieldsWriter: expected ErrLucene95WriteUnsupported, got %v", err)
	}
}

// TestRWHnswVectorsFormat_Constants verifies package-level constants.
func TestRWHnswVectorsFormat_Constants(t *testing.T) {
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
