// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"
)

// TestLucene102RWBinaryQuantizedVectorsFormat_Constructor verifies that the
// constructor sets correct name and version.
func TestLucene102RWBinaryQuantizedVectorsFormat_Constructor(t *testing.T) {
	f := NewLucene102BinaryQuantizedVectorsFormat("10.2")
	if f.Name != "Lucene102BinaryQuantizedVectorsFormat" {
		t.Errorf("Name = %q, want %q", f.Name, "Lucene102BinaryQuantizedVectorsFormat")
	}
	if f.Version != "10.2" {
		t.Errorf("Version = %q, want %q", f.Version, "10.2")
	}
}

// TestLucene102RWBinaryQuantizedVectorsFormat_HnswFormat verifies the HNSW
// format constructor.
func TestLucene102RWBinaryQuantizedVectorsFormat_HnswFormat(t *testing.T) {
	h := NewLucene102HnswBinaryQuantizedVectorsFormat("10.2")
	if h.Name != "Lucene102HnswBinaryQuantizedVectorsFormat" {
		t.Errorf("Name = %q, want %q", h.Name, "Lucene102HnswBinaryQuantizedVectorsFormat")
	}
	if h.Version != "10.2" {
		t.Errorf("Version = %q, want %q", h.Version, "10.2")
	}
}
