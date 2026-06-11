// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50RWTermVectorsFormat_Constructor verifies that the read-write
// term vectors format struct is constructed with the expected Name and
// Version.
func TestLucene50RWTermVectorsFormat_Constructor(t *testing.T) {
	f := NewLucene50TermVectorsFormat("v2")
	if f == nil {
		t.Fatal("NewLucene50TermVectorsFormat returned nil")
	}
	if f.Name != "Lucene50TermVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50TermVectorsFormat")
	}
	if f.Version != "v2" {
		t.Errorf("Version: got %q, want %q", f.Version, "v2")
	}
}

// TestLucene50RWTermVectorsFormat_EmptyVersion verifies that empty version
// is accepted.
func TestLucene50RWTermVectorsFormat_EmptyVersion(t *testing.T) {
	f := NewLucene50TermVectorsFormat("")
	if f.Version != "" {
		t.Errorf("Version: got %q, want empty", f.Version)
	}
}
