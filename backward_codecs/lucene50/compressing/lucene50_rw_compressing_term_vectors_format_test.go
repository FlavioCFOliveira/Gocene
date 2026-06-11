// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestLucene50RWCompressingTermVectorsFormat_Constructor verifies that
// the read-write term vectors format type is constructable and carries
// the correct Name.
func TestLucene50RWCompressingTermVectorsFormat_Constructor(t *testing.T) {
	f := NewLucene50CompressingTermVectorsFormat("rw")
	if f == nil {
		t.Fatal("NewLucene50CompressingTermVectorsFormat returned nil")
	}
	if f.Name != "Lucene50CompressingTermVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompressingTermVectorsFormat")
	}
	if f.Version != "rw" {
		t.Errorf("Version: got %q, want %q", f.Version, "rw")
	}
}

// TestLucene50RWCompressingTermVectorsFormat_DifferentVersions verifies
// that different version strings are correctly stored.
func TestLucene50RWCompressingTermVectorsFormat_DifferentVersions(t *testing.T) {
	versions := []string{"", "1.0", "v2", "latest"}
	for _, v := range versions {
		f := NewLucene50CompressingTermVectorsFormat(v)
		if f.Version != v {
			t.Errorf("Version: got %q, want %q", f.Version, v)
		}
		if f.Name != "Lucene50CompressingTermVectorsFormat" {
			t.Errorf("Name: got %q", f.Name)
		}
	}
}
