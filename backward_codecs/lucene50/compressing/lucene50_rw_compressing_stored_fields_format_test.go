// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestLucene50RWCompressingStoredFieldsFormat_Constructor verifies that
// the read-write format type is constructable and carries the correct Name.
func TestLucene50RWCompressingStoredFieldsFormat_Constructor(t *testing.T) {
	f := NewLucene50CompressingStoredFieldsFormat("rw")
	if f == nil {
		t.Fatal("NewLucene50CompressingStoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene50CompressingStoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompressingStoredFieldsFormat")
	}
	if f.Version != "rw" {
		t.Errorf("Version: got %q, want %q", f.Version, "rw")
	}
}

// TestLucene50RWCompressingStoredFieldsFormat_EmptyVersion verifies that an
// empty version string is accepted.
func TestLucene50RWCompressingStoredFieldsFormat_EmptyVersion(t *testing.T) {
	f := NewLucene50CompressingStoredFieldsFormat("")
	if f.Name != "Lucene50CompressingStoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompressingStoredFieldsFormat")
	}
	if f.Version != "" {
		t.Errorf("Version: got %q, want empty", f.Version)
	}
}

// TestLucene50RWCompressingStoredFieldsFormat_DifferentVersions verifies
// that different version strings are correctly stored.
func TestLucene50RWCompressingStoredFieldsFormat_DifferentVersions(t *testing.T) {
	versions := []string{"", "1.0", "v2", "latest"}
	for _, v := range versions {
		f := NewLucene50CompressingStoredFieldsFormat(v)
		if f.Version != v {
			t.Errorf("Version: got %q, want %q", f.Version, v)
		}
	}
}
