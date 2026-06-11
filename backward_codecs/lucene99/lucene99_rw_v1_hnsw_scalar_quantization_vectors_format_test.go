// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewLucene99HnswScalarQuantizedVectorsFormat_V1 verifies construction
// with a "v1" version string.
func TestNewLucene99HnswScalarQuantizedVectorsFormat_V1(t *testing.T) {
	f := NewLucene99HnswScalarQuantizedVectorsFormat("v1")
	if f.Name != "Lucene99HnswScalarQuantizedVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene99HnswScalarQuantizedVectorsFormat")
	}
	if f.Version != "v1" {
		t.Errorf("Version: got %q, want %q", f.Version, "v1")
	}
}

// TestNewLucene99SkipWriter_Defaults verifies that NewLucene99SkipWriter sets
// Name and Version correctly.
func TestNewLucene99SkipWriter_Defaults(t *testing.T) {
	w := NewLucene99SkipWriter("9.9.0")
	if w.Name != "Lucene99SkipWriter" {
		t.Errorf("Name: got %q, want %q", w.Name, "Lucene99SkipWriter")
	}
	if w.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", w.Version, "9.9.0")
	}
}

// TestNewLucene99SkipWriter_VersionTracking verifies that NewLucene99SkipWriter
// stores any version string faithfully.
func TestNewLucene99SkipWriter_VersionTracking(t *testing.T) {
	w := NewLucene99SkipWriter("v1-hnsw-quantized")
	if w.Version != "v1-hnsw-quantized" {
		t.Errorf("Version: got %q, want %q", w.Version, "v1-hnsw-quantized")
	}
}

// TestNewLucene99SkipWriter_EmptyVersion verifies that an empty version string
// is accepted.
func TestNewLucene99SkipWriter_EmptyVersion(t *testing.T) {
	w := NewLucene99SkipWriter("")
	if w.Version != "" {
		t.Errorf("Version: got %q, want empty", w.Version)
	}
}
