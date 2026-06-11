// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene91

import (
	"testing"
)

// TestRWHnswVectorsFormat_FormatConstructor verifies that Lucene91HnswVectorsFormat
// is constructible and sets Name and Version correctly.
//
// In the Java test tree, Lucene91RWHnswVectorsFormat is a test-support class; in
// Gocene the production format that the RW variant would extend is tested here.
func TestRWHnswVectorsFormat_FormatConstructor(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("9.1.0")
	if f.Name != "Lucene91HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene91HnswVectorsFormat")
	}
	if f.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "9.1.0")
	}
}

// TestRWHnswVectorsFormat_FormatCustomVersion verifies a different version string.
func TestRWHnswVectorsFormat_FormatCustomVersion(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("10.4.0")
	if f.Version != "10.4.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "10.4.0")
	}
}

// TestRWHnswVectorsFormat_CodecConstructor verifies the codec type is constructible.
func TestRWHnswVectorsFormat_CodecConstructor(t *testing.T) {
	c := NewLucene91Codec("9.1.0")
	if c.Name != "Lucene91Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene91Codec")
	}
	if c.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "9.1.0")
	}
}

// TestRWHnswVectorsFormat_ReaderConstructor verifies the reader type.
func TestRWHnswVectorsFormat_ReaderConstructor(t *testing.T) {
	r := NewLucene91HnswVectorsReader("9.1.0")
	if r.Name != "Lucene91HnswVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene91HnswVectorsReader")
	}
	if r.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.1.0")
	}
}

// TestRWHnswVectorsFormat_GraphConstructor verifies the on-heap graph type.
func TestRWHnswVectorsFormat_GraphConstructor(t *testing.T) {
	g := NewLucene91OnHeapHnswGraph("9.1.0")
	if g.Name != "Lucene91OnHeapHnswGraph" {
		t.Errorf("Name: got %q, want %q", g.Name, "Lucene91OnHeapHnswGraph")
	}
	if g.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", g.Version, "9.1.0")
	}
}

// TestRWHnswVectorsFormat_NeighborArrayConstructor verifies the neighbor array.
func TestRWHnswVectorsFormat_NeighborArrayConstructor(t *testing.T) {
	n := NewLucene91NeighborArray("9.1.0")
	if n.Name != "Lucene91NeighborArray" {
		t.Errorf("Name: got %q, want %q", n.Name, "Lucene91NeighborArray")
	}
	if n.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", n.Version, "9.1.0")
	}
}

// TestRWHnswVectorsFormat_BoundsCheckerConstructor verifies the bounds checker.
func TestRWHnswVectorsFormat_BoundsCheckerConstructor(t *testing.T) {
	b := NewLucene91BoundsChecker("9.1.0")
	if b.Name != "Lucene91BoundsChecker" {
		t.Errorf("Name: got %q, want %q", b.Name, "Lucene91BoundsChecker")
	}
	if b.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", b.Version, "9.1.0")
	}
}
