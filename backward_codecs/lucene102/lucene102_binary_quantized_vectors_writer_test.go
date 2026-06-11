// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"
)

// TestLucene102BinaryQuantizedVectorsWriter_Format verifies the format constructor.
func TestLucene102BinaryQuantizedVectorsWriter_Format(t *testing.T) {
	f := NewLucene102BinaryQuantizedVectorsFormat("10.2")
	if f.Name != "Lucene102BinaryQuantizedVectorsFormat" {
		t.Errorf("Name = %q, want %q", f.Name, "Lucene102BinaryQuantizedVectorsFormat")
	}
	if f.Version != "10.2" {
		t.Errorf("Version = %q, want %q", f.Version, "10.2")
	}
}

// TestLucene102BinaryQuantizedVectorsWriter_OffHeapValues verifies the
// OffHeapBinarizedVectorValues constructor.
func TestLucene102BinaryQuantizedVectorsWriter_OffHeapValues(t *testing.T) {
	oh := NewOffHeapBinarizedVectorValues("10.2")
	if oh.Name != "OffHeapBinarizedVectorValues" {
		t.Errorf("Name = %q, want %q", oh.Name, "OffHeapBinarizedVectorValues")
	}
	if oh.Version != "10.2" {
		t.Errorf("Version = %q, want %q", oh.Version, "10.2")
	}
}

// TestLucene102BinaryQuantizedVectorsWriter_Reader verifies the reader constructor.
func TestLucene102BinaryQuantizedVectorsWriter_Reader(t *testing.T) {
	r := NewLucene102BinaryQuantizedVectorsReader("10.2")
	if r.Name != "Lucene102BinaryQuantizedVectorsReader" {
		t.Errorf("Name = %q, want %q", r.Name, "Lucene102BinaryQuantizedVectorsReader")
	}
	if r.Version != "10.2" {
		t.Errorf("Version = %q, want %q", r.Version, "10.2")
	}
}
