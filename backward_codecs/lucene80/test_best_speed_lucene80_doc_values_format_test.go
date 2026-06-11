// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestBestSpeedLucene80DocValuesFormat_Name verifies the format Name().
func TestBestSpeedLucene80DocValuesFormat_Name(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestBestSpeedLucene80DocValuesFormat_Mode verifies that the default
// constructor sets BEST_SPEED mode.
func TestBestSpeedLucene80DocValuesFormat_Mode(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if f.Mode() != Lucene80DVModeBestSpeed {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestSpeed", f.Mode())
	}
}

// TestBestSpeedLucene80DocValuesFormat_ExplicitMode verifies with explicit
// BestSpeed mode.
func TestBestSpeedLucene80DocValuesFormat_ExplicitMode(t *testing.T) {
	f := NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestSpeed)
	if f.Mode() != Lucene80DVModeBestSpeed {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestSpeed", f.Mode())
	}
}
