// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"
)

// TestLucene102RWBinaryQuantizedVectorsFormat_Constructor verifies the
// format name and toString.
func TestLucene102RWBinaryQuantizedVectorsFormat_Constructor(t *testing.T) {
	f := NewLucene102BinaryQuantizedVectorsFormat()
	if got, want := f.Name(), "Lucene102BinaryQuantizedVectorsFormat"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	want := "Lucene102BinaryQuantizedVectorsFormat(name=Lucene102BinaryQuantizedVectorsFormat, " +
		"flatVectorScorer=Lucene102BinaryFlatVectorsScorer(nonQuantizedDelegate=DefaultFlatVectorScorer()), " +
		"rawVectorFormat=Lucene99FlatVectorsFormat(vectorsScorer=DefaultFlatVectorScorer()))"
	if got := f.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

// TestLucene102RWBinaryQuantizedVectorsFormat_HnswFormat verifies the HNSW
// format name.
func TestLucene102RWBinaryQuantizedVectorsFormat_HnswFormat(t *testing.T) {
	h := NewLucene102HnswBinaryQuantizedVectorsFormat()
	if got, want := h.Name(), "Lucene102HnswBinaryQuantizedVectorsFormat"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}
