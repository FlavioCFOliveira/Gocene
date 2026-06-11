// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50RWStoredFieldsFormat_Constructor verifies that the read-write
// stored fields format struct is constructed with the expected Name and
// Version.
func TestLucene50RWStoredFieldsFormat_Constructor(t *testing.T) {
	f := NewLucene50StoredFieldsFormat("v1")
	if f == nil {
		t.Fatal("NewLucene50StoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene50StoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50StoredFieldsFormat")
	}
	if f.Version != "v1" {
		t.Errorf("Version: got %q, want %q", f.Version, "v1")
	}
}

// TestLucene50RWStoredFieldsFormat_DifferentVersions verifies the version
// parameter is correctly propagated.
func TestLucene50RWStoredFieldsFormat_DifferentVersions(t *testing.T) {
	f := NewLucene50StoredFieldsFormat("latest")
	if f.Version != "latest" {
		t.Errorf("Version: got %q, want %q", f.Version, "latest")
	}
}
