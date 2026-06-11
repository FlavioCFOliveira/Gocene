// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

import (
	"testing"
)

// TestLucene60RWPointsFormat_Name verifies the PointsFormat Name method.
func TestLucene60RWPointsFormat_Name(t *testing.T) {
	f := NewLucene60PointsFormat()
	if n := f.Name(); n != "Lucene60PointsFormat" {
		t.Errorf("Name: got %q, want %q", n, "Lucene60PointsFormat")
	}
}

// TestLucene60RWPointsFormat_FieldsWriter verifies the legacy format
// returns an error from FieldsWriter.
func TestLucene60RWPointsFormat_FieldsWriter(t *testing.T) {
	f := NewLucene60PointsFormat()
	_, err := f.FieldsWriter(nil)
	if err == nil {
		t.Fatal("expected error from FieldsWriter on legacy format")
	}
}

// TestLucene60RWPointsFormat_VersionConstants verifies the version
// constants used by the Lucene60 points format.
func TestLucene60RWPointsFormat_VersionConstants(t *testing.T) {
	if pointsDataVersionStart != 0 {
		t.Errorf("pointsDataVersionStart: got %d, want 0", pointsDataVersionStart)
	}
	if pointsIndexVersionStart != 0 {
		t.Errorf("pointsIndexVersionStart: got %d, want 0", pointsIndexVersionStart)
	}
	if pointsDataVersionCurrent != pointsDataVersionStart {
		t.Errorf("pointsDataVersionCurrent mismatch")
	}
	if pointsIndexVersionCurrent != pointsIndexVersionStart {
		t.Errorf("pointsIndexVersionCurrent mismatch")
	}
}
