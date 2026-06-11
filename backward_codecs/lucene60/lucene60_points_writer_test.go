// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

import (
	"testing"
)

// TestLucene60PointsWriter_Format verifies the PointsFormat Name and
// the FieldsWriter error on this legacy read-only format.
func TestLucene60PointsWriter_Format(t *testing.T) {
	f := NewLucene60PointsFormat()
	if f == nil {
		t.Fatal("NewLucene60PointsFormat returned nil")
	}
	if f.Name() != "Lucene60PointsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name(), "Lucene60PointsFormat")
	}
	_, err := f.FieldsWriter(nil)
	if err == nil {
		t.Error("FieldsWriter: expected error on legacy format")
	}
	if err.Error() == "" {
		t.Error("FieldsWriter error: expected non-empty message")
	}
}

// TestLucene60PointsWriter_Constants verifies the points wire-format
// constants.
func TestLucene60PointsWriter_Constants(t *testing.T) {
	if pointsDataCodecName != "Lucene60PointsFormatData" {
		t.Errorf("pointsDataCodecName: got %q", pointsDataCodecName)
	}
	if pointsMetaCodecName != "Lucene60PointsFormatMeta" {
		t.Errorf("pointsMetaCodecName: got %q", pointsMetaCodecName)
	}
	if pointsDataExtension != "dim" {
		t.Errorf("pointsDataExtension: got %q", pointsDataExtension)
	}
	if pointsIndexExtension != "dii" {
		t.Errorf("pointsIndexExtension: got %q", pointsIndexExtension)
	}
	if pointsDataVersionStart != 0 {
		t.Errorf("pointsDataVersionStart: got %d", pointsDataVersionStart)
	}
	if pointsDataVersionCurrent != pointsDataVersionStart {
		t.Errorf("pointsDataVersionCurrent != pointsDataVersionStart")
	}
	if pointsIndexVersionStart != 0 {
		t.Errorf("pointsIndexVersionStart: got %d", pointsIndexVersionStart)
	}
	if pointsIndexVersionCurrent != pointsIndexVersionStart {
		t.Errorf("pointsIndexVersionCurrent != pointsIndexVersionStart")
	}
}

// TestLucene60PointsReader_ClosedDefaults verifies that a zero-value
// PointsReader behaves correctly on Close.
func TestLucene60PointsReader_ClosedDefaults(t *testing.T) {
	r := &Lucene60PointsReader{}
	if err := r.Close(); err != nil {
		t.Errorf("Close on zero reader: unexpected error: %v", err)
	}
	// Second close is a no-op (nil dataIn).
	if err := r.Close(); err != nil {
		t.Errorf("second Close: unexpected error: %v", err)
	}
}

// TestLucene60PointsReader_GetFilePointerForField verifies the default
// return value for an unknown field.
func TestLucene60PointsReader_GetFilePointerForField(t *testing.T) {
	r := &Lucene60PointsReader{fieldToFP: map[int]int64{0: 42}}
	if fp := r.GetFilePointerForField(0); fp != 42 {
		t.Errorf("GetFilePointerForField(0): got %d, want 42", fp)
	}
	if fp := r.GetFilePointerForField(99); fp != -1 {
		t.Errorf("GetFilePointerForField(99): got %d, want -1", fp)
	}
}
