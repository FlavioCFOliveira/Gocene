// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestLucene50CompressingStoredFieldsFormat_Constructor verifies that the
// format struct is constructed with the expected Name and Version.
func TestLucene50CompressingStoredFieldsFormat_Constructor(t *testing.T) {
	f := NewLucene50CompressingStoredFieldsFormat("test")
	if f == nil {
		t.Fatal("NewLucene50CompressingStoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene50CompressingStoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompressingStoredFieldsFormat")
	}
	if f.Version != "test" {
		t.Errorf("Version: got %q, want %q", f.Version, "test")
	}
}

// TestLucene50CompressingStoredFieldsReader_Constructor verifies that the
// reader struct is constructed with the expected Name and Version.
func TestLucene50CompressingStoredFieldsReader_Constructor(t *testing.T) {
	r := NewLucene50CompressingStoredFieldsReader("v1.0")
	if r == nil {
		t.Fatal("NewLucene50CompressingStoredFieldsReader returned nil")
	}
	if r.Name != "Lucene50CompressingStoredFieldsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene50CompressingStoredFieldsReader")
	}
	if r.Version != "v1.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "v1.0")
	}
}
