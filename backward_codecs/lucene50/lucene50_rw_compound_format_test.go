// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50RWCompoundFormat_Constructor verifies that the read-write
// compound format struct is constructed with the expected Name and Version.
func TestLucene50RWCompoundFormat_Constructor(t *testing.T) {
	f := NewLucene50CompoundFormat("test")
	if f == nil {
		t.Fatal("NewLucene50CompoundFormat returned nil")
	}
	if f.Name != "Lucene50CompoundFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50CompoundFormat")
	}
	if f.Version != "test" {
		t.Errorf("Version: got %q, want %q", f.Version, "test")
	}
}

// TestLucene50RWCompoundFormat_DifferentVersions verifies that different
// version strings are stored correctly.
func TestLucene50RWCompoundFormat_DifferentVersions(t *testing.T) {
	for _, v := range []string{"", "v1", "latest"} {
		f := NewLucene50CompoundFormat(v)
		if f.Version != v {
			t.Errorf("Version: got %q, want %q", f.Version, v)
		}
		if f.Name != "Lucene50CompoundFormat" {
			t.Errorf("Name: got %q", f.Name)
		}
	}
}
