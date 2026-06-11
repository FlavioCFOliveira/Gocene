// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50StoredFieldsFormatHighCompression_Constructor verifies that
// the stored fields format struct is constructed correctly.
func TestLucene50StoredFieldsFormatHighCompression_Constructor(t *testing.T) {
	f := NewLucene50StoredFieldsFormat("hc")
	if f == nil {
		t.Fatal("NewLucene50StoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene50StoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50StoredFieldsFormat")
	}
	if f.Version != "hc" {
		t.Errorf("Version: got %q, want %q", f.Version, "hc")
	}
}

// TestLucene50StoredFieldsFormatHighCompression_Constants verifies the
// block size constant used by stored fields format.
func TestLucene50StoredFieldsFormatHighCompression_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
}
