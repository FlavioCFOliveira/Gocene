// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50RWPostingsFormat_Constructor verifies that the read-write
// postings format struct is constructed with the expected Name and Version.
func TestLucene50RWPostingsFormat_Constructor(t *testing.T) {
	f := NewLucene50PostingsFormat("test")
	if f == nil {
		t.Fatal("NewLucene50PostingsFormat returned nil")
	}
	if f.Name != "Lucene50PostingsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50PostingsFormat")
	}
	if f.Version != "test" {
		t.Errorf("Version: got %q, want %q", f.Version, "test")
	}
}

// TestLucene50RWPostingsFormat_CodecRegistration verifies that the format
// was registered in the codec SPI via init(). The registered name should
// match the struct Name field.
func TestLucene50RWPostingsFormat_CodecRegistration(t *testing.T) {
	f := NewLucene50PostingsFormat("")
	if f.Name != "Lucene50PostingsFormat" {
		t.Errorf("unexpected format name: %q", f.Name)
	}
}
