// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"
)

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_Constructor verifies the
// HNSW binary quantized vectors format constructor.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_Constructor(t *testing.T) {
	h := NewLucene102HnswBinaryQuantizedVectorsFormat("10.2")
	if h.Name != "Lucene102HnswBinaryQuantizedVectorsFormat" {
		t.Errorf("Name = %q, want %q", h.Name, "Lucene102HnswBinaryQuantizedVectorsFormat")
	}
	if h.Version != "10.2" {
		t.Errorf("Version = %q, want %q", h.Version, "10.2")
	}
}

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_OffHeapValues verifies the
// OffHeapBinarizedVectorValues constructor.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_OffHeapValues(t *testing.T) {
	oh := NewOffHeapBinarizedVectorValues("10.2.0")
	if oh.Name != "OffHeapBinarizedVectorValues" {
		t.Errorf("Name = %q, want %q", oh.Name, "OffHeapBinarizedVectorValues")
	}
	if oh.Version != "10.2.0" {
		t.Errorf("Version = %q, want %q", oh.Version, "10.2.0")
	}
}

// TestLucene102RWHnswBinaryQuantizedVectorsFormat_BinaryFormat verifies the
// binary quantized vectors format constructor.
func TestLucene102RWHnswBinaryQuantizedVectorsFormat_BinaryFormat(t *testing.T) {
	f := NewLucene102BinaryQuantizedVectorsFormat("10.2")
	if f.Name != "Lucene102BinaryQuantizedVectorsFormat" {
		t.Errorf("Name = %q, want %q", f.Name, "Lucene102BinaryQuantizedVectorsFormat")
	}
	if f.Version != "10.2" {
		t.Errorf("Version = %q, want %q", f.Version, "10.2")
	}
}
