// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewOffHeapQuantizedByteVectorValues_Name verifies that
// NewOffHeapQuantizedByteVectorValues sets the correct Name.
func TestNewOffHeapQuantizedByteVectorValues_Name(t *testing.T) {
	v := NewOffHeapQuantizedByteVectorValues("9.9.0")
	if v.Name != "OffHeapQuantizedByteVectorValues" {
		t.Errorf("Name: got %q, want %q", v.Name, "OffHeapQuantizedByteVectorValues")
	}
}

// TestNewOffHeapQuantizedByteVectorValues_Version verifies the Version field.
func TestNewOffHeapQuantizedByteVectorValues_Version(t *testing.T) {
	v := NewOffHeapQuantizedByteVectorValues("test")
	if v.Version != "test" {
		t.Errorf("Version: got %q, want %q", v.Version, "test")
	}
}

// TestNewLucene99HnswScalarQuantizedVectorsFormat_Name verifies the Name field
// matches the Java class name.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_Name(t *testing.T) {
	f := NewLucene99HnswScalarQuantizedVectorsFormat("hnsw-v0")
	if f.Name != "Lucene99HnswScalarQuantizedVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene99HnswScalarQuantizedVectorsFormat")
	}
}

// TestNewLucene99HnswScalarQuantizedVectorsFormat_VersionSuffix verifies that
// version strings with suffixes are stored faithfully.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_VersionSuffix(t *testing.T) {
	f := NewLucene99HnswScalarQuantizedVectorsFormat("hnsw-v0")
	if f.Version != "hnsw-v0" {
		t.Errorf("Version: got %q, want %q", f.Version, "hnsw-v0")
	}
}
