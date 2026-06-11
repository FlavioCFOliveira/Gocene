// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestBaseLucene80DocValuesFormatTestCase_Name verifies that the default
// Lucene80DocValuesFormat instance reports the correct codec name.
func TestBaseLucene80DocValuesFormatTestCase_Name(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestBaseLucene80DocValuesFormatTestCase_Mode verifies the mode constants.
func TestBaseLucene80DocValuesFormatTestCase_Mode(t *testing.T) {
	if Lucene80DVModeBestSpeed != 0 {
		t.Errorf("Lucene80DVModeBestSpeed = %d, want 0", Lucene80DVModeBestSpeed)
	}
	if Lucene80DVModeBestCompression != 1 {
		t.Errorf("Lucene80DVModeBestCompression = %d, want 1", Lucene80DVModeBestCompression)
	}
}
