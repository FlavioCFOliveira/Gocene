// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewLucene99HnswScalarQuantizedVectorsFormat_Defaults verifies that
// NewLucene99HnswScalarQuantizedVectorsFormat sets Name and Version correctly.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_Defaults(t *testing.T) {
	f := NewLucene99HnswScalarQuantizedVectorsFormat("v0")
	if f.Name != "Lucene99HnswScalarQuantizedVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene99HnswScalarQuantizedVectorsFormat")
	}
	if f.Version != "v0" {
		t.Errorf("Version: got %q, want %q", f.Version, "v0")
	}
}

// TestNewLucene99HnswScalarQuantizedVectorsFormat_VersionTracking verifies that
// different version strings are tracked correctly.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_VersionTracking(t *testing.T) {
	versions := []string{"v0", "v1", "9.9.0", "10.4.0"}
	for _, v := range versions {
		f := NewLucene99HnswScalarQuantizedVectorsFormat(v)
		if f.Version != v {
			t.Errorf("Version: got %q, want %q", f.Version, v)
		}
	}
}

// TestNewLucene99HnswScalarQuantizedVectorsFormat_NameConstant verifies that the
// Name field is always the same regardless of version.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_NameConstant(t *testing.T) {
	f1 := NewLucene99HnswScalarQuantizedVectorsFormat("a")
	f2 := NewLucene99HnswScalarQuantizedVectorsFormat("b")
	if f1.Name != f2.Name {
		t.Errorf("Name should be constant: f1.Name=%q f2.Name=%q", f1.Name, f2.Name)
	}
}
