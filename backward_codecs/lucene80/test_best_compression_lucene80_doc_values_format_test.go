// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestBestCompressionLucene80DocValuesFormat_Name verifies the format Name().
func TestBestCompressionLucene80DocValuesFormat_Name(t *testing.T) {
	f := NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestCompression)
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestBestCompressionLucene80DocValuesFormat_Mode verifies that the
// BestCompression mode is correctly set.
func TestBestCompressionLucene80DocValuesFormat_Mode(t *testing.T) {
	f := NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestCompression)
	if f.Mode() != Lucene80DVModeBestCompression {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestCompression", f.Mode())
	}
}
