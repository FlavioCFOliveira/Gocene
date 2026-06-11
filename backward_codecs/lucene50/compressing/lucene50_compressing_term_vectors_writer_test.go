// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestLucene50CompressingTermVectorsFormat_Constructor verifies that the
// format struct is constructed with the expected Name and Version.
func TestLucene50CompressingTermVectorsFormat_Constructor(t *testing.T) {
	f := NewLucene50CompressingTermVectorsFormat("v2")
	if f == nil {
		t.Fatal("NewLucene50CompressingTermVectorsFormat returned nil")
	}
	if f.Name != "Lucene50CompressingTermVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompressingTermVectorsFormat")
	}
	if f.Version != "v2" {
		t.Errorf("Version: got %q, want %q", f.Version, "v2")
	}
}

// TestLucene50CompressingTermVectorsReader_Constructor verifies that the
// reader struct is constructed with the expected Name and Version.
func TestLucene50CompressingTermVectorsReader_Constructor(t *testing.T) {
	r := NewLucene50CompressingTermVectorsReader("v2")
	if r == nil {
		t.Fatal("NewLucene50CompressingTermVectorsReader returned nil")
	}
	if r.Name != "Lucene50CompressingTermVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene50CompressingTermVectorsReader")
	}
	if r.Version != "v2" {
		t.Errorf("Version: got %q, want %q", r.Version, "v2")
	}
}
